package main

// Updating the advice in saved results to the current research rubric without
// re-running them. Scores stay as they are (the search weights are already
// applied when results are read, scoring.go); what changes is the written
// advice: recommendations, improvements, summaries and evidence lines that
// rest on findings the research no longer supports are rewritten into their
// current equivalent or removed, and open to-dos built on them are reworded or
// archived. Each document is copied to guidance_backups before it changes, and
// gets a GuidanceRevision saying when and what changed.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"llmopt/internal/saas"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// currentGuidanceRules is what the research supports now, for the reviser.
const currentGuidanceRules = `Advice the 2026 research no longer supports (rewrite into the current equivalent, or remove):
- Adding quotations or statistics to gain a stated visibility percentage (+41%, +33% and similar figures came from a 2023 lab study that live engines don't reproduce).
- FAQ blocks, FAQ schema, summary boxes or page speed as ways to get cited by AI (they only appear to help across different websites, not for pages of the same site).
- Schema markup as an AI citation lever (it still helps rich results and entity understanding; say that, not citation).
- llms.txt (AI crawlers almost never request it).
- Unblocking Google-Extended to gain AI Overview visibility (per Google it doesn't affect Google Search or AI Overviews; it controls Gemini training and grounding).
- Treating robots.txt as a way to keep AI crawlers out (it states intent; some agents ignore it; real blocking needs network-level controls such as Cloudflare's).
- Counting a brand's own "best X" or self-ranked lists as third-party coverage, or recommending publishing them to win recommendations (assistants often cite them while recommending a competitor).
- Heavy investment in Reddit as the second-largest AI citation source (its citation share fell sharply in 2026; it is read far more than cited; genuine participation is still worthwhile).
- Tracking rank position in AI answer lists (list order changes from run to run).
- Outdated figures stated as current: "76% of AI Overview citations come from the top 10" (about 38% in 2026), "citation accuracy is 49-68%" (2026: 11% of AI Overview claims unsupported; 45% of assistant news answers had a significant issue), Reddit licensing deals at $60M/$70M as current, "GPTBot grew 305%", "Reddit is 40.1% of AI Overview citations", "AI ignores social content" (YouTube is the most-cited domain in AI Overviews).

Current advice to use instead:
- Make the page title and opening paragraph answer the buyer's question as they ask it; answer directly and early.
- Give the evidence assistants use: figures, definitions, comparisons, steps, explicit current prices; cite sources.
- Keep pricing and plan details explicit and current, and audit older pages that state outdated ones (pricing is what AI answers most often get wrong).
- Build third-party presence where AI answers draw from: Wikipedia, independent review and comparison sites, YouTube creators, genuine community participation.
- Allow the crawlers that fetch pages for AI answers (OAI-SearchBot, ChatGPT-User, Claude-SearchBot, PerplexityBot, Googlebot, Bingbot); blocking training crawlers is a business choice; check Cloudflare's AI bot setting.
- Refresh key product, pricing and comparison pages at least yearly, with honest update dates.
- Judge AI visibility by mention rate over repeated runs, per engine, against competitors of similar size.

How these show up in older results, and how to fix each (most results written before November 2026 contain several; check every improvement, recommendation, evidence line and summary sentence):
1. A summary box ("Key Takeaways", "Quick Answer", "TL;DR", "At a glance", "summary box") recommended as an AI extraction, RAG or citation target. Rewrite as: make the page's title and opening paragraph answer the question directly, in two or three sentences with the specific facts. Keep the site-specific facts the item names.
2. FAQ schema, FAQPage, or other Schema.org / structured-data markup recommended for AI citation, AI extraction, RAG or "LLM parsing". Remove the markup advice, or keep it only with its real purpose stated (search rich results, entity understanding). An FAQ page or section that answers real buyer questions in plain text is fine content advice; keep that part.
3. Adding quotations (expert, clinician or customer quotes) or statistics to raise AI visibility, authority for AI, or the odds of citation. Rewrite as: back the answer with specific, sourced facts (figures, named studies, dates, prices) and say who wrote or medically reviewed the page. Naming a real reviewer stays; "add a quote to get cited" goes.
4. Page speed, Core Web Vitals or "machine readability" framed as an AI citation lever. Remove the AI-citation framing.
5. Any stated uplift ("+41%", "+33%", "2x more likely to be cited", "increases AI citation by X%"). Remove the figure.
6. Evidence lines that count a missing FAQ schema, summary box, quotation or llms.txt against the site. Reword them as neutral observations or remove them.`

