package main

// Live model discovery and provider error handling.
//
// Each provider is asked which models the tenant's key can use, so the LLM test
// offers (and defaults to) current models instead of a hard-coded list that goes
// stale as providers release and retire models. The built-in Models() lists stay
// as the fallback when a provider can't be asked.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProviderHTTPError is a non-2xx answer from a provider's API. Its message keeps
// the "<Provider> API error (<status>): <body>" shape shown elsewhere, while
// letting callers tell a bad key from a retired model from a busy server.
type ProviderHTTPError struct {
	Provider string
	Status   int
	Body     string
}

func (e *ProviderHTTPError) Error() string {
	return fmt.Sprintf("%s API error (%d): %s", e.Provider, e.Status, e.Body)
}

// providerFailure says what a failed provider call means for the rest of a run.
type providerFailure int

const (
	failOther providerFailure = iota // this call failed; keep using the provider
	failRetry                        // rate limit, overload or server error: worth retrying
	failModel                        // the model is retired or not available to this key
	failKey                          // the key is invalid, revoked or out of credit
)

func isBillingMessage(body string) bool {
	for _, s := range []string{"credit balance", "insufficient_quota", "exceeded your current quota", "billing", "doesn't have any credits", "does not have any credits", "out of credits", "spending limit", "payment required"} {
		if strings.Contains(body, s) {
			return true
		}
	}
	return false
}

func isInvalidKeyMessage(body string) bool {
	for _, s := range []string{"api_key_invalid", "api key not valid", "incorrect api key", "invalid x-api-key", "invalid api key", "invalid_api_key"} {
		if strings.Contains(body, s) {
			return true
		}
	}
	return false
}

func isModelUnavailableMessage(body string) bool {
	if !strings.Contains(body, "model") {
		return false
	}
	for _, s := range []string{"not found", "does not exist", "do not have access", "not have access", "not supported", "no longer", "deprecated", "decommissioned", "retired", "must be verified", "not available", "invalid model", "unknown model"} {
		if strings.Contains(body, s) {
			return true
		}
	}
	return false
}

// classifyProviderError decides whether a failed call should be retried, should
// move on to another model, or means the provider can't be used at all.
func classifyProviderError(err error) providerFailure {
	if err == nil {
		return failOther
	}
	if errors.Is(err, ErrOverloaded) {
		return failRetry
	}
	var he *ProviderHTTPError
	if !errors.As(err, &he) {
		return failOther
	}
	body := strings.ToLower(he.Body)
	switch {
	case isBillingMessage(body):
		return failKey
	case he.Status == 401 || he.Status == 402 || isInvalidKeyMessage(body):
		return failKey
	case he.Status == 404 || isModelUnavailableMessage(body):
		return failModel
	case he.Status == 403:
		return failKey
	case he.Status == 408 || he.Status == 409 || he.Status == 425 || he.Status == 429 || he.Status >= 500:
		return failRetry
	}
	return failOther
}

// providerFailureReason turns the last error from a provider into a short,
// plain sentence for the results page.
func providerFailureReason(err error) string {
	var he *ProviderHTTPError
	body := ""
	if errors.As(err, &he) {
		body = strings.ToLower(he.Body)
	}
	switch classifyProviderError(err) {
	case failKey:
		if isBillingMessage(body) {
			return "the API key is out of credits or has a billing problem"
		}
		return "the API key was rejected (invalid, revoked or without access)"
	case failModel:
		return "none of its models could be used with this API key"
	case failRetry:
		return "its API stayed rate-limited or overloaded"
	}
	if err == nil {
		return "it returned no answers"
	}
	return "its API returned errors: " + truncateStr(err.Error(), 200)
}

// providerRetryDelays are the waits between attempts when a provider is
// rate-limited or overloaded. A variable so tests can shorten it.
var providerRetryDelays = []time.Duration{3 * time.Second, 8 * time.Second}

