package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"llmopt/internal/saas"
)

// --- Fingerprint ---

func computeFingerprint(ctx context.Context, mongoDB *MongoDB, domain string, tenantCtx context.Context) ReportFingerprint {
	fp := ReportFingerprint{}
	filter := func(extra ...bson.E) bson.D {
		f := tenantFilter(tenantCtx, bson.D{{Key: "domain", Value: domain}})
		return append(f, extra...)
	}

	// Latest optimization
	var latestOpt struct {
		CreatedAt time.Time `bson:"createdAt"`
	}
	optOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "createdAt", Value: 1}})
	if err := mongoDB.Optimizations().FindOne(ctx, filter(), optOpts).Decode(&latestOpt); err == nil {
		fp.LatestOptimizationAt = &latestOpt.CreatedAt
	}
	cnt, _ := mongoDB.Optimizations().CountDocuments(ctx, filter())
	fp.OptimizationCount = int(cnt)

	// Latest analysis
	var latestAn struct {
		CreatedAt time.Time `bson:"createdAt"`
	}
	anOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "createdAt", Value: 1}})
	if err := mongoDB.Analyses().FindOne(ctx, filter(), anOpts).Decode(&latestAn); err == nil {
		fp.AnalysisCreatedAt = &latestAn.CreatedAt
		fp.AnalysisExists = true
	}

	// Video analysis
	var va struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.VideoAnalyses().FindOne(ctx, filter(), options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&va); err == nil {
		fp.VideoGeneratedAt = &va.GeneratedAt
		fp.VideoExists = true
	}

	// Reddit analysis
	var ra struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.RedditAnalyses().FindOne(ctx, filter(), options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&ra); err == nil {
		fp.RedditGeneratedAt = &ra.GeneratedAt
		fp.RedditExists = true
	}

	// Search analysis
	var sa struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.SearchAnalyses().FindOne(ctx, filter(), options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&sa); err == nil {
		fp.SearchGeneratedAt = &sa.GeneratedAt
		fp.SearchExists = true
	}

	// Domain summary
	var ds struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.DomainSummaries().FindOne(ctx, filter(), options.FindOne().SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&ds); err == nil {
		fp.SummaryGeneratedAt = &ds.GeneratedAt
	}

	// Latest todo
	var latestTodo struct {
		CreatedAt time.Time `bson:"createdAt"`
	}
	todoFilter := tenantFilter(tenantCtx, bson.D{{Key: "domain", Value: domain}, {Key: "status", Value: "todo"}})
	todoOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "createdAt", Value: 1}})
	if err := mongoDB.Todos().FindOne(ctx, todoFilter, todoOpts).Decode(&latestTodo); err == nil {
		fp.LatestTodoAt = &latestTodo.CreatedAt
	}
	if n, err := mongoDB.Todos().CountDocuments(ctx, todoFilter); err == nil {
		fp.OpenTodoCount = int(n)
	}

	// Latest LLM Test (not a competitor test)
	var lt struct {
		GeneratedAt time.Time `bson:"generatedAt"`
	}
	if err := mongoDB.LLMTests().FindOne(ctx, filter(bson.E{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}}),
		options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "generatedAt", Value: 1}})).Decode(&lt); err == nil {
		fp.LLMTestGeneratedAt = &lt.GeneratedAt
	}

	// Latest advice update in any result or to-do
	for _, c := range []*mongo.Collection{mongoDB.Optimizations(), mongoDB.SearchAnalyses(), mongoDB.VideoAnalyses(), mongoDB.RedditAnalyses(), mongoDB.Todos()} {
		var g struct {
			Guidance *GuidanceRevision `bson:"guidance"`
		}
		if c.FindOne(ctx, filter(bson.E{Key: "guidance.revisedAt", Value: bson.D{{Key: "$exists", Value: true}}}),
			options.FindOne().SetSort(bson.D{{Key: "guidance.revisedAt", Value: -1}}).SetProjection(bson.D{{Key: "guidance.revisedAt", Value: 1}})).Decode(&g) == nil &&
			g.Guidance != nil && (fp.GuidanceRevisedAt == nil || g.Guidance.RevisedAt.After(*fp.GuidanceRevisedAt)) {
			t := g.Guidance.RevisedAt
			fp.GuidanceRevisedAt = &t
		}
	}

	return fp
}

func fingerprintEqual(a, b ReportFingerprint) bool {
	timeEq := func(t1, t2 *time.Time) bool {
		if t1 == nil && t2 == nil {
			return true
		}
		if t1 == nil || t2 == nil {
			return false
		}
		return t1.Unix() == t2.Unix()
	}
	return timeEq(a.LatestOptimizationAt, b.LatestOptimizationAt) &&
		timeEq(a.AnalysisCreatedAt, b.AnalysisCreatedAt) &&
		timeEq(a.VideoGeneratedAt, b.VideoGeneratedAt) &&
		timeEq(a.RedditGeneratedAt, b.RedditGeneratedAt) &&
		timeEq(a.SearchGeneratedAt, b.SearchGeneratedAt) &&
		timeEq(a.SummaryGeneratedAt, b.SummaryGeneratedAt) &&
		timeEq(a.LatestTodoAt, b.LatestTodoAt) &&
		timeEq(a.LLMTestGeneratedAt, b.LLMTestGeneratedAt) &&
		timeEq(a.GuidanceRevisedAt, b.GuidanceRevisedAt) &&
		a.OpenTodoCount == b.OpenTodoCount &&
		a.OptimizationCount == b.OptimizationCount &&
		a.AnalysisExists == b.AnalysisExists &&
		a.VideoExists == b.VideoExists &&
		a.RedditExists == b.RedditExists &&
		a.SearchExists == b.SearchExists &&
		a.Engine == b.Engine
}

// --- PDF Builder ---

func scoreToRGB(score int) (int, int, int) {
	if score >= 80 {
		return 16, 185, 129 // emerald
	}
	if score >= 60 {
		return 245, 158, 11 // amber
	}
	if score >= 40 {
		return 249, 115, 22 // orange
	}
	return 239, 68, 68 // red
}