// reviseInstructions frames every revision request.
const reviseInstructions = `You are updating saved analysis results for one website so their advice matches current research. The analysis is not being re-run: keep its findings about this site.

%s

Rules:
- Change text that relies on unsupported advice or outdated figures, including the patterns listed above. Leave everything else exactly as written, word for word.
- When an item recommends an unsupported tactic, rewrite it into the current advice that serves the same goal for this site, keeping its site-specific details; remove it only if nothing current serves that goal.
- Keep priorities, dimensions and ids as given. Don't add new items, don't invent facts about the site, and don't change scores (there are none here).
- Record every change in a "changes" list, one short plain sentence each (for example: "Replaced 'add FAQ schema to win AI citations' with: match the page title to the buyer's question"). When the input has "items", put each item's own "changes" list inside that item. Use an empty list only if nothing in it needed changing.

Return ONLY a JSON object with exactly the structure of the input, with the "changes" lists added, no markdown fencing.

%s:
%s`

// guidanceRefreshBatch is how many optimizations one revision call handles,
// and guidanceRefreshParallel how many calls run at once. The reviser echoes
// every list it's given, so one optimization per call keeps each reply well
// inside the providers' per-call time limit (four per call ran past it).
const (
	guidanceRefreshBatch    = 1
	guidanceRefreshParallel = 4
)

type dimGuidance struct {
	Evidence     []string `json:"evidence"`
	Improvements []string `json:"improvements"`
}

type optGuidance struct {
	ID                     string           `json:"id"`
	Question               string           `json:"question,omitempty"`
	ContentAuthority       dimGuidance      `json:"content_authority"`
	StructuralOptimization dimGuidance      `json:"structural_optimization"`
	SourceAuthority        dimGuidance      `json:"source_authority"`
	KnowledgePersistence   dimGuidance      `json:"knowledge_persistence"`
	Recommendations        []Recommendation `json:"recommendations"`
	Changes                []string         `json:"changes,omitempty"`
}

type searchGuidance struct {
	ExecutiveSummary  string                 `json:"executive_summary"`
	ConfidenceNote    string                 `json:"confidence_note"`
	AIOEvidence       []string               `json:"aio_readiness_evidence"`
	CrawlEvidence     []string               `json:"crawl_accessibility_evidence"`
	RobotsTxtPolicy   string                 `json:"robots_txt_policy"`
	MomentumEvidence  []string               `json:"brand_momentum_evidence"`
	FreshnessEvidence []string               `json:"content_freshness_evidence"`
	Recommendations   []SearchRecommendation `json:"recommendations"`
	Changes           []string               `json:"changes,omitempty"`
}

type narrativeGuidance[R any] struct {
	ExecutiveSummary string   `json:"executive_summary"`
	ConfidenceNote   string   `json:"confidence_note"`
	Recommendations  []R      `json:"recommendations"`
	Changes          []string `json:"changes,omitempty"`
}

type todoGuidance struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	Summary        string `json:"summary"`
	ExpectedImpact string `json:"expected_impact"`
	// Decision (in the reply): "keep", "rewrite" or "drop"; Reason says why.
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// reviseFunc sends one prompt to the reviser and returns its reply.
type reviseFunc func(ctx context.Context, prompt string) (string, error)

// revise asks the reviser to update payload (described by what) and decodes
// its reply into out.
func revise(ctx context.Context, call reviseFunc, what string, payload, out any) error {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	reply, err := call(ctx, fmt.Sprintf(reviseInstructions, currentGuidanceRules, what, string(b)))
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(stripJSONFencing(reply)), out); err == nil {
		return nil
	}
	if j := extractJSON(reply); j != "" {
		if err := json.Unmarshal([]byte(j), out); err == nil {
			return nil
		}
	}
	return fmt.Errorf("the reviser's reply couldn't be read")
}

// guidancePass versions the reviser's instructions: results revised with an
// older pass are revised again. Pass 2 lists how retired advice shows up in
// older results (pass 1 named the ideas only and left most of it in place).
const guidancePass = 2

