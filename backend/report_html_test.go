package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// fixtureReportData fills every section of the report.
func fixtureReportData() reportData {
	d := reportData{Domain: "example.com"}
	d.BrandProfile = &BrandProfile{BrandName: "Example Health"}
	d.Analysis = &Analysis{Model: "Model A", Result: AnalysisResult{
		SiteSummary: "Example Health offers online care — fast and private.",
		Questions: []Question{
			{Question: "What is Example Health?", Category: "Brand", Relevance: "high"},
			{Question: "Best online clinics?", Category: "Category", Relevance: "medium"},
		},
		CrawledPages: make([]CrawledPage, 12),
	}}
	for i := 0; i < 16; i++ {
		score := 30 + i*4
		d.Optimizations = append(d.Optimizations, Optimization{
			Question: fmt.Sprintf("Question number %d about example care?", i+1), Model: "Model A",
			Result: OptimizationResult{
				OverallScore:     score,
				ContentAuthority: DimensionScore{Score: score + 5}, StructuralOptimization: DimensionScore{Score: score - 5},
				SourceAuthority: DimensionScore{Score: score}, KnowledgePersistence: DimensionScore{Score: score + 2},
				Competitors:     []Competitor{{Domain: "rival.example", ScoreEstimate: 70, Strengths: "Detailed guides"}},
				Recommendations: []Recommendation{{Priority: "high", Action: fmt.Sprintf("Add an FAQ for question %d", i+1), ExpectedImpact: "More citations"}},
			},
		})
	}
	d.VideoAnalysis = &VideoAnalysis{Model: "Model B", Result: &VideoAuthorityResult{
		OverallScore: 47, ExecutiveSummary: "Few videos mention the brand.", ConfidenceNote: "Transcripts were partly unavailable.",
		TranscriptAuthority: TranscriptAuthorityPillar{Score: 40},
		TopicalDominance: TopicalDominancePillar{Score: 35,
			ShareOfVoice: []ShareOfVoiceEntry{{BrandName: "Example Health", MentionCount: 3, Percentage: 12}, {BrandName: "Rival", MentionCount: 15, Percentage: 60}},
			ContentGaps:  []ContentGap{{Query: "how online care works", Recommendation: "Publish an explainer"}}},
		CitationNetwork: CitationNetworkPillar{Score: 55,
			TopCreators:    []CreatorProfile{{ChannelTitle: "Health Channel", Role: "reviewer", AuthorityScore: 82}},
			CreatorTargets: []CreatorTarget{{ChannelTitle: "Wellness Weekly", OutreachReason: "Covers the category"}}},
		BrandNarrative: BrandNarrativePillar{Score: 60, NarrativeSummary: "Seen as convenient but expensive.",
			Sentiment: SentimentBreakdown{Positive: 6, Neutral: 3, Negative: 1, Total: 10}, KeyThemes: []string{"convenience", "price"}},
		VideoScorecards: []VideoScorecard{{Title: "Review of Example Health", OverallScore: 72}},
		Recommendations: []VideoRecommendation{{Action: "Pitch the Health Channel", Priority: "medium", ExpectedImpact: "Third-party mentions"}},
	}}
	d.RedditAnalysis = &RedditAnalysis{Model: "Model B", Result: &RedditAuthorityResult{
		OverallScore: 58, ExecutiveSummary: "Discussed often, recommended sometimes.",
		Presence:        RedditPresencePillar{Score: 62, ShareOfVoice: []RedditShareOfVoice{{BrandName: "Example Health", MentionCount: 9, Percentage: 30}}},
		Sentiment:       RedditSentimentPillar{Score: 55, NotableMentions: []RedditMentionExample{{Subreddit: "health", Title: "Anyone tried Example Health?", Sentiment: "positive", Context: "It worked for me", IsRecommendation: true}}},
		Competitive:     RedditCompetitivePillar{Score: 50},
		TrainingSignal:  RedditTrainingSignalPillar{Score: 66},
		Recommendations: []RedditRecommendation{{Action: "Answer questions in r/health", Priority: "low"}},
	}}
	d.SearchAnalysis = &SearchAnalysis{Model: "Model A", Result: &SearchVisibilityResult{
		OverallScore: 81, ExecutiveSummary: "Well placed for AI Overviews.",
		AIOReadiness: AIOReadinessPillar{Score: 85}, CrawlAccess: CrawlAccessibilityPillar{Score: 90,
			CrawlerDetails: []CrawlerStatus{{Name: "GPTBot", Allowed: true}, {Name: "CCBot", Allowed: false, Notes: "Blocked in robots.txt"}}},
		BrandMomentum: BrandSearchMomentumPillar{Score: 70}, ContentFreshness: ContentFreshnessPillar{Score: 75},
		CategoryDiscovery: &CategoryDiscoveryPillar{Score: 65},
		Recommendations:   []SearchRecommendation{{Action: "Unblock CCBot", Priority: "high", ExpectedImpact: "Training data presence"}},
	}}
	d.LLMTest = &LLMTest{OverallScore: 52,
		ProviderSummaries: []LLMTestSummary{
			{ProviderName: "OpenAI", Model: "chat-latest", OverallScore: 60, MentionRate: 70, RecommendRate: 40, AccuracyRate: 80, SentimentScore: 65, Answered: 10},
			{ProviderName: "Anthropic", Model: "Claude Opus 5.5", OverallScore: 45, MentionRate: 50, RecommendRate: 20, AccuracyRate: 70, SentimentScore: 55, Answered: 9, Failed: 1},
		},
		UnavailableProviders: []LLMTestUnavailable{{ProviderName: "Grok", Reason: "the API key is out of credits"}},
		Results: []LLMTestQueryResult{{Query: LLMTestQuery{Query: "What is Example Health?"}, ProviderResults: []LLMTestProviderResult{
			{ProviderName: "OpenAI", Score: 70, Mentioned: true, Recommended: true}, {ProviderName: "Anthropic", Score: 40, Mentioned: true}}}},
	}
	d.Summary = &DomainSummary{Result: DomainSummaryResult{
		ExecutiveSummary: "Example Health is known but rarely recommended.", AverageScore: 60, ScoreRange: [2]int{30, 90},
		Themes:      []SummaryTheme{{Title: "Thin third-party coverage", Description: "Few independent sources."}},
		ActionItems: []SummaryActionItem{{Priority: "low", Action: "Refresh old posts"}, {Priority: "high", Action: "Earn reviews from health media", ExpectedImpact: "Citations in answers"}},
	}}
	d.Todos = []TodoItem{{Status: "todo", Action: "Fix the pricing page", Priority: "high", SourceType: "optimization"}}
	d.Visibility = weighVisibility([]visibilityComponent{
		{Name: "Optimization", Weight: 0.30, Score: 60, Available: true},
		{Name: "Video Authority", Weight: 0.20, Score: 47, Available: true},
		{Name: "Reddit Authority", Weight: 0.20, Score: 58, Available: true},
		{Name: "Search Visibility", Weight: 0.15, Score: 81, Available: true},
		{Name: "LLM Test", Weight: 0.15, Available: false},
	})
	return d
}

