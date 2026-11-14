package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func setVar[T any](t *testing.T, p *T, v T) {
	orig := *p
	*p = v
	t.Cleanup(func() { *p = orig })
}

// fastAnthropic answers every query at once.
func fastAnthropic(n int, calls *int32) *TestProvider {
	p := newTestProvider()
	p.id, p.name = "anthropic", "Anthropic"
	p.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if isEvalPrompt(prompt) {
			return evalJSONFor(n, "anthropic", "gemini"), nil
		}
		if calls != nil {
			atomic.AddInt32(calls, 1)
		}
		return "Example Health is a good option.", nil
	}
	return p
}

func timeoutErr() error {
	return fmt.Errorf("request failed: %w", &url.Error{Op: "Post", URL: "https://example.invalid", Err: context.DeadlineExceeded})
}

func perMinuteQuota(retryAfter time.Duration) error {
	qe := geminiQuotaError([]byte(geminiPerMinute))
	qe.RetryAfter = retryAfter
	return qe
}

// A provider whose calls hang must not hold up another provider's queries,
// and never has more than testCallsPerProvider calls in flight.
func TestLLMTest_SlowProviderDoesNotHoldUpOthers(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "gemini")

	const n = 8
	gate := make(chan struct{})
	var once sync.Once
	open := func() { once.Do(func() { close(gate) }) }
	var anthropicCalls int32
	anth := fastAnthropic(n, &anthropicCalls)
	inner := anth.CallFn
	anth.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		resp, err := inner(ctx, apiKey, model, prompt, maxTokens)
		if atomic.LoadInt32(&anthropicCalls) == n {
			open() // every Anthropic query answered while Gemini was still stuck
		}
		return resp, err
	}
	var inFlight, maxInFlight int32
	gem := newTestProvider()
	gem.id, gem.name = "gemini", "Gemini"
	gem.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		cur := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)
		for {
			m := atomic.LoadInt32(&maxInFlight)
			if cur <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, cur) {
				break
			}
		}
		<-gate
		return "Example Health is a good option.", nil
	}
	withTestProviders(t, anth, gem)

	timedOut := false
	timer := time.AfterFunc(5*time.Second, func() { timedOut = true; open() })
	defer timer.Stop()
	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "gemini"}, "queries": testQueries(n),
	})
	if timedOut {
		t.Fatalf("Anthropic's queries waited on Gemini's stuck calls; events:\n%s", statusMessages(events))
	}
	if result == nil {
		t.Fatalf("no result:\n%s", statusMessages(events))
	}
	if m := atomic.LoadInt32(&maxInFlight); m > testCallsPerProvider {
		t.Errorf("Gemini had %d calls in flight; the cap is %d", m, testCallsPerProvider)
	}
}

// A per-minute rate limit pauses all of the provider's queries for as long as
// the provider asks, says so, and the queries then go through.
func TestLLMTest_RateLimitPausesProvider(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	setVar(t, &pauseNoteMin, 0)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "gemini")

	const n = 4
	const pause = 60 * time.Millisecond
	var mu sync.Mutex
	var calls int
	var firstLimit time.Time
	var early []time.Duration
	gem := newTestProvider()
	gem.id, gem.name = "gemini", "Gemini"
	gem.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls <= 2 {
			if firstLimit.IsZero() {
				firstLimit = time.Now()
			}
			return "", perMinuteQuota(pause)
		}
		if d := time.Since(firstLimit); d < pause-10*time.Millisecond {
			early = append(early, d)
		}
		return "Example Health is a good option.", nil
	}
	withTestProviders(t, fastAnthropic(n, nil), gem)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "gemini"}, "queries": testQueries(n),
	})
	if result == nil {
		t.Fatalf("no result:\n%s", statusMessages(events))
	}
	for _, s := range result.ProviderSummaries {
		if s.Answered != n {
			t.Errorf("%s answered %d of %d", s.ProviderID, s.Answered, n)
		}
	}
	if len(early) > 0 {
		t.Errorf("Gemini was called %v after its rate limit, inside the %v pause", early, pause)
	}
	msgs := statusMessages(events)
	if !strings.Contains(msgs, "Gemini is rate-limiting this API key; pausing its queries") || !strings.Contains(msgs, "limit: 10") {
		t.Errorf("no pause note with Google's message:\n%s", msgs)
	}
}

