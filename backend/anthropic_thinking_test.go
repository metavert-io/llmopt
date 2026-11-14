package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsAnthropicThinkingModel(t *testing.T) {
	cases := map[string]bool{
		"claude-opus-5-5":           true,
		"claude-opus-5":             true,
		"claude-sonnet-5-5":         true,
		"claude-haiku-5-5":          true,
		"claude-fable-5-2":          true,
		"claude-mythos-5-2":         true,
		"claude-opus-4-8":           false,
		"claude-sonnet-4-6":         false,
		"claude-haiku-4-5-20251001": false,
		"claude-3-5-haiku-20241022": false,
	}
	for id, want := range cases {
		if got := isAnthropicThinkingModel(id); got != want {
			t.Errorf("isAnthropicThinkingModel(%q) = %v, want %v", id, got, want)
		}
	}
}

// Claude 5-generation models return thinking blocks (empty text by default)
// before the answer. The answer must be the text blocks, not the first block.
func TestAnthropicCall_ThinkingBlockFirst(t *testing.T) {
	var gotMaxTokens float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotMaxTokens, _ = body["max_tokens"].(float64)
		if _, ok := body["thinking"]; ok {
			t.Errorf("request should leave thinking unset, got %v", body["thinking"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "thinking", "thinking": "", "signature": "sig"},
				{"type": "text", "text": "Example Health is "},
				{"type": "text", "text": "a well-reviewed option."},
			},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	got, err := (&AnthropicProvider{}).Call(context.Background(), "k", "claude-opus-5-5", "best telehealth?", 4096)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Example Health is a well-reviewed option." {
		t.Errorf("answer = %q", got)
	}
	if int(gotMaxTokens) != 4096+reasoningHeadroom {
		t.Errorf("max_tokens = %v, want %d (answer budget plus thinking headroom)", gotMaxTokens, 4096+reasoningHeadroom)
	}
}

func TestAnthropicCall_OlderModelKeepsBudget(t *testing.T) {
	var gotMaxTokens float64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotMaxTokens, _ = body["max_tokens"].(float64)
		json.NewEncoder(w).Encode(map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}})
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	if _, err := (&AnthropicProvider{}).Call(context.Background(), "k", "claude-sonnet-4-6", "q", 4096); err != nil {
		t.Fatal(err)
	}
	if int(gotMaxTokens) != 4096 {
		t.Errorf("max_tokens = %v, want 4096", gotMaxTokens)
	}
}

func TestParseAnthropicMessage_NoAnswer(t *testing.T) {
	cases := []struct {
		name, body, wantErr string
	}{
		{"refusal", `{"content":[],"stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber"}}`, "declined to answer (cyber)"},
		{"budget used up thinking", `{"content":[{"type":"thinking","thinking":""}],"stop_reason":"max_tokens"}`, "whole 16096-token output budget"},
		{"only thinking", `{"content":[{"type":"thinking","thinking":""}],"stop_reason":"end_turn"}`, "empty response"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseAnthropicMessage([]byte(c.body), 16096)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, c.wantErr)
			}
		})
	}
}

func TestAnthropicBuildStreamBody_ThinkingHeadroom(t *testing.T) {
	b, _ := (&AnthropicProvider{}).BuildStreamBody("claude-opus-5-5", 8000, "p", false)
	var body map[string]any
	json.Unmarshal(b, &body)
	if int(body["max_tokens"].(float64)) != 8000+reasoningHeadroom {
		t.Errorf("max_tokens = %v", body["max_tokens"])
	}
}