// callWithRetry calls the provider, retrying rate-limit and overload failures.
func callWithRetry(ctx context.Context, p LLMProvider, apiKey, model, prompt string, maxTokens int) (string, error) {
	for attempt := 0; ; attempt++ {
		resp, err := p.Call(ctx, apiKey, model, prompt, maxTokens)
		if err == nil || classifyProviderError(err) != failRetry || attempt >= len(providerRetryDelays) {
			return resp, err
		}
		select {
		case <-ctx.Done():
			return "", err
		case <-time.After(providerRetryDelays[attempt]):
		}
	}
}

// ── Model listing ────────────────────────────────────────────────────────

// ModelLister is implemented by providers that can say which models a key can use.
type ModelLister interface {
	// ListModels returns the chat models the key can use, best first.
	ListModels(ctx context.Context, apiKey string) ([]ModelDef, error)
}

const modelListTTL = 6 * time.Hour

// modelListFailureTTL is how long a failed listing is remembered, so a page
// that asks for models doesn't call the provider each time with a bad key.
const modelListFailureTTL = 10 * time.Minute

type modelCacheEntry struct {
	models  []ModelDef
	fetched time.Time
}

var modelCache = struct {
	sync.Mutex
	m map[string]modelCacheEntry
}{m: map[string]modelCacheEntry{}}

func modelCacheKey(providerID, apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return providerID + ":" + hex.EncodeToString(sum[:8])
}

func storeModelCache(providerID, apiKey string, models []ModelDef) {
	if len(models) == 0 {
		return
	}
	modelCache.Lock()
	modelCache.m[modelCacheKey(providerID, apiKey)] = modelCacheEntry{models: models, fetched: time.Now()}
	modelCache.Unlock()
}

// availableModels returns the models this key can use, best first, and whether
// the list came from the provider (live) or the built-in fallback.
func availableModels(ctx context.Context, p LLMProvider, apiKey string) ([]ModelDef, bool) {
	lister, ok := p.(ModelLister)
	if !ok || apiKey == "" {
		return p.Models(), false
	}
	key := modelCacheKey(p.ProviderID(), apiKey)
	modelCache.Lock()
	entry, hit := modelCache.m[key]
	modelCache.Unlock()
	if hit && entry.models != nil && time.Since(entry.fetched) < modelListTTL {
		return entry.models, true
	}
	if hit && entry.models == nil && time.Since(entry.fetched) < modelListFailureTTL {
		return p.Models(), false
	}

	lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	models, err := lister.ListModels(lctx, apiKey)
	if err != nil || len(models) == 0 {
		if err != nil {
			log.Printf("Listing %s models failed, using the built-in list: %v", p.Name(), err)
		}
		if hit && entry.models != nil {
			return entry.models, true // stale, but still the provider's own answer
		}
		modelCache.Lock()
		modelCache.m[key] = modelCacheEntry{fetched: time.Now()} // remember the failure briefly
		modelCache.Unlock()
		return p.Models(), false
	}
	storeModelCache(p.ProviderID(), apiKey, models)
	return models, true
}

// findModel returns the model with the given ID from the list.
func findModel(models []ModelDef, id string) (ModelDef, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelDef{}, false
}

// pickEvalModel chooses the model for an evaluation pass: the tenant's preferred
// model if the key can use it, else the provider's usual default if it is still
// offered, else the best model the key can use.
func pickEvalModel(ctx context.Context, p LLMProvider, apiKey, preferred string) string {
	models, live := availableModels(ctx, p, apiKey)
	if preferred != "" {
		if _, ok := findModel(models, preferred); ok || !live {
			return preferred
		}
	}
	if def := p.Models(); len(def) > 0 {
		if _, ok := findModel(models, def[0].ID); ok || !live {
			return def[0].ID
		}
	}
	return models[0].ID
}

// verifyModelFor picks the model a key check should use: the cheapest current
// model the key can see, so a retired model can't make a good key look broken.
// It returns status "invalid" when the model listing itself rejects the key.
func verifyModelFor(ctx context.Context, p LLMProvider, apiKey string) (model, status string) {
	lister, ok := p.(ModelLister)
	if !ok {
		return p.SmallModel(), ""
	}
	models, err := lister.ListModels(ctx, apiKey)
	if err != nil {
		var he *ProviderHTTPError
		if errors.As(err, &he) && !isBillingMessage(strings.ToLower(he.Body)) &&
			(he.Status == 401 || isInvalidKeyMessage(strings.ToLower(he.Body))) {
			return "", "invalid"
		}
		return p.SmallModel(), ""
	}
	storeModelCache(p.ProviderID(), apiKey, models)
	if small := smallestModel(p.ProviderID(), models); small != "" {
		return small, ""
	}
	return p.SmallModel(), ""
}

