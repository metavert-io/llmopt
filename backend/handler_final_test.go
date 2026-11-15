package main

// Tests targeting specific uncovered branches in low-coverage handlers:
//   - buildDomainSummaryPrompt (pure function, all data branches)
//   - isSummaryStale (newer analysis, included/not-included video/reddit)
//   - Brand AI handlers: no-API-key path (saasEnabled=true + no key in DB)
//   - Brand AI handlers: brand-with-data path (brand profile present in DB)
//   - handleRedditDiscover: invalid body
//   - handleRedditAnalyze: no-API-key, invalid body
//   - handleDeleteBrand: not-found path
//   - handlePatchBrandSubreddits: invalid body

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── buildDomainSummaryPrompt ──────────────────────────────────────────────

func TestBuildDomainSummaryPrompt_NilData(t *testing.T) {
	prompt := buildDomainSummaryPrompt("nil-test.com", nil, nil, nil, nil, summaryExtras{}, BrandContextInfo{})
	if !strings.Contains(prompt, "nil-test.com") {
		t.Error("prompt should contain domain name")
	}
}

func TestBuildDomainSummaryPrompt_WithAnalysisFull(t *testing.T) {
	analysis := &Analysis{
		Result: AnalysisResult{
			SiteSummary:  "This is a test site.",
			CrawledPages: []CrawledPage{{URL: "https://example.com"}, {URL: "https://example.com/about"}},
			Questions:    []Question{{Question: "What does example do?"}},
		},
	}
	prompt := buildDomainSummaryPrompt("example.com", nil, analysis, nil, nil, summaryExtras{}, BrandContextInfo{})
	if !strings.Contains(prompt, "SITE ANALYSIS") {
		t.Error("prompt should contain SITE ANALYSIS section")
	}
	if !strings.Contains(prompt, "This is a test site.") {
		t.Error("prompt should contain the site summary")
	}
}

func TestBuildDomainSummaryPrompt_WithOptimizations(t *testing.T) {
	optimizations := []Optimization{
		{
			Question: "Brand awareness query",
			Result: OptimizationResult{
				OverallScore:           80,
				ContentAuthority:       DimensionScore{Score: 75},
				StructuralOptimization: DimensionScore{Score: 70},
				SourceAuthority:        DimensionScore{Score: 65},
				KnowledgePersistence:   DimensionScore{Score: 60},
			},
		},
		{
			Question: "Another brand query",
			Result: OptimizationResult{
				OverallScore:           70,
				ContentAuthority:       DimensionScore{Score: 65},
				StructuralOptimization: DimensionScore{Score: 60},
				SourceAuthority:        DimensionScore{Score: 55},
				KnowledgePersistence:   DimensionScore{Score: 50},
			},
		},
	}
	prompt := buildDomainSummaryPrompt("example.com", optimizations, nil, nil, nil, summaryExtras{}, BrandContextInfo{})
	if !strings.Contains(prompt, "OPTIMIZATION REPORTS") {
		t.Error("prompt should contain OPTIMIZATION REPORTS section")
	}
	if !strings.Contains(prompt, "Brand awareness query") {
		t.Error("prompt should contain optimization question")
	}
}

func TestBuildDomainSummaryPrompt_WithVideo(t *testing.T) {
	videoAnalysis := &VideoAnalysis{
		Result: &VideoAuthorityResult{
			OverallScore:     78,
			ExecutiveSummary: "Good YouTube presence.",
		},
	}
	prompt := buildDomainSummaryPrompt("example.com", nil, nil, videoAnalysis, nil, summaryExtras{}, BrandContextInfo{})
	if !strings.Contains(prompt, "YOUTUBE VIDEO AUTHORITY") {
		t.Error("prompt should contain VIDEO AUTHORITY section")
	}
}

func TestBuildDomainSummaryPrompt_VideoNilResult(t *testing.T) {
	// VideoAnalysis with nil Result should NOT include the video section
	videoAnalysis := &VideoAnalysis{Result: nil}
	prompt := buildDomainSummaryPrompt("example.com", nil, nil, videoAnalysis, nil, summaryExtras{}, BrandContextInfo{})
	if strings.Contains(prompt, "YOUTUBE VIDEO AUTHORITY") {
		t.Error("prompt should NOT contain VIDEO section when Result is nil")
	}
}

