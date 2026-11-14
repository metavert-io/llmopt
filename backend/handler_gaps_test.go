package main

// Tests targeting low-coverage branches in main.go and report_pdf.go functions.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── computeFingerprint ───────────────────────────────────────────────────────

func TestComputeFingerprint_Empty(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	fp := computeFingerprint(ctx, db, "fingerprint-empty.com", tenantCtx)
	if fp.AnalysisExists {
		t.Error("expected AnalysisExists=false for empty domain")
	}
	if fp.VideoExists {
		t.Error("expected VideoExists=false for empty domain")
	}
	if fp.RedditExists {
		t.Error("expected RedditExists=false for empty domain")
	}
	if fp.SearchExists {
		t.Error("expected SearchExists=false for empty domain")
	}
	if fp.OptimizationCount != 0 {
		t.Errorf("expected OptimizationCount=0, got %d", fp.OptimizationCount)
	}
}

func TestComputeFingerprint_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Seed analyses and optimizations for this domain
	domain := "fingerprint-full.com"
	db.Analyses().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    domain,
		"createdAt": time.Now(),
	})
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    domain,
		"createdAt": time.Now(),
	})

	fp := computeFingerprint(ctx, db, domain, tenantCtx)
	if !fp.AnalysisExists {
		t.Error("expected AnalysisExists=true after seeding analysis")
	}
	if fp.AnalysisCreatedAt == nil {
		t.Error("expected AnalysisCreatedAt to be set")
	}
	if fp.OptimizationCount != 1 {
		t.Errorf("expected OptimizationCount=1, got %d", fp.OptimizationCount)
	}
	if fp.LatestOptimizationAt == nil {
		t.Error("expected LatestOptimizationAt to be set")
	}
}

// ── resolveAnthropicKey ──────────────────────────────────────────────────────

func TestResolveAnthropicKey_FallbackKey(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// In non-SaaS mode with a fallback key, resolveAnthropicKey should return it
	key, err := resolveAnthropicKey(ctx, db, testEncKey(), "test-fallback-key", false)
	if err != nil {
		t.Fatalf("expected no error with fallback key, got: %v", err)
	}
	if key != "test-fallback-key" {
		t.Errorf("expected 'test-fallback-key', got %q", key)
	}
}

func TestResolveAnthropicKey_NoKey(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant-no-key", "test-user")

	// In SaaS mode with no stored key → error
	// (non-SaaS mode with empty fallback returns "" with no error, per resolveProviderKey logic)
	_, err := resolveAnthropicKey(ctx, db, testEncKey(), "", true)
	if err == nil {
		t.Error("expected error with no stored key in SaaS mode")
	}
}

// ── seedEventDefinitions ─────────────────────────────────────────────────────

func TestSeedEventDefinitions(t *testing.T) {
	db := testMongoDB(t)

	// Call seedEventDefinitions - it upserts event definitions into MongoDB
	seedEventDefinitions(db)

	// Verify at least some documents were inserted
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := db.Database.Collection("event_definitions").CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatalf("error counting event_definitions: %v", err)
	}
	if count == 0 {
		t.Error("expected seedEventDefinitions to insert event definitions")
	}
}

// ── handleListTodos: missing filters ─────────────────────────────────────────

func TestHandleListTodos_OptimizationIDFilter(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	optID := primitive.NewObjectID()
	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "optimizationId": optID, "status": "todo", "action": "Task A", "createdAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "status": "todo", "action": "Task B", "createdAt": time.Now()},
	})

	handler := handleListTodos(db)
	req := testRequest(t, "GET", "/api/todos?optimization_id="+optID.Hex(), nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []TodoItem
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 1 {
		t.Errorf("expected 1 todo matching optimization_id filter, got %d", len(todos))
	}
}

