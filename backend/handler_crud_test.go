package main

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

// ── Analysis CRUD ───────────────────────────────────────────────────────

func TestHandleListAnalyses_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListAnalyses(db)

	req := testRequest(t, "GET", "/api/analyses", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []AnalysisSummary
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestHandleListAnalyses_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	// Seed two analyses for the test tenant
	db.Analyses().InsertMany(ctx, []any{
		Analysis{
			TenantID:  "test-tenant",
			Domain:    "example.com",
			Model:     "test-model",
			CreatedAt: time.Now().Add(-1 * time.Hour),
			Result: AnalysisResult{
				SiteSummary: "First analysis",
				Questions:   []Question{{Question: "Q1", Relevance: "high"}},
			},
		},
		Analysis{
			TenantID:  "test-tenant",
			Domain:    "other.com",
			Model:     "test-model",
			CreatedAt: time.Now(),
			Result: AnalysisResult{
				SiteSummary: "Second analysis",
			},
		},
	})

	handler := handleListAnalyses(db)
	req := testRequest(t, "GET", "/api/analyses", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []AnalysisSummary
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	// Should be sorted by createdAt desc (second analysis first)
	if summaries[0].Domain != "other.com" {
		t.Errorf("first summary domain: got %q, want %q", summaries[0].Domain, "other.com")
	}
}

func TestHandleListAnalyses_DomainFilter(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.Analyses().InsertMany(ctx, []any{
		Analysis{TenantID: "test-tenant", Domain: "example.com", CreatedAt: time.Now()},
		Analysis{TenantID: "test-tenant", Domain: "other.com", CreatedAt: time.Now()},
	})

	handler := handleListAnalyses(db)
	req := testRequest(t, "GET", "/api/analyses?domain=example.com", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []AnalysisSummary
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary with domain filter, got %d", len(summaries))
	}
	if summaries[0].Domain != "example.com" {
		t.Errorf("domain: got %q", summaries[0].Domain)
	}
}

func TestHandleListAnalyses_TenantIsolation(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	// Insert analyses for two different tenants
	db.Analyses().InsertMany(ctx, []any{
		Analysis{TenantID: "test-tenant", Domain: "mine.com", CreatedAt: time.Now()},
		Analysis{TenantID: "other-tenant", Domain: "theirs.com", CreatedAt: time.Now()},
	})

	handler := handleListAnalyses(db)
	req := testRequest(t, "GET", "/api/analyses", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []AnalysisSummary
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary (tenant isolation), got %d", len(summaries))
	}
	if summaries[0].Domain != "mine.com" {
		t.Errorf("domain: got %q, want %q", summaries[0].Domain, "mine.com")
	}
}

func TestHandleGetAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	res, _ := db.Analyses().InsertOne(ctx, Analysis{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		Model:     "test-model",
		CreatedAt: time.Now(),
		Result: AnalysisResult{
			SiteSummary: "Test summary",
			Questions:   []Question{{Question: "Q1"}},
		},
	})

	handler := handleGetAnalysis(db)
	oid := res.InsertedID.(primitive.ObjectID)
	req := testRequest(t, "GET", "/api/analyses/"+oid.Hex(), nil)
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var analysis Analysis
	json.NewDecoder(w.Body).Decode(&analysis)
	if analysis.Domain != "example.com" {
		t.Errorf("domain: got %q", analysis.Domain)
	}
	if analysis.Result.SiteSummary != "Test summary" {
		t.Errorf("summary: got %q", analysis.Result.SiteSummary)
	}
}

