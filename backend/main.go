package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"llmopt/internal/mcpserver"
	"llmopt/internal/ratelimit"
	"llmopt/internal/saas"

	"golang.org/x/sync/errgroup"
)

// normalizeDomain strips protocol, trailing slashes, and lowercases.
// "https://Anthropic.AI/" → "anthropic.ai"
func normalizeDomain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimRight(s, "/")
	return s
}

func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip surrounding quotes
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		// Don't overwrite explicitly set env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func main() {
	// Load .env from project root (one dir up from backend/) then current dir as fallback
	loadEnvFile("../.env")
	loadEnvFile(".env")

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required. Copy .env.dev.example to .env and fill in your key.")
	}

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("MONGODB_URI environment variable is required.")
	}

	ytKey := os.Getenv("YOUTUBE_API_KEY") // optional — video tab disabled if missing

	// Encryption key for API key storage (required in SaaS mode)
	var encryptionKey []byte
	if encKeyHex := os.Getenv("LLMOPT_ENCRYPTION_KEY"); encKeyHex != "" {
		var err error
		encryptionKey, err = parseEncryptionKey(encKeyHex)
		if err != nil {
			log.Fatalf("Invalid LLMOPT_ENCRYPTION_KEY: %v", err)
		}
		log.Println("API key encryption enabled")
	}

	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = "llmopt"
	}

	mongoDB, err := NewMongoDB(mongoURI, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoDB.Close(context.Background())
	log.Printf("Connected to MongoDB (database: %s)", dbName)

	// One-time migrations
	mongoDB.migrateDomains()   // normalize domain fields (strip protocol)
	mongoDB.migrateIndexes()   // drop old {domain:1} unique indexes for multi-tenant
	seedEventDefinitions(mongoDB)
	if ytKey != "" {
		log.Println("YouTube API key configured — Video Authority enabled (system key)")
	}

	// SaaS mode: multi-tenant auth via shared JWT with LastSaaS
	saasEnabled := os.Getenv("LLMOPT_SAAS_ENABLED") == "true"
	var sm *saas.Middleware
	if saasEnabled {
		jwtSecret := os.Getenv("LLMOPT_JWT_ACCESS_SECRET")
		if jwtSecret == "" {
			log.Fatal("LLMOPT_JWT_ACCESS_SECRET is required when LLMOPT_SAAS_ENABLED=true")
		}
		if encryptionKey == nil {
			log.Fatal("LLMOPT_ENCRYPTION_KEY is required when LLMOPT_SAAS_ENABLED=true (for API key encryption)")
		}
		jv := saas.NewJWTValidator(jwtSecret)
		sm = saas.NewMiddleware(jv, mongoDB.Database)
		log.Println("SaaS mode enabled — JWT auth active")

		// One-time migration: assign root tenant to existing data
		mongoDB.migrateTenantIDs()

		// One-time migration: convert per-record public flags to domain shares
		mongoDB.migratePublicToDomainShares()
	}

	// Distributed rate limiter (MongoDB-backed with in-memory fallback)
	rl := ratelimit.NewDistributedRateLimiter(mongoDB.Database)
	defer rl.Stop()
	log.Println("Rate limiter initialized")

	// Capture missing screenshots for popular brands (non-blocking)
	go ensurePopularScreenshots(mongoDB)

	// Background cleanup: prune old data every 6 hours
	go runCleanupJobs(mongoDB)

	// Global health checks: probe all LLM providers every 5 minutes for the timeline
	go runGlobalHealthChecks(mongoDB, apiKey, encryptionKey)

	// withAuth wraps a handler with JWT + tenant middleware (SaaS mode only).
	// In non-SaaS mode, returns the handler unwrapped.
	withAuth := func(h http.HandlerFunc) http.HandlerFunc {
		if sm == nil {
			return h
		}
		return func(w http.ResponseWriter, r *http.Request) {
			sm.RequireJWT(sm.RequireTenant(http.HandlerFunc(h))).ServeHTTP(w, r)
		}
	}

	// withRL wraps a handler with rate limiting by (client IP, request path).
	// Keying on path as well prevents one endpoint's counter from gating another's —
	// otherwise the most restrictive limit silently caps every endpoint per IP.
	withRL := func(cfg ratelimit.RateLimitConfig, h http.HandlerFunc) http.HandlerFunc {
		return rl.RateLimitHandler(cfg, func(r *http.Request) string {
			return ratelimit.GetClientIP(r) + "|" + r.URL.Path
		}, h)
	}

	mux := http.NewServeMux()

	// SaaS config endpoint — tells the frontend whether auth is required
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"saas_enabled": saasEnabled})
	})
	mux.HandleFunc("OPTIONS /api/config", handleOptions)

	// Bootstrap status — the SaaS frontend checks this to decide if /setup is needed.
	// The llmopt backend is always initialized (setup is handled by the LastSaaS CLI).
	mux.HandleFunc("GET /api/bootstrap/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"initialized": true})
	})
	mux.HandleFunc("OPTIONS /api/bootstrap/status", handleOptions)

	// Tenant-scoped routes (wrapped with auth in SaaS mode)
	mux.HandleFunc("POST /api/analyze", withAuth(withRL(ratelimit.AnalyzeLimit, handleAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("GET /api/analyses", withAuth(handleListAnalyses(mongoDB)))
	mux.HandleFunc("GET /api/analyses/{id}", withAuth(handleGetAnalysis(mongoDB)))
	mux.HandleFunc("DELETE /api/analyses/{id}", withAuth(handleDeleteAnalysis(mongoDB)))
	mux.HandleFunc("DELETE /api/optimizations/{id}", withAuth(handleDeleteOptimization(mongoDB)))
	mux.HandleFunc("POST /api/analyses/{id}/questions/{idx}/optimize", withAuth(withRL(ratelimit.OptimizeLimit, handleOptimize(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("GET /api/analyses/{id}/questions/{idx}/optimization", withAuth(handleGetOptimization(mongoDB)))
	mux.HandleFunc("GET /api/optimizations", withAuth(handleListOptimizations(mongoDB)))
	mux.HandleFunc("GET /api/optimizations/{id}", withAuth(handleGetOptimizationByID(mongoDB)))
	mux.HandleFunc("GET /api/domains/{domain}/share", withAuth(handleGetDomainShare(mongoDB)))
	mux.HandleFunc("PUT /api/domains/{domain}/share", withAuth(handleSetDomainShare(mongoDB)))
	mux.HandleFunc("GET /api/todos", withAuth(handleListTodos(mongoDB)))
	mux.HandleFunc("PATCH /api/todos/{id}", withAuth(handleUpdateTodo(mongoDB)))
	mux.HandleFunc("POST /api/todos/archive", withAuth(handleBulkArchiveTodos(mongoDB)))
	mux.HandleFunc("POST /api/domains/{domain}/summary", withAuth(withRL(ratelimit.SummaryLimit, handleGenerateDomainSummary(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("GET /api/domains/{domain}/summary", withAuth(handleGetDomainSummary(mongoDB)))
	mux.HandleFunc("GET /api/domains/{domain}/summary/status", withAuth(handleDomainSummaryStatus(mongoDB)))
	mux.HandleFunc("GET /api/brands", withAuth(handleListBrands(mongoDB)))
	mux.HandleFunc("GET /api/brands/{domain}", withAuth(handleGetBrand(mongoDB)))
	mux.HandleFunc("PUT /api/brands/{domain}", withAuth(handleSaveBrand(mongoDB)))
	mux.HandleFunc("DELETE /api/brands/{domain}", withAuth(handleDeleteBrand(mongoDB)))
	mux.HandleFunc("POST /api/brands/{domain}/discover-competitors", withAuth(withRL(ratelimit.BrandDiscoverLimit, handleDiscoverCompetitors(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("POST /api/brands/{domain}/suggest-queries", withAuth(withRL(ratelimit.BrandDiscoverLimit, handleSuggestQueries(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("POST /api/brands/{domain}/generate-description", withAuth(withRL(ratelimit.BrandDiscoverLimit, handleGenerateDescription(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("POST /api/brands/{domain}/predict-audience", withAuth(withRL(ratelimit.BrandDiscoverLimit, handlePredictAudience(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("POST /api/brands/{domain}/suggest-claims", withAuth(withRL(ratelimit.BrandDiscoverLimit, handleSuggestClaims(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("POST /api/brands/{domain}/predict-differentiators", withAuth(withRL(ratelimit.BrandDiscoverLimit, handlePredictDifferentiators(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("POST /api/video/discover", withAuth(withRL(ratelimit.VideoDiscoverLimit, handleVideoDiscover(mongoDB, encryptionKey, ytKey, saasEnabled))))
	mux.HandleFunc("POST /api/video/analyze", withAuth(withRL(ratelimit.VideoAnalyzeLimit, handleVideoAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled, ytKey))))
	mux.HandleFunc("GET /api/video/analyses/{domain}/details", withAuth(handleGetVideoDetails(mongoDB)))
	mux.HandleFunc("GET /api/video/analyses/{domain}", withAuth(handleGetVideoAnalysis(mongoDB)))
	mux.HandleFunc("GET /api/video/analyses", withAuth(handleListVideoAnalyses(mongoDB)))
	mux.HandleFunc("DELETE /api/video/analyses/{domain}", withAuth(handleDeleteVideoAnalysis(mongoDB)))
	mux.HandleFunc("POST /api/video/search-terms", withAuth(handleVideoSearchTerms(mongoDB)))

	// Reddit Authority Analyzer
	mux.HandleFunc("POST /api/reddit/discover", withAuth(withRL(ratelimit.RedditDiscoverLimit, handleRedditDiscover(mongoDB))))
	mux.HandleFunc("POST /api/reddit/analyze", withAuth(withRL(ratelimit.RedditAnalyzeLimit, handleRedditAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("GET /api/reddit/analyses/{domain}", withAuth(handleGetRedditAnalysis(mongoDB)))
	mux.HandleFunc("GET /api/reddit/analyses", withAuth(handleListRedditAnalyses(mongoDB)))
	mux.HandleFunc("DELETE /api/reddit/analyses/{domain}", withAuth(handleDeleteRedditAnalysis(mongoDB)))

	// Search Visibility Analyzer
	mux.HandleFunc("POST /api/search/analyze", withAuth(withRL(ratelimit.SearchAnalyzeLimit, handleSearchAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("GET /api/search/analyses/{domain}", withAuth(handleGetSearchAnalysis(mongoDB)))
	mux.HandleFunc("GET /api/search/analyses", withAuth(handleListSearchAnalyses(mongoDB)))
	mux.HandleFunc("DELETE /api/search/analyses/{domain}", withAuth(handleDeleteSearchAnalysis(mongoDB)))

	// Failed Analyses
	mux.HandleFunc("GET /api/failed-analyses", withAuth(handleListFailedAnalyses(mongoDB)))
	mux.HandleFunc("DELETE /api/failed-analyses/{id}", withAuth(handleDeleteFailedAnalysis(mongoDB)))

	// Visibility Score
	mux.HandleFunc("GET /api/visibility-score/{domain}", withAuth(handleVisibilityScore(mongoDB)))

	// LLM Test
	mux.HandleFunc("GET /api/providers/models", withAuth(handleListProviderModels(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("POST /api/test", withAuth(withRL(ratelimit.LLMTestLimit, handleLLMTest(mongoDB, encryptionKey, apiKey, saasEnabled))))
	mux.HandleFunc("GET /api/test/{domain}/history", withAuth(handleGetLLMTestHistory(mongoDB)))
	mux.HandleFunc("GET /api/test/{domain}/competitors", withAuth(handleGetCompetitorTests(mongoDB)))
	mux.HandleFunc("GET /api/test/{domain}/queries", withAuth(handleGetTestQueries(mongoDB)))
	mux.HandleFunc("PUT /api/test/{domain}/queries", withAuth(handlePutTestQueries(mongoDB)))
	mux.HandleFunc("GET /api/test/{domain}", withAuth(handleGetLLMTest(mongoDB)))
	mux.HandleFunc("DELETE /api/test/{domain}", withAuth(handleDeleteLLMTest(mongoDB)))
	mux.HandleFunc("POST /api/test/generate-queries", withAuth(withRL(ratelimit.BrandDiscoverLimit, handleGenerateTestQueries(mongoDB))))

	// PDF Report
	mux.HandleFunc("POST /api/domains/{domain}/report/pdf", withAuth(withRL(ratelimit.PDFGenerateLimit, handleGeneratePDF(mongoDB))))
	mux.HandleFunc("GET /api/domains/{domain}/report/pdf/{id}", withAuth(handleServePDF(mongoDB)))

	// User Access Keys (lok_ keys for MCP server access)
	mux.HandleFunc("GET /api/user/access-keys", withAuth(handleListUserAccessKeys(mongoDB)))
	mux.HandleFunc("POST /api/user/access-keys", withAuth(handleCreateUserAccessKey(mongoDB)))
	mux.HandleFunc("DELETE /api/user/access-keys/{id}", withAuth(handleDeleteUserAccessKey(mongoDB)))

	// API Key Management
	mux.HandleFunc("GET /api/settings/api-keys", withAuth(handleListAPIKeys(mongoDB)))
	mux.HandleFunc("PUT /api/settings/api-keys/{provider}", withAuth(handleSetAPIKey(mongoDB, encryptionKey)))
	mux.HandleFunc("DELETE /api/settings/api-keys/{provider}", withAuth(handleDeleteAPIKey(mongoDB)))
	mux.HandleFunc("POST /api/settings/api-keys/{provider}/verify", withAuth(handleVerifyAPIKey(mongoDB, encryptionKey)))
	mux.HandleFunc("GET /api/settings/api-keys/status", withAuth(handleAPIKeyStatus(mongoDB)))
	mux.HandleFunc("GET /api/settings/primary-provider", withAuth(handleGetPrimaryProvider(mongoDB)))
	mux.HandleFunc("PUT /api/settings/primary-provider", withAuth(handleSetPrimaryProvider(mongoDB)))
	mux.HandleFunc("GET /api/health/providers", withAuth(handleProviderHealthCheck(mongoDB, encryptionKey)))

	// ─── Public API v1 ────────────────────────────────────────────────────
	withAPIAuth := func(h http.HandlerFunc) http.HandlerFunc {
		if sm == nil {
			return h
		}
		return func(w http.ResponseWriter, r *http.Request) {
			sm.RequireAuth(http.HandlerFunc(h)).ServeHTTP(w, r)
		}
	}
	withRole := func(roles []string, h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			role := saas.MemberRoleFromContext(r.Context())
			for _, allowed := range roles {
				if role == allowed {
					h(w, r)
					return
				}
			}
			http.Error(w, `{"error":"insufficient permissions","code":"FORBIDDEN"}`, http.StatusForbidden)
		}
	}

	// Read-only endpoints
	mux.HandleFunc("GET /api/v1/domains", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1ListDomains(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/analysis", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetAnalysis(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/optimizations", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetOptimizations(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/video", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetVideo(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/reddit", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetReddit(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/search", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetSearch(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/summary", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetSummary(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/tests", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetTests(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/score", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetScore(mongoDB))))
	mux.HandleFunc("GET /api/v1/domains/{domain}/brand", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetBrand(mongoDB))))

	// Todo endpoints
	mux.HandleFunc("GET /api/v1/todos", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1ListTodos(mongoDB))))
	mux.HandleFunc("GET /api/v1/todos/{id}", withAPIAuth(withRL(ratelimit.APIReadLimit, handleAPIv1GetTodo(mongoDB))))
	mux.HandleFunc("PATCH /api/v1/todos/{id}", withAPIAuth(withRL(ratelimit.APIWriteLimit, withRole([]string{"admin", "owner"}, handleAPIv1UpdateTodo(mongoDB)))))
	mux.HandleFunc("POST /api/v1/todos/bulk-update", withAPIAuth(withRL(ratelimit.APIWriteLimit, withRole([]string{"admin", "owner"}, handleAPIv1BulkUpdateTodos(mongoDB)))))

	// API Documentation (no auth)
	mux.HandleFunc("GET /api/v1/docs", handleAPIv1Docs())

	// OPTIONS for v1 routes
	for _, p := range []string{
		"/api/v1/domains", "/api/v1/domains/{domain}/analysis", "/api/v1/domains/{domain}/optimizations",
		"/api/v1/domains/{domain}/video", "/api/v1/domains/{domain}/reddit", "/api/v1/domains/{domain}/search",
		"/api/v1/domains/{domain}/summary", "/api/v1/domains/{domain}/tests", "/api/v1/domains/{domain}/score",
		"/api/v1/domains/{domain}/brand", "/api/v1/todos", "/api/v1/todos/{id}", "/api/v1/todos/bulk-update",
		"/api/v1/docs",
	} {
		mux.HandleFunc("OPTIONS "+p, handleOptions)
	}

	// Public routes (no auth required)
	mux.HandleFunc("GET /api/health/claude", handleHealthCheck(apiKey, mongoDB))
	mux.HandleFunc("GET /api/health/history", handleHealthHistory(mongoDB))
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/share/popular", withRL(ratelimit.APIReadLimit, handleGetPopularDomains(mongoDB)))
	mux.HandleFunc("GET /api/share/popular/{domain}/screenshot", withRL(ratelimit.APIReadLimit, handleServeBrandScreenshot(mongoDB)))
	mux.HandleFunc("GET /api/share/{shareId}", withRL(ratelimit.APIReadLimit, handleGetSharedDomain(mongoDB)))
	mux.HandleFunc("GET /api/share/{shareId}/pdf", withRL(ratelimit.APIReadLimit, handleServeSharedPDF(mongoDB)))

	// OPTIONS handlers (CORS preflight — no auth)
	// Note: OPTIONS /api/config already registered above with the GET handler
	mux.HandleFunc("OPTIONS /api/health/claude", handleOptions)
	mux.HandleFunc("OPTIONS /api/health/providers", handleOptions)
	mux.HandleFunc("OPTIONS /api/health", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses/{id}/questions/{idx}/optimize", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses/{id}/questions/{idx}/optimization", handleOptions)
	mux.HandleFunc("OPTIONS /api/optimizations", handleOptions)
	mux.HandleFunc("OPTIONS /api/optimizations/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/share", handleOptions)
	mux.HandleFunc("OPTIONS /api/share/popular", handleOptions)
	mux.HandleFunc("OPTIONS /api/share/popular/{domain}/screenshot", handleOptions)
	mux.HandleFunc("OPTIONS /api/share/{shareId}", handleOptions)
	mux.HandleFunc("OPTIONS /api/share/{shareId}/pdf", handleOptions)
	mux.HandleFunc("OPTIONS /api/todos", handleOptions)
	mux.HandleFunc("OPTIONS /api/todos/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/health/history", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/summary", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/summary/status", handleOptions)
	mux.HandleFunc("PATCH /api/brands/{domain}/subreddits", withAuth(handlePatchBrandSubreddits(mongoDB)))
	mux.HandleFunc("OPTIONS /api/brands/{domain}/subreddits", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/discover-competitors", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/suggest-queries", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/generate-description", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/predict-audience", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/suggest-claims", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/predict-differentiators", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/discover", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyses/{domain}/details", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyses/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/todos/archive", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/discover", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/analyses/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/search/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/search/analyses/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/search/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/failed-analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/failed-analyses/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/report/pdf", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/report/pdf/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/visibility-score/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/providers/models", handleOptions)
	mux.HandleFunc("OPTIONS /api/test", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/{domain}/history", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/{domain}/competitors", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/{domain}/queries", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/generate-queries", handleOptions)
	mux.HandleFunc("OPTIONS /api/user/access-keys", handleOptions)
	mux.HandleFunc("OPTIONS /api/user/access-keys/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys/{provider}", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys/{provider}/verify", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys/status", handleOptions)

	// ─── MCP Server ──────────────────────────────────────────────────────
	mcpJWTSecret := os.Getenv("MCP_JWT_SECRET")
	if mcpJWTSecret == "" && len(encryptionKey) > 0 {
		mcpJWTSecret = string(encryptionKey)
	}
	if mcpJWTSecret == "" {
		mcpJWTSecret = "llmopt-mcp-default-secret"
	}
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://llmopt.metavert.io"
	}

	var oauthSrv *mcpserver.OAuthServer
	if sm != nil {
		oauthSrv = mcpserver.NewOAuthServer(sm, mcpJWTSecret, baseURL, mongoDB.Database)
	}
	mcpHandler := mcpserver.New(sm, mongoDB.Database, oauthSrv, baseURL)

	// OAuth discovery + endpoints
	if oauthSrv != nil {
		mux.HandleFunc("GET /.well-known/oauth-protected-resource", oauthSrv.HandleProtectedResource)
		mux.HandleFunc("GET /.well-known/oauth-authorization-server", oauthSrv.HandleServerMetadata)
		mux.HandleFunc("POST /oauth/register", oauthSrv.HandleRegister)
		mux.HandleFunc("GET /oauth/authorize", oauthSrv.HandleAuthorize)
		mux.HandleFunc("POST /oauth/authorize", oauthSrv.HandleAuthorizeSubmit)
		mux.HandleFunc("POST /oauth/token", oauthSrv.HandleToken)
		mux.HandleFunc("OPTIONS /oauth/register", handleOptions)
		mux.HandleFunc("OPTIONS /oauth/authorize", handleOptions)
		mux.HandleFunc("OPTIONS /oauth/token", handleOptions)
	}

	// MCP endpoint (streamable HTTP)
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)
	log.Printf("MCP server enabled at /mcp")

	// SaaS frontend: serve LastSaaS auth/admin pages when configured
	if saasFrontendDir := os.Getenv("LLMOPT_FRONTEND_DIR"); saasFrontendDir != "" {
		if info, statErr := os.Stat(saasFrontendDir); statErr == nil && info.IsDir() {
			log.Printf("Serving SaaS frontend from %s", saasFrontendDir)
			saasSPA := &spaHandler{staticPath: saasFrontendDir, indexPath: "index.html"}
			// Auth pages at root level
			for _, p := range []string{"/login", "/signup", "/forgot-password", "/reset-password", "/verify-email", "/auth/callback", "/auth/mfa", "/auth/magic-link", "/setup"} {
				mux.Handle(p, saasSPA)
			}
			// Admin/team/settings under /last/ — StripPrefix so asset paths
			// (e.g. /last/assets/index-ABC.js) resolve correctly in the SPA dir
			mux.Handle("/last/", http.StripPrefix("/last", saasSPA))
		}
	}

	// SEO routes (no auth required)
	mux.HandleFunc("GET /robots.txt", handleRobotsTxt())
	mux.HandleFunc("GET /sitemap.xml", handleSitemap(mongoDB))
	mux.HandleFunc("GET /research", handleResearchPage())
	mux.HandleFunc("GET /research/", handleResearchPage())
	mux.HandleFunc("GET /docs", handleDocsPage())
	mux.HandleFunc("GET /docs/", handleDocsPage())
	mux.HandleFunc("GET /static/", handleStaticAssets())

	// Serve main frontend static files if available
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "../frontend/dist"
	}
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		log.Printf("Serving frontend from %s", staticDir)

		// Server-rendered share pages (before SPA catch-all)
		mux.HandleFunc("GET /share/{shareId}", handleSharePageSSR(mongoDB, staticDir))
		mux.HandleFunc("GET /share/{shareId}/brand", handleShareBrandPage(mongoDB))
		mux.HandleFunc("GET /share/{shareId}/video", handleShareVideoPage(mongoDB))
		mux.HandleFunc("GET /share/{shareId}/reddit", handleShareRedditPage(mongoDB))
		mux.HandleFunc("GET /share/{shareId}/search", handleShareSearchPage(mongoDB))
		mux.HandleFunc("GET /share/{shareId}/todos", handleShareTodosPage(mongoDB))

		// Server-rendered homepage for unauthenticated visitors
		mux.HandleFunc("GET /{$}", handleHomePage(mongoDB, staticDir))

		// SPA catch-all for authenticated app routes
		mux.Handle("/", &spaHandler{staticPath: staticDir, indexPath: "index.html"})
	} else {
		log.Printf("No frontend directory at %s, API-only mode", staticDir)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("LLM Optimizer backend starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, startIdleShutdown(withCORS(mux))))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")
		next.ServeHTTP(w, r)
	})
}

// tenantFilter appends a tenantId filter element when running in SaaS mode.
// In non-SaaS mode (no tenant in context), returns the filter unchanged.
func tenantFilter(ctx context.Context, filter bson.D) bson.D {
	if tid := saas.TenantIDFromContext(ctx); tid != "" {
		return append(filter, bson.E{Key: "tenantId", Value: tid})
	}
	return filter
}

func handleOptions(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// generateShareID returns a 12-character base62 string using crypto/rand.
func generateShareID() string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 12)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// isShareAdmin checks if the current user is an owner or admin of their tenant.
func isShareAdmin(ctx context.Context) bool {
	role := saas.MemberRoleFromContext(ctx)
	return role == "owner" || role == "admin"
}

// isRootShareAdmin checks if the user is an owner/admin of the root tenant.
func isRootShareAdmin(ctx context.Context) bool {
	if !isShareAdmin(ctx) {
		return false
	}
	tenant := saas.TenantFromContext(ctx)
	return tenant != nil && tenant.IsRoot
}

// spaHandler serves static files with SPA fallback to index.html.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.staticPath, filepath.Clean(r.URL.Path))

	fi, err := os.Stat(path)
	if os.IsNotExist(err) || (err == nil && fi.IsDir()) {
		// Never cache index.html — ensures new JS bundles are picked up
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Hashed assets (Vite bundles) can be cached long-term
	if strings.Contains(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

// sseWriter wraps sendSSE with a mutex for concurrent goroutine safety.
var sseMu sync.Mutex

func sendSSE(w http.ResponseWriter, f http.Flusher, eventType string, data any) {
	sseMu.Lock()
	defer sseMu.Unlock()
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	f.Flush()
}

// startSSEHeartbeat emits a periodic SSE comment (": ping") to keep the
// connection alive through long, silent server-side work such as blocking LLM
// calls. Without it, intermediaries on the production path (Fly edge proxy →
// Caddy reverse proxy) close idle streaming connections, which surfaces in the
// browser as a fetch "Load failed"/"network error" mid-test. Comment lines are
// ignored by the frontend SSE parser, so they are invisible to clients. The
// returned stop function must be deferred; it blocks until the heartbeat
// goroutine has fully exited so it can never race with the handler's final
// write or the connection teardown.
func startSSEHeartbeat(w http.ResponseWriter, f http.Flusher) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				sseMu.Lock()
				fmt.Fprint(w, ": ping\n\n")
				f.Flush()
				sseMu.Unlock()
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

// saveAndSendDone parses the result, saves to MongoDB, and sends the done SSE event.
func saveAndSendDone(w http.ResponseWriter, flusher http.Flusher, ctx context.Context, mongoDB *MongoDB, domain string, rawText string, resultJSON string, model string, brandInfo BrandContextInfo) {
	resultJSON = stripJSONFencing(resultJSON)
	var analysisResult AnalysisResult
	if err := json.Unmarshal([]byte(resultJSON), &analysisResult); err == nil {
		analysis := Analysis{
			Domain:                domain,
			TenantID:              saas.TenantIDFromContext(ctx),
			RawText:               rawText,
			Result:                analysisResult,
			Model:                 model,
			BrandContextUsed:      brandInfo.Used,
			BrandProfileUpdatedAt: brandInfo.ProfileUpdatedAt,
			CreatedAt:             time.Now(),
		}
		insertResult, insertErr := mongoDB.Analyses().InsertOne(ctx, analysis)
		var savedID string
		if insertErr != nil {
			log.Printf("Failed to save analysis: %v", insertErr)
		} else if oid, ok := insertResult.InsertedID.(primitive.ObjectID); ok {
			savedID = oid.Hex()
		}
		sendSSE(w, flusher, "done", map[string]any{
			"result":                   resultJSON,
			"id":                       savedID,
			"cached":                   false,
			"model":                    model,
			"created_at":               analysis.CreatedAt,
			"brand_context_used":       brandInfo.Used,
			"brand_profile_updated_at": brandInfo.ProfileUpdatedAt,
		})
	} else {
		log.Printf("Failed to parse analysis result for saving: %v", err)
		sendSSE(w, flusher, "done", map[string]string{"result": resultJSON})
	}
}

// errOverloaded is kept as an alias for backward compatibility within this file.
var errOverloaded = ErrOverloaded

// Error code constants for classifying analysis failures.
const (
	ErrCodeAPIOverloaded = "api_overloaded"
	ErrCodeAPIError      = "api_error"
	ErrCodeAPIKeyInvalid = "api_key_invalid"
	ErrCodeParseError    = "parse_error"
	ErrCodeCancelled     = "cancelled"
	ErrCodeStreamStalled = "stream_stalled"
)

// classifyError maps an error to one of the error code constants.
func classifyError(err error) string {
	if err == nil {
		return ErrCodeAPIError
	}
	if errors.Is(err, ErrOverloaded) {
		return ErrCodeAPIOverloaded
	}
	if errors.Is(err, ErrStreamStalled) {
		return ErrCodeStreamStalled
	}
	msg := err.Error()
	if strings.Contains(msg, "context canceled") || strings.Contains(msg, "request cancelled") {
		return ErrCodeCancelled
	}
	if strings.Contains(msg, "Claude API error (401)") || strings.Contains(msg, "Claude API error (403)") {
		return ErrCodeAPIKeyInvalid
	}
	return ErrCodeAPIError
}

// userFriendlyError returns a user-facing message for an error code.
func userFriendlyError(code string) string {
	switch code {
	case ErrCodeAPIOverloaded:
		return "The AI service is temporarily overloaded. This is an upstream provider issue. Please try again in a few minutes."
	case ErrCodeAPIError:
		return "The AI service returned an error. This is an upstream provider issue, not a problem with your account."
	case ErrCodeAPIKeyInvalid:
		return "Your API key was rejected by the provider. Please check your key in Settings."
	case ErrCodeCancelled:
		return "Analysis was cancelled."
	case ErrCodeStreamStalled:
		return "The AI service stopped responding mid-analysis. This is an upstream provider issue. Please try again."
	case ErrCodeParseError:
		return "The AI returned a response we couldn't parse. Please try again."
	default:
		return "An unexpected error occurred. Please try again."
	}
}

// sendSSEError sends an SSE error event with structured fields including
// an error code and upstream flag so the frontend can display appropriate messaging.
func sendSSEError(w http.ResponseWriter, f http.Flusher, code string) {
	upstream := code == ErrCodeAPIOverloaded || code == ErrCodeAPIError || code == ErrCodeStreamStalled
	sendSSE(w, f, "error", map[string]any{
		"message":  userFriendlyError(code),
		"code":     code,
		"upstream": upstream,
	})
}

// saveFailedAnalysis records a failed analysis attempt to the failed_analyses collection.
func saveFailedAnalysis(mongoDB *MongoDB, reqCtx context.Context, domain, feedType, errorCode, model string) {
	record := FailedAnalysis{
		TenantID:     saas.TenantIDFromContext(reqCtx),
		Domain:       domain,
		FeedType:     feedType,
		ErrorCode:    errorCode,
		ErrorMessage: userFriendlyError(errorCode),
		Model:        model,
		FailedAt:     time.Now(),
	}
	// Use Background context since the request context may already be cancelled.
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := mongoDB.FailedAnalyses().InsertOne(saveCtx, record); err != nil {
		log.Printf("Failed to save error record for %s/%s: %v", feedType, domain, err)
	}
}

// stripJSONFencing removes markdown code fences (```json ... ```) that Claude
// sometimes wraps around JSON responses.
func stripJSONFencing(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}


// assessVideo calls the provider's small model to assess a single video's transcript for LLM authority signals.
func assessVideo(ctx context.Context, provider LLMProvider, apiKey string, video YouTubeVideo, domain string, searchTerms []string) (*VideoAssessment, error) {
	if video.Transcript == "" {
		return nil, nil
	}

	prompt := fmt.Sprintf(`Assess this YouTube video transcript for LLM authority signals.

Brand/Domain: %s
Target Search Terms: %s
Video: %s by %s
Views: %d | Published: %s

Transcript:
%s

Assess:
1. keyword_alignment (0-100): How well do target search terms appear naturally in spoken words?
2. quotability (0-100): Are there standalone, citable statements an LLM could extract?
3. info_density (0-100): Focused expert content vs. filler/rambling?
4. key_quotes: 2-3 most citable sentences (exact from transcript)
5. topics: Main topics covered (3-5 items)
6. brand_sentiment: How is %s discussed? (positive/negative/neutral/none)
7. summary: 1-2 sentence summary of what an LLM would extract from this video

Return ONLY valid JSON: {"keyword_alignment":N,"quotability":N,"info_density":N,"key_quotes":["..."],"topics":["..."],"brand_sentiment":"...","summary":"..."}`,
		domain,
		strings.Join(searchTerms, ", "),
		video.Title,
		video.ChannelTitle,
		video.ViewCount,
		video.PublishedAt.Format("2006-01-02"),
		video.Transcript,
		domain,
	)

	const maxRetries = 3
	backoff := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Check if the parent context (handler-level) is done before retrying
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			backoff *= 2
		}

		text, err := provider.Call(ctx, apiKey, provider.SmallModel(), prompt, 1024)
		if err != nil {
			// Retry on overloaded or per-call timeout (but not parent context cancellation)
			retryable := errors.Is(err, ErrOverloaded) || (strings.Contains(err.Error(), "context deadline exceeded") && ctx.Err() == nil)
			if retryable && attempt < maxRetries {
				log.Printf("Assessment retry %d/%d for %s: %v", attempt+1, maxRetries, video.VideoID, err)
				continue
			}
			return nil, err
		}

		// Extract JSON from response (may have markdown wrapping)
		text = strings.TrimSpace(text)
		if idx := strings.Index(text, "{"); idx >= 0 {
			if end := strings.LastIndex(text, "}"); end > idx {
				text = text[idx : end+1]
			}
		}

		var a VideoAssessment
		if err := json.Unmarshal([]byte(text), &a); err != nil {
			return nil, fmt.Errorf("failed to parse assessment: %w", err)
		}
		a.VideoID = video.VideoID
		a.Title = video.Title
		a.HasTranscript = true
		return &a, nil
	}
	return nil, fmt.Errorf("exhausted retries")
}

// assessVideos runs Phase 1: concurrent per-video assessments with the provider's small model.
// Returns a map of videoID -> assessment. Nil values mean no transcript or assessment failed.
func assessVideos(ctx context.Context, provider LLMProvider, apiKey string, videos []YouTubeVideo, domain string, searchTerms []string, mongoDB *MongoDB, w http.ResponseWriter, flusher http.Flusher) map[string]*VideoAssessment {
	results := make(map[string]*VideoAssessment)
	var mu sync.Mutex

	type assessResult struct {
		videoID    string
		assessment *VideoAssessment
		fromCache  bool
		err        error
	}

	resultsCh := make(chan assessResult, len(videos))
	sem := make(chan struct{}, 8) // 8 concurrent assessments

	for _, v := range videos {
		go func(video YouTubeVideo) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check cache first
			if cached, ok := cachedVideoAssessment(mongoDB, video.VideoID, domain, searchTerms); ok {
				resultsCh <- assessResult{videoID: video.VideoID, assessment: cached, fromCache: true}
				return
			}

			// Skip videos without transcripts
			if video.Transcript == "" {
				resultsCh <- assessResult{videoID: video.VideoID}
				return
			}

			a, err := assessVideo(ctx, provider, apiKey, video, domain, searchTerms)
			if err != nil {
				log.Printf("Warning: assessment failed for %s: %v", video.VideoID, err)
				resultsCh <- assessResult{videoID: video.VideoID, err: err}
				return
			}

			// Cache the result
			if a != nil {
				setCachedVideoAssessment(mongoDB, video.VideoID, domain, searchTerms, a)
			}
			resultsCh <- assessResult{videoID: video.VideoID, assessment: a}
		}(v)
	}

	cachedCount, assessedCount, skippedCount, failedCount := 0, 0, 0, 0
	for i := 0; i < len(videos); i++ {
		r := <-resultsCh
		mu.Lock()
		results[r.videoID] = r.assessment
		if r.fromCache {
			cachedCount++
		} else if r.assessment != nil {
			assessedCount++
		} else if r.err != nil {
			failedCount++
		} else {
			skippedCount++
		}
		mu.Unlock()

		sendSSE(w, flusher, "progress", map[string]string{
			"message": fmt.Sprintf("Assessing transcripts (%d/%d)... [%d cached, %d assessed, %d skipped, %d failed]",
				i+1, len(videos), cachedCount, assessedCount, skippedCount, failedCount),
		})
	}

	return results
}

func handleAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		var req struct {
			URL   string `json:"url"`
			Force bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}
		if req.URL == "" {
			sendSSE(w, flusher, "error", map[string]string{"message": "URL is required"})
			return
		}
		req.URL = normalizeDomain(req.URL)

		// Check for cached analysis (< 30 days old) unless force refresh
		if !req.Force {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			var cached Analysis
			cacheFilter := tenantFilter(r.Context(), bson.D{
				{Key: "domain", Value: req.URL},
				{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: time.Now().AddDate(0, 0, -30)}}},
			})
			err := mongoDB.Analyses().FindOne(ctx, cacheFilter, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&cached)
			cancel()

			if err == nil {
				sendSSE(w, flusher, "status", map[string]string{
					"message": "Found cached analysis...",
				})
				resultJSON, _ := json.Marshal(cached.Result)
				sendSSE(w, flusher, "done", map[string]any{
					"result":                   string(resultJSON),
					"id":                       cached.ID.Hex(),
					"cached":                   true,
					"model":                    cached.Model,
					"created_at":               cached.CreatedAt,
					"brand_context_used":       cached.BrandContextUsed,
					"brand_profile_updated_at": cached.BrandProfileUpdatedAt,
				})
				trackServerEvent(mongoDB, "custom.server.analyze.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": req.URL, "duration_ms": time.Since(startTime).Milliseconds(), "cached": true})
				return
			}
		}

		// Resolve primary LLM provider and API key for this tenant
		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Starting analysis of " + req.URL + "...",
		})

		brandInfo := lookupBrandContext(mongoDB, req.URL, saas.TenantIDFromContext(r.Context()))

		brandInstructions := ""
		if brandInfo.Used {
			brandInstructions = `

IMPORTANT — Brand Aspiration Cross-Reference:
The brand intelligence context below includes "Target Queries" and "Brand Claims". These represent aspirations — questions the brand WANTS people to ask about them, and claims they WANT to make — whether or not the site currently supports them.

After generating the questions you discover organically from the site, cross-reference every Target Query and Brand Claim against what you found:

1. If the site handles this well with strong, relevant content → include as a question with brand_status: "normal"
2. If the site mentions the topic but weakly, tangentially, or without dedicated content → brand_status: "aspirational"
3. If the site does NOT appear to address this at all → brand_status: "missing" with page_urls: []

Questions discovered organically (not from brand aspirations) should omit brand_status or use "normal".

## Discovery Questions — Category-First Audience Intent

In addition to questions discovered from the site and brand aspirations, generate 5-8 "discovery" questions. These represent what your target audience would search for in this CATEGORY without knowing this brand exists. Use the categories, target audience, and key use cases from the brand intelligence.

Discovery question patterns:
- "What is the best {category} for {use case}?"
- "How do I {key use case}?"
- "{Category} comparison {current year}"
- "Alternatives to {competitor}"

Mark these with brand_status: "discovery" and set page_urls to [] (these aren't derived from site content).

The JSON format for questions is:
{
  "question": "...",
  "relevance": "...",
  "category": "...",
  "page_urls": [...],
  "brand_status": "normal" | "aspirational" | "missing" | "discovery"
}`
		}

		prompt := fmt.Sprintf(`You are a website content analyzer. Your task is to understand what a website is about and determine what questions people would likely ask that this website can answer.

Website to analyze: %s

Please:
1. Search for and visit this website to understand its content, purpose, and offerings
2. Browse multiple pages on the site — the homepage, key subpages, product/service pages, about pages, blog posts, etc.
3. Analyze what the website provides — its products, services, information, etc.
4. Think about what questions a user might type into a search engine or AI assistant that this website would be well-suited to answer
5. Track which specific pages you visited and which pages are relevant to each question

Return your analysis as JSON in exactly this format (no markdown code fences, just raw JSON):
{
  "site_summary": "Brief description of what the website is about",
  "crawled_pages": [
    {
      "url": "Full URL of a page you visited",
      "title": "Page title or short description"
    }
  ],
  "questions": [
    {
      "question": "The question a user might ask",
      "relevance": "Why this website is relevant to this question",
      "category": "Category like Product, Pricing, How-to, Comparison, General, etc.",
      "page_urls": ["https://example.com/page1", "https://example.com/page2"]
    }
  ]
}

For crawled_pages, list every distinct page on the site you visited or found during your search.
For page_urls in each question, list the specific page URL(s) from the site that answer or are most relevant to that question. Use the actual URLs you found, not fabricated ones.

Generate %s diverse questions across different categories. Include questions at different levels of specificity — from broad queries to very specific ones.%s%s`, req.URL, func() string { if brandInfo.Used { return "20-28" }; return "15-20" }(), brandInstructions, brandInfo.ContextString)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						log.Printf("%s API (%s) overloaded, will retry (attempt %d/%d)", provider.Name(), model.ID, attempt+1, maxRetries)
						continue
					}
					break // exhausted retries, try next model
				}
				if err != nil {
					code := classifyError(err)
					sendSSEError(w, flusher, code)
					saveFailedAnalysis(mongoDB, r.Context(), req.URL, "site", code, model.Name)
					return
				}

				saveAndSendDone(w, flusher, r.Context(), mongoDB, req.URL, result.RawText, result.ResultJSON, model.Name, brandInfo)
				trackServerEvent(mongoDB, "custom.server.analyze.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": req.URL, "duration_ms": time.Since(startTime).Milliseconds(), "cached": false})
				return
			}

			log.Printf("%s API (%s) exhausted retries: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
		saveFailedAnalysis(mongoDB, r.Context(), req.URL, "site", ErrCodeAPIOverloaded, "")
	}
}

func handleListAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		sort := bson.D{{Key: "createdAt", Value: -1}}
		if domain := r.URL.Query().Get("domain"); domain != "" {
			filter = append(filter, bson.E{Key: "domain", Value: normalizeDomain(domain)})
			// When filtering by domain, sort brand-intel reports first, then by date
			sort = bson.D{{Key: "brandContextUsed", Value: -1}, {Key: "createdAt", Value: -1}}
		}

		opts := options.Find().
			SetSort(sort).
			SetLimit(50).
			SetProjection(bson.D{
				{Key: "domain", Value: 1},
				{Key: "createdAt", Value: 1},
				{Key: "model", Value: 1},
				{Key: "result.siteSummary", Value: 1},
				{Key: "result.questions", Value: 1},
				{Key: "result.crawledPages", Value: 1},
				{Key: "brandContextUsed", Value: 1},
				{Key: "brandProfileUpdatedAt", Value: 1},
			})

		cursor, err := mongoDB.Analyses().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var analyses []Analysis
		if err := cursor.All(ctx, &analyses); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		summaries := make([]AnalysisSummary, len(analyses))
		for i, a := range analyses {
			summaries[i] = AnalysisSummary{
				ID:                    a.ID,
				Domain:                a.Domain,
				SiteSummary:           a.Result.SiteSummary,
				QuestionCount:         len(a.Result.Questions),
				PageCount:             len(a.Result.CrawledPages),
				Model:                 a.Model,
				BrandContextUsed:      a.BrandContextUsed,
				BrandProfileUpdatedAt: a.BrandProfileUpdatedAt,
				CreatedAt:             a.CreatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleGetAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var analysis Analysis
		err = mongoDB.Analyses().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}})).Decode(&analysis)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

type modelCheckResult struct {
	Model      string `json:"model"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	LatencyMs  int64  `json:"latency_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

func handleHealthCheck(apiKey string, mongoDB *MongoDB) http.HandlerFunc {
	type modelDef struct {
		id, name string
	}
	models := []modelDef{
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-haiku-4-5-20251001", "Haiku 4.5"},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		results := make([]modelCheckResult, len(models))
		var wg sync.WaitGroup

		for i, model := range models {
			wg.Add(1)
			go func(idx int, m modelDef) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()

				body, _ := json.Marshal(map[string]any{
					"model":      m.id,
					"max_tokens": 10,
					"messages": []map[string]any{
						{"role": "user", "content": "Reply with just the word 'ok'."},
					},
				})

				httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
				if err != nil {
					results[idx] = modelCheckResult{Model: m.id, Name: m.name, Status: "error", Error: "Failed to create request"}
					return
				}
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("x-api-key", apiKey)
				httpReq.Header.Set("anthropic-version", "2023-06-01")

				start := time.Now()
				resp, err := llmHTTPClient.Do(httpReq)
				latency := time.Since(start)

				if err != nil {
					results[idx] = modelCheckResult{Model: m.id, Name: m.name, Status: "error", Error: err.Error()}
					return
				}
				defer resp.Body.Close()

				check := modelCheckResult{
					Model:      m.id,
					Name:       m.name,
					HTTPStatus: resp.StatusCode,
					LatencyMs:  latency.Milliseconds(),
				}

				switch {
				case resp.StatusCode == 200:
					check.Status = "available"
				case resp.StatusCode == 529:
					check.Status = "overloaded"
					errBody, _ := io.ReadAll(resp.Body)
					check.Error = string(errBody)
				default:
					check.Status = "error"
					errBody, _ := io.ReadAll(resp.Body)
					check.Error = string(errBody)
				}

				results[idx] = check
			}(i, model)
		}

		wg.Wait()

		checkedAt := time.Now()

		// NOTE: DB persistence removed — global health check goroutine now handles timeline data.

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"models":     results,
			"checked_at": checkedAt,
		})
	}
}

// runGlobalHealthChecks periodically probes each LLM provider using any available
// tenant key and stores the results in the health_checks collection for the timeline.
func runGlobalHealthChecks(mongoDB *MongoDB, systemAnthropicKey string, encKey []byte) {
	// Initial delay — let the server start up
	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	check := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Use root tenant's API keys for global health probes
		type checkItem struct {
			providerID string
			name       string
			verifyFunc func(ctx context.Context, key string) (string, error)
			apiKey     string
		}
		var toCheck []checkItem

		var rootTenant struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := mongoDB.Database.Collection("tenants").FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant); err != nil {
			log.Printf("Global health check: root tenant lookup failed: %v", err)
			return
		}
		rootTenantID := rootTenant.ID.Hex()

		// Load all root tenant keys at once
		keyCursor, err := mongoDB.TenantAPIKeys().Find(ctx, bson.M{"tenantId": rootTenantID})
		if err != nil {
			log.Printf("Global health check: failed to load root tenant keys: %v", err)
			return
		}
		var rootKeys []TenantAPIKey
		keyCursor.All(ctx, &rootKeys)
		keyCursor.Close(ctx)
		log.Printf("Global health check: found %d keys for root tenant %s", len(rootKeys), rootTenantID)

		rootKeyMap := map[string]TenantAPIKey{}
		for _, k := range rootKeys {
			rootKeyMap[k.Provider] = k
		}

		// LLM providers
		for id, p := range providers {
			var key string
			if id == "anthropic" && systemAnthropicKey != "" {
				key = systemAnthropicKey
			} else if doc, ok := rootKeyMap[id]; ok {
				plain, err := decryptSecret(doc.EncryptedKey, encKey)
				if err != nil {
					continue
				}
				key = plain
			} else {
				continue
			}
			pCopy := p // capture for closure
			toCheck = append(toCheck, checkItem{
				providerID: id,
				name:       p.Name(),
				verifyFunc: pCopy.VerifyKey,
				apiKey:     key,
			})
		}

		// YouTube (not an LLM provider but tracked for health)
		if doc, ok := rootKeyMap["youtube"]; ok {
			plain, err := decryptSecret(doc.EncryptedKey, encKey)
			if err == nil {
				toCheck = append(toCheck, checkItem{
					providerID: "youtube",
					name:       "YouTube",
					verifyFunc: func(ctx context.Context, key string) (string, error) {
						status := verifyYouTubeKey(ctx, key)
						return status, nil
					},
					apiKey: plain,
				})
			}
		}

		if len(toCheck) == 0 {
			log.Printf("Global health check: no providers to check (rootKeys=%d)", len(rootKeys))
			return
		}
		log.Printf("Global health check: probing %d providers", len(toCheck))

		// Probe each provider in parallel
		results := make([]ModelStatusRecord, len(toCheck))
		var wg sync.WaitGroup
		for i, ci := range toCheck {
			wg.Add(1)
			go func(idx int, ci checkItem) {
				defer wg.Done()
				checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
				defer checkCancel()

				start := time.Now()
				status, _ := ci.verifyFunc(checkCtx, ci.apiKey)
				latency := time.Since(start)

				rec := ModelStatusRecord{
					Model:     ci.providerID,
					Name:      ci.name,
					LatencyMs: latency.Milliseconds(),
				}
				switch status {
				case "active":
					rec.Status = "available"
					rec.HTTPStatus = 200
				case "invalid":
					rec.Status = "error"
				case "no_credits":
					rec.Status = "error"
				default:
					rec.Status = "error"
				}
				results[idx] = rec
			}(i, ci)
		}
		wg.Wait()

		// Store in health_checks collection
		record := HealthRecord{
			Models:    results,
			CheckedAt: time.Now(),
		}
		if _, err := mongoDB.HealthChecks().InsertOne(ctx, record); err != nil {
			log.Printf("Global health check: failed to save: %v", err)
		} else {
			names := make([]string, len(results))
			for i, r := range results {
				names[i] = fmt.Sprintf("%s=%s", r.Name, r.Status)
			}
			log.Printf("Global health check: saved [%s]", strings.Join(names, ", "))
		}
	}

	log.Println("Global health check: starting (30s delay)")
	check() // Run immediately
	for range ticker.C {
		check()
	}
}

// handleProviderHealthCheck checks all API keys the tenant has configured
// by calling VerifyKey on each provider. Returns per-provider health status.
func handleProviderHealthCheck(mongoDB *MongoDB, encKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()

		// Load all API keys for this tenant
		cursor, err := mongoDB.TenantAPIKeys().Find(ctx, bson.M{"tenantId": tenantID})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"models": []any{}, "checked_at": time.Now()})
			return
		}
		var keys []TenantAPIKey
		if err := cursor.All(ctx, &keys); err != nil || len(keys) == 0 {
			cursor.Close(ctx)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"models": []any{}, "checked_at": time.Now()})
			return
		}
		cursor.Close(ctx)

		// Check each provider in parallel
		type providerResult struct {
			Provider  string `json:"provider"`
			Model     string `json:"model"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			LatencyMs int64  `json:"latency_ms,omitempty"`
			Error     string `json:"error,omitempty"`
		}

		results := make([]providerResult, len(keys))
		var wg sync.WaitGroup

		for i, key := range keys {
			if key.Provider == "youtube" {
				ytStatus := "error"
				if key.Status == "active" {
					ytStatus = "available"
				}
				results[i] = providerResult{
					Provider: "youtube",
					Model:    "youtube-data-api",
					Name:     "YouTube Data API",
					Status:   ytStatus,
				}
				continue
			}

			provider := getProvider(key.Provider)
			if provider == nil {
				results[i] = providerResult{
					Provider: key.Provider,
					Model:    key.Provider,
					Name:     key.Provider,
					Status:   "error",
					Error:    "unknown provider",
				}
				continue
			}

			plainKey, err := decryptSecret(key.EncryptedKey, encKey)
			if err != nil {
				results[i] = providerResult{
					Provider: key.Provider,
					Model:    key.Provider,
					Name:     provider.Name(),
					Status:   "error",
					Error:    "decryption failed",
				}
				continue
			}

			wg.Add(1)
			go func(idx int, p LLMProvider, apiKey string, k TenantAPIKey) {
				defer wg.Done()

				checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
				defer checkCancel()

				start := time.Now()
				status, err := p.VerifyKey(checkCtx, apiKey)
				latency := time.Since(start)

				res := providerResult{
					Provider:  k.Provider,
					Model:     k.PreferredModel,
					Name:      p.Name(),
					LatencyMs: latency.Milliseconds(),
				}
				if res.Model == "" {
					// Use the first model as display name
					models := p.Models()
					if len(models) > 0 {
						res.Model = models[0].ID
					}
				}

				switch status {
				case "active":
					res.Status = "available"
				case "invalid":
					res.Status = "error"
					res.Error = "API key is invalid or revoked"
				case "no_credits":
					res.Status = "error"
					res.Error = "No credits / billing issue"
				default:
					res.Status = "error"
					if err != nil {
						res.Error = err.Error()
					}
				}

				results[idx] = res
			}(i, provider, plainKey, key)
		}

		wg.Wait()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"models":     results,
			"checked_at": time.Now(),
		})
	}
}

func handleHealthHistory(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Default: last 24 hours
		hours := 24
		if h := r.URL.Query().Get("hours"); h != "" {
			if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 && parsed <= 168 {
				hours = parsed
			}
		}

		since := time.Now().Add(-time.Duration(hours) * time.Hour)
		opts := options.Find().
			SetSort(bson.D{{Key: "checkedAt", Value: 1}}).
			SetLimit(500)

		cursor, err := mongoDB.HealthChecks().Find(ctx, bson.D{
			{Key: "checkedAt", Value: bson.D{{Key: "$gte", Value: since}}},
		}, opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var records []HealthRecord
		if err := cursor.All(ctx, &records); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if records == nil {
			records = []HealthRecord{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	}
}

// --- API Key Management Handlers ---

func handleListAPIKeys(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		cursor, err := mongoDB.TenantAPIKeys().Find(ctx, bson.M{"tenantId": tenantID})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var keys []TenantAPIKey
		if err := cursor.All(ctx, &keys); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if keys == nil {
			keys = []TenantAPIKey{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}
}

func handleSetAPIKey(mongoDB *MongoDB, encKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can manage API keys"}`, http.StatusForbidden)
			return
		}
		provider := r.PathValue("provider")
		validProviders := map[string]bool{"anthropic": true, "openai": true, "grok": true, "gemini": true, "youtube": true}
		if !validProviders[provider] {
			http.Error(w, `{"error":"invalid provider"}`, http.StatusBadRequest)
			return
		}

		var req struct {
			Key            string `json:"key"`
			PreferredModel string `json:"preferred_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		tenantID := saas.TenantIDFromContext(r.Context())

		// If no key provided, just update the preferred model on the existing key
		if req.Key == "" {
			if req.PreferredModel == "" {
				http.Error(w, `{"error":"key is required"}`, http.StatusBadRequest)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			filter := bson.M{"tenantId": tenantID, "provider": provider}
			res, err := mongoDB.TenantAPIKeys().UpdateOne(ctx, filter, bson.M{
				"$set": bson.M{"preferredModel": req.PreferredModel, "updatedAt": time.Now()},
			})
			if err != nil {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
				return
			}
			if res.MatchedCount == 0 {
				http.Error(w, `{"error":"no existing key for this provider"}`, http.StatusBadRequest)
				return
			}
			var updated TenantAPIKey
			mongoDB.TenantAPIKeys().FindOne(ctx, filter).Decode(&updated)
			updated.EncryptedKey = ""
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(updated)
			return
		}

		// Verify key with the provider
		status := "active"
		if provider == "youtube" {
			status = verifyYouTubeKey(r.Context(), req.Key)
		} else if p := getProvider(provider); p != nil {
			var err error
			status, err = p.VerifyKey(r.Context(), req.Key)
			if err != nil {
				log.Printf("API key verification error for tenant %s (%s): %v", tenantID, provider, err)
			}
		}

		// Encrypt the key
		encrypted, err := encryptSecret(req.Key, encKey)
		if err != nil {
			log.Printf("Failed to encrypt API key for tenant %s: %v", tenantID, err)
			http.Error(w, `{"error":"encryption failed"}`, http.StatusInternalServerError)
			return
		}

		// Build key prefix for display (first 8 chars or less)
		prefix := req.Key
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		prefix += "..."

		now := time.Now()
		doc := TenantAPIKey{
			TenantID:       tenantID,
			Provider:       provider,
			EncryptedKey:   encrypted,
			KeyPrefix:      prefix,
			PreferredModel: req.PreferredModel,
			Status:         status,
			LastVerifiedAt: &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := bson.M{"tenantId": tenantID, "provider": provider}
		update := bson.M{
			"$set": bson.M{
				"encryptedKey":   encrypted,
				"keyPrefix":      prefix,
				"preferredModel": req.PreferredModel,
				"status":         status,
				"lastVerifiedAt": now,
				"updatedAt":      now,
			},
			"$setOnInsert": bson.M{
				"tenantId":  tenantID,
				"provider":  provider,
				"createdAt": now,
			},
		}
		_, err = mongoDB.TenantAPIKeys().UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
		if err != nil {
			log.Printf("Failed to store API key for tenant %s: %v", tenantID, err)
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Return the result without the encrypted key
		doc.EncryptedKey = ""
		trackServerEvent(mongoDB, "custom.server.api_key.saved", saas.UserIDFromContext(r.Context()), tenantID, map[string]interface{}{"provider": provider})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}
}

func handleDeleteAPIKey(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can manage API keys"}`, http.StatusForbidden)
			return
		}
		provider := r.PathValue("provider")
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		result, err := mongoDB.TenantAPIKeys().DeleteOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": provider,
		})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"API key removed"}`))
	}
}

func handleVerifyAPIKey(mongoDB *MongoDB, encKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can manage API keys"}`, http.StatusForbidden)
			return
		}
		provider := r.PathValue("provider")
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		var doc TenantAPIKey
		err := mongoDB.TenantAPIKeys().FindOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": provider,
		}).Decode(&doc)
		if err != nil {
			http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
			return
		}

		plainKey, err := decryptSecret(doc.EncryptedKey, encKey)
		if err != nil {
			http.Error(w, `{"error":"decryption failed"}`, http.StatusInternalServerError)
			return
		}

		status := "active"
		if provider == "youtube" {
			status = verifyYouTubeKey(ctx, plainKey)
		} else if p := getProvider(provider); p != nil {
			status, err = p.VerifyKey(ctx, plainKey)
			if err != nil {
				log.Printf("API key re-verify error for tenant %s (%s): %v", tenantID, provider, err)
			}
		}

		now := time.Now()
		mongoDB.TenantAPIKeys().UpdateOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": provider,
		}, bson.M{"$set": bson.M{
			"status":         status,
			"lastVerifiedAt": now,
			"updatedAt":      now,
		}})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":          status,
			"last_verified_at": now,
		})
	}
}