func TestHandleListTodos_SourceTypeFilter(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "sourceType": "video", "status": "todo", "action": "Video Task", "createdAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "sourceType": "optimization", "status": "todo", "action": "Opt Task", "createdAt": time.Now()},
	})

	handler := handleListTodos(db)
	req := testRequest(t, "GET", "/api/todos?source_type=video", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []TodoItem
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 1 {
		t.Errorf("expected 1 todo with source_type=video, got %d", len(todos))
	}
}

func TestHandleListTodos_InvalidOptimizationID(t *testing.T) {
	db := testMongoDB(t)
	// Invalid hex optimization_id → filter skips the field, returns all todos
	handler := handleListTodos(db)
	req := testRequest(t, "GET", "/api/todos?optimization_id=not-a-hex", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleUpdateTodo: missing status paths ────────────────────────────────────

func TestHandleUpdateTodo_Backlogged(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	res, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"status":    "todo",
		"action":    "Backlog me",
		"createdAt": time.Now(),
	})
	oid := res.InsertedID.(primitive.ObjectID)

	handler := handleUpdateTodo(db)
	req := testRequest(t, "PATCH", "/api/todos/"+oid.Hex(), map[string]string{"status": "backlogged"})
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	var todo bson.M
	db.Todos().FindOne(ctx, bson.M{"_id": oid}).Decode(&todo)
	if todo["status"] != "backlogged" {
		t.Errorf("expected status=backlogged, got %q", todo["status"])
	}
}

func TestHandleUpdateTodo_Archived(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	res, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"status":    "todo",
		"action":    "Archive me",
		"createdAt": time.Now(),
	})
	oid := res.InsertedID.(primitive.ObjectID)

	handler := handleUpdateTodo(db)
	req := testRequest(t, "PATCH", "/api/todos/"+oid.Hex(), map[string]string{"status": "archived"})
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleUpdateTodo_InvalidStatus(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	res, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId": "test-tenant",
		"status":   "todo",
		"action":   "Task",
	})
	oid := res.InsertedID.(primitive.ObjectID)

	handler := handleUpdateTodo(db)
	req := testRequest(t, "PATCH", "/api/todos/"+oid.Hex(), map[string]string{"status": "invalid"})
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleUpdateTodo_NotFound(t *testing.T) {
	db := testMongoDB(t)
	nonExistentID := primitive.NewObjectID()

	handler := handleUpdateTodo(db)
	req := testRequest(t, "PATCH", "/api/todos/"+nonExistentID.Hex(), map[string]string{"status": "todo"})
	req.SetPathValue("id", nonExistentID.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

// ── handleBulkArchiveTodos: missing source_type paths ────────────────────────

func TestHandleBulkArchiveTodos_VideoSourceType(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "status": "todo", "domain": "bulk.com", "sourceType": "video", "action": "Vid Task"},
		bson.M{"tenantId": "test-tenant", "status": "todo", "domain": "bulk.com", "sourceType": "optimization", "action": "Opt Task"},
	})

	handler := handleBulkArchiveTodos(db)
	req := testRequest(t, "POST", "/api/todos/archive", map[string]string{
		"domain":      "bulk.com",
		"source_type": "video",
	})
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["archived_count"].(float64) != 1 {
		t.Errorf("expected 1 archived (video only), got %v", result["archived_count"])
	}
}

func TestHandleBulkArchiveTodos_OptimizationSourceType(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	question := "How does bulk-opt.com rank?"
	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "status": "todo", "domain": "bulk-opt.com", "question": question, "sourceType": "", "action": "Opt Todo"},
		bson.M{"tenantId": "test-tenant", "status": "todo", "domain": "bulk-opt.com", "question": question, "sourceType": "video", "action": "Video Todo"},
	})

	handler := handleBulkArchiveTodos(db)
	req := testRequest(t, "POST", "/api/todos/archive", map[string]any{
		"domain":      "bulk-opt.com",
		"source_type": "optimization",
		"question":    question,
	})
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	// Only the non-video todo matching the question should be archived
	if result["archived_count"].(float64) != 1 {
		t.Errorf("expected 1 archived (non-video opt), got %v", result["archived_count"])
	}
}

