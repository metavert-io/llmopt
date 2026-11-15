package main

// Scoring an LLM brand test. Each query can be asked several times (AI
// answers vary from run to run, so one run can mislead) and once more with
// web search. Scores are rates over all the runs, with the range between runs
// and how consistent the mentions were; an answer's position in a list is not
// scored, because list order changes from run to run.

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

const (
	// maxTestRuns is the most times one test asks each query.
	maxTestRuns = 5
	// evalBatchSize is how many answers one evaluation call grades, so the
	// evaluator's input and output stay well inside its limits.
	evalBatchSize = 30
	// evalParallel is how many evaluation calls run at once.
	evalParallel = 3
	// topCitedDomainsShown is how many cited sites a test keeps.
	topCitedDomainsShown = 12
)

// clampTestRuns returns runs within 1..maxTestRuns (0 = once).
func clampTestRuns(runs int) int {
	return max(1, min(maxTestRuns, runs))
}

// responseID reads an evaluator's response id, given as 12, "12" or "R12".
type responseID struct {
	set bool
	n   int
}

func (r *responseID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	s = strings.TrimLeft(s, "Rr#")
	if n, err := strconv.Atoi(s); err == nil {
		r.set, r.n = true, n
	}
	return nil // an unreadable id falls back to query and provider
}

// testEvalEntry is the evaluator's verdict on one answer.
type testEvalEntry struct {
	ResponseID  responseID `json:"response_id"`
	QueryIndex  int        `json:"query_index"`
	ProviderID  string     `json:"provider_id"`
	Mentioned   bool       `json:"mentioned"`
	Recommended bool       `json:"recommended"`
	Sentiment   string     `json:"sentiment"`
	Accuracy    string     `json:"accuracy"`
	Score       int        `json:"score"`
}

// testEvaluation is the evaluator's verdict on a batch of answers.
type testEvaluation struct {
	Evaluations []testEvalEntry `json:"evaluations"`
}

// evalBatches splits the answered responses into batches for evaluation.
func evalBatches(responses []testRawResponse) [][]testRawResponse {
	var answered []testRawResponse
	for _, r := range responses {
		if r.err == nil {
			answered = append(answered, r)
		}
	}
	var batches [][]testRawResponse
	for len(answered) > 0 {
		n := min(evalBatchSize, len(answered))
		batches = append(batches, answered[:n])
		answered = answered[n:]
	}
	return batches
}

// matchEvaluations maps each verdict to the answer it grades: by response id,
// or (when the evaluator left the id out) by query and provider, taking that
// pair's answers in order. idByName maps provider names and IDs (lowercased)
// to provider IDs. The result is keyed by response id.
func matchEvaluations(batch []testRawResponse, evals []testEvalEntry, idByName map[string]string) map[int]testEvalEntry {
	inBatch := map[int]bool{}
	type pair struct {
		qi  int
		pid string
	}
	queue := map[pair][]int{}
	for _, r := range batch {
		inBatch[r.id] = true
		queue[pair{r.queryIdx, r.providerID}] = append(queue[pair{r.queryIdx, r.providerID}], r.id)
	}
	out := map[int]testEvalEntry{}
	take := func(id int, e testEvalEntry) {
		if _, done := out[id]; !done {
			out[id] = e
		}
	}
	var rest []testEvalEntry
	for _, e := range evals {
		if e.ResponseID.set && inBatch[e.ResponseID.n] {
			take(e.ResponseID.n, e)
			continue
		}
		rest = append(rest, e)
	}
	for _, e := range rest {
		pid, ok := idByName[strings.ToLower(strings.TrimSpace(e.ProviderID))]
		if !ok {
			continue
		}
		for _, id := range queue[pair{e.QueryIndex, pid}] {
			if _, done := out[id]; !done {
				take(id, e)
				break
			}
		}
	}
	return out
}

// Stages of a web-search answer (see LLMTestStages).
const (
	stageNotFound            = "not_found"
	stageFoundNotNamed       = "found_not_named"
	stageNamedNotRecommended = "named_not_recommended"
	stageRecommended         = "recommended"
)

