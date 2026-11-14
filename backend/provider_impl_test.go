package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── Provider base URL test helper ─────────────────────────────────────────

// withAllProviderBases sets all provider base URLs to server.URL and restores on cleanup.
func withAllProviderBases(t *testing.T, server *httptest.Server) {
	t.Helper()
	origAnthropic := anthropicAPIBase
	origGemini := geminiAPIBase
	origGrok := grokAPIBase
	origOpenAI := openaiAPIBase

	anthropicAPIBase = server.URL
	geminiAPIBase = server.URL
	grokAPIBase = server.URL
	openaiAPIBase = server.URL

	t.Cleanup(func() {
		anthropicAPIBase = origAnthropic
		geminiAPIBase = origGemini
		grokAPIBase = origGrok
		openaiAPIBase = origOpenAI
	})
}

// ── AnthropicProvider.Call ────────────────────────────────────────────────

func TestAnthropicCall_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "hello world"},
			},
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	result, err := p.Call(context.Background(), "test-key", "claude-haiku-4-5-20251001", "say hi", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestAnthropicCall_Overloaded(t *testing.T) {
	for _, status := range []int{429, 529} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()
			withAllProviderBases(t, server)

			p := &AnthropicProvider{}
			_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
			if err != ErrOverloaded {
				t.Errorf("expected ErrOverloaded, got %v", err)
			}
		})
	}
}

func TestAnthropicCall_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "Claude API error") {
		t.Errorf("expected Claude API error, got %v", err)
	}
}

func TestAnthropicCall_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"content": []any{}})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got %v", err)
	}
}

// ── AnthropicProvider.Stream ───────────────────────────────────────────────

// anthropicSSEBody builds a valid Anthropic SSE stream with the given text.
func anthropicSSEBody(text string) string {
	escapedText, _ := json.Marshal(text)
	lines := []string{
		`data: {"type":"content_block_start","content_block":{"type":"text"}}`,
		``,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":` + string(escapedText) + `}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}
	return strings.Join(lines, "\n")
}

func TestAnthropicStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, anthropicSSEBody(`{"answer":"test"}`))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	body, _ := p.BuildStreamBody("claude-haiku-4-5-20251001", 100, "prompt", false)
	w := httptest.NewRecorder()
	result, err := p.Stream(context.Background(), "key", body, w, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.RawText == "" {
		t.Error("expected non-empty RawText")
	}
}

func TestAnthropicStream_Overloaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(529)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err != ErrOverloaded {
		t.Errorf("expected ErrOverloaded, got %v", err)
	}
}

func TestAnthropicStream_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("server error"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err == nil || !strings.Contains(err.Error(), "Claude API error") {
		t.Errorf("expected Claude API error, got %v", err)
	}
}

func TestAnthropicStream_ErrorEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"error","error":{"type":"api_error","message":"API error occurred"}}`+"\n\n")
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err == nil {
		t.Error("expected error from stream error event")
	}
}

func TestAnthropicStream_OverloadedEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"type":"error","error":{"type":"overloaded_error","message":"overloaded"}}`+"\n\n")
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err != ErrOverloaded {
		t.Errorf("expected ErrOverloaded from overloaded_error event, got %v", err)
	}
}