// resolveOwnerName looks up the display name of the tenant's owner.
func resolveOwnerName(ctx context.Context, mongoDB *MongoDB, tenantID string) string {
	oid, err := primitive.ObjectIDFromHex(tenantID)
	if err != nil {
		return ""
	}

	// Find the owner membership
	var membership struct {
		UserID primitive.ObjectID `bson:"userId"`
	}
	err = mongoDB.Database.Collection("tenant_memberships").FindOne(ctx, bson.M{
		"tenantId": oid,
		"role":     "owner",
	}).Decode(&membership)
	if err != nil {
		return ""
	}

	// Look up the owner's display name
	var user struct {
		DisplayName string `bson:"displayName"`
	}
	err = mongoDB.Database.Collection("users").FindOne(ctx, bson.M{
		"_id": membership.UserID,
	}).Decode(&user)
	if err != nil {
		return ""
	}
	return user.DisplayName
}

func handleAPIKeyStatus(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := saas.TenantIDFromContext(r.Context())
		role := saas.MemberRoleFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Resolve owner name for non-owners
		ownerName := ""
		if role != "owner" {
			ownerName = resolveOwnerName(ctx, mongoDB, tenantID)
		}

		// Check if ANY provider has an active key
		cursor, err := mongoDB.TenantAPIKeys().Find(ctx, bson.M{"tenantId": tenantID})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"has_key":    false,
				"status":     "unconfigured",
				"role":       role,
				"owner_name": ownerName,
			})
			return
		}
		defer cursor.Close(ctx)

		var keys []TenantAPIKey
		if err := cursor.All(ctx, &keys); err != nil || len(keys) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"has_key":    false,
				"status":     "unconfigured",
				"role":       role,
				"owner_name": ownerName,
			})
			return
		}

		// Determine primary provider
		var settings TenantSettings
		primaryProvider := "anthropic"
		if err := mongoDB.TenantSettings().FindOne(ctx, bson.M{"tenantId": tenantID}).Decode(&settings); err == nil && settings.PrimaryProvider != "" {
			primaryProvider = settings.PrimaryProvider
		}

		// has_key is true if any LLM key exists; status is "active" if any LLM key is active
		hasActive := false
		hasYouTube := false
		for _, k := range keys {
			if k.Provider == "youtube" {
				if k.Status == "active" {
					hasYouTube = true
				}
				continue
			}
			if k.Status == "active" {
				hasActive = true
			}
		}

		overallStatus := "inactive"
		if hasActive {
			overallStatus = "active"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"has_key":          hasActive,
			"status":           overallStatus,
			"primary_provider": primaryProvider,
			"role":             role,
			"owner_name":       ownerName,
			"has_youtube_key":  hasYouTube,
		})
	}
}

func handleGetPrimaryProvider(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var settings TenantSettings
		primaryProvider := "anthropic"
		if err := mongoDB.TenantSettings().FindOne(ctx, bson.M{"tenantId": tenantID}).Decode(&settings); err == nil && settings.PrimaryProvider != "" {
			primaryProvider = settings.PrimaryProvider
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"primary_provider": primaryProvider,
		})
	}
}

func handleSetPrimaryProvider(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can change the primary provider"}`, http.StatusForbidden)
			return
		}

		var req struct {
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if getProvider(req.Provider) == nil {
			http.Error(w, `{"error":"invalid provider"}`, http.StatusBadRequest)
			return
		}

		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Verify the tenant has an active key for this provider
		var doc TenantAPIKey
		err := mongoDB.TenantAPIKeys().FindOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": req.Provider,
		}).Decode(&doc)
		if err != nil {
			http.Error(w, `{"error":"no_key","message":"You must configure an API key for this provider first"}`, http.StatusBadRequest)
			return
		}

		now := time.Now()
		_, err = mongoDB.TenantSettings().UpdateOne(ctx,
			bson.M{"tenantId": tenantID},
			bson.M{
				"$set": bson.M{
					"primaryProvider": req.Provider,
					"updatedAt":       now,
				},
				"$setOnInsert": bson.M{
					"tenantId": tenantID,
				},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Printf("Failed to set primary provider for tenant %s: %v", tenantID, err)
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"primary_provider": req.Provider,
		})
	}
}

