package main

// The report as HTML: one template (reporttpl/report.html) styled like the app,
// with score cards, bar charts and action cards. Chromium prints it to the
// downloadable PDF; the same page, in "web" mode, is the report's web version.
// When no Chromium is available (a local copy without Chrome), the PDF falls
// back to the plain gofpdf report (report_pdf.go).

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

//go:embed reporttpl/report.html reporttpl/fonts/*.woff2
var reportTplFS embed.FS

// reportEngineVersion marks PDFs made by this engine and scoring rubric, so
// cached PDFs from an older engine or older weights are regenerated (see
// ReportFingerprint.Engine). Bump the number with any template or weight change.
const reportEngineVersion = "html-2"

// reportWeakestShown is how many of the weakest optimized questions get a full card.
const reportWeakestShown = 10

// reportAnswerRecsShown is how many answer fixes the Recommendations section lists.
const reportAnswerRecsShown = 15

type reportScoreCard struct {
	Label     string
	Score     int
	Available bool
	Note      string // what it measures, or its weight
}

type reportBar struct {
	Label  string
	Value  int // 0–100, the bar's length
	Text   string
	Detail string
}

type reportRec struct {
	Action   string
	Priority string
	Source   string
	Impact   string
	score    int // the question's score, for answer fixes (weakest first)
}

type reportRecGroup struct {
	Priority string
	Label    string
	Items    []reportRec
}

type reportOptCard struct {
	Anchor      string
	Question    string
	Overall     int
	Dims        []reportScoreCard
	Competitors []reportCompetitor
	Recs        []reportRec
}

type reportCompetitor struct {
	Domain    string
	Score     int
	Strengths string
}

type reportTOC struct {
	ID      string
	Title   string
	Missing bool
}

type reportView struct {
	Mode         string // "pdf" or "web"
	BrandName    string
	Domain       string
	Generated    string
	Models       []string
	FontCSS      template.CSS
	Visibility   reportScoreCard
	VisLabel     string
	VisBasis     string
	Dimensions   []reportScoreCard
	TOC          []reportTOC
	TopActions   []reportRec
	Summary      *reportSummary
	Site         *reportSite
	Opt          *reportOpt
	Video        *reportVideo
	Reddit       *reportReddit
	Search       *reportSearch
	Test         *reportTest
	RecGroups    []reportRecGroup // across YouTube, Reddit and Search
	AnswerRecs   []reportRec      // the most important answer fixes
	MoreAnswers  int              // answer fixes not shown (they are on each question and in the To-Do list)
	RecCount     int
	Research     []string
	Bibliography []reportBib
}

type reportSummary struct {
	Text      string
	Average   int
	RangeLow  int
	RangeHigh int
	Themes    []SummaryTheme
}

type reportSite struct {
	Summary      string
	Questions    []Question
	PagesCrawled int
}

type reportOpt struct {
	Count    int
	Average  int
	Dims     []reportScoreCard
	Ranked   []reportOptRow
	Weakest  []reportOptCard
	Appendix []reportOptCard
}

type reportOptRow struct {
	Anchor   string
	Question string
	Score    int
}

type reportVideo struct {
	Overall    int
	Pillars    []reportScoreCard
	Summary    string
	Narrative  string
	Sentiment  *reportSentiment
	Themes     []string
	SOV        []reportBar
	Gaps       []reportRec
	Creators   []reportBar
	Targets    []reportRec
	Note       string
	Scorecards []reportBar
}

type reportSentiment struct {
	Total, Positive, Neutral, Negative int // percentages, except Total
}

type reportReddit struct {
	Overall  int
	Pillars  []reportScoreCard
	Summary  string
	SOV      []reportBar
	Mentions []RedditMentionExample
	Note     string
}

type reportSearch struct {
	Overall  int
	Pillars  []reportScoreCard
	Summary  string
	Crawlers []CrawlerStatus
	Note     string
}

type reportTest struct {
	Overall      int
	Providers    []LLMTestSummary
	Unavailable  string
	Queries      []reportTestQuery
	TotalQueries int
	Runs         int
	ScoreLow     int
	ScoreHigh    int
	HasRange     bool
	HasSearch    bool
	SearchScore  int
	Cited        []LLMTestCitedDomain
}

