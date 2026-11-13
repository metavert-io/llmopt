package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"

	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/proxy"
)

// ytHTTPClient is a shared HTTP client for YouTube Data API v3 calls.
var ytHTTPClient = &http.Client{Timeout: 30 * time.Second}

// youtubeAPIBase is the base URL for YouTube Data API v3 (var for test swapping).
var youtubeAPIBase = "https://www.googleapis.com/youtube/v3"

// innertubePlayerURL is the InnerTube player API endpoint (var for test swapping).
var innertubePlayerURL = "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"

const (
	cacheTTL        = 14 * 24 * time.Hour  // 14 days for API search/metadata results
	transcriptTTL   = 365 * 24 * time.Hour // ~1 year for transcripts (essentially immutable)
	assessmentTTL   = 30 * 24 * time.Hour  // 30 days for video assessments (context-dependent)
)

// ── YouTube Data API v3 ─────────────────────────────────────────────────

// verifyYouTubeKey checks if a YouTube Data API v3 key is valid by making
// a lightweight search call. Returns "active", "invalid", or "error".
func verifyYouTubeKey(ctx context.Context, apiKey string) string {
	u := fmt.Sprintf("%s/search?part=id&q=test&type=video&maxResults=1&key=%s", youtubeAPIBase, apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "error"
	}
	resp, err := ytHTTPClient.Do(req)
	if err != nil {
		return "error"
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return "active"
	}
	if resp.StatusCode == 400 || resp.StatusCode == 403 {
		return "invalid"
	}
	return "error"
}

// youtubeSearch searches YouTube for videos matching a query.
func youtubeSearch(apiKey, query string, maxResults int) ([]YouTubeVideo, error) {
	u := fmt.Sprintf("%s/search?part=snippet&q=%s&type=video&maxResults=%d&key=%s",
		youtubeAPIBase, url.QueryEscape(query), maxResults, apiKey)

	resp, err := ytHTTPClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("youtube search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube search API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				ChannelID    string `json:"channelId"`
				Description  string `json:"description"`
				PublishedAt  string `json:"publishedAt"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("youtube search decode error: %w", err)
	}

	// Collect video IDs for batch detail fetch
	var videoIDs []string
	for _, item := range result.Items {
		videoIDs = append(videoIDs, item.ID.VideoID)
	}
	if len(videoIDs) == 0 {
		return nil, nil
	}

	// Fetch full details (statistics, etc.)
	return youtubeVideoDetails(apiKey, videoIDs)
}

// youtubeVideoDetails fetches detailed info for a batch of video IDs.
func youtubeVideoDetails(apiKey string, videoIDs []string) ([]YouTubeVideo, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	// YouTube API accepts up to 50 IDs per request
	var allVideos []YouTubeVideo
	for i := 0; i < len(videoIDs); i += 50 {
		end := i + 50
		if end > len(videoIDs) {
			end = len(videoIDs)
		}
		batch := videoIDs[i:end]

		u := fmt.Sprintf("%s/videos?part=snippet,statistics,contentDetails&id=%s&key=%s",
			youtubeAPIBase, url.QueryEscape(strings.Join(batch, ",")), apiKey)

		resp, err := ytHTTPClient.Get(u)
		if err != nil {
			return nil, fmt.Errorf("youtube video details request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("youtube video details API error (%d): %s", resp.StatusCode, string(body))
		}

		var result struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title        string   `json:"title"`
					ChannelTitle string   `json:"channelTitle"`
					ChannelID    string   `json:"channelId"`
					Description  string   `json:"description"`
					PublishedAt  string   `json:"publishedAt"`
					Tags         []string `json:"tags"`
				} `json:"snippet"`
				Statistics struct {
					ViewCount    string `json:"viewCount"`
					LikeCount    string `json:"likeCount"`
					CommentCount string `json:"commentCount"`
				} `json:"statistics"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("youtube video details decode error: %w", err)
		}

		for _, item := range result.Items {
			pub, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			views, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
			likes, _ := strconv.ParseInt(item.Statistics.LikeCount, 10, 64)
			comments, _ := strconv.ParseInt(item.Statistics.CommentCount, 10, 64)

			allVideos = append(allVideos, YouTubeVideo{
				VideoID:      item.ID,
				Title:        item.Snippet.Title,
				ChannelTitle: item.Snippet.ChannelTitle,
				ChannelID:    item.Snippet.ChannelID,
				Description:  item.Snippet.Description,
				PublishedAt:  pub,
				ViewCount:    views,
				LikeCount:    likes,
				CommentCount: comments,
				Duration:     item.ContentDetails.Duration,
				Tags:         item.Snippet.Tags,
			})
		}
	}

	return allVideos, nil
}