func TestHandleBulkArchiveTodos_RedditSourceType(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "status": "todo", "domain": "bulk-reddit.com", "sourceType": "reddit", "action": "Reddit Task"},
		bson.M{"tenantId": "test-tenant", "status": "todo", "domain": "bulk-reddit.com", "sourceType": "video", "action": "Video Task"},
	})

	handler := handleBulkArchiveTodos(db)
	req := testRequest(t, "POST", "/api/todos/archive", map[string]string{
		"domain":      "bulk-reddit.com",
		"source_type": "reddit",
	})
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["archived_count"].(float64) != 1 {
		t.Errorf("expected 1 reddit archived, got %v", result["archived_count"])
	}
}

// ── handleGetDomainShare: found with shareID path ────────────────────────────

func TestHandleGetDomainShare_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.DomainShares().InsertOne(ctx, bson.M{
		"tenantId":   "test-tenant",
		"domain":     "share-found.com",
		"shareId":    "share-abc-123",
		"visibility": "public",
		"viewCount":  10,
		"createdAt":  time.Now(),
	})

	handler := handleGetDomainShare(db)
	req := testRequest(t, "GET", "/api/share/share-found.com", nil)
	req.SetPathValue("domain", "share-found.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["visibility"] != "public" {
		t.Errorf("expected visibility=public, got %v", resp["visibility"])
	}
	if resp["share_id"] != "share-abc-123" {
		t.Errorf("expected share_id=share-abc-123, got %v", resp["share_id"])
	}
	// share_url should be set since shareId is non-empty
	if resp["share_url"] == "" {
		t.Error("expected share_url to be set")
	}
}

// ── handleSetDomainShare: missing branches ────────────────────────────────────

func TestHandleSetDomainShare_NoDomainData(t *testing.T) {
	db := testMongoDB(t)

	handler := handleSetDomainShare(db)
	body := map[string]string{"visibility": "public"}
	req := testRequest(t, "PUT", "/api/share/nodomain.com", body)
	req.SetPathValue("domain", "nodomain.com")
	w := httptest.NewRecorder()
	handler(w, req)

	// No analyses/optimizations/brand for this domain → 404
	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleSetDomainShare_Private(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Analyses().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "share-private.com",
		"createdAt": time.Now(),
	})

	handler := handleSetDomainShare(db)
	body := map[string]string{"visibility": "private"}
	req := testRequest(t, "PUT", "/api/share/share-private.com", body)
	req.SetPathValue("domain", "share-private.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["share_id"] != "" {
		t.Errorf("expected empty share_id for private, got %v", resp["share_id"])
	}
}

// ── handleGetOptimization: invalid path params ────────────────────────────────