// reportTestQuery is one query's answers, one entry per assistant (its runs
// combined) plus one per assistant's web-search answer.
type reportTestQuery struct {
	Query   string
	Answers []reportTestAnswer
}

type reportTestAnswer struct {
	Provider               string
	Score                  int
	Mentioned, Recommended int // answers that mentioned / recommended the brand
	Of                     int // answers graded
	Search                 bool
}

// testQueryAnswers combines each assistant's graded answers to one query.
func testQueryAnswers(qr LLMTestQueryResult) []reportTestAnswer {
	var out []reportTestAnswer
	at := map[string]int{}
	for _, pr := range qr.ProviderResults {
		if pr.Unscored || strings.HasPrefix(pr.Response, "Error: ") {
			continue
		}
		key := pr.ProviderID
		if pr.Search {
			key += "/search"
		}
		i, ok := at[key]
		if !ok {
			i = len(out)
			at[key] = i
			out = append(out, reportTestAnswer{Provider: pr.ProviderName, Search: pr.Search})
		}
		a := &out[i]
		a.Score += pr.Score // summed here, averaged below
		a.Of++
		if pr.Mentioned {
			a.Mentioned++
		}
		if pr.Recommended {
			a.Recommended++
		}
	}
	for i := range out {
		out[i].Score /= out[i].Of
	}
	return out
}

type reportBib struct {
	Ref, Title, Venue, URL string
}

func scoreClass(score int) string {
	switch {
	case score >= 80:
		return "g"
	case score >= 60:
		return "a"
	case score >= 40:
		return "o"
	}
	return "r"
}

// scoreVerdict matches the app's labels (scoreLabel in App.tsx).
func scoreVerdict(score int) string {
	switch {
	case score >= 80:
		return "Strong position"
	case score >= 60:
		return "Moderate: room for improvement"
	case score >= 40:
		return "Weak: significant improvements needed"
	}
	return "Poor: immediate action required"
}

func clampPct(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// prose undoes the plain report's ASCII stand-ins for typography.
func prose(s string) string {
	return strings.ReplaceAll(s, " -- ", " — ")
}

var reportFuncs = template.FuncMap{
	"scoreClass": scoreClass,
	"verdict":    scoreVerdict,
	"pct":        clampPct,
	"prose":      prose,
	"inc":        func(i int) int { return i + 1 },
	"upper":      strings.ToUpper,
	"title": func(s string) string {
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	},
	"sentimentClass": func(s string) string {
		switch strings.ToLower(s) {
		case "positive":
			return "g"
		case "negative":
			return "r"
		}
		return "n"
	},
}

var reportTemplate = template.Must(template.New("report.html").Funcs(reportFuncs).ParseFS(reportTplFS, "reporttpl/report.html"))

// reportFontCSS embeds Inter so the report looks the same wherever it's rendered.
var reportFontCSS = func() template.CSS {
	var b strings.Builder
	for _, w := range []int{400, 500, 600, 700, 800} {
		data, err := reportTplFS.ReadFile(fmt.Sprintf("reporttpl/fonts/inter-latin-%d-normal.woff2", w))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "@font-face{font-family:'Inter';font-style:normal;font-weight:%d;font-display:block;src:url(data:font/woff2;base64,%s) format('woff2');}\n",
			w, base64.StdEncoding.EncodeToString(data))
	}
	return template.CSS(b.String())
}()

var dimensionNotes = map[string]string{
	"Optimization":      "How well your pages answer the questions people ask AI",
	"Video Authority":   "How YouTube content shapes what AI says about you",
	"Reddit Authority":  "How Reddit discussions shape what AI says about you",
	"Search Visibility": "Readiness for AI Overviews and AI crawlers",
	"LLM Test":          "What ChatGPT, Claude, Gemini and others say today",
}

