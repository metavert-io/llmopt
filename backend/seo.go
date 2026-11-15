package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"
)

// ---------- shared types for templates ----------

type sharePageData struct {
	BrandName     string
	Domain        string
	ShareID       string
	Score         int
	HasScreenshot bool

	// Availability flags for tab nav
	HasBrand  bool
	HasVideo  bool
	HasReddit bool
	HasSearch bool
	HasTodos  bool

	// Tab-specific scores for overview links
	VideoScore  int
	RedditScore int
	SearchScore int
	TodoCount   int

	// Data
	Summary       string
	Components    []vsComponent
	Optimizations []Optimization
	Brand         *BrandProfile
	Video         *VideoAnalysis
	Reddit        *RedditAnalysis
	Search        *SearchAnalysis
	Todos         []TodoItem
}

type vsComponent struct {
	Name      string
	Score     int
	Weight    float64
	Available bool
}

// ---------- data fetching ----------

// fetchShareContext loads the DomainShare + brand name. Returns nil if not found.
func fetchShareContext(ctx context.Context, mongoDB *MongoDB, shareID string) (*DomainShare, string, bool) {
	var ds DomainShare
	err := mongoDB.DomainShares().FindOne(ctx, bson.M{
		"shareId":    shareID,
		"visibility": bson.M{"$in": []string{"public", "popular"}},
	}).Decode(&ds)
	if err != nil {
		return nil, "", false
	}

	brandName := ds.Domain
	var bp BrandProfile
	if mongoDB.BrandProfiles().FindOne(ctx, bson.M{
		"tenantId": ds.TenantID,
		"domain":   ds.Domain,
	}).Decode(&bp) == nil && bp.BrandName != "" {
		brandName = bp.BrandName
	}

	// Increment view count in background
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mongoDB.DomainShares().UpdateOne(bgCtx, bson.M{"_id": ds.ID}, bson.M{"$inc": bson.M{"viewCount": 1}})
	}()

	return &ds, brandName, true
}

