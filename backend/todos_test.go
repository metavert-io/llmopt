package main

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── createTodosFromOptimization ─────────────────────────────────────────────

func TestCreateTodosFromOptimization_NoRecommendations(t *testing.T) {
	db := testMongoDB(t)
	analysisID := primitive.NewObjectID()
	optID := primitive.NewObjectID()

	result := OptimizationResult{
		Recommendations: []Recommendation{}, // empty
	}
	createTodosFromOptimization(db, optID, analysisID, "example.com", "question", "test-tenant", "", result)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "example.com"})
	if count != 0 {
		t.Errorf("expected 0 todos, got %d", count)
	}
}

func TestCreateTodosFromOptimization_WithRecommendations(t *testing.T) {
	db := testMongoDB(t)
	analysisID := primitive.NewObjectID()
	optID := primitive.NewObjectID()

	result := OptimizationResult{
		Recommendations: []Recommendation{
			{Priority: "high", Action: "Write expert content", ExpectedImpact: "Better ranking", Dimension: "content_authority"},
			{Priority: "medium", Action: "Build backlinks", ExpectedImpact: "More citations", Dimension: "source_authority"},
		},
	}
	createTodosFromOptimization(db, optID, analysisID, "example.com", "How to rank?", "test-tenant", "", result)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "example.com", "tenantId": "test-tenant"})
	if count != 2 {
		t.Errorf("expected 2 todos, got %d", count)
	}
}

func TestCreateTodosFromOptimization_DiscoveryBrandStatus(t *testing.T) {
	db := testMongoDB(t)
	analysisID := primitive.NewObjectID()
	optID := primitive.NewObjectID()

	result := OptimizationResult{
		Recommendations: []Recommendation{
			{Priority: "high", Action: "Claim brand", ExpectedImpact: "Visibility", Dimension: "brand_presence"},
		},
	}
	createTodosFromOptimization(db, optID, analysisID, "brand-example.com", "Brand?", "t-brand", "discovery", result)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var todo TodoItem
	db.Todos().FindOne(ctx, bson.M{"domain": "brand-example.com"}).Decode(&todo)
	if len(todo.Tags) == 0 || todo.Tags[0] != "discovery" {
		t.Errorf("expected 'discovery' tag, got %v", todo.Tags)
	}
}

// ── createTodosFromVideoAnalysis ────────────────────────────────────────────

func TestCreateTodosFromVideoAnalysis_NoRecommendations(t *testing.T) {
	db := testMongoDB(t)
	videoID := primitive.NewObjectID()

	createTodosFromVideoAnalysis(db, videoID, "video-example.com", "test-tenant", []VideoRecommendation{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "video-example.com"})
	if count != 0 {
		t.Errorf("expected 0 todos, got %d", count)
	}
}

func TestCreateTodosFromVideoAnalysis_WithRecommendations(t *testing.T) {
	db := testMongoDB(t)
	videoID := primitive.NewObjectID()

	recs := []VideoRecommendation{
		{Action: "Add transcript", ExpectedImpact: "Better indexing", Dimension: "transcript_authority", Priority: "high"},
		{Action: "Cover more topics", ExpectedImpact: "Topical coverage", Dimension: "topical_dominance", Priority: "medium"},
	}
	createTodosFromVideoAnalysis(db, videoID, "video-example.com", "test-tenant", recs)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "video-example.com", "tenantId": "test-tenant"})
	if count != 2 {
		t.Errorf("expected 2 todos, got %d", count)
	}
}

func TestCreateTodosFromVideoAnalysis_DimensionLabel(t *testing.T) {
	db := testMongoDB(t)
	videoID := primitive.NewObjectID()

	recs := []VideoRecommendation{
		{Action: "Increase citations", Dimension: "citation_network", Priority: "high"},
	}
	createTodosFromVideoAnalysis(db, videoID, "dim-label.com", "test-tenant", recs)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var todo TodoItem
	db.Todos().FindOne(ctx, bson.M{"domain": "dim-label.com"}).Decode(&todo)
	if todo.Question != "Video: Citation Network" {
		t.Errorf("expected 'Video: Citation Network', got %q", todo.Question)
	}
}