func buildReportView(d reportData, now time.Time, mode string) reportView {
	v := reportView{Mode: mode, FontCSS: reportFontCSS}
	v.Domain = strings.TrimPrefix(strings.TrimPrefix(d.Domain, "https://"), "http://")
	v.BrandName = v.Domain
	if d.BrandProfile != nil && d.BrandProfile.BrandName != "" {
		v.BrandName = d.BrandProfile.BrandName
	}
	v.Generated = now.Format("January 2, 2006")

	// Models used anywhere in the report
	models := map[string]bool{}
	add := func(m string) {
		if m != "" {
			models[m] = true
		}
	}
	if d.Analysis != nil {
		add(d.Analysis.Model)
	}
	for _, o := range d.Optimizations {
		add(o.Model)
	}
	if d.VideoAnalysis != nil {
		add(d.VideoAnalysis.Model)
	}
	if d.RedditAnalysis != nil {
		add(d.RedditAnalysis.Model)
	}
	if d.SearchAnalysis != nil {
		add(d.SearchAnalysis.Model)
	}
	if d.LLMTest != nil {
		for _, ps := range d.LLMTest.ProviderSummaries {
			add(ps.Model)
		}
	}
	for m := range models {
		v.Models = append(v.Models, m)
	}
	sort.Strings(v.Models)

	// LLM Visibility Score: the same cards as the app's scorecard
	v.Visibility = reportScoreCard{Label: "LLM Visibility Score", Score: d.Visibility.Score, Available: d.Visibility.Available > 0}
	v.VisLabel = scoreVerdict(d.Visibility.Score)
	v.VisBasis = fmt.Sprintf("Based on %d of %d dimensions", d.Visibility.Available, d.Visibility.Total)
	for _, c := range d.Visibility.Components {
		v.Dimensions = append(v.Dimensions, reportScoreCard{
			Label: c.Name, Score: c.Score, Available: c.Available,
			Note: fmt.Sprintf("%d%% weight · %s", int(c.Weight*100+0.5), dimensionNotes[c.Name]),
		})
	}

	toc := func(id, title string, present bool) {
		v.TOC = append(v.TOC, reportTOC{ID: id, Title: title, Missing: !present})
	}

	// Executive summary
	toc("executive-summary", "Executive Summary", true)
	if s := d.Summary; s != nil {
		v.Summary = &reportSummary{Text: s.Result.ExecutiveSummary, Average: s.Result.AverageScore,
			RangeLow: s.Result.ScoreRange[0], RangeHigh: s.Result.ScoreRange[1], Themes: s.Result.Themes}
	}

	// Site analysis
	if a := d.Analysis; a != nil {
		v.Site = &reportSite{Summary: a.Result.SiteSummary, Questions: a.Result.Questions, PagesCrawled: len(a.Result.CrawledPages)}
	}
	toc("site-analysis", "Site Analysis", v.Site != nil)

	// Optimizations
	if len(d.Optimizations) > 0 {
		v.Opt = buildOptView(d.Optimizations)
	}
	title := "Answer Optimization"
	if v.Opt != nil {
		title = fmt.Sprintf("Answer Optimization (%d questions)", v.Opt.Count)
	}
	toc("optimization", title, v.Opt != nil)

	// YouTube
	if d.VideoAnalysis != nil && d.VideoAnalysis.Result != nil {
		v.Video = buildVideoView(d.VideoAnalysis.Result)
	}
	toc("youtube", "YouTube Video Authority", v.Video != nil)

	// Reddit
	if d.RedditAnalysis != nil && d.RedditAnalysis.Result != nil {
		rr := d.RedditAnalysis.Result
		rv := &reportReddit{Overall: rr.OverallScore, Summary: rr.ExecutiveSummary, Note: rr.ConfidenceNote,
			Pillars: []reportScoreCard{
				{Label: "Presence", Score: rr.Presence.Score, Available: true, Note: "How often you come up in relevant threads"},
				{Label: "Sentiment & Recommendations", Score: rr.Sentiment.Score, Available: true, Note: "How people talk about you, and whether they recommend you"},
				{Label: "Competitive Positioning", Score: rr.Competitive.Score, Available: true, Note: "How you compare with competitors in threads"},
				{Label: "Training Signal Strength", Score: rr.TrainingSignal.Score, Available: true, Note: "How likely these threads shape model knowledge"},
			}}
		for _, s := range rr.Presence.ShareOfVoice {
			rv.SOV = append(rv.SOV, reportBar{Label: s.BrandName, Value: clampPct(int(s.Percentage + 0.5)),
				Text: fmt.Sprintf("%.0f%%", s.Percentage), Detail: fmt.Sprintf("%d mention%s", s.MentionCount, pluralS(s.MentionCount))})
		}
		mentions := rr.Sentiment.NotableMentions
		if len(mentions) > 6 {
			mentions = mentions[:6]
		}
		rv.Mentions = mentions
		v.Reddit = rv
	}
	toc("reddit", "Reddit Authority", v.Reddit != nil)

	// Search
	if d.SearchAnalysis != nil && d.SearchAnalysis.Result != nil {
		sr := d.SearchAnalysis.Result
		sv := &reportSearch{Overall: sr.OverallScore, Summary: sr.ExecutiveSummary, Note: sr.ConfidenceNote, Crawlers: sr.CrawlAccess.CrawlerDetails,
			Pillars: []reportScoreCard{
				{Label: "AI Overview Readiness", Score: sr.AIOReadiness.Score, Available: true, Note: "Likelihood of appearing in Google's AI Overviews"},
				{Label: "Crawl Accessibility", Score: sr.CrawlAccess.Score, Available: true, Note: "Whether AI crawlers can reach your content"},
				{Label: "Brand Search Momentum", Score: sr.BrandMomentum.Score, Available: true, Note: "The trend in searches for your brand"},
				{Label: "Content Freshness", Score: sr.ContentFreshness.Score, Available: true, Note: "How recent your key content is"},
			}}
		if sr.CategoryDiscovery != nil {
			sv.Pillars = append(sv.Pillars, reportScoreCard{Label: "Category Discovery", Score: sr.CategoryDiscovery.Score, Available: true,
				Note: "Visibility for category searches that don't name you"})
		}
		v.Search = sv
	}
	toc("search", "Search Visibility", v.Search != nil)

	// LLM Test
	if t := d.LLMTest; t != nil && len(t.ProviderSummaries) > 0 {
		tv := &reportTest{Overall: t.OverallScore, Providers: t.ProviderSummaries, TotalQueries: len(t.Results),
			Runs: max(t.Runs, 1), Cited: t.TopCitedDomains}
		if t.SearchScore != nil {
			tv.HasSearch, tv.SearchScore = true, *t.SearchScore
		}
		if len(t.ScoreRange) == 2 {
			tv.ScoreLow, tv.ScoreHigh, tv.HasRange = t.ScoreRange[0], t.ScoreRange[1], true
		}
		if len(t.UnavailableProviders) > 0 {
			var notes []string
			for _, u := range t.UnavailableProviders {
				notes = append(notes, fmt.Sprintf("%s (%s)", u.ProviderName, u.Reason))
			}
			tv.Unavailable = strings.Join(notes, "; ")
		}
		qs := t.Results
		if len(qs) > 15 {
			qs = qs[:15]
		}
		for _, qr := range qs {
			tv.Queries = append(tv.Queries, reportTestQuery{Query: qr.Query.Query, Answers: testQueryAnswers(qr)})
		}
		v.Test = tv
	}
	toc("llm-test", "LLM Brand Test", v.Test != nil)

	// Recommendations
	recs := collectReportRecs(d)
	v.RecCount = len(recs)
	var cross, answers []reportRec
	for _, r := range recs {
		if r.Source == "Answers" {
			answers = append(answers, r)
		} else {
			cross = append(cross, r)
		}
	}
	for _, p := range []struct{ key, label string }{{"high", "High priority"}, {"medium", "Medium priority"}, {"low", "Low priority"}} {
		var items []reportRec
		for _, r := range cross {
			if r.Priority == p.key {
				items = append(items, r)
			}
		}
		if len(items) > 0 {
			v.RecGroups = append(v.RecGroups, reportRecGroup{Priority: p.key, Label: p.label, Items: items})
		}
	}
	// Answer fixes: most important first, from the weakest questions; the rest
	// are on each question's card and in the app's To-Do list.
	order := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.SliceStable(answers, func(i, j int) bool {
		if order[answers[i].Priority] != order[answers[j].Priority] {
			return order[answers[i].Priority] < order[answers[j].Priority]
		}
		return answers[i].score < answers[j].score
	})
	if len(answers) > reportAnswerRecsShown {
		v.MoreAnswers = len(answers) - reportAnswerRecsShown
		answers = answers[:reportAnswerRecsShown]
	}
	v.AnswerRecs = answers
	toc("recommendations", "Recommendations", len(recs) > 0)

	// Top 3 actions for the executive summary: the summary's own action items
	// first (they weigh every report together), else the top recommendations.
	if d.Summary != nil && len(d.Summary.Result.ActionItems) > 0 {
		items := append([]SummaryActionItem(nil), d.Summary.Result.ActionItems...)
		order := map[string]int{"high": 0, "medium": 1, "low": 2}
		sort.SliceStable(items, func(i, j int) bool {
			return order[strings.ToLower(items[i].Priority)] < order[strings.ToLower(items[j].Priority)]
		})
		for _, a := range items {
			if len(v.TopActions) == 3 {
				break
			}
			v.TopActions = append(v.TopActions, reportRec{Action: a.Action, Priority: strings.ToLower(a.Priority), Impact: a.ExpectedImpact, Source: a.Dimension})
		}
	}
	for _, r := range recs {
		if len(v.TopActions) == 3 {
			break
		}
		v.TopActions = append(v.TopActions, r)
	}

	if v.Opt != nil && len(v.Opt.Appendix) > 0 {
		toc("appendix", "Appendix: Other Optimized Questions", true)
	}
	toc("research", "Supporting Research", true)
	for _, p := range reportResearchDigest {
		v.Research = append(v.Research, prose(p))
	}
	for _, b := range reportBibliography {
		v.Bibliography = append(v.Bibliography, reportBib{Ref: b.ref, Title: prose(b.title), Venue: b.venue, URL: b.url})
	}
	return v
}