// youtubeChannelInfo fetches channel metadata.
func youtubeChannelInfo(apiKey, channelID string) (*YouTubeChannel, error) {
	u := fmt.Sprintf("%s/channels?part=snippet,statistics&id=%s&key=%s",
		youtubeAPIBase, url.QueryEscape(channelID), apiKey)

	resp, err := ytHTTPClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("youtube channel info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube channel info API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
			Statistics struct {
				SubscriberCount string `json:"subscriberCount"`
				VideoCount      string `json:"videoCount"`
				ViewCount       string `json:"viewCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("youtube channel info decode error: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}

	item := result.Items[0]
	subs, _ := strconv.ParseInt(item.Statistics.SubscriberCount, 10, 64)
	vids, _ := strconv.ParseInt(item.Statistics.VideoCount, 10, 64)
	views, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)

	return &YouTubeChannel{
		ChannelID:       item.ID,
		Title:           item.Snippet.Title,
		SubscriberCount: subs,
		VideoCount:      vids,
		ViewCount:       views,
	}, nil
}

// youtubeChannelVideos lists videos from a channel.
func youtubeChannelVideos(apiKey, channelID string, maxResults int) ([]YouTubeVideo, error) {
	u := fmt.Sprintf("%s/search?part=snippet&channelId=%s&type=video&maxResults=%d&order=date&key=%s",
		youtubeAPIBase, url.QueryEscape(channelID), maxResults, apiKey)

	resp, err := ytHTTPClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("youtube channel videos request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube channel videos API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("youtube channel videos decode error: %w", err)
	}

	var videoIDs []string
	for _, item := range result.Items {
		videoIDs = append(videoIDs, item.ID.VideoID)
	}
	if len(videoIDs) == 0 {
		return nil, nil
	}

	return youtubeVideoDetails(apiKey, videoIDs)
}

// ── Transcript Extraction ───────────────────────────────────────────────

var errNoCaptions = fmt.Errorf("no captions available")
var errBlocked = fmt.Errorf("blocked by YouTube")

var warpState struct {
	sync.Mutex
	client    *http.Client
	checkedAt time.Time
}

// warpRecheckInterval is how long a failed WARP check is remembered. WARP
// connects in the background at startup, so an early miss must not stick.
const warpRecheckInterval = 30 * time.Second

// warpClient returns an *http.Client that routes through the Cloudflare WARP SOCKS5 proxy
// at 127.0.0.1:40000 (set up in start-saas.sh on Fly.io). Returns nil if WARP is unavailable.
func warpClient() *http.Client {
	warpState.Lock()
	defer warpState.Unlock()
	if warpState.client != nil {
		return warpState.client
	}
	if !warpState.checkedAt.IsZero() && time.Since(warpState.checkedAt) < warpRecheckInterval {
		return nil
	}
	warpState.checkedAt = time.Now()

	const warpAddr = "127.0.0.1:40000"
	// Quick connectivity check
	conn, err := net.DialTimeout("tcp", warpAddr, 2*time.Second)
	if err != nil {
		log.Printf("WARP proxy not available at %s: %v", warpAddr, err)
		return nil
	}
	conn.Close()

	dialer, err := proxy.SOCKS5("tcp", warpAddr, nil, proxy.Direct)
	if err != nil {
		log.Printf("WARP SOCKS5 setup failed: %v", err)
		return nil
	}
	log.Printf("WARP proxy available at %s", warpAddr)
	warpState.client = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
	}
	return warpState.client
}

// preferredTranscriptMethod stores the name of the last method that successfully
// fetched a transcript. fetchTranscript reorders its method list to try this one
// first — YouTube blocks tend to apply uniformly, so once we find a working surface
// we should stay on it instead of paying latency for repeated dry runs.
var preferredTranscriptMethod atomic.Value // string

// fetchTranscript tries multiple methods in sequence to get a video's transcript.
// Each method targets a different YouTube API surface with different IP-trust behavior.
// Methods are tried in priority order; once one succeeds, its name is cached and
// floated to the top of the next call's list. WARP-proxied variants follow direct ones.
// Returns (transcript, methodName, error).
func fetchTranscript(videoID string) (string, string, error) {
	type method struct {
		name string
		fn   func(string) (string, error)
	}

	// Phase 1: Direct methods. Order = current expected reliability (most likely first).
	// yt-dlp goes first because it ships ongoing fixes for YouTube's bot-detection
	// (PO tokens, etc.) that pure-Go HTTP clients can't replicate. The InnerTube
	// methods follow as fast-path fallbacks for when yt-dlp itself is unavailable
	// or has a transient hiccup.
	methods := []method{
		{"yt-dlp", fetchTranscriptYtDlp},
		{"tv-embed", fetchTranscriptTVEmbed},
		{"mweb", fetchTranscriptMWeb},
		{"web", fetchTranscriptWeb},
		{"android", fetchTranscriptAndroid},
		{"scrape", fetchTranscriptWatchPage},
	}

	// Phase 2: WARP-proxied methods (if WARP is available), same priority order.
	if wc := warpClient(); wc != nil {
		methods = append(methods,
			method{"warp-yt-dlp", fetchTranscriptYtDlpWarp},
			method{"warp-tv-embed", makeWarpTVEmbed(wc)},
			method{"warp-mweb", makeWarpMWeb(wc)},
			method{"warp-web", makeWarpWeb(wc)},
			method{"warp-android", makeWarpAndroid(wc)},
			method{"warp-scrape", makeWarpScrape(wc)},
		)
	}

	// Float the most-recently-successful method to the front.
	if pref, _ := preferredTranscriptMethod.Load().(string); pref != "" {
		for i, m := range methods {
			if m.name == pref && i > 0 {
				methods[0], methods[i] = methods[i], methods[0]
				break
			}
		}
	}

	var lastErr error
	sawNoCaptions := false
	for _, m := range methods {
		transcript, err := m.fn(videoID)
		if err == nil && transcript != "" {
			preferredTranscriptMethod.Store(m.name)
			return transcript, m.name, nil
		}
		if errors.Is(err, errNoCaptions) {
			sawNoCaptions = true
			lastErr = errNoCaptions
			continue
		}
		if errors.Is(err, errBlocked) {
			log.Printf("Transcript [%s] %s: %v", videoID, m.name, err)
			lastErr = err
			continue // try next method — might work via different client or WARP
		}
		if err != nil {
			log.Printf("Transcript [%s] %s: %v", videoID, m.name, err)
			lastErr = err
			continue
		}
		lastErr = errNoCaptions
	}
	// If any method got playability OK with no captions, report that; otherwise report the last error
	if sawNoCaptions && lastErr != nil && errors.Is(lastErr, errBlocked) {
		return "", "", errNoCaptions // at least one method confirmed no captions
	}
	if lastErr == nil {
		lastErr = errNoCaptions
	}
	return "", "", lastErr
}

// innertubePlayerRequest makes an InnerTube player API call with the given client config.
// If httpClient is nil, a default 30s-timeout client is used.
func innertubePlayerRequest(videoID string, clientBody []byte, userAgent string, httpClient *http.Client) (map[string]any, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequest("POST",
		innertubePlayerURL,
		strings.NewReader(string(clientBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("innertube request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("%w: innertube rate limited (429)", errBlocked)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("innertube HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read player response: %w", err)
	}

	var playerResp map[string]any
	if err := json.Unmarshal(body, &playerResp); err != nil {
		return nil, fmt.Errorf("failed to parse player response: %w", err)
	}
	return playerResp, nil
}

// extractCaptionURL finds an English caption track URL from a player response.
func extractCaptionURL(videoID string, playerResp map[string]any) (string, error) {
	// Check playability — distinguish "blocked" from "no captions"
	if ps, ok := playerResp["playabilityStatus"].(map[string]any); ok {
		status, _ := ps["status"].(string)
		if status != "OK" {
			reason, _ := ps["reason"].(string)
			// These statuses indicate YouTube is blocking us, not that the video lacks captions
			switch status {
			case "LOGIN_REQUIRED", "ERROR", "UNPLAYABLE":
				return "", fmt.Errorf("%w: playability %s: %s", errBlocked, status, reason)
			default:
				return "", fmt.Errorf("%w: playability %s", errNoCaptions, status)
			}
		}
	}

	// Playability is OK — if captions section is missing entirely, YouTube may be
	// soft-blocking by stripping caption data. Return errNoCaptions since we can't
	// distinguish this from genuinely uncaptioned videos.
	captionsObj, ok := playerResp["captions"].(map[string]any)
	if !ok {
		return "", errNoCaptions
	}
	trackList, ok := captionsObj["playerCaptionsTracklistRenderer"].(map[string]any)
	if !ok {
		return "", errNoCaptions
	}
	tracks, ok := trackList["captionTracks"].([]any)
	if !ok || len(tracks) == 0 {
		return "", errNoCaptions
	}

	var captionURL string
	for _, t := range tracks {
		track, ok := t.(map[string]any)
		if !ok {
			continue
		}
		langCode, _ := track["languageCode"].(string)
		baseURL, _ := track["baseUrl"].(string)
		if baseURL == "" {
			continue
		}
		if captionURL == "" {
			captionURL = baseURL
		}
		if strings.HasPrefix(langCode, "en") {
			captionURL = baseURL
			break
		}
	}
	if captionURL == "" {
		return "", errNoCaptions
	}
	return captionURL, nil
}

// fetchCaptionXML fetches and parses a timedtext XML URL into plain text.
// If httpClient is nil, a default 30s-timeout client is used.
func fetchCaptionXML(captionURL, userAgent string, httpClient *http.Client) (string, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequest("GET", captionURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("timedtext fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read timedtext: %w", err)
	}

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("%w: timedtext rate limited (429)", errBlocked)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("timedtext HTTP %d (%d bytes)", resp.StatusCode, len(body))
	}
	if len(body) == 0 {
		return "", fmt.Errorf("%w: timedtext HTTP 200 but empty body", errBlocked)
	}

	return parseTimedTextXML(body)
}

// parseTimedTextXML parses YouTube caption XML (classic <text> or srv3 <body><p><s>) into plain text.
func parseTimedTextXML(data []byte) (string, error) {
	type SElement struct {
		Text string `xml:",chardata"`
	}
	type PElement struct {
		Text string     `xml:",chardata"`
		S    []SElement `xml:"s"`
	}
	type BodyElement struct {
		P []PElement `xml:"p"`
	}
	type TextElement struct {
		Text string `xml:",chardata"`
	}
	type TranscriptXML struct {
		XMLName xml.Name      `xml:""`
		Texts   []TextElement `xml:"text"`
		Body    BodyElement   `xml:"body"`
	}

	var transcript TranscriptXML
	if err := xml.Unmarshal(data, &transcript); err != nil {
		return "", fmt.Errorf("XML parse error: %w", err)
	}

	var parts []string
	if len(transcript.Body.P) > 0 {
		for _, p := range transcript.Body.P {
			if len(p.S) > 0 {
				for _, s := range p.S {
					if t := strings.TrimSpace(html.UnescapeString(s.Text)); t != "" {
						parts = append(parts, t)
					}
				}
			} else if t := strings.TrimSpace(html.UnescapeString(p.Text)); t != "" {
				parts = append(parts, t)
			}
		}
	}
	if len(parts) == 0 {
		for _, t := range transcript.Texts {
			if text := strings.TrimSpace(html.UnescapeString(t.Text)); text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.Join(parts, " "), nil
}

// fetchTranscriptAndroidWith fetches via ANDROID InnerTube using the given HTTP client.
func fetchTranscriptAndroidWith(videoID string, httpClient *http.Client) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":        "ANDROID",
				"clientVersion":     "19.09.37",
				"androidSdkVersion": 30,
				"hl":                "en",
				"gl":                "US",
			},
		},
		"videoId":        videoID,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	})
	ua := "com.google.android.youtube/19.09.37 (Linux; U; Android 11) gzip"

	playerResp, err := innertubePlayerRequest(videoID, body, ua, httpClient)
	if err != nil {
		return "", err
	}

	captionURL, err := extractCaptionURL(videoID, playerResp)
	if err != nil {
		return "", err
	}

	text, err := fetchCaptionXML(captionURL, ua, httpClient)
	if err != nil {
		return "", err
	}
	return text, nil
}

// fetchTranscriptWebWith fetches via WEB InnerTube using the given HTTP client.
func fetchTranscriptWebWith(videoID string, httpClient *http.Client) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20250312.04.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"videoId":        videoID,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	})
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"

	playerResp, err := innertubePlayerRequest(videoID, body, ua, httpClient)
	if err != nil {
		return "", err
	}

	captionURL, err := extractCaptionURL(videoID, playerResp)
	if err != nil {
		return "", err
	}

	text, err := fetchCaptionXML(captionURL, ua, httpClient)
	if err != nil {
		return "", err
	}
	return text, nil
}

// fetchTranscriptTVEmbedWith fetches via TVHTML5_SIMPLY_EMBEDDED_PLAYER InnerTube.
// The smart-TV embed surface tends to dodge bot-detection that gates the WEB client.
func fetchTranscriptTVEmbedWith(videoID string, httpClient *http.Client) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "TVHTML5_SIMPLY_EMBEDDED_PLAYER",
				"clientVersion": "2.0",
				"clientScreen":  "EMBED",
				"hl":            "en",
				"gl":            "US",
			},
			"thirdParty": map[string]any{
				"embedUrl": "https://www.youtube.com/",
			},
		},
		"videoId":        videoID,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	})
	ua := "Mozilla/5.0 (PlayStation; PlayStation 4/12.00) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15"

	playerResp, err := innertubePlayerRequest(videoID, body, ua, httpClient)
	if err != nil {
		return "", err
	}

	captionURL, err := extractCaptionURL(videoID, playerResp)
	if err != nil {
		return "", err
	}

	text, err := fetchCaptionXML(captionURL, ua, httpClient)
	if err != nil {
		return "", err
	}
	return text, nil
}

// fetchTranscriptMWebWith fetches via MWEB (mobile web) InnerTube using the given HTTP client.
// Mobile web tends to face lighter bot-detection than the desktop WEB client.
func fetchTranscriptMWebWith(videoID string, httpClient *http.Client) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "MWEB",
				"clientVersion": "2.20250311.03.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"videoId":        videoID,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	})
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1"

	playerResp, err := innertubePlayerRequest(videoID, body, ua, httpClient)
	if err != nil {
		return "", err
	}

	captionURL, err := extractCaptionURL(videoID, playerResp)
	if err != nil {
		return "", err
	}

	text, err := fetchCaptionXML(captionURL, ua, httpClient)
	if err != nil {
		return "", err
	}
	return text, nil
}

// fetchTranscriptScrapeWith fetches via watch page scraping using the given HTTP client.
func fetchTranscriptScrapeWith(videoID string, httpClient *http.Client) (string, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

	req, err := http.NewRequest("GET", "https://www.youtube.com/watch?v="+videoID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("watch page fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("watch page HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read watch page: %w", err)
	}

	pageStr := string(body)

	// Extract ytInitialPlayerResponse JSON from the page
	re := regexp.MustCompile(`var ytInitialPlayerResponse\s*=\s*(\{.+?\});`)
	match := re.FindStringSubmatch(pageStr)
	if len(match) < 2 {
		re2 := regexp.MustCompile(`ytInitialPlayerResponse"\s*:\s*(\{.+?\})\s*[,;]`)
		match = re2.FindStringSubmatch(pageStr)
	}
	if len(match) < 2 {
		return "", errNoCaptions
	}

	jsonStr := extractJSONObject(match[1])
	if jsonStr == "" {
		return "", errNoCaptions
	}

	var playerResp map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &playerResp); err != nil {
		return "", fmt.Errorf("failed to parse embedded player response: %w", err)
	}

	captionURL, err := extractCaptionURL(videoID, playerResp)
	if err != nil {
		return "", err
	}

	text, err := fetchCaptionXML(captionURL, ua, httpClient)
	if err != nil {
		return "", err
	}
	return text, nil
}