func pdfSectionHeader(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 10, pdfText(title), "", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetDrawColor(200, 200, 200)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.Line(x, y, 190, y)
	pdf.Ln(6)
}

func pdfSubHeader(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(50, 50, 50)
	pdf.CellFormat(0, 8, pdfText(title), "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func pdfScoreBox(pdf *gofpdf.Fpdf, label string, score int) {
	r, g, b := scoreToRGB(score)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(r, g, b)
	pdf.CellFormat(25, 7, pdfText(fmt.Sprintf("%d/100", score)), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(80, 80, 80)
	pdf.CellFormat(0, 7, pdfText(label), "", 1, "L", false, 0, "")
}

// pdfText converts UTF-8 to Windows-1252, the encoding of gofpdf's built-in
// fonts. Without it an em dash prints as "â€”" and accented letters break.
// Every string the plain report draws goes through it.
var pdfText = gofpdf.New("P", "mm", "A4", "").UnicodeTranslatorFromDescriptor("")

// pdfCleanText used to swap typography for ASCII; pdfText now encodes it
// properly, so text passes through unchanged.
func pdfCleanText(text string) string {
	return text
}

func pdfBodyText(pdf *gofpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)
	pdf.MultiCell(0, 5, pdfText(pdfCleanText(text)), "", "L", false)
	pdf.Ln(3)
}

func pdfBullet(pdf *gofpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 60)
	text = pdfCleanText(text)
	w, _ := pdf.GetPageSize()
	_, _, mr, _ := pdf.GetMargins()
	maxW := w - pdf.GetX() - mr - 2
	pdf.CellFormat(5, 5, pdfText("-"), "", 0, "L", false, 0, "")
	pdf.MultiCell(maxW, 5, pdfText(text), "", "L", false)
}

func pdfCheckPageBreak(pdf *gofpdf.Fpdf, neededMM float64) {
	_, h := pdf.GetPageSize()
	_, _, _, mb := pdf.GetMargins()
	if pdf.GetY()+neededMM > h-mb {
		pdf.AddPage()
	}
}

// tocEntry tracks a section name and its starting page.
type tocEntry struct {
	name    string
	page    int
	missing bool // true if the section data is not yet generated
}

