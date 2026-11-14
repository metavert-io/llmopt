package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// In local account mode the copy's own YouTube key is used instead of the
// account's saved one, without looking the saved one up.
func TestResolveYouTubeKey_LocalCopyKeyWins(t *testing.T) {
	setVar(t, &localYouTubeKey, "copy-key")
	got, err := resolveYouTubeKey(context.Background(), nil, nil, "system-key", true)
	if err != nil || got != "copy-key" {
		t.Errorf("got %q, %v", got, err)
	}
	localYouTubeKey = ""
	got, err = resolveYouTubeKey(context.Background(), nil, nil, "system-key", false)
	if err != nil || got != "system-key" {
		t.Errorf("non-SaaS without override: got %q, %v", got, err)
	}
}

func TestYouTubeErrText(t *testing.T) {
	quota := errors.New(`youtube search API error (429): { "error": { "code": 429, "message": "Quota exceeded for quota metric 'Search Queries' and limit 'Search Queries per day'", "errors": [{"reason": "rateLimitExceeded"}] } }`)
	if txt := youtubeErrText(quota); !strings.Contains(txt, "daily quota is used up") || strings.Contains(txt, "{") {
		t.Errorf("quota text = %q", txt)
	}
	other := errors.New("youtube search API error (400): keyInvalid")
	if txt := youtubeErrText(other); !strings.Contains(txt, "keyInvalid") {
		t.Errorf("other text = %q", txt)
	}
}