func TestCreateTodosFromVideoAnalysis_UnknownDimension(t *testing.T) {
	db := testMongoDB(t)
	videoID := primitive.NewObjectID()

	recs := []VideoRecommendation{
		{Action: "Custom action", Dimension: "custom_dimension", Priority: "high"},
	}
	createTodosFromVideoAnalysis(db, videoID, "unknown-dim.com", "test-tenant", recs)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var todo TodoItem
	db.Todos().FindOne(ctx, bson.M{"domain": "unknown-dim.com"}).Decode(&todo)
	if todo.Question != "Video: LLM Authority" {
		t.Errorf("expected 'Video: LLM Authority' for unknown dimension, got %q", todo.Question)
	}
}

// ── createTodosFromRedditAnalysis ────────────────────────────────────────────

func TestCreateTodosFromRedditAnalysis_FiltersByPriority(t *testing.T) {
	db := testMongoDB(t)

	recs := []RedditRecommendation{
		{Action: "Engage in subreddit", Priority: "high", Dimension: "presence"},
		{Action: "Monitor mentions", Priority: "medium", Dimension: "sentiment"},
		{Action: "Low priority item", Priority: "low", Dimension: "training_signal"}, // should be filtered
	}
	createTodosFromRedditAnalysis(db, "reddit-example.com", "test-tenant", recs)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "reddit-example.com"})
	if count != 2 {
		t.Errorf("expected 2 todos (high+medium), got %d", count)
	}
}

func TestCreateTodosFromRedditAnalysis_Empty(t *testing.T) {
	db := testMongoDB(t)
	createTodosFromRedditAnalysis(db, "reddit-empty.com", "test-tenant", []RedditRecommendation{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "reddit-empty.com"})
	if count != 0 {
		t.Errorf("expected 0 todos, got %d", count)
	}
}

// ── createTodosFromSearchAnalysis ─────────────────────────────────────────────

func TestCreateTodosFromSearchAnalysis_FiltersByPriority(t *testing.T) {
	db := testMongoDB(t)

	recs := []SearchRecommendation{
		{Action: "Optimize for search", Priority: "high", Dimension: "aio_readiness"},
		{Action: "Improve crawlability", Priority: "medium", Dimension: "crawl_accessibility"},
		{Action: "Low priority", Priority: "low", Dimension: "content_freshness"}, // should be filtered
	}
	createTodosFromSearchAnalysis(db, "search-example.com", "test-tenant", recs)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "search-example.com"})
	if count != 2 {
		t.Errorf("expected 2 todos (high+medium), got %d", count)
	}
}

func TestCreateTodosFromSearchAnalysis_DiscoveryTag(t *testing.T) {
	db := testMongoDB(t)

	recs := []SearchRecommendation{
		{Action: "Expand categories", Priority: "high", Dimension: "category_discovery"},
	}
	createTodosFromSearchAnalysis(db, "search-discovery.com", "test-tenant", recs)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var todo TodoItem
	db.Todos().FindOne(ctx, bson.M{"domain": "search-discovery.com"}).Decode(&todo)
	if len(todo.Tags) == 0 || todo.Tags[0] != "discovery" {
		t.Errorf("expected 'discovery' tag for category_discovery, got %v", todo.Tags)
	}
}

// ── deduplicateTodos ────────────────────────────────────────────────────────

func TestDeduplicateTodos_NoDuplicates(t *testing.T) {
	db := testMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert two distinct todos
	db.Todos().InsertMany(ctx, []any{
		TodoItem{TenantID: "t", Domain: "dedup-none.com", Action: "Write expert content about AI", Dimension: "content_authority", Status: "todo", Priority: "high", CreatedAt: time.Now()},
		TodoItem{TenantID: "t", Domain: "dedup-none.com", Action: "Build backlinks from tech sites", Dimension: "source_authority", Status: "todo", Priority: "medium", CreatedAt: time.Now()},
	})

	deduplicateTodos(db, "dedup-none.com", "t")

	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "dedup-none.com", "status": "todo"})
	if count != 2 {
		t.Errorf("expected 2 todos (no dups), got %d", count)
	}
}

