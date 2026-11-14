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

// geminiAPIBase is the base URL for the Gemini API. Overridable in tests.
var geminiAPIBase = "https://generativelanguage.googleapis.com"

// GeminiProvider implements LLMProvider for the Google Gemini API.
type GeminiProvider struct{}

func (p *GeminiProvider) ProviderID() string { return "gemini" }
func (p *GeminiProvider) Name() string       { return "Gemini" }

func (p *GeminiProvider) Models() []ModelDef {
	return []ModelDef{
		{ID: "gemini-3.1-pro-preview", Name: "Gemini 3.1 Pro"},
		{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash"},
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash"},
	}
}

func (p *GeminiProvider) SmallModel() string {
	return "gemini-2.5-flash"
}

// BuildStreamBody for Gemini is special — the model goes in the URL, not the body.
// We embed the model in a wrapper field so Stream() can extract it.
func (p *GeminiProvider) BuildStreamBody(model string, maxTokens int, prompt string, useWebSearch bool) ([]byte, error) {
	contents := []map[string]any{
		{
			"parts": []map[string]any{
				{"text": prompt},
			},
		},
	}

	body := map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"maxOutputTokens": maxTokens,
		},
		// Embed model for Stream() to extract
		"_model": model,
	}

	if useWebSearch {
		body["tools"] = []map[string]any{
			{
				"google_search": map[string]any{},
			},
		}
	}

	return json.Marshal(body)
}

func (p *GeminiProvider) Call(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
	if isGeminiThinkingModel(model) {
		maxTokens += reasoningHeadroom // thinking tokens share the output budget
	}
	body, _ := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": maxTokens,
		},
	})

	// The key goes in a header, not the URL, so it can't leak into error text.
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", geminiAPIBase, model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", apiKey)

	resp, err := llmHTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 529 || resp.StatusCode == 503 {
		return "", ErrOverloaded
	}

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", &ProviderHTTPError{Provider: "Gemini", Status: resp.StatusCode, Body: string(respBody)}
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}
	// An answer can arrive in several parts; join the visible ones.
	var text strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		if !part.Thought {
			text.WriteString(part.Text)
		}
	}
	if text.Len() == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}
	return text.String(), nil
}

func (p *GeminiProvider) Stream(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
	// Extract model from the body wrapper
	var wrapper struct {
		Model string `json:"_model"`
	}
	json.Unmarshal(body, &wrapper)
	model := wrapper.Model
	if model == "" {
		model = "gemini-3.1-pro-preview"
	}

	// Remove the _model field before sending to the API
	var bodyMap map[string]any
	json.Unmarshal(body, &bodyMap)
	delete(bodyMap, "_model")
	cleanBody, _ := json.Marshal(bodyMap)

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", geminiAPIBase, model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(cleanBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", apiKey)

	resp, err := llmStreamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 529 || resp.StatusCode == 503 {
		log.Printf("Gemini API returned %d (overloaded/rate-limited)", resp.StatusCode)
		return nil, ErrOverloaded
	}
	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(errBody))
	}

	sendSSE(w, flusher, "status", map[string]string{
		"message": "Connected to Gemini, beginning analysis...",
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
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if len(event.Candidates) > 0 && len(event.Candidates[0].Content.Parts) > 0 {
			text := event.Candidates[0].Content.Parts[0].Text
			if text != "" {
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

func (p *GeminiProvider) VerifyKey(ctx context.Context, apiKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	model, status := verifyModelFor(ctx, p, apiKey)
	if status != "" {
		return status, nil
	}
	body, _ := json.Marshal(map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": "Reply with just the word 'ok'."},
				},
			},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 10,
		},
	})

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", geminiAPIBase, model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "error", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", apiKey)

	resp, err := llmHTTPClient.Do(httpReq)
	if err != nil {
		return "error", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 200:
		return "active", nil
	case resp.StatusCode == 400:
		errBody, _ := io.ReadAll(resp.Body)
		errStr := strings.ToLower(string(errBody))
		if strings.Contains(errStr, "api_key_invalid") || strings.Contains(errStr, "api key not valid") {
			return "invalid", nil
		}
		return "error", fmt.Errorf("bad request: %s", string(errBody))
	case resp.StatusCode == 403:
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
