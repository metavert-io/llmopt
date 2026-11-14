package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeSubreddit(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"r_prefix", "r/golang", "golang"},
		{"slash_r_prefix", "/r/golang", "golang"},
		{"bare_name", "golang", "golang"},
		{"reddit_url", "https://www.reddit.com/r/golang", "golang"},
		{"reddit_url_trailing_slash", "https://www.reddit.com/r/golang/", "golang"},
		{"empty", "", ""},
		{"spaces", "invalid name", ""},
		{"just_r_slash", "r/", ""},
		{"with_underscores", "r/Ask_Reddit", "Ask_Reddit"},
		{"whitespace", "  r/golang  ", "golang"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSubreddit(tt.input)
			if got != tt.want {
				t.Errorf("normalizeSubreddit(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"very_short_max", "hello", 3, "..."},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestRedditCacheKey(t *testing.T) {
	key := redditCacheKey("search", "golang", "best practices", "year")
	want := "reddit:search:golang:best practices:year"
	if key != want {
		t.Errorf("redditCacheKey = %q, want %q", key, want)
	}
}

func TestRedditCacheKey_EmptyParts(t *testing.T) {
	key := redditCacheKey("thread", "", "", "")
	want := "reddit:thread:::"
	if key != want {
		t.Errorf("redditCacheKey = %q, want %q", key, want)
	}
}

// ── redditDiscoverThreads ─────────────────────────────────────────────────────

func TestRedditDiscoverThreads_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_search.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	var statusMessages []string
	statusFn := func(msg string) { statusMessages = append(statusMessages, msg) }

	threads, err := redditDiscoverThreads(
		[]string{"golang", "programming"},
		[]string{"unit testing"},
		"year",
		10,
		0, // no delay
		statusFn,
	)
	if err != nil {
		t.Fatalf("redditDiscoverThreads failed: %v", err)
	}
	// With 2 subreddits x 1 term = 2 searches, fixture has 2 threads each
	// But deduplication means we might get < 4 threads
	if len(threads) == 0 {
		t.Error("expected at least 1 thread")
	}
	if len(statusMessages) == 0 {
		t.Error("expected status messages to be called")
	}
}

func TestRedditDiscoverThreads_AllSubreddits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_search.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// Empty subreddits = search all of Reddit
	threads, err := redditDiscoverThreads(nil, []string{"test brand"}, "month", 5, 0, nil)
	if err != nil {
		t.Fatalf("redditDiscoverThreads (all) failed: %v", err)
	}
	if len(threads) == 0 {
		t.Error("expected threads from all-reddit search")
	}
}

