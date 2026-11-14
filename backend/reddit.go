package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
)

// redditCacheTTL controls how long Reddit search/thread data is cached.
const redditCacheTTL = 7 * 24 * time.Hour // 7 days

var redditClient = &http.Client{Timeout: 15 * time.Second}

// redditBaseURL is the base URL for Reddit (var for test swapping).
var redditBaseURL = "https://www.reddit.com"

// ── Reddit JSON API fetcher ─────────────────────────────────────────

var browserUAs = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.2 Safari/605.1.15",
}

func randomUA() string {
	return browserUAs[rand.Intn(len(browserUAs))]
}

// errRedditBlocked is Reddit refusing a request outright (HTTP 403, "You've been
// blocked by network security"). Reddit now answers anonymous JSON requests this
// way from servers and home connections alike, so retrying doesn't help.
var errRedditBlocked = errors.New("Reddit refused the request")

// redditHTTPGet fetches a URL with browser-like headers, using WARP proxy if available.
// It retries once via WARP on 429/403.
func redditHTTPGet(rawURL string) ([]byte, error) {
	do := func(client *http.Client, label string) ([]byte, int, error) {
		req, err := http.NewRequest("GET", rawURL, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("User-Agent", randomUA())
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("reddit %s request failed: %w", label, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return body, resp.StatusCode, nil
	}

	body, status, err := do(redditClient, "direct")
	if err != nil {
		return nil, err
	}
	if status == 200 {
		return body, nil
	}

	// Retry via WARP on 429/403
	if (status == 429 || status == 403) {
		if wc := warpClient(); wc != nil {
			log.Printf("Reddit %d on direct, retrying via WARP: %s", status, rawURL)
			body, status, err = do(wc, "warp")
			if err != nil {
				return nil, err
			}
			if status == 200 {
				return body, nil
			}
		}
	}

	if status == 403 {
		return nil, fmt.Errorf("%w (HTTP 403 for %s)", errRedditBlocked, rawURL)
	}
	return nil, fmt.Errorf("reddit HTTP %d for %s", status, rawURL)
}

// RedditThread represents a parsed Reddit thread.
type RedditThread struct {
	ID           string    `json:"id"`
	Subreddit    string    `json:"subreddit"`
	Title        string    `json:"title"`
	SelfText     string    `json:"self_text"`
	Author       string    `json:"author"`
	Score        int       `json:"score"`
	UpvoteRatio  float64   `json:"upvote_ratio"`
	NumComments  int       `json:"num_comments"`
	URL          string    `json:"url"`
	Permalink    string    `json:"permalink"`
	CreatedUTC   time.Time `json:"created_utc"`
	TopComments  []RedditComment `json:"top_comments,omitempty"`
	IsSelfPost   bool      `json:"is_self_post"`
	Source       string    `json:"source,omitempty"` // redditWebSearchSource when found through web search
}

// RedditComment represents a parsed Reddit comment.
type RedditComment struct {
	Author string `json:"author"`
	Body   string `json:"body"`
	Score  int    `json:"score"`
	Depth  int    `json:"depth"`
}

// redditSearch searches a subreddit for threads matching a query.
// Uses the .json endpoint: https://www.reddit.com/r/{sub}/search.json?q=...&sort=relevance&t=year
func redditSearch(subreddit, query string, timeFilter string, limit int) ([]RedditThread, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if timeFilter == "" {
		timeFilter = "year"
	}

	var baseURL string
	if subreddit == "" || subreddit == "all" {
		baseURL = redditBaseURL + "/search.json"
	} else {
		// Strip r/ prefix if present
		subreddit = strings.TrimPrefix(subreddit, "r/")
		baseURL = fmt.Sprintf("%s/r/%s/search.json", redditBaseURL, url.PathEscape(subreddit))
	}

	u := fmt.Sprintf("%s?q=%s&sort=relevance&t=%s&limit=%d&restrict_sr=1",
		baseURL, url.QueryEscape(query), url.QueryEscape(timeFilter), limit)

	body, err := redditHTTPGet(u)
	if err != nil {
		return nil, err
	}

	var listing struct {
		Data struct {
			Children []struct {
				Data json.RawMessage `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, fmt.Errorf("reddit search parse error: %w", err)
	}

	var threads []RedditThread
	for _, child := range listing.Data.Children {
		t, err := parseRedditPost(child.Data)
		if err != nil {
			continue
		}
		threads = append(threads, t)
	}
	return threads, nil
}

// redditFetchThread fetches a full thread with top comments.
func redditFetchThread(permalink string) (*RedditThread, error) {
	// Ensure permalink starts with /
	if !strings.HasPrefix(permalink, "/") {
		permalink = "/" + permalink
	}
	// Remove trailing slash, append .json
	permalink = strings.TrimRight(permalink, "/")
	u := fmt.Sprintf("%s%s.json?limit=25&sort=top&depth=2", redditBaseURL, permalink)

	body, err := redditHTTPGet(u)
	if err != nil {
		return nil, err
	}

	// Reddit thread JSON is an array of two listings: [post, comments]
	var listings []struct {
		Data struct {
			Children []struct {
				Kind string          `json:"kind"`
				Data json.RawMessage `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listings); err != nil {
		return nil, fmt.Errorf("reddit thread parse error: %w", err)
	}

	if len(listings) == 0 || len(listings[0].Data.Children) == 0 {
		return nil, fmt.Errorf("reddit thread empty")
	}

	thread, err := parseRedditPost(listings[0].Data.Children[0].Data)
	if err != nil {
		return nil, err
	}

	// Parse top comments
	if len(listings) > 1 {
		for _, child := range listings[1].Data.Children {
			if child.Kind != "t1" {
				continue
			}
			c, err := parseRedditComment(child.Data, 0)
			if err != nil {
				continue
			}
			thread.TopComments = append(thread.TopComments, c)
		}
	}

	return &thread, nil
}

func parseRedditPost(raw json.RawMessage) (RedditThread, error) {
	var p struct {
		ID          string  `json:"id"`
		Subreddit   string  `json:"subreddit"`
		Title       string  `json:"title"`
		SelfText    string  `json:"selftext"`
		Author      string  `json:"author"`
		Score       int     `json:"score"`
		UpvoteRatio float64 `json:"upvote_ratio"`
		NumComments int     `json:"num_comments"`
		URL         string  `json:"url"`
		Permalink   string  `json:"permalink"`
		CreatedUTC  float64 `json:"created_utc"`
		IsSelf      bool    `json:"is_self"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return RedditThread{}, err
	}
	return RedditThread{
		ID:          p.ID,
		Subreddit:   p.Subreddit,
		Title:       p.Title,
		SelfText:    p.SelfText,
		Author:      p.Author,
		Score:       p.Score,
		UpvoteRatio: p.UpvoteRatio,
		NumComments: p.NumComments,
		URL:         p.URL,
		Permalink:   p.Permalink,
		CreatedUTC:  time.Unix(int64(p.CreatedUTC), 0),
		IsSelfPost:  p.IsSelf,
	}, nil
}

func parseRedditComment(raw json.RawMessage, depth int) (RedditComment, error) {
	var c struct {
		Author string `json:"author"`
		Body   string `json:"body"`
		Score  int    `json:"score"`
		Depth  int    `json:"depth"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return RedditComment{}, err
	}
	return RedditComment{
		Author: c.Author,
		Body:   c.Body,
		Score:  c.Score,
		Depth:  c.Depth,
	}, nil
}

// resolveSubreddits normalizes subreddit entries from brand intelligence.
// Handles formats like "r/golang", "/r/golang", "golang", "https://reddit.com/r/golang"
var subredditRe = regexp.MustCompile(`(?:reddit\.com)?/?r/([A-Za-z0-9_]+)`)

func normalizeSubreddit(s string) string {
	s = strings.TrimSpace(s)
	if m := subredditRe.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	// If it's just a bare name without r/ prefix
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimPrefix(s, "r/")
	if s != "" && !strings.Contains(s, "/") && !strings.Contains(s, " ") {
		return s
	}
	return ""
}

// redditDiscoverThreads searches for threads mentioning a brand/query across subreddits.
// Returns deduplicated threads sorted by relevance.
func redditDiscoverThreads(subreddits []string, searchTerms []string, timeFilter string, maxPerQuery int, delayBetween time.Duration, statusFn func(string)) ([]RedditThread, error) {
	if maxPerQuery <= 0 {
		maxPerQuery = 15
	}

	seen := map[string]bool{}
	var allThreads []RedditThread

	totalSearches := len(subreddits) * len(searchTerms)
	if len(subreddits) == 0 {
		// Search all of Reddit
		subreddits = []string{"all"}
		totalSearches = len(searchTerms)
	}

	searchNum, answered, blocked := 0, 0, 0
	var firstErr, blockedErr error
	for _, sub := range subreddits {
		for _, term := range searchTerms {
			searchNum++
			if statusFn != nil {
				statusFn(fmt.Sprintf("Searching r/%s for \"%s\" (%d/%d)...", sub, term, searchNum, totalSearches))
			}

			threads, err := redditSearch(sub, term, timeFilter, maxPerQuery)
			if err != nil {
				log.Printf("Reddit search error (r/%s, %q): %v", sub, term, err)
				if firstErr == nil {
					firstErr = err
				}
				if errors.Is(err, errRedditBlocked) {
					blocked++
					if blockedErr == nil {
						blockedErr = err
					}
					if answered == 0 && blocked >= 2 {
						return nil, err // Reddit is refusing these requests; the rest would be refused too
					}
				}
				continue
			}
			answered++

			for _, t := range threads {
				if !seen[t.ID] {
					seen[t.ID] = true
					allThreads = append(allThreads, t)
				}
			}

			if delayBetween > 0 {
				time.Sleep(delayBetween)
			}
		}
	}

	if answered == 0 && blockedErr != nil {
		return nil, blockedErr // Reddit refused some searches and none worked: report the block, so the fallback runs
	}
	if answered == 0 && firstErr != nil {
		return nil, firstErr // every search failed: say why rather than "nothing found"
	}
	return allThreads, nil
}

// redditFetchThreadDetails fetches full thread details (with comments) for a list of threads.
// Uses delays to avoid rate limiting.
func redditFetchThreadDetails(threads []RedditThread, maxThreads int, delayBetween time.Duration, statusFn func(string)) []RedditThread {
	if maxThreads <= 0 || maxThreads > len(threads) {
		maxThreads = len(threads)
	}
	if maxThreads > 50 {
		maxThreads = 50
	}

	result := make([]RedditThread, 0, maxThreads)
	for i := 0; i < maxThreads; i++ {
		if statusFn != nil {
			statusFn(fmt.Sprintf("Fetching thread details (%d/%d): %s", i+1, maxThreads, truncate(threads[i].Title, 60)))
		}

		full, err := redditFetchThread(threads[i].Permalink)
		if err != nil {
			log.Printf("Reddit thread fetch error (%s): %v", threads[i].Permalink, err)
			if errors.Is(err, errRedditBlocked) {
				// Reddit is refusing thread pages; keep what the search returned for the rest.
				if statusFn != nil {
					statusFn("Reddit refused to send thread details; continuing with what the search returned")
				}
				return append(result, threads[i:maxThreads]...)
			}
			// Keep the search-result version without comments
			result = append(result, threads[i])
		} else {
			result = append(result, *full)
		}

		if delayBetween > 0 && i < maxThreads-1 {
			time.Sleep(delayBetween)
		}
	}
	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ── Reddit cache ─────────────────────────────────────────────────────

func redditCacheKey(prefix, subreddit, query, timeFilter string) string {
	return fmt.Sprintf("reddit:%s:%s:%s:%s", prefix, subreddit, query, timeFilter)
}

func cachedRedditSearch(mongoDB *MongoDB, subreddit, query, timeFilter string, limit int) ([]RedditThread, bool, error) {
	key := redditCacheKey("search", subreddit, query, timeFilter)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cached struct {
		Data string    `bson:"data"`
		At   time.Time `bson:"cachedAt"`
	}

	err := mongoDB.RedditCache().FindOne(ctx, map[string]string{"cacheKey": key}).Decode(&cached)
	if err == nil && time.Since(cached.At) < redditCacheTTL {
		var threads []RedditThread
		if json.Unmarshal([]byte(cached.Data), &threads) == nil {
			return threads, true, nil
		}
	}

	// Cache miss — fetch fresh
	threads, err := redditSearch(subreddit, query, timeFilter, limit)
	if err != nil {
		return nil, false, err
	}

	// Save to cache
	data, _ := json.Marshal(threads)
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer saveCancel()
	mongoDB.RedditCache().ReplaceOne(saveCtx,
		map[string]string{"cacheKey": key},
		map[string]any{
			"cacheKey":  key,
			"data":      string(data),
			"cachedAt":  time.Now(),
			"expiresAt": time.Now().Add(redditCacheTTL),
		},
		replaceUpsert,
	)

	return threads, false, nil
}

// redditHTTPClient returns an http.Client suitable for Reddit requests.
// Uses WARP proxy if available, otherwise direct.
func redditHTTPClient() *http.Client {
	if wc := warpClient(); wc != nil {
		return wc
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		},
	}
}

var replaceUpsert = &options.ReplaceOptions{}

func init() {
	t := true
	replaceUpsert.Upsert = &t
}