func TestAnthropicStream_WebSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Simulate web search flow
		fmt.Fprint(w, strings.Join([]string{
			`data: {"type":"content_block_start","content_block":{"type":"server_tool_use","name":"web_search"}}`,
			``,
			`data: {"type":"content_block_start","content_block":{"type":"web_search_tool_result"}}`,
			``,
			`data: {"type":"content_block_start","content_block":{"type":"text"}}`,
			``,
			`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"searched result"}}`,
			``,
			`data: {"type":"message_stop"}`,
			``,
		}, "\n"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", true)
	w := httptest.NewRecorder()
	result, err := p.Stream(context.Background(), "key", body, w, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !strings.Contains(result.RawText, "searched") {
		t.Errorf("expected 'searched' in RawText, got %q", result.RawText)
	}
}

// ── AnthropicProvider.VerifyKey ────────────────────────────────────────────

func TestAnthropicVerifyKey_Active(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	status, err := p.VerifyKey(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected 'active', got %q", status)
	}
}

func TestAnthropicVerifyKey_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	status, _ := p.VerifyKey(context.Background(), "bad-key")
	if status != "invalid" {
		t.Errorf("expected 'invalid', got %q", status)
	}
}

func TestAnthropicVerifyKey_NoCredits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"You have exceeded your credit limit"}}`))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	status, _ := p.VerifyKey(context.Background(), "key")
	if status != "no_credits" {
		t.Errorf("expected 'no_credits', got %q", status)
	}
}

func TestAnthropicVerifyKey_RateLimited_Active(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	status, _ := p.VerifyKey(context.Background(), "key")
	if status != "active" {
		t.Errorf("expected 'active' for plain rate limit, got %q", status)
	}
}

func TestAnthropicVerifyKey_Overloaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(529)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &AnthropicProvider{}
	status, _ := p.VerifyKey(context.Background(), "key")
	if status != "active" {
		t.Errorf("expected 'active' for 529, got %q", status)
	}
}

// ── GeminiProvider.Call ────────────────────────────────────────────────────

func TestGeminiCall_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"content": map[string]any{
					"parts": []map[string]any{{"text": "gemini response"}},
				}},
			},
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	result, err := p.Call(context.Background(), "key", "gemini-2.5-flash", "prompt", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "gemini response" {
		t.Errorf("expected 'gemini response', got %q", result)
	}
}

func TestGeminiCall_Overloaded(t *testing.T) {
	for _, status := range []int{429, 529, 503} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()
			withAllProviderBases(t, server)

			p := &GeminiProvider{}
			_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
			if !errors.Is(err, ErrOverloaded) {
				t.Errorf("expected ErrOverloaded for %d, got %v", status, err)
			}
		})
	}
}

func TestGeminiCall_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "Gemini API error") {
		t.Errorf("expected Gemini API error, got %v", err)
	}
}

func TestGeminiCall_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"candidates": []any{}})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got %v", err)
	}
}

// ── GeminiProvider.Stream ──────────────────────────────────────────────────

func TestGeminiStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"candidates":[{"content":{"parts":[{"text":"gemini stream result"}]}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	body, _ := p.BuildStreamBody("gemini-2.5-flash", 100, "prompt", false)
	w := httptest.NewRecorder()
	result, err := p.Stream(context.Background(), "key", body, w, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !strings.Contains(result.RawText, "gemini stream result") {
		t.Errorf("expected 'gemini stream result' in RawText, got %q", result.RawText)
	}
}

func TestGeminiStream_Overloaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err != ErrOverloaded {
		t.Errorf("expected ErrOverloaded, got %v", err)
	}
}

func TestGeminiStream_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("server error"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err == nil || !strings.Contains(err.Error(), "Gemini API error") {
		t.Errorf("expected Gemini API error, got %v", err)
	}
}

// ── GeminiProvider.VerifyKey ───────────────────────────────────────────────

func TestGeminiVerifyKey_Active(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"content": map[string]any{"parts": []map[string]any{{"text": "ok"}}}},
			},
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	status, err := p.VerifyKey(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected 'active', got %q", status)
	}
}

func TestGeminiVerifyKey_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"message":"API key not valid"}}`))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GeminiProvider{}
	status, _ := p.VerifyKey(context.Background(), "bad-key")
	if status != "invalid" {
		t.Errorf("expected 'invalid', got %q", status)
	}
}

// A quota 429 means the key works (Gemini's quotas are per model and per
// minute); only depleted prepaid credits or billing mean it can't be used.
func TestGeminiVerifyKey_Quota429(t *testing.T) {
	cases := map[string]string{
		`{"error":{"message":"quota exceeded"}}`: "active",
		`{"error":{"message":"Your prepayment credits are depleted. Please go to AI Studio to manage your project and billing."}}`: "no_credits",
	}
	for body, want := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(429)
			w.Write([]byte(body))
		}))
		withAllProviderBases(t, server)
		status, _ := (&GeminiProvider{}).VerifyKey(context.Background(), "key-"+want)
		if status != want {
			t.Errorf("%s: got %q, want %q", body, status, want)
		}
		server.Close()
	}
}

// ── GrokProvider.Call ──────────────────────────────────────────────────────

func TestGrokCall_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "grok response"}},
			},
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	result, err := p.Call(context.Background(), "key", "grok-3-mini", "prompt", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "grok response" {
		t.Errorf("expected 'grok response', got %q", result)
	}
}

func TestGrokCall_Overloaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err != ErrOverloaded {
		t.Errorf("expected ErrOverloaded, got %v", err)
	}
}

func TestGrokCall_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("error"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "Grok API error") {
		t.Errorf("expected Grok API error, got %v", err)
	}
}

func TestGrokCall_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got %v", err)
	}
}

// ── GrokProvider.Stream ────────────────────────────────────────────────────

