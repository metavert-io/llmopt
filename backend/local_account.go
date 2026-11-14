package main

// Local account mode: a copy of the app on its owner's own machine that works
// on one account's data in the same database as the hosted service, without
// LastSaaS and without signing in. Useful where the hosted service can't do
// something, such as fetching YouTube transcripts, which YouTube blocks from
// datacenter addresses but usually allows from a home connection.
//
// Every request acts as the configured account, with its tenant-scoped data
// and its saved API keys, so the server listens on a loopback address only;
// publish it to your own devices with something like `tailscale serve`. The
// hosted service owns the database: this mode runs no migrations and none of
// the background jobs (cleanup, provider health checks, screenshots).

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"llmopt/internal/saas"
)

type localAccount struct {
	userID   string
	tenantID string
	sm       *saas.Middleware
}

// localAccountFromEnv reads LLMOPT_LOCAL_USER_ID and LLMOPT_LOCAL_TENANT_ID.
// Both or neither must be set.
func localAccountFromEnv() (userID, tenantID string, enabled bool, err error) {
	userID, tenantID = os.Getenv("LLMOPT_LOCAL_USER_ID"), os.Getenv("LLMOPT_LOCAL_TENANT_ID")
	switch {
	case userID == "" && tenantID == "":
		return "", "", false, nil
	case userID == "" || tenantID == "":
		return "", "", false, fmt.Errorf("set both LLMOPT_LOCAL_USER_ID and LLMOPT_LOCAL_TENANT_ID for local account mode")
	}
	return userID, tenantID, true, nil
}

// check confirms the account exists and the user belongs to the tenant.
func (la *localAccount) check(ctx context.Context) (*saas.AuthInfo, error) {
	return la.sm.ResolveAuthInfo(ctx, la.userID, la.tenantID, "local")
}

// wrap runs h as the local account. The account is loaded on every request,
// so plan and membership changes made on the hosted service apply here too.
func (la *localAccount) wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		info, err := la.check(r.Context())
		if err != nil {
			log.Printf("Local account mode: %v", err)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"the local account could not be loaded"}`, http.StatusServiceUnavailable)
			return
		}
		h(w, r.WithContext(saas.SetAuthContext(r.Context(), info)))
	}
}

// listenAddr is where the server listens: LLMOPT_LISTEN_HOST (all interfaces
// when unset) and port. In local account mode the host defaults to 127.0.0.1
// and must be a loopback address, since nothing asks who is connecting.
func listenAddr(port string, local bool) (string, error) {
	host := os.Getenv("LLMOPT_LISTEN_HOST")
	if local {
		if host == "" {
			host = "127.0.0.1"
		}
		if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", fmt.Errorf("LLMOPT_LISTEN_HOST must be a loopback address in local account mode, not %q", host)
		}
	}
	return net.JoinHostPort(host, port), nil
}

// randomSecret makes a signing secret nobody knows, so that in local account
// mode, where LastSaaS doesn't issue tokens, no bearer token validates.
func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("generating a secret: %v", err)
	}
	return hex.EncodeToString(b)
}