// fetchFullShareData loads all data for the share overview page.
func fetchFullShareData(ctx context.Context, mongoDB *MongoDB, ds *DomainShare, brandName string) *sharePageData {
	td := bson.D{{Key: "tenantId", Value: ds.TenantID}, {Key: "domain", Value: ds.Domain}}

	data := &sharePageData{
		BrandName: brandName,
		Domain:    ds.Domain,
		ShareID:   ds.ShareID,
	}

	g, gctx := errgroup.WithContext(ctx)

	// Optimizations
	g.Go(func() error {
		cur, err := mongoDB.Optimizations().Find(gctx, td,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(20))
		if err != nil {
			return nil
		}
		var opts []Optimization
		if cur.All(gctx, &opts) == nil {
			data.Optimizations = opts
		}
		return nil
	})

	// Brand profile
	g.Go(func() error {
		var bp BrandProfile
		if mongoDB.BrandProfiles().FindOne(gctx, td).Decode(&bp) == nil {
			data.Brand = &bp
			data.HasBrand = true
		}
		return nil
	})

	// Video analysis
	g.Go(func() error {
		var va VideoAnalysis
		if mongoDB.VideoAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&va) == nil {
			data.Video = &va
			data.HasVideo = true
			if va.Result != nil {
				data.VideoScore = va.Result.OverallScore
			}
		}
		return nil
	})

	// Reddit analysis
	g.Go(func() error {
		var ra RedditAnalysis
		if mongoDB.RedditAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&ra) == nil {
			data.Reddit = &ra
			data.HasReddit = true
			if ra.Result != nil {
				data.RedditScore = ra.Result.OverallScore
			}
		}
		return nil
	})

	// Search analysis
	g.Go(func() error {
		var sa SearchAnalysis
		if mongoDB.SearchAnalyses().FindOne(gctx, td,
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&sa) == nil {
			data.Search = &sa
			data.HasSearch = true
			if sa.Result != nil {
				data.SearchScore = sa.Result.OverallScore
			}
		}
		return nil
	})

	// Todos
	g.Go(func() error {
		cur, err := mongoDB.Todos().Find(gctx, td,
			options.Find().SetSort(bson.D{{Key: "priority", Value: -1}, {Key: "createdAt", Value: -1}}))
		if err != nil {
			return nil
		}
		var todos []TodoItem
		if cur.All(gctx, &todos) == nil {
			data.Todos = todos
			data.TodoCount = len(todos)
			data.HasTodos = len(todos) > 0
		}
		return nil
	})

	// Domain summary
	g.Go(func() error {
		var dsm DomainSummary
		if mongoDB.DomainSummaries().FindOne(gctx, td).Decode(&dsm) == nil {
			data.Summary = dsm.Result.ExecutiveSummary
		}
		return nil
	})

	// Visibility score (weights: scoring.go)
	g.Go(func() error {
		vs := visibilityScoreFor(gctx, mongoDB, td)
		data.Score = vs.Score
		for _, c := range vs.Components {
			data.Components = append(data.Components, vsComponent{Name: c.Name, Score: c.Score, Weight: c.Weight, Available: c.Available})
		}
		return nil
	})

	// Screenshot check
	g.Go(func() error {
		cnt, err := mongoDB.BrandScreenshots().CountDocuments(gctx, bson.M{
			"domain":    ds.Domain,
			"sizeBytes": bson.M{"$gt": 0},
		})
		if err == nil && cnt > 0 {
			data.HasScreenshot = true
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Printf("fetchFullShareData error: %v", err)
	}

	return data
}

// ---------- page handlers ----------

// homeBrand is a brand entry for the homepage grid.
type homeBrand struct {
	Domain        string
	BrandName     string
	ShareID       string
	AvgScore      int
	ReportCount   int
	HasVideo      bool
	HasScreenshot bool
}

// handleHomePage serves the server-rendered marketing homepage.
func handleHomePage(mongoDB *MongoDB, staticDir string) http.HandlerFunc {
	type homeData struct {
		Brands []homeBrand
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// If path is not exactly "/" let SPA handle it
		if r.URL.Path != "/" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		// Browsers get the SPA (which handles auth client-side via localStorage JWT).
		// Crawlers and fetch tools get the server-rendered marketing page.
		if !isCrawler(r.UserAgent()) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		cur, err := mongoDB.DomainShares().Find(ctx, bson.M{"visibility": "popular"})
		if err != nil {
			w.Header().Set("Cache-Control", "public, max-age=300")
			renderPage(w, "home.html", homeData{})
			return
		}
		var shares []DomainShare
		if cur.All(ctx, &shares) != nil || len(shares) == 0 {
			w.Header().Set("Cache-Control", "public, max-age=300")
			renderPage(w, "home.html", homeData{})
			return
		}

		// Build lookup structures (same batch approach as handleGetPopularDomains)
		type domainKey struct{ tenantID, domain string }
		keyIndex := make(map[domainKey]int, len(shares))
		results := make([]homeBrand, len(shares))
		orFilter := make(bson.A, 0, len(shares))
		domains := make([]string, 0, len(shares))

		for i, s := range shares {
			results[i] = homeBrand{Domain: s.Domain, ShareID: s.ShareID, BrandName: s.Domain}
			keyIndex[domainKey{s.TenantID, s.Domain}] = i
			orFilter = append(orFilter, bson.M{"tenantId": s.TenantID, "domain": s.Domain})
			domains = append(domains, s.Domain)
		}
		batchFilter := bson.M{"$or": orFilter}

		g, gctx := errgroup.WithContext(ctx)

		// Brand names
		g.Go(func() error {
			bpCur, err := mongoDB.BrandProfiles().Find(gctx, batchFilter,
				options.Find().SetProjection(bson.M{"tenantId": 1, "domain": 1, "brandName": 1}))
			if err != nil {
				return nil
			}
			defer bpCur.Close(gctx)
			for bpCur.Next(gctx) {
				var bp struct {
					TenantID  string `bson:"tenantId"`
					Domain    string `bson:"domain"`
					BrandName string `bson:"brandName"`
				}
				if bpCur.Decode(&bp) == nil {
					if idx, ok := keyIndex[domainKey{bp.TenantID, bp.Domain}]; ok && bp.BrandName != "" {
						results[idx].BrandName = bp.BrandName
					}
				}
			}
			return nil
		})

		// Optimization counts + avg scores
		g.Go(func() error {
			pipeline := mongo.Pipeline{
				{{Key: "$match", Value: batchFilter}},
				{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: bson.D{{Key: "tenantId", Value: "$tenantId"}, {Key: "domain", Value: "$domain"}}},
					{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
					{Key: "avgScore", Value: bson.D{{Key: "$avg", Value: "$result.overallScore"}}},
				}}},
			}
			oCur, err := mongoDB.Optimizations().Aggregate(gctx, pipeline)
			if err != nil {
				return nil
			}
			defer oCur.Close(gctx)
			for oCur.Next(gctx) {
				var row struct {
					ID       struct{ TenantID, Domain string } `bson:"_id"`
					Count    int                               `bson:"count"`
					AvgScore float64                           `bson:"avgScore"`
				}
				if oCur.Decode(&row) == nil {
					if idx, ok := keyIndex[domainKey{row.ID.TenantID, row.ID.Domain}]; ok {
						results[idx].ReportCount = row.Count
						results[idx].AvgScore = int(row.AvgScore)
					}
				}
			}
			return nil
		})

		// Analysis counts (fallback for report count)
		g.Go(func() error {
			pipeline := mongo.Pipeline{
				{{Key: "$match", Value: batchFilter}},
				{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: bson.D{{Key: "tenantId", Value: "$tenantId"}, {Key: "domain", Value: "$domain"}}},
					{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
			}
			aCur, err := mongoDB.Analyses().Aggregate(gctx, pipeline)
			if err != nil {
				return nil
			}
			defer aCur.Close(gctx)
			for aCur.Next(gctx) {
				var row struct {
					ID    struct{ TenantID, Domain string } `bson:"_id"`
					Count int                               `bson:"count"`
				}
				if aCur.Decode(&row) == nil {
					if idx, ok := keyIndex[domainKey{row.ID.TenantID, row.ID.Domain}]; ok {
						// Only set if no optimization count yet
						if results[idx].ReportCount == 0 {
							results[idx].ReportCount = row.Count
						}
					}
				}
			}
			return nil
		})

		// Video existence
		g.Go(func() error {
			vCur, err := mongoDB.VideoAnalyses().Find(gctx, batchFilter,
				options.Find().SetProjection(bson.M{"tenantId": 1, "domain": 1}))
			if err != nil {
				return nil
			}
			defer vCur.Close(gctx)
			for vCur.Next(gctx) {
				var v struct {
					TenantID string `bson:"tenantId"`
					Domain   string `bson:"domain"`
				}
				if vCur.Decode(&v) == nil {
					if idx, ok := keyIndex[domainKey{v.TenantID, v.Domain}]; ok {
						results[idx].HasVideo = true
					}
				}
			}
			return nil
		})

		// Screenshot existence
		g.Go(func() error {
			ssCur, err := mongoDB.BrandScreenshots().Find(gctx,
				bson.M{"domain": bson.M{"$in": domains}, "sizeBytes": bson.M{"$gt": 0}},
				options.Find().SetProjection(bson.M{"domain": 1}))
			if err != nil {
				return nil
			}
			defer ssCur.Close(gctx)
			for ssCur.Next(gctx) {
				var ss struct {
					Domain string `bson:"domain"`
				}
				if ssCur.Decode(&ss) == nil {
					for i := range results {
						if results[i].Domain == ss.Domain {
							results[i].HasScreenshot = true
						}
					}
				}
			}
			return nil
		})

		_ = g.Wait()

		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "home.html", homeData{Brands: results})
	}
}

