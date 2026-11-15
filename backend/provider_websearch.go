package main

// Answers with live web search, for the LLM Test's "searched" pass: the same
// question as the remembered pass, answered with each provider's own web
// search tool, plus the pages the answer cites. Each provider has its own
// request and response shapes; the HTTP status handling matches its Call.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// webSearchAnswer is an answer given with web search and the pages it cites.
type webSearchAnswer struct {
	Text    string
	Sources []string // cited URLs (Gemini gives the site, not the page)
}

// webSearchCaller is a provider that can answer with live web search.
type webSearchCaller interface {
	CallWithWebSearch(ctx context.Context, apiKey, model, prompt string, maxTokens int) (webSearchAnswer, error)
}

// webSearchMaxUses bounds the searches one answer may run (Anthropic).
const webSearchMaxUses = 5

// webSearchContinuations bounds how often a paused Anthropic turn is resumed.
const webSearchContinuations = 3

// postJSON sends body to endpoint and returns the status and response body.
func postJSON(ctx context.Context, endpoint string, headers map[string]string, body []byte) (int, []byte, error) {
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("request failed: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// searchStatusError turns a non-200 status into the same errors the
// provider's Call returns: a billing problem is a ProviderHTTPError, a Gemini
// 429 is a quota error, other 429/503/529s are ErrOverloaded.
func searchStatusError(provider string, status int, body []byte) error {
	if status == http.StatusOK {
		return nil
	}
	if providerCreditError(provider, body) != nil {
		return &ProviderHTTPError{Provider: provider, Status: status, Body: string(body)}
	}
	if status == 429 && provider == "Gemini" {
		return geminiQuotaError(body)
	}
	if status == 429 || status == 529 || status == 503 {
		return ErrOverloaded
	}
	return &ProviderHTTPError{Provider: provider, Status: status, Body: string(body)}
}

// addSource appends u to list unless it's empty or already there.
func addSource(list []string, seen map[string]bool, u string) []string {
	u = strings.TrimSpace(u)
	if u == "" || seen[u] {
		return list
	}
	seen[u] = true
	return append(list, u)
}

// ── Anthropic ─────────────────────────────────────────────────────────────

func (p *AnthropicProvider) CallWithWebSearch(ctx context.Context, apiKey, model, prompt string, maxTokens int) (webSearchAnswer, error) {
	budget := anthropicOutputBudget(model, maxTokens)
	messages := []any{map[string]any{"role": "user", "content": prompt}}
	headers := map[string]string{"x-api-key": apiKey, "anthropic-version": "2023-06-01"}
	for turn := 0; ; turn++ {
		body, _ := json.Marshal(map[string]any{
			"model":      model,
			"max_tokens": budget,
			"messages":   messages,
			"tools": []map[string]any{
				{"type": "web_search_20250305", "name": "web_search", "max_uses": webSearchMaxUses},
			},
		})
		status, respBody, err := postJSON(ctx, anthropicAPIBase+"/v1/messages", headers, body)
		if err != nil {
			return webSearchAnswer{}, err
		}
		if err := searchStatusError("Claude", status, respBody); err != nil {
			return webSearchAnswer{}, err
		}
		var msg struct {
			Content    json.RawMessage `json:"content"`
			StopReason string          `json:"stop_reason"`
		}
		if err := json.Unmarshal(respBody, &msg); err != nil {
			return webSearchAnswer{}, fmt.Errorf("failed to parse response: %w", err)
		}
		if msg.StopReason == "pause_turn" && turn < webSearchContinuations {
			// The server-side search loop paused: send its turn back to resume.
			messages = append(messages, map[string]any{"role": "assistant", "content": msg.Content})
			continue
		}
		text, err := parseAnthropicMessage(respBody, budget)
		if err != nil {
			return webSearchAnswer{}, err
		}
		return webSearchAnswer{Text: text, Sources: anthropicCitedURLs(msg.Content)}, nil
	}
}

// anthropicCitedURLs lists the pages an answer's text cites.
func anthropicCitedURLs(content json.RawMessage) []string {
	var blocks []struct {
		Type      string `json:"type"`
		Citations []struct {
			URL string `json:"url"`
		} `json:"citations"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, b := range blocks {
		if b.Type != "text" {
			continue
		}
		for _, c := range b.Citations {
			out = addSource(out, seen, c.URL)
		}
	}
	return out
}

// ── OpenAI and Grok (Responses API) ───────────────────────────────────────

func (p *OpenAIProvider) CallWithWebSearch(ctx context.Context, apiKey, model, prompt string, maxTokens int) (webSearchAnswer, error) {
	return responsesWebSearch(ctx, "OpenAI", openaiAPIBase+"/v1/responses", apiKey, model, prompt, openaiOutputBudget(model, maxTokens))
}

// Grok's web search is only on the Responses API (Live Search on Chat
// Completions is retired).
func (p *GrokProvider) CallWithWebSearch(ctx context.Context, apiKey, model, prompt string, maxTokens int) (webSearchAnswer, error) {
	if isGrokReasoningModel(model) {
		maxTokens += reasoningHeadroom
	}
	return responsesWebSearch(ctx, "Grok", grokAPIBase+"/v1/responses", apiKey, model, prompt, maxTokens)
}

func responsesWebSearch(ctx context.Context, provider, endpoint, apiKey, model, prompt string, maxOutput int) (webSearchAnswer, error) {
	body, _ := json.Marshal(map[string]any{
		"model":             model,
		"max_output_tokens": maxOutput,
		"input":             []map[string]any{{"role": "user", "content": prompt}},
		"tools":             []map[string]any{{"type": "web_search"}},
	})
	status, respBody, err := postJSON(ctx, endpoint, map[string]string{"Authorization": "Bearer " + apiKey}, body)
	if err != nil {
		return webSearchAnswer{}, err
	}
	if err := searchStatusError(provider, status, respBody); err != nil {
		return webSearchAnswer{}, err
	}
	ans, err := parseResponsesWebSearch(respBody)
	if err != nil {
		return webSearchAnswer{}, fmt.Errorf("%s: %w", provider, err)
	}
	return ans, nil
}

// parseResponsesWebSearch reads a Responses API answer: the message text and
// its url_citation annotations (plus xAI's top-level citations list).
func parseResponsesWebSearch(respBody []byte) (webSearchAnswer, error) {
	var result struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Annotations []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
		Citations []string `json:"citations"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return webSearchAnswer{}, fmt.Errorf("failed to parse response: %w", err)
	}
	var text strings.Builder
	var sources []string
	seen := map[string]bool{}
	for _, item := range result.Output {
		if item.Type != "message" {
			continue
		}
		for _, c := range item.Content {
			if c.Type != "output_text" {
				continue
			}
			text.WriteString(c.Text)
			for _, a := range c.Annotations {
				if a.Type == "url_citation" {
					sources = addSource(sources, seen, a.URL)
				}
			}
		}
	}
	for _, u := range result.Citations {
		sources = addSource(sources, seen, u)
	}
	if strings.TrimSpace(text.String()) == "" {
		return webSearchAnswer{}, fmt.Errorf("empty answer")
	}
	return webSearchAnswer{Text: text.String(), Sources: sources}, nil
}

// ── Gemini (Google Search grounding) ──────────────────────────────────────

func (p *GeminiProvider) CallWithWebSearch(ctx context.Context, apiKey, model, prompt string, maxTokens int) (webSearchAnswer, error) {
	if isGeminiThinkingModel(model) {
		maxTokens += reasoningHeadroom
	}
	body, _ := json.Marshal(map[string]any{
		"contents":         []map[string]any{{"parts": []map[string]any{{"text": prompt}}}},
		"generationConfig": map[string]any{"maxOutputTokens": maxTokens},
		"tools":            []map[string]any{{"google_search": map[string]any{}}},
	})
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent", geminiAPIBase, model)
	status, respBody, err := postJSON(ctx, endpoint, map[string]string{"x-goog-api-key": apiKey}, body)
	if err != nil {
		return webSearchAnswer{}, err
	}
	if err := searchStatusError("Gemini", status, respBody); err != nil {
		return webSearchAnswer{}, err
	}
	return parseGeminiWebSearch(respBody)
}

// parseGeminiWebSearch reads a grounded Gemini answer. Its grounding sources
// are redirect links whose title is the site's domain, so the site is kept.
func parseGeminiWebSearch(respBody []byte) (webSearchAnswer, error) {
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
			GroundingMetadata struct {
				GroundingChunks []struct {
					Web struct {
						URI   string `json:"uri"`
						Title string `json:"title"`
					} `json:"web"`
				} `json:"groundingChunks"`
			} `json:"groundingMetadata"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return webSearchAnswer{}, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(result.Candidates) == 0 {
		return webSearchAnswer{}, fmt.Errorf("empty response from Gemini")
	}
	c := result.Candidates[0]
	var text strings.Builder
	for _, part := range c.Content.Parts {
		if !part.Thought {
			text.WriteString(part.Text)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return webSearchAnswer{}, fmt.Errorf("empty response from Gemini")
	}
	var sources []string
	seen := map[string]bool{}
	for _, ch := range c.GroundingMetadata.GroundingChunks {
		u := ch.Web.URI
		if host := urlHost(u); host == "" || strings.HasSuffix(host, "vertexaisearch.cloud.google.com") {
			if t := strings.TrimSpace(ch.Web.Title); t != "" && strings.Contains(t, ".") && !strings.Contains(t, " ") {
				u = "https://" + t + "/"
			}
		}
		sources = addSource(sources, seen, u)
	}
	return webSearchAnswer{Text: text.String(), Sources: sources}, nil
}

// urlHost returns u's lowercased host without "www.", or "".
func urlHost(u string) string {
	parsed, err := url.Parse(strings.TrimSpace(u))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

// isOwnSite reports whether u is on domain or one of its subdomains.
func isOwnSite(u, domain string) bool {
	host := urlHost(u)
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "www.")
	return host != "" && domain != "" && (host == domain || strings.HasSuffix(host, "."+domain))
}
