package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fakeReviser rewrites any item mentioning FAQ blocks and drops to-dos about
// them, the way the real reviser is asked to.
func fakeReviser(calls *int32) reviseFunc {
	return func(ctx context.Context, prompt string) (string, error) {
		atomic.AddInt32(calls, 1)
		i := strings.LastIndex(prompt, ":\n{")
		var payload map[string]any
		if err := json.Unmarshal([]byte(prompt[i+2:]), &payload); err != nil {
			return "", err
		}
		fix := func(rec map[string]any) bool {
			if a, _ := rec["action"].(string); strings.Contains(a, "FAQ") {
				rec["action"] = "Match the page title to the question buyers ask"
				return true
			}
			return false
		}
		if items, ok := payload["items"].([]any); ok {
			for _, it := range items {
				m := it.(map[string]any)
				if _, isTodo := m["summary"]; isTodo {
					m["decision"] = "keep"
					if a, _ := m["action"].(string); strings.Contains(a, "FAQ") {
						m["decision"], m["reason"] = "drop", "FAQ blocks aren't a citation lever"
					}
					continue
				}
				var changes []string
				for _, r := range m["recommendations"].([]any) {
					if fix(r.(map[string]any)) {
						changes = append(changes, "Replaced the FAQ recommendation")
					}
				}
				m["changes"] = changes
			}
		} else {
			var changes []string
			for _, r := range payload["recommendations"].([]any) {
				if fix(r.(map[string]any)) {
					changes = append(changes, "Replaced the FAQ recommendation")
				}
			}
			payload["changes"] = changes
		}
		b, _ := json.Marshal(payload)
		return string(b), nil
	}
}