func TestBuildDomainSummaryPrompt_WithReddit(t *testing.T) {
	redditAnalysis := &RedditAnalysis{
		Result: &RedditAuthorityResult{
			OverallScore:     65,
			ExecutiveSummary: "Moderate Reddit presence.",
		},
	}
	prompt := buildDomainSummaryPrompt("example.com", nil, nil, nil, redditAnalysis, summaryExtras{}, BrandContextInfo{})
	if !strings.Contains(prompt, "REDDIT AUTHORITY") {
		t.Error("prompt should contain REDDIT AUTHORITY section")
	}
}

func TestBuildDomainSummaryPrompt_WithBrandContext(t *testing.T) {
	brandInfo := BrandContextInfo{
		Used:          true,
		ContextString: "Brand: TestCo\nDescription: A SaaS company.\n",
	}
	prompt := buildDomainSummaryPrompt("example.com", nil, nil, nil, nil, summaryExtras{}, brandInfo)
	if !strings.Contains(prompt, "Brand Intelligence Context") {
		t.Error("prompt should contain Brand Intelligence Context section")
	}
	if !strings.Contains(prompt, "TestCo") {
		t.Error("prompt should contain brand name")
	}
}

func TestBuildDomainSummaryPrompt_AllData(t *testing.T) {
	analysis := &Analysis{
		Result: AnalysisResult{
			SiteSummary:  "Complete site.",
			CrawledPages: []CrawledPage{{URL: "https://x.com"}},
		},
	}
	optimizations := []Optimization{
		{
			Question: "Q1",
			Result: OptimizationResult{OverallScore: 85, ContentAuthority: DimensionScore{Score: 80}},
		},
	}
	videoAnalysis := &VideoAnalysis{
		Result: &VideoAuthorityResult{OverallScore: 70, ExecutiveSummary: "Good."},
	}
	redditAnalysis := &RedditAnalysis{
		Result: &RedditAuthorityResult{OverallScore: 60, ExecutiveSummary: "Moderate."},
	}
	brandInfo := BrandContextInfo{Used: true, ContextString: "Brand: AllTest\n"}

	prompt := buildDomainSummaryPrompt("x.com", optimizations, analysis, videoAnalysis, redditAnalysis, summaryExtras{}, brandInfo)
	for _, want := range []string{"x.com", "SITE ANALYSIS", "OPTIMIZATION REPORTS", "YOUTUBE VIDEO AUTHORITY", "REDDIT AUTHORITY", "Brand Intelligence Context"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected prompt to contain %q", want)
		}
	}
}

// ── isSummaryStale ────────────────────────────────────────────────────────

func TestIsSummaryStale_NotStaleEmptyDB(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		Domain:        "fresh.com",
		GeneratedAt:   time.Now().Add(-time.Minute), // just generated
	}

	stale, count := isSummaryStale(ctx, db, tenantCtx, "fresh.com", summary)
	if stale {
		t.Errorf("expected not stale for empty DB, count=%d", count)
	}
}

func TestIsSummaryStale_NewerAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert a site analysis newer than the summary
	db.Analyses().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "stale-analysis.com",
		"createdAt": time.Now(), // newer than summary
	})

	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		Domain:        "stale-analysis.com",
		GeneratedAt:   time.Now().Add(-2 * time.Hour),
	}

	stale, count := isSummaryStale(ctx, db, tenantCtx, "stale-analysis.com", summary)
	if !stale {
		t.Error("expected stale due to newer site analysis")
	}
	if count == 0 {
		t.Error("expected non-zero count for newer analysis")
	}
}

func TestIsSummaryStale_VideoNotIncluded_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert a video analysis
	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "no-video-summary.com",
		"generatedAt": time.Now().Add(-time.Hour),
	})

	// Summary does NOT include video — any video analysis should trigger staleness
	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		Domain:        "no-video-summary.com",
		GeneratedAt:   time.Now().Add(-2 * time.Hour),
		IncludesVideo: false,
	}

	stale, _ := isSummaryStale(ctx, db, tenantCtx, "no-video-summary.com", summary)
	if !stale {
		t.Error("expected stale because summary doesn't include video but video exists")
	}
}

func TestIsSummaryStale_VideoIncluded_NewerVideo(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert a video analysis newer than the summary
	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "stale-video.com",
		"generatedAt": time.Now(), // newer than summary
	})

	// Summary DOES include video — only stale if newer video appeared
	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		Domain:        "stale-video.com",
		GeneratedAt:   time.Now().Add(-2 * time.Hour),
		IncludesVideo: true,
	}

	stale, _ := isSummaryStale(ctx, db, tenantCtx, "stale-video.com", summary)
	if !stale {
		t.Error("expected stale due to newer video analysis")
	}
}