// fetchTranscriptYtDlpWith fetches subtitles via the yt-dlp subprocess.
// proxyURL is optional ("socks5://127.0.0.1:40000" routes via WARP; empty = direct).
// yt-dlp ships fast updates against YouTube bot-detection changes (PO tokens, client
// signing, etc.) that pure-Go HTTP clients can't keep up with — this is the durable
// path when InnerTube methods all return LOGIN_REQUIRED.
func fetchTranscriptYtDlpWith(videoID, proxyURL string) (string, error) {
	ytdlpPath, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", fmt.Errorf("yt-dlp not installed")
	}

	tmpDir, err := os.MkdirTemp("", "yt-transcript-*")
	if err != nil {
		return "", fmt.Errorf("mkdtemp failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	args := []string{
		"--skip-download",
		"--write-auto-subs",
		"--write-subs",
		"--sub-langs", "en.*,en",
		"--sub-format", "vtt",
		"--no-warnings",
		"--no-playlist",
		"--retries", "1",
		"--socket-timeout", "15",
		"--no-cache-dir",
		// EJS challenge-solver lib — auto-downloads on first use to solve YouTube's
		// JS challenges. Without this, deno alone can't decode the obfuscated player
		// signatures and every call falls back to LOGIN_REQUIRED.
		"--remote-components", "ejs:github",
		"-o", filepath.Join(tmpDir, "%(id)s"),
	}
	if proxyURL != "" {
		args = append(args, "--proxy", proxyURL)
	}
	args = append(args, "https://www.youtube.com/watch?v="+videoID)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrText := stderr.String()
		// Distinguish "no captions available" from real blocks/errors so the
		// outer fetchTranscript loop can stop trying when the video genuinely
		// has nothing to transcribe.
		if strings.Contains(stderrText, "no subtitles") ||
			strings.Contains(stderrText, "There are no subtitles") ||
			strings.Contains(stderrText, "Video unavailable") {
			return "", errNoCaptions
		}
		if strings.Contains(stderrText, "Sign in to confirm") ||
			strings.Contains(stderrText, "not a bot") {
			return "", fmt.Errorf("%w: yt-dlp: %s", errBlocked, truncStderr(stderrText))
		}
		return "", fmt.Errorf("yt-dlp failed: %v: %s", err, truncStderr(stderrText))
	}

	matches, _ := filepath.Glob(filepath.Join(tmpDir, "*.vtt"))
	if len(matches) == 0 {
		return "", errNoCaptions
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		return "", fmt.Errorf("read vtt: %w", err)
	}

	return parseVTT(string(data)), nil
}