// guidanceCurrent reports whether a result already follows the current rubric
// and reviser instructions.
func guidanceCurrent(resultRubric int, g *GuidanceRevision) bool {
	return resultRubric >= scoringRubricVersion || (g != nil && g.Rubric >= scoringRubricVersion && g.Pass >= guidancePass)
}

func guidanceMark(changes []string) *GuidanceRevision {
	return &GuidanceRevision{Rubric: scoringRubricVersion, Pass: guidancePass, RevisedAt: time.Now(), Changes: changes}
}

// keepList keeps the reviser's list unless it dropped everything from a list
// that had items and gave no reason (a reply that lost the field).
func keepList[T any](orig, revised []T, changes []string) []T {
	if len(revised) == 0 && len(orig) > 0 && len(changes) == 0 {
		return orig
	}
	if revised == nil {
		return []T{}
	}
	return revised
}

// diffNotes describes how a list changed, for when the reviser doesn't say:
// items rewritten in place, removed, or added.
func diffNotes(label string, before, after []string) []string {
	short := func(t string) string { return `"` + truncateStr(strings.TrimSpace(t), 110) + `"` }
	var notes []string
	if len(before) == len(after) {
		for i := range before {
			if strings.TrimSpace(before[i]) != strings.TrimSpace(after[i]) {
				notes = append(notes, fmt.Sprintf("%s: rewrote %s as %s", label, short(before[i]), short(after[i])))
			}
		}
		return notes
	}
	inAfter, inBefore := map[string]bool{}, map[string]bool{}
	for _, a := range after {
		inAfter[strings.TrimSpace(a)] = true
	}
	for _, b := range before {
		inBefore[strings.TrimSpace(b)] = true
		if !inAfter[strings.TrimSpace(b)] {
			notes = append(notes, fmt.Sprintf("%s: removed %s", label, short(b)))
		}
	}
	for _, a := range after {
		if !inBefore[strings.TrimSpace(a)] {
			notes = append(notes, fmt.Sprintf("%s: added %s", label, short(a)))
		}
	}
	return notes
}

// textNote notes a rewritten text field.
func textNote(label, before, after string) []string {
	if strings.TrimSpace(after) == "" || strings.TrimSpace(before) == strings.TrimSpace(after) {
		return nil
	}
	return []string{"Revised the " + label}
}

// recActions lists recommendations' actions and expected impacts, to diff.
func recActions[R any](recs []R, get func(R) string) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = get(r)
	}
	return out
}

// orNotes keeps the reviser's own notes, or the computed ones if it gave none.
func orNotes(given, computed []string) []string {
	if len(given) > 0 {
		return given
	}
	return computed
}

func keepText(orig, revised string) string {
	if strings.TrimSpace(revised) == "" {
		return orig
	}
	return revised
}

// backupDoc copies a document to guidance_backups before it changes.
func backupDoc(ctx context.Context, mongoDB *MongoDB, collection string, id primitive.ObjectID, tenantID, domain string) error {
	var orig bson.M
	if err := mongoDB.Database.Collection(collection).FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&orig); err != nil {
		return err
	}
	_, err := mongoDB.GuidanceBackups().InsertOne(ctx, bson.M{
		"collection": collection, "docId": id, "tenantId": tenantID, "domain": domain,
		"rubric": scoringRubricVersion, "createdAt": time.Now(), "original": orig,
	})
	return err
}

// guidanceStatus counts a domain's results whose advice predates the rubric.
type guidanceStatus struct {
	Optimizations int        `json:"optimizations"`
	Search        int        `json:"search"`
	Video         int        `json:"video"`
	Reddit        int        `json:"reddit"`
	Todos         int        `json:"todos"`
	Pending       int        `json:"pending"`
	Revised       int        `json:"revised"`
	RevisedAt     *time.Time `json:"revised_at,omitempty"`
	Rubric        int        `json:"rubric"`
}

// guidanceWork is everything a refresh needs to look at for one domain.
type guidanceWork struct {
	opts   []Optimization
	search *SearchAnalysis
	video  *VideoAnalysis
	reddit *RedditAnalysis
	todos  []TodoItem
	status guidanceStatus
}