func handleOptimize(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		// Parse path params
		analysisIDStr := r.PathValue("id")
		questionIdxStr := r.PathValue("idx")

		analysisOID, err := primitive.ObjectIDFromHex(analysisIDStr)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid analysis ID"})
			return
		}
		questionIdx, err := strconv.Atoi(questionIdxStr)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid question index"})
			return
		}

		// Parse optional force flag
		var req struct {
			Force bool `json:"force"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Load parent analysis
		dbCtx, dbCancel := context.WithTimeout(r.Context(), 10*time.Second)
		var analysis Analysis
		err = mongoDB.Analyses().FindOne(dbCtx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: analysisOID}})).Decode(&analysis)
		dbCancel()
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Analysis not found"})
			return
		}

		if questionIdx < 0 || questionIdx >= len(analysis.Result.Questions) {
			sendSSE(w, flusher, "error", map[string]string{"message": "Question index out of range"})
			return
		}

		question := analysis.Result.Questions[questionIdx]

		// Cache check
		if !req.Force {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			var cached Optimization
			optCacheFilter := tenantFilter(r.Context(), bson.D{
				{Key: "analysisId", Value: analysisOID},
				{Key: "questionIndex", Value: questionIdx},
				{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: time.Now().AddDate(0, 0, -30)}}},
			})
			err := mongoDB.Optimizations().FindOne(ctx, optCacheFilter, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&cached)
			cancel()

			if err == nil {
				sendSSE(w, flusher, "status", map[string]string{"message": "Found cached optimization..."})
				resultJSON, _ := json.Marshal(cached.Result)
				sendSSE(w, flusher, "done", map[string]any{
					"result":                   string(resultJSON),
					"id":                       cached.ID.Hex(),
					"cached":                   true,
					"model":                    cached.Model,
					"created_at":               cached.CreatedAt,
					"brand_context_used":       cached.BrandContextUsed,
					"brand_profile_updated_at": cached.BrandProfileUpdatedAt,
					"brand_status":             cached.BrandStatus,
				})
				trackServerEvent(mongoDB, "custom.server.optimize.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": analysis.Domain, "duration_ms": time.Since(startTime).Milliseconds(), "cached": true})
				return
			}
		}

		// Resolve primary LLM provider and API key for this tenant
		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Starting optimization analysis for: " + question.Question,
		})

		optBrandInfo := lookupBrandContext(mongoDB, analysis.Domain, saas.TenantIDFromContext(r.Context()))

		pageURLsList := strings.Join(question.PageURLs, "\n- ")
		if pageURLsList != "" {
			pageURLsList = "\n- " + pageURLsList
		}

		brandStatusNote := ""
		if question.BrandStatus == "aspirational" {
			brandStatusNote = `

NOTE — ASPIRATIONAL QUESTION: This question is a brand aspiration — the brand wants to be found for this topic, but the site only weakly addresses it. Recommendations should emphasize creating or significantly enhancing content for this topic. Score dimensions realistically based on the current weak coverage.`
		} else if question.BrandStatus == "missing" {
			brandStatusNote = `

NOTE — MISSING COVERAGE: This question is a brand aspiration NOT currently addressed on the site at all. Recommendations should focus on creating new content from scratch. All dimension scores will likely be very low. Consider what competitors do for this topic as a benchmark for what content to create.`
		} else if question.BrandStatus == "discovery" {
			brandStatusNote = `

NOTE — DISCOVERY QUESTION: This question represents what your target audience searches for without knowing your brand. They are searching the category, not your brand. Evaluate how well the site's content would satisfy this category-level question and position the brand as a solution. Score realistically — if no content addresses this, scores will be low. Recommendations should focus on creating discoverable content for this topic.`
		}

		prompt := fmt.Sprintf(`You are an LLM visibility analyst. Your task is to assess how likely a large language model (like ChatGPT, Claude, Perplexity, or Gemini with web search) is to surface and cite a website's answer to a specific question.

Website domain: %s
Question: %s
Relevant page URLs from this site:%s

Your analysis must evaluate four dimensions, each scored 0-100:

## Dimension 1: Content Authority (30%% weight)
Visit the page URL(s) above and evaluate:
- Does the content include quotations from authoritative sources? (Research shows +41%% visibility)
- Does it include specific statistics and data points? (+33%% visibility)
- Does it cite external references/sources inline? (+28%% visibility)
- Is the writing fluent and well-structured? (+29%% visibility)
- Does it use appropriate technical/domain terminology? (+19%% visibility)
- Is there keyword stuffing or marketing fluff? (-9%% visibility — harmful)

## Dimension 2: Structural Optimization (20%% weight)
- Is the answer to the question front-loaded (prominent) or buried deep in the page?
- Is the content concise and focused, or sprawling and padded?
- Are there machine-readable structures (Schema.org, comparison tables, FAQ blocks, bulleted lists)?
- Does the content explain "why" not just "what" — providing justification language?

## Dimension 3: Source Authority (30%% weight)
Search the web for third-party coverage:
- Is this site mentioned by independent review sites, industry publications, or analysts?
- Is this earned media coverage (independent third parties) rather than brand-owned or social content?
- AI search engines cite earned media 72-92%% of the time and virtually ignore social content.
- How does this site's authority compare to other sites answering the same question?

## Dimension 4: Knowledge Persistence (20%% weight)
Search the web to assess:
- How widely does this answer/information appear across the web? (Higher frequency = better)
- Is the content written in a clear, educational, didactic style?
- Would this content be effective as RAG context — clear, self-contained answer passages?
- Information widely repeated across high-quality sources is far more likely encoded in LLM weights.

Also identify:
- 3-5 competing websites that answer this same question (search for the question)
- For each competitor: estimate their overall score and note their key strengths
- 3-5 prioritized recommendations (high/medium/low) for improving the site's LLM visibility for this question

Return your analysis as JSON in exactly this format (no markdown code fences, just raw JSON):
{
  "overall_score": <0-100 weighted average>,
  "content_authority": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "structural_optimization": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "source_authority": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "knowledge_persistence": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "competitors": [
    {
      "domain": "competitor-site.com",
      "score_estimate": <0-100>,
      "strengths": "Brief description of why they rank well"
    }
  ],
  "recommendations": [
    {
      "priority": "high",
      "action": "Specific actionable step",
      "expected_impact": "What improvement to expect",
      "dimension": "content_authority"
    }
  ]
}

Be specific and evidence-based in your scoring. Reference actual content you found on the pages. The overall_score should be the weighted average: content_authority*0.30 + structural_optimization*0.20 + source_authority*0.30 + knowledge_persistence*0.20.%s%s`, analysis.Domain, question.Question, pageURLsList, brandStatusNote, optBrandInfo.ContextString)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						log.Printf("%s API (%s) overloaded for optimization, will retry (attempt %d/%d)", provider.Name(), model.ID, attempt+1, maxRetries)
						continue
					}
					break
				}
				if err != nil {
					code := classifyError(err)
					sendSSEError(w, flusher, code)
					saveFailedAnalysis(mongoDB, r.Context(), analysis.Domain, "optimization", code, model.Name)
					return
				}

				// Parse and save
				cleanJSON := stripJSONFencing(result.ResultJSON)
				var optResult OptimizationResult
				if err := json.Unmarshal([]byte(cleanJSON), &optResult); err != nil {
					log.Printf("Failed to parse optimization result: %v", err)
					sendSSE(w, flusher, "done", map[string]string{"result": result.ResultJSON})
					return
				}

				opt := Optimization{
					AnalysisID:            analysisOID,
					QuestionIndex:         questionIdx,
					Question:              question.Question,
					Domain:                analysis.Domain,
					TenantID:              saas.TenantIDFromContext(r.Context()),
					PageURLs:              question.PageURLs,
					Result:                optResult,
					RawText:               result.RawText,
					BrandStatus:           question.BrandStatus,
					Model:                 model.Name,
					BrandContextUsed:      optBrandInfo.Used,
					BrandProfileUpdatedAt: optBrandInfo.ProfileUpdatedAt,
					CreatedAt:             time.Now(),
				}
				insertResult, insertErr := mongoDB.Optimizations().InsertOne(r.Context(), opt)
				var savedID string
				if insertErr != nil {
					log.Printf("Failed to save optimization: %v", insertErr)
				} else if oid, ok := insertResult.InsertedID.(primitive.ObjectID); ok {
					savedID = oid.Hex()
					go createTodosFromOptimization(mongoDB, oid, analysisOID, analysis.Domain, question.Question, saas.TenantIDFromContext(r.Context()), question.BrandStatus, optResult)
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result":                   result.ResultJSON,
					"id":                       savedID,
					"cached":                   false,
					"model":                    model.Name,
					"created_at":               opt.CreatedAt,
					"brand_context_used":       optBrandInfo.Used,
					"brand_profile_updated_at": optBrandInfo.ProfileUpdatedAt,
					"brand_status":             question.BrandStatus,
				})
				trackServerEvent(mongoDB, "custom.server.optimize.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": analysis.Domain, "duration_ms": time.Since(startTime).Milliseconds(), "cached": false, "score": optResult.OverallScore})
				return
			}

			log.Printf("%s API (%s) exhausted retries for optimization: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
		saveFailedAnalysis(mongoDB, r.Context(), analysis.Domain, "optimization", ErrCodeAPIOverloaded, "")
	}
}

func handleGetOptimization(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		analysisIDStr := r.PathValue("id")
		questionIdxStr := r.PathValue("idx")

		analysisOID, err := primitive.ObjectIDFromHex(analysisIDStr)
		if err != nil {
			http.Error(w, "Invalid analysis ID", http.StatusBadRequest)
			return
		}
		questionIdx, err := strconv.Atoi(questionIdxStr)
		if err != nil {
			http.Error(w, "Invalid question index", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var opt Optimization
		err = mongoDB.Optimizations().FindOne(ctx, tenantFilter(r.Context(), bson.D{
			{Key: "analysisId", Value: analysisOID},
			{Key: "questionIndex", Value: questionIdx},
		}), options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&opt)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(opt)
	}
}

func handleListOptimizations(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetLimit(100).
			SetProjection(bson.D{
				{Key: "domain", Value: 1},
				{Key: "question", Value: 1},
				{Key: "questionIndex", Value: 1},
				{Key: "result.overallScore", Value: 1},
				{Key: "model", Value: 1},
				{Key: "public", Value: 1},
				{Key: "brandStatus", Value: 1},
				{Key: "brandContextUsed", Value: 1},
				{Key: "brandProfileUpdatedAt", Value: 1},
				{Key: "createdAt", Value: 1},
			})

		cursor, err := mongoDB.Optimizations().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var optimizations []Optimization
		if err := cursor.All(ctx, &optimizations); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		summaries := make([]OptimizationSummary, len(optimizations))
		for i, o := range optimizations {
			summaries[i] = OptimizationSummary{
				ID:                    o.ID,
				Domain:                o.Domain,
				Question:              o.Question,
				QuestionIndex:         o.QuestionIndex,
				OverallScore:          o.Result.OverallScore,
				Model:                 o.Model,
				Public:                o.Public,
				BrandStatus:           o.BrandStatus,
				BrandContextUsed:      o.BrandContextUsed,
				BrandProfileUpdatedAt: o.BrandProfileUpdatedAt,
				CreatedAt:             o.CreatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleGetOptimizationByID(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var opt Optimization
		err = mongoDB.Optimizations().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}})).Decode(&opt)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(opt)
	}
}

// todoSummary creates a 1-2 line summary from action and expected impact.
func todoSummary(action, impact string) string {
	// Truncate action to first sentence or 120 chars
	a := action
	if idx := strings.Index(a, ". "); idx > 0 && idx < 120 {
		a = a[:idx+1]
	} else if len(a) > 120 {
		a = a[:117] + "..."
	}
	// Append abbreviated impact if room
	if impact != "" {
		imp := impact
		if len(imp) > 80 {
			imp = imp[:77] + "..."
		}
		return a + " → " + imp
	}
	return a
}

func createTodosFromOptimization(mongoDB *MongoDB, optimizationID, analysisID primitive.ObjectID, domain, question, tenantID, brandStatus string, result OptimizationResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(result.Recommendations) == 0 {
		return
	}

	var tags []string
	if brandStatus == "discovery" {
		tags = []string{"discovery"}
	}

	var todos []any
	now := time.Now()
	for _, rec := range result.Recommendations {
		todos = append(todos, TodoItem{
			OptimizationID: optimizationID,
			AnalysisID:     analysisID,
			Domain:         domain,
			TenantID:       tenantID,
			Question:       question,
			Action:         rec.Action,
			Summary:        todoSummary(rec.Action, rec.ExpectedImpact),
			ExpectedImpact: rec.ExpectedImpact,
			Dimension:      rec.Dimension,
			Priority:       rec.Priority,
			Status:         "todo",
			CreatedAt:      now,
			Tags:           tags,
		})
	}

	_, err := mongoDB.Todos().InsertMany(ctx, todos)
	if err != nil {
		log.Printf("Failed to create todos from optimization: %v", err)
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

func createTodosFromVideoAnalysis(mongoDB *MongoDB, videoAnalysisID primitive.ObjectID, domain, tenantID string, recommendations []VideoRecommendation) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(recommendations) == 0 {
		return
	}

	// Delete previous video todos for this domain to avoid duplicates on re-run
	mongoDB.Todos().DeleteMany(ctx, bson.D{
		{Key: "videoAnalysisId", Value: videoAnalysisID},
	})

	dimLabels := map[string]string{
		"transcript_authority": "Video: Transcript Authority",
		"topical_dominance":   "Video: Topical Dominance",
		"citation_network":    "Video: Citation Network",
		"brand_narrative":     "Video: Brand Narrative",
	}

	var todos []any
	now := time.Now()
	for _, rec := range recommendations {
		question := dimLabels[rec.Dimension]
		if question == "" {
			question = "Video: LLM Authority"
		}
		todos = append(todos, TodoItem{
			VideoAnalysisID: &videoAnalysisID,
			SourceType:      "video",
			Domain:          domain,
			TenantID:        tenantID,
			Question:        question,
			Action:          rec.Action,
			Summary:         todoSummary(rec.Action, rec.ExpectedImpact),
			ExpectedImpact:  rec.ExpectedImpact,
			Dimension:       rec.Dimension,
			Priority:        rec.Priority,
			Status:          "todo",
			CreatedAt:       now,
		})
	}

	_, err := mongoDB.Todos().InsertMany(ctx, todos)
	if err != nil {
		log.Printf("Failed to create todos from video analysis: %v", err)
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

// deduplicateTodos finds semantically similar open todos for a domain and archives duplicates.
// Uses normalized word overlap (Jaccard similarity) to detect similar actions.
func deduplicateTodos(mongoDB *MongoDB, domain, tenantID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.D{
		{Key: "domain", Value: domain},
		{Key: "status", Value: "todo"},
	}
	if tenantID != "" {
		filter = append(filter, bson.E{Key: "tenantId", Value: tenantID})
	}

	cursor, err := mongoDB.Todos().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		log.Printf("deduplicateTodos: find error: %v", err)
		return
	}
	var todos []TodoItem
	if err := cursor.All(ctx, &todos); err != nil {
		log.Printf("deduplicateTodos: cursor error: %v", err)
		return
	}

	if len(todos) < 2 {
		return
	}

	// Normalize and tokenize action text for Jaccard similarity
	tokenize := func(s string) map[string]bool {
		words := map[string]bool{}
		for _, w := range strings.Fields(strings.ToLower(s)) {
			w = strings.Trim(w, ".,;:!?\"'()-")
			if len(w) > 2 { // skip small words
				words[w] = true
			}
		}
		return words
	}
	jaccard := func(a, b map[string]bool) float64 {
		if len(a) == 0 || len(b) == 0 {
			return 0
		}
		intersection := 0
		for w := range a {
			if b[w] {
				intersection++
			}
		}
		union := len(a) + len(b) - intersection
		if union == 0 {
			return 0
		}
		return float64(intersection) / float64(union)
	}

	type group struct {
		keepIdx int
		dupes   []int
	}

	tokens := make([]map[string]bool, len(todos))
	for i, t := range todos {
		tokens[i] = tokenize(t.Action)
	}

	merged := map[int]bool{}
	var groups []group

	for i := 0; i < len(todos); i++ {
		if merged[i] {
			continue
		}
		g := group{keepIdx: i}
		for j := i + 1; j < len(todos); j++ {
			if merged[j] {
				continue
			}
			// Same dimension and high textual similarity
			if todos[i].Dimension == todos[j].Dimension && jaccard(tokens[i], tokens[j]) > 0.6 {
				g.dupes = append(g.dupes, j)
				merged[j] = true
			}
		}
		if len(g.dupes) > 0 {
			groups = append(groups, g)
		}
	}

	if len(groups) == 0 {
		return
	}

	// Archive duplicates, keeping the highest-priority version
	priorityRank := map[string]int{"high": 3, "medium": 2, "low": 1}
	for _, g := range groups {
		bestIdx := g.keepIdx
		bestRank := priorityRank[todos[bestIdx].Priority]
		for _, di := range g.dupes {
			if priorityRank[todos[di].Priority] > bestRank {
				bestRank = priorityRank[todos[di].Priority]
				bestIdx = di
			}
		}

		// Archive all dupes except the best
		for _, idx := range append([]int{g.keepIdx}, g.dupes...) {
			if idx == bestIdx {
				continue
			}
			_, err := mongoDB.Todos().UpdateOne(ctx,
				bson.D{{Key: "_id", Value: todos[idx].ID}},
				bson.D{{Key: "$set", Value: bson.D{
					{Key: "status", Value: "archived"},
					{Key: "archivedReason", Value: fmt.Sprintf("Merged: similar to existing todo")},
				}}},
			)
			if err != nil {
				log.Printf("deduplicateTodos: archive error: %v", err)
			}
		}
	}

	totalArchived := 0
	for _, g := range groups {
		totalArchived += len(g.dupes)
	}
	if totalArchived > 0 {
		log.Printf("deduplicateTodos: archived %d duplicate todos for %s", totalArchived, domain)
	}
}

func handleListTodos(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		if status := r.URL.Query().Get("status"); status != "" {
			filter = append(filter, bson.E{Key: "status", Value: status})
		}
		if optID := r.URL.Query().Get("optimization_id"); optID != "" {
			oid, err := primitive.ObjectIDFromHex(optID)
			if err == nil {
				filter = append(filter, bson.E{Key: "optimizationId", Value: oid})
			}
		}
		if sourceType := r.URL.Query().Get("source_type"); sourceType != "" {
			filter = append(filter, bson.E{Key: "sourceType", Value: sourceType})
		}

		opts := options.Find().
			SetSort(bson.D{
				{Key: "createdAt", Value: -1},
			}).
			SetLimit(200)

		cursor, err := mongoDB.Todos().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var todos []TodoItem
		if err := cursor.All(ctx, &todos); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if todos == nil {
			todos = []TodoItem{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(todos)
	}
}

func handleUpdateTodo(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Status != "todo" && req.Status != "completed" && req.Status != "backlogged" && req.Status != "archived" {
			http.Error(w, "Status must be 'todo', 'completed', 'backlogged', or 'archived'", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		now := time.Now()
		var update bson.D
		switch req.Status {
		case "completed":
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
					{Key: "completedAt", Value: now},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "backloggedAt", Value: ""},
					{Key: "archivedAt", Value: ""},
				}},
			}
		case "backlogged":
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
					{Key: "backloggedAt", Value: now},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "completedAt", Value: ""},
					{Key: "archivedAt", Value: ""},
				}},
			}
		case "archived":
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
					{Key: "archivedAt", Value: now},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "completedAt", Value: ""},
					{Key: "backloggedAt", Value: ""},
				}},
			}
		default:
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "completedAt", Value: ""},
					{Key: "backloggedAt", Value: ""},
					{Key: "archivedAt", Value: ""},
				}},
			}
		}

		result, err := mongoDB.Todos().UpdateOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}}), update)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.MatchedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleBulkArchiveTodos(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SourceType string `json:"source_type"` // "video" or "optimization"
			Domain     string `json:"domain"`
			Question   string `json:"question"` // for optimization: archive todos matching this question
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		now := time.Now()
		filter := tenantFilter(r.Context(), bson.D{
			{Key: "domain", Value: req.Domain},
			{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"todo", "backlogged"}}}},
		})
		if req.SourceType == "video" {
			filter = append(filter, bson.E{Key: "sourceType", Value: "video"})
		} else if req.SourceType == "reddit" {
			filter = append(filter, bson.E{Key: "sourceType", Value: "reddit"})
		} else if req.SourceType == "optimization" && req.Question != "" {
			filter = append(filter, bson.E{Key: "question", Value: req.Question})
			// Optimization todos have empty sourceType for backwards compat
			filter = append(filter, bson.E{Key: "sourceType", Value: bson.D{{Key: "$ne", Value: "video"}}})
		}

		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "status", Value: "archived"},
				{Key: "archivedAt", Value: now},
			}},
			{Key: "$unset", Value: bson.D{
				{Key: "completedAt", Value: ""},
				{Key: "backloggedAt", Value: ""},
			}},
		}

		result, err := mongoDB.Todos().UpdateMany(ctx, filter, update)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"archived_count": result.ModifiedCount})
	}
}

// handleGetDomainShare returns the current share state for a domain.
func handleGetDomainShare(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var ds DomainShare
		err := mongoDB.DomainShares().FindOne(ctx, bson.M{
			"tenantId": tenantID,
			"domain":   domain,
		}).Decode(&ds)
		if err == mongo.ErrNoDocuments {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"visibility": "private",
				"share_id":   "",
				"share_url":  "",
			})
			return
		}
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		shareURL := ""
		if ds.ShareID != "" {
			shareURL = "/share/" + ds.ShareID
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"visibility": ds.Visibility,
			"share_id":   ds.ShareID,
			"share_url":  shareURL,
			"view_count": ds.ViewCount,
		})
	}
}