func TestHandleGetAnalysis_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetAnalysis(db)

	fakeID := primitive.NewObjectID().Hex()
	req := testRequest(t, "GET", "/api/analyses/"+fakeID, nil)
	req.SetPathValue("id", fakeID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleGetAnalysis_InvalidID(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetAnalysis(db)

	req := testRequest(t, "GET", "/api/analyses/not-a-valid-id", nil)
	req.SetPathValue("id", "not-a-valid-id")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleDeleteAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	res, _ := db.Analyses().InsertOne(ctx, Analysis{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		CreatedAt: time.Now(),
	})
	oid := res.InsertedID.(primitive.ObjectID)

	// Add related optimizations and todos
	optRes, _ := db.Optimizations().InsertOne(ctx, bson.M{
		"analysisId": oid,
		"tenantId":   "test-tenant",
		"domain":     "example.com",
	})
	optOID := optRes.InsertedID.(primitive.ObjectID)
	db.Todos().InsertOne(ctx, bson.M{
		"optimizationId": optOID,
		"tenantId":       "test-tenant",
	})

	handler := handleDeleteAnalysis(db)
	req := testRequest(t, "DELETE", "/api/analyses/"+oid.Hex(), nil)
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	// Verify cascade delete
	count, _ := db.Analyses().CountDocuments(ctx, bson.M{})
	if count != 0 {
		t.Errorf("analyses should be empty, got %d", count)
	}
	optCount, _ := db.Optimizations().CountDocuments(ctx, bson.M{})
	if optCount != 0 {
		t.Errorf("optimizations should be empty, got %d", optCount)
	}
	todoCount, _ := db.Todos().CountDocuments(ctx, bson.M{})
	if todoCount != 0 {
		t.Errorf("todos should be empty, got %d", todoCount)
	}
}

func TestHandleDeleteAnalysis_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleDeleteAnalysis(db)

	fakeID := primitive.NewObjectID().Hex()
	req := testRequest(t, "DELETE", "/api/analyses/"+fakeID, nil)
	req.SetPathValue("id", fakeID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

// ── Brand Profile CRUD ──────────────────────────────────────────────────

func TestHandleListBrands_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListBrands(db)

	req := testRequest(t, "GET", "/api/brands", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []BrandProfileSummary
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestHandleListBrands_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		BrandName: "Test Brand",
		UpdatedAt: time.Now(),
	})

	handler := handleListBrands(db)
	req := testRequest(t, "GET", "/api/brands", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []BrandProfileSummary
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].BrandName != "Test Brand" {
		t.Errorf("brand name: got %q", summaries[0].BrandName)
	}
}

func TestHandleGetBrand(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:    "test-tenant",
		Domain:      "example.com",
		BrandName:   "Test Brand",
		Description: "A test description",
		Categories:  []string{"tech", "ai"},
		UpdatedAt:   time.Now(),
	})

	handler := handleGetBrand(db)
	req := testRequest(t, "GET", "/api/brand/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var profile BrandProfile
	json.NewDecoder(w.Body).Decode(&profile)
	if profile.BrandName != "Test Brand" {
		t.Errorf("brand name: got %q", profile.BrandName)
	}
	if profile.Description != "A test description" {
		t.Errorf("description: got %q", profile.Description)
	}
}

func TestHandleGetBrand_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetBrand(db)

	req := testRequest(t, "GET", "/api/brand/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleSaveBrand(t *testing.T) {
	db := testMongoDB(t)
	handler := handleSaveBrand(db)

	brand := BrandProfile{
		BrandName:   "New Brand",
		Description: "A new brand profile",
		Categories:  []string{"tech"},
	}

	req := testRequest(t, "PUT", "/api/brand/example.com", brand)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusCreated)

	// Verify it was saved
	ctx := context.Background()
	var saved BrandProfile
	db.BrandProfiles().FindOne(ctx, bson.M{"domain": "example.com", "tenantId": "test-tenant"}).Decode(&saved)
	if saved.BrandName != "New Brand" {
		t.Errorf("saved brand name: got %q", saved.BrandName)
	}
}

