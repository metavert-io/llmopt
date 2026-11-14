package main

// Reddit threads through OpenAI's web search. Reddit refuses the anonymous
// search requests the Reddit tab makes (errRedditBlocked), from servers and home
// connections alike, and Anthropic's web search can't reach reddit.com at all.
// OpenAI's web search, limited to reddit.com, still finds threads, with enough
// of each (a summary and some reply excerpts) for the analysis. Only threads
// whose URL is among the search's own sources are kept, so a URL the model made
// up can't get in.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// redditWebSearchSource marks a thread found through web search: no upvotes,
	// comment counts or date, and its text is a summary with reply excerpts.
	redditWebSearchSource = "web_search"
	// redditWebSearchBatch is how many topics one request searches.
	redditWebSearchBatch = 4
	// redditWebSearchParallel is how many requests run at once.
	redditWebSearchParallel = 3
)

var redditThreadURLRe = regexp.MustCompile(`^https?://(?:www\.|old\.|new\.|np\.)?reddit\.com(/r/([A-Za-z0-9_]+)/comments/([a-z0-9]+)(?:/[^?#\s]*)?)`)

// parseRedditThreadURL returns a thread URL's permalink path, subreddit and ID.
func parseRedditThreadURL(u string) (permalink, subreddit, id string, ok bool) {
	m := redditThreadURLRe.FindStringSubmatch(strings.TrimSpace(u))
	if m == nil {
		return "", "", "", false
	}
	permalink = m[1]
	if !strings.HasSuffix(permalink, "/") {
		permalink += "/"
	}
	return permalink, m[2], m[3], true
}

// redditSearchModel picks the model for the searches: the newest GPT-5 "mini"
// the key lists (quick, cheap, and it can search the web), else the best model.
func redditSearchModel(models []ModelDef) string {
	for _, m := range models { // ranked best first
		id := strings.ToLower(m.ID)
		if strings.HasPrefix(id, "gpt-5") && strings.HasSuffix(id, "-mini") {
			return m.ID
		}
	}
	if len(models) > 0 && models[0].ID != openaiChatGPTAlias {
		return models[0].ID
	}
	return "gpt-5-mini"
}

func redditTimeHint(timeFilter string) string {
	switch timeFilter {
	case "day":
		return " Prefer threads from the past day."
	case "week":
		return " Prefer threads from the past week."
	case "month":
		return " Prefer threads from the past month."
	case "year":
		return " Prefer threads from the past year."
	}
	return ""
}

func redditWebSearchPrompt(topics, subreddits []string, timeFilter string) string {
	quoted := make([]string, len(topics))
	for i, t := range topics {
		quoted[i] = fmt.Sprintf("%q", t)
	}
	where := ""
	if len(subreddits) > 0 {
		subs := make([]string, len(subreddits))
		for i, s := range subreddits {
			subs[i] = "r/" + s
		}
		where = " Look in particular in " + strings.Join(subs, ", ") + ", and anywhere else on Reddit."
	}
	return fmt.Sprintf(`Find Reddit threads where people discuss each of these topics: %s. Search Reddit separately for each topic.%s%s

Then reply with ONLY a JSON array (no prose, no code fences) of the distinct threads you found, at most 20, each:
{"url": the thread's reddit.com/r/<subreddit>/comments/<id>/... URL exactly as in your search results,
 "title": the thread's title,
 "subreddit": its name without r/,
 "summary": 1-3 sentences on what the post asks or says and what the replies conclude,
 "comment_excerpts": up to 4 short quotes from replies exactly as they appear in the results (empty if none are visible),
 "mentions": brand, product or service names discussed in the thread}
Only include threads that appeared in your search results. If you found none, reply [].`, strings.Join(quoted, ", "), where, redditTimeHint(timeFilter))
}

type webSearchThread struct {
	URL             string   `json:"url"`
	Title           string   `json:"title"`
	Subreddit       string   `json:"subreddit"`
	Summary         string   `json:"summary"`
	CommentExcerpts []string `json:"comment_excerpts"`
	Mentions        []string `json:"mentions"`
}

