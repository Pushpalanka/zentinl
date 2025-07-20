package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/exp/slog"
	"html/template"
	"log"
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
		log.Fatalf("template error: %v", err)
	}

	authN, err := authentication.New(ctx, zitadel.New(*domain), *key,
		oidc.DefaultAuthentication(*clientID, *redirectURI, *key),
	)
	if err != nil {
		log.Fatalf("auth init error: %v", err)
	}
	mw := authentication.Middleware(authN)

	mux := http.NewServeMux()
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
