package main

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// A new LLM Test, an advice update or an archived to-do must regenerate a
// cached report.
func TestFingerprintTracksTestsAdviceAndTodos(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	tenantCtx := testAuthContext("test-tenant", "test-user")
	const domain = "cache.example"
	base := computeFingerprint(ctx, db, domain, tenantCtx)

	db.LLMTests().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "domain": domain, "generatedAt": time.Now()})
	afterTest := computeFingerprint(ctx, db, domain, tenantCtx)
	if fingerprintEqual(base, afterTest) || afterTest.LLMTestGeneratedAt == nil {
		t.Error("a new LLM Test must change the fingerprint")
	}

	res, _ := db.Todos().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "domain": domain, "status": "todo", "createdAt": time.Now().Add(-time.Hour)})
	withTodo := computeFingerprint(ctx, db, domain, tenantCtx)
	db.Todos().UpdateOne(ctx, bson.M{"_id": res.InsertedID}, bson.M{"$set": bson.M{"status": "archived"}})
	archived := computeFingerprint(ctx, db, domain, tenantCtx)
	if fingerprintEqual(withTodo, archived) {
		t.Error("archiving an open to-do must change the fingerprint")
	}

	db.SearchAnalyses().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "domain": domain, "generatedAt": time.Now().Add(-time.Hour)})
	before := computeFingerprint(ctx, db, domain, tenantCtx)
	db.SearchAnalyses().UpdateOne(ctx, bson.M{"domain": domain}, bson.M{"$set": bson.M{"guidance": GuidanceRevision{Rubric: scoringRubricVersion, RevisedAt: time.Now()}}})
	after := computeFingerprint(ctx, db, domain, tenantCtx)
	if fingerprintEqual(before, after) || after.GuidanceRevisedAt == nil {
		t.Error("an advice update must change the fingerprint")
	}
}
