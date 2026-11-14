package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"llmopt/internal/saas"
)

func TestListenAddr(t *testing.T) {
	cases := []struct {
		host  string
		local bool
		want  string
		fails bool
	}{
		{"", false, ":8080", false},
		{"", true, "127.0.0.1:8080", false},
		{"127.0.0.1", true, "127.0.0.1:8080", false},
		{"::1", true, "[::1]:8080", false},
		{"localhost", true, "localhost:8080", false},
		{"0.0.0.0", true, "", true},
		{"100.64.0.7", true, "", true}, // a Tailscale address is still not loopback
		{"192.168.4.20", true, "", true},
		{"0.0.0.0", false, "0.0.0.0:8080", false},
	}
	for _, c := range cases {
		t.Setenv("LLMOPT_LISTEN_HOST", c.host)
		got, err := listenAddr("8080", c.local)
		if (err != nil) != c.fails || got != c.want {
			t.Errorf("host %q local=%v: got %q, %v", c.host, c.local, got, err)
		}
	}
}

func TestLocalAccountFromEnv(t *testing.T) {
	t.Setenv("LLMOPT_LOCAL_USER_ID", "")
	t.Setenv("LLMOPT_LOCAL_TENANT_ID", "")
	if _, _, on, err := localAccountFromEnv(); on || err != nil {
		t.Errorf("unset: on=%v err=%v", on, err)
	}
	t.Setenv("LLMOPT_LOCAL_TENANT_ID", "t1")
	if _, _, on, err := localAccountFromEnv(); on || err == nil {
		t.Errorf("only the tenant set: on=%v err=%v", on, err)
	}
	t.Setenv("LLMOPT_LOCAL_USER_ID", "u1")
	if u, tn, on, err := localAccountFromEnv(); !on || err != nil || u != "u1" || tn != "t1" {
		t.Errorf("both set: %q %q on=%v err=%v", u, tn, on, err)
	}
}

// Every request runs as the configured account, with its tenant in context, and
// a user who isn't a member of the tenant gets nothing.
func TestLocalAccountWrap(t *testing.T) {
	db := testMongoDB(t)
	ctx := context.Background()
	userID, tenantID, otherTenant := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	mustInsert := func(coll string, doc bson.M) {
		if _, err := db.Database.Collection(coll).InsertOne(ctx, doc); err != nil {
			t.Fatal(err)
		}
	}
	mustInsert("users", bson.M{"_id": userID, "email": "owner@example.com", "isActive": true})
	mustInsert("tenants", bson.M{"_id": tenantID, "name": "Example", "isActive": true})
	mustInsert("tenants", bson.M{"_id": otherTenant, "name": "Other", "isActive": true})
	mustInsert("tenant_memberships", bson.M{"_id": primitive.NewObjectID(), "userId": userID, "tenantId": tenantID, "role": "owner", "joinedAt": time.Now()})

	sm := saas.NewMiddleware(saas.NewJWTValidator(randomSecret()), db.Database)
	var gotTenant, gotRole string
	h := func(w http.ResponseWriter, r *http.Request) {
		gotTenant = saas.TenantIDFromContext(r.Context())
		gotRole = saas.MemberRoleFromContext(r.Context())
	}

	la := &localAccount{userID: userID.Hex(), tenantID: tenantID.Hex(), sm: sm}
	w := httptest.NewRecorder()
	la.wrap(h)(w, httptest.NewRequest("GET", "/api/brands", nil))
	if w.Code != http.StatusOK || gotTenant != tenantID.Hex() || gotRole != "owner" {
		t.Errorf("member: code %d tenant %q role %q", w.Code, gotTenant, gotRole)
	}

	gotTenant = ""
	la = &localAccount{userID: userID.Hex(), tenantID: otherTenant.Hex(), sm: sm}
	if _, err := la.check(ctx); err == nil {
		t.Error("check passed for a tenant the user doesn't belong to")
	}
	w = httptest.NewRecorder()
	la.wrap(h)(w, httptest.NewRequest("GET", "/api/brands", nil))
	if w.Code != http.StatusServiceUnavailable || gotTenant != "" {
		t.Errorf("non-member: code %d, handler ran with tenant %q", w.Code, gotTenant)
	}
}