// smallestModel returns the newest model of the lowest tier in a best-first list.
func smallestModel(providerID string, models []ModelDef) string {
	best, bestTier := "", 1<<30
	for _, m := range models {
		if t := modelTier(providerID, m.ID); t < bestTier {
			best, bestTier = m.ID, t
		}
	}
	return best
}

// ── Ranking ──────────────────────────────────────────────────────────────

type rankedModel struct {
	def     ModelDef
	version float64
	tier    int
	preview bool
	created int64
}

// sortRanked orders models best first: newest generation, then the most capable
// tier, then stable before preview, then most recently released.
func sortRanked(ms []rankedModel) []ModelDef {
	sort.SliceStable(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if a.version != b.version {
			return a.version > b.version
		}
		if a.tier != b.tier {
			return a.tier > b.tier
		}
		if a.preview != b.preview {
			return !a.preview
		}
		if a.created != b.created {
			return a.created > b.created
		}
		return a.def.ID < b.def.ID
	})
	out := make([]ModelDef, 0, len(ms))
	seen := map[string]bool{}
	for _, m := range ms {
		if seen[m.def.ID] {
			continue
		}
		seen[m.def.ID] = true
		out = append(out, m.def)
	}
	return out
}

// versionAfter parses a "4", "4-1" or "4.1" generation number that follows
// prefix in id ("claude-opus-4-1-20250805" with prefix "claude-opus-" → 4.1).
// A two-digit-or-shorter minor is only taken when a hyphen or the end follows,
// so date suffixes such as "-0709" or "-20250514" are not read as versions.
var versionRe = regexp.MustCompile(`^(\d+)(?:[.-](\d{1,2}))?(?:-|$)`)

func versionAfter(id, prefix string) float64 {
	rest := strings.TrimPrefix(id, prefix)
	m := versionRe.FindStringSubmatch(rest)
	if m == nil {
		return 0
	}
	v := m[1]
	if m[2] != "" {
		v += "." + m[2]
	}
	f, _ := strconv.ParseFloat(v, 64)
	return f
}

// modelTier ranks a model within its generation: higher is more capable.
func modelTier(providerID, id string) int {
	id = strings.ToLower(id)
	switch providerID {
	case "anthropic":
		switch {
		case strings.Contains(id, "opus"):
			return 3
		case strings.Contains(id, "haiku"):
			return 1
		}
		return 2
	case "openai":
		switch {
		case strings.Contains(id, "nano"):
			return 0
		case strings.Contains(id, "mini"):
			return 1
		case strings.Contains(id, "chat"):
			return 2
		}
		return 3
	case "gemini":
		switch {
		case strings.Contains(id, "ultra"):
			return 4
		case strings.Contains(id, "pro"):
			return 3
		case strings.Contains(id, "lite"):
			return 1
		}
		return 2
	case "grok":
		switch {
		case strings.Contains(id, "mini"):
			return 1
		case strings.Contains(id, "fast"):
			return 2
		}
		return 3
	}
	return 2
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

var (
	datedSuffixRe   = regexp.MustCompile(`-(\d{4}-\d{2}-\d{2}|\d{8}|\d{4}|\d{3}|\d{2}-\d{2}|\d{2}-\d{4})$`)
	openaiVersionRe = regexp.MustCompile(`^gpt-(\d+(?:\.\d+)?)(o)?`)
	oSeriesRe       = regexp.MustCompile(`^o(\d+)`)
)

// prettyModelName turns "gpt-5.1-chat-latest" into "GPT-5.1 Chat Latest",
// "o4-mini" into "o4 Mini" and "grok-4-1-fast-reasoning" into
// "Grok 4.1 Fast Reasoning", for providers whose model list has no display names.
func prettyModelName(id string, version float64) string {
	lid := strings.ToLower(id)
	var head, rest string
	switch {
	case strings.HasPrefix(lid, "grok-"):
		r := strings.TrimPrefix(lid, "grok-")
		head, rest = "Grok", r
		if loc := versionRe.FindStringIndex(r); loc != nil && version > 0 {
			head = "Grok " + strconv.FormatFloat(version, 'f', -1, 64)
			rest = r[loc[1]:]
		}
	case strings.HasPrefix(lid, "gpt-"):
		r := strings.TrimPrefix(lid, "gpt-")
		head, rest = "GPT-"+r, ""
		if dash := strings.IndexByte(r, '-'); dash >= 0 {
			head, rest = "GPT-"+r[:dash], r[dash+1:]
		}
	default:
		head = lid
		if dash := strings.IndexByte(lid, '-'); dash >= 0 {
			head, rest = lid[:dash], lid[dash+1:]
		}
	}
	words := []string{head}
	for _, w := range strings.Split(rest, "-") {
		if w != "" {
			words = append(words, strings.ToUpper(w[:1])+w[1:])
		}
	}
	return strings.Join(words, " ")
}

// ── Provider implementations ─────────────────────────────────────────────

// getProviderJSON performs a GET against a provider API and decodes the JSON.
func getProviderJSON(req *http.Request, provider string, out any) error {
	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return &ProviderHTTPError{Provider: provider, Status: resp.StatusCode, Body: truncateStr(string(body), 500)}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing %s model list: %w", provider, err)
	}
	return nil
}

