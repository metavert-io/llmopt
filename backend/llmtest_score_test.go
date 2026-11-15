package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func scoredResult(pid string, run int, search, mentioned bool, score int, citations ...string) LLMTestProviderResult {
	pr := LLMTestProviderResult{ProviderID: pid, ProviderName: pid, Response: "answer", Run: run, Search: search,
		Mentioned: mentioned, Recommended: mentioned, Score: score, Sentiment: "neutral", Accuracy: "accurate", Citations: citations}
	for _, u := range citations {
		if isOwnSite(u, "example.com") {
			pr.CitesOwnSite = true
		}
	}
	return pr
}

func TestSummarizeTest_RunsSpreadAndSearch(t *testing.T) {
	results := []LLMTestQueryResult{
		{ProviderResults: []LLMTestProviderResult{
			scoredResult("a", 1, false, true, 80), scoredResult("a", 2, false, true, 80), scoredResult("a", 3, false, false, 20),
			scoredResult("a", 1, true, true, 90, "https://www.example.com/pricing", "https://reviews.test/best", "https://reviews.test/other"),
		}},
		{ProviderResults: []LLMTestProviderResult{
			scoredResult("a", 1, false, false, 20), scoredResult("a", 2, false, false, 20), scoredResult("a", 3, false, false, 20),
			scoredResult("a", 1, true, false, 30, "https://reviews.test/x"),
			{ProviderID: "a", Run: 1, Search: true, Response: "Error: timeout"},
		}},
	}
	sum := summarizeTest([]testProviderInfo{{id: "a", name: "A", model: "m", searchFailed: 1}}, results, 3, "example.com")
	if len(sum.providers) != 1 {
		t.Fatalf("providers = %+v", sum.providers)
	}
	s := sum.providers[0]
	// Remembered: 6 answers, 2 mentioned → 33%; scores (80+80+20+20+20+20)/6 = 40
	if s.Answered != 6 || s.MentionRate != 33 || s.OverallScore != 40 {
		t.Errorf("summary = %+v", s)
	}
	// Run 1 and 2: 1 of 2 mentioned (50%, score 50); run 3: 0 (0%, score 20)
	if sp := s.Spread; sp == nil || sp.MentionLow != 0 || sp.MentionHigh != 50 || sp.ScoreLow != 20 || sp.ScoreHigh != 50 || sp.Consistency != 50 {
		t.Errorf("spread = %+v", s.Spread)
	}
	if ss := s.Search; ss == nil || ss.Answered != 2 || ss.Failed != 1 || ss.MentionRate != 50 || ss.OwnSiteRate != 50 || ss.OverallScore != 60 {
		t.Errorf("search = %+v", s.Search)
	}
	if sum.overall != 40 || len(sum.scoreRange) != 2 || sum.scoreRange[0] != 20 || sum.scoreRange[1] != 50 {
		t.Errorf("overall %d range %v", sum.overall, sum.scoreRange)
	}
	if sum.searchScore == nil || *sum.searchScore != 60 {
		t.Errorf("search score = %v", sum.searchScore)
	}
	// reviews.test is cited by 2 answers (counted once per answer), example.com by 1
	want := []LLMTestCitedDomain{{Domain: "reviews.test", Answers: 2}, {Domain: "example.com", Answers: 1, Own: true}}
	if fmt.Sprint(sum.topCitedDomains) != fmt.Sprint(want) {
		t.Errorf("cited = %+v, want %+v", sum.topCitedDomains, want)
	}
}

func TestSummarizeTest_SingleRunHasNoSpread(t *testing.T) {
	results := []LLMTestQueryResult{{ProviderResults: []LLMTestProviderResult{
		scoredResult("a", 1, false, true, 70),
		{ProviderID: "a", Run: 1, Unscored: true, Response: "not graded"},
	}}}
	sum := summarizeTest([]testProviderInfo{{id: "a"}}, results, 1, "example.com")
	s := sum.providers[0]
	if s.Spread != nil || s.Search != nil || sum.scoreRange != nil || sum.searchScore != nil {
		t.Errorf("one run without search must have no spread or search: %+v %v", s, sum.scoreRange)
	}
	if s.Answered != 1 || s.OverallScore != 70 {
		t.Errorf("an ungraded answer must count in no rate: %+v", s)
	}
}

