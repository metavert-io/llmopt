package main

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// A search result stored before rubric version 2: the model's own pillar
// scores, GPTBot and ClaudeBot (training crawlers) blocked.
func oldSearchResultDoc(overall int) bson.M {
	return bson.M{
		"overallScore": overall,
		"aioReadiness": bson.M{
			"score": 55, "organicPresence": 60, "structuredData": 20, "contentFormat": 70, "answerProminence": 80,
		},
		"crawlAccessibility": bson.M{
			"score": 45, "aiBotAccess": 40, "sitemapQuality": 80, "renderAccessibility": 60,
			"crawlerDetails": bson.A{
				bson.M{"name": "GPTBot", "allowed": false},
				bson.M{"name": "ClaudeBot", "allowed": false},
				bson.M{"name": "PerplexityBot", "allowed": true},
				bson.M{"name": "Google-Extended", "allowed": true},
				bson.M{"name": "Googlebot", "allowed": true},
			},
		},
		"brandMomentum":    bson.M{"score": 70},
		"contentFreshness": bson.M{"score": 50},
		"executiveSummary": "stored summary",
	}
}

func decodeSearchResult(t *testing.T, doc bson.M) SearchVisibilityResult {
	t.Helper()
	b, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var r SearchVisibilityResult
	if err := bson.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSearchRescore_OldResultReweighted(t *testing.T) {
	r := decodeSearchResult(t, oldSearchResultDoc(50))
	// Schema counts little: parts weighted .25/.10/.30/.35 = 66 vs equally 57.5 → 55 + 8.5 → 64
	if r.AIOReadiness.Score != 64 {
		t.Errorf("AIO readiness = %d, want 64", r.AIOReadiness.Score)
	}
	// Blocked training crawlers stop costing points: search crawlers 100% allowed vs 3 of 5 listed → +40
	// access (40 → 80) and +20 on the pillar (45 → 65)
	if r.CrawlAccess.AIBotAccess != 80 || r.CrawlAccess.Score != 65 {
		t.Errorf("crawl = %d (bot access %d), want 65 (80)", r.CrawlAccess.Score, r.CrawlAccess.AIBotAccess)
	}
	// .25*64 + .20*65 + .30*70 + .25*50 = 62.5 → 63
	if r.OverallScore != 63 {
		t.Errorf("overall = %d, want 63", r.OverallScore)
	}
	if r.RubricVersion != 0 || !r.PillarsAdjusted {
		t.Errorf("an old result keeps rubric version 0 (got %d) and is marked adjusted (got %v)", r.RubricVersion, r.PillarsAdjusted)
	}
	if r.ExecutiveSummary != "stored summary" {
		t.Errorf("other fields must decode as stored, got %q", r.ExecutiveSummary)
	}
}

func TestSearchRescore_AdjustedOnce(t *testing.T) {
	first := decodeSearchResult(t, oldSearchResultDoc(50))
	b, err := bson.Marshal(first) // e.g. a copy saved back
	if err != nil {
		t.Fatal(err)
	}
	var again SearchVisibilityResult
	if err := bson.Unmarshal(b, &again); err != nil {
		t.Fatal(err)
	}
	if again.AIOReadiness.Score != first.AIOReadiness.Score || again.CrawlAccess.Score != first.CrawlAccess.Score || again.OverallScore != first.OverallScore {
		t.Errorf("re-reading an adjusted result changed it: %d/%d/%d → %d/%d/%d",
			first.AIOReadiness.Score, first.CrawlAccess.Score, first.OverallScore,
			again.AIOReadiness.Score, again.CrawlAccess.Score, again.OverallScore)
	}
}

// A result scored under version 2 keeps the model's pillars; only the
// overall is the weighted average.
func TestSearchRescore_CurrentRubricOnlyReweighs(t *testing.T) {
	doc := oldSearchResultDoc(10)
	doc["rubricVersion"] = scoringRubricVersion
	r := decodeSearchResult(t, doc)
	if r.AIOReadiness.Score != 55 || r.CrawlAccess.Score != 45 || r.CrawlAccess.AIBotAccess != 40 || r.PillarsAdjusted {
		t.Errorf("pillars changed: AIO %d, crawl %d, access %d, adjusted %v", r.AIOReadiness.Score, r.CrawlAccess.Score, r.CrawlAccess.AIBotAccess, r.PillarsAdjusted)
	}
	// .25*55 + .20*45 + .30*70 + .25*50 = 56.25 → 56
	if r.OverallScore != 56 {
		t.Errorf("overall = %d, want 56", r.OverallScore)
	}
}

func TestSearchRescore_SearchCrawlerBlockedCaps(t *testing.T) {
	doc := oldSearchResultDoc(50)
	doc["crawlAccessibility"].(bson.M)["score"] = 85
	doc["crawlAccessibility"].(bson.M)["crawlerDetails"] = bson.A{
		bson.M{"name": "GPTBot", "allowed": true},
		bson.M{"name": "PerplexityBot", "allowed": true},
		bson.M{"name": "Google-Extended", "allowed": true},
		bson.M{"name": "Googlebot", "allowed": false},
	}
	r := decodeSearchResult(t, doc)
	// Search crawlers 1.5 of 2.5 (60%) vs 3 of 4 listed (75%) → access 40 → 25, pillar 85 → 77,
	// then capped because Googlebot is blocked
	if r.CrawlAccess.AIBotAccess != 25 || r.CrawlAccess.Score != crawlCapWhenSearchBotBlocked {
		t.Errorf("crawl = %d (bot access %d), want %d (25)", r.CrawlAccess.Score, r.CrawlAccess.AIBotAccess, crawlCapWhenSearchBotBlocked)
	}
}

func TestSearchRescore_KeepsScoresWithoutParts(t *testing.T) {
	doc := bson.M{
		"overallScore":       33,
		"aioReadiness":       bson.M{"score": 50},
		"crawlAccessibility": bson.M{"score": 40, "aiBotAccess": 30},
		"brandMomentum":      bson.M{"score": 60},
		"contentFreshness":   bson.M{"score": 70},
		"categoryDiscovery":  bson.M{"score": 80},
	}
	r := decodeSearchResult(t, doc)
	if r.AIOReadiness.Score != 50 || r.CrawlAccess.Score != 40 || r.CrawlAccess.AIBotAccess != 30 {
		t.Errorf("pillars without parts must keep their stored scores: aio %d crawl %d access %d",
			r.AIOReadiness.Score, r.CrawlAccess.Score, r.CrawlAccess.AIBotAccess)
	}
	// With category discovery: .20*50 + .15*40 + .25*60 + .20*70 + .20*80 = 61
	if r.OverallScore != 61 {
		t.Errorf("overall = %d, want 61", r.OverallScore)
	}
}

func TestSearchRescore_EmptyResultKeepsOverall(t *testing.T) {
	r := decodeSearchResult(t, bson.M{"overallScore": 42})
	if r.OverallScore != 42 {
		t.Errorf("a result without pillars must keep its overall, got %d", r.OverallScore)
	}
	if r.CategoryDiscovery != nil {
		t.Error("missing category discovery must stay nil")
	}
}

func TestCrawlerPurpose(t *testing.T) {
	cases := map[string]string{
		"GPTBot": "training", "ClaudeBot": "training", "CCBot": "training",
		"OAI-SearchBot": "search", "ChatGPT-User": "search", "Claude-SearchBot": "search",
		"PerplexityBot": "search", " googlebot ": "search", "Bingbot": "search", "Google-Extended": "search",
		"SomeOtherBot": "",
	}
	for name, want := range cases {
		if got, _ := crawlerPurpose(name); got != want {
			t.Errorf("crawlerPurpose(%q) = %q, want %q", name, got, want)
		}
	}
	if _, w := crawlerPurpose("Google-Extended"); w != 0.5 {
		t.Errorf("Google-Extended weight = %v, want 0.5", w)
	}
}

// The prompt must ask the model for the same weights the app applies.
func TestSearchPromptWeightsMatchTables(t *testing.T) {
	parse := func(line string) []float64 {
		var out []float64
		for _, part := range strings.Split(line, ", ") {
			f := strings.Fields(part)
			n, err := strconv.Atoi(strings.TrimSuffix(f[len(f)-1], "%"))
			if err != nil {
				t.Fatalf("can't parse %q", part)
			}
			out = append(out, float64(n)/100)
		}
		return out
	}
	for _, withCD := range []bool{false, true} {
		aio, crawl, momentum, freshness, line := searchPromptWeights(withCD)
		want := searchWeights
		if withCD {
			want = searchWeightsWithCD
		}
		got := parse(line)
		if len(got) != len(want) {
			t.Fatalf("withCD=%v: %d weights in the prompt, %d in the table", withCD, len(got), len(want))
		}
		sum := 0.0
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("withCD=%v: weight %d is %v in the prompt, %v in the table", withCD, i, got[i], want[i])
			}
			sum += want[i]
		}
		if sum < 0.999 || sum > 1.001 {
			t.Errorf("withCD=%v: weights sum to %v", withCD, sum)
		}
		for i, s := range []string{aio, crawl, momentum, freshness} {
			if s != strconv.Itoa(int(want[i]*100+0.5))+"%" {
				t.Errorf("withCD=%v: pillar %d label %q doesn't match %v", withCD, i, s, want[i])
			}
		}
	}
	for _, ws := range [][]float64{aioPartWeights, aioPartWeightsOld} {
		sum := 0.0
		for _, w := range ws {
			sum += w
		}
		if sum < 0.999 || sum > 1.001 {
			t.Errorf("part weights %v sum to %v", ws, sum)
		}
	}
	sum := 0.0
	for _, c := range visibilityComponents() {
		sum += c.Weight
	}
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("visibility weights sum to %v", sum)
	}
}