// ListModels asks Anthropic which Claude models the key can use.
func (p *AnthropicProvider) ListModels(ctx context.Context, apiKey string) ([]ModelDef, error) {
	var ranked []rankedModel
	after := ""
	for page := 0; page < 5; page++ {
		u := anthropicAPIBase + "/v1/models?limit=1000"
		if after != "" {
			u += "&after_id=" + url.QueryEscape(after)
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		var body struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
				CreatedAt   string `json:"created_at"`
			} `json:"data"`
			HasMore bool   `json:"has_more"`
			LastID  string `json:"last_id"`
		}
		if err := getProviderJSON(req, "Claude", &body); err != nil {
			return nil, err
		}
		for _, m := range body.Data {
			if !strings.HasPrefix(m.ID, "claude-") {
				continue
			}
			rest := strings.TrimPrefix(m.ID, "claude-")
			var version float64
			if rest != "" && rest[0] >= '0' && rest[0] <= '9' {
				version = versionAfter(rest, "") // older "claude-3-5-sonnet" naming
			} else if dash := strings.IndexByte(rest, '-'); dash > 0 {
				version = versionAfter(rest[dash+1:], "")
			}
			name := m.DisplayName
			if name == "" {
				name = m.ID
			}
			var created int64
			if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
				created = t.Unix()
			}
			ranked = append(ranked, rankedModel{
				def:     ModelDef{ID: m.ID, Name: name},
				version: version,
				tier:    modelTier("anthropic", m.ID),
				created: created,
			})
		}
		if !body.HasMore || body.LastID == "" {
			break
		}
		after = body.LastID
	}
	return sortRanked(ranked), nil
}

// ListModels asks OpenAI which chat models the key can use.
func (p *OpenAIProvider) ListModels(ctx context.Context, apiKey string) ([]ModelDef, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", openaiAPIBase+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	if err := getProviderJSON(req, "OpenAI", &body); err != nil {
		return nil, err
	}
	var ranked []rankedModel
	for _, m := range body.Data {
		id := strings.ToLower(m.ID)
		if !openaiVersionRe.MatchString(id) && !oSeriesRe.MatchString(id) {
			continue
		}
		if containsAny(id, "audio", "realtime", "transcribe", "tts", "image", "search", "embedding", "instruct",
			"moderation", "codex", "computer-use", "deep-research", "-pro", "oss", "gpt-3.5", "gpt-4-", "preview") ||
			id == "gpt-4" || datedSuffixRe.MatchString(id) {
			continue
		}
		var version float64
		if mm := openaiVersionRe.FindStringSubmatch(id); mm != nil {
			version, _ = strconv.ParseFloat(mm[1], 64)
		} else if mm := oSeriesRe.FindStringSubmatch(id); mm != nil {
			n, _ := strconv.Atoi(mm[1])
			version = 4.1 + float64(n)*0.1 // o1…o4 came between GPT-4.1 and GPT-5
		}
		ranked = append(ranked, rankedModel{
			def:     ModelDef{ID: m.ID, Name: prettyModelName(m.ID, version)},
			version: version,
			tier:    modelTier("openai", id),
			created: m.Created,
		})
	}
	return sortRanked(ranked), nil
}

