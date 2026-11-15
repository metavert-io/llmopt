package main

// Data behind the downloadable report and its web version: everything the
// report shows for one domain, loaded once, plus the LLM Visibility Score that
// the app's scorecard shows (computeVisibilityScore is shared with
// GET /api/visibility-score so both always agree).

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// reportData is everything one report is built from.
type reportData struct {
	Domain         string
	BrandProfile   *BrandProfile
	Analysis       *Analysis
	Optimizations  []Optimization
	VideoAnalysis  *VideoAnalysis
	RedditAnalysis *RedditAnalysis
	SearchAnalysis *SearchAnalysis
	LLMTest        *LLMTest
	Summary        *DomainSummary
	Todos          []TodoItem
	Visibility     visibilityScore
}

// visibilityComponent is one dimension of the LLM Visibility Score.
type visibilityComponent struct {
	Name      string  `json:"name"`
	Score     int     `json:"score"`
	Weight    float64 `json:"weight"`
	Available bool    `json:"available"`
}

// visibilityScore is the weighted LLM Visibility Score over the dimensions that
// have been run (weights re-normalised over the available ones).
type visibilityScore struct {
	Score      int                   `json:"score"`
	Components []visibilityComponent `json:"components"`
	Available  int                   `json:"available"`
	Total      int                   `json:"total"`
}

// weighVisibility fills in the overall score and counts from the components.
func weighVisibility(components []visibilityComponent) visibilityScore {
	totalWeight, weightedSum, available := 0.0, 0.0, 0
	for _, c := range components {
		if c.Available {
			totalWeight += c.Weight
			weightedSum += float64(c.Score) * c.Weight
			available++
		}
	}
	score := 0
	if totalWeight > 0 {
		score = int(weightedSum / totalWeight)
	}
	return visibilityScore{Score: score, Components: components, Available: available, Total: len(components)}
}

// computeVisibilityScore loads each dimension's latest score for the domain in
// the request's tenant (the same tenant scoping as everywhere else, so a server
// without SaaS sign-in finds its data too). Weights: see scoring.go.
func computeVisibilityScore(ctx context.Context, mongoDB *MongoDB, domain string, reqCtx context.Context) visibilityScore {
	return visibilityScoreFor(ctx, mongoDB, tenantFilter(reqCtx, bson.D{{Key: "domain", Value: domain}}))
}

// loadReportData loads everything the report needs. progress (may be nil) is
// told what is being loaded.
func loadReportData(ctx context.Context, mongoDB *MongoDB, domain string, reqCtx context.Context, progress func(string)) reportData {
	say := func(msg string) {
		if progress != nil {
			progress(msg)
		}
	}
	d := reportData{Domain: domain}
	own := func(extra ...bson.E) bson.D {
		return append(tenantFilter(reqCtx, bson.D{{Key: "domain", Value: domain}}), extra...)
	}
	noRaw := bson.D{{Key: "rawText", Value: 0}}

	say("Loading site analysis...")
	var an Analysis
	if mongoDB.Analyses().FindOne(ctx, own(), options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(noRaw)).Decode(&an) == nil {
		d.Analysis = &an
	}

	say("Gathering optimization reports...")
	if cursor, err := mongoDB.Optimizations().Find(ctx, own(),
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(50).SetProjection(noRaw)); err == nil {
		_ = cursor.All(ctx, &d.Optimizations)
	}

	byGenerated := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(noRaw)
	say("Loading video analysis...")
	var va VideoAnalysis
	if mongoDB.VideoAnalyses().FindOne(ctx, own(), byGenerated).Decode(&va) == nil {
		d.VideoAnalysis = &va
	}
	say("Loading Reddit analysis...")
	var ra RedditAnalysis
	if mongoDB.RedditAnalyses().FindOne(ctx, own(), byGenerated).Decode(&ra) == nil {
		d.RedditAnalysis = &ra
	}
	say("Loading search analysis...")
	var sa SearchAnalysis
	if mongoDB.SearchAnalyses().FindOne(ctx, own(), byGenerated).Decode(&sa) == nil {
		d.SearchAnalysis = &sa
	}
	say("Loading LLM test results...")
	var lt LLMTest
	if mongoDB.LLMTests().FindOne(ctx, own(bson.E{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}}),
		options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})).Decode(&lt) == nil {
		d.LLMTest = &lt
	}
	say("Loading executive summary...")
	var ds DomainSummary
	if mongoDB.DomainSummaries().FindOne(ctx, own()).Decode(&ds) == nil {
		d.Summary = &ds
	}
	say("Loading action items...")
	if cursor, err := mongoDB.Todos().Find(ctx, own(bson.E{Key: "status", Value: "todo"}),
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200)); err == nil {
		_ = cursor.All(ctx, &d.Todos)
	}
	var bp BrandProfile
	if mongoDB.BrandProfiles().FindOne(ctx, own()).Decode(&bp) == nil {
		d.BrandProfile = &bp
	}
	d.Visibility = computeVisibilityScore(ctx, mongoDB, domain, reqCtx)
	return d
}