func loadGuidanceWork(ctx context.Context, mongoDB *MongoDB, filter bson.D) (guidanceWork, error) {
	var w guidanceWork
	w.status.Rubric = scoringRubricVersion
	own := func(extra ...bson.E) bson.D { return append(append(bson.D{}, filter...), extra...) }
	noRaw := bson.D{{Key: "rawText", Value: 0}}
	note := func(g *GuidanceRevision) {
		if g != nil && g.Rubric >= scoringRubricVersion {
			w.status.Revised++
			if w.status.RevisedAt == nil || g.RevisedAt.After(*w.status.RevisedAt) {
				t := g.RevisedAt
				w.status.RevisedAt = &t
			}
		}
	}

	var opts []Optimization
	cur, err := mongoDB.Optimizations().Find(ctx, own(), options.Find().SetProjection(noRaw).SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return w, err
	}
	if err := cur.All(ctx, &opts); err != nil {
		return w, err
	}
	for _, o := range opts {
		note(o.Guidance)
		if !guidanceCurrent(o.Result.RubricVersion, o.Guidance) {
			w.opts = append(w.opts, o)
		}
	}
	w.status.Optimizations = len(w.opts)

	latest := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(noRaw)
	var sa SearchAnalysis
	if mongoDB.SearchAnalyses().FindOne(ctx, own(), latest).Decode(&sa) == nil && sa.Result != nil {
		note(sa.Guidance)
		if !guidanceCurrent(sa.Result.RubricVersion, sa.Guidance) {
			w.search, w.status.Search = &sa, 1
		}
	}
	var va VideoAnalysis
	if mongoDB.VideoAnalyses().FindOne(ctx, own(), latest).Decode(&va) == nil && va.Result != nil {
		note(va.Guidance)
		if !guidanceCurrent(va.Result.RubricVersion, va.Guidance) {
			w.video, w.status.Video = &va, 1
		}
	}
	var ra RedditAnalysis
	if mongoDB.RedditAnalyses().FindOne(ctx, own(), latest).Decode(&ra) == nil && ra.Result != nil {
		note(ra.Guidance)
		if !guidanceCurrent(ra.Result.RubricVersion, ra.Guidance) {
			w.reddit, w.status.Reddit = &ra, 1
		}
	}

	var todos []TodoItem
	cur, err = mongoDB.Todos().Find(ctx, own(bson.E{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"todo", "backlogged"}}}}))
	if err != nil {
		return w, err
	}
	if err := cur.All(ctx, &todos); err != nil {
		return w, err
	}
	for _, t := range todos {
		if !guidanceCurrent(0, t.Guidance) {
			w.todos = append(w.todos, t)
		}
	}
	w.status.Todos = len(w.todos)
	w.status.Pending = w.status.Optimizations + w.status.Search + w.status.Video + w.status.Reddit + w.status.Todos
	return w, nil
}

// handleGuidanceStatus: GET /api/domains/{domain}/guidance — what still
// predates the current rubric.
func handleGuidanceStatus(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		work, err := loadGuidanceWork(ctx, mongoDB, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}))
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(work.status)
	}
}

// guidanceResult tallies one refresh.
type guidanceResult struct {
	Updated  int      `json:"updated"`
	Changes  int      `json:"changes"`
	Archived int      `json:"archived_todos"`
	Failed   []string `json:"failed,omitempty"`
}

