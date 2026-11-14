package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// GrokProvider implements LLMProvider for the xAI Grok API.
// The xAI API is OpenAI-compatible (chat completions format).
// grokAPIBase is the base URL for the Grok API. Overridable in tests.
var grokAPIBase = "https://api.x.ai"

type GrokProvider struct{}

func (p *GrokProvider) ProviderID() string { return "grok" }
func (p *GrokProvider) Name() string       { return "Grok" }

func (p *GrokProvider) Models() []ModelDef {
	return []ModelDef{
		{ID: "grok-4-0709", Name: "Grok 4"},
		{ID: "grok-4-fast-non-reasoning", Name: "Grok 4 Fast"},
		{ID: "grok-3", Name: "Grok 3"},
		{ID: "grok-3-mini", Name: "Grok 3 Mini"},
	}
}

func (p *GrokProvider) SmallModel() string {
	return "grok-3-mini"
}

func (p *GrokProvider) BuildStreamBody(model string, maxTokens int, prompt string, useWebSearch bool) ([]byte, error) {
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"stream":     true,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	if useWebSearch {
		body["search_parameters"] = map[string]any{
			"mode": "auto",
		}
	}
	return json.Marshal(body)
}

func (p *GrokProvider) Call(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
	if isGrokReasoningModel(model) {
		maxTokens += reasoningHeadroom // reasoning tokens share the output budget
	}
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", grokAPIBase+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := llmHTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 529 {
		return "", ErrOverloaded
	}

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", &ProviderHTTPError{Provider: "Grok", Status: resp.StatusCode, Body: string(respBody)}
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from Grok")
	}
	return result.Choices[0].Message.Content, nil
}

func (p *GrokProvider) Stream(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", grokAPIBase+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := llmStreamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 529 {
		log.Printf("Grok API returned %d (overloaded/rate-limited)", resp.StatusCode)
		return nil, ErrOverloaded
	}
	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Grok API error (%d): %s", resp.StatusCode, string(errBody))
	}

	sendSSE(w, flusher, "status", map[string]string{
		"message": "Connected to Grok, beginning analysis...",
	})

	var fullText strings.Builder
	lastProgressAt := time.Now()

	scanner := bufio.NewScanner(wrapWithIdleTimeout(resp, streamIdleTimeout))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := line[6:]
		if data == "[DONE]" {
			break
		}

		var event struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if len(event.Choices) > 0 {
			content := event.Choices[0].Delta.Content
			if content != "" {
				fullText.WriteString(content)
				sendSSE(w, flusher, "text", map[string]string{
					"content": content,
				})
				if time.Since(lastProgressAt) > 3*time.Second {
					chars := fullText.Len()
					sendSSE(w, flusher, "progress", map[string]string{
						"message": fmt.Sprintf("Generating analysis... (%dk chars received)", chars/1000),
					})
					lastProgressAt = time.Now()
				}
			}

			if event.Choices[0].FinishReason != nil {
				resultJSON := extractJSON(fullText.String())
				return &StreamResult{RawText: fullText.String(), ResultJSON: resultJSON}, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream reading error: %w", err)
	}

	if fullText.Len() > 0 {
		resultJSON := extractJSON(fullText.String())
		return &StreamResult{RawText: fullText.String(), ResultJSON: resultJSON}, nil
	}

	return nil, fmt.Errorf("stream ended without results")
}

func (p *GrokProvider) VerifyKey(ctx context.Context, apiKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	model, status := verifyModelFor(ctx, p, apiKey)
	if status != "" {
		return status, nil
	}
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 10,
		"messages": []map[string]any{
			{"role": "user", "content": "Reply with just the word 'ok'."},
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", grokAPIBase+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "error", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := llmHTTPClient.Do(httpReq)
	if err != nil {
		return "error", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 200:
		return "active", nil
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		errBody, _ := io.ReadAll(resp.Body)
		if isBillingMessage(strings.ToLower(string(errBody))) || strings.Contains(strings.ToLower(string(errBody)), "credits") {
			return "no_credits", nil
		}
		return "invalid", nil
	case resp.StatusCode == 429:
		errBody, _ := io.ReadAll(resp.Body)
		errStr := strings.ToLower(string(errBody))
		if strings.Contains(errStr, "quota") || strings.Contains(errStr, "billing") {
			return "no_credits", nil
		}
		return "active", nil
	default:
		errBody, _ := io.ReadAll(resp.Body)
		return "error", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(errBody))
	}
}
