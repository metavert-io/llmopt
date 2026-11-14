package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func resetModelCache(t *testing.T) {
	t.Helper()
	modelCache.Lock()
	modelCache.m = map[string]modelCacheEntry{}
	modelCache.Unlock()
	t.Cleanup(func() {
		modelCache.Lock()
		modelCache.m = map[string]modelCacheEntry{}
		modelCache.Unlock()
	})
}

func modelIDs(ms []ModelDef) []string {
	ids := make([]string, len(ms))
	for i, m := range ms {
		ids[i] = m.ID
	}
	return ids
}

func assertIDs(t *testing.T, got []ModelDef, want []string) {
	t.Helper()
	g := modelIDs(got)
	if strings.Join(g, ",") != strings.Join(want, ",") {
		t.Errorf("models:\n got  %v\n want %v", g, want)
	}
}

func TestClassifyProviderError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want providerFailure
	}{
		{"nil", nil, failOther},
		{"overloaded", ErrOverloaded, failRetry},
		{"plain error", errors.New("request failed: timeout"), failOther},
		{"401", &ProviderHTTPError{"Grok", 401, "unauthorized"}, failKey},
		{"anthropic low credit", &ProviderHTTPError{"Claude", 400, `{"error":{"message":"Your credit balance is too low"}}`}, failKey},
		{"openai quota", &ProviderHTTPError{"OpenAI", 429, `{"error":{"code":"insufficient_quota"}}`}, failKey},
		{"xai no credits", &ProviderHTTPError{"Grok", 403, "Your team doesn't have any credits"}, failKey},
		{"gemini bad key", &ProviderHTTPError{"Gemini", 400, `{"error":{"status":"INVALID_ARGUMENT","details":[{"reason":"API_KEY_INVALID"}]}}`}, failKey},
		{"404 model", &ProviderHTTPError{"Gemini", 404, "models/gemini-1.0-pro is not found for API version v1beta"}, failModel},
		{"400 model gone", &ProviderHTTPError{"Grok", 400, "The model grok-beta was deprecated"}, failModel},
		{"openai model access", &ProviderHTTPError{"OpenAI", 403, "Your organization must be verified to use the model `o3`"}, failModel},
		{"403 other", &ProviderHTTPError{"Gemini", 403, "PERMISSION_DENIED"}, failKey},
		{"500", &ProviderHTTPError{"OpenAI", 500, "server error"}, failRetry},
		{"400 other", &ProviderHTTPError{"OpenAI", 400, "bad request"}, failOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyProviderError(c.err); got != c.want {
				t.Errorf("classify(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestProviderHTTPErrorKeepsMessageShape(t *testing.T) {
	err := &ProviderHTTPError{Provider: "Grok", Status: 403, Body: "nope"}
	if err.Error() != "Grok API error (403): nope" {
		t.Errorf("got %q", err.Error())
	}
}

func TestVersionAfter(t *testing.T) {
	cases := map[string]float64{
		"4-1-fast-reasoning": 4.1,
		"4-0709":             4,
		"4":                  4,
		"3-mini":             3,
		"4.1-fast":           4.1,
		"4-5-20251101":       4.5,
		"4-20250514":         4,
		"3-5-sonnet":         3.5,
		"2.5-flash-lite":     2.5,
		"3.1-pro-preview":    3.1,
		"pro-latest":         0,
	}
	for in, want := range cases {
		if got := versionAfter(in, ""); got != want {
			t.Errorf("versionAfter(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestPrettyModelName(t *testing.T) {
	cases := []struct {
		id      string
		version float64
		want    string
	}{
		{"gpt-5.1-chat-latest", 5.1, "GPT-5.1 Chat Latest"},
		{"gpt-5-mini", 5, "GPT-5 Mini"},
		{"gpt-4o", 4, "GPT-4o"},
		{"o4-mini", 4.5, "o4 Mini"},
		{"o3", 4.4, "o3"},
		{"grok-4-1-fast-reasoning", 4.1, "Grok 4.1 Fast Reasoning"},
		{"grok-4-0709", 4, "Grok 4 0709"},
		{"grok-3-mini", 3, "Grok 3 Mini"},
		{"grok-4", 4, "Grok 4"},
	}
	for _, c := range cases {
		if got := prettyModelName(c.id, c.version); got != c.want {
			t.Errorf("prettyModelName(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestOpenAIListModels_FiltersAndRanks(t *testing.T) {
	ids := []string{
		"gpt-5.2", "gpt-5.2-2025-12-11", "gpt-5-mini", "gpt-5-nano", "gpt-4o", "gpt-4o-mini", "gpt-4.1",
		"gpt-4o-audio-preview", "gpt-realtime", "text-embedding-3-large", "o3", "o4-mini", "o3-pro",
		"gpt-5.1-chat-latest", "whisper-1", "dall-e-3", "gpt-5-pro", "gpt-image-1", "gpt-3.5-turbo",
		"gpt-4-turbo", "gpt-4", "chatgpt-4o-latest", "codex-mini-latest", "gpt-5.1-codex", "o3-deep-research",
		"gpt-4o-mini-search-preview", "omni-moderation-latest", "gpt-4o-2024-08-06",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("unexpected request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var data []map[string]any
		for i, id := range ids {
			data = append(data, map[string]any{"id": id, "created": 1700000000 + i, "object": "model"})
		}
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	got, err := (&OpenAIProvider{}).ListModels(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, []string{"gpt-5.2", "gpt-5.1-chat-latest", "gpt-5-mini", "gpt-5-nano", "o4-mini", "o3", "gpt-4.1", "gpt-4o", "gpt-4o-mini"})
	if got[0].Name != "GPT-5.2" {
		t.Errorf("name = %q", got[0].Name)
	}
}

func TestGeminiListModels_FiltersRanksAndUsesHeader(t *testing.T) {
	type gm struct {
		name    string
		methods []string
	}
	gen := []string{"generateContent", "countTokens"}
	models := []gm{
		{"models/gemini-2.5-pro", gen},
		{"models/gemini-2.5-flash", gen},
		{"models/gemini-2.5-flash-lite", gen},
		{"models/gemini-3.1-pro-preview", gen},
		{"models/gemini-3-flash-preview", gen},
		{"models/gemini-2.5-flash-preview-09-2025", gen},
		{"models/gemini-2.0-flash-001", gen},
		{"models/gemini-embedding-001", []string{"embedContent"}},
		{"models/gemini-2.5-flash-image", gen},
		{"models/gemini-flash-latest", gen},
		{"models/gemini-2.5-flash-preview-tts", gen},
		{"models/gemma-3-27b-it", gen},
	}
	var pages int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "" {
			t.Error("API key must not be sent in the URL")
		}
		if r.Header.Get("x-goog-api-key") != "gk" {
			t.Errorf("missing x-goog-api-key header")
		}
		// Two pages, to exercise pagination.
		half := len(models) / 2
		var slice []gm
		next := ""
		if r.URL.Query().Get("pageToken") == "" {
			slice, next = models[:half], "p2"
		} else {
			slice = models[half:]
		}
		atomic.AddInt32(&pages, 1)
		var out []map[string]any
		for _, m := range slice {
			out = append(out, map[string]any{"name": m.name, "displayName": strings.TrimPrefix(m.name, "models/") + " display", "supportedGenerationMethods": m.methods})
		}
		json.NewEncoder(w).Encode(map[string]any{"models": out, "nextPageToken": next})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	got, err := (&GeminiProvider{}).ListModels(context.Background(), "gk")
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Errorf("fetched %d pages, want 2", pages)
	}
	assertIDs(t, got, []string{"gemini-3.1-pro-preview", "gemini-3-flash-preview", "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite"})
	if got[0].Name != "gemini-3.1-pro-preview display" {
		t.Errorf("display name not used: %q", got[0].Name)
	}
}

func TestGrokListModels_FiltersAndRanks(t *testing.T) {
	ids := []string{"grok-4-1-fast-reasoning", "grok-4-1-fast-non-reasoning", "grok-4-0709", "grok-4-fast-reasoning",
		"grok-3", "grok-3-mini", "grok-code-fast-1", "grok-2-image-1212", "grok-2-vision-1212", "grok-imagine-video", "grok-2-1212"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data []map[string]any
		for _, id := range ids {
			data = append(data, map[string]any{"id": id, "created": 1700000000, "object": "model"})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	got, err := (&GrokProvider{}).ListModels(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, []string{"grok-4-1-fast-non-reasoning", "grok-4-1-fast-reasoning", "grok-4-0709", "grok-4-fast-reasoning", "grok-3", "grok-3-mini"})
	if got[2].Name != "Grok 4 0709" {
		t.Errorf("name = %q", got[2].Name)
	}
}

func TestAnthropicListModels_PaginatesAndRanks(t *testing.T) {
	page1 := []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5", "claude-fable-5-2"}
	page2 := []string{"claude-sonnet-4-6", "claude-opus-4-5-20251101", "claude-haiku-4-5-20251001", "claude-3-5-haiku-20241022"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "ak" || r.Header.Get("anthropic-version") == "" {
			t.Error("missing Anthropic auth headers")
		}
		ids, more, last := page1, true, "claude-fable-5-2"
		if r.URL.Query().Get("after_id") != "" {
			ids, more, last = page2, false, ""
		}
		var data []map[string]any
		for _, id := range ids {
			data = append(data, map[string]any{"id": id, "display_name": "Name of " + id, "created_at": "2026-01-01T00:00:00Z", "type": "model"})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data, "has_more": more, "last_id": last})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	got, err := (&AnthropicProvider{}).ListModels(context.Background(), "ak")
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "claude-fable-5-2",
		"claude-sonnet-4-6", "claude-opus-4-5-20251101", "claude-haiku-4-5-20251001", "claude-3-5-haiku-20241022"})
	if got[0].Name != "Name of claude-opus-5-5" {
		t.Errorf("display name not used: %q", got[0].Name)
	}
	if small := smallestModel("anthropic", got); small != "claude-haiku-5-5" {
		t.Errorf("smallest = %q", small)
	}
}

// listingProvider is a TestProvider that can also list models.
type listingProvider struct {
	*TestProvider
	listCalls int32
	listFn    func() ([]ModelDef, error)
}

func (p *listingProvider) ListModels(ctx context.Context, apiKey string) ([]ModelDef, error) {
	atomic.AddInt32(&p.listCalls, 1)
	return p.listFn()
}

func TestAvailableModels_CachesAndFallsBack(t *testing.T) {
	resetModelCache(t)
	tp := newTestProvider()
	lp := &listingProvider{TestProvider: tp, listFn: func() ([]ModelDef, error) {
		return []ModelDef{{ID: "live-best", Name: "Live Best"}}, nil
	}}

	got, live := availableModels(context.Background(), lp, "key1")
	if !live || got[0].ID != "live-best" {
		t.Fatalf("got %v live=%v", got, live)
	}
	availableModels(context.Background(), lp, "key1")
	if lp.listCalls != 1 {
		t.Errorf("list called %d times, want 1 (cached)", lp.listCalls)
	}

	// A different key is listed separately; a failure falls back to the built-in list.
	lp.listFn = func() ([]ModelDef, error) { return nil, errors.New("boom") }
	got, live = availableModels(context.Background(), lp, "key2")
	if live || got[0].ID != "test-model" {
		t.Errorf("fallback: got %v live=%v", got, live)
	}

	// The failure is remembered briefly: no second call for the same bad key.
	callsAfterFailure := lp.listCalls
	availableModels(context.Background(), lp, "key2")
	if lp.listCalls != callsAfterFailure {
		t.Errorf("a failed listing should not be retried right away")
	}

	// No key: built-in list without asking.
	before := lp.listCalls
	got, live = availableModels(context.Background(), lp, "")
	if live || got[0].ID != "test-model" || lp.listCalls != before {
		t.Errorf("no key: got %v live=%v calls=%d", got, live, lp.listCalls)
	}
}

func TestPickEvalModel(t *testing.T) {
	resetModelCache(t)
	tp := newTestProvider()
	tp.models = []ModelDef{{ID: "usual", Name: "Usual"}}
	lp := &listingProvider{TestProvider: tp, listFn: func() ([]ModelDef, error) {
		return []ModelDef{{ID: "newer", Name: "Newer"}, {ID: "usual", Name: "Usual"}, {ID: "pref", Name: "Pref"}}, nil
	}}
	if got := pickEvalModel(context.Background(), lp, "k", "pref"); got != "pref" {
		t.Errorf("preferred: %q", got)
	}
	if got := pickEvalModel(context.Background(), lp, "k", "gone"); got != "usual" {
		t.Errorf("usual default: %q", got)
	}
	tp.models = []ModelDef{{ID: "retired", Name: "Retired"}}
	if got := pickEvalModel(context.Background(), lp, "k", ""); got != "newer" {
		t.Errorf("retired default should fall to best live model, got %q", got)
	}
}

func TestOpenAICall_UsesResponsesAPIBody(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &body)
		fmt.Fprint(w, `{"output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"hi"}]}]}`)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	got, err := (&OpenAIProvider{}).Call(context.Background(), "k", "gpt-5.2", "hello", 4096)
	if err != nil || got != "hi" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, ok := body["messages"]; ok {
		t.Error("Responses API body must not contain messages")
	}
	if _, ok := body["max_tokens"]; ok {
		t.Error("Responses API body must not contain max_tokens")
	}
	if body["input"] == nil {
		t.Error("missing input")
	}
	if int(body["max_output_tokens"].(float64)) != 4096+reasoningHeadroom {
		t.Errorf("max_output_tokens = %v for a reasoning model", body["max_output_tokens"])
	}

	(&OpenAIProvider{}).Call(context.Background(), "k", "gpt-4o", "hello", 4096)
	if int(body["max_output_tokens"].(float64)) != 4096 {
		t.Errorf("max_output_tokens = %v for a non-reasoning model", body["max_output_tokens"])
	}

	sb, _ := (&OpenAIProvider{}).BuildStreamBody("gpt-4o", 1000, "p", false)
	if strings.Contains(string(sb), `"messages"`) || !strings.Contains(string(sb), `"input"`) {
		t.Errorf("stream body uses the wrong shape: %s", sb)
	}
}

func TestOpenAICall_InsufficientQuotaIsNotOverload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":{"message":"You exceeded your current quota","code":"insufficient_quota"}}`)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	_, err := (&OpenAIProvider{}).Call(context.Background(), "k", "gpt-4o", "hello", 100)
	if errors.Is(err, ErrOverloaded) || classifyProviderError(err) != failKey {
		t.Errorf("want a key/billing failure, got %v", err)
	}
}

func TestGeminiCall_KeyInHeaderAndJoinsParts(t *testing.T) {
	var gotMax float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "" || r.Header.Get("x-goog-api-key") != "gk" {
			t.Error("Gemini key must be sent as a header only")
		}
		var b struct {
			GenerationConfig struct {
				MaxOutputTokens float64 `json:"maxOutputTokens"`
			} `json:"generationConfig"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		gotMax = b.GenerationConfig.MaxOutputTokens
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"thinking...","thought":true},{"text":"Hello "},{"text":"world"}]}}]}`)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	got, err := (&GeminiProvider{}).Call(context.Background(), "gk", "gemini-3-pro-preview", "p", 4096)
	if err != nil || got != "Hello world" {
		t.Fatalf("got %q, %v", got, err)
	}
	if int(gotMax) != 4096+reasoningHeadroom {
		t.Errorf("maxOutputTokens = %v for a thinking model", gotMax)
	}
}

func TestVerifyKey_UsesSmallestListedModel(t *testing.T) {
	resetModelCache(t)
	var verifyPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{
				{"name": "models/gemini-3-pro-preview", "supportedGenerationMethods": []string{"generateContent"}},
				{"name": "models/gemini-3-flash-lite", "supportedGenerationMethods": []string{"generateContent"}},
			}})
			return
		}
		verifyPath = r.URL.Path
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	status, err := (&GeminiProvider{}).VerifyKey(context.Background(), "gk")
	if status != "active" || err != nil {
		t.Fatalf("status %q err %v", status, err)
	}
	if !strings.Contains(verifyPath, "gemini-3-flash-lite") {
		t.Errorf("verified with %q, want the smallest listed model", verifyPath)
	}
	// The listing is cached for the LLM test's model picker.
	if models, live := availableModels(context.Background(), &GeminiProvider{}, "gk"); !live || len(models) != 2 {
		t.Errorf("listing not cached: %v live=%v", models, live)
	}
}

func TestVerifyKey_ListingRejectsKey(t *testing.T) {
	resetModelCache(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":"invalid api key"}`)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	for _, p := range []LLMProvider{&OpenAIProvider{}, &GrokProvider{}, &AnthropicProvider{}, &GeminiProvider{}} {
		if status, _ := p.VerifyKey(context.Background(), "bad"); status != "invalid" {
			t.Errorf("%s: status %q, want invalid", p.Name(), status)
		}
	}
}

func TestCallWithRetry_RetriesOverload(t *testing.T) {
	orig := providerRetryDelays
	providerRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { providerRetryDelays = orig })

	tp := newTestProvider()
	calls := 0
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		calls++
		if calls < 3 {
			return "", ErrOverloaded
		}
		return "ok", nil
	}
	got, err := callWithRetry(context.Background(), tp, "k", "m", "p", 10)
	if err != nil || got != "ok" || calls != 3 {
		t.Errorf("got %q err %v after %d calls", got, err, calls)
	}

	calls = 0
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		calls++
		return "", &ProviderHTTPError{"Test", 401, "bad key"}
	}
	if _, err := callWithRetry(context.Background(), tp, "k", "m", "p", 10); err == nil || calls != 1 {
		t.Errorf("a bad key must not be retried: calls=%d", calls)
	}
}
