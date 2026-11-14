package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseRedditThreadURL(t *testing.T) {
	cases := []struct {
		url, permalink, sub, id string
		ok                      bool
	}{
		{"https://www.reddit.com/r/examplesub/comments/abc123x/some_title/", "/r/examplesub/comments/abc123x/some_title/", "examplesub", "abc123x", true},
		{"https://old.reddit.com/r/Another_Sub/comments/def456y", "/r/Another_Sub/comments/def456y/", "Another_Sub", "def456y", true},
		{"https://reddit.com/r/x/comments/abc123/t/?utm_source=share", "/r/x/comments/abc123/t/", "x", "abc123", true},
		{"https://www.reddit.com/r/examplesub/", "", "", "", false},
		{"https://example.com/r/x/comments/abc/", "", "", "", false},
	}
	for _, c := range cases {
		p, s, id, ok := parseRedditThreadURL(c.url)
		if ok != c.ok || p != c.permalink || s != c.sub || id != c.id {
			t.Errorf("%s: got %q %q %q %v", c.url, p, s, id, ok)
		}
	}
}

func TestRedditSearchModel(t *testing.T) {
	ranked := []ModelDef{{ID: "gpt-6.1-terra"}, {ID: "gpt-5.5"}, {ID: "gpt-5.4-mini"}, {ID: "gpt-5-mini"}, {ID: "chat-latest"}}
	if m := redditSearchModel(ranked); m != "gpt-5.4-mini" {
		t.Errorf("got %q", m)
	}
	if m := redditSearchModel([]ModelDef{{ID: "gpt-5.5"}}); m != "gpt-5.5" {
		t.Errorf("no mini: got %q", m)
	}
	if m := redditSearchModel(nil); m != "gpt-5-mini" {
		t.Errorf("no list: got %q", m)
	}
}

// openaiRedditServer answers the models list and a web search whose answer
// lists three threads, one of them not among the search's sources.
func openaiRedditServer(t *testing.T, searches *int32, bodies chan<- map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1/models") {
			w.Write([]byte(`{"data":[{"id":"gpt-5.5"},{"id":"gpt-5.4-mini"}]}`))
			return
		}
		if searches != nil {
			atomic.AddInt32(searches, 1)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		if bodies != nil {
			select {
			case bodies <- body:
			default:
			}
		}
		answer := "```json\n" + `[
 {"url":"https://www.reddit.com/r/examplesub/comments/aaa111/example_health_review/","title":"Example Health review","subreddit":"examplesub","summary":"Asks whether the service is worth it.","comment_excerpts":["It worked for me.",""],"mentions":["Example Health"]},
 {"url":"https://www.reddit.com/r/othersub/comments/bbb222/doctor_or_online/","title":"Doctor or online?","subreddit":"othersub","summary":"Doctor versus online services.","comment_excerpts":[],"mentions":[]},
 {"url":"https://www.reddit.com/r/made_up/comments/zzz999/not_in_sources/","title":"Invented","subreddit":"made_up","summary":"x","comment_excerpts":[],"mentions":[]}
]` + "\n```"
		out := map[string]any{"output": []any{
			map[string]any{"type": "web_search_call", "action": map[string]any{"type": "search", "sources": []any{
				map[string]any{"type": "url", "url": "https://www.reddit.com/r/examplesub/comments/aaa111/example_health_review/"},
				map[string]any{"type": "url", "url": "https://www.reddit.com/r/othersub/comments/bbb222/doctor_or_online/"},
			}}},
			map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": answer}}},
		}}
		json.NewEncoder(w).Encode(out)
	}))
}

func TestOpenAIRedditSearch_KeepsOnlySourcedThreads(t *testing.T) {
	bodies := make(chan map[string]any, 1)
	server := openaiRedditServer(t, nil, bodies)
	defer server.Close()
	setVar(t, &openaiAPIBase, server.URL)

	threads, err := openaiRedditSearch(t.Context(), "k", "gpt-5.4-mini", []string{"Example Health", "online prescriptions"}, "year")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 2 {
		t.Fatalf("got %d threads, want 2 (the invented URL dropped): %+v", len(threads), threads)
	}
	a := threads[0]
	if a.ID != "aaa111" || a.Subreddit != "examplesub" || a.Source != redditWebSearchSource ||
		a.Permalink != "/r/examplesub/comments/aaa111/example_health_review/" ||
		!strings.Contains(a.SelfText, "Names discussed: Example Health") || len(a.TopComments) != 1 {
		t.Errorf("thread a = %+v", a)
	}
	body := <-bodies
	tools, _ := json.Marshal(body["tools"])
	if !strings.Contains(string(tools), `"allowed_domains":["reddit.com"]`) || !strings.Contains(string(tools), `"web_search"`) {
		t.Errorf("tools = %s", tools)
	}
	if inc, _ := json.Marshal(body["include"]); !strings.Contains(string(inc), "web_search_call.action.sources") {
		t.Errorf("include = %s", inc)
	}
	if in, _ := json.Marshal(body["input"]); !strings.Contains(string(in), `\"Example Health\"`) || !strings.Contains(string(in), "past year") {
		t.Errorf("input = %s", in)
	}
}

func blockedRedditServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("<html>You've been blocked by network security.</html>"))
	}))
	withTestRedditBase(t, server)
	setVar(t, &warpState.client, nil)
	setVar(t, &warpState.checkedAt, time.Now()) // don't probe for a local WARP proxy
	return server
}

// When Reddit blocks the search, discovery finds the threads through OpenAI's
// web search and sends their text along for the analysis.
func TestRedditDiscover_FallsBackToWebSearch(t *testing.T) {
	db := testMongoDB(t)
	insertTestTenantKey(t, db, "openai")
	defer blockedRedditServer(t).Close()
	var searches int32
	openai := openaiRedditServer(t, &searches, nil)
	defer openai.Close()
	setVar(t, &openaiAPIBase, openai.URL)

	handler := handleRedditDiscover(db, testEncKey(), "fallback-key", true)
	w := httptest.NewRecorder()
	terms := []string{"a", "b", "c", "d", "e", "f"} // two batches
	handler(w, testRequest(t, "POST", "/api/reddit/discover", map[string]any{"domain": "example.com", "search_terms": terms}))
	events := parseSSEResponse(t, w.Body.Bytes())
	done := findSSEEvent(events, "done")
	if done == nil {
		t.Fatalf("no done event:\n%s", statusMessages(events))
	}
	var payload struct {
		Threads []struct {
			ID          string          `json:"id"`
			Source      string          `json:"source"`
			SelfText    string          `json:"self_text"`
			TopComments []RedditComment `json:"top_comments"`
		} `json:"threads"`
	}
	json.Unmarshal(done.Data, &payload)
	if len(payload.Threads) != 2 || payload.Threads[0].Source != "web_search" || payload.Threads[0].SelfText == "" {
		t.Errorf("threads = %+v", payload.Threads)
	}
	if c := atomic.LoadInt32(&searches); c != 2 {
		t.Errorf("%d OpenAI searches, want 2 (batches of %d)", c, redditWebSearchBatch)
	}
	if msgs := statusMessages(events); !strings.Contains(msgs, "through OpenAI's web search (gpt-5.4-mini)") {
		t.Errorf("no note about the fallback:\n%s", msgs)
	}
}

func TestRedditDiscover_BlockedWithoutOpenAIKey(t *testing.T) {
	db := testMongoDB(t)
	defer blockedRedditServer(t).Close()

	handler := handleRedditDiscover(db, testEncKey(), "fallback-key", true)
	w := httptest.NewRecorder()
	handler(w, testRequest(t, "POST", "/api/reddit/discover", map[string]any{"domain": "example.com", "search_terms": []string{"a", "b"}}))
	events := parseSSEResponse(t, w.Body.Bytes())
	e := findSSEEvent(events, "error")
	if e == nil || !strings.Contains(string(e.Data), "Add an OpenAI API key") {
		t.Errorf("events:\n%s", statusMessages(events))
	}
}

// Threads found through web search aren't fetched from Reddit again.
func TestRedditDetailsForAnalysis_SkipsWebSearchThreads(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(`[{"data":{"children":[{"data":{"id":"d1","subreddit":"x","title":"Direct","permalink":"/r/x/comments/d1/direct/","score":5}}]}},{"data":{"children":[]}}]`))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	in := []RedditThread{
		{ID: "d1", Permalink: "/r/x/comments/d1/direct/"},
		{ID: "w1", Permalink: "/r/x/comments/w1/web/", Source: redditWebSearchSource, SelfText: "summary"},
	}
	out := redditDetailsForAnalysis(in, nil)
	if len(out) != 2 || atomic.LoadInt32(&calls) != 1 {
		t.Errorf("got %d threads with %d fetches", len(out), calls)
	}
	for _, th := range out {
		if th.ID == "w1" && th.SelfText != "summary" {
			t.Errorf("web-search thread changed: %+v", th)
		}
	}
}

func TestBuildRedditAuthorityPrompt_WebSearchThread(t *testing.T) {
	p := buildRedditAuthorityPrompt("example.com", []RedditThread{{
		ID: "w1", Subreddit: "x", Title: "T", Permalink: "/r/x/comments/w1/t/", Source: redditWebSearchSource,
		SelfText: "A summary.", TopComments: []RedditComment{{Body: "A reply."}},
	}}, nil, nil, BrandContextInfo{})
	for _, want := range []string{"Found through web search", "Summary of the thread (from web search)", "Reply excerpts (from web search)", "- A reply."} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "Score: 0") || strings.Contains(p, "[0 pts]") {
		t.Error("prompt reports unknown engagement as zero")
	}
}