// vttInlineMarker strips inline timing/styling tags like <00:00:01.520> and <c>.
var vttInlineMarker = regexp.MustCompile(`<[^>]*>`)

// parseVTT converts a WebVTT caption file to plain text — strips timestamps,
// metadata, inline markers, and deduplicates consecutive repeated lines
// (auto-generated captions repeat each phrase as the running window scrolls).
func parseVTT(s string) string {
	var lines []string
	var prev string
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if line == "WEBVTT" || strings.Contains(line, "-->") {
			continue
		}
		if strings.HasPrefix(line, "Kind:") ||
			strings.HasPrefix(line, "Language:") ||
			strings.HasPrefix(line, "NOTE") ||
			strings.HasPrefix(line, "STYLE") {
			continue
		}
		cleaned := strings.TrimSpace(vttInlineMarker.ReplaceAllString(line, ""))
		cleaned = html.UnescapeString(cleaned)
		if cleaned == "" || cleaned == prev {
			continue
		}
		lines = append(lines, cleaned)
		prev = cleaned
	}
	return strings.Join(lines, " ")
}

func truncStderr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// Direct method wrappers (nil = default HTTP client)
func fetchTranscriptAndroid(videoID string) (string, error) {
	return fetchTranscriptAndroidWith(videoID, nil)
}
func fetchTranscriptWeb(videoID string) (string, error) {
	return fetchTranscriptWebWith(videoID, nil)
}
func fetchTranscriptTVEmbed(videoID string) (string, error) {
	return fetchTranscriptTVEmbedWith(videoID, nil)
}
func fetchTranscriptMWeb(videoID string) (string, error) {
	return fetchTranscriptMWebWith(videoID, nil)
}
func fetchTranscriptWatchPage(videoID string) (string, error) {
	return fetchTranscriptScrapeWith(videoID, nil)
}
func fetchTranscriptYtDlp(videoID string) (string, error) {
	return fetchTranscriptYtDlpWith(videoID, "")
}
func fetchTranscriptYtDlpWarp(videoID string) (string, error) {
	return fetchTranscriptYtDlpWith(videoID, "socks5://127.0.0.1:40000")
}