func TestMatchEvaluations(t *testing.T) {
	batch := []testRawResponse{
		{id: 4, queryIdx: 0, providerID: "openai"},
		{id: 5, queryIdx: 0, providerID: "openai", run: 2},
		{id: 6, queryIdx: 1, providerID: "anthropic"},
	}
	var evals []testEvalEntry
	if err := json.Unmarshal([]byte(`[
		{"response_id": "R5", "score": 55},
		{"response_id": 99, "query_index": 0, "provider_id": "OpenAI", "score": 44},
		{"query_index": 1, "provider_id": "Anthropic", "score": 66},
		{"response_id": "R6", "score": 11}
	]`), &evals); err != nil {
		t.Fatal(err)
	}
	got := matchEvaluations(batch, evals, map[string]string{"openai": "openai", "anthropic": "anthropic"})
	if got[5].Score != 55 || got[4].Score != 44 || got[6].Score != 11 {
		t.Errorf("matched = %+v", got)
	}
	if len(got) != 3 {
		t.Errorf("want 3 matches, got %d", len(got))
	}
}

func TestEvalBatches(t *testing.T) {
	var rs []testRawResponse
	for i := 0; i < 2*evalBatchSize+5; i++ {
		rs = append(rs, testRawResponse{id: i})
	}
	rs = append(rs, testRawResponse{id: 999, err: fmt.Errorf("no answer")})
	b := evalBatches(rs)
	if len(b) != 3 || len(b[0]) != evalBatchSize || len(b[2]) != 5 {
		t.Errorf("batches = %d (%d, …, %d)", len(b), len(b[0]), len(b[len(b)-1]))
	}
	for _, batch := range b {
		for _, r := range batch {
			if r.err != nil {
				t.Error("failed calls must not be graded")
			}
		}
	}
}

func TestParseResponsesWebSearch(t *testing.T) {
	ans, err := parseResponsesWebSearch([]byte(`{"output": [
		{"type": "web_search_call", "status": "completed"},
		{"type": "message", "content": [{"type": "output_text", "text": "Try Example.",
			"annotations": [{"type": "url_citation", "url": "https://example.com/a"}, {"type": "url_citation", "url": "https://example.com/a"}]}]}
	], "citations": ["https://other.test/b"]}`))
	if err != nil || ans.Text != "Try Example." || fmt.Sprint(ans.Sources) != "[https://example.com/a https://other.test/b]" {
		t.Errorf("ans = %+v, err %v", ans, err)
	}
	if _, err := parseResponsesWebSearch([]byte(`{"output": []}`)); err == nil {
		t.Error("an empty answer must be an error")
	}
}

func TestParseGeminiWebSearch(t *testing.T) {
	ans, err := parseGeminiWebSearch([]byte(`{"candidates": [{"content": {"parts": [{"text": "thinking", "thought": true}, {"text": "Example is good."}]},
		"groundingMetadata": {"groundingChunks": [
			{"web": {"uri": "https://vertexaisearch.cloud.google.com/grounding-api-redirect/abc", "title": "example.com"}},
			{"web": {"uri": "https://reviews.test/page", "title": "Reviews"}}
		]}}]}`))
	if err != nil || ans.Text != "Example is good." || fmt.Sprint(ans.Sources) != "[https://example.com/ https://reviews.test/page]" {
		t.Errorf("ans = %+v, err %v", ans, err)
	}
}

func TestIsOwnSite(t *testing.T) {
	for u, want := range map[string]bool{
		"https://example.com/x": true, "https://www.example.com": true, "https://blog.example.com/p": true,
		"https://notexample.com": false, "https://example.com.evil.test": false, "": false,
	} {
		if got := isOwnSite(u, "www.Example.com"); got != want {
			t.Errorf("isOwnSite(%q) = %v", u, got)
		}
	}
}

