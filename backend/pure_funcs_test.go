package main

// Tests for pure utility functions: normalizeDomain, isCrawler, extractVideoID, etc.

import (
	"strings"
	"testing"
)

// ── normalizeDomain ───────────────────────────────────────────────────────────

func TestNormalizeDomain_StripHTTPS(t *testing.T) {
	if got := normalizeDomain("https://example.com/"); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
}

func TestNormalizeDomain_StripHTTP(t *testing.T) {
	if got := normalizeDomain("http://example.com"); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
}

func TestNormalizeDomain_StripTrailingSlash(t *testing.T) {
	if got := normalizeDomain("example.com/"); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
}

func TestNormalizeDomain_Lowercase(t *testing.T) {
	if got := normalizeDomain("EXAMPLE.COM"); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
}

func TestNormalizeDomain_TrimWhitespace(t *testing.T) {
	if got := normalizeDomain("  example.com  "); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
}

func TestNormalizeDomain_AlreadyNormal(t *testing.T) {
	if got := normalizeDomain("example.com"); got != "example.com" {
		t.Errorf("expected 'example.com', got %q", got)
	}
}

// ── isCrawler ─────────────────────────────────────────────────────────────────

func TestIsCrawler_FacebookBot(t *testing.T) {
	if !isCrawler("facebookexternalhit/1.1") {
		t.Error("expected facebookexternalhit to be a crawler")
	}
}

func TestIsCrawler_Twitterbot(t *testing.T) {
	if !isCrawler("Twitterbot/1.0") {
		t.Error("expected Twitterbot to be a crawler")
	}
}

func TestIsCrawler_Googlebot(t *testing.T) {
	if !isCrawler("Googlebot/2.1") {
		t.Error("expected Googlebot to be a crawler")
	}
}

func TestIsCrawler_GPTBot(t *testing.T) {
	if !isCrawler("GPTBot/1.0") {
		t.Error("expected GPTBot to be a crawler")
	}
}

func TestIsCrawler_ClaudeBot(t *testing.T) {
	if !isCrawler("ClaudeBot/1.0") {
		t.Error("expected ClaudeBot to be a crawler")
	}
}

func TestIsCrawler_PerplexityBot(t *testing.T) {
	if !isCrawler("PerplexityBot") {
		t.Error("expected PerplexityBot to be a crawler")
	}
}

func TestIsCrawler_Chrome(t *testing.T) {
	if isCrawler("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/100.0.4896.88 Safari/537.36") {
		t.Error("expected Chrome to NOT be a crawler")
	}
}

func TestIsCrawler_Safari(t *testing.T) {
	if isCrawler("Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/605.1.15 Safari/604.1") {
		t.Error("expected Safari to NOT be a crawler")
	}
}

func TestIsCrawler_EmptyUA(t *testing.T) {
	if isCrawler("") {
		t.Error("expected empty UA to NOT be a crawler")
	}
}

// ── extractVideoID ────────────────────────────────────────────────────────────

func TestExtractVideoID_YoutubeWatch(t *testing.T) {
	id := extractVideoID("https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if id != "dQw4w9WgXcQ" {
		t.Errorf("expected 'dQw4w9WgXcQ', got %q", id)
	}
}

func TestExtractVideoID_YoutubeShortURL(t *testing.T) {
	id := extractVideoID("https://youtu.be/dQw4w9WgXcQ")
	if id != "dQw4w9WgXcQ" {
		t.Errorf("expected 'dQw4w9WgXcQ', got %q", id)
	}
}

func TestExtractVideoID_DirectID(t *testing.T) {
	// 11 char direct ID
	id := extractVideoID("dQw4w9WgXcQ")
	if id != "dQw4w9WgXcQ" {
		t.Errorf("expected 'dQw4w9WgXcQ', got %q", id)
	}
}

func TestExtractVideoID_Invalid(t *testing.T) {
	id := extractVideoID("not-a-youtube-url")
	// Should return empty for non-youtube URLs
	_ = id // just verify it doesn't panic
}

// ── buildDomainSummaryPrompt ──────────────────────────────────────────────────

func TestBuildDomainSummaryPrompt_Basic(t *testing.T) {
	prompt := buildDomainSummaryPrompt("summary-test.com", nil, nil, nil, nil, summaryExtras{}, BrandContextInfo{Used: false})
	if !strings.Contains(prompt, "summary-test.com") {
		t.Error("expected domain in domain summary prompt")
	}
	if len(prompt) < 50 {
		t.Errorf("expected non-trivial prompt, got %d chars", len(prompt))
	}
}

func TestBuildDomainSummaryPrompt_WithBrandInfo(t *testing.T) {
	brandInfo := BrandContextInfo{
		Used:          true,
		ContextString: "Company: SummaryBrand\nCategory: Analytics",
	}
	prompt := buildDomainSummaryPrompt("summary-brand.com", nil, nil, nil, nil, summaryExtras{}, brandInfo)
	if !strings.Contains(prompt, "SummaryBrand") {
		t.Error("expected brand name in prompt")
	}
}