func TestRefreshGuidance_UpdatesOldResultsOnly(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	const tenant, domain = "test-tenant", "guide.example"
	faq := Recommendation{Priority: "high", Action: "Add an FAQ block with schema to win AI citations", Dimension: "structural_optimization"}
	keep := Recommendation{Priority: "medium", Action: "Get reviewed by independent comparison sites", Dimension: "source_authority"}

	var oldIDs []primitive.ObjectID
	for i := 0; i < 5; i++ {
		res, err := db.Optimizations().InsertOne(ctx, Optimization{TenantID: tenant, Domain: domain, Question: "Q?", CreatedAt: time.Now(),
			Result: OptimizationResult{OverallScore: 60, ContentAuthority: DimensionScore{Score: 60, Evidence: []string{"e"}, Improvements: []string{"i"}},
				Recommendations: []Recommendation{faq, keep}}})
		if err != nil {
			t.Fatal(err)
		}
		oldIDs = append(oldIDs, res.InsertedID.(primitive.ObjectID))
	}
	cur, _ := db.Optimizations().InsertOne(ctx, Optimization{TenantID: tenant, Domain: domain, Question: "New?",
		Result: OptimizationResult{RubricVersion: scoringRubricVersion, Recommendations: []Recommendation{faq}}})
	if _, err := db.SearchAnalyses().InsertOne(ctx, SearchAnalysis{TenantID: tenant, Domain: domain, GeneratedAt: time.Now(),
		Result: &SearchVisibilityResult{RubricVersion: 2, ExecutiveSummary: "Summary", AIOReadiness: AIOReadinessPillar{Score: 50, Evidence: []string{"aio"}},
			Recommendations: []SearchRecommendation{{Action: "Add FAQ schema", Priority: "high"}}}}); err != nil {
		t.Fatal(err)
	}
	todo := func(action, status string) {
		if _, err := db.Todos().InsertOne(ctx, TodoItem{TenantID: tenant, Domain: domain, Action: action, Status: status, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	todo("Add an FAQ block", "todo")
	todo("Refresh the pricing page", "todo")
	todo("Add an FAQ block to the old page", "completed")

	filter := bson.D{{Key: "tenantId", Value: tenant}, {Key: "domain", Value: domain}}
	work, err := loadGuidanceWork(ctx, db, filter)
	if err != nil {
		t.Fatal(err)
	}
	if s := work.status; s.Optimizations != 5 || s.Search != 1 || s.Todos != 2 || s.Pending != 8 {
		t.Fatalf("status = %+v", s)
	}

	var calls int32
	res := refreshGuidance(ctx, db, fakeReviser(&calls), work, tenant, domain, func(string) {})
	if len(res.Failed) > 0 || res.Updated != 8 || res.Archived != 1 {
		t.Fatalf("result = %+v", res)
	}
	if c := atomic.LoadInt32(&calls); c != 5/guidanceRefreshBatch+1+1 { // optimization batches, search, to-dos
		t.Errorf("reviser calls = %d", c)
	}

	for _, id := range oldIDs {
		var o Optimization
		if err := db.Optimizations().FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&o); err != nil {
			t.Fatal(err)
		}
		if o.Guidance == nil || o.Guidance.Rubric != scoringRubricVersion || len(o.Guidance.Changes) != 1 {
			t.Errorf("guidance = %+v", o.Guidance)
		}
		if o.Result.Recommendations[0].Action != "Match the page title to the question buyers ask" || o.Result.Recommendations[1] != keep {
			t.Errorf("recommendations = %+v", o.Result.Recommendations)
		}
		if o.Result.OverallScore != 60 || o.Result.ContentAuthority.Score != 60 || o.Result.ContentAuthority.Evidence[0] != "e" {
			t.Errorf("scores and findings must stay: %+v", o.Result)
		}
	}
	var current Optimization
	db.Optimizations().FindOne(ctx, bson.D{{Key: "_id", Value: cur.InsertedID}}).Decode(&current)
	if current.Guidance != nil || current.Result.Recommendations[0] != faq {
		t.Error("a result already on the current rubric must be left alone")
	}
	var sa SearchAnalysis
	db.SearchAnalyses().FindOne(ctx, filter).Decode(&sa)
	if sa.Guidance == nil || sa.Result.Recommendations[0].Action == "Add FAQ schema" || sa.Result.ExecutiveSummary != "Summary" || sa.Result.AIOReadiness.Evidence[0] != "aio" {
		t.Errorf("search = %+v / %+v", sa.Guidance, sa.Result)
	}
	var todos []TodoItem
	c, _ := db.Todos().Find(ctx, filter)
	c.All(ctx, &todos)
	for _, td := range todos {
		switch td.Action {
		case "Add an FAQ block":
			if td.Status != "archived" || td.Guidance == nil {
				t.Errorf("the FAQ to-do must be archived: %+v", td)
			}
		case "Refresh the pricing page":
			if td.Status != "todo" || td.Guidance == nil {
				t.Errorf("the kept to-do must stay open and be marked: %+v", td)
			}
		case "Add an FAQ block to the old page":
			if td.Status != "completed" || td.Guidance != nil {
				t.Errorf("a completed to-do must be left alone: %+v", td)
			}
		}
	}
	if n, _ := db.GuidanceBackups().CountDocuments(ctx, bson.D{{Key: "domain", Value: domain}}); n != 8 {
		t.Errorf("backups = %d, want one per updated document", n)
	}
	after, _ := loadGuidanceWork(ctx, db, filter)
	if after.status.Pending != 0 || after.status.Revised != 6 || after.status.RevisedAt == nil {
		t.Errorf("after = %+v", after.status)
	}
}

func TestKeepList(t *testing.T) {
	orig := []string{"a", "b"}
	if got := keepList(orig, nil, nil); len(got) != 2 {
		t.Error("a reply that lost the list must keep the original")
	}
	if got := keepList(orig, []string{}, []string{"Removed both"}); len(got) != 0 {
		t.Error("an explained removal must be applied")
	}
	if got := keepList(orig, []string{"c"}, nil); len(got) != 1 || got[0] != "c" {
		t.Error("a revised list must be applied")
	}
	if keepText("orig", "  ") != "orig" || keepText("orig", "new") != "new" {
		t.Error("keepText")
	}
}

func TestAdviceUpdatedInReport(t *testing.T) {
	d := fixtureReportData()
	if adviceUpdated(d) != "" {
		t.Error("no update yet")
	}
	when := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	d.Optimizations[0].Guidance = &GuidanceRevision{Rubric: scoringRubricVersion, RevisedAt: when}
	html, err := renderReportHTML(buildReportView(d, time.Now(), "pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "updated on November 15, 2026 to reflect the 2026 research") {
		t.Error("the report must say when its advice was updated")
	}
}

func TestDiffNotes(t *testing.T) {
	if n := diffNotes("Rec", []string{"a", "b"}, []string{"a", "b"}); len(n) != 0 {
		t.Errorf("no change: %v", n)
	}
	if n := diffNotes("Rec", []string{"a", "b"}, []string{"a", "c"}); len(n) != 1 || !strings.Contains(n[0], `rewrote "b" as "c"`) {
		t.Errorf("rewrite: %v", n)
	}
	if n := diffNotes("Rec", []string{"a", "b"}, []string{"a"}); len(n) != 1 || !strings.Contains(n[0], `removed "b"`) {
		t.Errorf("removal: %v", n)
	}
}

// A reviser that edits without listing its changes still leaves notes, and
// results revised with an older pass are revised again.
func TestRefreshGuidance_ComputedNotesAndOlderPass(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	const tenant, domain = "test-tenant", "pass.example"
	faq := Recommendation{Priority: "high", Action: "Add an FAQ block with schema to win AI citations"}
	old := &GuidanceRevision{Rubric: scoringRubricVersion, Pass: guidancePass - 1, RevisedAt: time.Now().Add(-time.Hour)}
	res, err := db.Optimizations().InsertOne(ctx, Optimization{TenantID: tenant, Domain: domain, Question: "Q?", Guidance: old,
		Result: OptimizationResult{Recommendations: []Recommendation{faq}}})
	if err != nil {
		t.Fatal(err)
	}
	filter := bson.D{{Key: "tenantId", Value: tenant}, {Key: "domain", Value: domain}}
	work, _ := loadGuidanceWork(ctx, db, filter)
	if work.status.Optimizations != 1 {
		t.Fatalf("a result revised with an older pass must be pending: %+v", work.status)
	}
	var calls int32
	silent := func(ctx context.Context, prompt string) (string, error) {
		out, err := fakeReviser(&calls)(ctx, prompt)
		var m map[string]any
		json.Unmarshal([]byte(out), &m)
		for _, it := range m["items"].([]any) {
			delete(it.(map[string]any), "changes") // edits, but doesn't say so
		}
		b, _ := json.Marshal(m)
		return string(b), err
	}
	if r := refreshGuidance(ctx, db, silent, work, tenant, domain, func(string) {}); r.Updated != 1 {
		t.Fatalf("result = %+v", r)
	}
	var o Optimization
	db.Optimizations().FindOne(ctx, bson.D{{Key: "_id", Value: res.InsertedID}}).Decode(&o)
	if o.Guidance == nil || o.Guidance.Pass != guidancePass || len(o.Guidance.Changes) != 1 || !strings.Contains(o.Guidance.Changes[0], "Recommendation: rewrote") {
		t.Errorf("guidance = %+v", o.Guidance)
	}
}
