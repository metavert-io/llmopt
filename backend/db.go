package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDB(uri, database string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(50).
		SetMinPoolSize(1).
		SetMaxConnIdleTime(5 * time.Minute)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := &MongoDB{
		Client:   client,
		Database: client.Database(database),
	}

	db.ensureIndexes()
	return db, nil
}

func (m *MongoDB) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
	}

	_, err := m.Analyses().Indexes().CreateMany(ctx, indexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on analyses: %v", err)
	}

	optIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "analysisId", Value: 1}, {Key: "questionIndex", Value: 1}}},
		{Keys: bson.D{{Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
	}
	_, err = m.Optimizations().Indexes().CreateMany(ctx, optIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on optimizations: %v", err)
	}

	todoIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "optimizationId", Value: 1}}},
		{Keys: bson.D{{Key: "domain", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "videoAnalysisId", Value: 1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
	}
	_, err = m.Todos().Indexes().CreateMany(ctx, todoIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on todos: %v", err)
	}

	brandIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "updatedAt", Value: -1}}},
	}
	_, err = m.BrandProfiles().Indexes().CreateMany(ctx, brandIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on brand_profiles: %v", err)
	}

	healthIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "checkedAt", Value: -1}}},
		{Keys: bson.D{{Key: "checkedAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(30 * 24 * 3600)},
	}
	_, err = m.HealthChecks().Indexes().CreateMany(ctx, healthIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on health_checks: %v", err)
	}

	summaryIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
	}
	_, err = m.DomainSummaries().Indexes().CreateMany(ctx, summaryIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on domain_summaries: %v", err)
	}

	videoIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "generatedAt", Value: -1}}},
	}
	_, err = m.VideoAnalyses().Indexes().CreateMany(ctx, videoIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on video_analyses: %v", err)
	}

	shareIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "shareId", Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"shareId": bson.M{"$gt": ""}})},
		{Keys: bson.D{{Key: "visibility", Value: 1}}},
	}
	_, err = m.DomainShares().Indexes().CreateMany(ctx, shareIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on domain_shares: %v", err)
	}

	cacheIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "cacheKey", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0)},
	}
	_, err = m.YouTubeCache().Indexes().CreateMany(ctx, cacheIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on youtube_cache: %v", err)
	}

	redditIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "generatedAt", Value: -1}}},
	}
	_, err = m.RedditAnalyses().Indexes().CreateMany(ctx, redditIndexes)
	if err != nil {
		log.Printf("Warning: failed to create indexes on reddit_analyses: %v", err)
	}

	redditCacheIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "cacheKey", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0)},
	}
	_, err = m.RedditCache().Indexes().CreateMany(ctx, redditCacheIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on reddit_cache: %v", err)
	}

	reportPDFIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
	}
	_, err = m.ReportPDFs().Indexes().CreateMany(ctx, reportPDFIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on report_pdfs: %v", err)
	}

	screenshotIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "capturedAt", Value: -1}}},
	}
	_, err = m.BrandScreenshots().Indexes().CreateMany(ctx, screenshotIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on brand_screenshots: %v", err)
	}

	searchIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "generatedAt", Value: -1}}},
	}
	_, err = m.SearchAnalyses().Indexes().CreateMany(ctx, searchIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on search_analyses: %v", err)
	}

	// LLM Tests
	llmTestIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "domain", Value: 1}, {Key: "createdAt", Value: -1}}},
	}
	_, err = m.LLMTests().Indexes().CreateMany(ctx, llmTestIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on llm_tests: %v", err)
	}

	apiKeyIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "provider", Value: 1}},
			Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "tenantId", Value: 1}}},
	}
	_, err = m.TenantAPIKeys().Indexes().CreateMany(ctx, apiKeyIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on tenant_api_keys: %v", err)
	}

	settingsIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}},
			Options: options.Index().SetUnique(true)},
	}
	_, err = m.TenantSettings().Indexes().CreateMany(ctx, settingsIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on tenant_settings: %v", err)
	}

	failedIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}, {Key: "feedType", Value: 1}, {Key: "failedAt", Value: -1}}},
		{Keys: bson.D{{Key: "failedAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(7 * 24 * 3600)},
	}
	_, err = m.FailedAnalyses().Indexes().CreateMany(ctx, failedIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on failed_analyses: %v", err)
	}

	accessKeyIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "keyHash", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "isActive", Value: 1}}},
		{Keys: bson.D{{Key: "tenantId", Value: 1}}},
	}
	_, err = m.UserAccessKeys().Indexes().CreateMany(ctx, accessKeyIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on user_access_keys: %v", err)
	}

	querySetIdx := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "domain", Value: 1}},
			Options: options.Index().SetUnique(true)},
	}
	_, err = m.TestQuerySets().Indexes().CreateMany(ctx, querySetIdx)
	if err != nil {
		log.Printf("Warning: failed to create indexes on test_query_sets: %v", err)
	}
}

