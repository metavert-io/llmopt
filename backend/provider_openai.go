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

// openaiAPIBase is the base URL for the OpenAI API. Overridable in tests.
var openaiAPIBase = "https://api.openai.com"

// OpenAIProvider implements LLMProvider for the OpenAI API.
type OpenAIProvider struct{}

func (p *OpenAIProvider) ProviderID() string { return "openai" }
func (p *OpenAIProvider) Name() string       { return "OpenAI" }

func (p *OpenAIProvider) Models() []ModelDef {
	return []ModelDef{
		{ID: "gpt-4o", Name: "GPT-4o"},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini"},
	}
}

func (p *OpenAIProvider) SmallModel() string {
	return "gpt-4o-mini"
}

// openaiOutputBudget returns max_output_tokens for a Responses API call. Reasoning
// models spend part of the budget thinking, so they get extra room.
func openaiOutputBudget(model string, maxTokens int) int {
	if isOpenAIReasoningModel(model) {
		return maxTokens + reasoningHeadroom
	}
	return maxTokens
}

// The Responses API takes "input" and "max_output_tokens" (not the Chat
// Completions "messages" and "max_tokens", which it rejects).
func (p *OpenAIProvider) BuildStreamBody(model string, maxTokens int, prompt string, useWebSearch bool) ([]byte, error) {
	body := map[string]any{
		"model":             model,
		"max_output_tokens": openaiOutputBudget(model, maxTokens),
		"stream":            true,
		"input": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	if useWebSearch {
		body["tools"] = []map[string]any{
			{"type": "web_search_preview"},
		}
	}
	return json.Marshal(body)
}

func (p *OpenAIProvider) Call(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":             model,
		"max_output_tokens": openaiOutputBudget(model, maxTokens),
		"input": []map[string]any{
			{"role": "user", "content": prompt},
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiAPIBase+"/v1/responses", bytes.NewReader(body))
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

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 429 || resp.StatusCode == 529 {
		// A 429 for an exhausted quota is a billing problem, not a busy server.
		if resp.StatusCode == 429 && isBillingMessage(strings.ToLower(string(respBody))) {
			return "", &ProviderHTTPError{Provider: "OpenAI", Status: resp.StatusCode, Body: string(respBody)}
		}
		return "", ErrOverloaded
	}
	if resp.StatusCode != 200 {
		return "", &ProviderHTTPError{Provider: "OpenAI", Status: resp.StatusCode, Body: string(respBody)}
	}

	// OpenAI Responses API format
	var result struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	for _, item := range result.Output {
		if item.Type == "message" {
			for _, c := range item.Content {
				if c.Type == "output_text" {
					return c.Text, nil
				}
			}
		}
	}
	return "", fmt.Errorf("empty response from OpenAI")
}

func (p *OpenAIProvider) Stream(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiAPIBase+"/v1/responses", bytes.NewReader(body))
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
		log.Printf("OpenAI API returned %d (overloaded/rate-limited)", resp.StatusCode)
		return nil, ErrOverloaded
	}
	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(errBody))
	}

	sendSSE(w, flusher, "status", map[string]string{
		"message": "Connected to OpenAI, beginning analysis...",
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

		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "response.output_text.delta":
			delta, _ := event["delta"].(string)
			if delta != "" {
				fullText.WriteString(delta)
				sendSSE(w, flusher, "text", map[string]string{
					"content": delta,
				})
				if time.Since(lastProgressAt) > 3*time.Second {
					chars := fullText.Len()
					sendSSE(w, flusher, "progress", map[string]string{
						"message": fmt.Sprintf("Generating analysis... (%dk chars received)", chars/1000),
					})
					lastProgressAt = time.Now()
				}
			}

		case "response.web_search_call.in_progress":
			sendSSE(w, flusher, "status", map[string]string{
				"message": "Searching the web...",
			})

		case "response.output_text.done":
			// final text available
			if text, ok := event["text"].(string); ok && text != "" {
				fullText.Reset()
				fullText.WriteString(text)
			}

		case "response.completed", "response.done":
			resultJSON := extractJSON(fullText.String())
			return &StreamResult{RawText: fullText.String(), ResultJSON: resultJSON}, nil

		case "error":
			errMsg := "OpenAI API error"
			if msg, ok := event["message"].(string); ok {
				errMsg = msg
			}
			log.Printf("OpenAI API stream error: %s", errMsg)
			return nil, fmt.Errorf("%s", errMsg)
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

func (p *OpenAIProvider) VerifyKey(ctx context.Context, apiKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	model, status := verifyModelFor(ctx, p, apiKey)
	if status != "" {
		return status, nil
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": "Reply with just the word 'ok'.",
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openaiAPIBase+"/v1/responses", bytes.NewReader(body))
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
	case resp.StatusCode == 401:
		return "invalid", nil
	case resp.StatusCode == 429:
		errBody, _ := io.ReadAll(resp.Body)
		errStr := strings.ToLower(string(errBody))
		if strings.Contains(errStr, "quota") || strings.Contains(errStr, "billing") || strings.Contains(errStr, "exceeded") {
			return "no_credits", nil
		}
		return "active", nil
	default:
		errBody, _ := io.ReadAll(resp.Body)
		return "error", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(errBody))
	}
}