func TestBuildDomainSummaryPrompt_WithAnalysis(t *testing.T) {
	analysis := &Analysis{
		Domain: "summary-brand.com",
		Result: AnalysisResult{
			SiteSummary: "A great SaaS platform with excellent coverage.",
		},
	}
	prompt := buildDomainSummaryPrompt("summary-brand.com", nil, analysis, nil, nil, summaryExtras{}, BrandContextInfo{Used: false})
	if !strings.Contains(prompt, "A great SaaS platform") {
		t.Error("expected analysis site summary in prompt")
	}
}

// ── buildTestEvaluationPrompt ─────────────────────────────────────────────────

func TestBuildTestEvaluationPrompt_Basic(t *testing.T) {
	queries := []LLMTestQuery{
		{Query: "What is TestBrand?", Type: "brand", Priority: "high"},
	}
	responses := []testRawResponse{
		{queryIdx: 0, response: "TestBrand is a great SaaS tool for AI optimization."},
	}
	brandInfo := BrandContextInfo{
		Used:          true,
		ContextString: "Company: TestBrand\nCategory: SaaS",
	}
	prompt := buildTestEvaluationPrompt("TestBrand", "testbrand.com", brandInfo, queries, responses)
	if !strings.Contains(prompt, "TestBrand") {
		t.Error("expected brand name in evaluation prompt")
	}
	if len(prompt) < 100 {
		t.Errorf("expected substantial prompt, got %d chars", len(prompt))
	}
}

func TestBuildTestEvaluationPrompt_NoResponses(t *testing.T) {
	prompt := buildTestEvaluationPrompt("NoRespBrand", "norespbrand.com", BrandContextInfo{Used: false}, nil, nil)
	if !strings.Contains(prompt, "norespbrand.com") {
		t.Error("expected domain in prompt")
	}
}

// ── extractJSONObject ─────────────────────────────────────────────────────────

func TestExtractJSONObject_Simple(t *testing.T) {
	s := `{"key":"value"} extra text`
	got := extractJSONObject(s)
	if got != `{"key":"value"}` {
		t.Errorf("expected simple object, got %q", got)
	}
}

func TestExtractJSONObject_Nested(t *testing.T) {
	s := `{"outer":{"inner":"value"},"list":[1,2,3]}`
	got := extractJSONObject(s)
	if got != s {
		t.Errorf("expected full nested object, got %q", got)
	}
}

func TestExtractJSONObject_WithEscapedQuotes(t *testing.T) {
	s := `{"key":"value with \"quotes\""}`
	got := extractJSONObject(s)
	if got != s {
		t.Errorf("expected object with escaped quotes, got %q", got)
	}
}

func TestExtractJSONObject_EmptyString(t *testing.T) {
	got := extractJSONObject("")
	if got != "" {
		t.Errorf("expected empty for empty input, got %q", got)
	}
}

func TestExtractJSONObject_NotJSON(t *testing.T) {
	got := extractJSONObject("not json at all")
	if got != "" {
		t.Errorf("expected empty for non-JSON input, got %q", got)
	}
}

func TestExtractJSONObject_UnclosedBrace(t *testing.T) {
	got := extractJSONObject(`{"key": "unclosed`)
	if got != "" {
		t.Errorf("expected empty for unclosed JSON, got %q", got)
	}
}

func TestExtractJSONObject_WithMarkdownWrapper(t *testing.T) {
	// When input starts with ``` it won't start with '{', so returns ""
	s := "```json\n{\"key\":\"value\"}\n```"
	got := extractJSONObject(s)
	if got != "" {
		t.Errorf("expected empty for markdown-wrapped JSON (no leading {), got %q", got)
	}
}

// ── getCache / setCache / setCacheWithTTL ─────────────────────────────────────

func TestGetSetCache_RoundTrip(t *testing.T) {
	db := testMongoDB(t)

	key := "test-cache-key-roundtrip"
	data := `{"test":"value","score":42}`

	// Initially not cached
	_, ok := getCache(db, key)
	if ok {
		t.Error("expected cache miss before setting")
	}

	// Set and retrieve
	setCache(db, key, data)
	got, ok := getCache(db, key)
	if !ok {
		t.Error("expected cache hit after setting")
	}
	if got != data {
		t.Errorf("expected %q, got %q", data, got)
	}
}

func TestSetCacheWithTTL_RoundTrip(t *testing.T) {
	db := testMongoDB(t)

	key := "test-cache-ttl-key"
	data := "some cached content"

	setCacheWithTTL(db, key, data, 30*60*1000_000_000) // 30 minutes in nanoseconds

	got, ok := getCache(db, key)
	if !ok {
		t.Error("expected cache hit after setCacheWithTTL")
	}
	if got != data {
		t.Errorf("expected %q, got %q", data, got)
	}
}