// migrateIndexes drops old indexes that conflict with current index definitions.
// Safe to run multiple times — dropping a non-existent index just returns an error we ignore.
func (m *MongoDB) migrateIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Collections that had {domain: 1} unique indexes and now use {tenantId: 1, domain: 1} unique
	collections := []string{
		"brand_profiles", "domain_summaries", "video_analyses",
		"reddit_analyses", "report_pdfs", "search_analyses",
	}

	for _, name := range collections {
		coll := m.Database.Collection(name)
		_, err := coll.Indexes().DropOne(ctx, "domain_1")
		if err == nil {
			log.Printf("migrateIndexes: dropped old domain_1 unique index on %s", name)
		}
		// Also drop the old non-unique {tenantId: 1, domain: 1} to avoid conflict with new unique version
		_, _ = coll.Indexes().DropOne(ctx, "tenantId_1_domain_1")
	}

	// brand_screenshots: changed from {tenantId, domain} compound to {domain} unique
	ssColl := m.Database.Collection("brand_screenshots")
	if _, err := ssColl.Indexes().DropOne(ctx, "tenantId_1_domain_1"); err == nil {
		log.Printf("migrateIndexes: dropped old tenantId_1_domain_1 index on brand_screenshots")
	}
}

func (m *MongoDB) Analyses() *mongo.Collection {
	return m.Database.Collection("analyses")
}

func (m *MongoDB) Optimizations() *mongo.Collection {
	return m.Database.Collection("optimizations")
}

func (m *MongoDB) Todos() *mongo.Collection {
	return m.Database.Collection("todos")
}

func (m *MongoDB) BrandProfiles() *mongo.Collection {
	return m.Database.Collection("brand_profiles")
}

func (m *MongoDB) HealthChecks() *mongo.Collection {
	return m.Database.Collection("health_checks")
}

func (m *MongoDB) DomainSummaries() *mongo.Collection {
	return m.Database.Collection("domain_summaries")
}

func (m *MongoDB) VideoAnalyses() *mongo.Collection {
	return m.Database.Collection("video_analyses")
}

func (m *MongoDB) YouTubeCache() *mongo.Collection {
	return m.Database.Collection("youtube_cache")
}

func (m *MongoDB) DomainShares() *mongo.Collection {
	return m.Database.Collection("domain_shares")
}

func (m *MongoDB) RedditAnalyses() *mongo.Collection {
	return m.Database.Collection("reddit_analyses")
}

func (m *MongoDB) RedditCache() *mongo.Collection {
	return m.Database.Collection("reddit_cache")
}

// TODO: migrate to object storage (S3/R2) when storage becomes a concern
func (m *MongoDB) ReportPDFs() *mongo.Collection {
	return m.Database.Collection("report_pdfs")
}

// TODO: migrate to object storage (S3/R2) when storage becomes a concern
func (m *MongoDB) BrandScreenshots() *mongo.Collection {
	return m.Database.Collection("brand_screenshots")
}

func (m *MongoDB) TenantAPIKeys() *mongo.Collection {
	return m.Database.Collection("tenant_api_keys")
}

func (m *MongoDB) SearchAnalyses() *mongo.Collection {
	return m.Database.Collection("search_analyses")
}

func (m *MongoDB) LLMTests() *mongo.Collection {
	return m.Database.Collection("llm_tests")
}

func (m *MongoDB) TestQuerySets() *mongo.Collection {
	return m.Database.Collection("test_query_sets")
}

func (m *MongoDB) TenantSettings() *mongo.Collection {
	return m.Database.Collection("tenant_settings")
}

func (m *MongoDB) UserAccessKeys() *mongo.Collection {
	return m.Database.Collection("user_access_keys")
}

func (m *MongoDB) FailedAnalyses() *mongo.Collection {
	return m.Database.Collection("failed_analyses")
}

// allTenantCollections returns all collections that store tenant-scoped domain data.
func (m *MongoDB) allTenantCollections() []*mongo.Collection {
	return []*mongo.Collection{
		m.Analyses(), m.Optimizations(), m.BrandProfiles(),
		m.VideoAnalyses(), m.RedditAnalyses(), m.SearchAnalyses(),
		m.DomainSummaries(), m.LLMTests(),
	}
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}

// migrateDomains normalizes domain fields (strips protocol, lowercases) across all collections.
// Safe to run multiple times — only updates documents that have a protocol prefix.
func (m *MongoDB) migrateDomains() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	collections := []string{"analyses", "optimizations", "todos", "brand_profiles", "video_analyses", "domain_summaries", "reddit_analyses", "search_analyses"}

	for _, coll := range collections {
		c := m.Database.Collection(coll)
		// Find documents with http:// or https:// in the domain field
		filter := bson.D{{Key: "domain", Value: bson.D{{Key: "$regex", Value: "^https?://"}}}}
		cursor, err := c.Find(ctx, filter)
		if err != nil {
			log.Printf("migrateDomains: error querying %s: %v", coll, err)
			continue
		}
		var updated int
		for cursor.Next(ctx) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			oldDomain, ok := doc["domain"].(string)
			if !ok {
				continue
			}
			newDomain := normalizeDomain(oldDomain)
			if newDomain == oldDomain {
				continue
			}
			_, err := c.UpdateByID(ctx, doc["_id"], bson.D{{Key: "$set", Value: bson.D{{Key: "domain", Value: newDomain}}}})
			if err != nil {
				log.Printf("migrateDomains: error updating %s doc %v: %v", coll, doc["_id"], err)
			} else {
				updated++
			}
		}
		cursor.Close(ctx)
		if updated > 0 {
			log.Printf("migrateDomains: normalized %d domains in %s", updated, coll)
		}
	}
}