func TestRedditDiscoverThreads_SearchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "bad" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"data":{"children":[]}}`))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// One failed search is non-fatal when another search answered.
	threads, err := redditDiscoverThreads(
		[]string{"golang"},
		[]string{"bad", "good"},
		"year", 5, 0, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(threads) != 0 {
		t.Errorf("expected empty threads, got %d", len(threads))
	}
	// When every search fails, the error comes back (see TestRedditDiscover_AllFailedReturnsError).
}

func TestRedditDiscoverThreads_Deduplication(t *testing.T) {
	// Both searches return the same fixture (same thread IDs)
	// Threads should be deduplicated
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_search.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// Multiple search terms → same threads returned → dedup
	threads, err := redditDiscoverThreads(
		[]string{"golang"},
		[]string{"term1", "term2"},
		"year", 25, 0, nil,
	)
	if err != nil {
		t.Fatalf("redditDiscoverThreads failed: %v", err)
	}
	// The fixture has 2 threads (abc123 and def456)
	// Even though we search twice, they should be deduped
	ids := map[string]bool{}
	for _, th := range threads {
		ids[th.ID] = true
	}
	if len(ids) != len(threads) {
		t.Error("expected no duplicate thread IDs")
	}
}

func TestRedditDiscoverThreads_DefaultLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_search.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// maxPerQuery <= 0 should use default of 15
	threads, err := redditDiscoverThreads([]string{"golang"}, []string{"test"}, "year", 0, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = threads // just verifies no panic
}

// ── redditFetchThreadDetails ──────────────────────────────────────────────────

func TestRedditFetchThreadDetails_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_thread.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	threads := []RedditThread{
		{ID: "abc123", Title: "Thread 1", Permalink: "/r/golang/comments/abc123/thread_1/"},
		{ID: "def456", Title: "Thread 2", Permalink: "/r/golang/comments/def456/thread_2/"},
	}

	var statusMessages []string
	statusFn := func(msg string) { statusMessages = append(statusMessages, msg) }

	result := redditFetchThreadDetails(threads, 2, 0, statusFn)
	if len(result) != 2 {
		t.Errorf("expected 2 threads, got %d", len(result))
	}
	if len(statusMessages) == 0 {
		t.Error("expected status messages")
	}
}

func TestRedditFetchThreadDetails_FetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	threads := []RedditThread{
		{ID: "xyz", Title: "Error Thread", Permalink: "/r/test/comments/xyz/error/"},
	}

	result := redditFetchThreadDetails(threads, 1, 0, nil)
	// On fetch error, keeps original thread without full details
	if len(result) != 1 {
		t.Errorf("expected 1 result (fallback), got %d", len(result))
	}
	if result[0].ID != "xyz" {
		t.Errorf("expected original thread ID, got %q", result[0].ID)
	}
}

func TestRedditFetchThreadDetails_MaxThreadsCapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_thread.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// Create 60 threads but max should be capped at 50
	threads := make([]RedditThread, 60)
	for i := range threads {
		threads[i] = RedditThread{
			ID:        fmt.Sprintf("id%d", i),
			Title:     fmt.Sprintf("Thread %d", i),
			Permalink: fmt.Sprintf("/r/test/comments/id%d/thread_%d/", i, i),
		}
	}

	result := redditFetchThreadDetails(threads, 0, 0, nil)
	// 0 → len(threads) = 60, but capped at 50
	if len(result) > 50 {
		t.Errorf("expected max 50 threads, got %d", len(result))
	}
}

func TestRedditFetchThreadDetails_Empty(t *testing.T) {
	result := redditFetchThreadDetails([]RedditThread{}, 10, 0, nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// ── isProxyError ─────────────────────────────────────────────────────────────

func TestIsProxyError_SOCKS(t *testing.T) {
	err := fmt.Errorf("SOCKS5 connection refused")
	if !isProxyError(err) {
		t.Error("expected SOCKS error to be proxy error")
	}
}

func TestIsProxyError_Proxy(t *testing.T) {
	err := fmt.Errorf("proxy server unreachable")
	if !isProxyError(err) {
		t.Error("expected proxy error to be proxy error")
	}
}

func TestIsProxyError_ERR_PROXY(t *testing.T) {
	err := fmt.Errorf("ERR_PROXY_CONNECTION_FAILED")
	if !isProxyError(err) {
		t.Error("expected ERR_PROXY error to be proxy error")
	}
}

func TestIsProxyError_Other(t *testing.T) {
	err := fmt.Errorf("connection refused by server")
	if isProxyError(err) {
		t.Error("expected regular error to not be proxy error")
	}
}

// ── pluralS ──────────────────────────────────────────────────────────────────

func TestPluralS_One(t *testing.T) {
	if pluralS(1) != "" {
		t.Errorf("expected '' for 1, got %q", pluralS(1))
	}
}

func TestPluralS_Zero(t *testing.T) {
	if pluralS(0) != "s" {
		t.Errorf("expected 's' for 0, got %q", pluralS(0))
	}
}

func TestPluralS_Many(t *testing.T) {
	if pluralS(5) != "s" {
		t.Errorf("expected 's' for 5, got %q", pluralS(5))
	}
}

// ── redditHTTPGet - 429/403 path ──────────────────────────────────────────────

func TestRedditHTTPGet_RateLimited_NoWARP(t *testing.T) {
	// Mock server returns 429 (no WARP configured in test env)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// Replace redditClient with test server client
	origClient := redditClient
	redditClient = server.Client()
	t.Cleanup(func() { redditClient = origClient })

	url := server.URL + "/search.json?q=test"
	_, err := redditHTTPGet(url)
	if err == nil {
		t.Error("expected error for 429 with no WARP fallback")
	}
}

// ── redditDiscoverThreads with delay (smoke test) ─────────────────────────────

func TestRedditDiscoverThreads_WithStatusFn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_search.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	callCount := 0
	statusFn := func(msg string) {
		callCount++
		if msg == "" {
			t.Error("expected non-empty status message")
		}
	}

	_, err := redditDiscoverThreads(
		[]string{"sub1"},
		[]string{"term1", "term2"},
		"year", 5,
		time.Millisecond, // tiny delay
		statusFn,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 status calls (one per term), got %d", callCount)
	}
}