// ListModels asks Google which Gemini models the key can use.
func (p *GeminiProvider) ListModels(ctx context.Context, apiKey string) ([]ModelDef, error) {
	var ranked []rankedModel
	pageToken := ""
	for page := 0; page < 5; page++ {
		u := geminiAPIBase + "/v1beta/models?pageSize=1000"
		if pageToken != "" {
			u += "&pageToken=" + url.QueryEscape(pageToken)
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-goog-api-key", apiKey)
		var body struct {
			Models []struct {
				Name                       string   `json:"name"`
				DisplayName                string   `json:"displayName"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := getProviderJSON(req, "Gemini", &body); err != nil {
			return nil, err
		}
		for _, m := range body.Models {
			id := strings.TrimPrefix(m.Name, "models/")
			lid := strings.ToLower(id)
			canGenerate := false
			for _, method := range m.SupportedGenerationMethods {
				if method == "generateContent" {
					canGenerate = true
				}
			}
			if !canGenerate || !strings.HasPrefix(lid, "gemini-") {
				continue
			}
			if containsAny(lid, "embedding", "tts", "image", "live", "audio", "native", "robotics", "computer-use",
				"aqa", "learnlm", "-exp", "latest") || datedSuffixRe.MatchString(lid) {
				continue
			}
			version := versionAfter(lid, "gemini-")
			if version == 0 {
				continue
			}
			name := m.DisplayName
			if name == "" {
				name = id
			}
			ranked = append(ranked, rankedModel{
				def:     ModelDef{ID: id, Name: name},
				version: version,
				tier:    modelTier("gemini", lid),
				preview: strings.Contains(lid, "preview"),
			})
		}
		if body.NextPageToken == "" {
			break
		}
		pageToken = body.NextPageToken
	}
	return sortRanked(ranked), nil
}

// ListModels asks xAI which Grok models the key can use.
func (p *GrokProvider) ListModels(ctx context.Context, apiKey string) ([]ModelDef, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", grokAPIBase+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	if err := getProviderJSON(req, "Grok", &body); err != nil {
		return nil, err
	}
	var ranked []rankedModel
	for _, m := range body.Data {
		id := strings.ToLower(m.ID)
		if !strings.HasPrefix(id, "grok-") || containsAny(id, "image", "imagine", "vision", "embed", "code", "video", "beta") {
			continue
		}
		version := versionAfter(id, "grok-")
		if version < 3 {
			continue
		}
		ranked = append(ranked, rankedModel{
			def:     ModelDef{ID: m.ID, Name: prettyModelName(id, version)},
			version: version,
			tier:    modelTier("grok", id),
			created: m.Created,
		})
	}
	return sortRanked(ranked), nil
}

// ── Output budgets for reasoning models ──────────────────────────────────

// reasoningHeadroom is added to the output budget for models that think before
// answering, whose reasoning tokens count against the same limit; without it a
// short budget can be used up before any answer text is written.
const reasoningHeadroom = 12000

func isOpenAIReasoningModel(model string) bool {
	m := strings.ToLower(model)
	if strings.Contains(m, "chat") {
		return false
	}
	return oSeriesRe.MatchString(m) || strings.HasPrefix(m, "gpt-5")
}

func isGeminiThinkingModel(model string) bool {
	return versionAfter(strings.ToLower(model), "gemini-") >= 2.5
}

func isGrokReasoningModel(model string) bool {
	m := strings.ToLower(model)
	if strings.Contains(m, "non-reasoning") {
		return false
	}
	return versionAfter(m, "grok-") >= 4 || strings.Contains(m, "mini") || strings.Contains(m, "reasoning")
}