func TestHandleGetOptimization_InvalidID(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetOptimization(db)
	req := testRequest(t, "GET", "/api/analyses/invalid/questions/0/optimization", nil)
	req.SetPathValue("id", "invalid")
	req.SetPathValue("idx", "0")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleGetOptimization_InvalidIdx(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetOptimization(db)
	oid := primitive.NewObjectID()
	req := testRequest(t, "GET", "/api/analyses/"+oid.Hex()+"/questions/abc/optimization", nil)
	req.SetPathValue("id", oid.Hex())
	req.SetPathValue("idx", "abc")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── handleHealthHistory: with data ───────────────────────────────────────────

func TestHandleHealthHistory_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed health check history
	db.Database.Collection("provider_health_checks").InsertMany(ctx, []any{
		bson.M{
			"tenantId":   "test-tenant",
			"provider":   "anthropic",
			"status":     "available",
			"latency_ms": 150,
			"checkedAt":  time.Now().Add(-1 * time.Hour),
		},
		bson.M{
			"tenantId":   "test-tenant",
			"provider":   "anthropic",
			"status":     "available",
			"latency_ms": 200,
			"checkedAt":  time.Now().Add(-2 * time.Hour),
		},
	})

	handler := handleHealthHistory(db)
	req := testRequest(t, "GET", "/api/provider-health/history", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleProviderHealthCheck: YouTube key path ───────────────────────────────

func TestHandleProviderHealthCheck_YouTubeInactive(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed an inactive YouTube key
	plainKey := "fake-youtube-key"
	encKey := testEncKey()
	encrypted, _ := encryptSecret(plainKey, encKey)
	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:     "test-tenant",
		Provider:     "youtube",
		EncryptedKey: encrypted,
		Status:       "inactive",
		CreatedAt:    time.Now(),
	})

	handler := handleProviderHealthCheck(db, encKey)
	req := testRequest(t, "POST", "/api/provider-health/check", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	models := resp["models"].([]any)
	if len(models) == 0 {
		t.Error("expected at least one model in health check response")
	}
	// YouTube inactive should have status "error"
	found := false
	for _, m := range models {
		mo := m.(map[string]any)
		if mo["provider"] == "youtube" {
			found = true
			if mo["status"] != "error" {
				t.Errorf("expected youtube status=error for inactive key, got %v", mo["status"])
			}
		}
	}
	if !found {
		t.Error("expected youtube provider in health check results")
	}
}

// ── handleGetPopularDomains: verify cache invalidation ───────────────────────

func TestHandleGetPopularDomains_CacheReset(t *testing.T) {
	db := testMongoDB(t)

	// Reset the popular domains cache
	popularDomainsCache.Lock()
	popularDomainsCache.data = nil
	popularDomainsCache.expiresAt = time.Time{}
	popularDomainsCache.Unlock()

	handler := handleGetPopularDomains(db)
	req := httptest.NewRequest("GET", "/api/popular", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result []any
	json.NewDecoder(w.Body).Decode(&result)
	// No popular domains seeded, so should return empty
	if result == nil {
		t.Error("expected non-nil result (empty array)")
	}
}

// ── resolveOwnerName: success paths ──────────────────────────────────────────

func TestResolveOwnerName_MembershipButNoUser(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	tenantOID := primitive.NewObjectID()
	userOID := primitive.NewObjectID()

	// Seed a membership but no matching user
	db.Database.Collection("tenant_memberships").InsertOne(ctx, bson.M{
		"tenantId": tenantOID,
		"userId":   userOID,
		"role":     "owner",
	})
	// No user document → should return ""
	result := resolveOwnerName(ctx, db, tenantOID.Hex())
	if result != "" {
		t.Errorf("expected empty string when user not found, got %q", result)
	}
}

func TestResolveOwnerName_Success(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	tenantOID := primitive.NewObjectID()
	userOID := primitive.NewObjectID()

	db.Database.Collection("tenant_memberships").InsertOne(ctx, bson.M{
		"tenantId": tenantOID,
		"userId":   userOID,
		"role":     "owner",
	})
	db.Database.Collection("users").InsertOne(ctx, bson.M{
		"_id":         userOID,
		"displayName": "Example User",
	})

	result := resolveOwnerName(ctx, db, tenantOID.Hex())
	if result != "Example User" {
		t.Errorf("expected 'Example User', got %q", result)
	}
}

// ── handleGetSharedDomain: with data ─────────────────────────────────────────

func TestHandleGetSharedDomain_WithAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant-shared", "test-user")

	shareID := "shared-analysis-xyz"
	domain := "shared-test.com"

	db.DomainShares().InsertOne(ctx, DomainShare{
		ID:         primitive.NewObjectID(),
		TenantID:   "test-tenant-shared",
		Domain:     domain,
		ShareID:    shareID,
		Visibility: "public",
		CreatedAt:  time.Now(),
	})

	// Seed an analysis for this domain
	db.Analyses().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant-shared",
		"domain":    domain,
		"createdAt": time.Now(),
		"result": bson.M{
			"siteSummary": "A shared test domain",
			"questions":   bson.A{},
		},
	})

	handler := handleGetSharedDomain(db)
	req := httptest.NewRequest("GET", "/s/"+shareID, nil)
	req.SetPathValue("shareId", shareID)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}