func buildOptView(opts []Optimization) *reportOpt {
	ov := &reportOpt{Count: len(opts)}
	sums := [5]int{}
	for _, o := range opts {
		r := o.Result
		sums[0] += r.OverallScore
		sums[1] += r.ContentAuthority.Score
		sums[2] += r.StructuralOptimization.Score
		sums[3] += r.SourceAuthority.Score
		sums[4] += r.KnowledgePersistence.Score
	}
	n := len(opts)
	ov.Average = sums[0] / n
	ov.Dims = []reportScoreCard{
		{Label: "Content Authority", Score: sums[1] / n, Available: true, Note: "Depth, accuracy and expertise of your answers"},
		{Label: "Structural Optimization", Score: sums[2] / n, Available: true, Note: "How easily AI can extract and quote your pages"},
		{Label: "Source Authority", Score: sums[3] / n, Available: true, Note: "Third-party signals that back you up"},
		{Label: "Knowledge Persistence", Score: sums[4] / n, Available: true, Note: "How durably the answer sticks in model knowledge"},
	}

	sorted := append([]Optimization(nil), opts...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Result.OverallScore < sorted[j].Result.OverallScore })
	for i, o := range sorted {
		anchor := fmt.Sprintf("q-%d", i+1)
		ov.Ranked = append(ov.Ranked, reportOptRow{Anchor: anchor, Question: o.Question, Score: o.Result.OverallScore})
		card := reportOptCard{Anchor: anchor, Question: o.Question, Overall: o.Result.OverallScore,
			Dims: []reportScoreCard{
				{Label: "Content Authority", Score: o.Result.ContentAuthority.Score, Available: true},
				{Label: "Structural", Score: o.Result.StructuralOptimization.Score, Available: true},
				{Label: "Source Authority", Score: o.Result.SourceAuthority.Score, Available: true},
				{Label: "Knowledge Persistence", Score: o.Result.KnowledgePersistence.Score, Available: true},
			}}
		comps := o.Result.Competitors
		maxComps := 3
		if i >= reportWeakestShown {
			maxComps = 2 // the appendix stays compact
		}
		if len(comps) > maxComps {
			comps = comps[:maxComps]
		}
		for _, c := range comps {
			card.Competitors = append(card.Competitors, reportCompetitor{Domain: c.Domain, Score: c.ScoreEstimate, Strengths: c.Strengths})
		}
		recs := o.Result.Recommendations
		if len(recs) > 3 {
			recs = recs[:3]
		}
		for _, r := range recs {
			card.Recs = append(card.Recs, reportRec{Action: r.Action, Priority: strings.ToLower(r.Priority), Impact: r.ExpectedImpact})
		}
		if i < reportWeakestShown {
			ov.Weakest = append(ov.Weakest, card)
		} else {
			card.Recs = nil // the appendix stays compact
			ov.Appendix = append(ov.Appendix, card)
		}
	}
	return ov
}