// alnum keeps only lowercase letters and digits.
func alnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// searchStage says how far the brand got in a graded web-search answer:
// recommended, named but not recommended, found (its own site, or a page whose
// address names it, among the sources) but not named, or not found.
func searchStage(pr LLMTestProviderResult, domain, brandName string) string {
	switch {
	case pr.Recommended:
		return stageRecommended
	case pr.Mentioned:
		return stageNamedNotRecommended
	case pr.CitesOwnSite:
		return stageFoundNotNamed
	}
	var keys []string
	if k := alnum(brandName); len(k) >= 4 {
		keys = append(keys, k)
	}
	if label, _, _ := strings.Cut(strings.TrimPrefix(strings.ToLower(domain), "www."), "."); len(alnum(label)) >= 5 {
		keys = append(keys, alnum(label))
	}
	for _, u := range pr.Citations {
		addr := alnum(strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://"))
		for _, k := range keys {
			if strings.Contains(addr, k) {
				return stageFoundNotNamed
			}
		}
	}
	return stageNotFound
}

func (s *LLMTestStages) count(stage string) {
	switch stage {
	case stageRecommended:
		s.Recommended++
	case stageNamedNotRecommended:
		s.NamedNotRecommended++
	case stageFoundNotNamed:
		s.FoundNotNamed++
	case stageNotFound:
		s.NotFound++
	}
}

// testProviderInfo is what the summaries need to know about one provider.
type testProviderInfo struct {
	id, name, model      string
	failed, searchFailed int
}

// testSummaries is everything a test's scores are made of.
type testSummaries struct {
	providers       []LLMTestSummary
	overall         int
	scoreRange      []int
	searchScore     *int
	topCitedDomains []LLMTestCitedDomain
	diagnosis       *LLMTestSearchDiagnosis
}

type testTally struct{ n, score, mentioned, recommended, accurate, sentiment int }

func (t *testTally) add(pr LLMTestProviderResult) {
	t.n++
	t.score += pr.Score
	if pr.Mentioned {
		t.mentioned++
	}
	if pr.Recommended {
		t.recommended++
	}
	if pr.Accuracy == "accurate" {
		t.accurate++
	}
	switch pr.Sentiment {
	case "positive":
		t.sentiment += 100
	case "neutral":
		t.sentiment += 50
	case "negative":
		t.sentiment += 10
	}
}

func (t testTally) rate(k int) int { return k * 100 / t.n }

// summarizeTest scores the graded answers (results, grouped by query) for each
// provider: rates over every run, the range between runs, mention
// consistency, the web-search pass and the sites it cited.
func summarizeTest(providers []testProviderInfo, results []LLMTestQueryResult, runs int, domain string) testSummaries {
	var out testSummaries
	runs = clampTestRuns(runs)
	runScores := make([][]int, runs+1) // per run: each provider's score that run
	cited := map[string]int{}
	providerTotal, searchTotal, searchProviders := 0, 0, 0
	var diag LLMTestSearchDiagnosis

	for _, p := range providers {
		var all, search testTally
		perRun := make([]testTally, runs+1)
		mentionsByQuery := map[int][]bool{}
		citesOwn, searched := 0, 0
		var stages LLMTestStages
		var own LLMTestOwnSiteSplit
		for qi, qr := range results {
			for _, pr := range qr.ProviderResults {
				if pr.ProviderID != p.id || pr.Unscored || strings.HasPrefix(pr.Response, "Error: ") {
					continue
				}
				if pr.Search {
					search.add(pr)
					stages.count(pr.Stage)
					if pr.Searched {
						searched++
					}
					if pr.CitesOwnSite {
						citesOwn++
						own.WithOwnSite++
						if pr.Mentioned {
							own.MentionedWithOwnSite++
						}
					} else {
						own.WithoutOwnSite++
						if pr.Mentioned {
							own.MentionedWithoutOwnSite++
						}
					}
					seen := map[string]bool{}
					for _, u := range pr.Citations {
						if h := urlHost(u); h != "" && !seen[h] {
							seen[h] = true
							cited[h]++
						}
					}
					continue
				}
				all.add(pr)
				if r := pr.Run; r >= 1 && r <= runs {
					perRun[r].add(pr)
				}
				mentionsByQuery[qi] = append(mentionsByQuery[qi], pr.Mentioned)
			}
		}
		s := LLMTestSummary{ProviderID: p.id, ProviderName: p.name, Model: p.model, Answered: all.n, Failed: p.failed}
		if all.n > 0 {
			s.OverallScore = all.score / all.n
			s.MentionRate = all.rate(all.mentioned)
			s.RecommendRate = all.rate(all.recommended)
			s.AccuracyRate = all.rate(all.accurate)
			s.SentimentScore = all.sentiment / all.n
		}
		if runs > 1 {
			var sp *LLMTestSpread
			for r := 1; r <= runs; r++ {
				t := perRun[r]
				if t.n == 0 {
					continue
				}
				m, rec, sc := t.rate(t.mentioned), t.rate(t.recommended), t.score/t.n
				runScores[r] = append(runScores[r], sc)
				if sp == nil {
					sp = &LLMTestSpread{MentionLow: m, MentionHigh: m, RecommendLow: rec, RecommendHigh: rec, ScoreLow: sc, ScoreHigh: sc}
					continue
				}
				sp.MentionLow, sp.MentionHigh = min(sp.MentionLow, m), max(sp.MentionHigh, m)
				sp.RecommendLow, sp.RecommendHigh = min(sp.RecommendLow, rec), max(sp.RecommendHigh, rec)
				sp.ScoreLow, sp.ScoreHigh = min(sp.ScoreLow, sc), max(sp.ScoreHigh, sc)
			}
			eligible, consistent := 0, 0
			for _, ms := range mentionsByQuery {
				if len(ms) < 2 {
					continue
				}
				eligible++
				same := true
				for _, m := range ms[1:] {
					if m != ms[0] {
						same = false
					}
				}
				if same {
					consistent++
				}
			}
			if sp != nil && eligible > 0 {
				sp.Consistency = consistent * 100 / eligible
				s.Spread = sp
			}
		}
		if search.n > 0 || p.searchFailed > 0 {
			ss := &LLMTestSearchSummary{Answered: search.n, Failed: p.searchFailed, Searched: searched, Stages: stages, OwnSite: own}
			diag.Answered += search.n
			diag.Searched += searched
			diag.Stages.add(stages)
			diag.OwnSite.add(own)
			if search.n > 0 {
				ss.OverallScore = search.score / search.n
				ss.MentionRate = search.rate(search.mentioned)
				ss.RecommendRate = search.rate(search.recommended)
				ss.OwnSiteRate = search.rate(citesOwn)
				searchTotal += ss.OverallScore
				searchProviders++
			}
			s.Search = ss
		}
		providerTotal += s.OverallScore
		out.providers = append(out.providers, s)
	}
	if len(out.providers) > 0 {
		out.overall = providerTotal / len(out.providers)
	}
	if runs > 1 {
		lo, hi, any := 0, 0, false
		for r := 1; r <= runs; r++ {
			if len(runScores[r]) == 0 {
				continue
			}
			sum := 0
			for _, v := range runScores[r] {
				sum += v
			}
			avg := sum / len(runScores[r])
			if !any {
				lo, hi, any = avg, avg, true
			}
			lo, hi = min(lo, avg), max(hi, avg)
		}
		if any {
			out.scoreRange = []int{lo, hi}
		}
	}
	if searchProviders > 0 {
		v := searchTotal / searchProviders
		out.searchScore = &v
		out.diagnosis = &diag
	}
	for h, n := range cited {
		out.topCitedDomains = append(out.topCitedDomains, LLMTestCitedDomain{Domain: h, Answers: n, Own: isOwnSite("https://"+h, domain)})
	}
	sort.Slice(out.topCitedDomains, func(i, j int) bool {
		a, b := out.topCitedDomains[i], out.topCitedDomains[j]
		if a.Answers != b.Answers {
			return a.Answers > b.Answers
		}
		return a.Domain < b.Domain
	})
	if len(out.topCitedDomains) > topCitedDomainsShown {
		out.topCitedDomains = out.topCitedDomains[:topCitedDomainsShown]
	}
	return out
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
	return errEvalNotJSON
}

// sortedKeys returns a set's keys in order.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