func TestHandleSaveBrand_Update(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	// Insert existing brand
	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		BrandName: "Old Name",
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	})

	handler := handleSaveBrand(db)
	brand := BrandProfile{
		BrandName:   "Updated Name",
		Description: "Updated description",
	}

	req := testRequest(t, "PUT", "/api/brand/example.com", brand)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	// Verify update
	var saved BrandProfile
	db.BrandProfiles().FindOne(ctx, bson.M{"domain": "example.com", "tenantId": "test-tenant"}).Decode(&saved)
	if saved.BrandName != "Updated Name" {
		t.Errorf("updated brand name: got %q", saved.BrandName)
	}
}

func TestHandleDeleteBrand(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		BrandName: "To Delete",
	})

	handler := handleDeleteBrand(db)
	req := testRequest(t, "DELETE", "/api/brand/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	count, _ := db.BrandProfiles().CountDocuments(ctx, bson.M{"domain": "example.com"})
	if count != 0 {
		t.Errorf("brand should be deleted, got count %d", count)
	}
}

// ── Todo CRUD ───────────────────────────────────────────────────────────

func TestHandleListTodos_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListTodos(db)

	req := testRequest(t, "GET", "/api/todos", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []TodoItem
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(todos))
	}
}

func TestHandleListTodos_StatusFilter(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "status": "todo", "action": "Do something", "createdAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "status": "completed", "action": "Done thing", "createdAt": time.Now()},
	})

	handler := handleListTodos(db)
	req := testRequest(t, "GET", "/api/todos?status=todo", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []TodoItem
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo with status filter, got %d", len(todos))
	}
}

func TestHandleUpdateTodo(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	res, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"status":    "todo",
		"action":    "Test action",
		"domain":    "example.com",
		"createdAt": time.Now(),
	})
	oid := res.InsertedID.(primitive.ObjectID)

	handler := handleUpdateTodo(db)
	body := map[string]string{"status": "completed"}
	req := testRequest(t, "PATCH", "/api/todos/"+oid.Hex(), body)
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	// Verify status updated
	var todo bson.M
	db.Todos().FindOne(ctx, bson.M{"_id": oid}).Decode(&todo)
	if todo["status"] != "completed" {
		t.Errorf("todo status: got %q, want %q", todo["status"], "completed")
	}
}

func TestHandleUpdateTodo_InvalidID(t *testing.T) {
	db := testMongoDB(t)
	handler := handleUpdateTodo(db)

	body := map[string]string{"status": "completed"}
	req := testRequest(t, "PATCH", "/api/todos/invalid", body)
	req.SetPathValue("id", "invalid")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── API Key Management ──────────────────────────────────────────────────

func TestHandleListAPIKeys_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListAPIKeys(db)

	req := testRequest(t, "GET", "/api/keys", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string][]TenantAPIKey
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp["keys"]) != 0 {
		t.Errorf("expected 0 keys, got %d", len(resp["keys"]))
	}
}

func TestHandleListAPIKeys_WithKeys(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:  "test-tenant",
		Provider:  "anthropic",
		KeyPrefix: "sk-ant-a",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	handler := handleListAPIKeys(db)
	req := testRequest(t, "GET", "/api/keys", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string][]TenantAPIKey
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp["keys"]) != 1 {
		t.Fatalf("expected 1 key, got %d", len(resp["keys"]))
	}
	if resp["keys"][0].Provider != "anthropic" {
		t.Errorf("provider: got %q", resp["keys"][0].Provider)
	}
}

func TestHandleSetAPIKey(t *testing.T) {
	db := testMongoDB(t)
	handler := handleSetAPIKey(db, testEncKey())

	body := map[string]string{"key": "sk-ant-test-key-12345678"}
	req := testRequestOwner(t, "PUT", "/api/keys/anthropic", body)
	req.SetPathValue("provider", "anthropic")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	// Verify key was saved (encrypted)
	ctx := context.Background()
	var saved TenantAPIKey
	db.TenantAPIKeys().FindOne(ctx, bson.M{"tenantId": "test-tenant", "provider": "anthropic"}).Decode(&saved)
	if saved.EncryptedKey == "" {
		t.Error("encrypted key should not be empty")
	}
	if saved.KeyPrefix == "" {
		t.Error("key prefix should not be empty")
	}
}