func TestIsSummaryStale_RedditNotIncluded_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert a reddit analysis
	db.RedditAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "no-reddit-summary.com",
		"generatedAt": time.Now().Add(-time.Hour),
	})

	// Summary does NOT include reddit
	summary := DomainSummary{
		RubricVersion:  scoringRubricVersion,
		Domain:         "no-reddit-summary.com",
		GeneratedAt:    time.Now().Add(-2 * time.Hour),
		IncludesReddit: false,
	}

	stale, _ := isSummaryStale(ctx, db, tenantCtx, "no-reddit-summary.com", summary)
	if !stale {
		t.Error("expected stale because summary doesn't include reddit but reddit analysis exists")
	}
}

func TestIsSummaryStale_RedditIncluded_NewerReddit(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert a reddit analysis newer than summary
	db.RedditAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "stale-reddit.com",
		"generatedAt": time.Now(),
	})

	summary := DomainSummary{
		RubricVersion:  scoringRubricVersion,
		Domain:         "stale-reddit.com",
		GeneratedAt:    time.Now().Add(-2 * time.Hour),
		IncludesReddit: true,
	}

	stale, _ := isSummaryStale(ctx, db, tenantCtx, "stale-reddit.com", summary)
	if !stale {
		t.Error("expected stale due to newer reddit analysis")
	}
}

// ── Brand AI handlers: no-API-key path (saasEnabled=true) ────────────────
// In SaaS mode, resolvePrimaryLLM looks for an API key in MongoDB.
// testMongoDB starts clean (no key stored) → "api_key_required" error.

