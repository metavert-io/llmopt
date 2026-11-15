package main

// Scoring weights from the 2026 research, applied when scores are read.
//
// Search Visibility's overall score is recomputed from its pillars each time a
// search analysis is read (SearchVisibilityResult.UnmarshalBSON), and the LLM
// Visibility Score is weighed with the weights below on every request. Results
// scored before version 2 also get the two pillar changes the research calls
// for (schema counts little; blocking training crawlers costs nothing) as
// adjustments to the model's own pillar scores, which keep the rest of its
// judgment (e.g. a robots.txt it couldn't read).
// Changing a weight here therefore re-weights every stored analysis at once:
// the app's pages, the shared pages, the API, the executive summary and the
// report all show the new scoring without anything being re-run.
//
// What a pillar looks for is set by the prompts and only changes for analyses
// run after a change; each result records the rubric version it was scored
// with (rubric_version; absent = before version 2) so the app can say so.
//
// Version 2 (November 2026), from the 2026 research (see Supporting Research):
//   - LLM Visibility Score: the LLM Test (the only directly observed outcome)
//     and answer optimization weigh 25% each; Reddit drops to 15% as its share
//     of AI citations fell through 2026.
//   - Search Visibility: brand momentum weighs most (branded web mentions
//     correlate most strongly with AI visibility); AI Overview readiness less,
//     since only about 38% of AI Overview citations now come from the top 10.
//   - Schema markup counts for little in AI Overview readiness (no measurable
//     citation lift in 2026 tests).
//   - Blocking training crawlers is a business choice and no longer costs
//     points; blocking the crawlers that fetch pages for AI answers does.

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// scoringRubricVersion is the scoring rubric new analyses are scored with.
const scoringRubricVersion = 2

// visibilityComponents returns the LLM Visibility Score's dimensions with
// their weights (re-normalised over the dimensions a domain has).
func visibilityComponents() []visibilityComponent {
	return []visibilityComponent{
		{Name: "Optimization", Weight: 0.25},
		{Name: "Video Authority", Weight: 0.20},
		{Name: "Reddit Authority", Weight: 0.15},
		{Name: "Search Visibility", Weight: 0.15},
		{Name: "LLM Test", Weight: 0.25},
	}
}

// Search Visibility pillar weights, without and with Category Discovery.
var (
	searchWeights       = []float64{0.25, 0.20, 0.30, 0.25}       // AIO readiness, crawl, brand momentum, freshness
	searchWeightsWithCD = []float64{0.20, 0.15, 0.25, 0.20, 0.20} // … plus category discovery
	// AI Overview readiness's parts. Before version 2 they counted equally;
	// schema now counts little, and answer prominence and format most, as only
	// about 38% of AI Overview citations still come from the top 10.
	aioPartWeights    = []float64{0.25, 0.10, 0.30, 0.35} // organic presence, structured data, content format, answer prominence
	aioPartWeightsOld = []float64{0.25, 0.25, 0.25, 0.25}
	// The share of crawl accessibility that AI crawler access carries.
	crawlAccessWeight = 0.5
)

// crawlCapWhenSearchBotBlocked caps crawl accessibility when a major AI
// search crawler is blocked: pages it can't fetch can't be cited.
const crawlCapWhenSearchBotBlocked = 40

// searchPromptWeights gives the search prompt its pillar weights (AIO, crawl,
// momentum, freshness) and the weighted-average line, matching the weights above.
func searchPromptWeights(withCategories bool) (aio, crawl, momentum, freshness, line string) {
	if withCategories {
		return "20%", "15%", "25%", "20%",
			"AIO Readiness 20%, Crawl Accessibility 15%, Brand Momentum 25%, Content Freshness 20%, Category Discovery 20%"
	}
	return "25%", "20%", "30%", "25%",
		"AIO Readiness 25%, Crawl Accessibility 20%, Brand Momentum 30%, Content Freshness 25%"
}

func weightedF(scores []int, weights []float64) float64 {
	sum := 0.0
	for i, s := range scores {
		sum += float64(s) * weights[i]
	}
	return sum
}

func weighted(scores []int, weights []float64) int {
	return roundInt(weightedF(scores, weights))
}

func roundInt(f float64) int {
	if f < 0 {
		return -int(-f + 0.5)
	}
	return int(f + 0.5)
}

func clamp100(v int) int {
	return max(0, min(100, v))
}