// refreshGuidance updates every result in work. notify reports progress.
func refreshGuidance(ctx context.Context, mongoDB *MongoDB, call reviseFunc, work guidanceWork, tenantID, domain string, notify func(string)) guidanceResult {
	var res guidanceResult
	var mu sync.Mutex
	fail := func(what string, err error) {
		log.Printf("Guidance refresh %s (%s): %v", domain, what, err)
		mu.Lock()
		res.Failed = append(res.Failed, what)
		mu.Unlock()
	}
	done := func(changes []string) {
		mu.Lock()
		res.Updated++
		res.Changes += len(changes)
		mu.Unlock()
	}

	// Optimizations, guidanceRefreshBatch per call, guidanceRefreshParallel at a time.
	optsDone := 0
	var batches [][]Optimization
	for i := 0; i < len(work.opts); i += guidanceRefreshBatch {
		batches = append(batches, work.opts[i:min(i+guidanceRefreshBatch, len(work.opts))])
	}
	sem := make(chan struct{}, guidanceRefreshParallel)
	var wg sync.WaitGroup
	for bi, batch := range batches {
		wg.Add(1)
		go func(bi int, batch []Optimization) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var in struct {
				Items []optGuidance `json:"items"`
			}
			byID := map[string]Optimization{}
			for _, o := range batch {
				id := o.ID.Hex()
				byID[id] = o
				r := o.Result
				in.Items = append(in.Items, optGuidance{ID: id, Question: o.Question,
					ContentAuthority:       dimGuidance{r.ContentAuthority.Evidence, r.ContentAuthority.Improvements},
					StructuralOptimization: dimGuidance{r.StructuralOptimization.Evidence, r.StructuralOptimization.Improvements},
					SourceAuthority:        dimGuidance{r.SourceAuthority.Evidence, r.SourceAuthority.Improvements},
					KnowledgePersistence:   dimGuidance{r.KnowledgePersistence.Evidence, r.KnowledgePersistence.Improvements},
					Recommendations:        r.Recommendations})
			}
			var out struct {
				Items   []optGuidance `json:"items"`
				Changes []string      `json:"changes"` // some replies list a single item's changes here
			}
			if err := revise(ctx, call, "Answer optimization results (one item per question)", in, &out); err != nil {
				fail(fmt.Sprintf("optimizations batch %d", bi+1), err)
				return
			}
			if len(out.Items) == 1 && len(out.Items[0].Changes) == 0 && len(out.Changes) > 0 {
				out.Items[0].Changes = out.Changes
			}
			for _, it := range out.Items {
				o, ok := byID[it.ID]
				if !ok {
					continue
				}
				delete(byID, it.ID)
				r := o.Result
				var computed []string
				for _, d := range []struct {
					label         string
					before, after dimGuidance
				}{
					{"Content authority", dimGuidance{r.ContentAuthority.Evidence, r.ContentAuthority.Improvements}, it.ContentAuthority},
					{"Structure", dimGuidance{r.StructuralOptimization.Evidence, r.StructuralOptimization.Improvements}, it.StructuralOptimization},
					{"Source authority", dimGuidance{r.SourceAuthority.Evidence, r.SourceAuthority.Improvements}, it.SourceAuthority},
					{"Knowledge persistence", dimGuidance{r.KnowledgePersistence.Evidence, r.KnowledgePersistence.Improvements}, it.KnowledgePersistence},
				} {
					if d.after.Evidence != nil {
						computed = append(computed, diffNotes(d.label+" finding", d.before.Evidence, d.after.Evidence)...)
					}
					if d.after.Improvements != nil {
						computed = append(computed, diffNotes(d.label+" improvement", d.before.Improvements, d.after.Improvements)...)
					}
				}
				if it.Recommendations != nil {
					act := func(x Recommendation) string { return x.Action }
					computed = append(computed, diffNotes("Recommendation", recActions(r.Recommendations, act), recActions(it.Recommendations, act))...)
				}
				it.Changes = orNotes(it.Changes, computed)
				set := bson.D{
					{Key: "result.contentAuthority.evidence", Value: keepList(r.ContentAuthority.Evidence, it.ContentAuthority.Evidence, it.Changes)},
					{Key: "result.contentAuthority.improvements", Value: keepList(r.ContentAuthority.Improvements, it.ContentAuthority.Improvements, it.Changes)},
					{Key: "result.structuralOptimization.evidence", Value: keepList(r.StructuralOptimization.Evidence, it.StructuralOptimization.Evidence, it.Changes)},
					{Key: "result.structuralOptimization.improvements", Value: keepList(r.StructuralOptimization.Improvements, it.StructuralOptimization.Improvements, it.Changes)},
					{Key: "result.sourceAuthority.evidence", Value: keepList(r.SourceAuthority.Evidence, it.SourceAuthority.Evidence, it.Changes)},
					{Key: "result.sourceAuthority.improvements", Value: keepList(r.SourceAuthority.Improvements, it.SourceAuthority.Improvements, it.Changes)},
					{Key: "result.knowledgePersistence.evidence", Value: keepList(r.KnowledgePersistence.Evidence, it.KnowledgePersistence.Evidence, it.Changes)},
					{Key: "result.knowledgePersistence.improvements", Value: keepList(r.KnowledgePersistence.Improvements, it.KnowledgePersistence.Improvements, it.Changes)},
					{Key: "result.recommendations", Value: keepList(r.Recommendations, it.Recommendations, it.Changes)},
					{Key: "guidance", Value: guidanceMark(it.Changes)},
				}
				if err := applyGuidance(ctx, mongoDB, "optimizations", o.ID, tenantID, domain, set); err != nil {
					fail("optimization "+truncateStr(o.Question, 60), err)
					continue
				}
				done(it.Changes)
			}
			for _, o := range byID {
				fail("optimization "+truncateStr(o.Question, 60), fmt.Errorf("missing from the reviser's reply"))
			}
			mu.Lock()
			optsDone++
			n := optsDone
			mu.Unlock()
			notify(fmt.Sprintf("Updated answer optimizations: %d of %d.", n, len(batches)))
		}(bi, batch)
	}

	// The search, YouTube and Reddit analyses, alongside.
	if sa := work.search; sa != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := sa.Result
			in := searchGuidance{ExecutiveSummary: r.ExecutiveSummary, ConfidenceNote: r.ConfidenceNote,
				AIOEvidence: r.AIOReadiness.Evidence, CrawlEvidence: r.CrawlAccess.Evidence, RobotsTxtPolicy: r.CrawlAccess.RobotsTxtPolicy,
				MomentumEvidence: r.BrandMomentum.Evidence, FreshnessEvidence: r.ContentFreshness.Evidence, Recommendations: r.Recommendations}
			var out searchGuidance
			if err := revise(ctx, call, "Search visibility analysis", in, &out); err != nil {
				fail("search analysis", err)
				return
			}
			{
				var computed []string
				computed = append(computed, textNote("executive summary", r.ExecutiveSummary, out.ExecutiveSummary)...)
				computed = append(computed, textNote("confidence note", r.ConfidenceNote, out.ConfidenceNote)...)
				computed = append(computed, textNote("robots.txt summary", r.CrawlAccess.RobotsTxtPolicy, out.RobotsTxtPolicy)...)
				for _, l := range []struct {
					label         string
					before, after []string
				}{
					{"AI Overview finding", r.AIOReadiness.Evidence, out.AIOEvidence}, {"Crawl finding", r.CrawlAccess.Evidence, out.CrawlEvidence},
					{"Brand momentum finding", r.BrandMomentum.Evidence, out.MomentumEvidence}, {"Freshness finding", r.ContentFreshness.Evidence, out.FreshnessEvidence},
				} {
					if l.after != nil {
						computed = append(computed, diffNotes(l.label, l.before, l.after)...)
					}
				}
				if out.Recommendations != nil {
					act := func(x SearchRecommendation) string { return x.Action }
					computed = append(computed, diffNotes("Recommendation", recActions(r.Recommendations, act), recActions(out.Recommendations, act))...)
				}
				out.Changes = orNotes(out.Changes, computed)
			}
			set := bson.D{
				{Key: "result.executiveSummary", Value: keepText(r.ExecutiveSummary, out.ExecutiveSummary)},
				{Key: "result.confidenceNote", Value: keepText(r.ConfidenceNote, out.ConfidenceNote)},
				{Key: "result.aioReadiness.evidence", Value: keepList(r.AIOReadiness.Evidence, out.AIOEvidence, out.Changes)},
				{Key: "result.crawlAccessibility.evidence", Value: keepList(r.CrawlAccess.Evidence, out.CrawlEvidence, out.Changes)},
				{Key: "result.crawlAccessibility.robotsTxtPolicy", Value: keepText(r.CrawlAccess.RobotsTxtPolicy, out.RobotsTxtPolicy)},
				{Key: "result.brandMomentum.evidence", Value: keepList(r.BrandMomentum.Evidence, out.MomentumEvidence, out.Changes)},
				{Key: "result.contentFreshness.evidence", Value: keepList(r.ContentFreshness.Evidence, out.FreshnessEvidence, out.Changes)},
				{Key: "result.recommendations", Value: keepList(r.Recommendations, out.Recommendations, out.Changes)},
				{Key: "guidance", Value: guidanceMark(out.Changes)},
			}
			if err := applyGuidance(ctx, mongoDB, "search_analyses", sa.ID, tenantID, domain, set); err != nil {
				fail("search analysis", err)
				return
			}
			done(out.Changes)
			notify("Updated the search visibility analysis.")
		}()
	}
	if va := work.video; va != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := va.Result
			in := narrativeGuidance[VideoRecommendation]{ExecutiveSummary: r.ExecutiveSummary, ConfidenceNote: r.ConfidenceNote, Recommendations: r.Recommendations}
			var out narrativeGuidance[VideoRecommendation]
			if err := revise(ctx, call, "YouTube authority analysis", in, &out); err != nil {
				fail("YouTube analysis", err)
				return
			}
			{
				computed := append(textNote("executive summary", r.ExecutiveSummary, out.ExecutiveSummary), textNote("confidence note", r.ConfidenceNote, out.ConfidenceNote)...)
				if out.Recommendations != nil {
					act := func(x VideoRecommendation) string { return x.Action }
					computed = append(computed, diffNotes("Recommendation", recActions(r.Recommendations, act), recActions(out.Recommendations, act))...)
				}
				out.Changes = orNotes(out.Changes, computed)
			}
			set := bson.D{
				{Key: "result.executiveSummary", Value: keepText(r.ExecutiveSummary, out.ExecutiveSummary)},
				{Key: "result.confidenceNote", Value: keepText(r.ConfidenceNote, out.ConfidenceNote)},
				{Key: "result.recommendations", Value: keepList(r.Recommendations, out.Recommendations, out.Changes)},
				{Key: "guidance", Value: guidanceMark(out.Changes)},
			}
			if err := applyGuidance(ctx, mongoDB, "video_analyses", va.ID, tenantID, domain, set); err != nil {
				fail("YouTube analysis", err)
				return
			}
			done(out.Changes)
			notify("Updated the YouTube analysis.")
		}()
	}
	if ra := work.reddit; ra != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := ra.Result
			in := narrativeGuidance[RedditRecommendation]{ExecutiveSummary: r.ExecutiveSummary, ConfidenceNote: r.ConfidenceNote, Recommendations: r.Recommendations}
			var out narrativeGuidance[RedditRecommendation]
			if err := revise(ctx, call, "Reddit authority analysis", in, &out); err != nil {
				fail("Reddit analysis", err)
				return
			}
			{
				computed := append(textNote("executive summary", r.ExecutiveSummary, out.ExecutiveSummary), textNote("confidence note", r.ConfidenceNote, out.ConfidenceNote)...)
				if out.Recommendations != nil {
					act := func(x RedditRecommendation) string { return x.Action }
					computed = append(computed, diffNotes("Recommendation", recActions(r.Recommendations, act), recActions(out.Recommendations, act))...)
				}
				out.Changes = orNotes(out.Changes, computed)
			}
			set := bson.D{
				{Key: "result.executiveSummary", Value: keepText(r.ExecutiveSummary, out.ExecutiveSummary)},
				{Key: "result.confidenceNote", Value: keepText(r.ConfidenceNote, out.ConfidenceNote)},
				{Key: "result.recommendations", Value: keepList(r.Recommendations, out.Recommendations, out.Changes)},
				{Key: "guidance", Value: guidanceMark(out.Changes)},
			}
			if err := applyGuidance(ctx, mongoDB, "reddit_analyses", ra.ID, tenantID, domain, set); err != nil {
				fail("Reddit analysis", err)
				return
			}
			done(out.Changes)
			notify("Updated the Reddit analysis.")
		}()
	}
	wg.Wait()

	// Open to-dos, after the results they came from, 20 per call, in parallel.
	var todoWG sync.WaitGroup
	for i := 0; i < len(work.todos); i += 20 {
		todoWG.Add(1)
		go func(i int, batch []TodoItem) {
			defer todoWG.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			byID := map[string]TodoItem{}
			var in struct {
				Items []todoGuidance `json:"items"`
			}
			for _, t := range batch {
				byID[t.ID.Hex()] = t
				in.Items = append(in.Items, todoGuidance{ID: t.ID.Hex(), Action: t.Action, Summary: t.Summary, ExpectedImpact: t.ExpectedImpact})
			}
			var out struct {
				Items []todoGuidance `json:"items"`
			}
			if err := revise(ctx, call, `Open to-do items. For each, set "decision" to "keep" (unchanged), "rewrite" (with the rewritten action, summary and expected_impact) or "drop" (it only pursues unsupported advice), and give a short "reason" for a rewrite or drop. "changes" may stay empty here`, in, &out); err != nil {
				fail(fmt.Sprintf("to-dos %d-%d", i+1, i+len(batch)), err)
				return
			}
			for _, it := range out.Items {
				t, ok := byID[it.ID]
				if !ok {
					continue
				}
				delete(byID, it.ID)
				var set bson.D
				var changes []string
				switch it.Decision {
				case "drop":
					changes = []string{"Archived: " + keepText("advice the 2026 research doesn't support", it.Reason)}
					set = bson.D{{Key: "status", Value: "archived"}, {Key: "archivedAt", Value: time.Now()}}
					mu.Lock()
					res.Archived++
					mu.Unlock()
				case "rewrite":
					changes = []string{"Rewritten: " + keepText("updated to current advice", it.Reason)}
					set = bson.D{
						{Key: "action", Value: keepText(t.Action, it.Action)},
						{Key: "summary", Value: keepText(t.Summary, it.Summary)},
						{Key: "expectedImpact", Value: keepText(t.ExpectedImpact, it.ExpectedImpact)},
					}
				}
				set = append(set, bson.E{Key: "guidance", Value: guidanceMark(changes)})
				if err := applyGuidance(ctx, mongoDB, "todos", t.ID, tenantID, domain, set); err != nil {
					fail("to-do "+truncateStr(t.Action, 60), err)
					continue
				}
				done(changes)
			}
			for _, t := range byID {
				fail("to-do "+truncateStr(t.Action, 60), fmt.Errorf("missing from the reviser's reply"))
			}
			notify(fmt.Sprintf("Updated to-dos %d–%d of %d.", i+1, i+len(batch), len(work.todos)))
		}(i, work.todos[i:min(i+20, len(work.todos))])
	}
	todoWG.Wait()
	return res
}