func grokSSEBody(text string) string {
	escaped, _ := json.Marshal(text)
	return strings.Join([]string{
		`data: {"choices":[{"delta":{"content":` + string(escaped) + `},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
}

func TestGrokStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, grokSSEBody("grok stream result"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	body, _ := p.BuildStreamBody("grok-3-mini", 100, "prompt", false)
	w := httptest.NewRecorder()
	result, err := p.Stream(context.Background(), "key", body, w, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !strings.Contains(result.RawText, "grok stream result") {
		t.Errorf("expected 'grok stream result' in RawText, got %q", result.RawText)
	}
}

func TestGrokStream_Overloaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err != ErrOverloaded {
		t.Errorf("expected ErrOverloaded, got %v", err)
	}
}

func TestGrokStream_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("error"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err == nil || !strings.Contains(err.Error(), "Grok API error") {
		t.Errorf("expected Grok API error, got %v", err)
	}
}

// ── GrokProvider.VerifyKey ─────────────────────────────────────────────────

func TestGrokVerifyKey_Active(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	status, err := p.VerifyKey(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected 'active', got %q", status)
	}
}

func TestGrokVerifyKey_Invalid(t *testing.T) {
	for _, code := range []int{401, 403} {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()
			withAllProviderBases(t, server)

			p := &GrokProvider{}
			status, _ := p.VerifyKey(context.Background(), "bad-key")
			if status != "invalid" {
				t.Errorf("expected 'invalid' for %d, got %q", code, status)
			}
		})
	}
}

func TestGrokVerifyKey_NoCredits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"billing quota exceeded"}}`))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &GrokProvider{}
	status, _ := p.VerifyKey(context.Background(), "key")
	if status != "no_credits" {
		t.Errorf("expected 'no_credits', got %q", status)
	}
}

// ── OpenAIProvider.Call ────────────────────────────────────────────────────

func TestOpenAICall_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"output": []map[string]any{
				{
					"type": "message",
					"content": []map[string]any{
						{"type": "output_text", "text": "openai response"},
					},
				},
			},
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	result, err := p.Call(context.Background(), "key", "gpt-4o", "prompt", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "openai response" {
		t.Errorf("expected 'openai response', got %q", result)
	}
}

func TestOpenAICall_Overloaded(t *testing.T) {
	for _, status := range []int{429, 529} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()
			withAllProviderBases(t, server)

			p := &OpenAIProvider{}
			_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
			if err != ErrOverloaded {
				t.Errorf("expected ErrOverloaded for %d, got %v", status, err)
			}
		})
	}
}

func TestOpenAICall_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("server error"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "OpenAI API error") {
		t.Errorf("expected OpenAI API error, got %v", err)
	}
}

func TestOpenAICall_EmptyOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"output": []any{}})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	_, err := p.Call(context.Background(), "key", "model", "prompt", 100)
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected empty response error, got %v", err)
	}
}

// ── OpenAIProvider.Stream ──────────────────────────────────────────────────

func openaiSSEBody(text string) string {
	escaped, _ := json.Marshal(text)
	return strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":` + string(escaped) + `}`,
		``,
		`data: {"type":"response.completed"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
}

func TestOpenAIStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, openaiSSEBody("openai stream result"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	body, _ := p.BuildStreamBody("gpt-4o", 100, "prompt", false)
	w := httptest.NewRecorder()
	result, err := p.Stream(context.Background(), "key", body, w, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !strings.Contains(result.RawText, "openai stream result") {
		t.Errorf("expected 'openai stream result' in RawText, got %q", result.RawText)
	}
}

func TestOpenAIStream_Overloaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err != ErrOverloaded {
		t.Errorf("expected ErrOverloaded, got %v", err)
	}
}

func TestOpenAIStream_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("error"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	body, _ := p.BuildStreamBody("model", 100, "prompt", false)
	w := httptest.NewRecorder()
	_, err := p.Stream(context.Background(), "key", body, w, w)
	if err == nil || !strings.Contains(err.Error(), "OpenAI API error") {
		t.Errorf("expected OpenAI API error, got %v", err)
	}
}

// ── OpenAIProvider.VerifyKey ───────────────────────────────────────────────

func TestOpenAIVerifyKey_Active(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"output": []map[string]any{
				{"type": "message", "content": []map[string]any{
					{"type": "output_text", "text": "ok"},
				}},
			},
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	status, err := p.VerifyKey(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "active" {
		t.Errorf("expected 'active', got %q", status)
	}
}

func TestOpenAIVerifyKey_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	status, _ := p.VerifyKey(context.Background(), "bad-key")
	if status != "invalid" {
		t.Errorf("expected 'invalid', got %q", status)
	}
}

func TestOpenAIVerifyKey_NoCredits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		// OpenAI's real out-of-credit answer.
		w.Write([]byte(`{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","code":"insufficient_quota"}}`))
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	p := &OpenAIProvider{}
	status, _ := p.VerifyKey(context.Background(), "key")
	if status != "no_credits" {
		t.Errorf("expected 'no_credits', got %q", status)
	}
}

// ── provider.go: wrapWithIdleTimeout ─────────────────────────────────────

func TestWrapWithIdleTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	defer resp.Body.Close()

	wrapped := wrapWithIdleTimeout(resp, 5*time.Second)
	if wrapped == nil {
		t.Fatal("wrapWithIdleTimeout returned nil")
	}

	buf := make([]byte, 5)
	n, _ := wrapped.Read(buf)
	if n == 0 {
		t.Error("expected to read some bytes from wrapped body")
	}
}