func allZero(scores ...int) bool {
	for _, s := range scores {
		if s != 0 {
			return false
		}
	}
	return true
}

// rescore applies the current weights: results scored before version 2 first
// get the version 2 pillar adjustments (once: PillarsAdjusted guards against
// a re-saved result being adjusted again), then the overall score is the
// weighted average of the pillars. A result without pillars keeps its overall.
func (r *SearchVisibilityResult) rescore() {
	if r.RubricVersion < 2 && !r.PillarsAdjusted {
		r.adjustPillarsToV2()
		r.PillarsAdjusted = true
	}
	p := []int{r.AIOReadiness.Score, r.CrawlAccess.Score, r.BrandMomentum.Score, r.ContentFreshness.Score}
	w := searchWeights
	if r.CategoryDiscovery != nil {
		p, w = append(p, r.CategoryDiscovery.Score), searchWeightsWithCD
	}
	if !allZero(p...) {
		r.OverallScore = weighted(p, w)
	}
}

// adjustPillarsToV2 moves an older result's AI Overview readiness and crawl
// accessibility by what the version 2 criteria change, leaving the rest of
// the model's judgment in place.
func (r *SearchVisibilityResult) adjustPillarsToV2() {
	// Schema counts little: shift the score by the difference between the new
	// and the old (equal) weighting of its parts.
	a := &r.AIOReadiness
	if parts := []int{a.OrganicPresence, a.StructuredData, a.ContentFormat, a.AnswerProminence}; !allZero(parts...) && a.Score > 0 {
		a.Score = clamp100(a.Score + roundInt(weightedF(parts, aioPartWeights)-weightedF(parts, aioPartWeightsOld)))
	}
	r.CrawlAccess.adjustToV2()
}

// crawlerPurpose sorts crawlers by what blocking them costs. "search" crawlers
// fetch pages for AI answers and search results, so blocking them hides the
// site; "training" crawlers collect training data, and blocking them is a
// business choice with no penalty. Google-Extended controls Gemini training
// and grounding, and blocking it is associated with less AI Overview
// retrieval, so it counts half. Unknown names don't count either way.
func crawlerPurpose(name string) (purpose string, weight float64) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "oai-searchbot", "chatgpt-user", "claude-searchbot", "claude-user", "perplexitybot", "perplexity-user",
		"googlebot", "bingbot", "applebot", "duckassistbot":
		return "search", 1
	case "google-extended":
		return "search", 0.5
	case "gptbot", "claudebot", "anthropic-ai", "ccbot", "applebot-extended", "bytespider", "meta-externalagent", "cohere-ai":
		return "training", 0
	}
	return "", 0
}

// majorSearchCrawlers are the crawlers whose blocking caps crawl accessibility.
var majorSearchCrawlers = map[string]bool{
	"googlebot": true, "bingbot": true, "oai-searchbot": true, "perplexitybot": true, "claude-searchbot": true,
}

// adjustToV2 stops blocked training crawlers costing points: AI crawler
// access moves by the difference between the allowed share of the search
// crawlers and the allowed share of all the crawlers listed, and the pillar
// moves by that change times access's weight. Blocking a major search crawler
// caps the pillar. Without crawler details nothing changes.
func (c *CrawlAccessibilityPillar) adjustToV2() {
	var searchTotal, searchAllowed, allTotal, allAllowed float64
	blockedMajor := false
	for _, d := range c.CrawlerDetails {
		purpose, w := crawlerPurpose(d.Name)
		if purpose == "" {
			continue
		}
		allTotal++
		if d.Allowed {
			allAllowed++
		}
		if purpose != "search" {
			continue
		}
		searchTotal += w
		if d.Allowed {
			searchAllowed += w
		} else if majorSearchCrawlers[strings.ToLower(strings.TrimSpace(d.Name))] {
			blockedMajor = true
		}
	}
	if searchTotal == 0 || allTotal == 0 {
		return
	}
	delta := 100 * (searchAllowed/searchTotal - allAllowed/allTotal)
	c.AIBotAccess = clamp100(c.AIBotAccess + roundInt(delta))
	c.Score = clamp100(c.Score + roundInt(crawlAccessWeight*delta))
	if blockedMajor && c.Score > crawlCapWhenSearchBotBlocked {
		c.Score = crawlCapWhenSearchBotBlocked
	}
}

