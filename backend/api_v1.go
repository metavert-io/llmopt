package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

)

// ─── API v1 Handlers ────────────────────────────────────────────────────────

// handleAPIv1ListDomains returns all unique domains that have data for the tenant.
func handleAPIv1ListDomains(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		domainSet := make(map[string]bool)

		// Query distinct domains from all tenant-scoped collections
		collections := mongoDB.allTenantCollections()
		for _, col := range collections {
			vals, err := col.Distinct(ctx, "domain", filter)
			if err == nil {
				for _, v := range vals {
					if s, ok := v.(string); ok && s != "" {
						domainSet[s] = true
					}
				}
			}
		}

		domains := make([]string, 0, len(domainSet))
		for d := range domainSet {
			domains = append(domains, d)
		}
		sort.Strings(domains)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"domains": domains})
	}
}

// noRawText is a projection that excludes rawText from results.
var noRawText = bson.D{{Key: "rawText", Value: 0}}

func handleAPIv1GetAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetProjection(noRawText)

		var result bson.M
		err := mongoDB.Analyses().FindOne(ctx, filter, opts).Decode(&result)
		if err != nil {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleAPIv1GetOptimizations(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetProjection(noRawText)

		cursor, err := mongoDB.Optimizations().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if results == nil {
			results = []bson.M{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func handleAPIv1GetVideo(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetProjection(noRawText)

		var result bson.M
		err := mongoDB.VideoAnalyses().FindOne(ctx, filter, opts).Decode(&result)
		if err != nil {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleAPIv1GetReddit(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		var result bson.M
		err := mongoDB.RedditAnalyses().FindOne(ctx, filter, opts).Decode(&result)
		if err != nil {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleAPIv1GetSearch(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		raw, err := mongoDB.SearchAnalyses().FindOne(ctx, filter, opts).Raw()
		if err != nil {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}
		// Scores with the current weights (see scoring.go).
		result, err := rescoredSearchDoc(raw)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleAPIv1GetSummary(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})

		var result bson.M
		err := mongoDB.DomainSummaries().FindOne(ctx, filter).Decode(&result)
		if err != nil {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleAPIv1GetTests(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{
			{Key: "domain", Value: domain},
			{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
		})
		opts := options.Find().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetLimit(10)

		cursor, err := mongoDB.LLMTests().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if results == nil {
			results = []bson.M{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tests": results,
			"count": len(results),
		})
	}
}

func handleAPIv1GetScore(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(computeVisibilityScore(ctx, mongoDB, domain, r.Context()))
	}
}

func handleAPIv1GetBrand(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})

		var result bson.M
		err := mongoDB.BrandProfiles().FindOne(ctx, filter).Decode(&result)
		if err != nil {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// ─── Todo Handlers ──────────────────────────────────────────────────────────

func handleAPIv1ListTodos(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		if status := r.URL.Query().Get("status"); status != "" {
			filter = append(filter, bson.E{Key: "status", Value: status})
		}
		if domain := r.URL.Query().Get("domain"); domain != "" {
			filter = append(filter, bson.E{Key: "domain", Value: normalizeDomain(domain)})
		}

		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetLimit(200)

		cursor, err := mongoDB.Todos().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var todos []TodoItem
		if err := cursor.All(ctx, &todos); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if todos == nil {
			todos = []TodoItem{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(todos)
	}
}

func handleAPIv1GetTodo(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid ID","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}})

		var todo TodoItem
		err = mongoDB.Todos().FindOne(ctx, filter).Decode(&todo)
		if err != nil {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(todo)
	}
}

func handleAPIv1UpdateTodo(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid ID","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}

		if req.Status != "todo" && req.Status != "completed" && req.Status != "backlogged" && req.Status != "archived" {
			http.Error(w, `{"error":"status must be 'todo', 'completed', 'backlogged', or 'archived'","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		now := time.Now()
		var update bson.D
		switch req.Status {
		case "completed":
			update = bson.D{
				{Key: "$set", Value: bson.D{{Key: "status", Value: req.Status}, {Key: "completedAt", Value: now}}},
				{Key: "$unset", Value: bson.D{{Key: "backloggedAt", Value: ""}, {Key: "archivedAt", Value: ""}}},
			}
		case "backlogged":
			update = bson.D{
				{Key: "$set", Value: bson.D{{Key: "status", Value: req.Status}, {Key: "backloggedAt", Value: now}}},
				{Key: "$unset", Value: bson.D{{Key: "completedAt", Value: ""}, {Key: "archivedAt", Value: ""}}},
			}
		case "archived":
			update = bson.D{
				{Key: "$set", Value: bson.D{{Key: "status", Value: req.Status}, {Key: "archivedAt", Value: now}}},
				{Key: "$unset", Value: bson.D{{Key: "completedAt", Value: ""}, {Key: "backloggedAt", Value: ""}}},
			}
		default: // "todo"
			update = bson.D{
				{Key: "$set", Value: bson.D{{Key: "status", Value: req.Status}}},
				{Key: "$unset", Value: bson.D{{Key: "completedAt", Value: ""}, {Key: "backloggedAt", Value: ""}, {Key: "archivedAt", Value: ""}}},
			}
		}

		filter := tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}})
		result, err := mongoDB.Todos().UpdateOne(ctx, filter, update)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if result.MatchedCount == 0 {
			http.Error(w, `{"error":"not found","code":"NOT_FOUND"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"updated": true, "status": req.Status})
	}
}

func handleAPIv1BulkUpdateTodos(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs    []string `json:"ids"`
			Status string   `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}

		if len(req.IDs) == 0 {
			http.Error(w, `{"error":"ids array is required","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}
		if len(req.IDs) > 100 {
			http.Error(w, `{"error":"maximum 100 IDs per request","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}
		if req.Status != "todo" && req.Status != "completed" && req.Status != "backlogged" && req.Status != "archived" {
			http.Error(w, `{"error":"status must be 'todo', 'completed', 'backlogged', or 'archived'","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}

		oids := make([]primitive.ObjectID, 0, len(req.IDs))
		for _, id := range req.IDs {
			oid, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				http.Error(w, `{"error":"invalid ID in array: `+id+`","code":"BAD_REQUEST"}`, http.StatusBadRequest)
				return
			}
			oids = append(oids, oid)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		now := time.Now()
		setFields := bson.D{{Key: "status", Value: req.Status}}
		unsetFields := bson.D{}
		switch req.Status {
		case "completed":
			setFields = append(setFields, bson.E{Key: "completedAt", Value: now})
			unsetFields = bson.D{{Key: "backloggedAt", Value: ""}, {Key: "archivedAt", Value: ""}}
		case "backlogged":
			setFields = append(setFields, bson.E{Key: "backloggedAt", Value: now})
			unsetFields = bson.D{{Key: "completedAt", Value: ""}, {Key: "archivedAt", Value: ""}}
		case "archived":
			setFields = append(setFields, bson.E{Key: "archivedAt", Value: now})
			unsetFields = bson.D{{Key: "completedAt", Value: ""}, {Key: "backloggedAt", Value: ""}}
		default:
			unsetFields = bson.D{{Key: "completedAt", Value: ""}, {Key: "backloggedAt", Value: ""}, {Key: "archivedAt", Value: ""}}
		}

		update := bson.D{{Key: "$set", Value: setFields}}
		if len(unsetFields) > 0 {
			update = append(update, bson.E{Key: "$unset", Value: unsetFields})
		}

		filter := tenantFilter(r.Context(), bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: oids}}}})
		result, err := mongoDB.Todos().UpdateMany(ctx, filter, update)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"updated": result.ModifiedCount,
			"matched": result.MatchedCount,
			"status":  req.Status,
		})
	}
}

// ─── Documentation Endpoint ─────────────────────────────────────────────────

func handleAPIv1Docs() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte(apiV1DocsMarkdown))
	}
}