func buildVideoView(vr *VideoAuthorityResult) *reportVideo {
	vv := &reportVideo{Overall: vr.OverallScore, Summary: vr.ExecutiveSummary, Narrative: vr.BrandNarrative.NarrativeSummary,
		Themes: vr.BrandNarrative.KeyThemes, Note: vr.ConfidenceNote,
		Pillars: []reportScoreCard{
			{Label: "Transcript Authority", Score: vr.TranscriptAuthority.Score, Available: true, Note: "How quotable and on-topic your video transcripts are"},
			{Label: "Topical Dominance", Score: vr.TopicalDominance.Score, Available: true, Note: "Your share of the YouTube conversation in your category"},
			{Label: "Citation Network", Score: vr.CitationNetwork.Score, Available: true, Note: "Creators and channels who mention you"},
			{Label: "Brand Narrative", Score: vr.BrandNarrative.Score, Available: true, Note: "What the videos collectively say about you"},
		}}
	if s := vr.BrandNarrative.Sentiment; s.Total > 0 {
		vv.Sentiment = &reportSentiment{Total: s.Total, Positive: s.Positive * 100 / s.Total, Neutral: s.Neutral * 100 / s.Total, Negative: s.Negative * 100 / s.Total}
	}
	for _, s := range vr.TopicalDominance.ShareOfVoice {
		vv.SOV = append(vv.SOV, reportBar{Label: s.BrandName, Value: clampPct(int(s.Percentage + 0.5)),
			Text: fmt.Sprintf("%.0f%%", s.Percentage), Detail: fmt.Sprintf("%d mention%s", s.MentionCount, pluralS(s.MentionCount))})
	}
	for _, g := range vr.TopicalDominance.ContentGaps {
		vv.Gaps = append(vv.Gaps, reportRec{Action: g.Query, Impact: g.Recommendation})
	}
	for _, c := range vr.CitationNetwork.TopCreators {
		vv.Creators = append(vv.Creators, reportBar{Label: c.ChannelTitle, Value: clampPct(c.AuthorityScore), Text: fmt.Sprintf("%d", c.AuthorityScore), Detail: c.Role})
	}
	for _, t := range vr.CitationNetwork.CreatorTargets {
		vv.Targets = append(vv.Targets, reportRec{Action: t.ChannelTitle, Impact: t.OutreachReason})
	}
	cards := vr.VideoScorecards
	if len(cards) > 10 {
		cards = cards[:10]
	}
	for _, c := range cards {
		vv.Scorecards = append(vv.Scorecards, reportBar{Label: c.Title, Value: clampPct(c.OverallScore), Text: fmt.Sprintf("%d", c.OverallScore)})
	}
	return vv
}

