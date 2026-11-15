package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An Anthropic stream that fails with "credit balance is too low" is an
// out-of-credit error, and analyses report it as such.
func TestAnthropicStream_CreditBalanceTooLow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"Your credit balance is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase credits.\"}}\n\n"))
	}))
	defer server.Close()
	withAllProviderBases(t, server)
	body, _ := (&AnthropicProvider{}).BuildStreamBody("claude-opus-5-5", 100, "q", false)
	rec := httptest.NewRecorder()
	_, err := (&AnthropicProvider{}).Stream(context.Background(), "k", body, rec, rec)
	var ce *creditError
	if !errors.As(err, &ce) || ce.Provider != "Anthropic" {
		t.Fatalf("err = %v, want a creditError", err)
	}
	if code := classifyError(err); code != ErrCodeAPICredits {
		t.Errorf("classified %q", code)
	}
	if msg := userFriendlyError(ErrCodeAPICredits); !strings.Contains(msg, "out of credit") || strings.Contains(msg, "upstream provider issue") {
		t.Errorf("message %q", msg)
	}
}

// A 429 is read before deciding: insufficient quota is out of credit, a plain
// rate limit is still "overloaded" (and retried).
func TestOpenAIStream_429QuotaVsRateLimit(t *testing.T) {
	body := `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(body))
	}))
	defer server.Close()
	withAllProviderBases(t, server)
	req, _ := (&OpenAIProvider{}).BuildStreamBody("gpt-5.5", 100, "q", false)
	rec := httptest.NewRecorder()
	_, err := (&OpenAIProvider{}).Stream(context.Background(), "k", req, rec, rec)
	if classifyError(err) != ErrCodeAPICredits {
		t.Errorf("insufficient_quota: %v", err)
	}
	body = `{"error":{"message":"Rate limit reached. Visit https://platform.openai.com/account/billing to add a payment method.","type":"requests"}}`
	_, err = (&OpenAIProvider{}).Stream(context.Background(), "k", req, rec, rec)
	if !errors.Is(err, ErrOverloaded) {
		t.Errorf("rate limit: %v", err)
	}
}

func TestAnthropicModelsAreOpusFirst(t *testing.T) {
	m := (&AnthropicProvider{}).Models()
	if len(m) < 2 || m[0].ID != "claude-opus-5-5" || m[1].ID != "claude-sonnet-5-5" {
		t.Errorf("models %+v", m)
	}
}