// A model that hasn't answered once and times out is left for the next one.
func TestLLMTest_SilentModelSwitchesToNext(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "gemini")

	const n = 4
	var stuckCalls int32
	gem := newTestProvider()
	gem.id, gem.name = "gemini", "Gemini"
	gem.models = []ModelDef{{ID: "gemini-9-flash", Name: "Gemini 9 Flash"}, {ID: "gemini-9-pro", Name: "Gemini 9 Pro"}}
	gem.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if model == "gemini-9-flash" {
			atomic.AddInt32(&stuckCalls, 1)
			return "", timeoutErr()
		}
		return "Example Health is a good option.", nil
	}
	withTestProviders(t, fastAnthropic(n, nil), gem)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "gemini"}, "queries": testQueries(n),
	})
	if result == nil {
		t.Fatalf("no result:\n%s", statusMessages(events))
	}
	if len(result.UnavailableProviders) != 0 {
		t.Errorf("unavailable: %+v", result.UnavailableProviders)
	}
	for _, s := range result.ProviderSummaries {
		if s.ProviderID == "gemini" && s.Answered != n {
			t.Errorf("gemini answered %d of %d", s.Answered, n)
		}
	}
	if c := atomic.LoadInt32(&stuckCalls); c > testCallsPerProvider {
		t.Errorf("the silent model was tried %d times", c)
	}
	if msgs := statusMessages(events); !strings.Contains(msgs, "Gemini 9 Flash didn't answer in time; switching to Gemini 9 Pro") {
		t.Errorf("no switch note:\n%s", msgs)
	}
}

// A provider that keeps rate-limiting after it has answered is given up on once
// the run has waited maxProviderPause for it; the run still finishes.
func TestLLMTest_EndlessRateLimitGivesUp(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	setVar(t, &maxProviderPause, 100*time.Millisecond)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "gemini")

	const n = 10
	var calls int32
	gem := newTestProvider()
	gem.id, gem.name = "gemini", "Gemini"
	gem.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return "Example Health is a good option.", nil
		}
		return "", perMinuteQuota(30 * time.Millisecond)
	}
	withTestProviders(t, fastAnthropic(n, nil), gem)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "gemini"}, "queries": testQueries(n),
	})
	if result == nil {
		t.Fatalf("no result:\n%s", statusMessages(events))
	}
	if c := atomic.LoadInt32(&calls); c > 12 {
		t.Errorf("Gemini was called %d times after its pause budget ran out", c)
	}
	if msgs := statusMessages(events); !strings.Contains(msgs, "Gemini can't be used: its API stayed rate-limited") {
		t.Errorf("no give-up note:\n%s", msgs)
	}
}

// A call that takes longer than slowCallNote says so, once.
func TestLLMTest_SlowCallIsNoted(t *testing.T) {
	db := testMongoDB(t)
	shortRetries(t)
	setVar(t, &slowCallNote, 20*time.Millisecond)
	insertTestTenantKey(t, db, "anthropic")
	insertTestTenantKey(t, db, "gemini")

	const n = 3
	gem := newTestProvider()
	gem.id, gem.name = "gemini", "Gemini"
	gem.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		time.Sleep(60 * time.Millisecond)
		return "Example Health is a good option.", nil
	}
	withTestProviders(t, fastAnthropic(n, nil), gem)

	events, result := runLLMTestRequest(t, db, true, map[string]any{
		"domain": "example.com", "providers": []string{"anthropic", "gemini"}, "queries": testQueries(n),
	})
	if result == nil {
		t.Fatalf("no result:\n%s", statusMessages(events))
	}
	if c := strings.Count(statusMessages(events), "is slow to answer"); c != 1 {
		t.Errorf("slow note sent %d times:\n%s", c, statusMessages(events))
	}
}

func TestSwitchReasonsAndTimeouts(t *testing.T) {
	m := ModelDef{Name: "Gemini 9 Flash"}
	real := fmt.Errorf(`request failed: Post "https://x": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`)
	for _, err := range []error{timeoutErr(), real} {
		if !isTimeoutError(err) || switchReason(m, err) != "Gemini 9 Flash didn't answer in time" {
			t.Errorf("%v: timeout=%v reason=%q", err, isTimeoutError(err), switchReason(m, err))
		}
		if r := providerFailureReason(err); !strings.HasPrefix(r, "its models didn't answer in time") {
			t.Errorf("failure reason %q", r)
		}
	}
	if r := switchReason(m, perMinuteQuota(time.Second)); r != "Gemini 9 Flash stayed over this API key's rate limit" {
		t.Errorf("rate-limit reason %q", r)
	}
	if r := switchReason(m, geminiQuotaError([]byte(geminiNoFreeQuota))); r != "Gemini 9 Flash isn't available to this API key" {
		t.Errorf("no-quota reason %q", r)
	}
	if isTimeoutError(&ProviderHTTPError{Provider: "Gemini", Status: 500, Body: "boom"}) {
		t.Error("an HTTP error is not a timeout")
	}
}