// collectReportRecs gathers recommendations from every report, active to-dos
// first, without duplicates, ordered by priority then source.
func collectReportRecs(d reportData) []reportRec {
	var recs []reportRec
	seen := map[string]bool{}
	scoreOf := map[string]int{} // question -> its optimization score
	for _, o := range d.Optimizations {
		scoreOf[o.Question] = o.Result.OverallScore
	}
	lastScore := 101
	add := func(action, priority, source, impact string) {
		key := strings.ToLower(strings.TrimSpace(action))
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		priority = strings.ToLower(priority)
		if priority != "high" && priority != "low" {
			priority = "medium"
		}
		recs = append(recs, reportRec{Action: action, Priority: priority, Source: source, Impact: impact, score: lastScore})
	}
	for _, t := range d.Todos {
		if t.Status != "todo" {
			continue
		}
		source := "Answers"
		switch t.SourceType {
		case "video":
			source = "YouTube"
		case "reddit":
			source = "Reddit"
		case "search":
			source = "Search"
		}
		lastScore = 101
		if s, ok := scoreOf[t.Question]; ok {
			lastScore = s
		}
		add(t.Action, t.Priority, source, t.ExpectedImpact)
	}
	for _, o := range d.Optimizations {
		rs := o.Result.Recommendations
		if len(rs) > 3 {
			rs = rs[:3]
		}
		lastScore = o.Result.OverallScore
		for _, r := range rs {
			add(r.Action, r.Priority, "Answers", r.ExpectedImpact)
		}
	}
	lastScore = 101
	if d.VideoAnalysis != nil && d.VideoAnalysis.Result != nil {
		for _, r := range d.VideoAnalysis.Result.Recommendations {
			add(r.Action, r.Priority, "YouTube", r.ExpectedImpact)
		}
	}
	if d.RedditAnalysis != nil && d.RedditAnalysis.Result != nil {
		for _, r := range d.RedditAnalysis.Result.Recommendations {
			add(r.Action, r.Priority, "Reddit", r.ExpectedImpact)
		}
	}
	if d.SearchAnalysis != nil && d.SearchAnalysis.Result != nil {
		for _, r := range d.SearchAnalysis.Result.Recommendations {
			add(r.Action, r.Priority, "Search", r.ExpectedImpact)
		}
	}
	order := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.SliceStable(recs, func(i, j int) bool {
		if order[recs[i].Priority] != order[recs[j].Priority] {
			return order[recs[i].Priority] < order[recs[j].Priority]
		}
		return recs[i].Source < recs[j].Source
	})
	return recs
}