// WARP-proxied method factories
func makeWarpAndroid(wc *http.Client) func(string) (string, error) {
	return func(videoID string) (string, error) { return fetchTranscriptAndroidWith(videoID, wc) }
}
func makeWarpWeb(wc *http.Client) func(string) (string, error) {
	return func(videoID string) (string, error) { return fetchTranscriptWebWith(videoID, wc) }
}
func makeWarpTVEmbed(wc *http.Client) func(string) (string, error) {
	return func(videoID string) (string, error) { return fetchTranscriptTVEmbedWith(videoID, wc) }
}
func makeWarpMWeb(wc *http.Client) func(string) (string, error) {
	return func(videoID string) (string, error) { return fetchTranscriptMWebWith(videoID, wc) }
}
func makeWarpScrape(wc *http.Client) func(string) (string, error) {
	return func(videoID string) (string, error) { return fetchTranscriptScrapeWith(videoID, wc) }
}

// extractJSONObject extracts a balanced JSON object starting from the first {.
func extractJSONObject(s string) string {
	if len(s) == 0 || s[0] != '{' {
		return ""
	}
	depth := 0
	inStr := false
	escaped := false
	for i, c := range s {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inStr {
			escaped = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[:i+1]
			}
		}
	}
	return ""
}

// ── Caching Layer ───────────────────────────────────────────────────────