// applyGuidance backs a document up and then sets the revised fields.
func applyGuidance(ctx context.Context, mongoDB *MongoDB, collection string, id primitive.ObjectID, tenantID, domain string, set bson.D) error {
	if err := backupDoc(ctx, mongoDB, collection, id, tenantID, domain); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	_, err := mongoDB.Database.Collection(collection).UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: set}})
	return err
}

// handleRefreshGuidance: POST /api/domains/{domain}/guidance (SSE) updates the
// advice in the domain's saved results to the current rubric.
func handleRefreshGuidance(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
		defer cancel()
		r = r.WithContext(ctx)
		domain := normalizeDomain(r.PathValue("domain"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		defer startSSEHeartbeat(w, flusher)()
		var sseMu sync.Mutex
		notify := func(msg string) {
			sseMu.Lock()
			defer sseMu.Unlock()
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		}

		provider, apiKey, model, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		work, err := loadGuidanceWork(ctx, mongoDB, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}))
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to load the domain's results"})
			return
		}
		if work.status.Pending == 0 {
			sendSSE(w, flusher, "done", map[string]any{"result": guidanceResult{}, "status": work.status})
			return
		}
		notify(fmt.Sprintf("Updating the advice in %d optimizations, %d analyses and %d to-dos to the 2026 research (%s)...",
			work.status.Optimizations, work.status.Search+work.status.Video+work.status.Reddit, work.status.Todos, provider.Name()))

		call := func(ctx context.Context, prompt string) (string, error) {
			return callWithRetry(ctx, provider, apiKey, model, prompt, 12000)
		}
		res := refreshGuidance(ctx, mongoDB, call, work, saas.TenantIDFromContext(r.Context()), domain, notify)
		if len(res.Failed) > 0 && res.Updated == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "The advice couldn't be updated: " + strings.Join(res.Failed, "; ")})
			return
		}
		after, _ := loadGuidanceWork(ctx, mongoDB, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}))
		sendSSE(w, flusher, "done", map[string]any{"result": res, "status": after.status})
	}
}
