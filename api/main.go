package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization/oauth"
	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/auth"
	"github.com/zitadel/zitadel-go/v3/pkg/http/middleware"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
	"golang.org/x/exp/slog"
	"net/http"
	"os"
	"slices"
)

func main() {
	flag.Parse()

	ctx := context.Background()

	// Initiate the zitadel sdk by providing its domain
	// and as this example will focus on authorization (using OAuth2 Introspection),
	// you will also need to initialize that with the downloaded api key.json

	conf := zitadel.New(*domain)
	authZ, err := authorization.New(ctx, conf, oauth.DefaultAuthorization(*key))
	if err != nil {
		slog.Error("zitadel sdk could not initialize authorization", "error", err)
		os.Exit(1)
	}

	mw := middleware.New(authZ)

	c, err := client.New(ctx, conf)
	if err != nil {
		slog.Error("zitadel sdk could not initialize authorization", "error", err)
		os.Exit(1)
	}

	router := http.NewServeMux()

	// This endpoint is accessible by anyone and will always return "200 OK" to indicate the API is running
	router.Handle("/api/healthz", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			err = jsonResponse(w, "OK", http.StatusOK)
			if err != nil {
				slog.Error("error writing response", "error", err)
			}
		}))

	// This endpoint is only accessible with a valid authorization (in this case when "project.grant.member.read" permission is granted).
	// It will call ZITADEL to additionally get all permissions granted to the user in ZITADEL and return that.
	router.Handle("/api/permissions", mw.RequireAuthorization()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			authCtx := mw.Context(r.Context())
			slog.Info("user accessed permission check", "id", authCtx.UserID(), "username", authCtx.Username)

			// we use the callers on token to retrieve the permission on the ZITADEL API
			// this will only work if ZITADEL is contained in the tokens audience (e.g. a PAT will always do so)
			resp, err := c.AuthService().ListMyZitadelPermissions(client.AuthorizedUserCtx(r.Context()), &auth.ListMyZitadelPermissionsRequest{})
			if err != nil {
				slog.Error("error listing zitadel permissions", "error", err)
				return
			}

			if !isAllowed(w, resp) {
				return
			}

			err = jsonResponse(w, resp.Result, http.StatusOK)
			if err != nil {
				slog.Error("error writing response", "error", err)
			}
		})))

	lis := fmt.Sprintf(":%s", *port)
	slog.Info("server listening, press ctrl+c to stop", "addr", "http://localhost"+lis)
	err = http.ListenAndServe(lis, router)
	if !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server terminated", "error", err)
		os.Exit(1)
	}
}

func isAllowed(w http.ResponseWriter, resp *auth.ListMyZitadelPermissionsResponse) bool {
	if !slices.Contains(resp.Result, "project.grant.member.read") {
		err := jsonResponse(w, map[string]string{"error": "Forbidden: missing 'project.grant.member.read'"}, http.StatusForbidden)
		if err != nil {
			slog.Error("error writing error response", "error", err)
			return false
		}
		return false
	}
	return true
}

func jsonResponse(w http.ResponseWriter, resp any, status int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(resp)
}
