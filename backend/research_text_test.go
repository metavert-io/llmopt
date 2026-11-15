package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Claims retired by the November 2026 research review; none may come back
// into the report digest, the methodology page's digest or the prompts.
var retiredClaims = []string{
	"+41%",               // 2023 lab result, not a current fact
	"76% of AI Overview", // now about 38%
	"49-68",              // replaced by IMC 2026 / EBU-BBC figures
	"$60M", "$70M",       // 2024 deals, reported under renegotiation
	"virtually 0",           // YouTube is the most-cited domain
	"40.1%",                 // misattributed Reddit AIO share
	"12.47%",                // outdated AIO trigger rate
	"single most effective", // retired GEO claim
	"GPTBot up 305%",        // no longer a visibility lever
}

// Claims the November 2026 addendum retired from the research digests: one
// prompt set's Reddit share stated as a fact, YouTube "#1 across AI
// assistants" without the Google-surface caveat, and Wikipedia as ChatGPT's
// top domain.
var retiredDigestClaims = []string{
	"#2 cited domain (6.2%)",
	"Reddit is the #2",
	"and across AI assistants",
	"Wikipedia is ChatGPT's most-cited domain",
}

// addendumFindings must appear in every copy of the digest.
var addendumFindings = []string{
	"title and key paragraph match",
	"API and its consumer app",
	"15-25 runs",
	"retrieved but did not cite",
	"one of its own pages was cited",
	"AIPREF",
}

// checkDigest applies the retired-claim rules to one copy of the digest.
func checkDigest(t *testing.T, name, digest string) {
	t.Helper()
	for _, c := range append(append([]string{}, retiredClaims...), retiredDigestClaims...) {
		if strings.Contains(digest, c) {
			t.Errorf("%s still contains retired claim %q", name, c)
		}
	}
	// The SIGIR association may only appear with Google's own statement.
	if strings.Contains(digest, "less AI Overview retrieval") && !strings.Contains(digest, "Google says it does not affect inclusion or ranking in Google Search") {
		t.Errorf("%s states the Google-Extended association without Google's caveat", name)
	}
	for _, want := range addendumFindings {
		if !strings.Contains(digest, want) {
			t.Errorf("%s is missing the addendum finding %q", name, want)
		}
	}
}

func TestReportDigest_NoRetiredClaims(t *testing.T) {
	checkDigest(t, "report digest", strings.Join(reportResearchDigest, "\n"))
}

// key2026Sources must be cited by the report bibliography and the methodology page.
var key2026Sources = []string{
	"https://arxiv.org/abs/2605.14021", // IMC 2026, AI Overviews
	"https://arxiv.org/abs/2604.27790", // SIGIR 2026
	"https://arxiv.org/abs/2506.11097", // C-SEO Bench
	"https://arxiv.org/abs/2607.14035", // GEO survey
	"https://arxiv.org/abs/2604.07585", // Don't Measure Once
	"https://ahrefs.com/blog/ai-overview-citations-top-10/",
	"https://ahrefs.com/blog/schema-ai-citations/",
	"https://www.conductor.com/academy/reddit-ai-citation-decline/",
	"https://developers.openai.com/api/docs/bots",
	"https://www.ebu.ch/research/open/report/news-integrity-in-ai-assistants",
	// November 2026 addendum
	"https://aclanthology.org/2026.findings-acl.526/", // Kirsten et al., variance
	"https://arxiv.org/abs/2610.15033",                // Chen & Koudas, within-site features
	"https://arxiv.org/abs/2604.25707",                // citation absorption
	"https://arxiv.org/abs/2609.07559",                // 2023 levers re-tested
	"https://arxiv.org/abs/2609.23162",                // own-domain retrieval
	"https://arxiv.org/abs/2611.07614",                // popularity bias
	"https://arxiv.org/abs/2609.04047",                // runs needed
	"https://arxiv.org/abs/2609.18729",                // API vs consumer app
	"https://arxiv.org/abs/2607.14447",                // robots.txt compliance
	"https://arxiv.org/abs/2608.18352",                // AI Mode field experiment
	"https://www.meltwater.com/en/blog/ai-search-visibility-report-october-2026",
	"https://ahrefs.com/blog/why-chatgpt-cites-pages/",
	"https://developers.google.com/search/docs/crawling-indexing/google-common-crawlers",
}

func TestReportBibliography_Cites2026Sources(t *testing.T) {
	urls := map[string]bool{}
	for i, b := range reportBibliography {
		if b.url == "" || b.title == "" {
			t.Errorf("bibliography entry %s has no title or URL", b.ref)
		}
		if want := "[" + strconv.Itoa(i+1) + "]"; b.ref != want {
			t.Errorf("bibliography entry %d is numbered %s, want %s", i+1, b.ref, want)
		}
		urls[b.url] = true
	}
	for _, u := range key2026Sources {
		if !urls[u] {
			t.Errorf("report bibliography doesn't cite %s", u)
		}
	}
	found := false
	for _, b := range reportBibliography {
		if strings.Contains(b.title, "NanoKnow: How to Know What Your Language Model Knows") && strings.Contains(b.venue, "SIGIR 2026") {
			found = true
		}
	}
	if !found {
		t.Error("NanoKnow must be cited by its real title and venue")
	}
}

