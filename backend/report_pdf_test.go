package main

import (
	"testing"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// ── scoreToRGB ───────────────────────────────────────────────────────────────

func TestScoreToRGB_High(t *testing.T) {
	r, g, b := scoreToRGB(80)
	if r != 16 || g != 185 || b != 129 {
		t.Errorf("score 80: expected emerald (16,185,129), got (%d,%d,%d)", r, g, b)
	}
}

func TestScoreToRGB_Above80(t *testing.T) {
	r, g, b := scoreToRGB(95)
	if r != 16 || g != 185 || b != 129 {
		t.Errorf("score 95: expected emerald, got (%d,%d,%d)", r, g, b)
	}
}

func TestScoreToRGB_MediumHigh(t *testing.T) {
	r, g, b := scoreToRGB(60)
	if r != 245 || g != 158 || b != 11 {
		t.Errorf("score 60: expected amber (245,158,11), got (%d,%d,%d)", r, g, b)
	}
}

func TestScoreToRGB_Medium(t *testing.T) {
	r, g, b := scoreToRGB(40)
	if r != 249 || g != 115 || b != 22 {
		t.Errorf("score 40: expected orange (249,115,22), got (%d,%d,%d)", r, g, b)
	}
}

func TestScoreToRGB_Low(t *testing.T) {
	r, g, b := scoreToRGB(20)
	if r != 239 || g != 68 || b != 68 {
		t.Errorf("score 20: expected red (239,68,68), got (%d,%d,%d)", r, g, b)
	}
}

func TestScoreToRGB_Zero(t *testing.T) {
	r, g, b := scoreToRGB(0)
	if r != 239 || g != 68 || b != 68 {
		t.Errorf("score 0: expected red, got (%d,%d,%d)", r, g, b)
	}
}

// ── pdfCleanText ─────────────────────────────────────────────────────────────

func TestPdfCleanText_Bullet(t *testing.T) {
	result := pdfCleanText("• item one")
	if result != "- item one" {
		t.Errorf("expected '- item one', got %q", result)
	}
}

func TestPdfCleanText_EnDash(t *testing.T) {
	result := pdfCleanText("2020\u20132025")
	if result != "2020-2025" {
		t.Errorf("expected '2020-2025', got %q", result)
	}
}

func TestPdfCleanText_EmDash(t *testing.T) {
	result := pdfCleanText("hello\u2014world")
	if result != "hello--world" {
		t.Errorf("expected 'hello--world', got %q", result)
	}
}

func TestPdfCleanText_SmartQuotes(t *testing.T) {
	result := pdfCleanText("\u2018smart\u2019 quotes \u201chere\u201d")
	if result != "'smart' quotes \"here\"" {
		t.Errorf("expected smart quotes replaced, got %q", result)
	}
}

func TestPdfCleanText_NoSpecialChars(t *testing.T) {
	input := "normal text without special characters"
	result := pdfCleanText(input)
	if result != input {
		t.Errorf("expected unchanged, got %q", result)
	}
}

// ── fingerprintEqual ─────────────────────────────────────────────────────────

func TestFingerprintEqual_BothNil(t *testing.T) {
	a := ReportFingerprint{}
	b := ReportFingerprint{}
	if !fingerprintEqual(a, b) {
		t.Error("expected empty fingerprints to be equal")
	}
}

func TestFingerprintEqual_SameTimes(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	a := ReportFingerprint{
		AnalysisExists:   true,
		OptimizationCount: 3,
		AnalysisCreatedAt: &now,
	}
	b := ReportFingerprint{
		AnalysisExists:   true,
		OptimizationCount: 3,
		AnalysisCreatedAt: &now,
	}
	if !fingerprintEqual(a, b) {
		t.Error("expected fingerprints with same times to be equal")
	}
}

func TestFingerprintEqual_DifferentCount(t *testing.T) {
	a := ReportFingerprint{OptimizationCount: 1}
	b := ReportFingerprint{OptimizationCount: 2}
	if fingerprintEqual(a, b) {
		t.Error("expected different opt counts to be unequal")
	}
}

func TestFingerprintEqual_OneNilTime(t *testing.T) {
	now := time.Now()
	a := ReportFingerprint{AnalysisCreatedAt: &now}
	b := ReportFingerprint{AnalysisCreatedAt: nil}
	if fingerprintEqual(a, b) {
		t.Error("expected nil vs non-nil time to be unequal")
	}
}

func TestFingerprintEqual_DifferentTimes(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Hour)
	a := ReportFingerprint{AnalysisCreatedAt: &t1}
	b := ReportFingerprint{AnalysisCreatedAt: &t2}
	if fingerprintEqual(a, b) {
		t.Error("expected different times to be unequal")
	}
}

