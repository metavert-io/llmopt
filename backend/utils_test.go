package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"llmopt/internal/saas"
)

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"example.com", "example.com"},
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"HTTPS://Example.COM", "example.com"},
		{"https://example.com/", "example.com"},
		{"https://example.com///", "example.com"},
		{"  https://example.com  ", "example.com"},
		{"", ""},
		{"http://", ""},
		{"example.com/path/here", "example.com/path/here"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDomain(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTenantFilter_WithTenant(t *testing.T) {
	ctx := testAuthContext("tenant-123", "user-1")
	filter := tenantFilter(ctx, bson.D{{Key: "domain", Value: "example.com"}})

	// Should have domain + tenantId
	if len(filter) != 2 {
		t.Fatalf("expected 2 elements in filter, got %d", len(filter))
	}
	if filter[0].Key != "domain" {
		t.Errorf("first key: got %q, want %q", filter[0].Key, "domain")
	}
	if filter[1].Key != "tenantId" || filter[1].Value != "tenant-123" {
		t.Errorf("second element: got %v", filter[1])
	}
}

func TestTenantFilter_NoTenant(t *testing.T) {
	ctx := context.Background()
	filter := tenantFilter(ctx, bson.D{{Key: "domain", Value: "example.com"}})

	if len(filter) != 1 {
		t.Fatalf("expected 1 element (no tenant), got %d", len(filter))
	}
}

func TestTenantFilter_EmptyBase(t *testing.T) {
	ctx := testAuthContext("tid", "uid")
	filter := tenantFilter(ctx, bson.D{})
	if len(filter) != 1 || filter[0].Key != "tenantId" {
		t.Errorf("expected tenantId only, got %v", filter)
	}
}

func TestGenerateShareID(t *testing.T) {
	id := generateShareID()
	if len(id) != 12 {
		t.Errorf("expected 12 chars, got %d: %q", len(id), id)
	}

	// Should be base62
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			t.Errorf("non-base62 char %q in %q", c, id)
		}
	}

	// Should be unique (statistically)
	id2 := generateShareID()
	if id == id2 {
		t.Error("two consecutive generateShareID() returned the same value")
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ErrCodeAPIError},
		{"overloaded", fmt.Errorf("wrapped: %w", ErrOverloaded), ErrCodeAPIOverloaded},
		{"stalled", fmt.Errorf("wrapped: %w", ErrStreamStalled), ErrCodeStreamStalled},
		{"cancelled", fmt.Errorf("context canceled"), ErrCodeCancelled},
		{"request_cancelled", fmt.Errorf("request cancelled by client"), ErrCodeCancelled},
		{"auth_401", fmt.Errorf("Claude API error (401): bad key"), ErrCodeAPIKeyInvalid},
		{"auth_403", fmt.Errorf("Claude API error (403): forbidden"), ErrCodeAPIKeyInvalid},
		{"generic", errors.New("something broke"), ErrCodeAPIError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if got != tt.want {
				t.Errorf("classifyError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestUserFriendlyError(t *testing.T) {
	codes := []string{
		ErrCodeAPIOverloaded, ErrCodeAPIError, ErrCodeAPIKeyInvalid,
		ErrCodeCancelled, ErrCodeStreamStalled, ErrCodeParseError,
		"unknown_code",
	}
	for _, code := range codes {
		msg := userFriendlyError(code)
		if msg == "" {
			t.Errorf("userFriendlyError(%q) returned empty string", code)
		}
	}
}

func TestHasSubstantiveChanges(t *testing.T) {
	base := BrandProfile{
		BrandName:   "Test Brand",
		Description: "A test brand",
		Categories:  []string{"tech"},
	}

	t.Run("no_change", func(t *testing.T) {
		same := base
		if hasSubstantiveChanges(base, same) {
			t.Error("identical profiles should not have substantive changes")
		}
	})

	t.Run("name_change", func(t *testing.T) {
		changed := base
		changed.BrandName = "New Name"
		if !hasSubstantiveChanges(base, changed) {
			t.Error("different BrandName should be substantive")
		}
	})

	t.Run("category_change", func(t *testing.T) {
		changed := base
		changed.Categories = []string{"tech", "gaming"}
		if !hasSubstantiveChanges(base, changed) {
			t.Error("different Categories should be substantive")
		}
	})

	t.Run("nil_vs_empty_slices", func(t *testing.T) {
		a := BrandProfile{BrandName: "X"}
		b := BrandProfile{BrandName: "X", Categories: []string{}}
		if hasSubstantiveChanges(a, b) {
			t.Error("nil vs empty slice should not be substantive")
		}
	})

	t.Run("non_content_field_ignored", func(t *testing.T) {
		a := base
		b := base
		b.Public = true // non-content field
		if hasSubstantiveChanges(a, b) {
			t.Error("Public field change should not be substantive")
		}
	})
}

// ── isCrawler ──────────────────────────────────────────────────────────

func TestIsCrawler(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want bool
	}{
		{"googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true},
		{"twitterbot", "Twitterbot/1.0", true},
		{"facebookbot", "facebookexternalhit/1.1", true},
		{"gptbot", "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.0)", true},
		{"claudebot", "ClaudeBot/1.0", true},
		{"perplexitybot", "PerplexityBot/1.0", true},
		{"normal_browser", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36", false},
		{"empty", "", false},
		{"curl", "curl/7.64.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCrawler(tt.ua); got != tt.want {
				t.Errorf("isCrawler(%q) = %v, want %v", tt.ua, got, tt.want)
			}
		})
	}
}

// ── todoSummary ────────────────────────────────────────────────────────

func TestTodoSummary(t *testing.T) {
	tests := []struct {
		name, action, impact, wantContains string
	}{
		{"short_action_no_impact", "Fix the title tag", "", "Fix the title tag"},
		{"action_with_impact", "Add schema markup", "Improve visibility", "Add schema markup → Improve visibility"},
		{"long_action_truncated", strings.Repeat("a", 200), "", "..."},
		{"first_sentence_only", "Fix the title. Then do more stuff later.", "", "Fix the title."},
		{"long_impact_truncated", "Do X", strings.Repeat("b", 100), "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := todoSummary(tt.action, tt.impact)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("todoSummary(%q, %q) = %q, want to contain %q", tt.action, tt.impact, got, tt.wantContains)
			}
		})
	}
}