func buildReportPDF(
	domain string,
	brandProfile *BrandProfile,
	analysis *Analysis,
	optimizations []Optimization,
	videoAnalysis *VideoAnalysis,
	redditAnalysis *RedditAnalysis,
	searchAnalysis *SearchAnalysis,
	llmTest *LLMTest,
	summary *DomainSummary,
	todos []TodoItem,
) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)

	displayDomain := strings.TrimPrefix(domain, "https://")
	displayDomain = strings.TrimPrefix(displayDomain, "http://")

	// Brand name for cover page
	brandName := displayDomain
	if brandProfile != nil && brandProfile.BrandName != "" {
		brandName = brandProfile.BrandName
	}

	// --- Cover Page ---
	pdf.AddPage()
	pdf.Ln(40)
	pdf.SetFont("Helvetica", "B", 28)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 14, pdfText("AI Visibility Report"), "", 1, "C", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "", 12)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(0, 7, pdfText("How AI Search Engines See Your Brand"), "", 1, "C", false, 0, "")
	pdf.Ln(10)

	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetTextColor(50, 50, 50)
	pdf.CellFormat(0, 10, pdfText(brandName), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 7, pdfText("Generated "+time.Now().Format("January 2, 2006")), "", 1, "C", false, 0, "")

	// AI Models used
	modelSet := map[string]bool{}
	if analysis != nil && analysis.Model != "" {
		modelSet[analysis.Model] = true
	}
	for _, opt := range optimizations {
		if opt.Model != "" {
			modelSet[opt.Model] = true
		}
	}
	if videoAnalysis != nil && videoAnalysis.Model != "" {
		modelSet[videoAnalysis.Model] = true
	}
	if redditAnalysis != nil && redditAnalysis.Model != "" {
		modelSet[redditAnalysis.Model] = true
	}
	if searchAnalysis != nil && searchAnalysis.Model != "" {
		modelSet[searchAnalysis.Model] = true
	}
	if llmTest != nil {
		for _, ps := range llmTest.ProviderSummaries {
			if ps.Model != "" {
				modelSet[ps.Model] = true
			}
		}
	}
	if len(modelSet) > 0 {
		modelNames := make([]string, 0, len(modelSet))
		for m := range modelSet {
			modelNames = append(modelNames, m)
		}
		sort.Strings(modelNames)
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 6, pdfText("AI Models: "+strings.Join(modelNames, ", ")), "", 1, "C", false, 0, "")
	}

	pdf.Ln(12)

	// Branding
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(140, 140, 140)
	pdf.CellFormat(0, 6, pdfText("Powered by LLM Optimizer"), "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 130, 200)
	pdf.CellFormat(0, 5, pdfText("llmopt.metavert.io"), "", 1, "C", false, 0, "")
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(140, 140, 140)
	pdf.CellFormat(0, 5, pdfText("Open-source (MIT) \u2014 install and run it yourself:"), "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(100, 130, 200)
	pdf.CellFormat(0, 5, pdfText("github.com/metavert-io/llmopt"), "", 1, "C", false, 0, "")

	// --- Table of Contents Page ---
	pdf.AddPage()
	tocPageNum := pdf.PageNo()
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 12, pdfText("Table of Contents"), "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetDrawColor(200, 200, 200)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.Line(x, y, 190, y)
	pdf.Ln(10)
	// We'll come back to fill this in after rendering all sections.
	// Reserve vertical space — will be overwritten in second pass.

	// Track TOC entries as we render
	toc := []tocEntry{}

	// Count active todos for TOC
	todoCount := 0
	for _, t := range todos {
		if t.Status == "todo" {
			todoCount++
		}
	}

	// --- 1. Executive Summary ---
	if summary != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Executive Summary", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Executive Summary")

		if summary.Result.ExecutiveSummary != "" {
			pdfBodyText(pdf, summary.Result.ExecutiveSummary)
		}

		if summary.Result.AverageScore > 0 {
			pdf.Ln(2)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.SetTextColor(50, 50, 50)
			r, g, b := scoreToRGB(summary.Result.AverageScore)
			pdf.CellFormat(0, 7, pdfText("Average Score: "), "", 0, "L", false, 0, "")
			pdf.SetTextColor(r, g, b)
			scoreStr := fmt.Sprintf("%d/100", summary.Result.AverageScore)
			if summary.Result.ScoreRange[0] > 0 && summary.Result.ScoreRange[1] > 0 {
				scoreStr += fmt.Sprintf(" (range: %d-%d)", summary.Result.ScoreRange[0], summary.Result.ScoreRange[1])
			}
			pdf.CellFormat(0, 7, pdfText(scoreStr), "", 1, "L", false, 0, "")
			pdf.Ln(4)
		}

		if len(summary.Result.Themes) > 0 {
			pdfSubHeader(pdf, "Key Themes")
			for _, theme := range summary.Result.Themes {
				pdfCheckPageBreak(pdf, 15)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.SetTextColor(50, 50, 50)
				pdf.CellFormat(0, 6, pdfText(theme.Title), "", 1, "L", false, 0, "")
				if theme.Description != "" {
					pdfBodyText(pdf, theme.Description)
				}
			}
		}

	} else {
		toc = append(toc, tocEntry{name: "Executive Summary", missing: true})
	}

	// --- 2. Site Analysis ---
	if analysis != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Site Analysis", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Site Analysis")

		if analysis.Result.SiteSummary != "" {
			pdfSubHeader(pdf, "Site Summary")
			pdfBodyText(pdf, analysis.Result.SiteSummary)
		}

		if len(analysis.Result.Questions) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, fmt.Sprintf("Questions Discovered (%d)", len(analysis.Result.Questions)))

			// Table header
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(80, 80, 80)
			pdf.SetFillColor(245, 245, 245)
			pdf.CellFormat(90, 6, pdfText("Question"), "B", 0, "L", true, 0, "")
			pdf.CellFormat(35, 6, pdfText("Category"), "B", 0, "L", true, 0, "")
			pdf.CellFormat(0, 6, pdfText("Relevance"), "B", 1, "L", true, 0, "")

			pdf.SetFont("Helvetica", "", 8)
			pdf.SetTextColor(60, 60, 60)
			for _, q := range analysis.Result.Questions {
				pdfCheckPageBreak(pdf, 6)
				question := pdfCleanText(q.Question)
				if len(question) > 70 {
					question = question[:67] + "..."
				}
				cat := q.Category
				if len(cat) > 20 {
					cat = cat[:17] + "..."
				}
				pdf.CellFormat(90, 5, pdfText(question), "", 0, "L", false, 0, "")
				pdf.CellFormat(35, 5, pdfText(cat), "", 0, "L", false, 0, "")
				pdf.CellFormat(0, 5, pdfText(q.Relevance), "", 1, "L", false, 0, "")
			}
			pdf.Ln(3)
		}

		if len(analysis.Result.CrawledPages) > 0 {
			pdfCheckPageBreak(pdf, 10)
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(100, 100, 100)
			pdf.CellFormat(0, 6, pdfText(fmt.Sprintf("Pages crawled: %d", len(analysis.Result.CrawledPages))), "", 1, "L", false, 0, "")
		}
	} else {
		toc = append(toc, tocEntry{name: "Site Analysis", missing: true})
	}

	// --- 3. Optimization Reports ---
	if len(optimizations) > 0 {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: fmt.Sprintf("Optimization Reports (%d)", len(optimizations)), page: pdf.PageNo()})
		pdfSectionHeader(pdf, fmt.Sprintf("Optimization Reports (%d)", len(optimizations)))

		for i, opt := range optimizations {
			if i > 0 {
				pdfCheckPageBreak(pdf, 50)
				pdf.Ln(4)
				pdf.SetDrawColor(220, 220, 220)
				lx := pdf.GetX()
				ly := pdf.GetY()
				pdf.Line(lx, ly, 190, ly)
				pdf.Ln(4)
			}

			pdfCheckPageBreak(pdf, 40)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.SetTextColor(40, 40, 40)
			q := pdfCleanText(opt.Question)
			if len(q) > 120 {
				q = q[:117] + "..."
			}
			pdf.MultiCell(0, 6, pdfText(q), "", "L", false)
			pdf.Ln(2)

			r, g, b := scoreToRGB(opt.Result.OverallScore)
			pdf.SetFont("Helvetica", "B", 14)
			pdf.SetTextColor(r, g, b)
			pdf.CellFormat(30, 8, pdfText(fmt.Sprintf("%d/100", opt.Result.OverallScore)), "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(120, 120, 120)
			pdf.CellFormat(0, 8, pdfText("Overall Score"), "", 1, "L", false, 0, "")
			pdf.Ln(2)

			pdfScoreBox(pdf, "Content Authority", opt.Result.ContentAuthority.Score)
			pdfScoreBox(pdf, "Structural Optimization", opt.Result.StructuralOptimization.Score)
			pdfScoreBox(pdf, "Source Authority", opt.Result.SourceAuthority.Score)
			pdfScoreBox(pdf, "Knowledge Persistence", opt.Result.KnowledgePersistence.Score)
			pdf.Ln(2)

			if len(opt.Result.Competitors) > 0 {
				pdfCheckPageBreak(pdf, 20)
				pdf.Ln(2)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.SetTextColor(80, 80, 80)
				pdf.CellFormat(0, 6, pdfText("Competitive Landscape:"), "", 1, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 9)
				for _, comp := range opt.Result.Competitors {
					pdfCheckPageBreak(pdf, 6)
					cr, cg, cb := scoreToRGB(comp.ScoreEstimate)
					pdf.SetTextColor(cr, cg, cb)
					pdf.CellFormat(20, 5, pdfText(fmt.Sprintf("%d/100", comp.ScoreEstimate)), "", 0, "L", false, 0, "")
					pdf.SetTextColor(60, 60, 60)
					name := comp.Domain
					if len(name) > 30 {
						name = name[:27] + "..."
					}
					pdf.CellFormat(50, 5, pdfText(name), "", 0, "L", false, 0, "")
					strengths := comp.Strengths
					if len(strengths) > 60 {
						strengths = strengths[:57] + "..."
					}
					pdf.SetTextColor(100, 100, 100)
					pdf.CellFormat(0, 5, pdfText(pdfCleanText(strengths)), "", 1, "L", false, 0, "")
				}
			}
		}
	} else {
		toc = append(toc, tocEntry{name: "Optimization Reports", missing: true})
	}

	// --- 4. Video Authority ---
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "YouTube Video Authority", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "YouTube Video Authority")

		vr := videoAnalysis.Result

		r, g, b := scoreToRGB(vr.OverallScore)
		pdf.SetFont("Helvetica", "B", 18)
		pdf.SetTextColor(r, g, b)
		pdf.CellFormat(30, 10, pdfText(fmt.Sprintf("%d", vr.OverallScore)), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 10, pdfText("/ 100  Overall Video Authority Score"), "", 1, "L", false, 0, "")
		pdf.Ln(4)

		pdfScoreBox(pdf, "Transcript Authority", vr.TranscriptAuthority.Score)
		pdfScoreBox(pdf, "Topical Dominance", vr.TopicalDominance.Score)
		pdfScoreBox(pdf, "Citation Network", vr.CitationNetwork.Score)
		pdfScoreBox(pdf, "Brand Narrative", vr.BrandNarrative.Score)
		pdf.Ln(4)

		if vr.ExecutiveSummary != "" {
			pdfSubHeader(pdf, "Summary")
			pdfBodyText(pdf, vr.ExecutiveSummary)
		}

		// What LLMs probably believe about the brand (narrative + sentiment)
		if vr.BrandNarrative.NarrativeSummary != "" {
			pdfCheckPageBreak(pdf, 24)
			pdfSubHeader(pdf, "What LLMs Probably Believe About You")
			pdfBodyText(pdf, vr.BrandNarrative.NarrativeSummary)
			if s := vr.BrandNarrative.Sentiment; s.Total > 0 {
				pdf.SetFont("Helvetica", "", 10)
				pdf.SetTextColor(80, 80, 80)
				pdf.CellFormat(0, 5, pdfText(fmt.Sprintf("Sentiment across %d mentions: %d%% positive, %d%% neutral, %d%% negative",
					s.Total, s.Positive*100/s.Total, s.Neutral*100/s.Total, s.Negative*100/s.Total)), "", 1, "L", false, 0, "")
				pdf.Ln(2)
			}
		}

		// Key themes LLMs associate with the brand
		if len(vr.BrandNarrative.KeyThemes) > 0 {
			pdfCheckPageBreak(pdf, 14)
			pdfSubHeader(pdf, "Key Themes")
			pdfBodyText(pdf, strings.Join(vr.BrandNarrative.KeyThemes, " - "))
		}

		// Share of voice vs. competitors
		if len(vr.TopicalDominance.ShareOfVoice) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Share of Voice")
			for _, sov := range vr.TopicalDominance.ShareOfVoice {
				pdfCheckPageBreak(pdf, 6)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.SetTextColor(80, 80, 80)
				pdf.CellFormat(20, 5, pdfText(fmt.Sprintf("%.0f%%", sov.Percentage)), "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 10)
				pdf.SetTextColor(60, 60, 60)
				pdf.CellFormat(0, 5, pdfText(fmt.Sprintf("%s (%d mentions)", pdfCleanText(sov.BrandName), sov.MentionCount)), "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		}

		// Content gap opportunities
		if len(vr.TopicalDominance.ContentGaps) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Content Gap Opportunities")
			for _, cg := range vr.TopicalDominance.ContentGaps {
				pdfCheckPageBreak(pdf, 10)
				line := cg.Query
				if cg.Recommendation != "" {
					line = cg.Query + " - " + cg.Recommendation
				}
				pdfBullet(pdf, line)
			}
			pdf.Ln(2)
		}

		// Top third-party creators citing the brand
		if len(vr.CitationNetwork.TopCreators) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Top Creators")
			for _, c := range vr.CitationNetwork.TopCreators {
				pdfCheckPageBreak(pdf, 6)
				pdf.SetFont("Helvetica", "B", 10)
				cr, cg, cb := scoreToRGB(c.AuthorityScore)
				pdf.SetTextColor(cr, cg, cb)
				pdf.CellFormat(15, 5, pdfText(fmt.Sprintf("%d", c.AuthorityScore)), "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 10)
				pdf.SetTextColor(60, 60, 60)
				detail := pdfCleanText(c.ChannelTitle)
				if c.Role != "" {
					detail += " (" + c.Role + ")"
				}
				pdf.CellFormat(0, 5, pdfText(detail), "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		}

		// Creator outreach targets
		if len(vr.CitationNetwork.CreatorTargets) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Creator Outreach Targets")
			for _, ct := range vr.CitationNetwork.CreatorTargets {
				pdfCheckPageBreak(pdf, 10)
				line := ct.ChannelTitle
				if ct.OutreachReason != "" {
					line = ct.ChannelTitle + " - " + ct.OutreachReason
				}
				pdfBullet(pdf, line)
			}
			pdf.Ln(2)
		}

		if vr.ConfidenceNote != "" {
			pdfCheckPageBreak(pdf, 14)
			pdf.SetFont("Helvetica", "I", 9)
			pdf.SetTextColor(120, 120, 120)
			pdf.MultiCell(0, 4, pdfText(pdfCleanText("Note: "+vr.ConfidenceNote)), "", "L", false)
			pdf.Ln(2)
		}

		if len(vr.VideoScorecards) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Video Scorecards")
			limit := len(vr.VideoScorecards)
			if limit > 10 {
				limit = 10
			}
			for _, sc := range vr.VideoScorecards[:limit] {
				pdfCheckPageBreak(pdf, 10)
				cr, cg, cb := scoreToRGB(sc.OverallScore)
				pdf.SetTextColor(cr, cg, cb)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.CellFormat(20, 5, pdfText(fmt.Sprintf("%d/100", sc.OverallScore)), "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(60, 60, 60)
				title := sc.Title
				if len(title) > 80 {
					title = title[:77] + "..."
				}
				pdf.CellFormat(0, 5, pdfText(pdfCleanText(title)), "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		}

	} else {
		toc = append(toc, tocEntry{name: "YouTube Video Authority", missing: true})
	}

	// --- 5. Reddit Authority ---
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Reddit Authority", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Reddit Authority")

		rr := redditAnalysis.Result

		r, g, b := scoreToRGB(rr.OverallScore)
		pdf.SetFont("Helvetica", "B", 18)
		pdf.SetTextColor(r, g, b)
		pdf.CellFormat(30, 10, pdfText(fmt.Sprintf("%d", rr.OverallScore)), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 10, pdfText("/ 100  Overall Reddit Authority Score"), "", 1, "L", false, 0, "")
		pdf.Ln(4)

		pdfScoreBox(pdf, "Presence", rr.Presence.Score)
		pdfScoreBox(pdf, "Sentiment & Recommendations", rr.Sentiment.Score)
		pdfScoreBox(pdf, "Competitive Positioning", rr.Competitive.Score)
		pdfScoreBox(pdf, "Training Signal Strength", rr.TrainingSignal.Score)
		pdf.Ln(4)

		if rr.ExecutiveSummary != "" {
			pdfSubHeader(pdf, "Summary")
			pdfBodyText(pdf, rr.ExecutiveSummary)
		}

		if len(rr.Presence.ShareOfVoice) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "Share of Voice")
			for _, sov := range rr.Presence.ShareOfVoice {
				pdfCheckPageBreak(pdf, 6)
				pdf.SetFont("Helvetica", "B", 10)
				pdf.SetTextColor(80, 80, 80)
				pdf.CellFormat(20, 5, pdfText(fmt.Sprintf("%.0f%%", sov.Percentage)), "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 10)
				pdf.SetTextColor(60, 60, 60)
				pdf.CellFormat(0, 5, pdfText(fmt.Sprintf("%s (%d mentions)", sov.BrandName, sov.MentionCount)), "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)
		}

	} else {
		toc = append(toc, tocEntry{name: "Reddit Authority", missing: true})
	}

	// --- 6. Search Visibility ---
	if searchAnalysis != nil && searchAnalysis.Result != nil {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Search Visibility", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Search Visibility")

		sr := searchAnalysis.Result

		r, g, b := scoreToRGB(sr.OverallScore)
		pdf.SetFont("Helvetica", "B", 18)
		pdf.SetTextColor(r, g, b)
		pdf.CellFormat(30, 10, pdfText(fmt.Sprintf("%d", sr.OverallScore)), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 10, pdfText("/ 100  Overall Search Visibility Score"), "", 1, "L", false, 0, "")
		pdf.Ln(4)

		pdfScoreBox(pdf, "AI Overview Readiness", sr.AIOReadiness.Score)
		pdfScoreBox(pdf, "Crawl Accessibility", sr.CrawlAccess.Score)
		pdfScoreBox(pdf, "Brand Search Momentum", sr.BrandMomentum.Score)
		pdfScoreBox(pdf, "Content Freshness", sr.ContentFreshness.Score)
		if sr.CategoryDiscovery != nil {
			pdfScoreBox(pdf, "Category Discovery", sr.CategoryDiscovery.Score)
		}
		pdf.Ln(4)

		if sr.ExecutiveSummary != "" {
			pdfSubHeader(pdf, "Summary")
			pdfBodyText(pdf, sr.ExecutiveSummary)
		}

		// Crawler details
		if len(sr.CrawlAccess.CrawlerDetails) > 0 {
			pdfCheckPageBreak(pdf, 20)
			pdfSubHeader(pdf, "AI Crawler Access")
			for _, c := range sr.CrawlAccess.CrawlerDetails {
				pdfCheckPageBreak(pdf, 6)
				pdf.SetFont("Helvetica", "B", 9)
				if c.Allowed {
					pdf.SetTextColor(16, 185, 129) // emerald
				} else {
					pdf.SetTextColor(239, 68, 68) // red
				}
				status := "Blocked"
				if c.Allowed {
					status = "Allowed"
				}
				pdf.CellFormat(20, 5, pdfText(status), "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(60, 60, 60)
				pdf.CellFormat(30, 5, pdfText(c.Name), "", 0, "L", false, 0, "")
				if c.Notes != "" {
					pdf.SetTextColor(100, 100, 100)
					notes := c.Notes
					if len(notes) > 70 {
						notes = notes[:67] + "..."
					}
					pdf.CellFormat(0, 5, pdfText(pdfCleanText(notes)), "", 0, "L", false, 0, "")
				}
				pdf.Ln(5)
			}
			pdf.Ln(2)
		}

	} else {
		toc = append(toc, tocEntry{name: "Search Visibility", missing: true})
	}

	// --- 7. LLM Brand Test ---
	if llmTest != nil && len(llmTest.ProviderSummaries) > 0 {
		pdf.AddPage()
		toc = append(toc, tocEntry{name: "LLM Brand Test", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "LLM Brand Test")

		r, g, b := scoreToRGB(llmTest.OverallScore)
		pdf.SetFont("Helvetica", "B", 18)
		pdf.SetTextColor(r, g, b)
		pdf.CellFormat(30, 10, pdfText(fmt.Sprintf("%d", llmTest.OverallScore)), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 10, pdfText("/ 100  Overall Brand Test Score"), "", 1, "L", false, 0, "")
		pdf.Ln(4)

		// Provider summaries table
		pdfSubHeader(pdf, "Provider Results")
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetFillColor(245, 245, 245)
		pdf.CellFormat(45, 6, pdfText("Provider / Model"), "B", 0, "L", true, 0, "")
		pdf.CellFormat(20, 6, pdfText("Score"), "B", 0, "C", true, 0, "")
		pdf.CellFormat(25, 6, pdfText("Mention"), "B", 0, "C", true, 0, "")
		pdf.CellFormat(30, 6, pdfText("Recommend"), "B", 0, "C", true, 0, "")
		pdf.CellFormat(25, 6, pdfText("Accuracy"), "B", 0, "C", true, 0, "")
		pdf.CellFormat(0, 6, pdfText("Sentiment"), "B", 1, "C", true, 0, "")

		for _, ps := range llmTest.ProviderSummaries {
			pdfCheckPageBreak(pdf, 6)
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(60, 60, 60)
			label := ps.ProviderName
			if ps.Model != "" {
				label += " (" + ps.Model + ")"
			}
			if len(label) > 30 {
				label = label[:27] + "..."
			}
			pdf.CellFormat(45, 5, pdfText(label), "", 0, "L", false, 0, "")
			sr, sg, sb := scoreToRGB(ps.OverallScore)
			pdf.SetTextColor(sr, sg, sb)
			pdf.SetFont("Helvetica", "B", 9)
			pdf.CellFormat(20, 5, pdfText(fmt.Sprintf("%d", ps.OverallScore)), "", 0, "C", false, 0, "")
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(60, 60, 60)
			pdf.CellFormat(25, 5, pdfText(fmt.Sprintf("%d%%", ps.MentionRate)), "", 0, "C", false, 0, "")
			pdf.CellFormat(30, 5, pdfText(fmt.Sprintf("%d%%", ps.RecommendRate)), "", 0, "C", false, 0, "")
			pdf.CellFormat(25, 5, pdfText(fmt.Sprintf("%d%%", ps.AccuracyRate)), "", 0, "C", false, 0, "")
			pdf.CellFormat(0, 5, pdfText(fmt.Sprintf("%d/100", ps.SentimentScore)), "", 1, "C", false, 0, "")
		}
		if len(llmTest.UnavailableProviders) > 0 {
			// Say which selected providers couldn't answer, so a missing row
			// isn't read as the provider ignoring the brand.
			var notes []string
			for _, u := range llmTest.UnavailableProviders {
				notes = append(notes, fmt.Sprintf("%s (%s)", u.ProviderName, u.Reason))
			}
			pdf.Ln(2)
			pdfCheckPageBreak(pdf, 10)
			pdf.SetFont("Helvetica", "I", 8)
			pdf.SetTextColor(140, 110, 40)
			pdf.MultiCell(0, 4, pdfText("Not included: "+strings.Join(notes, "; ")+". Scores cover the providers that answered."), "", "L", false)
		}
		pdf.Ln(4)

		// Query-level results (top 15)
		if len(llmTest.Results) > 0 {
			pdfCheckPageBreak(pdf, 20)
			limit := len(llmTest.Results)
			if limit > 15 {
				limit = 15
			}
			pdfSubHeader(pdf, fmt.Sprintf("Query Results (%d queries)", len(llmTest.Results)))
			for _, qr := range llmTest.Results[:limit] {
				pdfCheckPageBreak(pdf, 12)
				pdf.SetFont("Helvetica", "B", 9)
				pdf.SetTextColor(50, 50, 50)
				query := pdfCleanText(qr.Query.Query)
				if len(query) > 80 {
					query = query[:77] + "..."
				}
				pdf.CellFormat(0, 5, pdfText(query), "", 1, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 8)
				for _, a := range testQueryAnswers(qr) {
					pdf.SetTextColor(100, 100, 100)
					nameStr := a.Provider
					if len(nameStr) > 15 {
						nameStr = nameStr[:12] + "..."
					}
					if a.Search {
						nameStr += " (web)"
					}
					pdf.CellFormat(28, 4.5, pdfText("  "+nameStr), "", 0, "L", false, 0, "")
					psr, psg, psb := scoreToRGB(a.Score)
					pdf.SetTextColor(psr, psg, psb)
					pdf.CellFormat(12, 4.5, pdfText(fmt.Sprintf("%d", a.Score)), "", 0, "L", false, 0, "")
					pdf.SetTextColor(100, 100, 100)
					var outcome string
					switch {
					case a.Of > 1:
						outcome = fmt.Sprintf("mentioned in %d of %d runs, recommended in %d", a.Mentioned, a.Of, a.Recommended)
					case a.Recommended > 0:
						outcome = "mentioned, recommended"
					case a.Mentioned > 0:
						outcome = "mentioned"
					default:
						outcome = "not mentioned"
					}
					pdf.CellFormat(0, 4.5, pdfText(outcome), "", 1, "L", false, 0, "")
				}
				pdf.Ln(1.5)
			}
		}

	} else {
		toc = append(toc, tocEntry{name: "LLM Brand Test", missing: true})
	}

	// --- 8. Recommendations (consolidated) ---
	type pdfRec struct {
		action   string
		priority string
		source   string // "General", "YouTube", "Reddit"
		impact   string
	}
	var allRecs []pdfRec
	seen := map[string]bool{}

	addRec := func(action, priority, source, impact string) {
		key := strings.ToLower(strings.TrimSpace(action))
		if seen[key] || key == "" {
			return
		}
		seen[key] = true
		if priority == "" {
			priority = "medium"
		}
		allRecs = append(allRecs, pdfRec{action: action, priority: priority, source: source, impact: impact})
	}

	// Active todos first (user-curated, highest importance)
	for _, t := range todos {
		if t.Status != "todo" {
			continue
		}
		source := "General"
		if t.SourceType == "video" {
			source = "YouTube"
		} else if t.SourceType == "reddit" {
			source = "Reddit"
		} else if t.SourceType == "search" {
			source = "Search"
		}
		addRec(t.Action, t.Priority, source, t.ExpectedImpact)
	}

	// Optimization report recommendations
	for _, opt := range optimizations {
		recs := opt.Result.Recommendations
		if len(recs) > 3 {
			recs = recs[:3]
		}
		for _, rec := range recs {
			addRec(rec.Action, rec.Priority, "General", rec.ExpectedImpact)
		}
	}

	// Video recommendations
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		for _, rec := range videoAnalysis.Result.Recommendations {
			addRec(rec.Action, rec.Priority, "YouTube", rec.ExpectedImpact)
		}
	}

	// Reddit recommendations
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		for _, rec := range redditAnalysis.Result.Recommendations {
			addRec(rec.Action, rec.Priority, "Reddit", rec.ExpectedImpact)
		}
	}

	// Search recommendations
	if searchAnalysis != nil && searchAnalysis.Result != nil {
		for _, rec := range searchAnalysis.Result.Recommendations {
			addRec(rec.Action, rec.Priority, "Search", rec.ExpectedImpact)
		}
	}

	if len(allRecs) > 0 {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		sort.Slice(allRecs, func(i, j int) bool {
			pi := priorityOrder[allRecs[i].priority]
			pj := priorityOrder[allRecs[j].priority]
			if pi != pj {
				return pi < pj
			}
			// Within same priority, group by source for readability
			return allRecs[i].source < allRecs[j].source
		})

		pdf.AddPage()
		toc = append(toc, tocEntry{name: "Recommendations", page: pdf.PageNo()})
		pdfSectionHeader(pdf, "Recommendations")

		// Disclaimer
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(100, 100, 100)
		pdf.MultiCell(0, 4.5, pdfText(pdfCleanText("The following recommendations are intended to improve visibility and authority specifically within large language models and AI-powered search systems. They do not necessarily reflect broader business constraints, priorities, or objectives, and should be considered alongside your overall strategy for digital presence and brand development.")), "", "L", false)
		pdf.Ln(6)

		currentPriority := ""
		for _, rec := range allRecs {
			if rec.priority != currentPriority {
				currentPriority = rec.priority
				pdfCheckPageBreak(pdf, 15)
				pdf.Ln(3)
				pdf.SetFont("Helvetica", "B", 11)
				priorityColor := map[string][3]int{
					"high":   {239, 68, 68},
					"medium": {245, 158, 11},
					"low":    {107, 114, 128},
				}
				c := priorityColor[currentPriority]
				if c == [3]int{} {
					c = [3]int{107, 114, 128}
				}
				pdf.SetTextColor(c[0], c[1], c[2])
				pdf.CellFormat(0, 7, pdfText(strings.ToUpper(currentPriority)+" PRIORITY"), "", 1, "L", false, 0, "")
				pdf.Ln(1)
			}

			pdfCheckPageBreak(pdf, 12)
			// Source tag + action
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(100, 100, 100)
			tagW := pdf.GetStringWidth("["+rec.source+"] ") + 2
			pdf.CellFormat(tagW, 5, pdfText("["+rec.source+"]"), "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(60, 60, 60)
			w, _ := pdf.GetPageSize()
			_, _, mr, _ := pdf.GetMargins()
			maxW := w - pdf.GetX() - mr
			pdf.MultiCell(maxW, 5, pdfText(pdfCleanText(rec.action)), "", "L", false)
			if rec.impact != "" {
				pdf.SetFont("Helvetica", "I", 9)
				pdf.SetTextColor(140, 140, 140)
				pdf.CellFormat(tagW, 4, pdfText(""), "", 0, "L", false, 0, "")
				impact := rec.impact
				if len(impact) > 100 {
					impact = impact[:97] + "..."
				}
				pdf.CellFormat(0, 4, pdfText("Impact: "+pdfCleanText(impact)), "", 1, "L", false, 0, "")
			}
		}
	} else {
		toc = append(toc, tocEntry{name: "Recommendations", missing: true})
	}

	// --- 9. Supporting Research ---
	pdf.AddPage()
	toc = append(toc, tocEntry{name: "Supporting Research", page: pdf.PageNo()})
	pdfSectionHeader(pdf, "Supporting Research")

	pdfSubHeader(pdf, "Research Digest")
	for _, para := range reportResearchDigest {
		pdfCheckPageBreak(pdf, 25)
		pdfBodyText(pdf, para)
	}

	pdf.Ln(4)
	pdfSubHeader(pdf, "Bibliography")

	for _, bib := range reportBibliography {
		pdfCheckPageBreak(pdf, 14)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(80, 80, 80)
		pdf.CellFormat(10, 5, pdfText(bib.ref), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(40, 40, 40)
		w, _ := pdf.GetPageSize()
		_, _, mr, _ := pdf.GetMargins()
		maxW := w - pdf.GetX() - mr
		pdf.MultiCell(maxW, 4.5, pdfText(fmt.Sprintf("%s. %s.", pdfCleanText(bib.title), bib.venue)), "", "L", false)
		pdf.SetX(pdf.GetX() + 10)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(100, 130, 200)
		pdf.CellFormat(0, 4, pdfText(bib.url), "", 1, "L", false, 0, "")
		pdf.Ln(1.5)
	}

	// --- Fill in Table of Contents (second pass) ---
	pdf.SetPage(tocPageNum)
	// Position after the header area
	pdf.SetY(42)

	for _, entry := range toc {
		pdf.SetFont("Helvetica", "", 11)
		if entry.missing {
			// Missing section: amber italic
			pdf.SetTextColor(160, 120, 40)
			pdf.SetFont("Helvetica", "I", 11)
			pdf.CellFormat(0, 7, pdfText(fmt.Sprintf("    %s  -  Not yet generated", entry.name)), "", 1, "L", false, 0, "")
		} else {
			pdf.SetTextColor(40, 40, 40)
			// Section name on left, page number on right
			nameW := pdf.GetStringWidth(entry.name)
			pageStr := fmt.Sprintf("%d", entry.page)
			pageW := pdf.GetStringWidth(pageStr)
			w, _ := pdf.GetPageSize()
			ml, _, mr, _ := pdf.GetMargins()
			available := w - ml - mr
			dotW := available - nameW - pageW - 8 // 8mm padding
			dots := ""
			if dotW > 0 {
				dotCount := int(dotW / pdf.GetStringWidth("."))
				for i := 0; i < dotCount; i++ {
					dots += "."
				}
			}
			pdf.CellFormat(0, 7, pdfText(fmt.Sprintf("    %s %s %s", entry.name, dots, pageStr)), "", 1, "L", false, 0, "")
		}
	}

	// --- Footer on all pages (with AutoPageBreak disabled to prevent blank pages) ---
	pdf.SetAutoPageBreak(false, 0)
	totalPages := pdf.PageCount()
	for i := 1; i <= totalPages; i++ {
		pdf.SetPage(i)
		_, h := pdf.GetPageSize()
		pdf.SetY(h - 12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(160, 160, 160)
		pdf.CellFormat(0, 5, pdfText(fmt.Sprintf("Page %d of %d  |  LLM Optimizer  |  %s", i, totalPages, displayDomain)), "", 0, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	if pdf.Err() {
		return nil, fmt.Errorf("PDF error: %s", pdf.Error())
	}
	return buf.Bytes(), nil
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// --- Handlers ---

func handleGeneratePDF(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		sendSSE(w, flusher, "status", map[string]string{"message": "Checking for cached report..."})

		// Compute current fingerprint (for this engine's design)
		fp := computeFingerprint(ctx, mongoDB, domain, r.Context())
		fp.Engine = reportEngineVersion

		if fp.OptimizationCount == 0 && !fp.AnalysisExists && !fp.VideoExists && !fp.RedditExists {
			sendSSE(w, flusher, "error", map[string]string{"message": "No reports found for this domain"})
			return
		}

		// Check cache
		var cached ReportPDF
		cacheFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		err := mongoDB.ReportPDFs().FindOne(ctx, cacheFilter).Decode(&cached)
		if err == nil && fingerprintEqual(cached.Fingerprint, fp) {
			sendSSE(w, flusher, "done", map[string]any{
				"pdf_id":     cached.ID.Hex(),
				"cached":     true,
				"size_bytes": cached.SizeBytes,
			})
			return
		}

		// Gather data and build the report (report_data.go, report_html.go)
		progress := func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		}
		data := loadReportData(ctx, mongoDB, domain, r.Context(), progress)
		progress(fmt.Sprintf("Found %d optimization reports. Generating the PDF report...", len(data.Optimizations)))
		pdfBytes, engine, err := generateReportPDF(ctx, data)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to generate PDF: " + err.Error()})
			return
		}
		fp.Engine = engine

		// Upsert into cache
		sendSSE(w, flusher, "status", map[string]string{"message": "Saving report..."})
		now := time.Now()
		tenantID := saas.TenantIDFromContext(r.Context())
		doc := ReportPDF{
			TenantID:    tenantID,
			Domain:      domain,
			PDFData:     pdfBytes,
			SizeBytes:   len(pdfBytes),
			Fingerprint: fp,
			GeneratedAt: now,
		}

		replaceFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		replaceUpsert := &options.ReplaceOptions{}
		replaceUpsert.SetUpsert(true)
		result, err := mongoDB.ReportPDFs().ReplaceOne(ctx, replaceFilter, doc, replaceUpsert)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to cache PDF: " + err.Error()})
			return
		}

		// Determine the ID
		pdfID := ""
		if result.UpsertedID != nil {
			if oid, ok := result.UpsertedID.(primitive.ObjectID); ok {
				pdfID = oid.Hex()
			}
		}
		if pdfID == "" {
			var existing ReportPDF
			if err := mongoDB.ReportPDFs().FindOne(ctx, replaceFilter, options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&existing); err == nil {
				pdfID = existing.ID.Hex()
			}
		}

		sendSSE(w, flusher, "done", map[string]any{
			"pdf_id":     pdfID,
			"cached":     false,
			"size_bytes": len(pdfBytes),
		})
	}
}

func handleServePDF(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		idStr := r.PathValue("id")

		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid PDF ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{
			{Key: "_id", Value: oid},
			{Key: "domain", Value: domain},
		})

		var pdf ReportPDF
		if err := mongoDB.ReportPDFs().FindOne(ctx, filter).Decode(&pdf); err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "PDF not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error", http.StatusInternalServerError)
			}
			return
		}

		displayDomain := strings.TrimPrefix(domain, "https://")
		displayDomain = strings.TrimPrefix(displayDomain, "http://")
		filename := displayDomain + "-llm-report.pdf"

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdf.PDFData)))
		w.Write(pdf.PDFData)
	}
}