// A paused Anthropic search turn is sent back to resume, and the answer's
// citations become its sources.
func TestAnthropicWebSearch_ResumesPausedTurn(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		mu.Lock()
		bodies = append(bodies, m)
		n := len(bodies)
		mu.Unlock()
		if n == 1 {
			io.WriteString(w, `{"stop_reason": "pause_turn", "content": [{"type": "server_tool_use", "id": "s1", "name": "web_search", "input": {"query": "q"}}]}`)
			return
		}
		io.WriteString(w, `{"stop_reason": "end_turn", "content": [
			{"type": "web_search_tool_result", "tool_use_id": "s1", "content": [{"type": "web_search_result", "url": "https://seen-not-cited.test"}]},
			{"type": "text", "text": "Example is a fine choice", "citations": [{"type": "web_search_result_location", "url": "https://example.com/p"}]},
			{"type": "text", "text": "."}
		]}`)
	}))
	defer srv.Close()
	orig := anthropicAPIBase
	anthropicAPIBase = srv.URL
	defer func() { anthropicAPIBase = orig }()

	ans, err := (&AnthropicProvider{}).CallWithWebSearch(context.Background(), "k", "claude-opus-5-5", "best?", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if ans.Text != "Example is a fine choice." || fmt.Sprint(ans.Sources) != "[https://example.com/p]" {
		t.Errorf("ans = %+v", ans)
	}
	if len(bodies) != 2 {
		t.Fatalf("want 2 requests, got %d", len(bodies))
	}
	msgs, _ := bodies[1]["messages"].([]any)
	if len(msgs) != 2 || msgs[1].(map[string]any)["role"] != "assistant" {
		t.Errorf("the resumed request must carry the paused turn: %v", bodies[1]["messages"])
	}
	if tools, _ := bodies[0]["tools"].([]any); len(tools) != 1 {
		t.Errorf("tools = %v", bodies[0]["tools"])
	}
}

// searchingProvider is a test provider that can also answer with web search.
type searchingProvider struct {
	*TestProvider
	searchFn func(prompt string) (webSearchAnswer, error)
}

func (p *searchingProvider) CallWithWebSearch(ctx context.Context, apiKey, model, prompt string, maxTokens int) (webSearchAnswer, error) {
	return p.searchFn(prompt)
}

var evalBlockRe = regexp.MustCompile(`### Response R(\d+) — Query (\d+) \(asked to [^,]+, provider_id: (\w+)[^)]*\)\n\*\*Query\*\*: [^\n]*\n\*\*Response\*\*: ([^\n]*)`)

// gradeByResponseID grades every response in an evaluation prompt by id:
// mentioned (score 80) when the answer names the brand, else 20.
func gradeByResponseID(prompt string) string {
	var evals []map[string]any
	for _, m := range evalBlockRe.FindAllStringSubmatch(prompt, -1) {
		mentioned := strings.Contains(m[4], "Example Health")
		score := 20
		if mentioned {
			score = 80
		}
		evals = append(evals, map[string]any{"response_id": "R" + m[1], "mentioned": mentioned, "recommended": mentioned,
			"sentiment": "neutral", "accuracy": "accurate", "score": score})
	}
	b, _ := json.Marshal(map[string]any{"evaluations": evals})
	return string(b)
}