// ── computeBrandCompleteness ───────────────────────────────────────────

func TestComputeBrandCompleteness(t *testing.T) {
	t.Run("empty_profile", func(t *testing.T) {
		score := computeBrandCompleteness(BrandProfile{})
		if score != 0 {
			t.Errorf("empty profile: got %d, want 0", score)
		}
	})

	t.Run("partial_profile", func(t *testing.T) {
		p := BrandProfile{
			BrandName:   "Test",    // +8
			Description: "A brand", // +10
		}
		score := computeBrandCompleteness(p)
		if score != 18 {
			t.Errorf("partial profile: got %d, want 18", score)
		}
	})

	t.Run("full_profile", func(t *testing.T) {
		p := BrandProfile{
			BrandName:       "Test",
			Description:     "A brand",
			Categories:      []string{"tech"},
			Products:        []string{"app"},
			PrimaryAudience: "developers",
			KeyUseCases:     []string{"coding"},
			Competitors: []BrandCompetitor{
				{Name: "a"}, {Name: "b"}, {Name: "c"},
			},
			TargetQueries: []TargetQuery{
				{Query: "1"}, {Query: "2"}, {Query: "3"}, {Query: "4"}, {Query: "5"},
			},
			KeyMessages:      []KeyMessage{{Claim: "msg"}},
			Differentiators:  []string{"diff"},
			PresenceComplete: true,
		}
		score := computeBrandCompleteness(p)
		if score != 100 {
			t.Errorf("full profile: got %d, want 100", score)
		}
	})
}