// handleSetDomainShare sets the sharing visibility for a domain.
func handleSetDomainShare(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		tenantID := saas.TenantIDFromContext(r.Context())

		if !isShareAdmin(r.Context()) {
			http.Error(w, `{"error":"must be owner or admin to share"}`, http.StatusForbidden)
			return
		}

		var req struct {
			Visibility string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Visibility != "private" && req.Visibility != "public" && req.Visibility != "popular" {
			http.Error(w, `{"error":"visibility must be private, public, or popular"}`, http.StatusBadRequest)
			return
		}

		if req.Visibility == "popular" && !isRootShareAdmin(r.Context()) {
			http.Error(w, `{"error":"only root tenant admins can mark domains as popular"}`, http.StatusForbidden)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Verify the tenant actually has data for this domain
		count, err := mongoDB.Analyses().CountDocuments(ctx, bson.M{"tenantId": tenantID, "domain": domain})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if count == 0 {
			// Also check optimizations and brand profiles
			count, _ = mongoDB.Optimizations().CountDocuments(ctx, bson.M{"tenantId": tenantID, "domain": domain})
			if count == 0 {
				count, _ = mongoDB.BrandProfiles().CountDocuments(ctx, bson.M{"tenantId": tenantID, "domain": domain})
			}
		}
		if count == 0 {
			http.Error(w, `{"error":"no data found for this domain"}`, http.StatusNotFound)
			return
		}

		now := time.Now()
		shareID := ""
		if req.Visibility == "public" || req.Visibility == "popular" {
			shareID = generateShareID()
		}

		filter := bson.M{"tenantId": tenantID, "domain": domain}
		update := bson.M{
			"$set": bson.M{
				"visibility": req.Visibility,
				"shareId":    shareID,
				"updatedAt":  now,
			},
			"$setOnInsert": bson.M{
				"tenantId":  tenantID,
				"domain":    domain,
				"createdAt": now,
			},
		}
		opts := options.Update().SetUpsert(true)
		_, err = mongoDB.DomainShares().UpdateOne(ctx, filter, update, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Capture screenshot when domain is marked as popular
		if req.Visibility == "popular" {
			go captureBrandScreenshot(mongoDB, domain, tenantID)
		}

		shareURL := ""
		if shareID != "" {
			shareURL = "/share/" + shareID
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"visibility": req.Visibility,
			"share_id":   shareID,
			"share_url":  shareURL,
		})
	}
}

// handleGetSharedDomain returns all domain data for a public/popular share link.
func handleGetSharedDomain(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		var ds DomainShare
		err := mongoDB.DomainShares().FindOne(ctx, bson.M{
			"shareId":    shareID,
			"visibility": bson.M{"$in": []string{"public", "popular"}},
		}).Decode(&ds)
		if err == mongo.ErrNoDocuments {
			http.Error(w, `{"error":"share not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Increment view count (fire-and-forget)
		go func() {
			bgCtx, bgCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer bgCancel()
			mongoDB.DomainShares().UpdateOne(bgCtx, bson.M{"_id": ds.ID}, bson.M{"$inc": bson.M{"viewCount": 1}})
		}()

		tenantDomain := bson.M{"tenantId": ds.TenantID, "domain": ds.Domain}

		// Fetch all data concurrently using errgroup
		var (
			analyses        []Analysis
			optimizations   []Optimization
			brandProfile    *BrandProfile
			videoAnalysis   *VideoAnalysis
			redditAnalysis  *RedditAnalysis
			searchAnalysis  *SearchAnalysis
			llmTest         *LLMTest
			todos           []TodoItem
			domainSummary   *DomainSummary
			visibilityScore map[string]any
		)

		g, gctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			cur, err := mongoDB.Analyses().Find(gctx, tenantDomain,
				options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(20))
			if err != nil {
				return nil
			}
			cur.All(gctx, &analyses)
			cur.Close(gctx)
			for i := range analyses {
				analyses[i].RawText = ""
			}
			return nil
		})

		g.Go(func() error {
			cur, err := mongoDB.Optimizations().Find(gctx, tenantDomain,
				options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(50))
			if err != nil {
				return nil
			}
			cur.All(gctx, &optimizations)
			cur.Close(gctx)
			for i := range optimizations {
				optimizations[i].RawText = ""
			}
			return nil
		})

		g.Go(func() error {
			var bp BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(gctx, tenantDomain).Decode(&bp); err == nil {
				brandProfile = &bp
			}
			return nil
		})

		g.Go(func() error {
			var va VideoAnalysis
			if err := mongoDB.VideoAnalyses().FindOne(gctx, tenantDomain).Decode(&va); err == nil {
				va.RawText = ""
				videoAnalysis = &va
			}
			return nil
		})

		g.Go(func() error {
			var ra RedditAnalysis
			if err := mongoDB.RedditAnalyses().FindOne(gctx, tenantDomain,
				options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})).Decode(&ra); err == nil {
				ra.RawText = ""
				redditAnalysis = &ra
			}
			return nil
		})

		g.Go(func() error {
			var sa SearchAnalysis
			if err := mongoDB.SearchAnalyses().FindOne(gctx, tenantDomain,
				options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})).Decode(&sa); err == nil {
				sa.RawText = ""
				searchAnalysis = &sa
			}
			return nil
		})

		g.Go(func() error {
			var lt LLMTest
			testFilter := bson.M{
				"tenantId":     ds.TenantID,
				"domain":       ds.Domain,
				"competitorOf": bson.M{"$in": []any{"", nil}},
			}
			if err := mongoDB.LLMTests().FindOne(gctx, testFilter,
				options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})).Decode(&lt); err == nil {
				llmTest = &lt
			}
			return nil
		})

		g.Go(func() error {
			todoFilter := bson.M{"tenantId": ds.TenantID, "domain": ds.Domain, "status": "todo"}
			cur, err := mongoDB.Todos().Find(gctx, todoFilter,
				options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
			if err != nil {
				return nil
			}
			cur.All(gctx, &todos)
			cur.Close(gctx)
			return nil
		})

		g.Go(func() error {
			var dsm DomainSummary
			if err := mongoDB.DomainSummaries().FindOne(gctx, tenantDomain).Decode(&dsm); err == nil {
				dsm.RawText = ""
				domainSummary = &dsm
			}
			return nil
		})

		// Compute visibility score for the shared view
		g.Go(func() error {
			type vsComponent struct {
				Name      string  `json:"name"`
				Score     int     `json:"score"`
				Weight    float64 `json:"weight"`
				Available bool    `json:"available"`
			}
			components := []vsComponent{
				{Name: "Optimization", Weight: 0.30},
				{Name: "Video Authority", Weight: 0.20},
				{Name: "Reddit Authority", Weight: 0.20},
				{Name: "Search Visibility", Weight: 0.15},
				{Name: "LLM Test", Weight: 0.15},
			}
			td := bson.D{{Key: "tenantId", Value: ds.TenantID}, {Key: "domain", Value: ds.Domain}}

			// Optimization average
			cur, err := mongoDB.Optimizations().Find(gctx, td,
				options.Find().SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}))
			if err == nil {
				var opts []struct {
					Result struct {
						OverallScore int `bson:"overallScore"`
					} `bson:"result"`
				}
				if cur.All(gctx, &opts) == nil && len(opts) > 0 {
					total := 0
					for _, o := range opts {
						total += o.Result.OverallScore
					}
					components[0].Score = total / len(opts)
					components[0].Available = true
				}
			}

			// Video authority
			var vaScore struct {
				Result *struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
			}
			if mongoDB.VideoAnalyses().FindOne(gctx, td,
				options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
			).Decode(&vaScore) == nil && vaScore.Result != nil {
				components[1].Score = vaScore.Result.OverallScore
				components[1].Available = true
			}

			// Reddit authority
			var raScore struct {
				Result *struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
			}
			if mongoDB.RedditAnalyses().FindOne(gctx, td,
				options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
			).Decode(&raScore) == nil && raScore.Result != nil {
				components[2].Score = raScore.Result.OverallScore
				components[2].Available = true
			}

			// Search visibility
			var saScore struct {
				Result *struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
			}
			if mongoDB.SearchAnalyses().FindOne(gctx, td,
				options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
			).Decode(&saScore) == nil && saScore.Result != nil {
				components[3].Score = saScore.Result.OverallScore
				components[3].Available = true
			}

			// LLM Test
			var ltScore struct {
				OverallScore int `bson:"overallScore"`
			}
			if mongoDB.LLMTests().FindOne(gctx,
				bson.D{
					{Key: "domain", Value: ds.Domain},
					{Key: "tenantId", Value: ds.TenantID},
					{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
				},
				options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "overallScore", Value: 1}}),
			).Decode(&ltScore) == nil {
				components[4].Score = ltScore.OverallScore
				components[4].Available = true
			}

			totalWeight := 0.0
			weightedSum := 0.0
			availableCount := 0
			for _, c := range components {
				if c.Available {
					totalWeight += c.Weight
					weightedSum += float64(c.Score) * c.Weight
					availableCount++
				}
			}
			score := 0
			if totalWeight > 0 {
				score = int(weightedSum / totalWeight)
			}
			visibilityScore = map[string]any{
				"score":      score,
				"components": components,
				"available":  availableCount,
				"total":      len(components),
			}
			return nil
		})

		_ = g.Wait()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"domain":           ds.Domain,
			"visibility":       ds.Visibility,
			"analyses":         analyses,
			"optimizations":    optimizations,
			"brand_profile":    brandProfile,
			"video_analysis":   videoAnalysis,
			"reddit_analysis":  redditAnalysis,
			"search_analysis":  searchAnalysis,
			"llm_test":         llmTest,
			"todos":            todos,
			"domain_summary":   domainSummary,
			"visibility_score": visibilityScore,
		})
	}
}

// popularDomainsCache caches the popular domains JSON response (5-minute TTL).
var popularDomainsCache struct {
	sync.Mutex
	data      []byte
	expiresAt time.Time
}

// handleGetPopularDomains returns all domains marked as "popular".
// Uses batch queries instead of per-domain lookups and caches the result for 5 minutes.
func handleGetPopularDomains(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check cache
		popularDomainsCache.Lock()
		if len(popularDomainsCache.data) > 0 && time.Now().Before(popularDomainsCache.expiresAt) {
			cached := popularDomainsCache.data
			popularDomainsCache.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write(cached)
			return
		}
		popularDomainsCache.Unlock()

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		cursor, err := mongoDB.DomainShares().Find(ctx, bson.M{"visibility": "popular"})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var shares []DomainShare
		if err := cursor.All(ctx, &shares); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		if len(shares) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		type PopularDomain struct {
			Domain        string `json:"domain"`
			BrandName     string `json:"brand_name"`
			ShareID       string `json:"share_id"`
			AvgScore      int    `json:"avg_score"`
			ReportCount   int    `json:"report_count"`
			AnalysisCount int    `json:"analysis_count"`
			HasVideo      bool   `json:"has_video"`
			HasScreenshot bool   `json:"has_screenshot"`
		}

		// Build lookup key → index map and $or filter for batch queries
		type domainKey struct{ tenantID, domain string }
		keyIndex := make(map[domainKey]int, len(shares))
		results := make([]PopularDomain, len(shares))
		orFilter := make(bson.A, 0, len(shares))
		domains := make([]string, 0, len(shares))

		for i, s := range shares {
			results[i] = PopularDomain{Domain: s.Domain, ShareID: s.ShareID}
			keyIndex[domainKey{s.TenantID, s.Domain}] = i
			orFilter = append(orFilter, bson.M{"tenantId": s.TenantID, "domain": s.Domain})
			domains = append(domains, s.Domain)
		}

		batchFilter := bson.M{"$or": orFilter}

		// Batch: brand profiles (brand names)
		bpCur, err := mongoDB.BrandProfiles().Find(ctx, batchFilter,
			options.Find().SetProjection(bson.M{"tenantId": 1, "domain": 1, "brandName": 1}))
		if err == nil {
			defer bpCur.Close(ctx)
			for bpCur.Next(ctx) {
				var bp struct {
					TenantID  string `bson:"tenantId"`
					Domain    string `bson:"domain"`
					BrandName string `bson:"brandName"`
				}
				if bpCur.Decode(&bp) == nil {
					if idx, ok := keyIndex[domainKey{bp.TenantID, bp.Domain}]; ok {
						results[idx].BrandName = bp.BrandName
					}
				}
			}
		}

		// Batch: analysis counts via aggregation
		analysisPipeline := mongo.Pipeline{
			{{Key: "$match", Value: batchFilter}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "tenantId", Value: "$tenantId"}, {Key: "domain", Value: "$domain"}}},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}}},
		}
		aCur, err := mongoDB.Analyses().Aggregate(ctx, analysisPipeline)
		if err == nil {
			defer aCur.Close(ctx)
			for aCur.Next(ctx) {
				var row struct {
					ID    struct{ TenantID, Domain string } `bson:"_id"`
					Count int                               `bson:"count"`
				}
				if aCur.Decode(&row) == nil {
					if idx, ok := keyIndex[domainKey{row.ID.TenantID, row.ID.Domain}]; ok {
						results[idx].AnalysisCount = row.Count
					}
				}
			}
		}

		// Batch: optimization counts + avg scores via aggregation
		optPipeline := mongo.Pipeline{
			{{Key: "$match", Value: batchFilter}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "tenantId", Value: "$tenantId"}, {Key: "domain", Value: "$domain"}}},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
				{Key: "avgScore", Value: bson.D{{Key: "$avg", Value: "$result.overallScore"}}},
			}}},
		}
		oCur, err := mongoDB.Optimizations().Aggregate(ctx, optPipeline)
		if err == nil {
			defer oCur.Close(ctx)
			for oCur.Next(ctx) {
				var row struct {
					ID       struct{ TenantID, Domain string } `bson:"_id"`
					Count    int                               `bson:"count"`
					AvgScore float64                           `bson:"avgScore"`
				}
				if oCur.Decode(&row) == nil {
					if idx, ok := keyIndex[domainKey{row.ID.TenantID, row.ID.Domain}]; ok {
						results[idx].ReportCount = row.Count
						results[idx].AvgScore = int(row.AvgScore)
					}
				}
			}
		}

		// Batch: video analysis existence
		vCur, err := mongoDB.VideoAnalyses().Find(ctx, batchFilter,
			options.Find().SetProjection(bson.M{"tenantId": 1, "domain": 1}))
		if err == nil {
			defer vCur.Close(ctx)
			for vCur.Next(ctx) {
				var v struct {
					TenantID string `bson:"tenantId"`
					Domain   string `bson:"domain"`
				}
				if vCur.Decode(&v) == nil {
					if idx, ok := keyIndex[domainKey{v.TenantID, v.Domain}]; ok {
						results[idx].HasVideo = true
					}
				}
			}
		}

		// Batch: screenshot existence
		ssCur, err := mongoDB.BrandScreenshots().Find(ctx,
			bson.M{"domain": bson.M{"$in": domains}, "sizeBytes": bson.M{"$gt": 0}},
			options.Find().SetProjection(bson.M{"domain": 1}))
		if err == nil {
			defer ssCur.Close(ctx)
			for ssCur.Next(ctx) {
				var ss struct {
					Domain string `bson:"domain"`
				}
				if ssCur.Decode(&ss) == nil {
					// Mark all results with this domain
					for i := range results {
						if results[i].Domain == ss.Domain {
							results[i].HasScreenshot = true
						}
					}
				}
			}
		}

		// Fall back: use analysis count as report count if no optimizations
		for i := range results {
			if results[i].ReportCount == 0 {
				results[i].ReportCount = results[i].AnalysisCount
			}
		}

		respBytes, _ := json.Marshal(results)

		// Update cache
		popularDomainsCache.Lock()
		popularDomainsCache.data = respBytes
		popularDomainsCache.expiresAt = time.Now().Add(5 * time.Minute)
		popularDomainsCache.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write(respBytes)
	}
}

// handleServeBrandScreenshot serves a cached screenshot JPEG for a popular domain.
func handleServeBrandScreenshot(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var ss BrandScreenshot
		filter := bson.M{"domain": domain, "sizeBytes": bson.M{"$gt": 0}}
		if err := mongoDB.BrandScreenshots().FindOne(ctx, filter).Decode(&ss); err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "Screenshot not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", ss.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(ss.ImageData)))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("ETag", fmt.Sprintf(`"%s-%d"`, domain, ss.CapturedAt.Unix()))
		w.Write(ss.ImageData)
	}
}

// isCrawler checks if the User-Agent is a social media or search engine crawler.
func isCrawler(ua string) bool {
	ua = strings.ToLower(ua)
	crawlers := []string{
		"facebookexternalhit", "twitterbot", "linkedinbot", "slackbot",
		"discordbot", "whatsapp", "telegrambot", "googlebot", "bingbot",
		"applebot", "yandexbot", "baiduspider", "duckduckbot",
		"ia_archiver", "embedly", "quora link preview", "outbrain",
		"pinterest", "redditbot", "rogerbot", "showyoubot", "vkshare",
		"w3c_validator", "facebot", "kakaotalk-scrap", "naverbot",
		"seznambot", "yahoo! slurp", "semrushbot", "ahrefsbot",
		"petalbot", "bytespider", "gptbot", "chatgpt-user", "claudebot",
		"anthropic-ai", "perplexitybot", "cohere-ai",
	}
	for _, c := range crawlers {
		if strings.Contains(ua, c) {
			return true
		}
	}
	return false
}

// handleShareOG serves dynamic OG meta tags for crawlers, SPA for browsers.
func handleShareOG(mongoDB *MongoDB, staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")

		// If not a crawler, serve the SPA
		if !isCrawler(r.UserAgent()) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		// Crawler: look up share data and serve OG tags
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var ds DomainShare
		err := mongoDB.DomainShares().FindOne(ctx, bson.M{
			"shareId":    shareID,
			"visibility": bson.M{"$in": []string{"public", "popular"}},
		}).Decode(&ds)
		if err != nil {
			// Not found — serve SPA (which will show 404)
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		// Get brand name
		brandName := ds.Domain
		var bp BrandProfile
		if err := mongoDB.BrandProfiles().FindOne(ctx, bson.M{"tenantId": ds.TenantID, "domain": ds.Domain}).Decode(&bp); err == nil && bp.BrandName != "" {
			brandName = bp.BrandName
		}

		// Build OG meta
		title := brandName + " — LLM Visibility Report"
		desc := "See how " + brandName + " performs across AI search engines, YouTube, Reddit, and more. LLM Visibility Score and actionable optimization insights."
		baseURL := "https://llmopt.metavert.io"
		shareURL := baseURL + "/share/" + shareID

		// Screenshot URL for popular domains
		imageTag := ""
		if ds.Visibility == "popular" {
			var ss BrandScreenshot
			if err := mongoDB.BrandScreenshots().FindOne(ctx, bson.M{"domain": ds.Domain, "sizeBytes": bson.M{"$gt": 0}}).Decode(&ss); err == nil {
				imgURL := baseURL + "/api/share/popular/" + ds.Domain + "/screenshot"
				imageTag = fmt.Sprintf(`<meta property="og:image" content="%s" /><meta property="og:image:width" content="%d" /><meta property="og:image:height" content="%d" /><meta name="twitter:image" content="%s" />`,
					html.EscapeString(imgURL), ss.Width, ss.Height, html.EscapeString(imgURL))
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<title>%s</title>
<meta name="description" content="%s" />
<meta property="og:type" content="website" />
<meta property="og:title" content="%s" />
<meta property="og:description" content="%s" />
<meta property="og:url" content="%s" />
<meta property="og:site_name" content="LLM Optimizer" />
%s
<meta name="twitter:card" content="%s" />
<meta name="twitter:title" content="%s" />
<meta name="twitter:description" content="%s" />
<link rel="canonical" href="%s" />
</head>
<body>
<h1>%s</h1>
<p>%s</p>
<p><a href="%s">View the full report on LLM Optimizer</a></p>
</body>
</html>`,
			html.EscapeString(title),
			html.EscapeString(desc),
			html.EscapeString(title),
			html.EscapeString(desc),
			html.EscapeString(shareURL),
			imageTag,
			func() string {
				if imageTag != "" {
					return "summary_large_image"
				}
				return "summary"
			}(),
			html.EscapeString(title),
			html.EscapeString(desc),
			html.EscapeString(shareURL),
			html.EscapeString(title),
			html.EscapeString(desc),
			html.EscapeString(shareURL))
	}
}

// handleRobotsTxt serves a robots.txt for the site.
func handleRobotsTxt() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fmt.Fprint(w, `User-agent: *
Allow: /
Allow: /research
Allow: /docs
Allow: /share/
Disallow: /api/
Disallow: /last/
Disallow: /login
Disallow: /signup
Disallow: /setup

Sitemap: https://llmopt.metavert.io/sitemap.xml
`)
	}
}

// Sitemap types
type sitemapURL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

type sitemapIndex struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// handleSitemap generates a sitemap.xml with main pages, research, and popular brands.
func handleSitemap(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := "https://llmopt.metavert.io"
		now := time.Now().Format("2006-01-02")

		urls := []sitemapURL{
			{Loc: baseURL + "/", LastMod: now},
			{Loc: baseURL + "/research", LastMod: now},
			{Loc: baseURL + "/docs", LastMod: now},
		}

		// Add popular domains with sub-pages
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		cursor, err := mongoDB.DomainShares().Find(ctx, bson.M{"visibility": "popular"})
		if err == nil {
			var shares []DomainShare
			if cursor.All(ctx, &shares) == nil {
				for _, s := range shares {
					lastMod := s.UpdatedAt.Format("2006-01-02")
					shareBase := baseURL + "/share/" + s.ShareID
					urls = append(urls,
						sitemapURL{Loc: shareBase, LastMod: lastMod},
						sitemapURL{Loc: shareBase + "/brand", LastMod: lastMod},
						sitemapURL{Loc: shareBase + "/video", LastMod: lastMod},
						sitemapURL{Loc: shareBase + "/reddit", LastMod: lastMod},
						sitemapURL{Loc: shareBase + "/search", LastMod: lastMod},
						sitemapURL{Loc: shareBase + "/todos", LastMod: lastMod},
					)
				}
			}
			cursor.Close(ctx)
		}

		sm := sitemapIndex{
			XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
			URLs:  urls,
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write([]byte(xml.Header))
		enc := xml.NewEncoder(w)
		enc.Indent("", "  ")
		enc.Encode(sm)
	}
}

// Brand Intelligence handlers

func handleListBrands(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		opts := options.Find().
			SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
			SetLimit(50)

		cursor, err := mongoDB.BrandProfiles().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var profiles []BrandProfile
		if err := cursor.All(ctx, &profiles); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		summaries := make([]BrandProfileSummary, len(profiles))
		for i, p := range profiles {
			summaries[i] = BrandProfileSummary{
				ID:                    p.ID,
				Domain:                p.Domain,
				BrandName:             p.BrandName,
				CompetitorCount:       len(p.Competitors),
				QueryCount:            len(p.TargetQueries),
				Completeness:          computeBrandCompleteness(p),
				Public:                p.Public,
				UpdatedAt:             p.UpdatedAt,
				BrandContentUpdatedAt: p.BrandContentUpdatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleGetBrand(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var profile BrandProfile
		err := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&profile)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

// hasSubstantiveChanges compares non-presence brand fields to detect whether
// the "content" of the brand profile changed (vs just presence/metadata).
func hasSubstantiveChanges(old, new BrandProfile) bool {
	type contentFields struct {
		BrandName       string            `json:"bn"`
		Description     string            `json:"d"`
		Categories      []string          `json:"c"`
		Products        []string          `json:"p"`
		PrimaryAudience string            `json:"pa"`
		KeyUseCases     []string          `json:"ku"`
		Competitors     []BrandCompetitor `json:"co"`
		TargetQueries   []TargetQuery     `json:"tq"`
		KeyMessages     []KeyMessage      `json:"km"`
		Differentiators []string          `json:"di"`
	}
	normalize := func(bp BrandProfile) contentFields {
		cf := contentFields{
			BrandName:       bp.BrandName,
			Description:     bp.Description,
			Categories:      bp.Categories,
			Products:        bp.Products,
			PrimaryAudience: bp.PrimaryAudience,
			KeyUseCases:     bp.KeyUseCases,
			Competitors:     bp.Competitors,
			TargetQueries:   bp.TargetQueries,
			KeyMessages:     bp.KeyMessages,
			Differentiators: bp.Differentiators,
		}
		// Normalize nil slices to empty so JSON comparison is stable
		if cf.Categories == nil {
			cf.Categories = []string{}
		}
		if cf.Products == nil {
			cf.Products = []string{}
		}
		if cf.KeyUseCases == nil {
			cf.KeyUseCases = []string{}
		}
		if cf.Competitors == nil {
			cf.Competitors = []BrandCompetitor{}
		}
		if cf.TargetQueries == nil {
			cf.TargetQueries = []TargetQuery{}
		}
		if cf.KeyMessages == nil {
			cf.KeyMessages = []KeyMessage{}
		}
		if cf.Differentiators == nil {
			cf.Differentiators = []string{}
		}
		return cf
	}
	oldJSON, _ := json.Marshal(normalize(old))
	newJSON, _ := json.Marshal(normalize(new))
	return !bytes.Equal(oldJSON, newJSON)
}

func handleSaveBrand(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		var req BrandProfile
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		now := time.Now()
		tid := saas.TenantIDFromContext(r.Context())

		// Determine if substantive content changed (vs presence-only edits)
		brandFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		var existing BrandProfile
		var contentUpdatedAt *time.Time
		err := mongoDB.BrandProfiles().FindOne(ctx, brandFilter).Decode(&existing)
		if err == nil {
			// Existing profile found — check for substantive changes
			if hasSubstantiveChanges(existing, req) {
				contentUpdatedAt = &now
			} else {
				contentUpdatedAt = existing.BrandContentUpdatedAt
			}
		} else {
			// New profile — set content timestamp
			contentUpdatedAt = &now
		}

		setFields := bson.D{
			{Key: "domain", Value: domain},
			{Key: "brandName", Value: req.BrandName},
			{Key: "description", Value: req.Description},
			{Key: "categories", Value: req.Categories},
			{Key: "products", Value: req.Products},
			{Key: "primaryAudience", Value: req.PrimaryAudience},
			{Key: "keyUseCases", Value: req.KeyUseCases},
			{Key: "competitors", Value: req.Competitors},
			{Key: "targetQueries", Value: req.TargetQueries},
			{Key: "keyMessages", Value: req.KeyMessages},
			{Key: "differentiators", Value: req.Differentiators},
			{Key: "presence", Value: req.Presence},
			{Key: "presenceComplete", Value: req.PresenceComplete},
			{Key: "public", Value: req.Public},
			{Key: "updatedAt", Value: now},
		}
		if contentUpdatedAt != nil {
			setFields = append(setFields, bson.E{Key: "brandContentUpdatedAt", Value: contentUpdatedAt})
		}

		update := bson.D{
			{Key: "$set", Value: setFields},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "createdAt", Value: now},
				{Key: "tenantId", Value: tid},
			}},
		}

		opts := options.Update().SetUpsert(true)
		result, uErr := mongoDB.BrandProfiles().UpdateOne(ctx, brandFilter, update, opts)
		if uErr != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Fetch the saved profile to return it
		var saved BrandProfile
		err = mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&saved)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if result.UpsertedCount > 0 {
			w.WriteHeader(http.StatusCreated)
		}
		json.NewEncoder(w).Encode(saved)
	}
}