func TestResearchPage_Cites2026SourcesAndAnchorsResolve(t *testing.T) {
	for _, u := range key2026Sources {
		if !strings.Contains(researchHTML, `href="`+u+`"`) {
			t.Errorf("methodology page doesn't cite %s", u)
		}
	}
	digest := researchHTML[strings.Index(researchHTML, `<div class="digest">`):]
	digest = digest[:strings.Index(digest, "</div>")]
	checkDigest(t, "methodology digest", regexp.MustCompile(`<[^>]+>`).ReplaceAllString(digest, ""))
	for _, c := range []string{"single most effective", "virtually 0%", "40.1%", "12.47%"} {
		if strings.Contains(researchHTML, c) {
			t.Errorf("methodology page still contains retired claim %q", c)
		}
	}
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`id="([a-z0-9-]+)"`).FindAllStringSubmatch(researchHTML, -1) {
		ids[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`href="#([a-z0-9-]+)"`).FindAllStringSubmatch(researchHTML, -1) {
		if !ids[m[1]] {
			t.Errorf("methodology page links to #%s, which doesn't exist", m[1])
		}
	}
}

func TestAnalysisPrompts_NoRetiredClaims(t *testing.T) {
	p := videoContextParams{BrandName: "Brand"}
	prompts := map[string]string{
		"reddit":    buildRedditAuthorityPrompt("brand.example", nil, nil, nil, BrandContextInfo{}),
		"search":    buildSearchVisibilityPrompt("brand.example", BrandContextInfo{}, nil, nil, nil),
		"video 1":   buildPillar1Prompt(p),
		"video 4":   buildPillar4Prompt(p),
		"synthesis": buildSynthesisPrompt("Brand", "{}", "{}", "{}", "{}"),
	}
	for name, prompt := range prompts {
		for _, c := range retiredClaims {
			if strings.Contains(prompt, c) {
				t.Errorf("%s prompt still contains retired claim %q", name, c)
			}
		}
	}
	search := prompts["search"]
	for _, want := range []string{"About 38%", "OAI-SearchBot", "Cloudflare", "key commercial and comparison pages", "llms.txt"} {
		if !strings.Contains(search, want) {
			t.Errorf("search prompt is missing %q", want)
		}
	}
	if !strings.Contains(prompts["reddit"], "fell about 50%") {
		t.Error("reddit prompt should say Reddit's citation share fell")
	}
}

// The analysis prompts are formatted once, so they must not show the model
// a doubled "%%" or a formatting error.
func TestAnalysisPrompts_NoFormatResidue(t *testing.T) {
	p := videoContextParams{BrandName: "Brand"}
	for name, s := range map[string]string{
		"reddit":    buildRedditAuthorityPrompt("brand.example", nil, nil, nil, BrandContextInfo{}),
		"search":    buildSearchVisibilityPrompt("brand.example", BrandContextInfo{}, nil, nil, nil),
		"video 1":   buildPillar1Prompt(p),
		"video 2":   buildPillar2Prompt(p),
		"video 3":   buildPillar3Prompt(p),
		"video 4":   buildPillar4Prompt(p),
		"synthesis": buildSynthesisPrompt("Brand", "{}", "{}", "{}", "{}"),
	} {
		for _, bad := range []string{"%!", "%%"} {
			if i := strings.Index(s, bad); i >= 0 {
				t.Errorf("%s: %q near %q", name, bad, s[max(0, i-70):min(len(s), i+15)])
			}
		}
	}
}

// The in-app methodology (App.tsx) carries the same research: a source card
// for every report source, and the same digest rules.
func TestInAppMethodology_MatchesReport(t *testing.T) {
	b, err := os.ReadFile("../frontend/src/App.tsx")
	if err != nil {
		t.Skip("frontend source not available:", err)
	}
	app := string(b)
	for _, e := range reportBibliography {
		if !strings.Contains(app, "url: '"+e.url+"'") {
			t.Errorf("in-app methodology has no source card for %s (%s)", e.ref, e.url)
		}
	}
	start := strings.Index(app, "Research Digest\n")
	if start < 0 {
		t.Fatal("in-app research digest not found")
	}
	digest := app[start:]
	digest = digest[:strings.Index(digest, "</section>")]
	checkDigest(t, "in-app digest", regexp.MustCompile(`<[^>]+>`).ReplaceAllString(digest, ""))
}