// ── extractJSON ────────────────────────────────────────────────────────

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"valid_json", `{"key":"value"}`, `{"key":"value"}`},
		{"json_code_block", "```json\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"plain_code_block", "```\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"embedded_in_text", "Here is the result: {\"key\":\"value\"} and more text", `{"key":"value"}`},
		{"no_json", "just plain text", "just plain text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON = %q, want %q", got, tt.want)
			}
		})
	}
}

// ── sortThreadsByScore ─────────────────────────────────────────────────

func TestSortThreadsByScore(t *testing.T) {
	threads := []RedditThread{
		{Title: "low", Score: 5},
		{Title: "high", Score: 100},
		{Title: "mid", Score: 50},
	}
	sortThreadsByScore(threads)
	if threads[0].Title != "high" || threads[1].Title != "mid" || threads[2].Title != "low" {
		t.Errorf("sort order wrong: %v", []string{threads[0].Title, threads[1].Title, threads[2].Title})
	}
}

func TestSortThreadsByScore_Empty(t *testing.T) {
	sortThreadsByScore(nil)            // should not panic
	sortThreadsByScore([]RedditThread{}) // should not panic
}

// ── truncateStr ────────────────────────────────────────────────────────

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ── buildDomainSummaryPrompt ───────────────────────────────────────────

func TestBuildDomainSummaryPrompt(t *testing.T) {
	t.Run("with_analysis", func(t *testing.T) {
		analysis := &Analysis{
			Result: AnalysisResult{
				SiteSummary:  "Great site",
				CrawledPages: []CrawledPage{{URL: "https://example.com"}},
				Questions:    []Question{{Question: "What is it?"}},
			},
		}
		prompt := buildDomainSummaryPrompt("example.com", nil, analysis, nil, nil, BrandContextInfo{})
		if !strings.Contains(prompt, "example.com") {
			t.Error("prompt should contain domain")
		}
		if !strings.Contains(prompt, "SITE ANALYSIS") {
			t.Error("prompt should contain site analysis section")
		}
		if !strings.Contains(prompt, "Site Analysis") {
			t.Error("prompt should list Site Analysis in Reports Included")
		}
	})

	t.Run("with_optimizations", func(t *testing.T) {
		opts := []Optimization{
			{Question: "How to improve?", Result: OptimizationResult{OverallScore: 70}},
		}
		prompt := buildDomainSummaryPrompt("test.com", opts, nil, nil, nil, BrandContextInfo{})
		if !strings.Contains(prompt, "OPTIMIZATION REPORTS") {
			t.Error("prompt should contain optimization section")
		}
	})

	t.Run("with_brand_context", func(t *testing.T) {
		brandInfo := BrandContextInfo{Used: true, ContextString: "Brand: Test Corp"}
		prompt := buildDomainSummaryPrompt("test.com", nil, nil, nil, nil, brandInfo)
		if !strings.Contains(prompt, "Brand Intelligence Context") {
			t.Error("prompt should include brand context")
		}
	})
}

// ── handleRobotsTxt ────────────────────────────────────────────────────

func TestHandleRobotsTxt(t *testing.T) {
	handler := handleRobotsTxt()
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	body := w.Body.String()
	if !strings.Contains(body, "User-agent: *") {
		t.Error("robots.txt should contain User-agent directive")
	}
	if !strings.Contains(body, "Disallow: /api/") {
		t.Error("robots.txt should disallow /api/")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("Content-Type: got %q, want text/plain", ct)
	}
}

// ── handleOptions ──────────────────────────────────────────────────────

func TestHandleOptions(t *testing.T) {
	req := httptest.NewRequest("OPTIONS", "/api/anything", nil)
	w := httptest.NewRecorder()
	handleOptions(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── withCORS ───────────────────────────────────────────────────────────

func TestWithCORS(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := withCORS(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS Allow-Origin header")
	}
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "GET") {
		t.Error("missing GET in Allow-Methods")
	}
}

// ── handleAPIv1Docs ────────────────────────────────────────────────────

func TestHandleAPIv1Docs(t *testing.T) {
	handler := handleAPIv1Docs()
	req := httptest.NewRequest("GET", "/api/v1/docs", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/markdown") {
		t.Errorf("Content-Type: got %q, want text/markdown", ct)
	}
	if !strings.Contains(w.Body.String(), "LLM Optimizer API") {
		t.Error("docs should contain API title")
	}
}

// ── handleListProviderModels (with empty providers) ───────────────────

func TestHandleListProviderModels_Empty(t *testing.T) {
	withTestProviders(t) // empty providers

	handler := handleListProviderModels(nil, nil, "", false)
	req := httptest.NewRequest("GET", "/api/models", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestIsShareAdmin(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{"owner", true},
		{"admin", true},
		{"member", false},
		{"viewer", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			info := &saas.AuthInfo{
				UserID:   "u1",
				TenantID: "t1",
				Role:     tt.role,
				Method:   "test",
			}
			ctx := saas.SetAuthContext(context.Background(), info)
			if got := isShareAdmin(ctx); got != tt.want {
				t.Errorf("isShareAdmin(role=%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

// ── buildPillar prompts ────────────────────────────────────────────────

func TestBuildPillarPrompts(t *testing.T) {
	p := videoContextParams{
		Domain:          "example.com",
		BrandName:       "ExampleCo",
		BrandInfo:       BrandContextInfo{Used: true, ContextString: "Brand: ExampleCo"},
		OwnVideos:       []YouTubeVideo{{VideoID: "v1", Title: "Own Video 1", ViewCount: 1000}},
		Competitors:     []string{"competitor.com"},
		SearchTerms:     []string{"example product"},
		ThirdPartyCount: 5,
		Digests: []BatchDigest{
			{
				BatchIndex:     0,
				VideoCount:     5,
				Summary:        "Batch summary",
				TopCreators:    []string{"Creator1"},
				TopicsCovered:  []string{"topic1"},
				SentimentTally: map[string]int{"positive": 3, "neutral": 1, "negative": 1, "none": 0},
				NotableQuotes:  []string{"great product"},
				ContentGaps:    []string{"missing topic"},
			},
		},
	}

	t.Run("pillar1", func(t *testing.T) {
		result := buildPillar1Prompt(p)
		if !strings.Contains(result, "TRANSCRIPT AUTHORITY") {
			t.Error("pillar1 should contain TRANSCRIPT AUTHORITY")
		}
		if !strings.Contains(result, "ExampleCo") {
			t.Error("pillar1 should contain brand name")
		}
	})

	t.Run("pillar2", func(t *testing.T) {
		result := buildPillar2Prompt(p)
		if !strings.Contains(result, "TOPICAL DOMINANCE") {
			t.Error("pillar2 should contain TOPICAL DOMINANCE")
		}
	})

	t.Run("pillar3", func(t *testing.T) {
		result := buildPillar3Prompt(p)
		if !strings.Contains(result, "CITATION NETWORK") {
			t.Error("pillar3 should contain CITATION NETWORK")
		}
	})

	t.Run("pillar4", func(t *testing.T) {
		result := buildPillar4Prompt(p)
		if !strings.Contains(result, "BRAND NARRATIVE") {
			t.Error("pillar4 should contain BRAND NARRATIVE")
		}
	})
}

// ── buildSynthesisPrompt ───────────────────────────────────────────────

func TestBuildSynthesisPrompt(t *testing.T) {
	result := buildSynthesisPrompt("ExampleCo", "p1 data", "p2 data", "p3 data", "p4 data")
	if !strings.Contains(result, "ExampleCo") {
		t.Error("synthesis prompt should contain brand name")
	}
	if !strings.Contains(result, "p1 data") {
		t.Error("synthesis prompt should contain pillar data")
	}
}

// ── buildRedditAuthorityPrompt ─────────────────────────────────────────

func TestBuildRedditAuthorityPrompt(t *testing.T) {
	threads := []RedditThread{
		{Title: "Thread 1", Score: 100, URL: "https://reddit.com/r/test/1", TopComments: []RedditComment{{Body: "Good stuff", Score: 50}}},
	}
	brandInfo := BrandContextInfo{Used: true, ContextString: "Brand: TestCo"}
	result := buildRedditAuthorityPrompt("test.com", threads, []string{"comp.com"}, []string{"test keyword"}, brandInfo)

	if !strings.Contains(result, "test.com") {
		t.Error("prompt should contain domain")
	}
	if !strings.Contains(result, "Thread 1") {
		t.Error("prompt should contain thread title")
	}
	if !strings.Contains(result, "Brand Intelligence") {
		t.Error("prompt should contain brand context")
	}
}

// ── buildSearchVisibilityPrompt ────────────────────────────────────────

func TestBuildSearchVisibilityPrompt(t *testing.T) {
	brandInfo := BrandContextInfo{Used: true, ContextString: "Brand: TestCo"}
	result := buildSearchVisibilityPrompt("test.com", brandInfo, []string{"comp.com"}, []string{"ai tools"}, []string{"what is the best ai tool"})

	if !strings.Contains(result, "test.com") {
		t.Error("prompt should contain domain")
	}
	if !strings.Contains(result, "ai tools") {
		t.Error("prompt should contain category keywords")
	}
}

// ── buildTestEvaluationPrompt ──────────────────────────────────────────

func TestBuildTestEvaluationPrompt(t *testing.T) {
	brandInfo := BrandContextInfo{Used: true, ContextString: "Brand: TestCo"}
	queries := []LLMTestQuery{{Query: "What is the best tool?", Type: "recommendation"}}
	responses := []testRawResponse{{providerID: "anthropic", providerName: "Anthropic", model: "claude", queryIdx: 0, response: "TestCo is great"}}
	result := buildTestEvaluationPrompt("TestCo", "test.com", brandInfo, queries, responses)

	if !strings.Contains(result, "TestCo") {
		t.Error("prompt should contain brand name")
	}
	if !strings.Contains(result, "What is the best tool") {
		t.Error("prompt should contain query")
	}
}

// ── sendSSE ────────────────────────────────────────────────────────────

func TestSendSSE(t *testing.T) {
	w := httptest.NewRecorder()
	sendSSE(w, w, "status", map[string]string{"message": "test"})

	body := w.Body.String()
	if !strings.Contains(body, "event: status") {
		t.Error("SSE should contain event type")
	}
	if !strings.Contains(body, `"message":"test"`) {
		t.Error("SSE should contain JSON data")
	}
}

// ── sendSSEError ───────────────────────────────────────────────────────

func TestSendSSEError(t *testing.T) {
	w := httptest.NewRecorder()
	sendSSEError(w, w, ErrCodeAPIOverloaded)

	body := w.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Error("SSE error should have error event type")
	}
	if !strings.Contains(body, ErrCodeAPIOverloaded) {
		t.Error("SSE error should contain error code")
	}
	if !strings.Contains(body, `"upstream":true`) {
		t.Error("SSE error for overloaded should be upstream")
	}
}

func TestSendSSEError_NonUpstream(t *testing.T) {
	w := httptest.NewRecorder()
	sendSSEError(w, w, ErrCodeCancelled)

	body := w.Body.String()
	if !strings.Contains(body, `"upstream":false`) {
		t.Error("SSE error for cancelled should not be upstream")
	}
}
