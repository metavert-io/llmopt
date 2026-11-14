package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func getSavedQueries(t *testing.T, db *MongoDB, domain string) (queries []LLMTestQuery, source string) {
	t.Helper()
	req := testRequest(t, "GET", "/api/test/"+domain+"/queries", nil)
	req.SetPathValue("domain", domain)
	w := httptest.NewRecorder()
	handleGetTestQueries(db)(w, req)
	assertStatus(t, w, 200)
	resp := assertJSON[struct {
		Queries []LLMTestQuery `json:"queries"`
		Source  string         `json:"source"`
	}](t, w)
	return resp.Queries, resp.Source
}

func TestTestQueries_SaveAndLoad(t *testing.T) {
	db := testMongoDB(t)
	const domain = "example.com"

	if qs, src := getSavedQueries(t, db, domain); src != "none" || len(qs) != 0 {
		t.Fatalf("fresh domain: source %q, %d queries", src, len(qs))
	}

	body := map[string]any{"queries": []map[string]any{
		{"query": "  What is Example Health?  ", "type": "brand", "priority": "high"},
		{"query": "   "}, // blank rows are dropped
		{"query": "best telehealth for teens?"},
	}}
	req := testRequest(t, "PUT", "/api/test/"+domain+"/queries", body)
	req.SetPathValue("domain", domain)
	w := httptest.NewRecorder()
	handlePutTestQueries(db)(w, req)
	assertStatus(t, w, 200)

	qs, src := getSavedQueries(t, db, domain)
	if src != "saved" || len(qs) != 2 {
		t.Fatalf("source %q, queries %+v", src, qs)
	}
	if qs[0].Query != "What is Example Health?" || qs[1].Type != "custom" || qs[1].Priority != "medium" {
		t.Errorf("queries not cleaned: %+v", qs)
	}

	// Another tenant doesn't see them.
	other := httptest.NewRequest("GET", "/api/test/"+domain+"/queries", nil)
	other = other.WithContext(testAuthContext("other-tenant", "u2"))
	other.SetPathValue("domain", domain)
	w = httptest.NewRecorder()
	handleGetTestQueries(db)(w, other)
	if resp := assertJSON[struct {
		Source string `json:"source"`
	}](t, w); resp.Source != "none" {
		t.Errorf("other tenant sees source %q", resp.Source)
	}
}

// Before anything is saved, the latest test's queries come back.
func TestTestQueries_FallBackToLastTest(t *testing.T) {
	db := testMongoDB(t)
	const domain = "example.org"
	_, err := db.LLMTests().InsertOne(context.Background(), LLMTest{
		TenantID: "test-tenant", Domain: domain, GeneratedAt: time.Now(),
		Queries: []LLMTestQuery{{Query: "from the last run", Type: "custom", Priority: "high"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	qs, src := getSavedQueries(t, db, domain)
	if src != "last_test" || len(qs) != 1 || qs[0].Query != "from the last run" {
		t.Errorf("source %q, queries %+v", src, qs)
	}
}

func TestCleanTestQueries_Caps(t *testing.T) {
	var in []LLMTestQuery
	for i := 0; i < maxSavedTestQueries+5; i++ {
		in = append(in, LLMTestQuery{Query: "q"})
	}
	in[0].Query = strings.Repeat("é", maxSavedTestQueryLen) // 2 bytes each
	out := cleanTestQueries(in)
	if len(out) != maxSavedTestQueries {
		t.Errorf("kept %d queries", len(out))
	}
	if len(out[0].Query) > maxSavedTestQueryLen || !strings.HasSuffix(out[0].Query, "é") {
		t.Errorf("query cut badly: %d bytes", len(out[0].Query))
	}
}