func TestFingerprintEqual_DifferentBoolFlags(t *testing.T) {
	a := ReportFingerprint{AnalysisExists: true, VideoExists: false}
	b := ReportFingerprint{AnalysisExists: true, VideoExists: true}
	if fingerprintEqual(a, b) {
		t.Error("expected different bool flags to be unequal")
	}
}

// ── PDF builder helpers (require gofpdf.Fpdf) ─────────────────────────────

func newTestPDF() *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	return pdf
}

func TestPdfSectionHeader_NoCrash(t *testing.T) {
	pdf := newTestPDF()
	pdfSectionHeader(pdf, "Test Section")
	// verify no error
	if pdf.Error() != nil {
		t.Errorf("pdfSectionHeader caused error: %v", pdf.Error())
	}
}

func TestPdfSubHeader_NoCrash(t *testing.T) {
	pdf := newTestPDF()
	pdfSubHeader(pdf, "Test Subsection")
	if pdf.Error() != nil {
		t.Errorf("pdfSubHeader caused error: %v", pdf.Error())
	}
}

func TestPdfScoreBox_NoCrash(t *testing.T) {
	pdf := newTestPDF()
	pdfScoreBox(pdf, "Score Label", 75)
	if pdf.Error() != nil {
		t.Errorf("pdfScoreBox caused error: %v", pdf.Error())
	}
}

func TestPdfBodyText_NoCrash(t *testing.T) {
	pdf := newTestPDF()
	pdfBodyText(pdf, "This is body text with some content to render.")
	if pdf.Error() != nil {
		t.Errorf("pdfBodyText caused error: %v", pdf.Error())
	}
}

func TestPdfBullet_NoCrash(t *testing.T) {
	pdf := newTestPDF()
	pdfBullet(pdf, "Bullet point text item")
	if pdf.Error() != nil {
		t.Errorf("pdfBullet caused error: %v", pdf.Error())
	}
}

func TestPdfCheckPageBreak_NoCrash(t *testing.T) {
	pdf := newTestPDF()
	pdfCheckPageBreak(pdf, 10)
	if pdf.Error() != nil {
		t.Errorf("pdfCheckPageBreak caused error: %v", pdf.Error())
	}
}

// ── buildReportPDF ────────────────────────────────────────────────────────────

func TestBuildReportPDF_MinimalInput(t *testing.T) {
	// All optional data nil — just domain + nil pointers
	pdfBytes, err := buildReportPDF(
		"example.com",
		nil, // brandProfile
		nil, // analysis
		nil, // optimizations
		nil, // videoAnalysis
		nil, // redditAnalysis
		nil, // searchAnalysis
		nil, // llmTest
		nil, // summary
		nil, // todos
	)
	if err != nil {
		t.Fatalf("buildReportPDF with nil inputs returned error: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Error("expected non-empty PDF bytes")
	}
	// PDF magic bytes: %PDF
	if len(pdfBytes) < 4 || string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:min(20, len(pdfBytes))]))
	}
}

