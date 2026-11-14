package main

// Per-provider state for an LLM brand test: which models to try, and whether a
// provider has turned out to be unusable so its remaining queries are skipped
// while the other providers carry on.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// maxModelSwitches bounds how many times one run moves to a provider's next model.
const maxModelSwitches = 3

type testProviderRun struct {
	provider LLMProvider
	apiKey   string
	notify   func(string) // progress notes for the client

	mu       sync.Mutex
	models   []ModelDef // candidates, the chosen model first
	modelIdx int
	switches int
	deadErr  error // set once the key, or every model, has failed for good
	lastErr  error
	answered int
	failed   int
}

func (pr *testProviderRun) currentModel() ModelDef {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.models[pr.modelIdx]
}

// movedPast reports whether this run gave up on the model as unavailable.
func (pr *testProviderRun) movedPast(id string) bool {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	for _, m := range pr.models[:pr.modelIdx] {
		if m.ID == id {
			return true
		}
	}
	return false
}

// markDead stops further calls to this provider and tells the client once.
func (pr *testProviderRun) markDead(err error) {
	pr.mu.Lock()
	first := pr.deadErr == nil
	if first {
		pr.deadErr = err
	}
	pr.mu.Unlock()
	if first {
		pr.notify(fmt.Sprintf("%s can't be used: %s. Its remaining queries are skipped; the other providers continue.",
			pr.provider.Name(), providerFailureReason(err)))
	}
}

func (pr *testProviderRun) record(err error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if err == nil {
		pr.answered++
		return
	}
	pr.failed++
	pr.lastErr = err
}

// ask sends one query. It moves to the provider's next model when the current
// one is retired, and stops calling the provider once its key is known to be
// unusable. skipped is true when no call was made because of an earlier failure.
func (pr *testProviderRun) ask(ctx context.Context, query string, maxTokens int) (resp string, model ModelDef, err error, skipped bool) {
	for attempt := 0; attempt <= maxModelSwitches; attempt++ {
		pr.mu.Lock()
		model = pr.models[pr.modelIdx]
		dead := pr.deadErr
		pr.mu.Unlock()
		if dead != nil {
			return "", model, dead, attempt == 0
		}

		resp, err = callWithRetry(ctx, pr.provider, pr.apiKey, model.ID, query, maxTokens)
		if err == nil && strings.TrimSpace(resp) == "" {
			// A blank answer would be scored as "brand not mentioned".
			err = fmt.Errorf("%s returned an empty answer", model.Name)
		}
		if err == nil {
			return resp, model, nil, false
		}
		if ctx.Err() != nil {
			return "", model, err, false
		}
		switch classifyProviderError(err) {
		case failKey:
			pr.markDead(err)
			return "", model, err, false
		case failModel:
			pr.mu.Lock()
			if pr.models[pr.modelIdx].ID != model.ID {
				pr.mu.Unlock() // another query already moved on; retry on the new model
				continue
			}
			if pr.modelIdx+1 < len(pr.models) && pr.switches < maxModelSwitches {
				pr.modelIdx++
				pr.switches++
				next := pr.models[pr.modelIdx]
				pr.mu.Unlock()
				pr.notify(fmt.Sprintf("%s: %s isn't available to this API key; switching to %s.", pr.provider.Name(), model.Name, next.Name))
				continue
			}
			pr.mu.Unlock()
			pr.markDead(err)
			return "", model, err, false
		default:
			return "", model, err, false
		}
	}
	return "", model, err, false
}

// testModelOrder puts the requested model first when the key can use it, and
// otherwise the provider's test default (see testDefaultModel). The rest follow
// as fallbacks. ok is false when a model was requested that isn't in the list.
func testModelOrder(providerID string, models []ModelDef, requested string) (ordered []ModelDef, ok bool) {
	ok = true
	if requested != "" {
		if _, found := findModel(models, requested); !found {
			ok, requested = false, ""
		}
	}
	if requested == "" {
		requested, _ = testDefaultModel(providerID, models)
	}
	for i, m := range models {
		if m.ID == requested {
			ordered = append([]ModelDef{m}, models[:i]...)
			return append(ordered, models[i+1:]...), ok
		}
	}
	return models, ok
}

func describeUnavailable(list []LLMTestUnavailable) string {
	parts := make([]string, 0, len(list))
	for _, u := range list {
		parts = append(parts, fmt.Sprintf("%s (%s)", u.ProviderName, u.Reason))
	}
	return strings.Join(parts, ", ")
}

func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// testEvaluation is the evaluator's verdict on each query/provider answer.
type testEvaluation struct {
	Evaluations []struct {
		QueryIndex  int    `json:"query_index"`
		ProviderID  string `json:"provider_id"`
		Mentioned   bool   `json:"mentioned"`
		Recommended bool   `json:"recommended"`
		Sentiment   string `json:"sentiment"`
		Accuracy    string `json:"accuracy"`
		Score       int    `json:"score"`
	} `json:"evaluations"`
}

// parseTestEvaluation reads the evaluator's JSON, tolerating code fences and
// surrounding prose.
func parseTestEvaluation(raw string, out *testEvaluation) error {
	if err := json.Unmarshal([]byte(stripJSONFencing(raw)), out); err == nil {
		return nil
	}
	if j := extractJSON(raw); j != "" {
		if err := json.Unmarshal([]byte(j), out); err == nil {
			return nil
		}
	}
	return fmt.Errorf("evaluation is not valid JSON")
}