// Three runs plus a web-search pass: every answer is kept and graded, rates
// cover all runs, and the searched answers are scored on their own.
func TestLLMTest_RepeatedRunsAndWebSearch(t *testing.T) {
	db := testMongoDB(t)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "grok")

	var mu sync.Mutex
	calls := map[string]int{}
	tp := newTestProvider()
	tp.id, tp.name = "anthropic", "Anthropic"
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if isEvalPrompt(prompt) {
			return gradeByResponseID(prompt), nil
		}
		mu.Lock()
		calls[prompt]++
		k := calls[prompt]
		mu.Unlock()
		// Query 0 names the brand in two of its three runs, query 1 never.
		if strings.HasSuffix(prompt, "0?") && k <= 2 {
			return "Example Health is a good option.", nil
		}
		return "Try another service.", nil
	}
	searcher := &searchingProvider{TestProvider: tp, searchFn: func(prompt string) (webSearchAnswer, error) {
		return webSearchAnswer{Text: "Example Health is listed by reviewers.", Sources: []string{"https://www.example.com/pricing", "https://reviews.test/best"}}, nil
	}}
	plain := newTestProvider() // can't search
	plain.id, plain.name = "grok", "Grok"
	plain.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "Example Health exists.", nil
	}
	original := providers
	providers = map[string]LLMProvider{"anthropic": searcher, "grok": plain}
	t.Cleanup(func() { providers = original })

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "grok"}, "queries": testQueries(2),
		"runs": 3, "web_search": true,
	})
	if result == nil {
		t.Fatalf("expected done; events:\n%s", statusMessages(events))
	}
	if result.Runs != 3 || !result.WebSearch || result.RubricVersion != scoringRubricVersion {
		t.Errorf("runs %d, web search %v, rubric %d", result.Runs, result.WebSearch, result.RubricVersion)
	}
	for qi, qr := range result.Results {
		if len(qr.ProviderResults) != 7 { // Anthropic 3 runs + 1 search, Grok 3 runs
			t.Errorf("query %d: %d answers, want 7", qi, len(qr.ProviderResults))
		}
		if first := qr.ProviderResults[0]; first.ProviderID != "anthropic" || first.Search || first.Run != 1 {
			t.Errorf("query %d: answers out of order: %+v", qi, first)
		}
	}
	var a, g *LLMTestSummary
	for i := range result.ProviderSummaries {
		switch result.ProviderSummaries[i].ProviderID {
		case "anthropic":
			a = &result.ProviderSummaries[i]
		case "grok":
			g = &result.ProviderSummaries[i]
		}
	}
	if a == nil || g == nil {
		t.Fatalf("summaries = %+v", result.ProviderSummaries)
	}
	// Anthropic remembered: 2 of 6 answers name the brand
	if a.Answered != 6 || a.MentionRate != 33 || a.Spread == nil || a.Spread.Consistency != 50 {
		t.Errorf("anthropic = %+v spread %+v", a, a.Spread)
	}
	if a.Search == nil || a.Search.Answered != 2 || a.Search.MentionRate != 100 || a.Search.OwnSiteRate != 100 {
		t.Errorf("anthropic search = %+v", a.Search)
	}
	if g.Search != nil || g.MentionRate != 100 || g.Spread == nil || g.Spread.Consistency != 100 {
		t.Errorf("grok = %+v", g)
	}
	if result.SearchScore == nil || *result.SearchScore != 80 {
		t.Errorf("search score = %v", result.SearchScore)
	}
	if len(result.TopCitedDomains) != 2 || !result.TopCitedDomains[0].Own && !result.TopCitedDomains[1].Own {
		t.Errorf("cited = %+v", result.TopCitedDomains)
	}
	if !strings.Contains(statusMessages(events), "Grok can't answer with web search") {
		t.Errorf("expected a note that Grok can't search; events:\n%s", statusMessages(events))
	}
}

// The report shows repeated runs and the web-search pass, with each
// assistant's runs combined into one entry per query.
func TestRenderReportHTML_TestRunsAndSearch(t *testing.T) {
	d := fixtureReportData()
	search := 71
	d.LLMTest.Runs, d.LLMTest.ScoreRange, d.LLMTest.SearchScore = 3, []int{45, 58}, &search
	d.LLMTest.TopCitedDomains = []LLMTestCitedDomain{{Domain: "reviews.test", Answers: 4}, {Domain: "example.com", Answers: 2, Own: true}}
	d.LLMTest.ProviderSummaries[0].Spread = &LLMTestSpread{MentionLow: 60, MentionHigh: 80, RecommendLow: 40, RecommendHigh: 40, Consistency: 75}
	d.LLMTest.ProviderSummaries[0].Search = &LLMTestSearchSummary{OverallScore: 71, MentionRate: 90, RecommendRate: 50, OwnSiteRate: 30, Answered: 10}
	d.LLMTest.Results = []LLMTestQueryResult{{Query: LLMTestQuery{Query: "Best online clinic?"}, ProviderResults: []LLMTestProviderResult{
		{ProviderID: "openai", ProviderName: "OpenAI", Run: 1, Score: 70, Mentioned: true},
		{ProviderID: "openai", ProviderName: "OpenAI", Run: 2, Score: 50, Mentioned: true},
		{ProviderID: "openai", ProviderName: "OpenAI", Run: 3, Score: 30},
		{ProviderID: "openai", ProviderName: "OpenAI", Run: 1, Search: true, Score: 90, Mentioned: true, Recommended: true},
	}}}
	for _, mode := range []string{"pdf", "web"} {
		html, err := renderReportHTML(buildReportView(d, time.Now(), mode))
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"<no value>", "ZgotmplZ"} {
			if strings.Contains(html, bad) {
				t.Errorf("%s: output contains %q", mode, bad)
			}
		}
		for _, want := range []string{
			"asked 3 times", "ranged 45–58", "60–80%", "75%", "With web search", "Searched", ">71<",
			"reviews.test", "Your site", "OpenAI 50 · mentions 2/3", "OpenAI (web) 90 · recommends",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: output lacks %q", mode, want)
			}
		}
	}
	if _, engine, err := generateReportPDF(context.Background(), d); err != nil || engine == "" {
		t.Errorf("PDF: engine %q, err %v", engine, err)
	}
}
