package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Reddit's "blocked by network security" 403 stops discovery after two refused
// searches and comes back as errRedditBlocked, not as "no threads found".
func TestRedditDiscover_BlockedStopsEarly(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("<html>You've been blocked by network security.</html>"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)
	setVar(t, &warpState.client, nil) // no WARP fallback in tests

	_, err := redditDiscoverThreads([]string{"all"}, []string{"a", "b", "c", "d", "e"}, "year", 5, 0, nil)
	if !errors.Is(err, errRedditBlocked) {
		t.Fatalf("err = %v, want errRedditBlocked", err)
	}
	if c := atomic.LoadInt32(&calls); c != 2 {
		t.Errorf("made %d searches after Reddit refused; want 2", c)
	}
}

// When every search fails for another reason, discovery returns that error.
func TestRedditDiscover_AllFailedReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	_, err := redditDiscoverThreads(nil, []string{"a", "b"}, "year", 5, 0, nil)
	if err == nil || errors.Is(err, errRedditBlocked) || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err = %v", err)
	}
}

// A refused thread page keeps the search results for that thread and the rest,
// without asking Reddit for each one.
func TestRedditFetchThreadDetails_BlockedKeepsSearchResults(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	withTestRedditBase(t, server)
	setVar(t, &warpState.client, nil)

	in := []RedditThread{{ID: "1", Permalink: "/r/x/comments/1/a/"}, {ID: "2", Permalink: "/r/x/comments/2/b/"}, {ID: "3", Permalink: "/r/x/comments/3/c/"}}
	var notes []string
	out := redditFetchThreadDetails(in, 3, 0, func(m string) { notes = append(notes, m) })
	if len(out) != 3 || out[2].ID != "3" {
		t.Errorf("got %+v", out)
	}
	if c := atomic.LoadInt32(&calls); c != 1 {
		t.Errorf("asked Reddit %d times after it refused", c)
	}
	if !strings.Contains(strings.Join(notes, "\n"), "Reddit refused to send thread details") {
		t.Errorf("notes: %v", notes)
	}
}