func TestDeduplicateTodos_WithDuplicates(t *testing.T) {
	db := testMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert two very similar todos (should be deduplicated)
	db.Todos().InsertMany(ctx, []any{
		TodoItem{TenantID: "t2", Domain: "dedup-yes.com", Action: "Write expert content about AI models", Dimension: "content_authority", Status: "todo", Priority: "low", CreatedAt: time.Now()},
		TodoItem{TenantID: "t2", Domain: "dedup-yes.com", Action: "Write expert content about AI models deep", Dimension: "content_authority", Status: "todo", Priority: "high", CreatedAt: time.Now().Add(time.Second)},
	})

	deduplicateTodos(db, "dedup-yes.com", "t2")

	// Wait for goroutine to complete (deduplicateTodos may run synchronously in test)
	time.Sleep(200 * time.Millisecond)

	todoCount, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "dedup-yes.com", "status": "todo"})
	archivedCount, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "dedup-yes.com", "status": "archived"})

	// At least one should be archived
	if archivedCount == 0 {
		t.Errorf("expected at least one archived todo, got todo=%d archived=%d", todoCount, archivedCount)
	}
}

func TestDeduplicateTodos_LessThanTwo(t *testing.T) {
	db := testMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert only one todo
	db.Todos().InsertOne(ctx, TodoItem{
		TenantID: "t3", Domain: "dedup-one.com",
		Action: "Single todo", Dimension: "content_authority",
		Status: "todo", Priority: "high", CreatedAt: time.Now(),
	})

	deduplicateTodos(db, "dedup-one.com", "t3")

	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "dedup-one.com", "status": "todo"})
	if count != 1 {
		t.Errorf("expected 1 todo (can't dedup with only 1), got %d", count)
	}
}

func TestDeduplicateTodos_DifferentDimensions(t *testing.T) {
	db := testMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Same text but different dimensions — should NOT be deduplicated
	db.Todos().InsertMany(ctx, []any{
		TodoItem{TenantID: "t4", Domain: "dedup-dim.com", Action: "improve content quality significantly for users", Dimension: "content_authority", Status: "todo", Priority: "high", CreatedAt: time.Now()},
		TodoItem{TenantID: "t4", Domain: "dedup-dim.com", Action: "improve content quality significantly for users", Dimension: "source_authority", Status: "todo", Priority: "medium", CreatedAt: time.Now().Add(time.Second)},
	})

	deduplicateTodos(db, "dedup-dim.com", "t4")

	count, _ := db.Todos().CountDocuments(ctx, bson.M{"domain": "dedup-dim.com", "status": "todo"})
	if count != 2 {
		t.Errorf("expected 2 todos (different dimensions), got %d", count)
	}
}

// ── isSummaryStale ───────────────────────────────────────────────────────────

func TestIsSummaryStale_NotStale(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")
	bgCtx := context.Background()

	// Summary generated after all data - should not be stale
	generatedAt := time.Now()
	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		TenantID:      "test-tenant",
		Domain:        "stale-test.com",
		GeneratedAt:   generatedAt,
	}

	stale, _ := isSummaryStale(bgCtx, db, ctx, "stale-test.com", summary)
	if stale {
		t.Error("expected not stale (no data newer than summary)")
	}
}

func TestIsSummaryStale_NewOptimization(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")
	bgCtx := context.Background()

	// Old summary
	oldTime := time.Now().Add(-time.Hour)
	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		TenantID:      "test-tenant",
		Domain:        "stale-opt.com",
		GeneratedAt:   oldTime,
	}

	// Insert an optimization newer than the summary
	db.Optimizations().InsertOne(bgCtx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "stale-opt.com",
		"createdAt": time.Now(), // newer than summary
	})

	stale, count := isSummaryStale(bgCtx, db, ctx, "stale-opt.com", summary)
	if !stale {
		t.Error("expected stale because newer optimization exists")
	}
	if count <= 0 {
		t.Errorf("expected count > 0, got %d", count)
	}
}

func TestIsSummaryStale_NewAnalysis(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")
	bgCtx := context.Background()

	oldTime := time.Now().Add(-time.Hour)
	summary := DomainSummary{
		RubricVersion: scoringRubricVersion,
		TenantID:      "test-tenant",
		Domain:        "stale-an.com",
		GeneratedAt:   oldTime,
	}

	// Insert an analysis newer than the summary
	db.Analyses().InsertOne(bgCtx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "stale-an.com",
		"createdAt": time.Now(),
	})

	stale, _ := isSummaryStale(bgCtx, db, ctx, "stale-an.com", summary)
	if !stale {
		t.Error("expected stale because newer analysis exists")
	}
}