func getCache(mongoDB *MongoDB, key string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cached YouTubeCache
	err := mongoDB.YouTubeCache().FindOne(ctx, bson.D{
		{Key: "cacheKey", Value: key},
		{Key: "expiresAt", Value: bson.D{{Key: "$gt", Value: time.Now()}}},
	}).Decode(&cached)
	if err != nil {
		return "", false
	}
	return cached.Data, true
}

func setCache(mongoDB *MongoDB, key, data string) {
	setCacheWithTTL(mongoDB, key, data, cacheTTL)
}

func setCacheWithTTL(mongoDB *MongoDB, key, data string, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	_, err := mongoDB.YouTubeCache().ReplaceOne(ctx,
		bson.D{{Key: "cacheKey", Value: key}},
		YouTubeCache{
			CacheKey:  key,
			Data:      data,
			CachedAt:  now,
			ExpiresAt: now.Add(ttl),
		},
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		log.Printf("Warning: failed to cache YouTube data for key %s: %v", key, err)
	}
}

// cachedYouTubeSearch wraps youtubeSearch with a cache layer.
func cachedYouTubeSearch(mongoDB *MongoDB, apiKey, query string, maxResults int) ([]YouTubeVideo, error) {
	cacheKey := fmt.Sprintf("search:%s:%d", query, maxResults)
	if data, ok := getCache(mongoDB, cacheKey); ok {
		var videos []YouTubeVideo
		if err := json.Unmarshal([]byte(data), &videos); err == nil {
			return videos, nil
		}
	}

	videos, err := youtubeSearch(apiKey, query, maxResults)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(videos); err == nil {
		setCache(mongoDB, cacheKey, string(data))
	}
	return videos, nil
}

// cachedVideoDetails wraps youtubeVideoDetails with a cache layer.
func cachedVideoDetails(mongoDB *MongoDB, apiKey string, videoIDs []string) ([]YouTubeVideo, error) {
	// Check which IDs are cached vs need fetching
	var cached []YouTubeVideo
	var uncachedIDs []string

	for _, id := range videoIDs {
		cacheKey := fmt.Sprintf("video:%s", id)
		if data, ok := getCache(mongoDB, cacheKey); ok {
			var vid YouTubeVideo
			if err := json.Unmarshal([]byte(data), &vid); err == nil {
				cached = append(cached, vid)
				continue
			}
		}
		uncachedIDs = append(uncachedIDs, id)
	}

	if len(uncachedIDs) == 0 {
		return cached, nil
	}

	fresh, err := youtubeVideoDetails(apiKey, uncachedIDs)
	if err != nil {
		return nil, err
	}

	// Cache each new video individually
	for _, vid := range fresh {
		if data, err := json.Marshal(vid); err == nil {
			setCache(mongoDB, fmt.Sprintf("video:%s", vid.VideoID), string(data))
		}
	}

	return append(cached, fresh...), nil
}