// openaiRedditSearch runs one OpenAI web search request for some topics and
// returns the threads it found that are among its search sources.
func openaiRedditSearch(ctx context.Context, apiKey, model string, topics, subreddits []string, timeFilter string) ([]RedditThread, error) {
	body, _ := json.Marshal(map[string]any{
		"model":             model,
		"max_output_tokens": openaiOutputBudget(model, 8000),
		"tools": []map[string]any{{
			"type":    "web_search",
			"filters": map[string]any{"allowed_domains": []string{"reddit.com"}},
		}},
		"include": []string{"web_search_call.action.sources"},
		"input":   []map[string]any{{"role": "user", "content": redditWebSearchPrompt(topics, subreddits, timeFilter)}},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", openaiAPIBase+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, &ProviderHTTPError{Provider: "OpenAI", Status: resp.StatusCode, Body: truncateStr(string(respBody), 500)}
	}

	var result struct {
		Output []struct {
			Type   string `json:"type"`
			Action struct {
				Sources []struct {
					URL string `json:"url"`
				} `json:"sources"`
			} `json:"action"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("reading OpenAI's answer: %w", err)
	}
	sourced := map[string]bool{}
	var text strings.Builder
	for _, item := range result.Output {
		switch item.Type {
		case "web_search_call":
			for _, s := range item.Action.Sources {
				if _, _, id, ok := parseRedditThreadURL(s.URL); ok {
					sourced[id] = true
				}
			}
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					text.WriteString(c.Text)
				}
			}
		}
	}

	raw := strings.TrimSpace(stripJSONFencing(text.String()))
	if i, j := strings.Index(raw, "["), strings.LastIndex(raw, "]"); i >= 0 && j > i {
		raw = raw[i : j+1]
	}
	var found []webSearchThread
	if err := json.Unmarshal([]byte(raw), &found); err != nil {
		return nil, fmt.Errorf("OpenAI's answer wasn't a list of threads: %w", err)
	}

	var threads []RedditThread
	dropped := 0
	for _, f := range found {
		permalink, sub, id, ok := parseRedditThreadURL(f.URL)
		if !ok || !sourced[id] {
			dropped++
			continue
		}
		t := RedditThread{
			ID:        id,
			Subreddit: sub,
			Title:     strings.TrimSpace(f.Title),
			SelfText:  strings.TrimSpace(f.Summary),
			URL:       "https://www.reddit.com" + permalink,
			Permalink: permalink,
			Source:    redditWebSearchSource,
		}
		if len(f.Mentions) > 0 {
			t.SelfText += "\nNames discussed: " + strings.Join(f.Mentions, ", ")
		}
		for _, e := range f.CommentExcerpts {
			if e = strings.TrimSpace(e); e != "" {
				t.TopComments = append(t.TopComments, RedditComment{Body: e})
			}
		}
		t.IsSelfPost = true
		threads = append(threads, t)
	}
	if dropped > 0 {
		log.Printf("Reddit web search: dropped %d of %d threads not among the search's sources", dropped, len(found))
	}
	return threads, nil
}

// redditDiscoverViaWebSearch searches the topics in batches, a few requests at a
// time, and returns the distinct threads found. Subreddits the user chose are a
// hint (Reddit's own search also always covers all of Reddit). It fails only
// when every request does.
func redditDiscoverViaWebSearch(ctx context.Context, apiKey, model string, topics, subreddits []string, timeFilter string, statusFn func(string)) ([]RedditThread, error) {
	var batches [][]string
	for i := 0; i < len(topics); i += redditWebSearchBatch {
		end := i + redditWebSearchBatch
		if end > len(topics) {
			end = len(topics)
		}
		batches = append(batches, topics[i:end])
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		seen     = map[string]bool{}
		threads  []RedditThread
		failures int
		firstErr error
		done     int
	)
	sem := make(chan struct{}, redditWebSearchParallel)
	for _, b := range batches {
		wg.Add(1)
		go func(b []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			got, err := openaiRedditSearch(ctx, apiKey, model, b, subreddits, timeFilter)
			mu.Lock()
			defer mu.Unlock()
			done++
			if err != nil {
				log.Printf("Reddit web search (%s) failed: %v", strings.Join(b, ", "), err)
				failures++
				if firstErr == nil {
					firstErr = err
				}
			}
			for _, t := range got {
				if !seen[t.ID] {
					seen[t.ID] = true
					threads = append(threads, t)
				}
			}
			if statusFn != nil {
				statusFn(fmt.Sprintf("Searched Reddit through OpenAI (%d/%d): %d threads so far", done, len(batches), len(threads)))
			}
		}(b)
	}
	wg.Wait()
	if failures == len(batches) && firstErr != nil {
		return nil, firstErr
	}
	return threads, nil
}

// redditDetailsForAnalysis fetches full content for threads from Reddit's own
// search; threads found through web search keep what the search gave.
func redditDetailsForAnalysis(threads []RedditThread, statusFn func(string)) []RedditThread {
	var fetch, ready []RedditThread
	for _, t := range threads {
		if t.Source == redditWebSearchSource {
			ready = append(ready, t)
		} else {
			fetch = append(fetch, t)
		}
	}
	if len(fetch) == 0 {
		return ready
	}
	return append(redditFetchThreadDetails(fetch, len(fetch), 2*time.Second, statusFn), ready...)
}