func renderReportHTML(v reportView) (string, error) {
	var buf bytes.Buffer
	if err := reportTemplate.Execute(&buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// reportFooter is Chromium's page footer (header/footer templates can't use
// the page's fonts or styles).
func reportFooter(v reportView) string {
	esc := template.HTMLEscapeString
	return `<div style="width:100%;font-family:Helvetica,Arial,sans-serif;font-size:7.5px;color:#94a3b8;padding:0 14mm;display:flex;justify-content:space-between;">` +
		`<span>` + esc(v.BrandName) + ` · ` + esc(v.Domain) + `</span><span>AI Visibility Report · LLM Optimizer</span>` +
		`<span>Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></div>`
}

// htmlToPDF prints a page with headless Chromium (LLMOPT_CHROME_PATH, or the
// usual install locations).
func htmlToPDF(ctx context.Context, html, footer string) ([]byte, error) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
	}
	if p := os.Getenv("LLMOPT_CHROME_PATH"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	bctx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	var pdf []byte
	err := chromedp.Run(bctx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			tree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(tree.Frame.ID, html).Do(ctx)
		}),
		chromedp.Evaluate(`document.fonts.ready.then(() => true)`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				WithDisplayHeaderFooter(true).
				WithHeaderTemplate("<span></span>").
				WithFooterTemplate(footer).
				WithGenerateDocumentOutline(true).
				WithGenerateTaggedPDF(true).
				Do(ctx)
			pdf = buf
			return err
		}),
	)
	if err != nil {
		return nil, err
	}
	return pdf, nil
}

// generateReportPDF makes the downloadable report: the HTML report printed by
// Chromium, or the plain report when Chromium isn't available. engine says which.
func generateReportPDF(ctx context.Context, d reportData) (pdf []byte, engine string, err error) {
	view := buildReportView(d, time.Now(), "pdf")
	html, err := renderReportHTML(view)
	if err == nil {
		pdf, err = htmlToPDF(ctx, html, reportFooter(view))
		if err == nil {
			return pdf, reportEngineVersion, nil
		}
		log.Printf("Report: Chromium couldn't print the report (%v); using the plain PDF", err)
	} else {
		log.Printf("Report: rendering the HTML report failed (%v); using the plain PDF", err)
	}
	pdf, err = buildReportPDF(d.Domain, d.BrandProfile, d.Analysis, d.Optimizations, d.VideoAnalysis, d.RedditAnalysis,
		d.SearchAnalysis, d.LLMTest, d.Summary, d.Todos)
	return pdf, "plain", err
}