// handleDocsPage serves the server-rendered API documentation.
func handleDocsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		renderPage(w, "docs.html", nil)
	}
}

// handleSharePageSSR serves the full server-rendered share overview.
func handleSharePageSSR(mongoDB *MongoDB, staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			// Not found — serve SPA for potential private share
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		data := fetchFullShareData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share.html", data)
	}
}

// handleShareBrandPage serves the brand intelligence sub-page.
func handleShareBrandPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_brand.html", data)
	}
}

// handleShareVideoPage serves the video authority sub-page.
func handleShareVideoPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_video.html", data)
	}
}

// handleShareRedditPage serves the Reddit authority sub-page.
func handleShareRedditPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_reddit.html", data)
	}
}

// handleShareSearchPage serves the search visibility sub-page.
func handleShareSearchPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_search.html", data)
	}
}

// handleShareTodosPage serves the action items sub-page.
func handleShareTodosPage(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		ds, brandName, ok := fetchShareContext(ctx, mongoDB, shareID)
		if !ok {
			http.NotFound(w, r)
			return
		}

		data := fetchSubPageData(ctx, mongoDB, ds, brandName)
		w.Header().Set("Cache-Control", "public, max-age=300")
		renderPage(w, "share_todos.html", data)
	}
}

// fetchSubPageData loads the share data needed for sub-page templates.
// Loads all data since sub-pages need tab availability flags.
func fetchSubPageData(ctx context.Context, mongoDB *MongoDB, ds *DomainShare, brandName string) *sharePageData {
	return fetchFullShareData(ctx, mongoDB, ds, brandName)
}