func TestHandleSetAPIKey_InvalidProvider(t *testing.T) {
	db := testMongoDB(t)
	handler := handleSetAPIKey(db, testEncKey())

	body := map[string]string{"key": "some-key"}
	req := testRequestOwner(t, "PUT", "/api/keys/invalid_provider", body)
	req.SetPathValue("provider", "invalid_provider")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleDeleteAPIKey(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:  "test-tenant",
		Provider:  "openai",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	handler := handleDeleteAPIKey(db)
	req := testRequestOwner(t, "DELETE", "/api/keys/openai", nil)
	req.SetPathValue("provider", "openai")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	count, _ := db.TenantAPIKeys().CountDocuments(ctx, bson.M{"tenantId": "test-tenant", "provider": "openai"})
	if count != 0 {
		t.Errorf("key should be deleted, got count %d", count)
	}
}

// ── Health History ──────────────────────────────────────────────────────

func TestHandleHealthHistory(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.HealthChecks().InsertOne(ctx, HealthRecord{
		CheckedAt: time.Now(),
		Models: []ModelStatusRecord{
			{Model: "test-model", Name: "Test", Status: "available", LatencyMs: 500},
		},
	})

	handler := handleHealthHistory(db)
	req := testRequest(t, "GET", "/api/health/history", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var records []HealthRecord
	json.NewDecoder(w.Body).Decode(&records)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
}

// ── Provider Models ─────────────────────────────────────────────────────

func TestHandleListProviderModels(t *testing.T) {
	handler := handleListProviderModels(nil, nil, "", false)

	req := testRequestNoAuth(t, "GET", "/api/providers/models", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) == 0 {
		t.Error("response should contain at least one provider")
	}
	// Each provider should have id, name, models
	for _, p := range resp {
		if p["id"] == nil || p["name"] == nil || p["models"] == nil {
			t.Errorf("provider missing required fields: %v", p)
		}
	}
}

// ── Failed Analyses ─────────────────────────────────────────────────────

func TestHandleListFailedAnalyses_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListFailedAnalyses(db)

	req := testRequest(t, "GET", "/api/failed-analyses", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleDeleteFailedAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	res, _ := db.FailedAnalyses().InsertOne(ctx, bson.M{
		"tenantId": "test-tenant",
		"domain":   "example.com",
		"feedType": "analysis",
		"failedAt": time.Now(),
		"error":    "test error",
	})
	oid := res.InsertedID.(primitive.ObjectID)

	handler := handleDeleteFailedAnalysis(db)
	req := testRequest(t, "DELETE", "/api/failed-analyses/"+oid.Hex(), nil)
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── Optimization CRUD ───────────────────────────────────────────────────

func TestHandleListOptimizations_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListOptimizations(db)

	req := testRequest(t, "GET", "/api/optimizations", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleListOptimizations_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.Optimizations().InsertMany(ctx, []any{
		bson.M{
			"tenantId":  "test-tenant",
			"domain":    "example.com",
			"model":     "test-model",
			"question":  "How does this work?",
			"createdAt": time.Now(),
		},
		bson.M{
			"tenantId":  "test-tenant",
			"domain":    "other.com",
			"model":     "test-model",
			"question":  "What is this?",
			"createdAt": time.Now(),
		},
	})

	handler := handleListOptimizations(db)
	req := testRequest(t, "GET", "/api/optimizations", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var results []bson.M
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 2 {
		t.Errorf("expected 2 optimizations, got %d", len(results))
	}
}

func TestHandleGetOptimizationByID(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	res, _ := db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "example.com",
		"model":     "test-model",
		"question":  "How does this work?",
		"createdAt": time.Now(),
		"result":    bson.M{"score": 75},
	})
	oid := res.InsertedID.(primitive.ObjectID)

	handler := handleGetOptimizationByID(db)
	req := testRequest(t, "GET", "/api/optimizations/"+oid.Hex(), nil)
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleGetOptimizationByID_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetOptimizationByID(db)

	fakeID := primitive.NewObjectID().Hex()
	req := testRequest(t, "GET", "/api/optimizations/"+fakeID, nil)
	req.SetPathValue("id", fakeID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleGetOptimizationByID_InvalidID(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetOptimizationByID(db)

	req := testRequest(t, "GET", "/api/optimizations/invalid", nil)
	req.SetPathValue("id", "invalid")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── API Key Status ──────────────────────────────────────────────────────

func TestHandleAPIKeyStatus_NoKeys(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIKeyStatus(db)

	req := testRequest(t, "GET", "/api/keys/status", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleAPIKeyStatus_WithKeys(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()

	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:  "test-tenant",
		Provider:  "anthropic",
		KeyPrefix: "sk-ant",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	handler := handleAPIKeyStatus(db)
	req := testRequest(t, "GET", "/api/keys/status", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── Delete Optimization ────────────────────────────────────────────────

func TestHandleDeleteOptimization(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	res, _ := db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "example.com",
		"question":  "How to improve?",
		"createdAt": time.Now(),
	})
	oid := res.InsertedID.(primitive.ObjectID)

	// Also insert a todo linked to this optimization
	db.Todos().InsertOne(ctx, bson.M{
		"tenantId":       "test-tenant",
		"domain":         "example.com",
		"optimizationId": oid,
		"status":         "todo",
		"action":         "Do this",
	})

	handler := handleDeleteOptimization(db)
	req := testRequest(t, "DELETE", "/api/optimizations/"+oid.Hex(), nil)
	req.SetPathValue("id", oid.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	// Verify todo was cascade-deleted
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"optimizationId": oid})
	if count != 0 {
		t.Errorf("expected 0 todos after cascade delete, got %d", count)
	}
}

func TestHandleDeleteOptimization_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleDeleteOptimization(db)

	fakeID := primitive.NewObjectID().Hex()
	req := testRequest(t, "DELETE", "/api/optimizations/"+fakeID, nil)
	req.SetPathValue("id", fakeID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

// ── Video Analysis CRUD ────────────────────────────────────────────────

func TestHandleGetVideoAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
		"result":      bson.M{"overallScore": 75},
	})

	handler := handleGetVideoAnalysis(db)
	req := testRequest(t, "GET", "/api/video/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleGetVideoAnalysis_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetVideoAnalysis(db)

	req := testRequest(t, "GET", "/api/video/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleListVideoAnalyses_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListVideoAnalyses(db)

	req := testRequest(t, "GET", "/api/video", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleDeleteVideoAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
	})

	handler := handleDeleteVideoAnalysis(db)
	req := testRequest(t, "DELETE", "/api/video/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── Reddit Analysis CRUD ───────────────────────────────────────────────

func TestHandleGetRedditAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.RedditAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
		"result":      bson.M{"overallScore": 60},
	})

	handler := handleGetRedditAnalysis(db)
	req := testRequest(t, "GET", "/api/reddit/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleGetRedditAnalysis_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetRedditAnalysis(db)

	req := testRequest(t, "GET", "/api/reddit/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleListRedditAnalyses_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListRedditAnalyses(db)

	req := testRequest(t, "GET", "/api/reddit", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleDeleteRedditAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.RedditAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
	})

	handler := handleDeleteRedditAnalysis(db)
	req := testRequest(t, "DELETE", "/api/reddit/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── Search Analysis CRUD ───────────────────────────────────────────────

func TestHandleGetSearchAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.SearchAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
		"result":      bson.M{"overallScore": 80},
	})

	handler := handleGetSearchAnalysis(db)
	req := testRequest(t, "GET", "/api/search/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleGetSearchAnalysis_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetSearchAnalysis(db)

	req := testRequest(t, "GET", "/api/search/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleListSearchAnalyses_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleListSearchAnalyses(db)

	req := testRequest(t, "GET", "/api/search", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleDeleteSearchAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.SearchAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
	})

	handler := handleDeleteSearchAnalysis(db)
	req := testRequest(t, "DELETE", "/api/search/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── LLM Test CRUD ──────────────────────────────────────────────────────

func TestHandleGetLLMTest(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.LLMTests().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
	})

	handler := handleGetLLMTest(db)
	req := testRequest(t, "GET", "/api/llm-tests/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleGetLLMTest_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetLLMTest(db)

	req := testRequest(t, "GET", "/api/llm-tests/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleGetLLMTestHistory(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.LLMTests().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "generatedAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "generatedAt": time.Now().Add(-time.Hour)},
	})

	handler := handleGetLLMTestHistory(db)
	req := testRequest(t, "GET", "/api/llm-tests/example.com/history", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleDeleteLLMTest(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.LLMTests().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
	})

	handler := handleDeleteLLMTest(db)
	req := testRequest(t, "DELETE", "/api/llm-tests/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── Visibility Score ───────────────────────────────────────────────────

func TestHandleVisibilityScore_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleVisibilityScore(db)

	req := testRequest(t, "GET", "/api/visibility-score/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["score"].(float64) != 0 {
		t.Errorf("expected score 0, got %v", resp["score"])
	}
}

func TestHandleVisibilityScore_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "example.com",
		"createdAt": time.Now(),
		"result":    bson.M{"overallScore": 70},
	})
	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
		"result":      bson.M{"overallScore": 80},
	})

	handler := handleVisibilityScore(db)
	req := testRequest(t, "GET", "/api/visibility-score/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["score"].(float64) == 0 {
		t.Error("expected non-zero score")
	}
}