func TestHandleDiscoverCompetitors_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleDiscoverCompetitors(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "GET", "/api/brand/example.com/competitors/discover", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

func TestHandleGenerateDescription_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleGenerateDescription(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "GET", "/api/brand/example.com/description/generate", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

func TestHandlePredictAudience_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handlePredictAudience(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "GET", "/api/brand/example.com/audience/predict", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

func TestHandleSuggestClaims_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSuggestClaims(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "GET", "/api/brand/example.com/claims/suggest", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

func TestHandlePredictDifferentiators_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handlePredictDifferentiators(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "GET", "/api/brand/example.com/differentiators/predict", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

// ── Brand AI handlers: brand-with-data paths ─────────────────────────────

func TestHandlePredictAudience_WithBrand(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "audiencebrand.com",
		"brandName":   "AudienceBrand",
		"description": "A platform for audience intelligence.",
		"categories":  bson.A{"Analytics", "Marketing"},
		"products":    bson.A{"Dashboard", "Reports"},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handlePredictAudience(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/audiencebrand.com/audience/predict", nil)
	req.SetPathValue("domain", "audiencebrand.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event with brand context; body: %s", w.Body.String())
	}
}

func TestHandleSuggestClaims_WithBrand(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "claimsbrand.com",
		"brandName":   "ClaimsCo",
		"description": "A leading claims management system.",
		"categories":  bson.A{"InsurTech", "SaaS"},
		"products":    bson.A{"ClaimsAPI", "Dashboard"},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSuggestClaims(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/claimsbrand.com/claims/suggest", nil)
	req.SetPathValue("domain", "claimsbrand.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event with brand context; body: %s", w.Body.String())
	}
}

func TestHandlePredictDifferentiators_WithBrandAndCompetitors(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "diffbrand.com",
		"brandName":   "DiffBrand",
		"description": "The best brand in its space.",
		"categories":  bson.A{"SaaS", "AI"},
		"products":    bson.A{"DiffTool"},
		"competitors": bson.A{
			bson.M{"name": "CompetitorAlpha", "url": "competitor-alpha.com"},
			bson.M{"name": "CompetitorBeta", "url": "competitor-beta.com"},
		},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handlePredictDifferentiators(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/diffbrand.com/differentiators/predict", nil)
	req.SetPathValue("domain", "diffbrand.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event with brand+competitors context; body: %s", w.Body.String())
	}
}

func TestHandleDiscoverCompetitors_WithBrand(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "discoverybrand.com",
		"brandName":   "DiscoveryBrand",
		"description": "Finding competitors since 2020.",
		"categories":  bson.A{"Analytics"},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleDiscoverCompetitors(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/brand/discoverybrand.com/competitors/discover", nil)
	req.SetPathValue("domain", "discoverybrand.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event with brand context; body: %s", w.Body.String())
	}
}

// ── handleRedditDiscover: missing branches ────────────────────────────────

func TestHandleRedditDiscover_InvalidBody(t *testing.T) {
	db := testMongoDB(t)

	handler := handleRedditDiscover(db, testEncKey(), "fallback-key", false)
	// Send non-JSON body
	rawReq := httptest.NewRequest("POST", "/api/reddit/discover", bytes.NewReader([]byte("not-json")))
	rawReq = rawReq.WithContext(testAuthContext("test-tenant", "test-user"))
	rawReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, rawReq)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error for invalid body; body: %s", w.Body.String())
	}
}

func TestHandleRedditDiscover_BrandNameAsSearchTerm(t *testing.T) {
	db := testMongoDB(t)

	// Mock Reddit server returning empty results
	redditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"children":[]}}`))
	}))
	defer redditServer.Close()
	withTestRedditBase(t, redditServer)

	handler := handleRedditDiscover(db, testEncKey(), "fallback-key", false)
	// Include brand_name but no explicit subreddits → "all" gets appended
	req := testRequest(t, "POST", "/api/reddit/discover", map[string]any{
		"domain":       "brand.com",
		"brand_name":   "BrandCo",
		"search_terms": []string{"brand analytics"},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	// No threads found → SSE error ("No Reddit threads found...")
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Logf("events: %v; body: %s", events, w.Body.String())
		if len(events) == 0 {
			t.Fatal("expected at least one SSE event")
		}
	}
}

// ── handleRedditAnalyze: missing branches ────────────────────────────────

func TestHandleRedditAnalyze_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleRedditAnalyze(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "POST", "/api/reddit/analyze", map[string]any{
		"domain": "example.com",
		"threads": []map[string]any{
			{"id": "t1", "subreddit": "r/test", "title": "Thread"},
		},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

func TestHandleRedditAnalyze_InvalidBody(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleRedditAnalyze(db, testEncKey(), "fake-key", false)
	rawReq := httptest.NewRequest("POST", "/api/reddit/analyze", bytes.NewReader([]byte("not-json")))
	rawReq = rawReq.WithContext(testAuthContext("test-tenant", "test-user"))
	rawReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, rawReq)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for invalid body; body: %s", w.Body.String())
	}
}

// ── handleDeleteBrand: not-found path ────────────────────────────────────

func TestHandleDeleteBrand_NotFound(t *testing.T) {
	db := testMongoDB(t)

	handler := handleDeleteBrand(db)
	req := testRequest(t, "DELETE", "/api/brand/nonexistent.com", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

// ── handlePatchBrandSubreddits: invalid body ──────────────────────────────

func TestHandlePatchBrandSubreddits_InvalidBody(t *testing.T) {
	db := testMongoDB(t)

	handler := handlePatchBrandSubreddits(db)
	rawReq := httptest.NewRequest("PATCH", "/api/brand/example.com/subreddits", bytes.NewReader([]byte("not-json")))
	rawReq = rawReq.WithContext(testAuthContext("test-tenant", "test-user"))
	rawReq.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, rawReq)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── handleSearchAnalyze: no-API-key path ─────────────────────────────────

func TestHandleSearchAnalyze_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSearchAnalyze(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "POST", "/api/search/analyze", map[string]any{
		"domain": "example.com",
		"force":  true,
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

// ── isSummaryStale: newerOpts path ───────────────────────────────────────

func TestIsSummaryStale_NewerOptimizations(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert an optimization newer than the summary
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "stale-opt.com",
		"createdAt": time.Now(),
	})

	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		Domain:        "stale-opt.com",
		GeneratedAt:   time.Now().Add(-2 * time.Hour),
	}

	stale, count := isSummaryStale(ctx, db, tenantCtx, "stale-opt.com", summary)
	if !stale {
		t.Error("expected stale due to newer optimization")
	}
	if count == 0 {
		t.Error("expected non-zero count for newer optimization")
	}
}

// ── handleSuggestQueries: no-API-key path ────────────────────────────────

func TestHandleSuggestQueries_NoAPIKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	// First insert a brand profile so the handler doesn't early-exit on missing profile
	ctx := testAuthContext("test-tenant", "test-user")
	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "apikey-test.com",
		"brandName": "APIKeyTest",
	})

	handler := handleSuggestQueries(db, testEncKey(), "", true) // saasEnabled=true
	req := testRequest(t, "GET", "/api/brand/apikey-test.com/queries/suggest", nil)
	req.SetPathValue("domain", "apikey-test.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected SSE error event for missing API key; body: %s", w.Body.String())
	}
}

// ── handleRedditAnalyze: selected thread filtering ────────────────────────

func TestHandleRedditAnalyze_WithSelectedThreads(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		sendSSE(w, flusher, "text", map[string]string{"content": "analyzing..."})
		return &StreamResult{RawText: validRedditAuthorityResultJSON, ResultJSON: validRedditAuthorityResultJSON}, nil
	}
	withTestProviders(t, tp)

	// Set up a test Reddit server returning 404 (graceful fallback)
	redditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer redditServer.Close()
	withTestRedditBase(t, redditServer)

	handler := handleRedditAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/reddit/analyze", map[string]any{
		"domain": "example.com",
		"threads": []map[string]any{
			{"id": "t1", "subreddit": "r/test", "title": "Thread 1", "permalink": "/r/test/t1"},
			{"id": "t2", "subreddit": "r/test", "title": "Thread 2", "permalink": "/r/test/t2"},
		},
		// Only select t1 — exercises the selectedSet filtering
		"selected_thread_ids": []string{"t1"},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event with selected threads; events: %v; body: %s", events, w.Body.String())
	}
}

// ── Optimizations count paths for isSummaryStale ─────────────────────────

func TestIsSummaryStale_VideoIncluded_NotStale(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert an old video analysis (not newer than summary)
	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "old-video.com",
		"generatedAt": time.Now().Add(-3 * time.Hour), // older than summary
	})

	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		Domain:        "old-video.com",
		GeneratedAt:   time.Now().Add(-2 * time.Hour), // summary is newer than video
		IncludesVideo: true,
	}

	stale, _ := isSummaryStale(ctx, db, tenantCtx, "old-video.com", summary)
	if stale {
		t.Error("expected not stale: summary newer than video analysis")
	}
}

func TestIsSummaryStale_RedditIncluded_NotStale(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")

	// Insert an old reddit analysis
	db.RedditAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "old-reddit.com",
		"generatedAt": time.Now().Add(-3 * time.Hour),
	})

	summary := DomainSummary{
		RubricVersion:  scoringRubricVersion,
		Domain:         "old-reddit.com",
		GeneratedAt:    time.Now().Add(-2 * time.Hour),
		IncludesReddit: true,
	}

	stale, _ := isSummaryStale(ctx, db, tenantCtx, "old-reddit.com", summary)
	if stale {
		t.Error("expected not stale: summary newer than reddit analysis")
	}
}

// ── handleDeleteBrand: success path (verify not-found completes the coverage) ──

func TestHandleDeleteBrand_AlreadyDeleted(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert then delete brand first
	db.BrandProfiles().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "todelete2.com",
		"brandName": "ToDelete2",
	})

	handler := handleDeleteBrand(db)
	// First delete (success)
	req := testRequest(t, "DELETE", "/api/brand/todelete2.com", nil)
	req.SetPathValue("domain", "todelete2.com")
	w := httptest.NewRecorder()
	handler(w, req)
	assertStatus(t, w, http.StatusOK)

	// Second delete (not found)
	req2 := testRequest(t, "DELETE", "/api/brand/todelete2.com", nil)
	req2.SetPathValue("domain", "todelete2.com")
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	assertStatus(t, w2, http.StatusNotFound)
}

// ── sortThreadsByScore ─────────────────────────────────────────────────────

func TestSortThreadsByScore_FourItems(t *testing.T) {
	threads := []RedditThread{
		{ID: "a", Score: 50},
		{ID: "b", Score: 200},
		{ID: "c", Score: 10},
		{ID: "d", Score: 100},
	}
	sortThreadsByScore(threads)
	if threads[0].ID != "b" || threads[1].ID != "d" || threads[2].ID != "a" || threads[3].ID != "c" {
		t.Errorf("sort order wrong: %v", threads)
	}
}

// ── normalizeSubreddit ────────────────────────────────────────────────────

func TestNormalizeSubreddit_Variants(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"r/golang", "golang"},
		{"/r/golang", "golang"},
		{"golang", "golang"},
		{"https://reddit.com/r/golang", "golang"},
		{"r/golang/", "golang"},
		{"   r/spaces   ", "spaces"},
		{"has space", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := normalizeSubreddit(tc.input)
		if got != tc.want {
			t.Errorf("normalizeSubreddit(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ── handleVisibilityScore: missing branches ───────────────────────────────

func TestHandleVisibilityScore_WithOptimizationData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert various analysis types to exercise per-source scoring branches
	optID := primitive.NewObjectID()
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "vscore.com",
		"_id":       optID,
		"question":  "Test query",
		"createdAt": time.Now(),
		"result": bson.M{
			"overallScore": 80,
		},
	})

	handler := handleVisibilityScore(db)
	req := testRequest(t, "GET", "/api/visibility-score/vscore.com", nil)
	req.SetPathValue("domain", "vscore.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}