// UnmarshalBSON decodes a stored search result and applies the current weights.
func (r *SearchVisibilityResult) UnmarshalBSON(data []byte) error {
	type plain SearchVisibilityResult // no methods: no recursion
	if err := bson.Unmarshal(data, (*plain)(r)); err != nil {
		return err
	}
	r.rescore()
	return nil
}

// searchScoreProjection lists the stored fields the search score is computed
// from, under prefix (e.g. "result.").
func searchScoreProjection(prefix string) bson.D {
	fields := []string{
		"overallScore", "rubricVersion", "pillarsAdjusted",
		"aioReadiness.score", "aioReadiness.organicPresence", "aioReadiness.structuredData",
		"aioReadiness.contentFormat", "aioReadiness.answerProminence",
		"crawlAccessibility.score", "crawlAccessibility.aiBotAccess", "crawlAccessibility.sitemapQuality",
		"crawlAccessibility.renderAccessibility", "crawlAccessibility.crawlerDetails",
		"brandMomentum.score", "contentFreshness.score", "categoryDiscovery.score",
	}
	p := make(bson.D, 0, len(fields))
	for _, f := range fields {
		p = append(p, bson.E{Key: prefix + f, Value: 1})
	}
	return p
}

// rescoredSearchDoc returns a stored search analysis as a generic document
// (as the API returns it) with its result rescored.
func rescoredSearchDoc(raw bson.Raw) (bson.M, error) {
	var doc bson.M
	if err := bson.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var sa SearchAnalysis
	if err := bson.Unmarshal(raw, &sa); err != nil || sa.Result == nil {
		return doc, nil
	}
	b, err := bson.Marshal(sa.Result)
	if err != nil {
		return doc, nil
	}
	var res bson.M
	if bson.Unmarshal(b, &res) == nil {
		doc["result"] = res
	}
	return doc, nil
}

// visibilityScoreFor loads each dimension's latest score for one domain;
// base selects the domain (and tenant).
func visibilityScoreFor(ctx context.Context, mongoDB *MongoDB, base bson.D) visibilityScore {
	components := visibilityComponents()
	own := func(extra ...bson.E) bson.D {
		return append(append(bson.D{}, base...), extra...)
	}
	overall := bson.D{{Key: "result.overallScore", Value: 1}}
	type scored struct {
		Result *struct {
			OverallScore int `bson:"overallScore"`
		} `bson:"result"`
	}

	// 1. Optimization: average over all of the domain's optimizations
	if cursor, err := mongoDB.Optimizations().Find(ctx, own(), options.Find().SetProjection(overall)); err == nil {
		var opts []scored
		if cursor.All(ctx, &opts) == nil && len(opts) > 0 {
			total := 0
			for _, o := range opts {
				if o.Result != nil {
					total += o.Result.OverallScore
				}
			}
			components[0].Score, components[0].Available = total/len(opts), true
		}
	}

	latest := func(projection bson.D) *options.FindOneOptions {
		return options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(projection)
	}
	// 2. Video authority (latest)
	var va scored
	if mongoDB.VideoAnalyses().FindOne(ctx, own(), latest(overall)).Decode(&va) == nil && va.Result != nil {
		components[1].Score, components[1].Available = va.Result.OverallScore, true
	}
	// 3. Reddit authority (latest)
	var ra scored
	if mongoDB.RedditAnalyses().FindOne(ctx, own(), latest(overall)).Decode(&ra) == nil && ra.Result != nil {
		components[2].Score, components[2].Available = ra.Result.OverallScore, true
	}
	// 4. Search visibility (latest), rescored from its pillars
	var sa struct {
		Result *SearchVisibilityResult `bson:"result"`
	}
	if mongoDB.SearchAnalyses().FindOne(ctx, own(), latest(searchScoreProjection("result."))).Decode(&sa) == nil && sa.Result != nil {
		components[3].Score, components[3].Available = sa.Result.OverallScore, true
	}
	// 5. LLM Test (latest, not a competitor test)
	var lt struct {
		OverallScore int `bson:"overallScore"`
	}
	if mongoDB.LLMTests().FindOne(ctx,
		own(bson.E{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}}),
		latest(bson.D{{Key: "overallScore", Value: 1}}),
	).Decode(&lt) == nil {
		components[4].Score, components[4].Available = lt.OverallScore, true
	}
	return weighVisibility(components)
}
