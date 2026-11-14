package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// insertTestTenantKey stores an encrypted provider key for the test tenant.
func insertTestTenantKey(t *testing.T, db *MongoDB, provider string) {
	t.Helper()
	enc, err := encryptSecret("key-for-"+provider, testEncKey())
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.TenantAPIKeys().InsertOne(context.Background(), TenantAPIKey{
		TenantID: "test-tenant", Provider: provider, EncryptedKey: enc, Status: "active",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

// evalJSONFor scores every query for each of the given provider IDs.
func evalJSONFor(numQueries int, providerIDs ...string) string {
	var evals []map[string]any
	for qi := 0; qi < numQueries; qi++ {
		for _, pid := range providerIDs {
			evals = append(evals, map[string]any{
				"query_index": qi, "provider_id": pid, "mentioned": true, "recommended": true,
				"sentiment": "positive", "accuracy": "accurate", "score": 70,
			})
		}
	}
	b, _ := json.Marshal(map[string]any{"evaluations": evals})
	return string(b)
}

func isEvalPrompt(prompt string) bool {
	return strings.Contains(prompt, "You are evaluating how well different AI assistants")
}

func testQueries(n int) []map[string]any {
	var qs []map[string]any
	for i := 0; i < n; i++ {
		qs = append(qs, map[string]any{"query": fmt.Sprintf("best telehealth option %d?", i), "type": "category", "priority": "high"})
	}
	return qs
}

func runLLMTestRequest(t *testing.T, db *MongoDB, saasMode bool, body map[string]any) ([]SSEEvent, *LLMTest) {
	t.Helper()
	handler := handleLLMTest(db, testEncKey(), "fallback-key", saasMode)
	w := httptest.NewRecorder()
	handler(w, testRequest(t, "POST", "/api/test", body))
	events := parseSSEResponse(t, w.Body.Bytes())
	done := findSSEEvent(events, "done")
	if done == nil {
		return events, nil
	}
	var payload struct {
		Result string `json:"result"`
	}
	json.Unmarshal(done.Data, &payload)
	var result LLMTest
	if err := json.Unmarshal([]byte(payload.Result), &result); err != nil {
		t.Fatalf("bad done payload: %v", err)
	}
	return events, &result
}

func statusMessages(events []SSEEvent) string {
	var msgs []string
	for _, e := range events {
		var m struct {
			Message string `json:"message"`
		}
		json.Unmarshal(e.Data, &m)
		msgs = append(msgs, e.Type+": "+m.Message)
	}
	return strings.Join(msgs, "\n")
}

func shortRetries(t *testing.T) {
	orig := providerRetryDelays
	providerRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { providerRetryDelays = orig })
}

// A provider whose key is rejected is dropped after its first failures; the
// others still answer and the test completes without it in the scores.
func TestLLMTest_DeadProviderIsSkippedOthersComplete(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "grok")

	const n = 12
	good := newTestProvider()
	good.id, good.name = "anthropic", "Anthropic"
	good.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if isEvalPrompt(prompt) {
			return evalJSONFor(n, "anthropic"), nil
		}
		return "Example Health is a good option.", nil
	}
	var grokCalls int32
	bad := newTestProvider()
	bad.id, bad.name = "grok", "Grok"
	bad.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		atomic.AddInt32(&grokCalls, 1)
		return "", &ProviderHTTPError{Provider: "Grok", Status: 403, Body: "Your team doesn't have any credits"}
	}
	withTestProviders(t, good, bad)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "grok"}, "queries": testQueries(n),
	})
	if result == nil {
		t.Fatalf("expected done event; events:\n%s", statusMessages(events))
	}
	if c := atomic.LoadInt32(&grokCalls); c >= n {
		t.Errorf("Grok was called %d times; it should be dropped after its key fails", c)
	}
	if len(result.ProviderSummaries) != 1 || result.ProviderSummaries[0].ProviderID != "anthropic" {
		t.Fatalf("summaries = %+v", result.ProviderSummaries)
	}
	if s := result.ProviderSummaries[0]; s.Answered != n || s.OverallScore != 70 || s.MentionRate != 100 {
		t.Errorf("anthropic summary = %+v", s)
	}
	if result.OverallScore != 70 {
		t.Errorf("overall = %d; an unavailable provider must not drag the score down", result.OverallScore)
	}
	if len(result.UnavailableProviders) != 1 || result.UnavailableProviders[0].ProviderID != "grok" ||
		!strings.Contains(result.UnavailableProviders[0].Reason, "credits") {
		t.Errorf("unavailable = %+v", result.UnavailableProviders)
	}
	for _, qr := range result.Results {
		for _, pr := range qr.ProviderResults {
			if pr.ProviderID == "grok" {
				t.Errorf("unavailable provider should not appear in per-query results")
			}
		}
	}
	if !strings.Contains(statusMessages(events), "Grok can't be used") {
		t.Errorf("expected a note that Grok can't be used; events:\n%s", statusMessages(events))
	}
}