func handleServeSharedPDF(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// Look up the share record
		var ds DomainShare
		err := mongoDB.DomainShares().FindOne(ctx, bson.M{
			"shareId":    shareID,
			"visibility": bson.M{"$in": []string{"public", "popular"}},
		}).Decode(&ds)
		if err == mongo.ErrNoDocuments {
			http.Error(w, `{"error":"share not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Find the cached PDF for this tenant+domain
		var pdf ReportPDF
		if err := mongoDB.ReportPDFs().FindOne(ctx, bson.M{
			"tenantId": ds.TenantID,
			"domain":   ds.Domain,
		}).Decode(&pdf); err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, `{"error":"no PDF report available"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			}
			return
		}

		displayDomain := strings.TrimPrefix(ds.Domain, "https://")
		displayDomain = strings.TrimPrefix(displayDomain, "http://")
		filename := displayDomain + "-llm-report.pdf"

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdf.PDFData)))
		w.Write(pdf.PDFData)
	}
}

// Ensure imports are used
var _ = json.Marshal

// handleReportHTML serves the report as a web page (the same design as the PDF).
func handleReportHTML(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		serveReportPage(w, loadReportData(ctx, mongoDB, domain, r.Context(), nil))
	}
}

// handleSharedReportHTML serves the web report for a public or popular share.
func handleSharedReportHTML(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		var ds DomainShare
		err := mongoDB.DomainShares().FindOne(ctx, bson.M{
			"shareId":    r.PathValue("shareId"),
			"visibility": bson.M{"$in": []string{"public", "popular"}},
		}).Decode(&ds)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "This shared report doesn't exist or is no longer shared.", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		// Load the report as the account that shared it.
		owner := saas.SetAuthContext(r.Context(), &saas.AuthInfo{TenantID: ds.TenantID})
		serveReportPage(w, loadReportData(ctx, mongoDB, ds.Domain, owner, nil))
	}
}

func serveReportPage(w http.ResponseWriter, d reportData) {
	if d.Analysis == nil && len(d.Optimizations) == 0 && d.VideoAnalysis == nil && d.RedditAnalysis == nil &&
		d.SearchAnalysis == nil && d.LLMTest == nil {
		http.Error(w, "No reports found for this domain yet.", http.StatusNotFound)
		return
	}
	html, err := renderReportHTML(buildReportView(d, time.Now(), "web"))
	if err != nil {
		log.Printf("Report: rendering the web report failed: %v", err)
		http.Error(w, "The report couldn't be rendered.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.Write([]byte(html))
}