// cachedTranscript wraps fetchTranscript with a cache layer.
// Returns (transcript, fromCache, error). Only non-empty transcripts are cached
// so that failed extractions are retried on subsequent runs.
// cachedTranscript returns (transcript, fromCache, method, error).
// method is the name of the fetch method that succeeded (empty if cached).
func cachedTranscript(mongoDB *MongoDB, videoID string) (string, bool, string, error) {
	cacheKey := fmt.Sprintf("transcript:%s", videoID)
	if data, ok := getCache(mongoDB, cacheKey); ok {
		return data, true, "", nil
	}

	transcript, method, err := fetchTranscript(videoID)
	if err != nil {
		return "", false, "", err
	}

	// Only cache non-empty transcripts so failed extractions are retried
	if transcript != "" {
		setCacheWithTTL(mongoDB, cacheKey, transcript, transcriptTTL)
	}
	return transcript, false, method, nil
}

// cachedVideoAssessment retrieves or stores a video assessment in the cache.
// Cache key includes domain+searchTerms hash since assessments are context-dependent.
func cachedVideoAssessment(mongoDB *MongoDB, videoID, domain string, searchTerms []string) (*VideoAssessment, bool) {
	h := sha256.Sum256([]byte(domain + "|" + strings.Join(searchTerms, ",")))
	cacheKey := fmt.Sprintf("assessment:%s:%x", videoID, h[:6])

	if data, ok := getCache(mongoDB, cacheKey); ok {
		var a VideoAssessment
		if err := json.Unmarshal([]byte(data), &a); err == nil {
			return &a, true
		}
	}
	return nil, false
}

func setCachedVideoAssessment(mongoDB *MongoDB, videoID, domain string, searchTerms []string, assessment *VideoAssessment) {
	h := sha256.Sum256([]byte(domain + "|" + strings.Join(searchTerms, ",")))
	cacheKey := fmt.Sprintf("assessment:%s:%x", videoID, h[:6])

	data, err := json.Marshal(assessment)
	if err != nil {
		return
	}
	setCacheWithTTL(mongoDB, cacheKey, string(data), assessmentTTL)
}

// ── URL Helpers ─────────────────────────────────────────────────────────

// extractVideoID extracts the video ID from various YouTube URL formats.
func extractVideoID(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)

	// Direct video ID (11 chars)
	if len(rawURL) == 11 && !strings.Contains(rawURL, "/") && !strings.Contains(rawURL, ".") {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Host)

	// youtu.be/VIDEO_ID
	if host == "youtu.be" || host == "www.youtu.be" {
		return strings.TrimPrefix(parsed.Path, "/")
	}

	// youtube.com/watch?v=VIDEO_ID
	if strings.Contains(host, "youtube.com") {
		if v := parsed.Query().Get("v"); v != "" {
			return v
		}
		// youtube.com/shorts/VIDEO_ID
		if strings.HasPrefix(parsed.Path, "/shorts/") {
			return strings.TrimPrefix(parsed.Path, "/shorts/")
		}
		// youtube.com/embed/VIDEO_ID
		if strings.HasPrefix(parsed.Path, "/embed/") {
			return strings.TrimPrefix(parsed.Path, "/embed/")
		}
	}

	return ""
}

// resolveChannelID resolves a YouTube channel URL to a channel ID.
// Handles /channel/ID, /@handle, /c/name formats.
func resolveChannelID(apiKey, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	path := strings.TrimSuffix(parsed.Path, "/")

	// /channel/UC... format — direct channel ID
	if strings.HasPrefix(path, "/channel/") {
		return strings.TrimPrefix(path, "/channel/"), nil
	}

	// /@handle format — use forHandle API
	if strings.HasPrefix(path, "/@") {
		handle := strings.TrimPrefix(path, "/@")
		return resolveChannelByHandle(apiKey, handle)
	}

	// /c/CustomName or /user/Username — use forUsername
	if strings.HasPrefix(path, "/c/") || strings.HasPrefix(path, "/user/") {
		name := strings.TrimPrefix(path, "/c/")
		name = strings.TrimPrefix(name, "/user/")
		return resolveChannelByUsername(apiKey, name)
	}

	// Bare path like /SomeChannel — try as handle
	if strings.Count(path, "/") == 1 {
		handle := strings.TrimPrefix(path, "/")
		if handle != "" {
			chID, err := resolveChannelByHandle(apiKey, handle)
			if err == nil {
				return chID, nil
			}
		}
	}

	return "", fmt.Errorf("could not resolve channel ID from URL: %s", rawURL)
}

func resolveChannelByHandle(apiKey, handle string) (string, error) {
	u := fmt.Sprintf("%s/channels?part=id&forHandle=%s&key=%s",
		youtubeAPIBase, url.QueryEscape(handle), apiKey)

	resp, err := ytHTTPClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Items) == 0 {
		return "", fmt.Errorf("channel not found for handle: %s", handle)
	}
	return result.Items[0].ID, nil
}

func resolveChannelByUsername(apiKey, username string) (string, error) {
	u := fmt.Sprintf("%s/channels?part=id&forUsername=%s&key=%s",
		youtubeAPIBase, url.QueryEscape(username), apiKey)

	resp, err := ytHTTPClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Items) == 0 {
		return "", fmt.Errorf("channel not found for username: %s", username)
	}
	return result.Items[0].ID, nil
}