// Stored analyses scored under the old rubric get the new weights wherever
// the LLM Visibility Score is computed, including the search projection.
func TestVisibilityScoreFor_RescoresStoredData(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	const tenant, domain = "test-tenant", "rescore.example"
	base := bson.M{"tenantId": tenant, "domain": domain}
	with := func(extra bson.M) bson.M {
		m := bson.M{}
		for k, v := range base {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	now := time.Now()
	mustInsert := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := db.Optimizations().InsertOne(ctx, with(bson.M{"result": bson.M{"overallScore": 80}}))
	mustInsert(err)
	_, err = db.VideoAnalyses().InsertOne(ctx, with(bson.M{"generatedAt": now, "result": bson.M{"overallScore": 60}}))
	mustInsert(err)
	_, err = db.RedditAnalyses().InsertOne(ctx, with(bson.M{"generatedAt": now, "result": bson.M{"overallScore": 40}}))
	mustInsert(err)
	_, err = db.SearchAnalyses().InsertOne(ctx, with(bson.M{"generatedAt": now, "result": oldSearchResultDoc(50)}))
	mustInsert(err)
	_, err = db.LLMTests().InsertOne(ctx, with(bson.M{"generatedAt": now, "overallScore": 70}))
	mustInsert(err)

	vs := visibilityScoreFor(ctx, db, bson.D{{Key: "tenantId", Value: tenant}, {Key: "domain", Value: domain}})
	if vs.Available != 5 {
		t.Fatalf("available = %d, want 5", vs.Available)
	}
	if got := vs.Components[3].Score; got != 63 {
		t.Errorf("search component = %d, want the rescored 63 (stored 50)", got)
	}
	// .25*80 + .20*60 + .15*40 + .15*63 + .25*70 = 64.95 → 64
	if vs.Score != 64 {
		t.Errorf("visibility = %d, want 64", vs.Score)
	}

	// The API's generic document carries the rescored result too.
	raw, err := db.SearchAnalyses().FindOne(ctx, base).Raw()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := rescoredSearchDoc(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, ok := doc["result"].(bson.M)
	if !ok {
		t.Fatalf("result is %T", doc["result"])
	}
	if res["overallScore"] != int32(63) || doc["domain"] != domain {
		t.Errorf("API document: overall %v (%T), domain %v", res["overallScore"], res["overallScore"], doc["domain"])
	}
}

func TestVisibilityScoreFor_NullSearchResult(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	if _, err := db.SearchAnalyses().InsertOne(ctx, bson.M{"tenantId": "t", "domain": "null.example", "generatedAt": time.Now(), "result": nil}); err != nil {
		t.Fatal(err)
	}
	vs := visibilityScoreFor(ctx, db, bson.D{{Key: "tenantId", Value: "t"}, {Key: "domain", Value: "null.example"}})
	if vs.Components[3].Available || vs.Available != 0 {
		t.Errorf("a search analysis without a result must not count: %+v", vs)
	}
}

func TestSummaryStale_OldRubric(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")
	recent := time.Now().Add(-time.Minute)
	if stale, _ := isSummaryStale(ctx, db, tenantCtx, "rubric.example", DomainSummary{Domain: "rubric.example", GeneratedAt: recent}); !stale {
		t.Error("a summary written before the 2026 rubric must be stale")
	}
	current := DomainSummary{Domain: "rubric.example", GeneratedAt: recent, RubricVersion: scoringRubricVersion}
	if stale, _ := isSummaryStale(ctx, db, tenantCtx, "rubric.example", current); stale {
		t.Error("a current summary with nothing newer must not be stale")
	}
	if _, err := db.SearchAnalyses().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "domain": "rubric.example", "generatedAt": time.Now()}); err != nil {
		t.Fatal(err)
	}
	if stale, _ := isSummaryStale(ctx, db, tenantCtx, "rubric.example", current); !stale {
		t.Error("a newer search analysis must make the summary stale")
	}
}

func TestDomainSummaryPrompt_IncludesScoringInputs(t *testing.T) {
	vs := weighVisibility([]visibilityComponent{
		{Name: "Optimization", Weight: 0.25, Score: 80, Available: true},
		{Name: "LLM Test", Weight: 0.25, Score: 60, Available: true},
		{Name: "Reddit Authority", Weight: 0.15},
	})
	extras := summaryExtras{
		Search: &SearchAnalysis{Result: &SearchVisibilityResult{OverallScore: 67, ExecutiveSummary: "search says hi",
			CategoryDiscovery: &CategoryDiscoveryPillar{Score: 12}}},
		LLMTest: &LLMTest{OverallScore: 60, Queries: make([]LLMTestQuery, 3),
			ProviderSummaries: []LLMTestSummary{{ProviderName: "Assistant A", OverallScore: 60, MentionRate: 50, RecommendRate: 25, AccuracyRate: 90}}},
		Visibility: &vs,
	}
	p := buildDomainSummaryPrompt("example.com", nil, nil, nil, nil, extras, BrandContextInfo{})
	for _, want := range []string{
		"=== LLM VISIBILITY SCORE ===", "Overall: 70/100", "- Optimization: 80/100 (weight 25%)", "- Reddit Authority: not run (weight 15%)",
		"=== SEARCH VISIBILITY ===", "Overall Search Visibility Score: 67/100", "Category Discovery: 12", "search says hi",
		"=== LLM TEST", "over 3 queries", "Assistant A: 60/100 (mentioned in 50%, recommended in 25%, accurate in 90%)",
		"Search Visibility, LLM Test",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
	if strings.Contains(p, "%!") {
		t.Error("prompt has a formatting error")
	}
}
