package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	oidc2 "github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/exp/slog"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zitadel/zitadel-go/v3/pkg/authentication"
	"github.com/zitadel/zitadel-go/v3/pkg/authentication/oidc"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	t, err := template.ParseFS(templates, "templates/*.html")
	if err != nil {
		slog.Error("template error: %v", err)
		os.Exit(1)
	}

	authN, err := authentication.New(ctx, zitadel.New(*domain), *key,
		oidc.DefaultAuthentication(*clientID, *redirectURI, *key),
	)
	if err != nil {
		slog.Error("Failed to initialize ZITADEL authentication", "error", err)
		os.Exit(1)
	}
	mw := authentication.Middleware(authN)

	mux := http.NewServeMux()
	registerRoutes(mux, authN, mw, t)

	serverAddr := fmt.Sprintf(":%s", *port)
	slog.Info("Server starting", "addr", serverAddr)

	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutdown signal received, stopping server...")

	if err := server.Shutdown(context.Background()); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}
}

func registerRoutes(mux *http.ServeMux, authN *authentication.Authenticator[*oidc.UserInfoContext[*oidc2.IDTokenClaims, *oidc2.UserInfo]], mw *authentication.Interceptor[*oidc.UserInfoContext[*oidc2.IDTokenClaims, *oidc2.UserInfo]], t *template.Template) {
	mux.Handle("/auth/", authN)

	mux.Handle("/", mw.CheckAuthentication()(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if authentication.IsAuthenticated(req.Context()) {
			http.Redirect(w, req, "/profile", http.StatusFound)
			return
		}
		if err := t.ExecuteTemplate(w, "home.html", nil); err != nil {
			slog.Error("Failed to execute home template", "error", err)
		}
	})))

	mux.Handle("/profile", mw.RequireAuthentication()(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		authCtx := mw.Context(req.Context())
		userJSON, err := json.MarshalIndent(authCtx.UserInfo, "", "  ")
		if err != nil {
			slog.Error("error marshalling profile", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if err := t.ExecuteTemplate(w, "profile.html", string(userJSON)); err != nil {
			slog.Error("template execution error", "error", err)
		}
	})))

	mux.Handle("/call-api", mw.CheckAuthentication()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			authCtx := mw.Context(r.Context())
			token := authCtx.GetTokens().AccessToken

			apiReq, _ := http.NewRequest("GET", "http://localhost:8090/api/permissions", nil)
			apiReq.Header.Set("Authorization", "Bearer "+token)

			client := http.Client{}
			resp, err := client.Do(apiReq)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			if err := writeResponse(w, resp); err != nil {
				slog.Error("Failed to write response", "error", err)
			}
		})))
}

func writeResponse(w http.ResponseWriter, resp *http.Response) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	response := map[string]interface{}{
		"status": resp.StatusCode,
		"data":   json.RawMessage(bodyBytes),
	}

	combinedJSON, err := json.Marshal(response)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, err = w.Write(combinedJSON)
	return err
}