func TestWeighVisibility(t *testing.T) {
	vs := fixtureReportData().Visibility
	// (60*.3 + 47*.2 + 58*.2 + 81*.15) / .85 = 51.15 / .85 = 60.2
	if vs.Score != 60 || vs.Available != 4 || vs.Total != 5 {
		t.Errorf("got %+v", vs)
	}
}

func TestBuildReportView_AllSections(t *testing.T) {
	v := buildReportView(fixtureReportData(), time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), "pdf")
	if v.BrandName != "Example Health" || v.Visibility.Score != 60 || len(v.Dimensions) != 5 {
		t.Fatalf("header: %+v", v.Visibility)
	}
	for _, e := range v.TOC {
		if e.Missing {
			t.Errorf("section %q marked missing", e.Title)
		}
	}
	if v.Opt == nil || len(v.Opt.Weakest) != reportWeakestShown || len(v.Opt.Appendix) != 6 || v.Opt.Ranked[0].Score != 30 {
		t.Fatalf("optimization view: %+v", v.Opt)
	}
	// The summary's high-priority action item comes first.
	if len(v.TopActions) != 3 || v.TopActions[0].Action != "Earn reviews from health media" {
		t.Errorf("top actions: %+v", v.TopActions)
	}
	if v.Video == nil || v.Video.Sentiment == nil || v.Video.Sentiment.Positive != 60 || v.Reddit == nil || v.Search == nil || len(v.Search.Pillars) != 5 || v.Test == nil {
		t.Errorf("sections: video %+v reddit %v search %v test %v", v.Video, v.Reddit != nil, v.Search != nil, v.Test != nil)
	}
	if len(v.RecGroups) == 0 || v.RecGroups[0].Priority != "high" {
		t.Errorf("recommendation groups: %+v", v.RecGroups)
	}
	for _, g := range v.RecGroups {
		for _, r := range g.Items {
			if r.Source == "Answers" {
				t.Errorf("answer fix %q in the cross-channel groups", r.Action)
			}
		}
	}
	// 17 answer fixes (16 questions + 1 to-do): 15 shown, weakest question first.
	if len(v.AnswerRecs) != reportAnswerRecsShown || v.MoreAnswers != 2 || v.AnswerRecs[0].Action != "Add an FAQ for question 1" {
		t.Errorf("answer recs: %d shown, %d more, first %q", len(v.AnswerRecs), v.MoreAnswers, v.AnswerRecs[0].Action)
	}
}

