package main

// Per-provider state for an LLM brand test: which models to try, and whether a
// provider has turned out to be unusable so its remaining queries are skipped
// while the other providers carry on.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// maxModelSwitches bounds how many times one run moves to a provider's next
// model. Switching away from a model the key can't use costs one quick error,
// and a free Gemini key can't use several of the models listed first.
const maxModelSwitches = 6

// testCallsPerProvider is how many queries one provider answers at a time.
// Each provider has its own lanes, so a slow or rate-limited provider holds up
// only its own queries, and a key with a small per-minute quota isn't flooded.
const testCallsPerProvider = 3

// errTestOutOfTime marks a call the test's answer deadline cut short or never
// made, so a slow provider can't sink the whole test.
var errTestOutOfTime = errors.New("the test ran out of time before this call finished")

// llmTestTimes gives a test its total time and the part of it answers may use
// (the rest is kept for grading): 15 minutes per pass over the queries, at
// most an hour, with a fifth (3-10 minutes) for grading.
var llmTestTimes = func(passes int) (total, answers time.Duration) {
	total = time.Duration(passes) * 15 * time.Minute
	if total > time.Hour {
		total = time.Hour
	}
	reserve := total / 5
	if reserve < 3*time.Minute {
		reserve = 3 * time.Minute
	}
	if reserve > 10*time.Minute {
		reserve = 10 * time.Minute
	}
	return total, total - reserve
}

// maxRateLimitRetries is how many times one query is retried while the
// provider asks the run to slow down (quota per minute); other retryable
// errors use providerRetryDelays.
const maxRateLimitRetries = 4

var (
	// maxProviderPause caps the total time one run waits on a provider's rate
	// limit; past it the provider's remaining queries are skipped, so the run
	// still finishes well inside its time budget.
	maxProviderPause = 5 * time.Minute
	// pauseNoteMin is the shortest pause worth telling the user about.
	pauseNoteMin = 10 * time.Second
	// slowCallNote is how long one call may take before the user hears why
	// a provider's answers aren't arriving.
	slowCallNote = 75 * time.Second
)

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
	// searchAnswered and searchFailed count the web-search pass separately.
	searchAnswered int
	searchFailed   int
	pauseUntil     time.Time     // no calls before this: the provider asked the run to slow down
	paused         time.Duration // total pause so far
	slowNoted      bool
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

func (pr *testProviderRun) record(err error, search bool) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	switch {
	case search && err == nil:
		pr.searchAnswered++
	case search:
		pr.searchFailed++ // not lastErr: that explains why the provider couldn't answer at all
	case err == nil:
		pr.answered++
	default:
		pr.failed++
		pr.lastErr = err
	}
}

// canSearch reports whether the provider can answer with web search.
func (pr *testProviderRun) canSearch() bool {
	_, ok := pr.provider.(webSearchCaller)
	return ok
}

// ask sends one query. It moves to the provider's next model when the current
// one is retired, isn't available to the key, or (before it has answered
// anything) times out or stays rate-limited, and stops calling the provider
// once its key is known to be unusable. skipped is true when no call was made
// because of an earlier failure. With search, the query is answered with web
// search on the current model, once: a failed search doesn't move the
// provider to another model or stop its other queries.
func (pr *testProviderRun) ask(ctx context.Context, query string, maxTokens int, search bool) (resp webSearchAnswer, model ModelDef, err error, skipped bool) {
	for attempt := 0; attempt <= maxModelSwitches; attempt++ {
		pr.mu.Lock()
		model = pr.models[pr.modelIdx]
		dead := pr.deadErr
		pr.mu.Unlock()
		if dead != nil {
			return webSearchAnswer{}, model, dead, attempt == 0
		}

		resp, err = pr.call(ctx, model, query, maxTokens, search)
		if err == nil && strings.TrimSpace(resp.Text) == "" {
			// A blank answer would be scored as "brand not mentioned".
			err = fmt.Errorf("%s returned an empty answer", model.Name)
		}
		if err == nil {
			return resp, model, nil, false
		}
		if ctx.Err() != nil {
			return webSearchAnswer{}, model, err, false
		}
		if search {
			log.Printf("LLM test: %s (%s) web search failed: %s", pr.provider.Name(), model.ID, truncateStr(err.Error(), 400))
			return webSearchAnswer{}, model, err, false
		}
		log.Printf("LLM test: %s (%s) failed: %s", pr.provider.Name(), model.ID, truncateStr(err.Error(), 400))
		if pr.pausedTooLong() {
			pr.markDead(err)
			return webSearchAnswer{}, model, err, false
		}
		class := classifyProviderError(err)
		if (isTimeoutError(err) || isRateLimitQuota(err)) && pr.currentModel().ID != model.ID {
			// Another query already left this model (and may have answered on the
			// next one while this call was still waiting): retry on the new model.
			continue
		}
		if pr.answeredNone() && (isTimeoutError(err) || isRateLimitQuota(err)) {
			// A model that hasn't answered once in this run and doesn't answer in
			// time, or stays over its quota, is no use to this key today; the
			// provider's other models have their own capacity and quotas.
			class = failModel
		}
		switch class {
		case failKey:
			pr.markDead(err)
			return webSearchAnswer{}, model, err, false
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
				pr.notify(fmt.Sprintf("%s: %s; switching to %s.", pr.provider.Name(), switchReason(model, err), next.Name))
				continue
			}
			pr.mu.Unlock()
			pr.markDead(err)
			return webSearchAnswer{}, model, err, false
		default:
			return webSearchAnswer{}, model, err, false
		}
	}
	return webSearchAnswer{}, model, err, false
}

