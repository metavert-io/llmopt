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

// anthropicAPIBase is the base URL for the Anthropic API. Overridable in tests.
var anthropicAPIBase = "https://api.anthropic.com"

// AnthropicProvider implements LLMProvider for the Anthropic Claude API.
type AnthropicProvider struct{}

func (p *AnthropicProvider) ProviderID() string { return "anthropic" }
func (p *AnthropicProvider) Name() string       { return "Anthropic" }

func (p *AnthropicProvider) Models() []ModelDef {
	return []ModelDef{
		{ID: "claude-sonnet-4-6", Name: "Sonnet 4.6"},
		{ID: "claude-haiku-4-5-20251001", Name: "Haiku 4.5"},
	}
}

func (p *AnthropicProvider) SmallModel() string {
	return "claude-haiku-4-5-20251001"
}

func (p *AnthropicProvider) BuildStreamBody(model string, maxTokens int, prompt string, useWebSearch bool) ([]byte, error) {
	body := map[string]any{
		"model":      model,
		"max_tokens": anthropicOutputBudget(model, maxTokens),
		"stream":     true,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	if useWebSearch {
		body["tools"] = []map[string]any{
			{
				"type": "web_search_20250305",
				"name": "web_search",
			},
		}
	}
	return json.Marshal(body)
}

func (p *AnthropicProvider) Call(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
	budget := anthropicOutputBudget(model, maxTokens)
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": budget,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	})

	// Per-call timeout: 3 minutes for Sonnet-level calls (non-streaming waits for full response)
	callCtx, callCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer callCancel()

	httpReq, err := http.NewRequestWithContext(callCtx, "POST", anthropicAPIBase+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := llmHTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 529 || resp.StatusCode == 429 {
		return "", ErrOverloaded
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", fmt.Errorf("request failed: %w", readErr)
	}
	if resp.StatusCode != 200 {
		return "", &ProviderHTTPError{Provider: "Claude", Status: resp.StatusCode, Body: string(respBody)}
	}

	return parseAnthropicMessage(respBody, budget)
}

// parseAnthropicMessage returns the answer text of a Messages API response.
// Thinking models put thinking blocks (with empty text by default) ahead of the
// answer, and citations split an answer into several text blocks, so every
// text block is joined rather than reading the first block.
func parseAnthropicMessage(respBody []byte, budget int) (string, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason  string `json:"stop_reason"`
		StopDetails *struct {
			Category string `json:"category"`
		} `json:"stop_details"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	var text strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	if result.StopReason == "refusal" {
		category := ""
		if result.StopDetails != nil && result.StopDetails.Category != "" {
			category = " (" + result.StopDetails.Category + ")"
		}
		return "", fmt.Errorf("Claude declined to answer%s", category)
	}
	answer := strings.TrimSpace(text.String())
	if answer == "" {
		if result.StopReason == "max_tokens" {
			return "", fmt.Errorf("Claude used its whole %d-token output budget before writing an answer", budget)
		}
		return "", fmt.Errorf("empty response from Claude (stop reason %q)", result.StopReason)
	}
	return answer, nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIBase+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := llmStreamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 529 || resp.StatusCode == 429 {
		log.Printf("Claude API returned %d (overloaded/rate-limited)", resp.StatusCode)
		return nil, ErrOverloaded
	}
	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Claude API error (%d): %s", resp.StatusCode, string(errBody))
	}

	sendSSE(w, flusher, "status", map[string]string{
		"message": "Connected to Claude, beginning analysis...",
	})

	var fullText strings.Builder
	var currentBlockType string
	searchCount := 0
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
		case "error":
			errObj, _ := event["error"].(map[string]any)
			errType := ""
			errMsg := "Claude API error"
			if errObj != nil {
				errType, _ = errObj["type"].(string)
				if msg, ok := errObj["message"].(string); ok {
					errMsg = msg
				}
			}
			log.Printf("Claude API stream error: type=%s message=%s", errType, errMsg)
			if errType == "overloaded_error" {
				return nil, ErrOverloaded
			}
			return nil, fmt.Errorf("%s", errMsg)

		case "content_block_start":
			block, _ := event["content_block"].(map[string]any)
			if block == nil {
				continue
			}
			blockType, _ := block["type"].(string)
			currentBlockType = blockType

			switch blockType {
			case "server_tool_use":
				name, _ := block["name"].(string)
				if name == "web_search" {
					searchCount++
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("Searching the web (search #%d)...", searchCount),
					})
				}
			case "web_search_tool_result":
				sendSSE(w, flusher, "status", map[string]string{
					"message": "Processing search results...",
				})
			case "text":
				sendSSE(w, flusher, "status", map[string]string{
					"message": "Generating analysis...",
				})
			}

		case "content_block_delta":
			delta, _ := event["delta"].(map[string]any)
			if delta == nil {
				continue
			}
			deltaType, _ := delta["type"].(string)
			if deltaType == "text_delta" && currentBlockType == "text" {
				text, _ := delta["text"].(string)
				fullText.WriteString(text)
				sendSSE(w, flusher, "text", map[string]string{
					"content": text,
				})
				if time.Since(lastProgressAt) > 3*time.Second {
					chars := fullText.Len()
					sendSSE(w, flusher, "progress", map[string]string{
						"message": fmt.Sprintf("Generating analysis... (%dk chars received)", chars/1000),
					})
					lastProgressAt = time.Now()
				}
			}

		case "message_stop":
			resultJSON := extractJSON(fullText.String())
			return &StreamResult{RawText: fullText.String(), ResultJSON: resultJSON}, nil
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

func (p *AnthropicProvider) VerifyKey(ctx context.Context, apiKey string) (string, error) {
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIBase+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "error", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		if strings.Contains(errStr, "credit") || strings.Contains(errStr, "billing") {
			return "no_credits", nil
		}
		return "active", nil
	case resp.StatusCode == 529:
		return "active", nil
	case resp.StatusCode == 400:
		// A low credit balance comes back as a 400 invalid_request_error.
		errBody, _ := io.ReadAll(resp.Body)
		if isBillingMessage(strings.ToLower(string(errBody))) {
			return "no_credits", nil
		}
		return "error", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(errBody))
	default:
		errBody, _ := io.ReadAll(resp.Body)
		return "error", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(errBody))
	}
}
