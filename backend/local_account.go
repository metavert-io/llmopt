package main

// Local account mode: a copy of the app on its owner's own machine that works
// on one account's data in the same database as the hosted service, without
// LastSaaS and without signing in. Useful where the hosted service can't do
// something, such as fetching YouTube transcripts, which YouTube blocks from
// datacenter addresses but usually allows from a home connection.
//
// Every request acts as the configured account, with its tenant-scoped data
// and its saved API keys, so the server listens on a loopback address only;
// publish it to your own devices with something like `tailscale serve`, and
// list that address in LLMOPT_LOCAL_ORIGINS. Only the copy's own pages can use
// it: requests from other sites (another tab in the same browser) and requests
// addressed to any other host name (DNS rebinding) are refused. The
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
	"net/url"
	"os"
	"strings"

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

// guard lets only the local copy's own pages reach it. A browser on a device
// that can reach the copy would otherwise let any web page call it, and every
// call acts as the account. It checks:
//   - Host: loopback on this port, or a host from LLMOPT_LOCAL_ORIGINS (the
//     address the copy is published at, e.g. https://my-mac.example.ts.net:8448),
//     which stops DNS rebinding;
//   - Origin, when the browser sends one: the same list;
//   - Sec-Fetch-Site: never "cross-site".
//
// It sends no CORS headers, so other origins can't read responses either.
func (la *localAccount) guard(next http.Handler, port string) http.Handler {
	origins, hosts := map[string]bool{}, map[string]bool{}
	for _, h := range []string{"127.0.0.1", "localhost", "[::1]"} {
		origins["http://"+h+":"+port] = true
		hosts[h+":"+port] = true
	}
	for _, o := range strings.Split(os.Getenv("LLMOPT_LOCAL_ORIGINS"), ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			log.Printf("Local account mode: ignoring LLMOPT_LOCAL_ORIGINS entry %q (want scheme://host[:port])", o)
			continue
		}
		origins[u.Scheme+"://"+u.Host] = true
		hosts[u.Host] = true
	}
	refuse := func(w http.ResponseWriter, why string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":%q}`, "this local copy only answers its own pages ("+why+")")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case !hosts[r.Host]:
			refuse(w, "unknown host; add the address you open it at to LLMOPT_LOCAL_ORIGINS")
		case r.Header.Get("Origin") != "" && !origins[r.Header.Get("Origin")]:
			refuse(w, "request from another site")
		case r.Header.Get("Sec-Fetch-Site") == "cross-site":
			refuse(w, "request from another site")
		default:
			next.ServeHTTP(w, r)
		}
	})
}