func handlePatchBrandSubreddits(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		var req struct {
			Subreddits []string `json:"subreddits"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_, err := mongoDB.BrandProfiles().UpdateOne(ctx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
			bson.D{{Key: "$set", Value: bson.D{{Key: "presence.subreddits", Value: req.Subreddits}}}},
		)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
}

func handleDeleteBrand(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		result, err := mongoDB.BrandProfiles().DeleteOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleDeleteAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// Find all optimizations for this analysis to cascade-delete their todos
		cursor, err := mongoDB.Optimizations().Find(ctx, bson.D{{Key: "analysisId", Value: oid}}, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		var optIDs []primitive.ObjectID
		for cursor.Next(ctx) {
			var doc struct {
				ID primitive.ObjectID `bson:"_id"`
			}
			if cursor.Decode(&doc) == nil {
				optIDs = append(optIDs, doc.ID)
			}
		}

		// Delete todos for all those optimizations
		if len(optIDs) > 0 {
			mongoDB.Todos().DeleteMany(ctx, bson.D{{Key: "optimizationId", Value: bson.D{{Key: "$in", Value: optIDs}}}})
		}

		// Delete all optimizations for this analysis
		mongoDB.Optimizations().DeleteMany(ctx, bson.D{{Key: "analysisId", Value: oid}})

		// Delete the analysis itself
		result, err := mongoDB.Analyses().DeleteOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleDeleteOptimization(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Delete all todos for this optimization
		mongoDB.Todos().DeleteMany(ctx, bson.D{{Key: "optimizationId", Value: oid}})

		// Delete the optimization itself
		result, err := mongoDB.Optimizations().DeleteOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// Domain Summary handlers

func buildDomainSummaryPrompt(domain string, optimizations []Optimization, analysis *Analysis, videoAnalysis *VideoAnalysis, redditAnalysis *RedditAnalysis, brandInfo BrandContextInfo) string {
	var sections strings.Builder

	// Site Analysis (summary only)
	if analysis != nil {
		sections.WriteString("\n=== SITE ANALYSIS ===\n")
		if analysis.Result.SiteSummary != "" {
			sections.WriteString("Site Summary: " + analysis.Result.SiteSummary + "\n")
		}
		sections.WriteString(fmt.Sprintf("Pages Crawled: %d\n", len(analysis.Result.CrawledPages)))
		sections.WriteString(fmt.Sprintf("Questions Discovered: %d\n", len(analysis.Result.Questions)))
	}

	// Optimization Reports (scores only)
	if len(optimizations) > 0 {
		sections.WriteString(fmt.Sprintf("\n=== OPTIMIZATION REPORTS (%d) ===\n", len(optimizations)))
		for i, opt := range optimizations {
			sections.WriteString(fmt.Sprintf("- Report %d: \"%s\" — Overall: %d, Content Authority: %d, Structural: %d, Source Authority: %d, Knowledge Persistence: %d\n",
				i+1, opt.Question, opt.Result.OverallScore,
				opt.Result.ContentAuthority.Score, opt.Result.StructuralOptimization.Score,
				opt.Result.SourceAuthority.Score, opt.Result.KnowledgePersistence.Score))
		}
	}

	// Video Authority (executive summary + score only)
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		vr := videoAnalysis.Result
		sections.WriteString("\n=== YOUTUBE VIDEO AUTHORITY ===\n")
		sections.WriteString(fmt.Sprintf("Overall Video Authority Score: %d/100\n", vr.OverallScore))
		if vr.ExecutiveSummary != "" {
			sections.WriteString("Summary: " + vr.ExecutiveSummary + "\n")
		}
	}

	// Reddit Authority (executive summary + score only)
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		rr := redditAnalysis.Result
		sections.WriteString("\n=== REDDIT AUTHORITY ===\n")
		sections.WriteString(fmt.Sprintf("Overall Reddit Authority Score: %d/100\n", rr.OverallScore))
		if rr.ExecutiveSummary != "" {
			sections.WriteString("Summary: " + rr.ExecutiveSummary + "\n")
		}
	}

	brandSection := ""
	if brandInfo.Used && brandInfo.ContextString != "" {
		brandSection = fmt.Sprintf("\n--- Brand Intelligence Context ---\n%s\n--- End Brand Context ---\n", brandInfo.ContextString)
	}

	// Build inventory of what's included
	var included []string
	if analysis != nil {
		included = append(included, "Site Analysis")
	}
	if len(optimizations) > 0 {
		included = append(included, fmt.Sprintf("%d Optimization Reports", len(optimizations)))
	}
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		included = append(included, "YouTube Video Authority")
	}
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		included = append(included, "Reddit Authority")
	}

	return fmt.Sprintf(`You are an LLM visibility strategist. You are given high-level summaries and scores from multiple report types for a single domain. Synthesize these into a unified strategic overview.

Domain: %s
Reports Included: %s
%s%s
INSTRUCTIONS:

1. **Executive Summary**: Write a 2-3 paragraph strategic overview of this domain's LLM visibility position across all available channels. Cover the biggest strengths, weaknesses, and overall trajectory. Weave together findings from all report types present.

2. **Themes**: Identify 3-5 recurring patterns across the reports. Reference sources by label (e.g., "Optimization Report 3", "Video Authority", "Reddit Authority", "Site Analysis"). Themes should span report types when possible.

3. **Priority Action Items**: Based on the score patterns and summaries, recommend 5-10 prioritized actions. Use priority levels: high, medium, low.

4. **Contradictions**: If different report summaries give conflicting signals, surface those explicitly with a recommendation on how to reconcile. If none, return an empty array.

5. **Dimension Trends**: Calculate the average score (0-100) for each optimization dimension across optimization reports. If no optimization reports exist, omit or use 0.

Return as JSON (no markdown code fences, just raw JSON):
{
  "executive_summary": "2-3 paragraph strategic overview covering all report types",
  "average_score": 65,
  "score_range": [40, 85],
  "themes": [
    {"title": "Theme name", "description": "What this means and why it matters", "report_refs": ["Optimization Report 1", "Video Authority"]}
  ],
  "action_items": [
    {"priority": "high", "action": "Specific action", "dimension": "content_authority", "expected_impact": "Expected improvement", "source_reports": ["Optimization Report 1", "Reddit Authority"]}
  ],
  "contradictions": [
    {"topic": "What is contradicted", "positions": ["Optimization reports say X", "Reddit analysis says Y"], "report_refs": ["Optimization Report 1", "Reddit Authority"], "recommendation": "How to reconcile"}
  ],
  "dimension_trends": {"content_authority": 60, "structural_optimization": 55, "source_authority": 70, "knowledge_persistence": 50}
}

If there are no contradictions, return an empty array for contradictions. Be specific and actionable.`, domain, strings.Join(included, ", "), sections.String(), brandSection)
}

// isSummaryStale checks if any data source has new data since the summary was generated.
func isSummaryStale(ctx context.Context, mongoDB *MongoDB, tenantCtx context.Context, domain string, summary DomainSummary) (bool, int64) {
	domainFilter := tenantFilter(tenantCtx, bson.D{{Key: "domain", Value: domain}})

	// Newer optimizations
	newerOpts, _ := mongoDB.Optimizations().CountDocuments(ctx, tenantFilter(tenantCtx, bson.D{
		{Key: "domain", Value: domain},
		{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}},
	}))
	if newerOpts > 0 {
		return true, newerOpts
	}

	// Newer site analysis
	newerAnalysis, _ := mongoDB.Analyses().CountDocuments(ctx, append(domainFilter, bson.E{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}}))
	if newerAnalysis > 0 {
		return true, newerAnalysis
	}

	// New video analysis that wasn't included
	if !summary.IncludesVideo {
		var va struct{ ID primitive.ObjectID `bson:"_id"` }
		if err := mongoDB.VideoAnalyses().FindOne(ctx, domainFilter, options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&va); err == nil {
			return true, 0
		}
	} else {
		newerVideo, _ := mongoDB.VideoAnalyses().CountDocuments(ctx, append(domainFilter, bson.E{Key: "generatedAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}}))
		if newerVideo > 0 {
			return true, 0
		}
	}

	// New Reddit analysis that wasn't included
	if !summary.IncludesReddit {
		var ra struct{ ID primitive.ObjectID `bson:"_id"` }
		if err := mongoDB.RedditAnalyses().FindOne(ctx, domainFilter, options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&ra); err == nil {
			return true, 0
		}
	} else {
		newerReddit, _ := mongoDB.RedditAnalyses().CountDocuments(ctx, append(domainFilter, bson.E{Key: "generatedAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}}))
		if newerReddit > 0 {
			return true, 0
		}
	}

	return false, 0
}

func handleDomainSummaryStatus(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		domainFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		var summary DomainSummary
		err := mongoDB.DomainSummaries().FindOne(ctx, domainFilter).Decode(&summary)

		if err == mongo.ErrNoDocuments {
			count, _ := mongoDB.Optimizations().CountDocuments(ctx, domainFilter)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"exists":             false,
				"total_report_count": count,
			})
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		totalCount, _ := mongoDB.Optimizations().CountDocuments(ctx, domainFilter)
		stale, newerCount := isSummaryStale(ctx, mongoDB, r.Context(), domain, summary)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"exists":             true,
			"generated_at":       summary.GeneratedAt,
			"included_count":     summary.ReportCount,
			"includes_analysis":  summary.IncludesAnalysis,
			"includes_video":     summary.IncludesVideo,
			"includes_reddit":    summary.IncludesReddit,
			"total_report_count": totalCount,
			"newer_report_count": newerCount,
			"stale":              stale,
		})
	}
}

func handleGetDomainSummary(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		domainFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		var summary DomainSummary
		err := mongoDB.DomainSummaries().FindOne(ctx, domainFilter).Decode(&summary)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		stale, newerCount := isSummaryStale(ctx, mongoDB, r.Context(), domain, summary)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"summary":            summary,
			"stale":              stale,
			"newer_report_count": newerCount,
		})
	}
}

func handleGenerateDomainSummary(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		domain := normalizeDomain(r.PathValue("domain"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		// Resolve primary LLM provider and API key for this tenant
		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey // used by provider.Stream below

		// Load all optimizations for this domain (max 30)
		dbCtx, dbCancel := context.WithTimeout(r.Context(), 15*time.Second)
		cursor, err := mongoDB.Optimizations().Find(dbCtx, tenantFilter(r.Context(), bson.D{
			{Key: "domain", Value: domain},
		}), options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(30))
		dbCancel()
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to load optimizations"})
			return
		}
		var optimizations []Optimization
		ctx2, cancel2 := context.WithTimeout(r.Context(), 15*time.Second)
		if err := cursor.All(ctx2, &optimizations); err != nil {
			cancel2()
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to read optimizations"})
			return
		}
		cancel2()

		// Load site analysis
		var analysis *Analysis
		var an Analysis
		anCtx, anCancel := context.WithTimeout(r.Context(), 10*time.Second)
		anFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		anOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.Analyses().FindOne(anCtx, anFilter, anOpts).Decode(&an); err == nil {
			analysis = &an
		}
		anCancel()

		// Load video analysis
		var videoAnalysis *VideoAnalysis
		var va VideoAnalysis
		vaCtx, vaCancel := context.WithTimeout(r.Context(), 10*time.Second)
		vaFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		vaOpts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.VideoAnalyses().FindOne(vaCtx, vaFilter, vaOpts).Decode(&va); err == nil {
			videoAnalysis = &va
		}
		vaCancel()

		// Load Reddit analysis
		var redditAnalysis *RedditAnalysis
		var ra RedditAnalysis
		raCtx, raCancel := context.WithTimeout(r.Context(), 10*time.Second)
		raFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		raOpts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.RedditAnalyses().FindOne(raCtx, raFilter, raOpts).Decode(&ra); err == nil {
			redditAnalysis = &ra
		}
		raCancel()

		hasVideo := videoAnalysis != nil && videoAnalysis.Result != nil
		hasReddit := redditAnalysis != nil && redditAnalysis.Result != nil

		if len(optimizations) == 0 && analysis == nil && !hasVideo && !hasReddit {
			sendSSE(w, flusher, "error", map[string]string{"message": "No reports found for this domain"})
			return
		}

		brandInfo := lookupBrandContext(mongoDB, domain, saas.TenantIDFromContext(r.Context()))

		// Build status message
		var parts []string
		if len(optimizations) > 0 {
			parts = append(parts, fmt.Sprintf("%d optimization reports", len(optimizations)))
		}
		if analysis != nil {
			parts = append(parts, "site analysis")
		}
		if hasVideo {
			parts = append(parts, "video authority")
		}
		if hasReddit {
			parts = append(parts, "Reddit authority")
		}
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Synthesizing %s for %s...", strings.Join(parts, ", "), domain),
		})

		prompt := buildDomainSummaryPrompt(domain, optimizations, analysis, videoAnalysis, redditAnalysis, brandInfo)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			// No tools needed — pure synthesis of existing data
			claudeBody, _ := provider.BuildStreamBody(model.ID, 8192, prompt, false)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					code := classifyError(err)
					sendSSEError(w, flusher, code)
					saveFailedAnalysis(mongoDB, r.Context(), domain, "summary", code, model.Name)
					return
				}

				// Parse and save the summary
				cleanJSON := stripJSONFencing(result.ResultJSON)
				var summaryResult DomainSummaryResult
				if err := json.Unmarshal([]byte(cleanJSON), &summaryResult); err != nil {
					sendSSEError(w, flusher, ErrCodeParseError)
					saveFailedAnalysis(mongoDB, r.Context(), domain, "summary", ErrCodeParseError, model.Name)
					return
				}

				optIDs := make([]primitive.ObjectID, len(optimizations))
				for i, o := range optimizations {
					optIDs[i] = o.ID
				}

				summary := DomainSummary{
					Domain:           domain,
					TenantID:         saas.TenantIDFromContext(r.Context()),
					Result:           summaryResult,
					RawText:          result.RawText,
					Model:            model.Name,
					OptimizationIDs:  optIDs,
					ReportCount:      len(optimizations),
					IncludesAnalysis: analysis != nil,
					IncludesVideo:    hasVideo,
					IncludesReddit:   hasReddit,
					GeneratedAt:      time.Now(),
				}

				saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
				_, saveErr := mongoDB.DomainSummaries().ReplaceOne(saveCtx,
					tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
					summary,
					options.Replace().SetUpsert(true),
				)
				saveCancel()
				if saveErr != nil {
					log.Printf("Failed to save domain summary: %v", saveErr)
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result":            result.ResultJSON,
					"model":             model.Name,
					"generated_at":      summary.GeneratedAt,
					"report_count":      summary.ReportCount,
					"includes_analysis": summary.IncludesAnalysis,
					"includes_video":    summary.IncludesVideo,
					"includes_reddit":   summary.IncludesReddit,
					"domain":            domain,
				})
				trackServerEvent(mongoDB, "custom.server.summary.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": domain, "duration_ms": time.Since(startTime).Milliseconds()})
				return
			}

			log.Printf("%s API (%s) exhausted retries for domain summary: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
		saveFailedAnalysis(mongoDB, r.Context(), domain, "summary", ErrCodeAPIOverloaded, "")
	}
}

func handleDiscoverCompetitors(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context (optional)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil && brand.BrandName != "" {
			brandContext = fmt.Sprintf("\nKnown brand name: %s\nDescription: %s\nCategories: %s\n",
				brand.BrandName, brand.Description, strings.Join(brand.Categories, ", "))
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Discovering competitors for " + domain + "...",
		})

		prompt := fmt.Sprintf(`You are a competitive intelligence analyst. Your task is to discover competitors for a given website/brand.

Website domain: %s%s

Follow this discovery process:

1. **Site Analysis**: Search for and visit %s to understand what they do, their product category, and market positioning.

2. **Search-Based Discovery**: Run these types of searches:
   - "[product category] alternatives"
   - "[brand name] vs"
   - "best [category] tools/software/services"
   - "[brand name] competitors"
   - Look at results from G2, Capterra, TrustRadius, and similar review/comparison sites

3. **LLM Knowledge Probe**: Based on your own knowledge, who are the main competitors? This reveals what's already in LLM training data.

4. **Cross-Reference**: For each competitor found, note where you found them (search results, review sites, or your own knowledge).

Identify 5-15 competitors. For each, determine their relationship to the target brand.

Return your findings as JSON (no markdown code fences, just raw JSON):
{
  "competitors": [
    {
      "name": "Competitor Name",
      "url": "https://competitor.com",
      "relationship": "direct",
      "source": "search",
      "confidence": 0.9,
      "notes": "Brief note about why they're a competitor"
    }
  ]
}

- relationship: "direct" (same category), "indirect" (overlapping use cases), "aspirational" (larger/successful peer), or "adjacent" (complementary product)
- source: "search" (found in web search results), "review_site" (found on G2/Capterra/etc), "llm_knowledge" (from your training data), or "multiple" (found in multiple places)
- confidence: 0.0-1.0 how confident you are this is a real competitor`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSEError(w, flusher, classifyError(err))
					return
				}

				// Send results (not saved to DB — user reviews first)
				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for competitor discovery: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
	}
}

func handleSuggestQueries(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load brand profile for context
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		if brandErr != nil {
			sendSSE(w, flusher, "error", map[string]string{
				"message": "Brand profile not found. Please save basic brand info first.",
			})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Generating query suggestions for " + brand.BrandName + "...",
		})

		competitorNames := make([]string, len(brand.Competitors))
		for i, c := range brand.Competitors {
			competitorNames[i] = c.Name
		}

		prompt := fmt.Sprintf(`You are an LLM visibility strategist. Generate target search queries that a brand should optimize for in LLM responses.

Brand: %s
Website: %s
Description: %s
Categories: %s
Products/Features: %s
Target Audience: %s
Key Use Cases: %s
Known Competitors: %s

Generate 20-30 target queries organized by type. These are queries where the brand wants to appear in LLM-generated responses.

Query types:
- **brand**: Queries that include the brand name (e.g., "[brand] review", "is [brand] good?", "[brand] pricing")
- **category**: Generic queries about the product category (e.g., "best [category] tools", "top [category] software 2025")
- **comparison**: Head-to-head queries (e.g., "[brand] vs [competitor]" for each major competitor)
- **problem**: Problem/need-oriented queries where the brand's product is the answer (e.g., "how to [use case]", "best way to [solve problem]")

For each query, assign a priority:
- **high**: Core business queries, high commercial intent
- **medium**: Important for visibility but less direct
- **low**: Nice to have, exploratory queries

Return as JSON (no markdown code fences, just raw JSON):
{
  "queries": [
    {
      "query": "The actual search query text",
      "priority": "high",
      "type": "category"
    }
  ]
}`, brand.BrandName, domain, brand.Description,
			strings.Join(brand.Categories, ", "),
			strings.Join(brand.Products, ", "),
			brand.PrimaryAudience,
			strings.Join(brand.KeyUseCases, ", "),
			strings.Join(competitorNames, ", "))

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, false)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSEError(w, flusher, classifyError(err))
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for query suggestion: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
	}
}

func handleGenerateDescription(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing " + domain + " to generate description...",
		})

		prompt := fmt.Sprintf(`You are a brand analyst. Visit and analyze the website at %s to produce a concise brand description.

1. Search for and visit the homepage and key pages of %s
2. Understand what the company does, who it serves, and what makes it distinctive
3. Write a clear, factual 2-3 sentence description

Return as JSON (no markdown code fences, just raw JSON):
{
  "description": "The 2-3 sentence brand description",
  "brand_name": "The company/brand name as it appears on the site",
  "categories": ["category1", "category2"],
  "products": ["product1", "feature1"]
}`, domain, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 4096, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSEError(w, flusher, classifyError(err))
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for description generation: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
	}
}

func handlePredictAudience(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil {
			if brand.BrandName != "" {
				brandContext += fmt.Sprintf("\nBrand: %s\n", brand.BrandName)
			}
			if brand.Description != "" {
				brandContext += fmt.Sprintf("Description: %s\n", brand.Description)
			}
			if len(brand.Categories) > 0 {
				brandContext += fmt.Sprintf("Categories: %s\n", strings.Join(brand.Categories, ", "))
			}
			if len(brand.Products) > 0 {
				brandContext += fmt.Sprintf("Products: %s\n", strings.Join(brand.Products, ", "))
			}
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing " + domain + " to predict target audience...",
		})

		prompt := fmt.Sprintf(`You are a brand strategist. Visit and analyze the website at %s to determine who the target audience is and what key use cases the product/service addresses.
%s
Steps:
1. Search for and visit the homepage and key pages of %s
2. Identify the primary target demographic — roles, industries, company sizes, or consumer segments
3. Identify specific use cases, problems solved, and jobs-to-be-done
4. Be specific rather than generic — mention actual roles, industries, or scenarios

Return as JSON (no markdown code fences, just raw JSON):
{
  "primary_audience": "A specific 2-3 sentence description of the primary target audience",
  "key_use_cases": ["specific use case 1", "specific use case 2", "..."]
}`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 4096, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSEError(w, flusher, classifyError(err))
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for audience prediction: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
	}
}

func handleSuggestClaims(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil {
			if brand.BrandName != "" {
				brandContext += fmt.Sprintf("\nBrand: %s\n", brand.BrandName)
			}
			if brand.Description != "" {
				brandContext += fmt.Sprintf("Description: %s\n", brand.Description)
			}
			if len(brand.Categories) > 0 {
				brandContext += fmt.Sprintf("Categories: %s\n", strings.Join(brand.Categories, ", "))
			}
			if len(brand.Products) > 0 {
				brandContext += fmt.Sprintf("Products: %s\n", strings.Join(brand.Products, ", "))
			}
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Searching " + domain + " for brand claims and proof points...",
		})

		prompt := fmt.Sprintf(`You are a brand analyst. Visit and analyze the website at %s to discover the brand's key claims, value propositions, proof points, and statistics.
%s
Steps:
1. Search for and visit the homepage, about page, product pages, and any case study or testimonial pages on %s
2. Look for: factual claims (e.g., "Used by 10,000+ teams"), value propositions, statistics, awards, certifications, customer proof points
3. Identify which claims are backed by evidence on specific pages
4. Assign priority based on how prominently the claim is featured

Return as JSON (no markdown code fences, just raw JSON):
{
  "claims": [
    {
      "claim": "The specific claim text",
      "evidence_url": "URL where this claim appears",
      "priority": "high|medium|low"
    }
  ]
}

Include 5-15 claims, prioritizing the most prominent and verifiable ones.`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 8192, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSEError(w, flusher, classifyError(err))
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for claim suggestion: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
	}
}

func handlePredictDifferentiators(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context (including competitors)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil {
			if brand.BrandName != "" {
				brandContext += fmt.Sprintf("\nBrand: %s\n", brand.BrandName)
			}
			if brand.Description != "" {
				brandContext += fmt.Sprintf("Description: %s\n", brand.Description)
			}
			if len(brand.Categories) > 0 {
				brandContext += fmt.Sprintf("Categories: %s\n", strings.Join(brand.Categories, ", "))
			}
			if len(brand.Products) > 0 {
				brandContext += fmt.Sprintf("Products: %s\n", strings.Join(brand.Products, ", "))
			}
			if len(brand.Competitors) > 0 {
				names := make([]string, len(brand.Competitors))
				for i, c := range brand.Competitors {
					names[i] = c.Name
				}
				brandContext += fmt.Sprintf("Known competitors: %s\n", strings.Join(names, ", "))
			}
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing " + domain + " to identify differentiators...",
		})

		prompt := fmt.Sprintf(`You are a competitive analyst. Visit and analyze the website at %s to identify what makes this brand unique compared to competitors.
%s
Steps:
1. Search for and visit the homepage, product pages, and comparison/features pages on %s
2. Identify unique features, approaches, technologies, or positioning that set this brand apart
3. If competitors are known, compare against them to find true differentiators
4. Focus on tangible, specific differentiators rather than generic marketing language

STRICT FORMAT RULES:
- Each differentiator MUST be 2-5 words. Maximum 5 words. No exceptions.
- NEVER use commas within a differentiator phrase. Use hyphens or "and" instead.
- Think of these as tags or badges, not descriptions or sentences.

Good: "AI-powered automation" "No per-seat pricing" "Open-source core" "Real-time collaboration" "Enterprise-grade security"
Bad: "Uses advanced AI to automate workflows" (too long)
Bad: "Flexible, scalable architecture" (contains comma)

Return as JSON (no markdown code fences, just raw JSON):
{
  "differentiators": ["phrase 1", "phrase 2", "phrase 3"]
}

Include 5-12 differentiators, ordered by distinctiveness.`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 8192, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSEError(w, flusher, classifyError(err))
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for differentiator prediction: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSEError(w, flusher, ErrCodeAPIOverloaded)
	}
}

// ── Video Authority Analyzer Handlers ────────────────────────────────────

func handleVideoDiscover(mongoDB *MongoDB, encKey []byte, systemYTKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		ytKey, err := resolveYouTubeKey(r.Context(), mongoDB, encKey, systemYTKey, saasEnabled)
		if err != nil {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, ok := w.(http.Flusher)
			if ok {
				sendSSE(w, flusher, "error", map[string]string{"message": "YouTube API key required. Add your key in Settings → API Keys."})
			} else {
				http.Error(w, `{"error":"YouTube API key required"}`, http.StatusServiceUnavailable)
			}
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		var req struct {
			Domain               string   `json:"domain"`
			BrandName            string   `json:"brand_name"`
			ChannelURL           string   `json:"channel_url"`
			SearchTerms          []string `json:"search_terms"`
			Competitors          []string `json:"competitors"`
			IncludeBrandSearches *bool    `json:"include_brand_searches,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}
		includeBrandSearches := true // default on (opt-out)
		if req.IncludeBrandSearches != nil {
			includeBrandSearches = *req.IncludeBrandSearches
		}

		progress := func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		}

		// Fetch brand profile for category data (auto-generated video searches)
		var categories, keyUseCases []string
		if req.Domain != "" {
			bCtx, bCancel := context.WithTimeout(r.Context(), 5*time.Second)
			var brand BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(bCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: normalizeDomain(req.Domain)}})).Decode(&brand); err == nil {
				categories = brand.Categories
				keyUseCases = brand.KeyUseCases
			}
			bCancel()
		}

		videos, quotaUsed, err := discoverVideos(mongoDB, ytKey, req.BrandName, req.ChannelURL, req.SearchTerms, req.Competitors, categories, keyUseCases, includeBrandSearches, progress)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
			return
		}

		// Strip videos to lightweight summaries for the SSE payload
		type videoSummary struct {
			VideoID      string    `json:"video_id"`
			Title        string    `json:"title"`
			ChannelTitle string    `json:"channel_title"`
			ChannelID    string    `json:"channel_id"`
			PublishedAt  time.Time `json:"published_at"`
			ViewCount    int64     `json:"view_count"`
			Duration     string    `json:"duration"`
			RelevanceTag string    `json:"relevance_tag"`
		}
		summaries := make([]videoSummary, len(videos))
		for i, v := range videos {
			summaries[i] = videoSummary{
				VideoID:      v.VideoID,
				Title:        v.Title,
				ChannelTitle: v.ChannelTitle,
				ChannelID:    v.ChannelID,
				PublishedAt:  v.PublishedAt,
				ViewCount:    v.ViewCount,
				Duration:     v.Duration,
				RelevanceTag: v.RelevanceTag,
			}
		}
		sendSSE(w, flusher, "done", map[string]any{
			"videos":         summaries,
			"quota_estimate": quotaUsed,
		})
		trackServerEvent(mongoDB, "custom.server.video_discover.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": req.Domain, "duration_ms": time.Since(startTime).Milliseconds(), "video_count": len(summaries)})
	}
}

// BatchDigest is a compact summary of a batch of third-party video assessments,
// produced by the small model (Haiku) in Phase 2a. Fed into Sonnet for final synthesis.
type BatchDigest struct {
	BatchIndex     int            `json:"batch_index"`
	VideoCount     int            `json:"video_count"`
	TopCreators    []string       `json:"top_creators"`
	TopicsCovered  []string       `json:"topics_covered"`
	SentimentTally map[string]int `json:"sentiment_tally"`
	NotableQuotes  []string       `json:"notable_quotes"`
	ContentGaps    []string       `json:"content_gaps"`
	Summary        string         `json:"summary"`
}