func TestBuildReportPDF_WithBrandProfile(t *testing.T) {
	brand := &BrandProfile{
		BrandName: "Test Brand",
		Domain:    "example.com",
	}
	pdfBytes, err := buildReportPDF("example.com", brand, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with brand returned error: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Error("expected non-empty PDF bytes")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestBuildReportPDF_WithSummary(t *testing.T) {
	summary := &DomainSummary{
		Domain: "example.com",
		Result: DomainSummaryResult{
			ExecutiveSummary: "This is a test executive summary for the brand.",
			AverageScore:     72,
			ScoreRange:       [2]int{60, 85},
			Themes: []SummaryTheme{
				{Title: "Content Authority", Description: "Strong technical content coverage."},
				{Title: "Citation Gap", Description: "Limited third-party references."},
			},
		},
		GeneratedAt: time.Now(),
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, nil, nil, nil, nil, nil, summary, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with summary returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithAnalysis(t *testing.T) {
	analysis := &Analysis{
		Domain: "example.com",
		Model:  "claude-sonnet-4-6",
		Result: AnalysisResult{
			SiteSummary: "A technology company with strong content.",
			Questions: []Question{
				{Question: "What is the company's main product?", Relevance: "high", Category: "product"},
				{Question: "Who are the main competitors?", Relevance: "medium", Category: "competitive"},
			},
		},
		CreatedAt: time.Now(),
	}
	pdfBytes, err := buildReportPDF("example.com", nil, analysis, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with analysis returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithOptimizations(t *testing.T) {
	opts := []Optimization{
		{
			Question: "How do AI engines perceive this brand?",
			Domain:   "example.com",
			Model:    "claude-haiku-4-5-20251001",
			Result: OptimizationResult{
				OverallScore: 65,
				ContentAuthority: DimensionScore{
					Score:        70,
					Evidence:     []string{"Good FAQ content", "Regular blog posts"},
					Improvements: []string{"Add expert citations", "Improve depth"},
				},
				StructuralOptimization: DimensionScore{Score: 60},
				SourceAuthority:        DimensionScore{Score: 75},
				KnowledgePersistence:  DimensionScore{Score: 55},
				Competitors: []Competitor{
					{Domain: "competitor.com", ScoreEstimate: 80, Strengths: "Strong SEO presence"},
				},
				Recommendations: []Recommendation{
					{Priority: "high", Action: "Add expert quotes", ExpectedImpact: "Better citations", Dimension: "source_authority"},
				},
			},
			CreatedAt: time.Now(),
		},
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, opts, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with optimizations returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithVideoAnalysis(t *testing.T) {
	videoAnalysis := &VideoAnalysis{
		Domain: "example.com",
		Model:  "claude-sonnet-4-6",
		Result: &VideoAuthorityResult{
			OverallScore: 72,
			TranscriptAuthority: TranscriptAuthorityPillar{
				Score:    75,
				Evidence: []string{"Good transcript coverage"},
			},
			TopicalDominance: TopicalDominancePillar{
				Score:         68,
				TopicsCovered: 8,
				TopicsTotal:   12,
			},
			CitationNetwork: CitationNetworkPillar{
				Score: 65,
				TopCreators: []CreatorProfile{
					{ChannelTitle: "Tech Channel", AuthorityScore: 90, Sentiment: "positive"},
				},
			},
			BrandNarrative: BrandNarrativePillar{
				Score:            70,
				NarrativeSummary: "Positive brand narrative.",
			},
			ExecutiveSummary: "Strong video authority with room for improvement.",
			Recommendations: []VideoRecommendation{
				{Action: "Add transcripts to all videos", Priority: "high", Dimension: "transcript_authority"},
			},
		},
		GeneratedAt: time.Now(),
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, nil, videoAnalysis, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with videoAnalysis returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithRedditAnalysis(t *testing.T) {
	redditAnalysis := &RedditAnalysis{
		Domain: "example.com",
		Model:  "claude-sonnet-4-6",
		Result: &RedditAuthorityResult{
			OverallScore:     68,
			ExecutiveSummary: "Moderate Reddit presence with positive sentiment.",
			Recommendations: []RedditRecommendation{
				{Action: "Engage in r/technology", Priority: "high", Dimension: "presence"},
			},
		},
		GeneratedAt: time.Now(),
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, nil, nil, redditAnalysis, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with redditAnalysis returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithSearchAnalysis(t *testing.T) {
	searchAnalysis := &SearchAnalysis{
		Domain: "example.com",
		Model:  "claude-sonnet-4-6",
		Result: &SearchVisibilityResult{
			OverallScore:     74,
			ExecutiveSummary: "Good search visibility with structured data.",
			AIOReadiness: AIOReadinessPillar{
				Score:    78,
				Evidence: []string{"Strong FAQ pages", "Good structured data"},
			},
			Recommendations: []SearchRecommendation{
				{Action: "Improve schema markup", Priority: "high", Dimension: "aio_readiness"},
			},
		},
		GeneratedAt: time.Now(),
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, nil, nil, nil, searchAnalysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with searchAnalysis returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithLLMTest(t *testing.T) {
	llmTest := &LLMTest{
		Domain:      "example.com",
		BrandName:   "Test Brand",
		OverallScore: 70,
		ProviderSummaries: []LLMTestSummary{
			{
				ProviderID:     "anthropic",
				ProviderName:   "Anthropic",
				Model:          "claude-sonnet-4-6",
				OverallScore:   75,
				MentionRate:    80,
				RecommendRate:  60,
				AccuracyRate:   85,
				SentimentScore: 70,
			},
		},
		Results: []LLMTestQueryResult{
			{
				Query: LLMTestQuery{
					Query: "What is Test Brand?",
					Type:  "brand",
				},
			},
		},
		GeneratedAt: time.Now(),
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, nil, nil, nil, nil, llmTest, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF with llmTest returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithUnavailableProviders(t *testing.T) {
	llmTest := &LLMTest{
		Domain:       "example.com",
		BrandName:    "Test Brand",
		OverallScore: 70,
		ProviderSummaries: []LLMTestSummary{
			{ProviderID: "anthropic", ProviderName: "Anthropic", Model: "Claude Opus 5.5", OverallScore: 70, Answered: 11, Failed: 1},
		},
		UnavailableProviders: []LLMTestUnavailable{
			{ProviderID: "grok", ProviderName: "Grok", Reason: "the API key is out of credits or has a billing problem"},
			{ProviderID: "gemini", ProviderName: "Gemini", Reason: "no API key is configured for it"},
		},
		GeneratedAt: time.Now(),
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, nil, nil, nil, nil, llmTest, nil, nil)
	if err != nil {
		t.Fatalf("buildReportPDF returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_WithTodos(t *testing.T) {
	todos := []TodoItem{
		{
			Domain:         "example.com",
			Action:         "Write expert content about AI",
			ExpectedImpact: "Better search ranking",
			Dimension:      "content_authority",
			Priority:       "high",
			Status:         "todo",
			CreatedAt:      time.Now(),
		},
		{
			Domain:    "example.com",
			Action:    "Build backlinks",
			Priority:  "medium",
			Dimension: "source_authority",
			Status:    "completed",
			CreatedAt: time.Now(),
		},
	}
	pdfBytes, err := buildReportPDF("example.com", nil, nil, nil, nil, nil, nil, nil, nil, todos)
	if err != nil {
		t.Fatalf("buildReportPDF with todos returned error: %v", err)
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}

func TestBuildReportPDF_FullData(t *testing.T) {
	// Test with all sections populated to maximize coverage
	brand := &BrandProfile{BrandName: "Full Brand", Domain: "full.com"}
	summary := &DomainSummary{
		Domain: "full.com",
		Result: DomainSummaryResult{
			ExecutiveSummary: "Comprehensive test summary.",
			AverageScore:     78,
			ScoreRange:       [2]int{65, 90},
			Themes:           []SummaryTheme{{Title: "Strength", Description: "Technical depth."}},
		},
		GeneratedAt: time.Now(),
	}
	analysis := &Analysis{
		Domain: "full.com",
		Model:  "claude-sonnet-4-6",
		Result: AnalysisResult{SiteSummary: "A leading tech company."},
		CreatedAt: time.Now(),
	}
	opts := []Optimization{{
		Question: "How visible is Full Brand to AI?",
		Domain:   "full.com",
		Result: OptimizationResult{
			OverallScore:          80,
			ContentAuthority:      DimensionScore{Score: 85},
			StructuralOptimization: DimensionScore{Score: 75},
			SourceAuthority:       DimensionScore{Score: 80},
			KnowledgePersistence:  DimensionScore{Score: 80},
		},
		CreatedAt: time.Now(),
	}}
	videoAnalysis := &VideoAnalysis{
		Domain: "full.com",
		Model:  "claude-sonnet-4-6",
		Result: &VideoAuthorityResult{
			OverallScore:     75,
			ExecutiveSummary: "Good video presence.",
		},
		GeneratedAt: time.Now(),
	}
	redditAnalysis := &RedditAnalysis{
		Domain: "full.com",
		Result: &RedditAuthorityResult{OverallScore: 70, ExecutiveSummary: "Positive Reddit presence."},
		GeneratedAt: time.Now(),
	}
	searchAnalysis := &SearchAnalysis{
		Domain: "full.com",
		Result: &SearchVisibilityResult{OverallScore: 72, ExecutiveSummary: "Good search presence."},
		GeneratedAt: time.Now(),
	}
	llmTest := &LLMTest{
		Domain:    "full.com",
		BrandName: "Full Brand",
		ProviderSummaries: []LLMTestSummary{{
			ProviderID: "anthropic", ProviderName: "Anthropic", Model: "claude-sonnet-4-6", OverallScore: 80,
		}},
		GeneratedAt: time.Now(),
	}
	todos := []TodoItem{{
		Domain: "full.com", Action: "Improve content", Priority: "high",
		Dimension: "content_authority", Status: "todo", CreatedAt: time.Now(),
	}}

	pdfBytes, err := buildReportPDF("full.com", brand, analysis, opts, videoAnalysis, redditAnalysis, searchAnalysis, llmTest, summary, todos)
	if err != nil {
		t.Fatalf("buildReportPDF full data returned error: %v", err)
	}
	if len(pdfBytes) < 1000 {
		t.Errorf("expected substantial PDF, got only %d bytes", len(pdfBytes))
	}
	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header, got: %q", string(pdfBytes[:4]))
	}
}