func TestRenderReportHTML(t *testing.T) {
	for _, mode := range []string{"pdf", "web"} {
		html, err := renderReportHTML(buildReportView(fixtureReportData(), time.Now(), mode))
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"<no value>", "ZgotmplZ", "â€"} {
			if strings.Contains(html, bad) {
				t.Errorf("%s: output contains %q", mode, bad)
			}
		}
		for _, want := range []string{
			"LLM Visibility Score", `id="executive-summary"`, `id="optimization"`, `id="youtube"`, `id="reddit"`, `id="search"`,
			`id="llm-test"`, `id="recommendations"`, `id="appendix"`, `id="research"`, "online care — fast", "Earn reviews from health media",
			"Not run yet", `style="--p: 60"`, `style="width: 60%"`, "@font-face", "Blocked", "r/health",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: output lacks %q", mode, want)
			}
		}
		if n := strings.Count(html, "card score-card"); n < 25 {
			t.Errorf("%s: only %d score cards", mode, n)
		}
		if !strings.Contains(html, `<body class="`+mode+`">`) {
			t.Errorf("%s: body class", mode)
		}
	}
}

// A domain with only a site analysis still renders, with the other sections
// listed as not yet generated.
func TestRenderReportHTML_Sparse(t *testing.T) {
	d := reportData{Domain: "sparse.example", Analysis: &Analysis{Result: AnalysisResult{SiteSummary: "Hi"}}}
	d.Visibility = weighVisibility([]visibilityComponent{{Name: "Optimization", Weight: 1}})
	v := buildReportView(d, time.Now(), "pdf")
	html, err := renderReportHTML(v)
	if err != nil {
		t.Fatal(err)
	}
	missing := 0
	for _, e := range v.TOC {
		if e.Missing {
			missing++
		}
	}
	if missing != 6 || !strings.Contains(html, "Not yet generated") || strings.Contains(html, `id="optimization"`) {
		t.Errorf("missing=%d", missing)
	}
}

func TestPdfTextEncodesTypography(t *testing.T) {
	if got := pdfText("a — b ’ é"); got != "a \x97 b \x92 \xe9" {
		t.Errorf("got %q", got)
	}
}

// reportTestChrome finds a Chromium to print with, or "" (the tests then skip).
func reportTestChrome() string {
	if p := os.Getenv("LLMOPT_CHROME_PATH"); p != "" {
		return p
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func TestGenerateReportPDF_Chromium(t *testing.T) {
	chrome := reportTestChrome()
	if chrome == "" {
		t.Skip("no Chromium (set LLMOPT_CHROME_PATH)")
	}
	t.Setenv("LLMOPT_CHROME_PATH", chrome)
	pdf, engine, err := generateReportPDF(context.Background(), fixtureReportData())
	if err != nil || engine != reportEngineVersion {
		t.Fatalf("engine %q err %v", engine, err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") || len(pdf) < 20000 {
		t.Errorf("not a plausible PDF (%d bytes)", len(pdf))
	}
	if out := os.Getenv("REPORT_SAMPLE_OUT"); out != "" {
		os.WriteFile(out, pdf, 0o644)
	}
}

// Without a usable Chromium the report falls back to the plain PDF.
func TestGenerateReportPDF_FallsBackWithoutChromium(t *testing.T) {
	t.Setenv("LLMOPT_CHROME_PATH", "/nonexistent/chromium")
	pdf, engine, err := generateReportPDF(context.Background(), fixtureReportData())
	if err != nil || engine != "plain" || !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("engine %q err %v", engine, err)
	}
}

// Cutting text to a length never splits a character (the cause of broken
// to-do summaries ending in "queri�...").
func TestCutAtRuneKeepsCharactersWhole(t *testing.T) {
	s := strings.Repeat("a", 115) + "—queries and more"
	for _, got := range []string{truncateStr(s, 120), truncate(s, 120), todoSummary(s, strings.Repeat("b", 76)+"’ impact text")} {
		if !utf8.ValidString(got) {
			t.Errorf("invalid UTF-8: %q", got)
		}
	}
	if got := cutAtRune("ab—cd", 3); got != "ab" {
		t.Errorf("cutAtRune = %q", got)
	}
	if got := cutAtRune("abc", 10); got != "abc" {
		t.Errorf("cutAtRune short = %q", got)
	}
}
