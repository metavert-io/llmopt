package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A trimmed Gemini 429 for a model the free tier doesn't cover.
const geminiNoFreeQuota = `{"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details. * Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_requests, limit: 0, model: gemini-3.6-pro","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"41s"}]}}`

// A per-minute limit on a model the key can use.
const geminiPerMinute = `{"error":{"code":429,"message":"You exceeded your current quota. * Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_requests, limit: 10, model: gemini-3.5-flash","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"quotaId":"GenerateRequestsPerMinutePerProjectPerModel-FreeTier"}]},{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"7.5s"}]}}`

func TestGeminiQuotaError(t *testing.T) {
	qe := geminiQuotaError([]byte(geminiNoFreeQuota))
	if !qe.PerModel || qe.RetryAfter != 41*time.Second || classifyProviderError(qe) != failModel {
		t.Errorf("no-free-quota: %+v class=%v", qe, classifyProviderError(qe))
	}
	if !strings.Contains(providerFailureReason(qe), "limit: 0") {
		t.Errorf("reason drops Google's message: %q", providerFailureReason(qe))
	}
	qe = geminiQuotaError([]byte(geminiPerMinute))
	if qe.PerModel || qe.RetryAfter != 7500*time.Millisecond || classifyProviderError(qe) != failRetry {
		t.Errorf("per-minute: %+v class=%v", qe, classifyProviderError(qe))
	}
	daily := strings.Replace(geminiPerMinute, "PerMinute", "PerDay", 1)
	if qe = geminiQuotaError([]byte(daily)); !qe.PerModel {
		t.Errorf("daily quota should move to another model: %+v", qe)
	}
}

// Gemini's Call returns the quota error (not a bare "overloaded"), and the key
// check treats a 429 as a working key.
func TestGeminiCallAndVerify_Quota429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.Write([]byte(`{"models":[{"name":"models/gemini-3.5-flash-lite","supportedGenerationMethods":["generateContent"]}]}`))
			return
		}
		w.WriteHeader(429)
		w.Write([]byte(geminiNoFreeQuota))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	_, err := (&GeminiProvider{}).Call(context.Background(), "k", "gemini-3.6-pro", "q", 100)
	if classifyProviderError(err) != failModel {
		t.Errorf("Call error %v classified %v", err, classifyProviderError(err))
	}
	status, err := (&GeminiProvider{}).VerifyKey(context.Background(), "k-verify")
	if status != "active" || err != nil {
		t.Errorf("VerifyKey = %q, %v", status, err)
	}
}

// When the default model has no quota, the test moves to the next model and
// the provider still answers.
func TestLLMTest_GeminiNoQuotaModelSwitches(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	insertTestTenantKey(t, db, "gemini")

	const n = 2
	gem := newTestProvider()
	gem.id, gem.name = "gemini", "Gemini"
	gem.models = []ModelDef{{ID: "gemini-3.6-flash", Name: "Gemini 3.6 Flash"}, {ID: "gemini-3.5-flash", Name: "Gemini 3.5 Flash"}}
	gem.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if model == "gemini-3.6-flash" {
			return "", geminiQuotaError([]byte(geminiNoFreeQuota))
		}
		if isEvalPrompt(prompt) {
			return evalJSONFor(n, "gemini"), nil
		}
		return "Example Health is a good option.", nil
	}
	withTestProviders(t, gem)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"gemini"}, "queries": testQueries(n),
	})
	if result == nil {
		t.Fatalf("no result:\n%s", statusMessages(events))
	}
	if len(result.UnavailableProviders) != 0 || result.ProviderSummaries[0].Answered != n {
		t.Errorf("summaries %+v unavailable %+v", result.ProviderSummaries, result.UnavailableProviders)
	}
	if !strings.Contains(statusMessages(events), "switching to Gemini 3.5 Flash") {
		t.Errorf("no switch notice:\n%s", statusMessages(events))
	}
}

// A Gemini 429 for depleted prepaid credits ends the key's use at once instead
// of being retried as a per-minute limit.
func TestGeminiCall_DepletedCreditsIsAKeyFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"code":429,"message":"Your prepayment credits are depleted. Please go to AI Studio to manage your project and billing.","status":"RESOURCE_EXHAUSTED"}}`))
	}))
	defer server.Close()
	withAllProviderBases(t, server)
	_, err := (&GeminiProvider{}).Call(context.Background(), "k", "gemini-4-hydrogen", "q", 100)
	if c := classifyProviderError(err); c != failKey {
		t.Errorf("classified %v, want failKey (err %v)", c, err)
	}
	if r := providerFailureReason(err); !strings.Contains(r, "out of credits") {
		t.Errorf("reason %q", r)
	}
}