// call sends one query to model, waiting out any pause the provider asked for
// and retrying rate-limit and overload failures. A rate limit pauses all of
// this provider's queries, not just this one, so its lanes don't keep hitting
// a limit that has no room.
func (pr *testProviderRun) call(ctx context.Context, model ModelDef, query string, maxTokens int, search bool) (webSearchAnswer, error) {
	for attempt := 0; ; attempt++ {
		if err := pr.waitPause(ctx); err != nil {
			return webSearchAnswer{}, err
		}
		resp, err := pr.callNotingSlow(ctx, model, query, maxTokens, search)
		if err == nil || ctx.Err() != nil || classifyProviderError(err) != failRetry {
			return resp, err
		}
		retries := len(providerRetryDelays)
		if isRateLimitQuota(err) {
			retries = maxRateLimitRetries
		}
		if attempt >= retries || !pr.pause(retryWait(attempt, err), err) {
			return webSearchAnswer{}, err
		}
	}
}

// callNotingSlow makes one provider call and, if it takes longer than
// slowCallNote, tells the user once per run that this provider is slow. The
// note is sent before the call returns, never after the run has finished.
func (pr *testProviderRun) callNotingSlow(ctx context.Context, model ModelDef, query string, maxTokens int, search bool) (webSearchAnswer, error) {
	done := make(chan struct{})
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		t := time.NewTimer(slowCallNote)
		defer t.Stop()
		select {
		case <-done:
		case <-t.C:
			pr.noteSlow(model)
		}
	}()
	var resp webSearchAnswer
	var err error
	if ws, ok := pr.provider.(webSearchCaller); search && ok {
		resp, err = ws.CallWithWebSearch(ctx, pr.apiKey, model.ID, query, maxTokens)
	} else if search {
		err = fmt.Errorf("%s can't answer with web search here", pr.provider.Name())
	} else {
		resp.Text, err = pr.provider.Call(ctx, pr.apiKey, model.ID, query, maxTokens)
	}
	close(done)
	<-watched
	return resp, err
}

func (pr *testProviderRun) noteSlow(model ModelDef) {
	pr.mu.Lock()
	first := !pr.slowNoted && pr.deadErr == nil
	pr.slowNoted = true
	pr.mu.Unlock()
	if first {
		pr.notify(fmt.Sprintf("%s (%s) is slow to answer: one query has taken over a minute so far. Still waiting; the other providers carry on.",
			pr.provider.Name(), model.Name))
	}
}

// waitPause blocks until the provider's current pause is over.
func (pr *testProviderRun) waitPause(ctx context.Context) error {
	for {
		pr.mu.Lock()
		d := time.Until(pr.pauseUntil)
		dead := pr.deadErr
		pr.mu.Unlock()
		if dead != nil {
			return dead
		}
		if d <= 0 {
			return nil
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// pause holds all of this provider's queries for wait. It returns false once
// the run has spent maxProviderPause waiting on this provider.
func (pr *testProviderRun) pause(wait time.Duration, err error) bool {
	now := time.Now()
	until := now.Add(wait)
	pr.mu.Lock()
	started := !pr.pauseUntil.After(now)
	if until.After(pr.pauseUntil) {
		from := now
		if pr.pauseUntil.After(now) {
			from = pr.pauseUntil
		}
		pr.paused += until.Sub(from)
		pr.pauseUntil = until
	}
	over := pr.paused > maxProviderPause
	pr.mu.Unlock()
	if over {
		return false
	}
	if started && wait >= pauseNoteMin {
		detail := ""
		var qe *QuotaError
		if errors.As(err, &qe) && qe.Message != "" {
			detail = fmt.Sprintf(" (%s: %s)", qe.Provider, truncateStr(qe.Message, 200))
		}
		log.Printf("LLM test: %s rate-limited, pausing %s: %s", pr.provider.Name(), wait.Round(time.Second), truncateStr(err.Error(), 300))
		pr.notify(fmt.Sprintf("%s is rate-limiting this API key; pausing its queries for %s%s.",
			pr.provider.Name(), wait.Round(time.Second), detail))
	}
	return true
}

func (pr *testProviderRun) pausedTooLong() bool {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.paused > maxProviderPause
}

func (pr *testProviderRun) answeredNone() bool {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.answered == 0
}

// isTimeoutError reports a call that got no reply in time (the HTTP client's
// timeout), as opposed to the run's own deadline.
func isTimeoutError(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "Client.Timeout exceeded")
}

// isRateLimitQuota reports a quota that refills (requests per minute), as
// opposed to one the model doesn't have for this key at all.
func isRateLimitQuota(err error) bool {
	var qe *QuotaError
	return errors.As(err, &qe) && !qe.PerModel
}

// switchReason says why a run is leaving a model, for the progress notes.
func switchReason(model ModelDef, err error) string {
	switch {
	case isTimeoutError(err):
		return model.Name + " didn't answer in time"
	case isRateLimitQuota(err):
		return model.Name + " stayed over this API key's rate limit"
	}
	return model.Name + " isn't available to this API key"
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

// errEvalNotJSON: the evaluator's reply couldn't be read.
var errEvalNotJSON = fmt.Errorf("evaluation is not valid JSON")
