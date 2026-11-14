package main

// Saved LLM Test queries: the query list a tenant builds for a domain is kept
// on the server, so it is there again after leaving the page and on the next run.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	maxSavedTestQueries  = 100
	maxSavedTestQueryLen = 1000
)

// cleanTestQueries trims the queries, drops blank ones, fills in a missing
// type or priority, and caps the list's size.
func cleanTestQueries(in []LLMTestQuery) []LLMTestQuery {
	out := make([]LLMTestQuery, 0, len(in))
	for _, q := range in {
		q.Query = strings.TrimSpace(q.Query)
		if q.Query == "" {
			continue
		}
		if len(q.Query) > maxSavedTestQueryLen {
			q.Query = truncateUTF8(q.Query, maxSavedTestQueryLen)
		}
		if q.Type == "" {
			q.Type = "custom"
		}
		if q.Priority == "" {
			q.Priority = "medium"
		}
		out = append(out, q)
		if len(out) == maxSavedTestQueries {
			break
		}
	}
	return out
}

// truncateUTF8 cuts s to at most n bytes without splitting a character.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && n < len(s) && (s[n]&0xC0) == 0x80 {
		n--
	}
	return s[:n]
}

// saveTestQueries stores the domain's query list for the tenant in ctx.
func saveTestQueries(ctx context.Context, mongoDB *MongoDB, domain string, queries []LLMTestQuery) (TestQuerySet, error) {
	set := TestQuerySet{
		Domain:    domain,
		Queries:   cleanTestQueries(queries),
		UpdatedAt: time.Now().UTC(),
	}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "queries", Value: set.Queries},
		{Key: "updatedAt", Value: set.UpdatedAt},
	}}}
	_, err := mongoDB.TestQuerySets().UpdateOne(ctx,
		tenantFilter(ctx, bson.D{{Key: "domain", Value: domain}}),
		update, options.Update().SetUpsert(true))
	return set, err
}

// handleGetTestQueries returns the domain's saved query list. Before anything
// has been saved it falls back to the queries of the domain's latest test.
// source is "saved", "last_test" or "none".
func handleGetTestQueries(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		resp := struct {
			Queries   []LLMTestQuery `json:"queries"`
			Source    string         `json:"source"`
			UpdatedAt *time.Time     `json:"updated_at,omitempty"`
		}{Queries: []LLMTestQuery{}, Source: "none"}

		var set TestQuerySet
		err := mongoDB.TestQuerySets().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&set)
		switch {
		case err == nil:
			resp.Source = "saved"
			resp.Queries = append(resp.Queries, set.Queries...)
			resp.UpdatedAt = &set.UpdatedAt
		case errors.Is(err, mongo.ErrNoDocuments):
			var last LLMTest
			err = mongoDB.LLMTests().FindOne(ctx,
				tenantFilter(r.Context(), bson.D{
					{Key: "domain", Value: domain},
					{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
				}),
				options.FindOne().
					SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
					SetProjection(bson.D{{Key: "queries", Value: 1}, {Key: "generatedAt", Value: 1}}),
			).Decode(&last)
			if err == nil && len(last.Queries) > 0 {
				resp.Source = "last_test"
				resp.Queries = append(resp.Queries, last.Queries...)
			} else if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// handlePutTestQueries replaces the domain's saved query list.
func handlePutTestQueries(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		var req struct {
			Queries []LLMTestQuery `json:"queries"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		set, err := saveTestQueries(ctx, mongoDB, domain, req.Queries)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(set)
	}
}