// ── Domain Summary Status ──────────────────────────────────────────────

func TestHandleDomainSummaryStatus_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleDomainSummaryStatus(db)

	req := testRequest(t, "GET", "/api/summary/status/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["exists"].(bool) != false {
		t.Error("expected exists=false")
	}
}

func TestHandleGetDomainSummary(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.DomainSummaries().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": time.Now(),
		"result":      bson.M{"executive_summary": "A great domain"},
	})

	handler := handleGetDomainSummary(db)
	req := testRequest(t, "GET", "/api/summary/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleGetDomainSummary_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetDomainSummary(db)

	req := testRequest(t, "GET", "/api/summary/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

// ── Patch Brand Subreddits ─────────────────────────────────────────────

func TestHandlePatchBrandSubreddits(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		BrandName: "Test",
	})

	handler := handlePatchBrandSubreddits(db)
	body := map[string]any{"subreddits": []string{"golang", "programming"}}
	req := testRequest(t, "PATCH", "/api/brands/example.com/subreddits", body)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── Get Optimization (by analysis ID + question index) ─────────────────

func TestHandleGetOptimization(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	analysisID := primitive.NewObjectID()
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":      "test-tenant",
		"domain":        "example.com",
		"analysisId":    analysisID,
		"questionIndex": 0,
		"question":      "How to improve SEO?",
		"createdAt":     time.Now(),
	})

	handler := handleGetOptimization(db)
	req := testRequest(t, "GET", "/api/optimizations/"+analysisID.Hex()+"/0", nil)
	req.SetPathValue("id", analysisID.Hex())
	req.SetPathValue("idx", "0")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleGetOptimization_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetOptimization(db)

	fakeID := primitive.NewObjectID().Hex()
	req := testRequest(t, "GET", "/api/optimizations/"+fakeID+"/0", nil)
	req.SetPathValue("id", fakeID)
	req.SetPathValue("idx", "0")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}