// ── Discovery Helper ────────────────────────────────────────────────────

// discoverVideos runs the auto-discovery process: searches YouTube for
// brand-related, competitor, and category content.
// includeBrandSearches gates the auto-generated `<brand> review`, `<brand> tutorial`,
// and `<brand> vs <competitor>` queries — disable when the brand name is a common
// phrase that would generate false positives.
func discoverVideos(mongoDB *MongoDB, apiKey string, brandName, channelURL string, searchTerms, competitors, categories, keyUseCases []string, includeBrandSearches bool, progress func(string)) ([]YouTubeVideo, int, error) {
	seen := make(map[string]bool)
	var allVideos []YouTubeVideo
	quotaUsed := 0

	addVideos := func(videos []YouTubeVideo, tag string) {
		for i := range videos {
			if !seen[videos[i].VideoID] {
				seen[videos[i].VideoID] = true
				videos[i].RelevanceTag = tag
				allVideos = append(allVideos, videos[i])
			}
		}
	}

	// 1. Own channel videos (if provided)
	if channelURL != "" {
		progress("Fetching your channel videos...")
		channelID, err := resolveChannelID(apiKey, channelURL)
		if err == nil {
			quotaUsed += 2
			videos, err := youtubeChannelVideos(apiKey, channelID, 50)
			if err == nil {
				addVideos(videos, "own")
				progress(fmt.Sprintf("Found %d videos from your channel", len(videos)))
			} else {
				log.Printf("Warning: failed to fetch channel videos: %v", err)
				progress(fmt.Sprintf("Could not fetch channel videos: %s", truncateErr(err, 200)))
			}
		} else {
			log.Printf("Warning: failed to resolve channel ID from %s: %v", channelURL, err)
			progress(fmt.Sprintf("Could not resolve channel URL: %s", truncateErr(err, 200)))
		}
	}

	// 2. Brand-specific searches
	searchCount := 0
	failedCount := 0
	var firstErr error
	recordErr := func(query string, err error) {
		if err == nil {
			return
		}
		failedCount++
		if firstErr == nil {
			firstErr = err
		}
		log.Printf("YouTube search failed [%q]: %v", query, err)
	}

	totalSearches := 0
	if brandName != "" && includeBrandSearches {
		totalSearches += 2 // review + tutorial
		totalSearches += len(competitors)
	}
	totalSearches += len(searchTerms)

	if totalSearches > 0 {
		progress(fmt.Sprintf("Searching YouTube across %d queries...", totalSearches))
	}

	if brandName != "" && includeBrandSearches {
		for _, suffix := range []string{"review", "tutorial"} {
			query := brandName + " " + suffix
			quotaUsed += 2
			videos, err := cachedYouTubeSearch(mongoDB, apiKey, query, 10)
			recordErr(query, err)
			if err == nil {
				addVideos(videos, "direct_mention")
			}
			searchCount++
		}

		// 3. Competitor comparison searches (gated on same flag — brand name appears in every query)
		for _, comp := range competitors {
			query := brandName + " vs " + comp
			quotaUsed += 2
			videos, err := cachedYouTubeSearch(mongoDB, apiKey, query, 5)
			recordErr(query, err)
			if err == nil {
				addVideos(videos, "competitor_comparison")
			}
			searchCount++
		}
	}

	// 4. Search term queries
	for _, term := range searchTerms {
		quotaUsed += 2
		videos, err := cachedYouTubeSearch(mongoDB, apiKey, term, 10)
		recordErr(term, err)
		if err == nil {
			addVideos(videos, "category_content")
		}
		searchCount++
	}

	// 5. Auto-generated category discovery searches (max 4 to limit quota)
	var categorySearches []string
	year := time.Now().Year()
	for _, cat := range categories {
		if len(categorySearches) >= 2 {
			break
		}
		categorySearches = append(categorySearches, fmt.Sprintf("best %s tools %d", cat, year))
	}
	for _, uc := range keyUseCases {
		if len(categorySearches) >= 4 {
			break
		}
		categorySearches = append(categorySearches, fmt.Sprintf("how to %s", uc))
	}
	totalSearches += len(categorySearches)
	for _, term := range categorySearches {
		quotaUsed += 2
		videos, err := cachedYouTubeSearch(mongoDB, apiKey, term, 5)
		recordErr(term, err)
		if err == nil {
			addVideos(videos, "category_content")
		}
		searchCount++
	}

	if totalSearches > 0 {
		if failedCount > 0 {
			progress(fmt.Sprintf("Completed %d searches — %d unique videos found (%d failed: %s)",
				totalSearches, len(allVideos), failedCount, truncateErr(firstErr, 200)))
		} else {
			progress(fmt.Sprintf("Completed %d searches — %d unique videos found", totalSearches, len(allVideos)))
		}
	}
	return allVideos, quotaUsed, nil
}

// truncateErr returns the error string trimmed to max characters for display in progress messages.
func truncateErr(err error, max int) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// ── Unused import guard ─────────────────────────────────────────────────

var _ = mongo.ErrNoDocuments