// A selected provider with no key is skipped with a note instead of failing the test.
func TestLLMTest_ProviderWithoutKeyIsSkipped(t *testing.T) {
	db := testMongoDB(t)
	insertTestTenantKey(t, db, "anthropic")

	good := newTestProvider()
	good.id, good.name = "anthropic", "Anthropic"
	good.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if isEvalPrompt(prompt) {
			return evalJSONFor(2, "anthropic"), nil
		}
		return "answer", nil
	}
	gem := newTestProvider()
	gem.id, gem.name = "gemini", "Gemini"
	gem.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		t.Error("a provider without a key must not be called")
		return "", nil
	}
	withTestProviders(t, good, gem)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "gemini"}, "queries": testQueries(2),
	})
	if result == nil {
		t.Fatalf("expected done; events:\n%s", statusMessages(events))
	}
	if len(result.UnavailableProviders) != 1 || result.UnavailableProviders[0].ProviderID != "gemini" {
		t.Errorf("unavailable = %+v", result.UnavailableProviders)
	}
	if !strings.Contains(statusMessages(events), "Skipping Gemini") {
		t.Errorf("expected a skip note; events:\n%s", statusMessages(events))
	}
}

// A retired model makes the run move to the provider's next model.
func TestLLMTest_RetiredModelFallsBackToNext(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id, tp.name = "anthropic", "Anthropic"
	tp.models = []ModelDef{{ID: "old-model", Name: "Old"}, {ID: "new-model", Name: "New"}}
	var oldCalls int32
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if model == "old-model" {
			atomic.AddInt32(&oldCalls, 1)
			return "", &ProviderHTTPError{Provider: "Claude", Status: 404, Body: `{"error":{"type":"not_found_error","message":"model: old-model"}}`}
		}
		if isEvalPrompt(prompt) {
			return evalJSONFor(4, "anthropic"), nil
		}
		return "answer", nil
	}
	withTestProviders(t, tp)

	events, result := runLLMTestRequest(t, db, false, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic"}, "queries": testQueries(4),
	})
	if result == nil {
		t.Fatalf("expected done; events:\n%s", statusMessages(events))
	}
	if s := result.ProviderSummaries[0]; s.Model != "New" || s.Answered != 4 {
		t.Errorf("summary = %+v", s)
	}
	if !strings.Contains(statusMessages(events), "switching to New") {
		t.Errorf("expected a switch note; events:\n%s", statusMessages(events))
	}
}

// When the primary provider can't evaluate, another provider that answered does.
func TestLLMTest_EvaluationFallsBackToAnotherProvider(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "openai")

	primary := newTestProvider()
	primary.id, primary.name = "anthropic", "Anthropic"
	primary.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if isEvalPrompt(prompt) {
			return "", &ProviderHTTPError{Provider: "Claude", Status: 500, Body: "internal error"}
		}
		return "answer", nil
	}
	other := newTestProvider()
	other.id, other.name = "openai", "OpenAI"
	other.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if isEvalPrompt(prompt) {
			return "Here you go:\n```json\n" + evalJSONFor(2, "anthropic", "OpenAI") + "\n```", nil
		}
		return "answer", nil
	}
	withTestProviders(t, primary, other)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "openai"}, "queries": testQueries(2),
	})
	if result == nil {
		t.Fatalf("expected done; events:\n%s", statusMessages(events))
	}
	if len(result.ProviderSummaries) != 2 {
		t.Fatalf("summaries = %+v", result.ProviderSummaries)
	}
	for _, s := range result.ProviderSummaries {
		// The evaluator named OpenAI by display name; it must still be matched.
		if s.OverallScore != 70 {
			t.Errorf("%s score = %d", s.ProviderID, s.OverallScore)
		}
	}
}

// If no provider can answer, the test fails with each provider's reason.
func TestLLMTest_AllProvidersFail(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id, tp.name = "anthropic", "Anthropic"
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "", &ProviderHTTPError{Provider: "Claude", Status: 401, Body: "invalid x-api-key"}
	}
	withTestProviders(t, tp)

	events, result := runLLMTestRequest(t, db, false, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic"}, "queries": testQueries(3),
	})
	if result != nil {
		t.Fatal("expected no result")
	}
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil || !strings.Contains(string(errEvent.Data), "No provider could answer") {
		t.Errorf("events:\n%s", statusMessages(events))
	}
}

// A requested model the key can't use is replaced by the best available one.
func TestLLMTest_UnknownRequestedModelUsesBest(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id, tp.name = "anthropic", "Anthropic"
	tp.models = []ModelDef{{ID: "best", Name: "Best"}, {ID: "other", Name: "Other"}}
	var usedModel atomic.Value
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if isEvalPrompt(prompt) {
			return evalJSONFor(1, "anthropic"), nil
		}
		usedModel.Store(model)
		return "answer", nil
	}
	withTestProviders(t, tp)

	events, result := runLLMTestRequest(t, db, false, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic"}, "queries": testQueries(1),
		"models": map[string]string{"anthropic": "retired-model"},
	})
	if result == nil {
		t.Fatalf("expected done; events:\n%s", statusMessages(events))
	}
	if usedModel.Load() != "best" {
		t.Errorf("used %v, want best", usedModel.Load())
	}

	// A requested model that is offered is honoured.
	_, result = runLLMTestRequest(t, db, false, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic"}, "queries": testQueries(1),
		"models": map[string]string{"anthropic": "other"},
	})
	if result == nil || usedModel.Load() != "other" {
		t.Errorf("used %v, want other", usedModel.Load())
	}
}
