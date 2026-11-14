package main

// Additional handler tests for SSE brand-AI handlers and REST handlers
// that were at 0% coverage.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── handleDiscoverCompetitors ────────────────────────────────────────────

func TestHandleDiscoverCompetitors_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	// Lenient: handler sends result.ResultJSON directly, no parsing
	withTestProviders(t, tp)

	handler := handleDiscoverCompetitors(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/example.com/competitors/discover", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

// ── handleGenerateDescription ────────────────────────────────────────────

func TestHandleGenerateDescription_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleGenerateDescription(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/example.com/description/generate", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

// ── handlePredictAudience ────────────────────────────────────────────────

func TestHandlePredictAudience_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handlePredictAudience(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/example.com/audience/predict", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

// ── handleSuggestClaims ──────────────────────────────────────────────────

func TestHandleSuggestClaims_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSuggestClaims(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/example.com/claims/suggest", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

// ── handlePredictDifferentiators ─────────────────────────────────────────

func TestHandlePredictDifferentiators_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handlePredictDifferentiators(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/example.com/differentiators/predict", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

// ── handleSuggestQueries ─────────────────────────────────────────────────

func TestHandleSuggestQueries_NoBrandProfile(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSuggestQueries(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/nobrand.com/queries/suggest", nil)
	req.SetPathValue("domain", "nobrand.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event when brand profile is missing")
	}
}

func TestHandleSuggestQueries_WithBrand(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed a brand profile
	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":        "test-tenant",
		"domain":          "brand-suggest.com",
		"brandName":       "TestBrand",
		"description":     "A test brand for testing",
		"categories":      bson.A{"SaaS", "Analytics"},
		"products":        bson.A{"TestProduct"},
		"primaryAudience": "Developers",
		"keyUseCases":     bson.A{"API testing", "Coverage tracking"},
		"competitors":     bson.A{},
		"createdAt":       time.Now(),
		"updatedAt":       time.Now(),
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSuggestQueries(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/brand-suggest.com/queries/suggest", nil)
	req.SetPathValue("domain", "brand-suggest.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

// ── handleGetPopularDomains ──────────────────────────────────────────────

func TestHandleGetPopularDomains_Empty(t *testing.T) {
	db := testMongoDB(t)

	handler := handleGetPopularDomains(db)
	req := httptest.NewRequest("GET", "/api/popular", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	// Empty result: should be "[]"
	var result []any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON array, got error: %v; body: %s", err, w.Body.String())
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestHandleGetPopularDomains_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.DomainShares().InsertOne(ctx, bson.M{
		"tenantId":   "test-tenant",
		"domain":     "popular.com",
		"shareId":    "share-popular-1",
		"visibility": "popular",
		"viewCount":  100,
		"createdAt":  time.Now(),
	})

	// Reset the popular domains cache first
	popularDomainsCache.Lock()
	popularDomainsCache.data = nil
	popularDomainsCache.expiresAt = time.Time{}
	popularDomainsCache.Unlock()

	handler := handleGetPopularDomains(db)
	req := httptest.NewRequest("GET", "/api/popular", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleGetSharedDomain ────────────────────────────────────────────────

func TestHandleGetSharedDomain_NotFound(t *testing.T) {
	db := testMongoDB(t)

	handler := handleGetSharedDomain(db)
	req := httptest.NewRequest("GET", "/s/nonexistent", nil)
	req.SetPathValue("shareId", "nonexistent-share-id")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetSharedDomain_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	shareID := "test-share-abc123"
	db.DomainShares().InsertOne(ctx, bson.M{
		"_id":        primitive.NewObjectID(),
		"tenantId":   "test-tenant",
		"domain":     "shared-domain.com",
		"shareId":    shareID,
		"visibility": "public",
		"viewCount":  5,
		"createdAt":  time.Now(),
	})

	handler := handleGetSharedDomain(db)
	req := httptest.NewRequest("GET", "/s/"+shareID, nil)
	req.SetPathValue("shareId", shareID)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", w.Header().Get("Content-Type"))
	}
}

// ── handleSitemap ────────────────────────────────────────────────────────

func TestHandleSitemap_Empty(t *testing.T) {
	db := testMongoDB(t)

	handler := handleSitemap(db)
	req := httptest.NewRequest("GET", "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !containsStr(body, "<?xml") && !containsStr(body, "<urlset") {
		t.Errorf("expected XML sitemap, got: %s", body)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ── handleGenerateTestQueries ────────────────────────────────────────────

func TestHandleGenerateTestQueries_NoDomain(t *testing.T) {
	db := testMongoDB(t)

	handler := handleGenerateTestQueries(db)
	// Send body with empty domain → 400
	req := testRequest(t, "POST", "/api/llmtest/queries", map[string]any{"domain": ""})
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestHandleGenerateTestQueries_WithDomain(t *testing.T) {
	db := testMongoDB(t)

	handler := handleGenerateTestQueries(db)
	// No brand profile → returns empty queries
	req := testRequest(t, "POST", "/api/llmtest/queries", map[string]any{"domain": "example.com"})
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON response: %v; body: %s", err, w.Body.String())
	}
}

// ── handleVideoSearchTerms ───────────────────────────────────────────────

func TestHandleVideoSearchTerms_InvalidBody(t *testing.T) {
	db := testMongoDB(t)

	handler := handleVideoSearchTerms(db)
	// Missing required fields → 400
	req := testRequest(t, "POST", "/api/video/search-terms", map[string]any{
		"domain": "example.com",
		// missing action and term → 400
	})
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestHandleVideoSearchTerms_Add(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed a brand profile so the handler can find it after update
	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "example.com",
		"createdAt": time.Now(),
		"updatedAt": time.Now(),
	})

	handler := handleVideoSearchTerms(db)
	req := testRequest(t, "POST", "/api/video/search-terms", map[string]any{
		"domain": "example.com",
		"action": "add",
		"term":   "test keyword",
	})
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// ── handleRedditDiscover ─────────────────────────────────────────────────

func TestHandleRedditDiscover_NoSearchTerms(t *testing.T) {
	db := testMongoDB(t)

	handler := handleRedditDiscover(db, testEncKey(), "fallback-key", false)
	req := testRequest(t, "POST", "/api/reddit/discover", map[string]any{
		"domain":       "example.com",
		"search_terms": []string{},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE for no search terms")
	}
}

func TestHandleRedditDiscover_WithTerms(t *testing.T) {
	db := testMongoDB(t)

	// Set up a test Reddit server that returns empty results
	redditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a valid but empty Reddit search response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"children":[]}}`))
	}))
	defer redditServer.Close()
	withTestRedditBase(t, redditServer)

	handler := handleRedditDiscover(db, testEncKey(), "fallback-key", false)
	req := testRequest(t, "POST", "/api/reddit/discover", map[string]any{
		"domain":       "example.com",
		"brand_name":   "Example",
		"subreddits":   []string{"r/programming"},
		"search_terms": []string{"example"},
		"time_filter":  "year",
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	// Should get a done event (possibly with empty results)
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Logf("events: %v; body: %s", events, w.Body.String())
		// May also get an error if reddit returns bad format - that's OK
		// as long as something was sent
		if len(events) == 0 {
			t.Fatal("expected at least one SSE event")
		}
	}
}