const apiV1DocsMarkdown = `# LLM Optimizer API v1

## Authentication

All API requests require authentication via Bearer token. Two methods are supported:

### API Key (recommended for integrations)

Generate an API key from the admin panel (Settings > API Keys). Include it in the Authorization header:

` + "```" + `
Authorization: Bearer lsk_your_api_key_here
` + "```" + `

**Admin keys** automatically resolve to the root tenant — no additional headers needed.
**User keys** require the ` + "`X-Tenant-ID`" + ` header to specify which tenant's data to access.

### JWT Token (browser sessions)

For browser-based access, use the JWT token from your login session with the ` + "`X-Tenant-ID`" + ` header:

` + "```" + `
Authorization: Bearer eyJ...
X-Tenant-ID: 64a1b2c3d4e5f6g7h8i9j0k1
` + "```" + `

---

## Rate Limits

- **Read endpoints**: 60 requests/minute
- **Write endpoints**: 30 requests/minute

Rate limit headers are included in responses:
- ` + "`X-RateLimit-Remaining`" + `: requests remaining in window
- ` + "`X-RateLimit-Reset`" + `: UTC timestamp when window resets

---

## Error Responses

All errors return JSON with ` + "`error`" + ` and ` + "`code`" + ` fields:

` + "```json" + `
{"error": "not found", "code": "NOT_FOUND"}
` + "```" + `

| HTTP Status | Code | Description |
|---|---|---|
| 400 | BAD_REQUEST | Invalid input or missing required fields |
| 401 | UNAUTHORIZED | Missing or invalid authentication |
| 403 | FORBIDDEN | Insufficient permissions (admin/owner required) |
| 404 | NOT_FOUND | Resource not found |
| 429 | RATE_LIMITED | Rate limit exceeded |

---

## Endpoints

### List Domains

Returns all domains that have data for your tenant.

` + "```" + `
GET /api/v1/domains
` + "```" + `

**Response:**

` + "```json" + `
{
  "domains": ["anthropic.ai", "example.com", "openai.com"]
}
` + "```" + `

---

### Get Analysis Report

Returns the latest site analysis for a domain.

` + "```" + `
GET /api/v1/domains/{domain}/analysis
` + "```" + `

**Response:** Analysis object with ` + "`result`" + ` containing ` + "`site_summary`" + `, ` + "`questions`" + ` array, and ` + "`crawled_pages`" + `.

---

### Get Optimizations

Returns all answer optimization reports for a domain.

` + "```" + `
GET /api/v1/domains/{domain}/optimizations
` + "```" + `

**Response:** Array of optimization objects, each with 4-dimension scores (content_authority, structural_optimization, source_authority, knowledge_persistence), competitors, and recommendations.

---

### Get Video Authority

Returns the latest video authority analysis (4-pillar: Transcript Authority, Topical Dominance, Citation Network, Brand Narrative).

` + "```" + `
GET /api/v1/domains/{domain}/video
` + "```" + `

---

### Get Reddit Authority

Returns the latest Reddit authority analysis (4-pillar: Presence, Sentiment, Competitive, Training Signal).

` + "```" + `
GET /api/v1/domains/{domain}/reddit
` + "```" + `

---

### Get Search Visibility

Returns the latest search visibility analysis (5-pillar: AIO Readiness, Crawl Accessibility, Brand Momentum, Content Freshness, Category Discovery).

` + "```" + `
GET /api/v1/domains/{domain}/search
` + "```" + `

---

### Get Domain Summary

Returns the cross-report executive summary for a domain.

` + "```" + `
GET /api/v1/domains/{domain}/summary
` + "```" + `

---

### Get LLM Test Results

Returns up to 10 most recent LLM knowledge test runs for a domain.

` + "```" + `
GET /api/v1/domains/{domain}/tests
` + "```" + `

**Response:**

` + "```json" + `
{
  "tests": [...],
  "count": 3
}
` + "```" + `

---

### Get Visibility Score

Returns the aggregate visibility score computed from all available report types.

` + "```" + `
GET /api/v1/domains/{domain}/score
` + "```" + `

**Response:**

` + "```json" + `
{
  "score": 72,
  "components": [
    {"name": "Optimization", "score": 85, "weight": 0.30, "available": true},
    {"name": "Video Authority", "score": 60, "weight": 0.20, "available": true},
    {"name": "Reddit Authority", "score": 0, "weight": 0.20, "available": false},
    {"name": "Search Visibility", "score": 70, "weight": 0.15, "available": true},
    {"name": "LLM Test", "score": 65, "weight": 0.15, "available": true}
  ],
  "available": 4,
  "total": 5
}
` + "```" + `

---

### Get Brand Profile

Returns the brand profile for a domain.

` + "```" + `
GET /api/v1/domains/{domain}/brand
` + "```" + `

---

### List Todos

Returns action items (to-dos) with optional filters.

` + "```" + `
GET /api/v1/todos
GET /api/v1/todos?status=todo
GET /api/v1/todos?domain=example.com
GET /api/v1/todos?status=todo&domain=example.com
` + "```" + `

**Query Parameters:**

| Parameter | Description |
|---|---|
| status | Filter by status: ` + "`todo`" + `, ` + "`completed`" + `, ` + "`backlogged`" + `, ` + "`archived`" + ` |
| domain | Filter by domain name |

---

### Get Todo

Returns a single todo item by ID.

` + "```" + `
GET /api/v1/todos/{id}
` + "```" + `

---

### Update Todo Status

Updates the status of a single todo item. **Requires admin or owner role.**

` + "```" + `
PATCH /api/v1/todos/{id}
` + "```" + `

**Request Body:**

` + "```json" + `
{"status": "completed"}
` + "```" + `

Valid statuses: ` + "`todo`" + `, ` + "`completed`" + `, ` + "`backlogged`" + `, ` + "`archived`" + `

**Response:**

` + "```json" + `
{"updated": true, "status": "completed"}
` + "```" + `

---

### Bulk Update Todos

Updates the status of multiple todo items at once. **Requires admin or owner role.** Maximum 100 IDs per request.

` + "```" + `
POST /api/v1/todos/bulk-update
` + "```" + `

**Request Body:**

` + "```json" + `
{
  "ids": ["64a1b2c3d4e5f6g7h8i9j0k1", "64a1b2c3d4e5f6g7h8i9j0k2"],
  "status": "archived"
}
` + "```" + `

**Response:**

` + "```json" + `
{"updated": 2, "matched": 2, "status": "archived"}
` + "```" + `
`