// digestVideoBatch takes a batch of third-party video assessments and asks the small model
// to produce a compact digest summarizing brand mentions, sentiment, creators, and gaps.
func digestVideoBatch(ctx context.Context, provider LLMProvider, apiKey string, videos []YouTubeVideo, assessments map[string]*VideoAssessment, domain string, searchTerms []string, batchIdx int) (*BatchDigest, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summarize this batch of %d third-party YouTube video assessments for brand authority analysis.\n\n", len(videos)))
	sb.WriteString(fmt.Sprintf("Brand/Domain: %s\nTarget Search Terms: %s\n\n", domain, strings.Join(searchTerms, ", ")))

	for i, v := range videos {
		sb.WriteString(fmt.Sprintf("--- Video %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Title: %s\nChannel: %s\nViews: %d | Published: %s\n", v.Title, v.ChannelTitle, v.ViewCount, v.PublishedAt.Format("2006-01-02")))
		a := assessments[v.VideoID]
		if a != nil && a.HasTranscript {
			sb.WriteString(fmt.Sprintf("Scores: keyword=%d, quotability=%d, density=%d\n", a.KeywordAlignment, a.Quotability, a.InfoDensity))
			if len(a.KeyQuotes) > 0 {
				sb.WriteString(fmt.Sprintf("Quotes: \"%s\"\n", strings.Join(a.KeyQuotes, "\" | \"")))
			}
			if len(a.Topics) > 0 {
				sb.WriteString(fmt.Sprintf("Topics: %s\n", strings.Join(a.Topics, ", ")))
			}
			sb.WriteString(fmt.Sprintf("Sentiment: %s\n", a.BrandSentiment))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", a.Summary))
		} else {
			sb.WriteString("Transcript: [NOT AVAILABLE]\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf(`Produce a compact digest of these %d videos. Return ONLY valid JSON:
{
  "top_creators": ["channel names of the most authoritative/relevant creators in this batch"],
  "topics_covered": ["main topics across this batch"],
  "sentiment_tally": {"positive": N, "neutral": N, "negative": N, "none": N},
  "notable_quotes": ["2-4 most citable quotes mentioning %s from any video"],
  "content_gaps": ["topics where %s is absent but competitors are discussed"],
  "summary": "2-3 sentence overview of what this batch reveals about %s's video landscape"
}`, len(videos), domain, domain, domain))

	const maxRetries = 2
	backoff := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			backoff *= 2
		}

		text, err := provider.Call(ctx, apiKey, provider.SmallModel(), sb.String(), 2048)
		if err == errOverloaded {
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("small model overloaded after %d retries", maxRetries)
		}
		if err != nil {
			return nil, err
		}

		text = strings.TrimSpace(text)
		if idx := strings.Index(text, "{"); idx >= 0 {
			if end := strings.LastIndex(text, "}"); end > idx {
				text = text[idx : end+1]
			}
		}

		var d BatchDigest
		if err := json.Unmarshal([]byte(text), &d); err != nil {
			return nil, fmt.Errorf("failed to parse batch digest: %w", err)
		}
		d.BatchIndex = batchIdx
		d.VideoCount = len(videos)
		return &d, nil
	}
	return nil, fmt.Errorf("exhausted retries for batch digest")
}

// digestThirdPartyVideos splits third-party videos into batches and produces compact digests concurrently.
func digestThirdPartyVideos(ctx context.Context, provider LLMProvider, apiKey string, videos []YouTubeVideo, assessments map[string]*VideoAssessment, domain string, searchTerms []string, w http.ResponseWriter, flusher http.Flusher) ([]BatchDigest, error) {
	const batchSize = 40
	const workers = 4

	numBatches := (len(videos) + batchSize - 1) / batchSize
	batches := make([][]YouTubeVideo, 0, numBatches)
	for i := 0; i < len(videos); i += batchSize {
		end := i + batchSize
		if end > len(videos) {
			end = len(videos)
		}
		batches = append(batches, videos[i:end])
	}

	type digestResult struct {
		idx    int
		digest *BatchDigest
		err    error
	}

	resultsCh := make(chan digestResult, len(batches))
	sem := make(chan struct{}, workers)

	for i, batch := range batches {
		go func(idx int, vids []YouTubeVideo) {
			sem <- struct{}{}
			defer func() { <-sem }()

			d, err := digestVideoBatch(ctx, provider, apiKey, vids, assessments, domain, searchTerms, idx)
			resultsCh <- digestResult{idx: idx, digest: d, err: err}
		}(i, batch)
	}

	digests := make([]BatchDigest, len(batches))
	var firstErr error
	for i := 0; i < len(batches); i++ {
		r := <-resultsCh
		if r.err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("batch %d: %w", r.idx, r.err)
			}
			log.Printf("Warning: batch digest %d failed: %v", r.idx, r.err)
		} else if r.digest != nil {
			digests[r.idx] = *r.digest
		}
		sendSSE(w, flusher, "progress", map[string]string{
			"message": fmt.Sprintf("Digesting third-party videos (%d/%d batches)...", i+1, len(batches)),
		})
	}

	// Allow partial success — if at least half completed, proceed
	completedCount := 0
	for _, d := range digests {
		if d.VideoCount > 0 {
			completedCount++
		}
	}
	if completedCount == 0 && firstErr != nil {
		return nil, firstErr
	}

	// Filter out empty digests
	var result []BatchDigest
	for _, d := range digests {
		if d.VideoCount > 0 {
			result = append(result, d)
		}
	}
	return result, nil
}

// videoContextParams holds the shared context for all pillar prompts.
type videoContextParams struct {
	Domain          string
	BrandName       string
	SearchTerms     []string
	Competitors     []string
	BrandInfo       BrandContextInfo
	OwnVideos       []YouTubeVideo
	ThirdPartyCount int
	Digests         []BatchDigest
	Assessments     map[string]*VideoAssessment
}

func newVideoContextParams(domain string, ownVideos []YouTubeVideo, thirdPartyCount int, digests []BatchDigest, competitors, searchTerms []string, brandInfo BrandContextInfo, assessments map[string]*VideoAssessment) videoContextParams {
	brandName := domain
	if brandInfo.Used {
		for _, line := range strings.Split(brandInfo.ContextString, "\n") {
			if strings.HasPrefix(line, "Company: ") {
				brandName = strings.TrimPrefix(line, "Company: ")
				break
			}
		}
	}
	return videoContextParams{
		Domain:          domain,
		BrandName:       brandName,
		SearchTerms:     searchTerms,
		Competitors:     competitors,
		BrandInfo:       brandInfo,
		OwnVideos:       ownVideos,
		ThirdPartyCount: thirdPartyCount,
		Digests:         digests,
		Assessments:     assessments,
	}
}

// writeVideoAssessment writes a single video's assessment data to a string builder.
func writeVideoAssessment(sb *strings.Builder, v YouTubeVideo, assessments map[string]*VideoAssessment) {
	a := assessments[v.VideoID]
	if a != nil && a.HasTranscript {
		sb.WriteString(fmt.Sprintf("Assessment: keyword_alignment=%d, quotability=%d, info_density=%d\n", a.KeywordAlignment, a.Quotability, a.InfoDensity))
		if len(a.KeyQuotes) > 0 {
			sb.WriteString(fmt.Sprintf("Key Quotes: \"%s\"\n", strings.Join(a.KeyQuotes, "\" | \"")))
		}
		if len(a.Topics) > 0 {
			sb.WriteString(fmt.Sprintf("Topics: %s\n", strings.Join(a.Topics, ", ")))
		}
		sb.WriteString(fmt.Sprintf("Brand Sentiment: %s\n", a.BrandSentiment))
		sb.WriteString(fmt.Sprintf("Summary: %s\n", a.Summary))
	} else if v.Transcript != "" {
		transcript := v.Transcript
		if len(transcript) > 1000 {
			transcript = transcript[:1000] + "... [truncated]"
		}
		sb.WriteString(fmt.Sprintf("Transcript (raw): %s\n", transcript))
	} else {
		sb.WriteString("Transcript: [NOT AVAILABLE — invisible to LLMs]\n")
		if v.Description != "" {
			desc := v.Description
			if len(desc) > 500 {
				desc = desc[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("Description: %s\n", desc))
		}
	}
}

// writePreamble writes the shared context header for any pillar prompt.
func writePreamble(sb *strings.Builder, p videoContextParams) {
	sb.WriteString(fmt.Sprintf(`You are an expert in Video LLM Authority analysis.

Brand: %s | Domain: %s
Target Search Terms: %s
Known Competitors: %s
`, p.BrandName, p.Domain, strings.Join(p.SearchTerms, ", "), strings.Join(p.Competitors, ", ")))
	if p.BrandInfo.Used {
		sb.WriteString(p.BrandInfo.ContextString)
		sb.WriteString("\n")
	}
}

// writeOwnChannelVideos writes the own-channel video data section.
func writeOwnChannelVideos(sb *strings.Builder, p videoContextParams) {
	if len(p.OwnVideos) > 0 {
		sb.WriteString(fmt.Sprintf("\n=== OWN CHANNEL VIDEOS (%d) ===\n\n", len(p.OwnVideos)))
		for i, v := range p.OwnVideos {
			sb.WriteString(fmt.Sprintf("--- Own Video %d ---\n", i+1))
			sb.WriteString(fmt.Sprintf("Title: %s | Video ID: %s\n", v.Title, v.VideoID))
			sb.WriteString(fmt.Sprintf("Views: %d | Likes: %d | Comments: %d\n", v.ViewCount, v.LikeCount, v.CommentCount))
			sb.WriteString(fmt.Sprintf("Duration: %s | Published: %s\n", v.Duration, v.PublishedAt.Format("2006-01-02")))
			if v.Description != "" {
				desc := v.Description
				if len(desc) > 500 {
					desc = desc[:500] + "..."
				}
				sb.WriteString(fmt.Sprintf("Description: %s\n", desc))
			}
			if len(v.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(v.Tags, ", ")))
			}
			writeVideoAssessment(sb, v, p.Assessments)
			sb.WriteString("\n")
		}
	}
}

// writeDigests writes the third-party batch digest section.
func writeDigests(sb *strings.Builder, p videoContextParams) {
	if len(p.Digests) > 0 {
		sb.WriteString(fmt.Sprintf("\n=== THIRD-PARTY VIDEO DIGESTS (%d videos, %d batches) ===\n\n", p.ThirdPartyCount, len(p.Digests)))
		for _, d := range p.Digests {
			sb.WriteString(fmt.Sprintf("--- Batch %d (%d videos) ---\n", d.BatchIndex+1, d.VideoCount))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", d.Summary))
			if len(d.TopCreators) > 0 {
				sb.WriteString(fmt.Sprintf("Top Creators: %s\n", strings.Join(d.TopCreators, ", ")))
			}
			if len(d.TopicsCovered) > 0 {
				sb.WriteString(fmt.Sprintf("Topics: %s\n", strings.Join(d.TopicsCovered, ", ")))
			}
			if d.SentimentTally != nil {
				sb.WriteString(fmt.Sprintf("Sentiment: positive=%d, neutral=%d, negative=%d, none=%d\n",
					d.SentimentTally["positive"], d.SentimentTally["neutral"], d.SentimentTally["negative"], d.SentimentTally["none"]))
			}
			if len(d.NotableQuotes) > 0 {
				sb.WriteString(fmt.Sprintf("Notable Quotes: \"%s\"\n", strings.Join(d.NotableQuotes, "\" | \"")))
			}
			if len(d.ContentGaps) > 0 {
				sb.WriteString(fmt.Sprintf("Content Gaps: %s\n", strings.Join(d.ContentGaps, "; ")))
			}
			sb.WriteString("\n")
		}
	}
}

// buildPillar1Prompt builds the Transcript Authority + Video Scorecards prompt.
func buildPillar1Prompt(p videoContextParams) string {
	var sb strings.Builder
	writePreamble(&sb, p)
	writeOwnChannelVideos(&sb, p)
	sb.WriteString(fmt.Sprintf(`
Analyze the own-channel videos above for TRANSCRIPT AUTHORITY — how well the brand's spoken content signals expertise to LLMs.

LLMs consume transcripts, not video. Quotation-ready content gets +41%%%% visibility; statistics +33%%%%. U-shaped attention: beginning and end of transcripts get disproportionate weight.

Evaluate:
- Keyword alignment: target search terms in spoken words?
- Quotability: standalone citable statements?
- Statistical evidence: specific numbers/benchmarks?
- Information density: focused vs. rambling?
- Front-loading: key claims in first 20%%%% of transcript?
- Entity explicitness: brand name spoken clearly?
- CRITICAL: No transcript = cap contribution at 10.

Also produce a scorecard for EACH own-channel video:
- transcript_power (45%%%%): spoken content quality as LLM training data
- structural_extractability (30%%%%): how easily LLMs can parse it
- discovery_surface (25%%%%): findability by AI retrieval
- overall_score = transcript_power * 0.45 + structural_extractability * 0.30 + discovery_surface * 0.25

Return ONLY valid JSON:
{
  "transcript_authority": {
    "score": 65, "evidence": ["...", "..."],
    "transcript_coverage": 80, "keyword_alignment": 55, "quotability_score": 60, "information_density": 70
  },
  "video_scorecards": [
    {"video_id": "...", "title": "...", "overall_score": 70, "transcript_power": 60, "structural_extractability": 75, "discovery_surface": 80, "has_transcript": true, "key_findings": ["...", "..."]}
  ]
}`))
	return sb.String()
}

// buildPillar2Prompt builds the Topical Dominance prompt.
func buildPillar2Prompt(p videoContextParams) string {
	var sb strings.Builder
	writePreamble(&sb, p)
	writeOwnChannelVideos(&sb, p)
	writeDigests(&sb, p)
	sb.WriteString(fmt.Sprintf(`
Analyze ALL video data above for TOPICAL DOMINANCE — how comprehensively the brand owns key topic areas vs. competitors.

Evaluate:
- Topics covered vs. total topics in the space
- Coverage depth: surface mentions vs. in-depth treatment
- Share of voice: %%%% of videos mentioning each brand
- Content gaps: topics where competitors are present but %s is absent (score each 0-100)
- First-mover opportunities in unclaimed territory

Return ONLY valid JSON:
{
  "topical_dominance": {
    "score": 50, "evidence": ["...", "..."],
    "topics_covered": 4, "topics_total": 8, "coverage_depth": 55, "vs_competitors": "...",
    "share_of_voice": [{"brand_name": "X", "mention_count": 10, "percentage": 25.0}],
    "content_gaps": [{"query": "...", "competitors_mentioned": ["A"], "opportunity_score": 80, "video_count": 5, "recommendation": "..."}]
  }
}`, p.BrandName))
	return sb.String()
}

// buildPillar3Prompt builds the Citation Network prompt.
func buildPillar3Prompt(p videoContextParams) string {
	var sb strings.Builder
	writePreamble(&sb, p)
	writeDigests(&sb, p)
	sb.WriteString(`
Analyze the third-party video digests for CITATION NETWORK — how connected and referenced the brand is by other creators.

Views and subscriber counts do NOT predict AI citation. Structural factors matter most.

Evaluate:
- Creator authority (0-100) based on transcript quality, topical consistency — NOT subscriber count
- Each creator's role: advocate/critic/neutral
- Concentration risk: is narrative dominated by 1-2 creators?
- High-authority creators who cover competitors but NOT the brand (outreach targets)

Return ONLY valid JSON:
{
  "citation_network": {
    "score": 45, "evidence": ["...", "..."],
    "creator_mentions": 8, "authoritative_refs": 3, "concentration_risk": "...",
    "top_creators": [{"channel_title": "...", "channel_id": "...", "subscriber_count": 100000, "sentiment": "positive", "video_count": 2, "total_views": 50000, "role": "advocate", "authority_score": 75}],
    "creator_targets": [{"channel_title": "...", "channel_id": "...", "subscriber_count": 500000, "category_relevance": "...", "competitors_mentioned": ["A"], "outreach_reason": "..."}]
  }
}`)
	return sb.String()
}

// buildPillar4Prompt builds the Brand Narrative Quality prompt.
func buildPillar4Prompt(p videoContextParams) string {
	var sb strings.Builder
	writePreamble(&sb, p)
	writeDigests(&sb, p)
	sb.WriteString(fmt.Sprintf(`
Analyze the third-party video digests for BRAND NARRATIVE QUALITY — how %s is framed when it appears in third-party content.

Perplexity generates one-sided answers 83.4%%%% of the time — negative patterns get amplified. Citation accuracy is only 49-68%%%%.

Evaluate:
- For brand mentions: sentiment, mention_context (recommendation/tutorial/comparison/complaint/passing), mention_position (early/middle/late), extractability (high/medium/low)
- Weight early + high-extractability mentions higher (U-shaped attention)
- Apply 30%%%% confidence discount for LLM narrative divergence
- Narrative coherence: consistent or contradictory?
- Vulnerability assessment: negative patterns LLMs would amplify?

Return ONLY valid JSON:
{
  "brand_narrative": {
    "score": 62, "evidence": ["...", "..."],
    "sentiment": {"positive": 6, "neutral": 3, "negative": 1, "total": 10},
    "narrative_summary": "Based on the video evidence...",
    "narrative_coherence": 70, "key_themes": ["Theme 1", "Theme 2"],
    "brand_mentions": [{"video_id": "...", "title": "...", "channel_title": "...", "view_count": 50000, "sentiment": "positive", "mention_context": "Recommended as top pick", "mention_position": "early", "extractability": "high", "competitors_mentioned": ["A"]}]
  }
}`, p.BrandName))
	return sb.String()
}

// buildSynthesisPrompt takes the 4 pillar JSON results and produces overall score + executive summary + recommendations.
func buildSynthesisPrompt(brandName string, pillar1, pillar2, pillar3, pillar4 string) string {
	return fmt.Sprintf(`You are synthesizing a Video LLM Authority report for %s from 4 independently-analyzed pillars.

PILLAR RESULTS:
%s
%s
%s
%s

TASK:
1. Calculate overall_score = transcript_authority.score * 0.30 + topical_dominance.score * 0.25 + citation_network.score * 0.25 + brand_narrative.score * 0.20
2. Write executive_summary: 2-3 paragraph strategic overview of the brand's video LLM authority position, drawing on all 4 pillars.
3. Write confidence_note: explicit statement about citation accuracy limitations (49-68%%) and what they mean for this brand.
4. Provide 5-12 structured recommendations spanning all 4 dimensions.

Return ONLY valid JSON:
{
  "overall_score": 58,
  "executive_summary": "...",
  "confidence_note": "...",
  "recommendations": [
    {"action": "...", "expected_impact": "...", "dimension": "transcript_authority", "priority": "high", "video_id": "..."}
  ]
}`, brandName, pillar1, pillar2, pillar3, pillar4)
}

func handleVideoAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool, systemYTKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		ytKey, err := resolveYouTubeKey(r.Context(), mongoDB, encKey, systemYTKey, saasEnabled)
		if err != nil {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, ok := w.(http.Flusher)
			if ok {
				sendSSE(w, flusher, "error", map[string]string{"message": "YouTube API key required. Add your key in Settings → API Keys."})
			} else {
				http.Error(w, `{"error":"YouTube API key required"}`, http.StatusServiceUnavailable)
			}
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		var req struct {
			Domain           string              `json:"domain"`
			Config           VideoAnalysisConfig `json:"config"`
			SelectedVideoIDs []string            `json:"selected_video_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}

		if len(req.SelectedVideoIDs) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "No videos selected for analysis"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)

		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))

		// Phase 1: Gather video data
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Fetching metadata for %d videos...", len(req.SelectedVideoIDs)),
		})

		videos, err := cachedVideoDetails(mongoDB, ytKey, req.SelectedVideoIDs)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to fetch video details: " + err.Error()})
			return
		}

		// Fetch transcripts with adaptive backoff on rate limiting
		total := len(videos)
		cachedCount, noCaptionsCount, blockedCount, errorCount := 0, 0, 0, 0
		methodCounts := map[string]int{} // tracks which fetch method succeeded
		consecutiveErrors := 0
		delay := 500 * time.Millisecond
		allMethodNames := []string{"android", "web", "scrape", "warp-android", "warp-web", "warp-scrape"}

		for i := range videos {
			freshTotal := 0
			for _, c := range methodCounts {
				freshTotal += c
			}
			var methodParts []string
			for _, m := range allMethodNames {
				if c, ok := methodCounts[m]; ok && c > 0 {
					methodParts = append(methodParts, fmt.Sprintf("%d %s", c, m))
				}
			}
			fetchedStr := fmt.Sprintf("%d fetched", freshTotal)
			if len(methodParts) > 0 {
				fetchedStr = fmt.Sprintf("%d fetched (%s)", freshTotal, strings.Join(methodParts, ", "))
			}

			sendSSE(w, flusher, "progress", map[string]string{
				"message": fmt.Sprintf("Extracting transcripts (%d/%d)... [%d cached, %s, %d no captions, %d blocked, %d errors]",
					i+1, total, cachedCount, fetchedStr, noCaptionsCount, blockedCount, errorCount),
			})
			transcript, fromCache, method, err := cachedTranscript(mongoDB, videos[i].VideoID)

			if err != nil {
				if errors.Is(err, errNoCaptions) {
					noCaptionsCount++
					consecutiveErrors = 0
				} else if errors.Is(err, errBlocked) {
					blockedCount++
					consecutiveErrors++
					if consecutiveErrors >= 3 {
						newDelay := delay * 2
						if newDelay > 5*time.Second {
							newDelay = 5 * time.Second
						}
						if newDelay > delay {
							delay = newDelay
							log.Printf("Transcript: %d consecutive blocked/errors, increasing delay to %v", consecutiveErrors, delay)
						}
					}
				} else {
					log.Printf("Transcript error for %s: %v", videos[i].VideoID, err)
					errorCount++
					consecutiveErrors++
					if consecutiveErrors >= 3 {
						newDelay := delay * 2
						if newDelay > 5*time.Second {
							newDelay = 5 * time.Second
						}
						if newDelay > delay {
							delay = newDelay
							log.Printf("Transcript: %d consecutive errors, increasing delay to %v", consecutiveErrors, delay)
						}
					}
				}
			} else {
				consecutiveErrors = 0
				if delay > 500*time.Millisecond {
					delay = 500 * time.Millisecond
				}
			}

			videos[i].Transcript = transcript
			if transcript != "" {
				if fromCache {
					cachedCount++
				} else {
					methodCounts[method]++
				}
			} else if err == nil {
				noCaptionsCount++
			}
			// Delay between uncached fetches
			if !fromCache && i < total-1 {
				time.Sleep(delay)
			}
		}

		freshTotal := 0
		for _, c := range methodCounts {
			freshTotal += c
		}
		var methodParts []string
		for _, m := range allMethodNames {
			if c, ok := methodCounts[m]; ok && c > 0 {
				methodParts = append(methodParts, fmt.Sprintf("%d %s", c, m))
			}
		}
		fetchedStr := fmt.Sprintf("%d fetched", freshTotal)
		if len(methodParts) > 0 {
			fetchedStr = fmt.Sprintf("%d fetched (%s)", freshTotal, strings.Join(methodParts, ", "))
		}
		transcriptCount := cachedCount + freshTotal
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Transcripts: %d/%d videos (%d cached, %s, %d no captions, %d blocked, %d errors)",
				transcriptCount, total, cachedCount, fetchedStr, noCaptionsCount, blockedCount, errorCount),
		})

		// Determine own channel ID for filtering
		var ownChannelID string
		if req.Config.ChannelURL != "" {
			chID, err := resolveChannelID(ytKey, req.Config.ChannelURL)
			if err == nil {
				ownChannelID = chID
			}
		}

		// Split videos into own vs third-party
		var ownVideos, thirdPartyVideos []YouTubeVideo
		for _, v := range videos {
			if ownChannelID != "" && v.ChannelID == ownChannelID {
				v.RelevanceTag = "own"
				ownVideos = append(ownVideos, v)
			} else {
				if v.RelevanceTag == "" {
					v.RelevanceTag = "category_content"
				}
				thirdPartyVideos = append(thirdPartyVideos, v)
			}
		}

		usedModel := ""

		models := provider.Models()

		// Helper to run an LLM analysis with retries and fallback
		runAnalysis := func(prompt, phaseName string) (string, string, error) {
			for mi, model := range models {
				if mi > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s unavailable, falling back to %s for %s...", models[mi-1].Name, model.Name, phaseName),
					})
				}

				claudeBody, _ := provider.BuildStreamBody(model.ID, 65536, prompt, false)

				const maxRetries = 3
				backoff := 2 * time.Second
				var lastErr error

				for attempt := 0; attempt <= maxRetries; attempt++ {
					if attempt > 0 {
						sendSSE(w, flusher, "status", map[string]string{
							"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
						})
						select {
						case <-time.After(backoff):
						case <-r.Context().Done():
							return "", "", fmt.Errorf("request cancelled")
						}
						backoff *= 2
					}

					result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
					if err == errOverloaded || errors.Is(err, ErrStreamStalled) {
						lastErr = err
						log.Printf("%s API (%s) retryable error for %s (attempt %d/%d): %v", provider.Name(), model.ID, phaseName, attempt+1, maxRetries, err)
						if attempt < maxRetries {
							continue
						}
						break
					}
					if err != nil {
						return "", "", err
					}

					return result.ResultJSON, model.Name, nil
				}

				log.Printf("%s API (%s) exhausted retries for %s: %v", provider.Name(), model.ID, phaseName, lastErr)
			}
			return "", "", fmt.Errorf("all models overloaded")
		}

		// Extract competitor names from brand context
		var competitorNames []string
		if brandInfo.Used {
			cCtx, cCancel := context.WithTimeout(r.Context(), 5*time.Second)
			var brand BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(cCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&brand); err == nil {
				for _, c := range brand.Competitors {
					competitorNames = append(competitorNames, c.Name)
				}
			}
			cCancel()
		}

		// Phase 1: Per-video transcript assessments with Haiku
		sendSSE(w, flusher, "status", map[string]string{
			"message": "Phase 1: Assessing individual transcripts...",
		})
		assessments := assessVideos(r.Context(), provider, apiKey, videos, req.Domain, req.Config.SearchTerms, mongoDB, w, flusher)

		assessedCount := 0
		for _, a := range assessments {
			if a != nil {
				assessedCount++
			}
		}
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Phase 1 complete: %d/%d videos assessed. Starting final analysis...", assessedCount, transcriptCount),
		})

		// Phase 2a: Batch-digest third-party videos into compact summaries
		var digests []BatchDigest
		if len(thirdPartyVideos) > 40 {
			sendSSE(w, flusher, "status", map[string]string{
				"message": fmt.Sprintf("Phase 2a: Digesting %d third-party videos in batches...", len(thirdPartyVideos)),
			})
			digests, err = digestThirdPartyVideos(r.Context(), provider, apiKey, thirdPartyVideos, assessments, req.Domain, req.Config.SearchTerms, w, flusher)
			if err != nil {
				code := classifyError(err)
				sendSSEError(w, flusher, code)
				saveFailedAnalysis(mongoDB, r.Context(), req.Domain, "video", code, "")
				return
			}
			sendSSE(w, flusher, "status", map[string]string{
				"message": fmt.Sprintf("Phase 2a complete: %d batches digested. Starting synthesis...", len(digests)),
			})
		} else {
			// Small enough to include directly — build single digest from all third-party
			if len(thirdPartyVideos) > 0 {
				d, dErr := digestVideoBatch(r.Context(), provider, apiKey, thirdPartyVideos, assessments, req.Domain, req.Config.SearchTerms, 0)
				if dErr != nil {
					log.Printf("Warning: single-batch digest failed, proceeding without: %v", dErr)
				} else {
					digests = []BatchDigest{*d}
				}
			}
		}

		// Phase 2b: Run 4 pillar analyses concurrently using streaming
		// (SSE writes are mutex-protected, streaming gives progress visibility + idle timeout)
		ctxParams := newVideoContextParams(req.Domain, ownVideos, len(thirdPartyVideos), digests, competitorNames, req.Config.SearchTerms, brandInfo, assessments)

		type pillarResult struct {
			pillar int // 1-4
			json   string
			model  string
			err    error
		}

		pillarCh := make(chan pillarResult, 4)
		pillarPrompts := map[int]struct {
			name   string
			prompt string
		}{
			1: {"Transcript Authority", buildPillar1Prompt(ctxParams)},
			2: {"Topical Dominance", buildPillar2Prompt(ctxParams)},
			3: {"Citation Network", buildPillar3Prompt(ctxParams)},
			4: {"Brand Narrative", buildPillar4Prompt(ctxParams)},
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Phase 2b: Analyzing 4 pillars concurrently...",
		})

		for pNum, pp := range pillarPrompts {
			go func(num int, name, prompt string) {
				rJSON, mName, pErr := runAnalysis(prompt, name)
				pillarCh <- pillarResult{pillar: num, json: stripJSONFencing(rJSON), model: mName, err: pErr}
			}(pNum, pp.name, pp.prompt)
		}

		pillarJSONs := make(map[int]string)
		var pillarErr error
		for i := 0; i < 4; i++ {
			pr := <-pillarCh
			if pr.err != nil {
				if pillarErr == nil {
					pillarErr = fmt.Errorf("pillar %d: %w", pr.pillar, pr.err)
				}
				log.Printf("Pillar %d failed: %v", pr.pillar, pr.err)
			} else {
				pillarJSONs[pr.pillar] = pr.json
				if usedModel == "" {
					usedModel = pr.model
				}
			}
			sendSSE(w, flusher, "progress", map[string]string{
				"message": fmt.Sprintf("Pillar analysis %d/4 complete...", i+1),
			})
		}

		if len(pillarJSONs) < 4 {
			code := classifyError(pillarErr)
			sendSSEError(w, flusher, code)
			saveFailedAnalysis(mongoDB, r.Context(), req.Domain, "video", code, "")
			return
		}

		// Parse each pillar result into the unified struct
		var result VideoAuthorityResult

		// Pillar 1: Transcript Authority + Video Scorecards
		var p1 struct {
			TranscriptAuthority TranscriptAuthorityPillar `json:"transcript_authority"`
			VideoScorecards     []VideoScorecard          `json:"video_scorecards"`
		}
		if err := json.Unmarshal([]byte(pillarJSONs[1]), &p1); err != nil {
			log.Printf("Warning: failed to parse pillar 1: %v", err)
		} else {
			result.TranscriptAuthority = p1.TranscriptAuthority
			result.VideoScorecards = p1.VideoScorecards
		}

		// Pillar 2: Topical Dominance
		var p2 struct {
			TopicalDominance TopicalDominancePillar `json:"topical_dominance"`
		}
		if err := json.Unmarshal([]byte(pillarJSONs[2]), &p2); err != nil {
			log.Printf("Warning: failed to parse pillar 2: %v", err)
		} else {
			result.TopicalDominance = p2.TopicalDominance
		}

		// Pillar 3: Citation Network
		var p3 struct {
			CitationNetwork CitationNetworkPillar `json:"citation_network"`
		}
		if err := json.Unmarshal([]byte(pillarJSONs[3]), &p3); err != nil {
			log.Printf("Warning: failed to parse pillar 3: %v", err)
		} else {
			result.CitationNetwork = p3.CitationNetwork
		}

		// Pillar 4: Brand Narrative
		var p4 struct {
			BrandNarrative BrandNarrativePillar `json:"brand_narrative"`
		}
		if err := json.Unmarshal([]byte(pillarJSONs[4]), &p4); err != nil {
			log.Printf("Warning: failed to parse pillar 4: %v", err)
		} else {
			result.BrandNarrative = p4.BrandNarrative
		}

		// Phase 2c: Synthesis — overall score, executive summary, recommendations
		sendSSE(w, flusher, "status", map[string]string{
			"message": "Phase 2c: Synthesizing overall report...",
		})
		synthPrompt := buildSynthesisPrompt(ctxParams.BrandName, pillarJSONs[1], pillarJSONs[2], pillarJSONs[3], pillarJSONs[4])
		synthJSON, synthModel, err := runAnalysis(synthPrompt, "Synthesis")
		if err != nil {
			// Synthesis failed — compute overall score manually and proceed without summary
			log.Printf("Synthesis failed, computing score manually: %v", err)
			result.OverallScore = int(float64(result.TranscriptAuthority.Score)*0.30 +
				float64(result.TopicalDominance.Score)*0.25 +
				float64(result.CitationNetwork.Score)*0.25 +
				float64(result.BrandNarrative.Score)*0.20)
			result.ExecutiveSummary = "Synthesis unavailable — pillar scores computed independently."
			result.ConfidenceNote = "Citation accuracy in AI search is 49-68%. Results should be interpreted with caution."
		} else {
			if synthModel != "" {
				usedModel = synthModel
			}
			synthJSON = stripJSONFencing(synthJSON)
			var synth struct {
				OverallScore     int                   `json:"overall_score"`
				ExecutiveSummary string                `json:"executive_summary"`
				ConfidenceNote   string                `json:"confidence_note"`
				Recommendations  []VideoRecommendation `json:"recommendations"`
			}
			if err := json.Unmarshal([]byte(synthJSON), &synth); err != nil {
				log.Printf("Warning: failed to parse synthesis: %v", err)
				result.OverallScore = int(float64(result.TranscriptAuthority.Score)*0.30 +
					float64(result.TopicalDominance.Score)*0.25 +
					float64(result.CitationNetwork.Score)*0.25 +
					float64(result.BrandNarrative.Score)*0.20)
			} else {
				result.OverallScore = synth.OverallScore
				result.ExecutiveSummary = synth.ExecutiveSummary
				result.ConfidenceNote = synth.ConfidenceNote
				result.Recommendations = synth.Recommendations
			}
		}

		// Marshal the final result for storage
		resultBytes, _ := json.Marshal(result)
		resultJSON := string(resultBytes)

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analysis complete — Overall Score: %d/100", result.OverallScore),
		})

		// Save results
		analysis := VideoAnalysis{
			Domain:           req.Domain,
			TenantID:         saas.TenantIDFromContext(r.Context()),
			Config:           req.Config,
			Videos:           videos,
			Result:           &result,
			RawText:          resultJSON,
			Model:            usedModel,
			BrandContextUsed: brandInfo.Used,
			GeneratedAt:      time.Now(),
		}

		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		upsertResult, saveErr := mongoDB.VideoAnalyses().ReplaceOne(saveCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			analysis,
			options.Replace().SetUpsert(true),
		)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save video analysis: %v", saveErr)
		}

		// Create todos from recommendations
		if saveErr == nil {
			var analysisID primitive.ObjectID
			if upsertResult.UpsertedID != nil {
				analysisID = upsertResult.UpsertedID.(primitive.ObjectID)
			} else {
				fetchCtx, fetchCancel := context.WithTimeout(r.Context(), 5*time.Second)
				var existing VideoAnalysis
				if err := mongoDB.VideoAnalyses().FindOne(fetchCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&existing); err == nil {
					analysisID = existing.ID
				}
				fetchCancel()
			}
			if !analysisID.IsZero() {
				go createTodosFromVideoAnalysis(mongoDB, analysisID, req.Domain, saas.TenantIDFromContext(r.Context()), result.Recommendations)
			}
		}

		// Build result for frontend
		resultMap := map[string]any{
			"domain":             req.Domain,
			"config":             req.Config,
			"videos":             videos,
			"result":             &result,
			"model":              usedModel,
			"brand_context_used": brandInfo.Used,
			"generated_at":       analysis.GeneratedAt,
		}

		frontendJSON, _ := json.Marshal(resultMap)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(frontendJSON),
		})
		trackServerEvent(mongoDB, "custom.server.video.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": req.Domain, "duration_ms": time.Since(startTime).Milliseconds(), "video_count": len(videos)})
	}
}

func handleGetVideoAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		var analysis VideoAnalysis
		err := mongoDB.VideoAnalyses().FindOne(ctx, filter, opts).Decode(&analysis)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

func handleGetVideoDetails(mongoDB *MongoDB) http.HandlerFunc {
	const transcriptSnippetLen = 500

	type videoDetail struct {
		VideoID          string           `json:"video_id"`
		Title            string           `json:"title"`
		Transcript       string           `json:"transcript"`
		TranscriptLength int              `json:"transcript_length"`
		Assessment       *VideoAssessment `json:"assessment"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		var analysis VideoAnalysis
		err := mongoDB.VideoAnalyses().FindOne(ctx, filter, opts).Decode(&analysis)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			}
			return
		}

		details := make([]videoDetail, 0, len(analysis.Videos))
		for _, v := range analysis.Videos {
			snippet := v.Transcript
			if len(snippet) > transcriptSnippetLen {
				snippet = snippet[:transcriptSnippetLen]
			}
			d := videoDetail{
				VideoID:          v.VideoID,
				Title:            v.Title,
				Transcript:       snippet,
				TranscriptLength: len(v.Transcript),
			}
			if a, ok := cachedVideoAssessment(mongoDB, v.VideoID, domain, analysis.Config.SearchTerms); ok {
				a.VideoID = v.VideoID
				a.Title = v.Title
				a.HasTranscript = v.Transcript != ""
				d.Assessment = a
			}
			details = append(details, d)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(details)
	}
}

func handleListVideoAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Exclude bulky fields
		opts := options.Find().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetLimit(50).
			SetProjection(bson.D{
				{Key: "rawText", Value: 0},
				{Key: "videos.transcript", Value: 0},
				{Key: "videos.description", Value: 0},
				{Key: "videos.tags", Value: 0},
				{Key: "result.videoScorecards", Value: 0},
				{Key: "result.brandNarrative.brandMentions", Value: 0},
				{Key: "result.citationNetwork.topCreators", Value: 0},
				{Key: "result.citationNetwork.creatorTargets", Value: 0},
				{Key: "result.topicalDominance.contentGaps", Value: 0},
			})

		cursor, err := mongoDB.VideoAnalyses().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var summaries []map[string]any
		for _, r := range results {
			summary := map[string]any{
				"id":           r["_id"],
				"domain":       r["domain"],
				"model":        r["model"],
				"generated_at": r["generatedAt"],
			}
			if res, ok := r["result"].(bson.M); ok {
				if score, ok := res["overallScore"]; ok {
					summary["overall_score"] = score
				}
			}
			if vids, ok := r["videos"].(bson.A); ok {
				summary["video_count"] = len(vids)
			} else {
				summary["video_count"] = 0
			}
			summaries = append(summaries, summary)
		}

		if summaries == nil {
			summaries = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleDeleteVideoAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Find the analysis first to get its ID for cascade delete
		delFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		var analysis struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		findErr := mongoDB.VideoAnalyses().FindOne(ctx, delFilter).Decode(&analysis)

		result, err := mongoDB.VideoAnalyses().DeleteOne(ctx, delFilter)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Cascade delete associated todos
		if findErr == nil && result.DeletedCount > 0 {
			mongoDB.Todos().DeleteMany(ctx, bson.D{{Key: "videoAnalysisId", Value: analysis.ID}})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": result.DeletedCount > 0,
		})
	}
}

func handleVideoSearchTerms(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain string `json:"domain"`
			Action string `json:"action"`
			Term   string `json:"term"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		req.Domain = normalizeDomain(req.Domain)
		req.Term = strings.TrimSpace(req.Term)
		if req.Domain == "" || req.Term == "" || (req.Action != "add" && req.Action != "remove") {
			http.Error(w, `{"error":"domain, term, and action (add|remove) are required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})

		var update bson.D
		if req.Action == "add" {
			update = bson.D{
				{Key: "$addToSet", Value: bson.D{{Key: "videoSearchAdditions", Value: req.Term}}},
				{Key: "$pull", Value: bson.D{{Key: "videoSearchRemovals", Value: req.Term}}},
			}
		} else {
			update = bson.D{
				{Key: "$addToSet", Value: bson.D{{Key: "videoSearchRemovals", Value: req.Term}}},
				{Key: "$pull", Value: bson.D{{Key: "videoSearchAdditions", Value: req.Term}}},
			}
		}

		_, err := mongoDB.BrandProfiles().UpdateOne(ctx, filter, update)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Return updated profile's search customizations
		var profile BrandProfile
		if err := mongoDB.BrandProfiles().FindOne(ctx, filter).Decode(&profile); err != nil {
			http.Error(w, `{"error":"profile not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"video_search_additions": profile.VideoSearchAdditions,
			"video_search_removals":  profile.VideoSearchRemovals,
		})
	}
}

// ── Reddit Authority Analyzer handlers ─────────────────────────────

func handleRedditDiscover(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		var req struct {
			Domain      string   `json:"domain"`
			BrandName   string   `json:"brand_name"`
			Subreddits  []string `json:"subreddits"`
			SearchTerms []string `json:"search_terms"`
			Competitors []string `json:"competitors"`
			TimeFilter  string   `json:"time_filter"` // "month", "year", "all"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}

		if len(req.SearchTerms) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "At least one search term is required"})
			return
		}

		// Normalize subreddits
		var subs []string
		for _, s := range req.Subreddits {
			if n := normalizeSubreddit(s); n != "" {
				subs = append(subs, n)
			}
		}
		// Always include a broad Reddit-wide search so threads from
		// subreddits not explicitly listed in the brand profile are discovered.
		hasAll := false
		for _, s := range subs {
			if strings.EqualFold(s, "all") {
				hasAll = true
				break
			}
		}
		if !hasAll {
			subs = append(subs, "all")
		}

		timeFilter := req.TimeFilter
		if timeFilter == "" {
			timeFilter = "year"
		}

		// Build search terms: explicit terms + brand name + competitor names
		allTerms := make([]string, 0, len(req.SearchTerms)+1+len(req.Competitors))
		seen := map[string]bool{}
		for _, t := range req.SearchTerms {
			t = strings.TrimSpace(t)
			if t != "" && !seen[strings.ToLower(t)] {
				seen[strings.ToLower(t)] = true
				allTerms = append(allTerms, t)
			}
		}
		// Add brand name as a search term if not already present
		if req.BrandName != "" && !seen[strings.ToLower(req.BrandName)] {
			seen[strings.ToLower(req.BrandName)] = true
			allTerms = append(allTerms, req.BrandName)
		}

		progress := func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		}

		progress(fmt.Sprintf("Discovering Reddit threads across %d subreddits with %d search terms...", len(subs), len(allTerms)))

		threads, err := redditDiscoverThreads(subs, allTerms, timeFilter, 15, 2*time.Second, progress)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Reddit discovery failed: " + err.Error()})
			return
		}

		if len(threads) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "No Reddit threads found matching your search criteria"})
			return
		}

		progress(fmt.Sprintf("Found %d unique threads. Fetching thread details...", len(threads)))

		// Sort by score descending and fetch top threads with comments
		sortThreadsByScore(threads)
		maxFetch := 40
		if len(threads) < maxFetch {
			maxFetch = len(threads)
		}
		detailed := redditFetchThreadDetails(threads[:maxFetch], maxFetch, 2*time.Second, progress)

		// Build summaries for frontend
		type threadSummary struct {
			ID          string    `json:"id"`
			Subreddit   string    `json:"subreddit"`
			Title       string    `json:"title"`
			Score       int       `json:"score"`
			UpvoteRatio float64   `json:"upvote_ratio"`
			NumComments int       `json:"num_comments"`
			Permalink   string    `json:"permalink"`
			CreatedUTC  time.Time `json:"created_utc"`
			IsSelfPost  bool      `json:"is_self_post"`
			HasComments bool      `json:"has_comments"`
		}
		summaries := make([]threadSummary, len(detailed))
		for i, t := range detailed {
			summaries[i] = threadSummary{
				ID:          t.ID,
				Subreddit:   t.Subreddit,
				Title:       t.Title,
				Score:       t.Score,
				UpvoteRatio: t.UpvoteRatio,
				NumComments: t.NumComments,
				Permalink:   t.Permalink,
				CreatedUTC:  t.CreatedUTC,
				IsSelfPost:  t.IsSelfPost,
				HasComments: len(t.TopComments) > 0,
			}
		}

		// Collect subreddits discovered from results that weren't in the original request
		requestedSubs := map[string]bool{}
		for _, s := range req.Subreddits {
			if n := normalizeSubreddit(s); n != "" {
				requestedSubs[strings.ToLower(n)] = true
			}
		}
		discoveredSubs := map[string]bool{}
		for _, t := range detailed {
			sub := strings.ToLower(t.Subreddit)
			if sub != "" && !requestedSubs[sub] {
				discoveredSubs[sub] = true
			}
		}
		var discoveredList []string
		for sub := range discoveredSubs {
			discoveredList = append(discoveredList, sub)
		}
		sort.Strings(discoveredList)

		sendSSE(w, flusher, "done", map[string]any{
			"threads":              summaries,
			"total":                len(threads),
			"discovered_subreddits": discoveredList,
		})
	}
}

func sortThreadsByScore(threads []RedditThread) {
	for i := 1; i < len(threads); i++ {
		for j := i; j > 0 && threads[j].Score > threads[j-1].Score; j-- {
			threads[j], threads[j-1] = threads[j-1], threads[j]
		}
	}
}

func handleRedditAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		var req struct {
			Domain            string   `json:"domain"`
			Config            RedditAnalysisConfig `json:"config"`
			SelectedThreadIDs []string `json:"selected_thread_ids"`
			// Full thread data from discovery
			Threads []RedditThread `json:"threads"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}

		if len(req.Threads) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "No threads selected for analysis"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)

		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))

		// Fetch full thread details with comments for selected threads
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Fetching full content for %d threads...", len(req.Threads)),
		})

		selectedSet := map[string]bool{}
		for _, id := range req.SelectedThreadIDs {
			selectedSet[id] = true
		}

		// Filter to selected threads
		var threadsToAnalyze []RedditThread
		for _, t := range req.Threads {
			if len(selectedSet) == 0 || selectedSet[t.ID] {
				threadsToAnalyze = append(threadsToAnalyze, t)
			}
		}

		// Fetch full thread details with comments
		detailed := redditFetchThreadDetails(threadsToAnalyze, len(threadsToAnalyze), 2*time.Second, func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		})

		// Extract competitor names
		var competitorNames []string
		if brandInfo.Used {
			cCtx, cCancel := context.WithTimeout(r.Context(), 5*time.Second)
			var brand BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(cCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&brand); err == nil {
				for _, c := range brand.Competitors {
					competitorNames = append(competitorNames, c.Name)
				}
			}
			cCancel()
		}

		// Build the analysis prompt
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analyzing %d Reddit threads for LLM authority signals...", len(detailed)),
		})

		prompt := buildRedditAuthorityPrompt(req.Domain, detailed, competitorNames, req.Config.SearchTerms, brandInfo)

		// Model fallback chain (same as video)
		usedModel := ""
		models := provider.Models()

		runAnalysis := func(prompt, phaseName string) (string, string, error) {
			for mi, model := range models {
				if mi > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s unavailable, falling back to %s for %s...", models[mi-1].Name, model.Name, phaseName),
					})
				}

				claudeBody, _ := provider.BuildStreamBody(model.ID, 65536, prompt, false)

				const maxRetries = 3
				backoff := 2 * time.Second
				var lastErr error

				for attempt := 0; attempt <= maxRetries; attempt++ {
					if attempt > 0 {
						sendSSE(w, flusher, "status", map[string]string{
							"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
						})
						select {
						case <-time.After(backoff):
						case <-r.Context().Done():
							return "", "", fmt.Errorf("request cancelled")
						}
						backoff *= 2
					}

					result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
					if err == errOverloaded {
						lastErr = err
						if attempt < maxRetries {
							continue
						}
						break
					}
					if err != nil {
						return "", "", err
					}

					return result.ResultJSON, model.Name, nil
				}

				log.Printf("%s API (%s) exhausted retries for %s: %v", provider.Name(), model.ID, phaseName, lastErr)
			}
			return "", "", fmt.Errorf("all models overloaded")
		}

		resultJSON, modelName, err := runAnalysis(prompt, "Reddit Authority")
		if err != nil {
			code := classifyError(err)
			sendSSEError(w, flusher, code)
			saveFailedAnalysis(mongoDB, r.Context(), req.Domain, "reddit", code, "")
			return
		}
		usedModel = modelName

		resultJSON = stripJSONFencing(resultJSON)
		var result RedditAuthorityResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			log.Printf("Warning: failed to parse reddit authority result: %v", err)
			sendSSEError(w, flusher, ErrCodeParseError)
			saveFailedAnalysis(mongoDB, r.Context(), req.Domain, "reddit", ErrCodeParseError, usedModel)
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analysis complete — Overall Score: %d/100", result.OverallScore),
		})

		// Convert threads to summaries for storage
		storedThreads := make([]RedditThreadSummary, len(detailed))
		for i, t := range detailed {
			storedThreads[i] = RedditThreadSummary{
				ID:           t.ID,
				Subreddit:    t.Subreddit,
				Title:        t.Title,
				SelfText:     truncate(t.SelfText, 500),
				Author:       t.Author,
				Score:        t.Score,
				UpvoteRatio:  t.UpvoteRatio,
				NumComments:  t.NumComments,
				URL:          t.URL,
				Permalink:    t.Permalink,
				CreatedUTC:   t.CreatedUTC,
				IsSelfPost:   t.IsSelfPost,
				CommentCount: len(t.TopComments),
			}
		}

		// Save results
		analysis := RedditAnalysis{
			Domain:           req.Domain,
			TenantID:         saas.TenantIDFromContext(r.Context()),
			Config:           req.Config,
			Threads:          storedThreads,
			Result:           &result,
			RawText:          resultJSON,
			Model:            usedModel,
			BrandContextUsed: brandInfo.Used,
			GeneratedAt:      time.Now(),
		}

		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		_, saveErr := mongoDB.RedditAnalyses().ReplaceOne(saveCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			analysis,
			options.Replace().SetUpsert(true),
		)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save reddit analysis: %v", saveErr)
		}

		// Create todos from recommendations
		if saveErr == nil && len(result.Recommendations) > 0 {
			go createTodosFromRedditAnalysis(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()), result.Recommendations)
		}

		// Build result for frontend
		resultMap := map[string]any{
			"domain":             req.Domain,
			"config":             req.Config,
			"threads":            storedThreads,
			"result":             &result,
			"model":              usedModel,
			"brand_context_used": brandInfo.Used,
			"generated_at":       analysis.GeneratedAt,
		}

		frontendJSON, _ := json.Marshal(resultMap)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(frontendJSON),
		})
		trackServerEvent(mongoDB, "custom.server.reddit.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": req.Domain, "duration_ms": time.Since(startTime).Milliseconds()})
	}
}

func createTodosFromRedditAnalysis(mongoDB *MongoDB, domain, tenantID string, recommendations []RedditRecommendation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, rec := range recommendations {
		if rec.Priority != "high" && rec.Priority != "medium" {
			continue
		}
		todo := TodoItem{
			TenantID:       tenantID,
			SourceType:     "reddit",
			Domain:         domain,
			Question:       "Reddit Authority",
			Action:         rec.Action,
			Summary:        rec.Action,
			ExpectedImpact: rec.ExpectedImpact,
			Dimension:      rec.Dimension,
			Priority:       rec.Priority,
			Status:         "todo",
			CreatedAt:      time.Now(),
		}
		if _, err := mongoDB.Todos().InsertOne(ctx, todo); err != nil {
			log.Printf("Failed to create reddit todo: %v", err)
		}
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

func buildRedditAuthorityPrompt(domain string, threads []RedditThread, competitors, searchTerms []string, brandInfo BrandContextInfo) string {
	var sb strings.Builder
	brandName := domain
	if brandInfo.Used {
		for _, line := range strings.Split(brandInfo.ContextString, "\n") {
			if strings.HasPrefix(line, "Company: ") {
				brandName = strings.TrimPrefix(line, "Company: ")
				break
			}
		}
	}

	sb.WriteString(fmt.Sprintf(`You are an expert at analyzing Reddit presence for LLM authority optimization.

## Context

**Target Brand**: %s (domain: %s)
`, brandName, domain))

	if brandInfo.Used {
		sb.WriteString(fmt.Sprintf("\n**Brand Intelligence**:\n%s\n", brandInfo.ContextString))
	}

	if len(competitors) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Key Competitors**: %s\n", strings.Join(competitors, ", ")))
	}

	if len(searchTerms) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Search Terms Used**: %s\n", strings.Join(searchTerms, ", ")))
	}

	sb.WriteString(`
## Research Context

Reddit is the #2 source for LLM-generated citations, behind only YouTube. Reddit threads—especially highly-upvoted recommendation threads—strongly influence how LLMs answer questions about products, services, and brands. Key dynamics:

1. **Training Data Weight**: Reddit was foundational in LLM training (WebText, Common Crawl). Google pays $60M/year and OpenAI $70M/year for Reddit data access.
2. **Multi-User Validation**: Upvotes and comment consensus create credibility signals that single-author content cannot match.
3. **Recommendation Threads**: "Best X for Y" threads are among the most influential for LLM responses to comparison queries.
4. **Community Authority**: Mentions in authoritative, topic-specific subreddits carry more weight than general subreddits.
5. **Recency Signal**: Recent discussions with active engagement signal ongoing relevance.

## Task

Analyze the following Reddit threads to assess this brand's Reddit authority—how strongly Reddit discussions would influence LLMs to cite, recommend, or reference this brand.

## Thread Data

`)

	for i, t := range threads {
		sb.WriteString(fmt.Sprintf("### Thread %d: r/%s — %s\n", i+1, t.Subreddit, t.Title))
		sb.WriteString(fmt.Sprintf("Score: %d | Upvote Ratio: %.0f%% | Comments: %d | Posted: %s\n",
			t.Score, t.UpvoteRatio*100, t.NumComments, t.CreatedUTC.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("Permalink: https://reddit.com%s\n", t.Permalink))

		if t.SelfText != "" {
			text := t.SelfText
			if len(text) > 1000 {
				text = text[:1000] + "..."
			}
			sb.WriteString(fmt.Sprintf("\n**Post Body**:\n%s\n", text))
		}

		if len(t.TopComments) > 0 {
			sb.WriteString("\n**Top Comments**:\n")
			for j, c := range t.TopComments {
				if j >= 10 {
					break
				}
				body := c.Body
				if len(body) > 500 {
					body = body[:500] + "..."
				}
				sb.WriteString(fmt.Sprintf("- [%d pts] %s\n", c.Score, body))
			}
		}
		sb.WriteString("\n---\n\n")
	}

	sb.WriteString(fmt.Sprintf(`## Output Format

Return a JSON object with the following structure. ALL scores are 0-100. Be thorough and evidence-based.

{
  "overall_score": <0-100 weighted average>,
  "presence": {
    "score": <0-100>,
    "evidence": ["evidence point 1", "..."],
    "total_mentions": <count of threads where %s is explicitly mentioned>,
    "unique_subreddits": <count of unique subreddits with mentions>,
    "share_of_voice": [
      {"brand_name": "%s", "mention_count": <n>, "percentage": <0-100>},
      {"brand_name": "<competitor>", "mention_count": <n>, "percentage": <0-100>}
    ],
    "mention_trend": "growing|stable|declining"
  },
  "sentiment": {
    "score": <0-100>,
    "evidence": ["..."],
    "sentiment": {"positive": <n>, "neutral": <n>, "negative": <n>, "total": <n>},
    "recommendation_rate": <0-100 percent of mentions that recommend the brand>,
    "top_praise": ["praise theme 1", "..."],
    "top_criticism": ["criticism theme 1", "..."],
    "notable_mentions": [
      {
        "thread_id": "<id>",
        "subreddit": "<subreddit>",
        "title": "<thread title>",
        "score": <thread score>,
        "sentiment": "positive|neutral|negative",
        "context": "<excerpt showing the mention>",
        "is_recommendation": <true if user recommends the brand>
      }
    ]
  },
  "competitive": {
    "score": <0-100>,
    "evidence": ["..."],
    "win_rate": <0-100 percent of head-to-head comparisons where brand wins>,
    "comparison_threads": <count of threads comparing brand to competitors>,
    "differentiators": ["unique strength cited by Reddit users", "..."],
    "competitor_strengths": ["competitor advantage not countered", "..."],
    "head_to_head_examples": [<same format as notable_mentions>]
  },
  "training_signal": {
    "score": <0-100>,
    "evidence": ["..."],
    "high_score_threads": <count of threads with 50+ score>,
    "deep_threads": <count of threads with 10+ comments>,
    "authority_tier": "strong|moderate|weak",
    "key_threads": [<most influential threads in notable_mention format>],
    "recommendations": ["specific action to improve Reddit training signal", "..."]
  },
  "executive_summary": "<2-3 paragraph analysis of the brand's Reddit presence and its implications for LLM authority>",
  "confidence_note": "<brief note on data limitations or confidence level>",
  "recommendations": [
    {
      "action": "<specific actionable recommendation>",
      "expected_impact": "<expected outcome>",
      "dimension": "presence|sentiment|competitive|training_signal",
      "priority": "high|medium|low"
    }
  ]
}

**Scoring Guidelines**:
- **Presence (25%% weight)**: Volume and breadth of mentions. High = mentioned across many relevant subreddits, frequently. Low = rarely discussed.
- **Sentiment (25%% weight)**: Tone and recommendation strength. High = frequently recommended, positive consensus. Low = criticized or ignored.
- **Competitive (25%% weight)**: Position vs. competitors. High = wins head-to-head comparisons, cited as best-in-class. Low = loses comparisons, positioned as inferior.
- **Training Signal (25%% weight)**: Likelihood that Reddit content will influence LLM training. High = many high-upvote, deep-comment threads in authoritative subreddits. Low = low-engagement or shallow discussions.

Return ONLY the JSON object, no markdown fencing.
`, brandName, brandName))

	return sb.String()
}

func handleGetRedditAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		var analysis RedditAnalysis
		err := mongoDB.RedditAnalyses().FindOne(ctx, filter, opts).Decode(&analysis)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

func handleListRedditAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		opts := options.Find().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetLimit(50).
			SetProjection(bson.D{
				{Key: "rawText", Value: 0},
				{Key: "threads.selfText", Value: 0},
			})

		cursor, err := mongoDB.RedditAnalyses().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var summaries []map[string]any
		for _, r := range results {
			summary := map[string]any{
				"id":           r["_id"],
				"domain":       r["domain"],
				"model":        r["model"],
				"generated_at": r["generatedAt"],
			}
			if res, ok := r["result"].(bson.M); ok {
				if score, ok := res["overallScore"]; ok {
					summary["overall_score"] = score
				}
			}
			if threads, ok := r["threads"].(bson.A); ok {
				summary["thread_count"] = len(threads)
			} else {
				summary["thread_count"] = 0
			}
			summaries = append(summaries, summary)
		}

		if summaries == nil {
			summaries = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleDeleteRedditAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		delFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		result, err := mongoDB.RedditAnalyses().DeleteOne(ctx, delFilter)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": result.DeletedCount > 0,
		})
	}
}

// ============================================================
// Search Visibility Analysis
// ============================================================

func buildSearchVisibilityPrompt(domain string, brandInfo BrandContextInfo, competitors, categoryKeywords, categoryQueries []string) string {
	var sb strings.Builder
	brandName := domain
	if brandInfo.Used {
		for _, line := range strings.Split(brandInfo.ContextString, "\n") {
			if strings.HasPrefix(line, "Company: ") {
				brandName = strings.TrimPrefix(line, "Company: ")
				break
			}
		}
	}

	sb.WriteString(fmt.Sprintf(`You are an expert at analyzing search visibility signals that affect whether AI systems — including Google AI Overviews, ChatGPT, Claude, Gemini, and Perplexity — will discover, index, and cite a website's content.

## Context

**Target Brand**: %s (domain: %s)
`, brandName, domain))

	if brandInfo.Used {
		sb.WriteString(fmt.Sprintf("\n**Brand Intelligence**:\n%s\n", brandInfo.ContextString))
	}

	if len(competitors) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Key Competitors**: %s\n", strings.Join(competitors, ", ")))
	}

	sb.WriteString(fmt.Sprintf(`
## Research Context

Search visibility affects AI citation through two distinct pathways:

**Google AI Overviews (strong SEO correlation):**
- 76%% of AI Overview citations pull from top-10 organic pages (Ahrefs, 1.9M citations study)
- Being cited in an AIO increases CTR by 80%%+ for that site
- Reddit (40.1%%) and Wikipedia (26.3%%) dominate AIO citations
- 82%% of AIOs appear for keywords with <1,000 monthly searches (long-tail dominated)

**Standalone LLMs (weak SEO correlation):**
- Only 12%% of ChatGPT/Claude/Gemini cited URLs rank in Google's top 10
- Brand web mentions (0.664 correlation) and YouTube mentions (0.737) are stronger predictors than backlinks (0.37)
- 28%% of ChatGPT's top-cited pages have zero Google organic visibility

**Crawl Accessibility:**
- GPTBot grew 305%% YoY; OpenAI's crawl-to-referral ratio is 1,700:1
- OpenAI, Anthropic, and Perplexity each now run 3 separate bots: training, indexing, and user-fetch
- Blocking training bots while allowing search bots is a valid strategy; blocking everything hurts AI visibility
- ~21%% of top-1000 sites block GPTBot

**Content Freshness:**
- AI assistants cite content that is 25.7%% newer than traditional Google results (Ahrefs, 17M citations)
- 65%% of AI bot crawl hits target content published within the past year
- ChatGPT, Perplexity strongly favor recent content; Google AIOs actually prefer older authoritative content

**Brand Search Momentum:**
- Brand search volume: 0.334 correlation with AI citation frequency
- Winner-takes-all: brands in top 25%% for web mentions average 169 AI Overview mentions vs 14 for the 50th-75th percentile (12x gap)

## Your Task

Using the web_search tool, conduct a comprehensive search visibility analysis of %s. You must:

1. **Visit the site's robots.txt** — search for "%s robots.txt" and/or try to access it directly. Identify which AI crawlers are explicitly allowed or disallowed (GPTBot, ClaudeBot, PerplexityBot, Googlebot, and their SearchBot/User variants).

2. **Check for structured data** — Visit key pages of the site and check for Schema.org markup (JSON-LD), OpenGraph tags, structured content (tables, FAQ sections, comparison formats).

3. **Assess organic search presence** — Search for the brand name and key product/service terms. How prominently does the site appear in search results? How well does it rank for relevant informational queries?

4. **Evaluate content freshness** — Check publication dates, last-updated indicators, blog/content publishing frequency. Are key pages being regularly updated?

5. **Assess brand search momentum** — Search for the brand name to gauge how well-known it is. Look for web mentions, reviews, news coverage. Compare against any competitors.

6. **Check sitemap** — Search for the site's sitemap.xml and assess its completeness.
`, domain, domain))

	// Add category discovery step if brand has categories
	hasCategories := len(categoryKeywords) > 0 || len(categoryQueries) > 0
	if hasCategories {
		sb.WriteString(`
7. **Assess category discovery** — Search for the brand's category keywords listed below WITHOUT the brand name. Does the brand appear in these generic category results? How visible is it compared to competitors? This measures whether people searching the category (without knowing the brand) would discover it.

`)
		if len(categoryKeywords) > 0 {
			sb.WriteString(fmt.Sprintf("Category Keywords: %s\n", strings.Join(categoryKeywords, ", ")))
		}
		if len(categoryQueries) > 0 {
			sb.WriteString(fmt.Sprintf("Category Intent Queries: %s\n", strings.Join(categoryQueries, ", ")))
		}
	}

	// Weights depend on whether category discovery is included
	aioW, crawlW, momentumW, freshnessW := "30%", "20%", "25%", "25%"
	weightLine := "AIO Readiness 30%, Crawl Accessibility 20%, Brand Momentum 25%, Content Freshness 25%"
	if hasCategories {
		aioW, crawlW, momentumW, freshnessW = "25%", "15%", "20%", "20%"
		weightLine = "AIO Readiness 25%, Crawl Accessibility 15%, Brand Momentum 20%, Content Freshness 20%, Category Discovery 20%"
	}

	sb.WriteString(fmt.Sprintf(`
Return your analysis as a JSON object with this exact structure:

{
  "overall_score": <0-100 integer>,
  "aio_readiness": {
    "score": <0-100 integer, weighted %s>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "organic_presence": <0-100>,
    "structured_data": <0-100>,
    "content_format": <0-100>,
    "answer_prominence": <0-100>
  },
  "crawl_accessibility": {
    "score": <0-100 integer, weighted %s>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "robots_txt_policy": "<summary of robots.txt AI crawler policy>",
    "ai_bot_access": <0-100>,
    "sitemap_quality": <0-100>,
    "render_accessibility": <0-100>,
    "crawler_details": [
      {"name": "GPTBot", "allowed": true/false, "notes": "..."},
      {"name": "ClaudeBot", "allowed": true/false, "notes": "..."},
      {"name": "PerplexityBot", "allowed": true/false, "notes": "..."},
      {"name": "Google-Extended", "allowed": true/false, "notes": "..."},
      {"name": "Googlebot", "allowed": true/false, "notes": "..."}
    ]
  },
  "brand_momentum": {
    "score": <0-100 integer, weighted %s>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "brand_search_trend": "growing" | "stable" | "declining",
    "competitor_compare": "<narrative comparison>",
    "web_mention_strength": <0-100>,
    "entity_recognition": <0-100>
  },
  "content_freshness": {
    "score": <0-100 integer, weighted %s>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "average_content_age": "<narrative description>",
    "update_frequency": "frequent" | "moderate" | "infrequent" | "stale",
    "freshness_signals": <0-100>,
    "content_decay_risk": <0-100>
  },`, aioW, crawlW, momentumW, freshnessW))

	if hasCategories {
		sb.WriteString(`
  "category_discovery": {
    "score": <0-100 integer, weighted 20%>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "category_visibility": <0-100>,
    "intent_coverage": <0-100>,
    "competitor_gap": <0-100>,
    "discovery_potential": <0-100>
  },`)
	}

	sb.WriteString(fmt.Sprintf(`
  "executive_summary": "<2-3 paragraph summary of search visibility posture and its implications for AI citation>",
  "confidence_note": "<brief note on data limitations>",
  "recommendations": [
    {
      "action": "<specific action to take>",
      "priority": "high" | "medium" | "low",
      "expected_impact": "<what improvement to expect>",
      "dimension": "<which pillar this addresses>"
    }
  ]
}

IMPORTANT:
- The overall_score should be a weighted average: %s
- Provide 3-6 evidence items per pillar with specific, verifiable findings
- Include 4-8 prioritized recommendations
- Be specific and cite actual findings from your searches
- Return ONLY the JSON object, no markdown fencing or explanation
`, weightLine))

	return sb.String()
}

func handleSearchAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}

		var req struct {
			Domain string `json:"domain"`
			Force  bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}
		if req.Domain == "" {
			sendSSE(w, flusher, "error", map[string]string{"message": "Domain is required"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)

		// Check for cached result (30-day TTL)
		if !req.Force {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			var existing SearchAnalysis
			err := mongoDB.SearchAnalyses().FindOne(ctx,
				tenantFilter(r.Context(), bson.D{
					{Key: "domain", Value: req.Domain},
					{Key: "generatedAt", Value: bson.D{{Key: "$gt", Value: time.Now().AddDate(0, 0, -30)}}},
				}),
			).Decode(&existing)
			cancel()
			if err == nil && existing.Result != nil {
				resultMap := map[string]any{
					"domain":             existing.Domain,
					"result":             existing.Result,
					"model":              existing.Model,
					"brand_context_used": existing.BrandContextUsed,
					"generated_at":       existing.GeneratedAt,
				}
				frontendJSON, _ := json.Marshal(resultMap)
				sendSSE(w, flusher, "done", map[string]any{
					"result": string(frontendJSON),
					"cached": true,
				})
				return
			}
		}

		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))

		// Extract competitor names, categories, and category queries
		var competitorNames, categoryKeywords, categoryQueries []string
		if brandInfo.Used {
			cCtx, cCancel := context.WithTimeout(r.Context(), 5*time.Second)
			var brand BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(cCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&brand); err == nil {
				for _, c := range brand.Competitors {
					competitorNames = append(competitorNames, c.Name)
				}
				categoryKeywords = brand.Categories
				for _, tq := range brand.TargetQueries {
					if tq.Type == "category" || tq.Type == "problem" {
						categoryQueries = append(categoryQueries, tq.Query)
					}
				}
			}
			cCancel()
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing search visibility signals...",
		})

		prompt := buildSearchVisibilityPrompt(req.Domain, brandInfo, competitorNames, categoryKeywords, categoryQueries)

		// Model fallback chain
		usedModel := ""
		models := provider.Models()

		var resultJSON string
		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 65536, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s issue, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						sendSSE(w, flusher, "error", map[string]string{"message": "Request cancelled"})
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					code := classifyError(err)
					sendSSEError(w, flusher, code)
					saveFailedAnalysis(mongoDB, r.Context(), req.Domain, "search", code, model.Name)
					return
				}

				resultJSON = result.ResultJSON
				usedModel = model.Name
				goto done
			}

			log.Printf("%s API (%s) exhausted retries: %v", provider.Name(), model.ID, lastErr)
		}

		if usedModel == "" {
			sendSSEError(w, flusher, ErrCodeAPIOverloaded)
			saveFailedAnalysis(mongoDB, r.Context(), req.Domain, "search", ErrCodeAPIOverloaded, "")
			return
		}

	done:
		resultJSON = stripJSONFencing(resultJSON)
		var result SearchVisibilityResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			log.Printf("Warning: failed to parse search visibility result: %v", err)
			sendSSEError(w, flusher, ErrCodeParseError)
			saveFailedAnalysis(mongoDB, r.Context(), req.Domain, "search", ErrCodeParseError, usedModel)
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analysis complete — Overall Score: %d/100", result.OverallScore),
		})

		// Save results
		analysis := SearchAnalysis{
			Domain:           req.Domain,
			TenantID:         saas.TenantIDFromContext(r.Context()),
			Result:           &result,
			RawText:          resultJSON,
			Model:            usedModel,
			BrandContextUsed: brandInfo.Used,
			GeneratedAt:      time.Now(),
		}

		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		_, saveErr := mongoDB.SearchAnalyses().ReplaceOne(saveCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			analysis,
			options.Replace().SetUpsert(true),
		)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save search analysis: %v", saveErr)
		}

		// Create todos from recommendations
		if saveErr == nil && len(result.Recommendations) > 0 {
			go createTodosFromSearchAnalysis(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()), result.Recommendations)
		}

		// Build result for frontend
		resultMap := map[string]any{
			"domain":             req.Domain,
			"result":             &result,
			"model":              usedModel,
			"brand_context_used": brandInfo.Used,
			"generated_at":       analysis.GeneratedAt,
		}

		frontendJSON, _ := json.Marshal(resultMap)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(frontendJSON),
		})
		trackServerEvent(mongoDB, "custom.server.search.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": req.Domain, "duration_ms": time.Since(startTime).Milliseconds()})
	}
}

func createTodosFromSearchAnalysis(mongoDB *MongoDB, domain, tenantID string, recommendations []SearchRecommendation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, rec := range recommendations {
		if rec.Priority != "high" && rec.Priority != "medium" {
			continue
		}
		var tags []string
		if rec.Dimension == "category_discovery" {
			tags = []string{"discovery"}
		}
		todo := TodoItem{
			TenantID:       tenantID,
			SourceType:     "search",
			Domain:         domain,
			Question:       "Search Visibility",
			Action:         rec.Action,
			Summary:        rec.Action,
			ExpectedImpact: rec.ExpectedImpact,
			Dimension:      rec.Dimension,
			Priority:       rec.Priority,
			Status:         "todo",
			CreatedAt:      time.Now(),
			Tags:           tags,
		}
		if _, err := mongoDB.Todos().InsertOne(ctx, todo); err != nil {
			log.Printf("Failed to create search todo: %v", err)
		}
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

func handleGetSearchAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var analysis SearchAnalysis
		err := mongoDB.SearchAnalyses().FindOne(ctx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
		).Decode(&analysis)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

func handleListSearchAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		opts := options.Find().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetProjection(bson.D{
				{Key: "domain", Value: 1},
				{Key: "result.overall_score", Value: 1},
				{Key: "model", Value: 1},
				{Key: "generatedAt", Value: 1},
			})

		cursor, err := mongoDB.SearchAnalyses().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var summaries []SearchAnalysisSummary
		for cursor.Next(ctx) {
			var doc struct {
				ID          primitive.ObjectID `bson:"_id"`
				Domain      string             `bson:"domain"`
				Result      *struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
				Model       string    `bson:"model"`
				GeneratedAt time.Time `bson:"generatedAt"`
			}
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			s := SearchAnalysisSummary{
				ID:          doc.ID,
				Domain:      doc.Domain,
				Model:       doc.Model,
				GeneratedAt: doc.GeneratedAt,
			}
			if doc.Result != nil {
				score := doc.Result.OverallScore
				s.OverallScore = &score
			}
			summaries = append(summaries, s)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleDeleteSearchAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		delFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		result, err := mongoDB.SearchAnalyses().DeleteOne(ctx, delFilter)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": result.DeletedCount > 0,
		})
	}
}

func handleListFailedAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		if domain := r.URL.Query().Get("domain"); domain != "" {
			filter = append(filter, bson.E{Key: "domain", Value: normalizeDomain(domain)})
		}

		opts := options.Find().
			SetSort(bson.D{{Key: "failedAt", Value: -1}}).
			SetLimit(20)

		cursor, err := mongoDB.FailedAnalyses().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var results []FailedAnalysis
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if results == nil {
			results = []FailedAnalysis{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func handleDeleteFailedAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}})
		result, err := mongoDB.FailedAnalyses().DeleteOne(ctx, filter)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": result.DeletedCount > 0,
		})
	}
}

// BrandContextInfo holds brand context lookup results.
type BrandContextInfo struct {
	Used             bool
	ProfileUpdatedAt *time.Time
	ContextString    string
}

// lookupBrandContext returns brand context for use in prompts. If no profile exists, returns empty info.
// tenantID is optional — when non-empty, filters brand profiles by tenant.
func lookupBrandContext(mongoDB *MongoDB, domain, tenantID string) BrandContextInfo {
	domain = normalizeDomain(domain)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{Key: "domain", Value: domain}}
	if tenantID != "" {
		filter = append(filter, bson.E{Key: "tenantId", Value: tenantID})
	}
	var brand BrandProfile
	err := mongoDB.BrandProfiles().FindOne(ctx, filter).Decode(&brand)
	if err != nil {
		return BrandContextInfo{}
	}

	var parts []string
	if brand.BrandName != "" {
		parts = append(parts, fmt.Sprintf("Company: %s", brand.BrandName))
	}
	if brand.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", brand.Description))
	}
	if len(brand.Categories) > 0 {
		parts = append(parts, fmt.Sprintf("Categories: %s", strings.Join(brand.Categories, ", ")))
	}
	if len(brand.Products) > 0 {
		parts = append(parts, fmt.Sprintf("Products/Features: %s", strings.Join(brand.Products, ", ")))
	}
	if brand.PrimaryAudience != "" {
		parts = append(parts, fmt.Sprintf("Target Audience: %s", brand.PrimaryAudience))
	}
	if len(brand.KeyUseCases) > 0 {
		parts = append(parts, fmt.Sprintf("Key Use Cases: %s", strings.Join(brand.KeyUseCases, ", ")))
	}
	if len(brand.Competitors) > 0 {
		names := make([]string, len(brand.Competitors))
		for i, c := range brand.Competitors {
			names[i] = c.Name
		}
		parts = append(parts, fmt.Sprintf("Known Competitors: %s", strings.Join(names, ", ")))
	}
	if len(brand.Differentiators) > 0 {
		parts = append(parts, fmt.Sprintf("Key Differentiators: %s", strings.Join(brand.Differentiators, ", ")))
	}
	if len(brand.KeyMessages) > 0 {
		claims := make([]string, len(brand.KeyMessages))
		for i, m := range brand.KeyMessages {
			if m.EvidenceURL != "" {
				claims[i] = fmt.Sprintf("%s (evidence: %s) [%s]", m.Claim, m.EvidenceURL, m.Priority)
			} else {
				claims[i] = fmt.Sprintf("%s [%s]", m.Claim, m.Priority)
			}
		}
		parts = append(parts, fmt.Sprintf("Brand Claims (claims the brand aspires to make):\n- %s", strings.Join(claims, "\n- ")))
	}
	if len(brand.TargetQueries) > 0 {
		tqs := make([]string, len(brand.TargetQueries))
		for i, tq := range brand.TargetQueries {
			tqs[i] = fmt.Sprintf("%s [%s, %s]", tq.Query, tq.Priority, tq.Type)
		}
		parts = append(parts, fmt.Sprintf("Target Queries (questions the brand wants to be found for):\n- %s", strings.Join(tqs, "\n- ")))
	}

	updatedAt := brand.UpdatedAt
	if len(parts) == 0 {
		return BrandContextInfo{ProfileUpdatedAt: &updatedAt}
	}

	contextStr := fmt.Sprintf("\n\n--- Brand Intelligence Context for %s ---\n%s\n--- End Brand Context ---\n", domain, strings.Join(parts, "\n"))
	return BrandContextInfo{
		Used:             true,
		ProfileUpdatedAt: &updatedAt,
		ContextString:    contextStr,
	}
}

// computeBrandCompleteness calculates a 0-100 completeness score for a brand profile.
func computeBrandCompleteness(p BrandProfile) int {
	score := 0
	if p.BrandName != "" {
		score += 8
	}
	if p.Description != "" {
		score += 10
	}
	if len(p.Categories) > 0 {
		score += 6
	}
	if len(p.Products) > 0 {
		score += 6
	}
	if p.PrimaryAudience != "" {
		score += 8
	}
	if len(p.KeyUseCases) > 0 {
		score += 7
	}
	if len(p.Competitors) > 0 {
		score += 15
	}
	if len(p.Competitors) >= 3 {
		score += 10
	}
	if len(p.TargetQueries) > 0 {
		score += 8
	}
	if len(p.TargetQueries) >= 5 {
		score += 4
	}
	if len(p.KeyMessages) > 0 {
		score += 4
	}
	if len(p.Differentiators) > 0 {
		score += 4
	}
	if p.PresenceComplete || p.Presence.YouTubeURL != "" || len(p.Presence.Subreddits) > 0 || len(p.Presence.ReviewSiteURLs) > 0 {
		score += 10
	}
	if score > 100 {
		score = 100
	}
	return score
}

func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	if json.Valid([]byte(text)) {
		return text
	}

	// Check for ```json code blocks
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(text[start:], "```"); end != -1 {
			candidate := strings.TrimSpace(text[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	// Check for ``` code blocks
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + 3
		if nl := strings.Index(text[start:], "\n"); nl != -1 {
			start += nl + 1
		}
		if end := strings.Index(text[start:], "```"); end != -1 {
			candidate := strings.TrimSpace(text[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	// Find outermost { }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end > start {
		candidate := text[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	return text
}

// ==================== LLM Test Handlers ====================

type testRawResponse struct {
	providerID   string
	providerName string
	model        string
	queryIdx     int
	response     string
	err          error
}

func handleGenerateTestQueries(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		req.Domain = normalizeDomain(req.Domain)
		if req.Domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		tenantID := saas.TenantIDFromContext(r.Context())
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		filter := bson.D{{Key: "domain", Value: req.Domain}}
		if tenantID != "" {
			filter = append(filter, bson.E{Key: "tenantId", Value: tenantID})
		}
		var brand BrandProfile
		err := mongoDB.BrandProfiles().FindOne(ctx, filter).Decode(&brand)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"queries":    []LLMTestQuery{},
				"brand_name": req.Domain,
			})
			return
		}

		var queries []LLMTestQuery

		// Use existing target queries from brand profile
		for _, tq := range brand.TargetQueries {
			queries = append(queries, LLMTestQuery{
				Query:    tq.Query,
				Type:     tq.Type,
				Priority: tq.Priority,
			})
		}

		brandName := brand.BrandName
		if brandName == "" {
			brandName = req.Domain
		}

		// Auto-generate additional queries from brand profile fields
		seen := map[string]bool{}
		for _, q := range queries {
			seen[strings.ToLower(q.Query)] = true
		}
		addQuery := func(query, qType, priority string) {
			if len(queries) >= 12 {
				return
			}
			lower := strings.ToLower(query)
			if seen[lower] {
				return
			}
			seen[lower] = true
			queries = append(queries, LLMTestQuery{Query: query, Type: qType, Priority: priority})
		}

		// Brand awareness (always)
		if len(queries) == 0 {
			addQuery(fmt.Sprintf("What is %s?", brandName), "brand", "high")
		}

		// Category discovery
		for i, cat := range brand.Categories {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("What are the best %s tools?", cat), "category", "high")
		}

		// Product-specific queries
		for i, prod := range brand.Products {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("What is %s?", prod), "brand", "medium")
		}

		// Audience-targeted queries
		if brand.PrimaryAudience != "" && len(brand.Categories) > 0 {
			addQuery(fmt.Sprintf("Best %s for %s", brand.Categories[0], strings.ToLower(brand.PrimaryAudience)), "category", "medium")
		}

		// Competitor comparisons
		for i, comp := range brand.Competitors {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("How does %s compare to %s?", brandName, comp.Name), "comparison", "medium")
		}

		// Use case / discovery queries
		for i, uc := range brand.KeyUseCases {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("How do I %s?", strings.ToLower(uc)), "discovery", "medium")
		}

		// Key message verification
		for i, km := range brand.KeyMessages {
			if i >= 1 {
				break
			}
			addQuery(fmt.Sprintf("Is it true that %s?", strings.ToLower(km.Claim)), "brand", "low")
		}

		// Differentiator queries
		if len(brand.Differentiators) > 0 {
			addQuery(fmt.Sprintf("What makes %s different from competitors?", brandName), "comparison", "medium")
		}

		// Reddit presence queries
		if len(brand.Presence.Subreddits) > 0 {
			addQuery(fmt.Sprintf("What does Reddit say about %s?", brandName), "discovery", "low")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"queries":    queries,
			"brand_name": brandName,
		})
	}
}

// handleListProviderModels lists, per LLM provider, the models the tenant's key
// can use today (best first), asking each provider live and caching the answer.
// Providers without a key, or that can't be asked, show the built-in list.
func handleListProviderModels(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type modelInfo struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		type providerModels struct {
			ID      string      `json:"id"`
			Name    string      `json:"name"`
			Models  []modelInfo `json:"models"`
			Default string      `json:"default,omitempty"` // the LLM Test's default (see testDefaultModel)
			// DefaultNote says why Default was picked, for the model picker.
			DefaultNote string `json:"default_note,omitempty"`
			Live        bool   `json:"live"` // true when the list came from the provider
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		var ids []string
		for _, pid := range validProviderIDs() {
			if getProvider(pid) != nil {
				ids = append(ids, pid)
			}
		}
		result := make([]providerModels, len(ids))
		var wg sync.WaitGroup
		for i, pid := range ids {
			wg.Add(1)
			go func(i int, p LLMProvider) {
				defer wg.Done()
				key, err := resolveProviderKey(ctx, mongoDB, encKey, fallbackKey, saasEnabled, p.ProviderID())
				if err != nil {
					key = ""
				}
				models, live := availableModels(ctx, p, key)
				pm := providerModels{ID: p.ProviderID(), Name: p.Name(), Live: live, Models: []modelInfo{}}
				for _, m := range models {
					pm.Models = append(pm.Models, modelInfo{ID: m.ID, Name: m.Name})
				}
				pm.Default, pm.DefaultNote = testDefaultModel(p.ProviderID(), models)
				result[i] = pm
			}(i, getProvider(pid))
		}
		wg.Wait()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleGetLLMTest(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var test LLMTest
		err := mongoDB.LLMTests().FindOne(ctx,
			tenantFilter(r.Context(), bson.D{
				{Key: "domain", Value: domain},
				{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
			}),
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&test)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":"not found"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(test)
	}
}

func handleGetLLMTestHistory(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Return all test runs (excluding competitor tests), sorted newest first, limit 20
		// Project out full response text for compactness
		cursor, err := mongoDB.LLMTests().Find(ctx,
			tenantFilter(r.Context(), bson.D{
				{Key: "domain", Value: domain},
				{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
			}),
			options.Find().
				SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
				SetLimit(20).
				SetProjection(bson.D{
					{Key: "results.provider_results.response", Value: 0},
				}),
		)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var tests []LLMTest
		if err := cursor.All(ctx, &tests); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tests)
	}
}

func handleGetCompetitorTests(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Find all competitor tests where competitorOf matches the primary domain
		// Return only the most recent test per competitor domain
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.D{
				{Key: "competitorOf", Value: domain},
				{Key: "tenantId", Value: saas.TenantIDFromContext(r.Context())},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "generatedAt", Value: -1}}}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$domain"},
				{Key: "doc", Value: bson.D{{Key: "$first", Value: "$$ROOT"}}},
			}}},
			{{Key: "$replaceRoot", Value: bson.D{{Key: "newRoot", Value: "$doc"}}}},
			{{Key: "$project", Value: bson.D{
				{Key: "results.provider_results.response", Value: 0},
			}}},
		}

		cursor, err := mongoDB.LLMTests().Aggregate(ctx, pipeline)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var tests []LLMTest
		if err := cursor.All(ctx, &tests); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tests)
	}
}

func handleDeleteLLMTest(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		_, err := mongoDB.LLMTests().DeleteMany(ctx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
		)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

func handleLLMTest(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generous budget: Phase 1 runs many provider calls (in parallel) plus
		// a large evaluation pass. 15 min leaves headroom for big provider matrices.
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		startTime := time.Now()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()

		var req struct {
			Domain       string            `json:"domain"`
			Providers    []string          `json:"providers"`
			Queries      []LLMTestQuery    `json:"queries"`
			Models       map[string]string `json:"models"`        // providerID → modelID override
			CompetitorOf string            `json:"competitor_of"` // if set, this is a competitor test for the given primary domain
			Force        bool              `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)
		if req.Domain == "" {
			sendSSE(w, flusher, "error", map[string]string{"message": "Domain is required"})
			return
		}
		if len(req.Providers) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "At least one provider is required"})
			return
		}
		if len(req.Queries) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "At least one query is required"})
			return
		}
		if req.CompetitorOf == "" {
			// Keep the run's queries as the domain's saved list, so they're there
			// next time even if the page never got to save them.
			saveCtx, saveCancel := context.WithTimeout(r.Context(), 5*time.Second)
			if _, err := saveTestQueries(saveCtx, mongoDB, req.Domain, req.Queries); err != nil {
				log.Printf("LLM test: saving queries for %s: %v", req.Domain, err)
			}
			saveCancel()
		}

		notify := func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		}

		// Resolve each provider's key. A provider that can't be used is noted and
		// skipped rather than failing the whole test: the others still answer.
		var runs []*testProviderRun
		var unavailable []LLMTestUnavailable
		seen := map[string]bool{}
		for _, pid := range req.Providers {
			if seen[pid] {
				continue
			}
			seen[pid] = true
			p := getProvider(pid)
			if p == nil {
				unavailable = append(unavailable, LLMTestUnavailable{ProviderID: pid, ProviderName: pid, Reason: "it isn't a supported provider"})
				continue
			}
			key, err := resolveProviderKey(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled, pid)
			if err != nil || key == "" {
				reason := "no API key is configured for it"
				if err != nil && !errors.Is(err, errAPIKeyRequired) {
					log.Printf("LLM test: resolving %s key: %v", pid, err)
					reason = "its saved API key couldn't be read"
				}
				unavailable = append(unavailable, LLMTestUnavailable{ProviderID: pid, ProviderName: p.Name(), Reason: reason})
				notify(fmt.Sprintf("Skipping %s: %s.", p.Name(), reason))
				continue
			}
			runs = append(runs, &testProviderRun{provider: p, apiKey: key, notify: notify})
		}
		if len(runs) == 0 {
			sendSSE(w, flusher, "error", map[string]string{
				"message": "None of the selected providers can be used: " + describeUnavailable(unavailable) + ". Add or fix their API keys in Settings.",
			})
			return
		}

		// Pick each provider's model from what its key can use today: the one the
		// tenant chose, else the one the provider's chat app most likely uses.
		var wg sync.WaitGroup
		for _, run := range runs {
			wg.Add(1)
			go func(run *testProviderRun) {
				defer wg.Done()
				models, _ := availableModels(r.Context(), run.provider, run.apiKey)
				requested := req.Models[run.provider.ProviderID()]
				ordered, found := testModelOrder(run.provider.ProviderID(), models, requested)
				if !found && len(ordered) > 0 {
					notify(fmt.Sprintf("%s: %s isn't offered to this API key any more; using %s instead.", run.provider.Name(), requested, ordered[0].Name))
				}
				run.models = ordered
			}(run)
		}
		wg.Wait()
		usable := runs[:0]
		for _, run := range runs {
			if len(run.models) == 0 {
				unavailable = append(unavailable, LLMTestUnavailable{ProviderID: run.provider.ProviderID(), ProviderName: run.provider.Name(), Reason: "no models are available to its API key"})
				continue
			}
			usable = append(usable, run)
			notify(fmt.Sprintf("%s will answer with %s.", run.provider.Name(), run.models[0].Name))
		}
		runs = usable
		if len(runs) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "None of the selected providers can be used: " + describeUnavailable(unavailable) + "."})
			return
		}

		// Look up brand context
		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))
		brandName := req.Domain
		if brandInfo.Used {
			for _, line := range strings.Split(brandInfo.ContextString, "\n") {
				if strings.HasPrefix(line, "Company: ") {
					brandName = strings.TrimPrefix(line, "Company: ")
					break
				}
			}
		}

		// Phase 1: ask every provider every query. Calls run concurrently (capped),
		// interleaved across providers so one slow or rate-limited provider doesn't
		// hold up the rest. Per-call failures are recorded, never fatal.
		type queryJob struct {
			run      *testProviderRun
			queryIdx int
			query    string
		}
		var jobs []queryJob
		for qi, q := range req.Queries {
			for _, run := range runs {
				jobs = append(jobs, queryJob{run: run, queryIdx: qi, query: q.Query})
			}
		}
		totalCalls := len(jobs)
		notify(fmt.Sprintf("Testing %d queries across %d providers (%d total calls)...", len(req.Queries), len(runs), totalCalls))

		responses := make([]testRawResponse, 0, totalCalls)
		var respMu sync.Mutex
		completedCalls := 0

		var g errgroup.Group
		g.SetLimit(6) // cap concurrent provider calls to stay within BYOK rate limits
		for _, j := range jobs {
			j := j
			g.Go(func() error {
				resp, model, err, skipped := j.run.ask(r.Context(), j.query, 4096)
				j.run.record(err)
				respMu.Lock()
				completedCalls++
				n := completedCalls
				responses = append(responses, testRawResponse{
					providerID:   j.run.provider.ProviderID(),
					providerName: j.run.provider.Name(),
					model:        model.Name,
					queryIdx:     j.queryIdx,
					response:     resp,
					err:          err,
				})
				respMu.Unlock()
				if skipped {
					return nil
				}
				outcome := ""
				if err != nil {
					outcome = " — no answer"
				}
				notify(fmt.Sprintf("[%d/%d] %s (%s): \"%s\"%s", n, totalCalls, j.run.provider.Name(), model.Name, truncateStr(j.query, 60), outcome))
				return nil
			})
		}
		_ = g.Wait()
		if r.Context().Err() != nil {
			return // client went away or the budget ran out
		}

		// Providers that answered nothing are reported, not scored as zero.
		var active []*testProviderRun
		var failedNames []string
		dead := map[string]bool{}
		for _, run := range runs {
			if run.answered > 0 {
				active = append(active, run)
				continue
			}
			dead[run.provider.ProviderID()] = true
			failedNames = append(failedNames, run.provider.Name())
			unavailable = append(unavailable, LLMTestUnavailable{
				ProviderID:   run.provider.ProviderID(),
				ProviderName: run.provider.Name(),
				Model:        run.currentModel().Name,
				Reason:       providerFailureReason(run.lastErr),
			})
		}
		if len(active) == 0 {
			sendSSE(w, flusher, "error", map[string]string{
				"message": "No provider could answer: " + describeUnavailable(unavailable) + ".",
			})
			return
		}
		if len(failedNames) > 0 {
			var okNames []string
			for _, run := range active {
				okNames = append(okNames, run.provider.Name())
			}
			notify(fmt.Sprintf("Continuing with %s; %s couldn't answer.", joinNames(okNames), joinNames(failedNames)))
		}

		// Phase 2: evaluate the answers. Try the tenant's primary provider first,
		// then the providers that just answered (their keys are known to work).
		evalPrompt := buildTestEvaluationPrompt(brandName, req.Domain, brandInfo, req.Queries, responses)
		type evalCandidate struct {
			provider LLMProvider
			apiKey   string
			model    string
		}
		// evalModelFor avoids a model this run already found retired.
		evalModelFor := func(p LLMProvider, key, preferred string) string {
			m := pickEvalModel(r.Context(), p, key, preferred)
			for _, run := range active {
				if run.provider.ProviderID() == p.ProviderID() && run.apiKey == key && run.movedPast(m) {
					return run.currentModel().ID
				}
			}
			return m
		}
		var cands []evalCandidate
		primary, primKey, primModel, primErr := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if primErr == nil && primary != nil && primKey != "" && !dead[primary.ProviderID()] {
			cands = append(cands, evalCandidate{primary, primKey, evalModelFor(primary, primKey, primModel)})
		} else if primErr != nil && !errors.Is(primErr, errAPIKeyRequired) {
			log.Printf("LLM test: failed to resolve primary LLM: %v", primErr)
		}
		byAnswers := append([]*testProviderRun(nil), active...)
		sort.SliceStable(byAnswers, func(i, j int) bool { return byAnswers[i].answered > byAnswers[j].answered })
		for _, run := range byAnswers {
			if len(cands) > 0 && cands[0].provider.ProviderID() == run.provider.ProviderID() && cands[0].apiKey == run.apiKey {
				continue
			}
			cands = append(cands, evalCandidate{run.provider, run.apiKey, evalModelFor(run.provider, run.apiKey, "")})
		}

		var evalResult testEvaluation
		var evalErrs []string
		evaluated := false
		for _, c := range cands {
			notify(fmt.Sprintf("Evaluating responses against brand profile (%s)...", c.provider.Name()))
			evalResp, err := callWithRetry(r.Context(), c.provider, c.apiKey, c.model, evalPrompt, 16384)
			if err != nil && classifyProviderError(err) == failModel {
				// The chosen model is gone; try the best one this key can use.
				if models, _ := availableModels(r.Context(), c.provider, c.apiKey); len(models) > 0 && models[0].ID != c.model {
					evalResp, err = callWithRetry(r.Context(), c.provider, c.apiKey, models[0].ID, evalPrompt, 16384)
				}
			}
			if err != nil {
				log.Printf("LLM test evaluation with %s (%s) failed: %v", c.provider.Name(), c.model, err)
				evalErrs = append(evalErrs, fmt.Sprintf("%s: %s", c.provider.Name(), truncateStr(err.Error(), 200)))
				continue
			}
			if err := parseTestEvaluation(evalResp, &evalResult); err != nil {
				log.Printf("LLM test evaluation from %s unreadable: %v — raw: %s", c.provider.Name(), err, truncateStr(evalResp, 200))
				evalErrs = append(evalErrs, fmt.Sprintf("%s: the evaluation couldn't be read", c.provider.Name()))
				continue
			}
			evaluated = true
			break
		}
		if !evaluated {
			msg := "Evaluation failed: " + strings.Join(evalErrs, "; ")
			if len(cands) == 0 {
				msg = "No API key available to evaluate results. Add an API key for your primary provider (or one of the providers you're testing) in Settings."
			}
			sendSSE(w, flusher, "error", map[string]string{"message": msg})
			return
		}

		// Index the evaluations by query and provider. Evaluators sometimes give
		// the provider's name instead of its ID, so accept either.
		idByName := map[string]string{}
		for _, run := range runs {
			idByName[strings.ToLower(run.provider.Name())] = run.provider.ProviderID()
			idByName[strings.ToLower(run.provider.ProviderID())] = run.provider.ProviderID()
		}
		type evalKey struct {
			queryIdx   int
			providerID string
		}
		evals := map[evalKey]int{}
		for i, e := range evalResult.Evaluations {
			if pid, ok := idByName[strings.ToLower(strings.TrimSpace(e.ProviderID))]; ok {
				evals[evalKey{e.QueryIndex, pid}] = i
			}
		}

		// Build per-query results for the providers that answered, in the order
		// the providers were selected.
		order := map[string]int{}
		for i, run := range runs {
			order[run.provider.ProviderID()] = i
		}
		sort.SliceStable(responses, func(i, j int) bool {
			if responses[i].queryIdx != responses[j].queryIdx {
				return responses[i].queryIdx < responses[j].queryIdx
			}
			return order[responses[i].providerID] < order[responses[j].providerID]
		})
		queryResults := make([]LLMTestQueryResult, len(req.Queries))
		for qi, q := range req.Queries {
			queryResults[qi] = LLMTestQueryResult{Query: q, ProviderResults: []LLMTestProviderResult{}}
		}
		type tally struct{ score, mentioned, recommended, accurate, sentiment, answered int }
		tallies := map[string]*tally{}
		for _, run := range active {
			tallies[run.provider.ProviderID()] = &tally{}
		}
		for _, resp := range responses {
			t := tallies[resp.providerID]
			if t == nil {
				continue // provider answered nothing; listed as unavailable instead
			}
			pr := LLMTestProviderResult{
				ProviderID:   resp.providerID,
				ProviderName: resp.providerName,
				Model:        resp.model,
				Response:     resp.response,
			}
			if resp.err != nil {
				pr.Response = "Error: " + truncateStr(resp.err.Error(), 300)
				pr.Sentiment = "absent"
				pr.Accuracy = "not_applicable"
			} else {
				t.answered++
				if i, ok := evals[evalKey{resp.queryIdx, resp.providerID}]; ok {
					e := evalResult.Evaluations[i]
					pr.Mentioned, pr.Recommended = e.Mentioned, e.Recommended
					pr.Sentiment, pr.Accuracy, pr.Score = e.Sentiment, e.Accuracy, e.Score
				}
				t.score += pr.Score
				if pr.Mentioned {
					t.mentioned++
				}
				if pr.Recommended {
					t.recommended++
				}
				if pr.Accuracy == "accurate" {
					t.accurate++
				}
				switch pr.Sentiment {
				case "positive":
					t.sentiment += 100
				case "neutral":
					t.sentiment += 50
				case "negative":
					t.sentiment += 10
				}
			}
			queryResults[resp.queryIdx].ProviderResults = append(queryResults[resp.queryIdx].ProviderResults, pr)
		}

		// Per-provider summaries; rates are over the queries each provider answered.
		var summaries []LLMTestSummary
		overallTotal := 0
		for _, run := range active {
			t := tallies[run.provider.ProviderID()]
			s := LLMTestSummary{
				ProviderID:   run.provider.ProviderID(),
				ProviderName: run.provider.Name(),
				Model:        run.currentModel().Name,
				Answered:     t.answered,
				Failed:       run.failed,
			}
			if n := t.answered; n > 0 {
				s.OverallScore = t.score / n
				s.MentionRate = t.mentioned * 100 / n
				s.RecommendRate = t.recommended * 100 / n
				s.AccuracyRate = t.accurate * 100 / n
				s.SentimentScore = t.sentiment / n
			}
			overallTotal += s.OverallScore
			summaries = append(summaries, s)
		}
		overallScore := overallTotal / len(summaries)

		// Determine run number
		runNumber := 1
		runCtx, runCancel := context.WithTimeout(r.Context(), 5*time.Second)
		var prevTest LLMTest
		if err := mongoDB.LLMTests().FindOne(runCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			options.FindOne().SetSort(bson.D{{Key: "runNumber", Value: -1}}),
		).Decode(&prevTest); err == nil {
			runNumber = prevTest.RunNumber + 1
		}
		runCancel()

		test := LLMTest{
			TenantID:             saas.TenantIDFromContext(r.Context()),
			Domain:               req.Domain,
			BrandName:            brandName,
			RunNumber:            runNumber,
			CompetitorOf:         normalizeDomain(req.CompetitorOf),
			Queries:              req.Queries,
			Results:              queryResults,
			ProviderSummaries:    summaries,
			UnavailableProviders: unavailable,
			OverallScore:         overallScore,
			BrandContextUsed:     brandInfo.Used,
			GeneratedAt:          time.Now(),
		}

		// Save to DB (insert new document for history tracking)
		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		insertResult, saveErr := mongoDB.LLMTests().InsertOne(saveCtx, test)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save LLM test: %v", saveErr)
		} else if oid, ok := insertResult.InsertedID.(primitive.ObjectID); ok {
			test.ID = oid
		}

		doneMsg := fmt.Sprintf("Test complete — Overall Score: %d/100", overallScore)
		if len(unavailable) > 0 {
			var names []string
			for _, u := range unavailable {
				names = append(names, u.ProviderName)
			}
			doneMsg += fmt.Sprintf(" (from the providers that answered; %s couldn't be used)", joinNames(names))
		}
		notify(doneMsg)

		testJSON, _ := json.Marshal(test)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(testJSON),
		})
		trackServerEvent(mongoDB, "custom.server.test.complete", saas.UserIDFromContext(r.Context()), saas.TenantIDFromContext(r.Context()), map[string]interface{}{"domain": req.Domain, "duration_ms": time.Since(startTime).Milliseconds(), "provider_count": len(req.Providers), "query_count": len(req.Queries), "unavailable_count": len(unavailable)})
	}
}

func handleVisibilityScore(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		tenantID := saas.TenantIDFromContext(r.Context())

		type component struct {
			Name      string  `json:"name"`
			Score     int     `json:"score"`
			Weight    float64 `json:"weight"`
			Available bool    `json:"available"`
		}

		components := []component{
			{Name: "Optimization", Weight: 0.30},
			{Name: "Video Authority", Weight: 0.20},
			{Name: "Reddit Authority", Weight: 0.20},
			{Name: "Search Visibility", Weight: 0.15},
			{Name: "LLM Test", Weight: 0.15},
		}

		// 1. Optimization average score
		cursor, err := mongoDB.Optimizations().Find(ctx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
			options.Find().SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		)
		if err == nil {
			var opts []struct {
				Result struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
			}
			if cursor.All(ctx, &opts) == nil && len(opts) > 0 {
				total := 0
				for _, o := range opts {
					total += o.Result.OverallScore
				}
				components[0].Score = total / len(opts)
				components[0].Available = true
			}
		}

		// 2. Video authority (latest)
		var va struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.VideoAnalyses().FindOne(ctx,
			bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&va) == nil && va.Result != nil {
			components[1].Score = va.Result.OverallScore
			components[1].Available = true
		}

		// 3. Reddit authority (latest)
		var ra struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.RedditAnalyses().FindOne(ctx,
			bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&ra) == nil && ra.Result != nil {
			components[2].Score = ra.Result.OverallScore
			components[2].Available = true
		}

		// 4. Search visibility (latest)
		var sa struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.SearchAnalyses().FindOne(ctx,
			bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&sa) == nil && sa.Result != nil {
			components[3].Score = sa.Result.OverallScore
			components[3].Available = true
		}

		// 5. LLM Test (latest non-competitor)
		var lt struct {
			OverallScore int `bson:"overallScore"`
		}
		if mongoDB.LLMTests().FindOne(ctx,
			bson.D{
				{Key: "domain", Value: domain},
				{Key: "tenantId", Value: tenantID},
				{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
			},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "overallScore", Value: 1}}),
		).Decode(&lt) == nil {
			components[4].Score = lt.OverallScore
			components[4].Available = true
		}

		// Compute weighted score (re-weight proportionally for available components only)
		totalWeight := 0.0
		weightedSum := 0.0
		for _, c := range components {
			if c.Available {
				totalWeight += c.Weight
				weightedSum += float64(c.Score) * c.Weight
			}
		}

		score := 0
		if totalWeight > 0 {
			score = int(weightedSum / totalWeight)
		}

		availableCount := 0
		for _, c := range components {
			if c.Available {
				availableCount++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"score":      score,
			"components": components,
			"available":  availableCount,
			"total":      len(components),
		})
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func buildTestEvaluationPrompt(brandName, domain string, brandInfo BrandContextInfo, queries []LLMTestQuery, responses []testRawResponse) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`You are evaluating how well different AI assistants represent a brand in their responses to user queries.

## Brand Information
**Brand**: %s
**Domain**: %s
`, brandName, domain))

	if brandInfo.Used {
		sb.WriteString(fmt.Sprintf("\n%s\n", brandInfo.ContextString))
	}

	sb.WriteString(`
## Instructions

For each query-response pair, evaluate:
1. **mentioned**: Is the brand explicitly mentioned by name? (boolean)
2. **recommended**: Is the brand recommended, suggested, or positioned as a good option? (boolean)
3. **sentiment**: Overall sentiment toward the brand in the response. Use "absent" if the brand isn't mentioned at all. (positive/neutral/negative/absent)
4. **accuracy**: Are factual claims about the brand accurate based on the brand information above? Use "not_applicable" if brand isn't discussed. (accurate/partially_accurate/inaccurate/not_applicable)
5. **score**: Overall brand representation quality 0-100. Consider: Is the brand mentioned? Positioned favorably? Information accurate? Recommended when relevant?

## Query-Response Pairs

`)

	for _, resp := range responses {
		if resp.err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf(`### Query %d (asked to %s, provider_id: %s)
**Query**: %s
**Response**: %s

`, resp.queryIdx, resp.providerName, resp.providerID, queries[resp.queryIdx].Query, resp.response))
	}

	sb.WriteString(`## Output Format

Return ONLY a JSON object with this exact structure:
{"evaluations": [{"query_index": 0, "provider_id": "anthropic", "mentioned": true, "recommended": false, "sentiment": "neutral", "accuracy": "accurate", "score": 65}, ...]}

Include one evaluation entry per query-response pair. Return ONLY the JSON object, no markdown fencing.`)

	return sb.String()
}

// ── Background Cleanup Jobs ──────────────────────────────────────────────

// runCleanupJobs runs periodic maintenance tasks every 6 hours:
// - Prune old health checks (>30 days)
// - Prune archived todos (>90 days)
// - Cap analyses per domain/tenant (keep newest 20)
// - Cap optimizations per domain/tenant (keep newest 50)
// - Refresh stale popular screenshots (>7 days, max 3 per cycle)
func runCleanupJobs(mongoDB *MongoDB) {
	// Run once immediately at startup, then every 6 hours
	doCleanup(mongoDB)
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		doCleanup(mongoDB)
	}
}

func doCleanup(mongoDB *MongoDB) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 1. Delete health checks older than 30 days
	cutoff30d := time.Now().Add(-30 * 24 * time.Hour)
	res, err := mongoDB.HealthChecks().DeleteMany(ctx, bson.M{"checkedAt": bson.M{"$lt": cutoff30d}})
	if err == nil && res.DeletedCount > 0 {
		log.Printf("Cleanup: deleted %d health checks older than 30 days", res.DeletedCount)
	}

	// 2. Delete archived todos older than 90 days
	cutoff90d := time.Now().Add(-90 * 24 * time.Hour)
	res, err = mongoDB.Todos().DeleteMany(ctx, bson.M{
		"status":     "archived",
		"archivedAt": bson.M{"$lt": cutoff90d},
	})
	if err == nil && res.DeletedCount > 0 {
		log.Printf("Cleanup: deleted %d archived todos older than 90 days", res.DeletedCount)
	}

	// 3. Cap analyses to 20 per domain/tenant
	capCollection(ctx, mongoDB.Analyses(), "domain", "tenantId", "createdAt", 20)

	// 4. Cap optimizations to 50 per domain/tenant
	capCollection(ctx, mongoDB.Optimizations(), "domain", "tenantId", "createdAt", 50)

	// 5. Refresh stale popular screenshots
	refreshStaleScreenshots(mongoDB)
}

// capCollection caps documents per domain+tenant, keeping the newest `limit` documents.
func capCollection(ctx context.Context, coll *mongo.Collection, domainField, tenantField, sortField string, limit int) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "domain", Value: "$" + domainField},
				{Key: "tenantId", Value: "$" + tenantField},
			}},
			{Key: "count", Value: bson.M{"$sum": 1}},
		}}},
		{{Key: "$match", Value: bson.M{"count": bson.M{"$gt": limit}}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)

	var totalDeleted int64
	for cursor.Next(ctx) {
		var result struct {
			ID struct {
				Domain   string `bson:"domain"`
				TenantID string `bson:"tenantId"`
			} `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}

		excess := result.Count - limit
		if excess <= 0 {
			continue
		}

		// Find the oldest IDs to delete
		filter := bson.M{domainField: result.ID.Domain, tenantField: result.ID.TenantID}
		oldCursor, err := coll.Find(ctx, filter,
			options.Find().SetSort(bson.D{{Key: sortField, Value: 1}}).SetLimit(int64(excess)).SetProjection(bson.M{"_id": 1}))
		if err != nil {
			continue
		}
		var ids []primitive.ObjectID
		for oldCursor.Next(ctx) {
			var doc struct {
				ID primitive.ObjectID `bson:"_id"`
			}
			if err := oldCursor.Decode(&doc); err == nil {
				ids = append(ids, doc.ID)
			}
		}
		oldCursor.Close(ctx)

		if len(ids) > 0 {
			delRes, err := coll.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": ids}})
			if err == nil {
				totalDeleted += delRes.DeletedCount
			}
		}
	}
	if totalDeleted > 0 {
		log.Printf("Cleanup: capped %s, deleted %d excess documents", coll.Name(), totalDeleted)
	}
}

// refreshStaleScreenshots re-captures popular domain screenshots older than 7 days.
func refreshStaleScreenshots(mongoDB *MongoDB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	cursor, err := mongoDB.DomainShares().Find(ctx, bson.M{"visibility": "popular"})
	if err != nil {
		return
	}
	var shares []DomainShare
	cursor.All(ctx, &shares)
	cursor.Close(ctx)

	refreshed := 0
	for _, s := range shares {
		if refreshed >= 3 {
			break
		}
		var ss BrandScreenshot
		err := mongoDB.BrandScreenshots().FindOne(ctx, bson.M{"domain": s.Domain}).Decode(&ss)
		if err != nil || ss.CapturedAt.Before(cutoff) {
			go captureBrandScreenshot(mongoDB, s.Domain, s.TenantID)
			refreshed++
		}
	}
	if refreshed > 0 {
		log.Printf("Cleanup: refreshing %d stale screenshots", refreshed)
	}
}
