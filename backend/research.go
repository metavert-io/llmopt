package main

import (
	"fmt"
	"net/http"
)

// handleResearchPage serves the /research page as complete server-rendered HTML
// for SEO — crawlers and browsers alike get the full content without JavaScript.
func handleResearchPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fmt.Fprint(w, researchHTML)
	}
}

// researchHTML is the complete server-rendered research page.
const researchHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Research Citations — LLM Optimizer</title>
<meta name="description" content="Peer-reviewed research underpinning LLM Optimizer's AI search visibility analysis. Scoring frameworks, dimension weights, and recommendations derived from academic work on generative engine optimization, training data influence, and AI citation dynamics.">
<meta name="keywords" content="LLM research, generative engine optimization, GEO, AI search citations, AI visibility research, training data, NanoKnow, YouTube AI citations, Reddit AI training data, AI Overview citations">
<link rel="canonical" href="https://llmopt.metavert.io/research">

<!-- Open Graph -->
<meta property="og:type" content="article">
<meta property="og:title" content="Research Citations — LLM Optimizer">
<meta property="og:description" content="Peer-reviewed research underpinning LLM Optimizer's AI search visibility analysis. Scoring frameworks, dimension weights, and recommendations derived from academic work.">
<meta property="og:url" content="https://llmopt.metavert.io/research">
<meta property="og:site_name" content="LLM Optimizer">

<!-- Twitter Card -->
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="Research Citations — LLM Optimizer">
<meta name="twitter:description" content="Peer-reviewed research underpinning LLM Optimizer's AI search visibility analysis.">

<!-- Schema.org structured data -->
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Article",
  "headline": "Research Citations — LLM Optimizer",
  "description": "Peer-reviewed research underpinning LLM Optimizer's AI search visibility analysis. Scoring frameworks, dimension weights, and recommendations derived from academic work on generative engine optimization, training data influence, and AI citation dynamics.",
  "url": "https://llmopt.metavert.io/research",
  "publisher": {
    "@type": "Organization",
    "name": "Metavert LLC",
    "url": "https://www.metavert.io"
  },
  "mainEntityOfPage": "https://llmopt.metavert.io/research",
  "about": [
    {"@type": "Thing", "name": "Generative Engine Optimization"},
    {"@type": "Thing", "name": "AI Search Visibility"},
    {"@type": "Thing", "name": "Large Language Models"}
  ]
}
</script>

<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
html{-webkit-text-size-adjust:100%}
body{font-family:'Inter',system-ui,-apple-system,sans-serif;background:#0a0a0f;color:#a3a3b8;line-height:1.6;-webkit-font-smoothing:antialiased}
a{color:#7c6aef;text-decoration:none;transition:color .15s}
a:hover{color:#9b8af2}
.container{max-width:56rem;margin:0 auto;padding:0 1rem}
@media(min-width:640px){.container{padding:0 1.5rem}}

/* Header */
header{position:sticky;top:0;z-index:50;background:rgba(10,10,15,.8);backdrop-filter:blur(16px);border-bottom:1px solid #1e1e2e}
header .inner{max-width:56rem;margin:0 auto;padding:0 1rem;height:4rem;display:flex;align-items:center;justify-content:space-between}
@media(min-width:640px){header .inner{padding:0 1.5rem}}
header a.logo{display:flex;align-items:center;gap:.75rem;color:#fff;font-size:1.25rem;font-weight:600;letter-spacing:-.025em}
.logo-icon{width:2rem;height:2rem;border-radius:.5rem;background:linear-gradient(135deg,#7c6aef,#a855f7);display:flex;align-items:center;justify-content:center}
.logo-icon svg{width:1rem;height:1rem;color:#fff}
header a.back{color:#6b6b80;font-size:.875rem;transition:color .15s;display:flex;align-items:center;gap:.375rem}
header a.back:hover{color:#fff}

/* Main */
main{max-width:48rem;margin:0 auto;padding:3rem 1rem}
@media(min-width:640px){main{padding:3rem 1.5rem}}
h2{font-size:1.875rem;font-weight:700;color:#fff;margin-bottom:.5rem}
.subtitle{color:#6b6b80;font-size:.875rem;margin-bottom:2rem}

/* Sections */
section{margin-bottom:3rem}
h3{font-size:1.125rem;font-weight:600;color:#fff;margin-bottom:.5rem;display:flex;align-items:center;gap:.5rem}
h3 svg{width:1.25rem;height:1.25rem;color:#7c6aef;flex-shrink:0}
.section-desc{color:#6b6b80;font-size:.875rem;margin-bottom:1.5rem}

/* Digest card */
.digest{background:rgba(10,10,15,.6);border:1px solid #1e1e2e;border-radius:1rem;padding:1.5rem}
@media(min-width:640px){.digest{padding:2rem}}
.digest p{font-size:.875rem;line-height:1.75;margin-bottom:1rem}
.digest p:last-child{margin-bottom:0}
.digest strong{color:#fff}
.digest em{font-style:italic}
.digest a{color:#7c6aef}
.digest a:hover{color:#9b8af2}

/* Paper cards */
.paper{display:block;background:rgba(10,10,15,.5);border:1px solid #1e1e2e;border-radius:.75rem;padding:1rem;margin-bottom:.75rem;transition:border-color .15s;scroll-margin-top:6rem}
.paper:hover{border-color:rgba(124,106,239,.4)}
.paper-title{color:#fff;font-size:.875rem;font-weight:500;transition:color .15s}
.paper:hover .paper-title{color:#9b8af2}
.paper-venue{color:#4a4a5e;font-size:.75rem;margin-top:.125rem}
.paper-desc{color:#6b6b80;font-size:.75rem;margin-top:.5rem;line-height:1.6}

/* Framework cards */
.fw-card{background:rgba(10,10,15,.5);border:1px solid #1e1e2e;border-radius:.75rem;padding:1.25rem;margin-bottom:1rem}
.fw-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:.5rem}
.fw-name{color:#fff;font-weight:500;font-size:.875rem}
.fw-weight{font-size:.75rem;padding:.125rem .5rem;border-radius:9999px;background:#1e1e2e;color:#6b6b80;border:1px solid #2a2a3e}
.fw-source{color:#4a4a5e;font-size:.6875rem;margin-bottom:.5rem}
.fw-desc{color:#6b6b80;font-size:.75rem;line-height:1.6}

/* Finding cards */
.finding{display:block;background:rgba(10,10,15,.5);border:1px solid #1e1e2e;border-radius:.75rem;padding:1rem;margin-bottom:.75rem;transition:border-color .15s}
.finding:hover{border-color:rgba(124,106,239,.4)}
.finding-title{color:#fff;font-size:.875rem;font-weight:500;transition:color .15s}
.finding:hover .finding-title{color:#9b8af2}
.finding-detail{color:#6b6b80;font-size:.75rem;margin-top:.375rem;line-height:1.6}
.finding-source{color:#4a4a5e;font-size:.6875rem;margin-top:.5rem}

/* CTA */
.cta{background:linear-gradient(135deg,rgba(124,106,239,.2),rgba(168,85,247,.2));border:1px solid rgba(124,106,239,.3);border-radius:1rem;padding:2rem;text-align:center;margin-bottom:3rem}
.cta h3{justify-content:center;font-size:1.25rem;margin-bottom:.5rem}
.cta p{color:#a3a3b8;font-size:.875rem;max-width:32rem;margin:0 auto 1.5rem}
.cta-buttons{display:flex;flex-wrap:wrap;gap:.75rem;justify-content:center}
.btn-primary{display:inline-flex;align-items:center;gap:.5rem;padding:.75rem 1.5rem;background:#6d5bd0;color:#fff;border-radius:.75rem;font-weight:600;transition:background .15s}
.btn-primary:hover{background:#7c6aef;color:#fff}
.btn-secondary{display:inline-flex;align-items:center;gap:.5rem;padding:.75rem 1.5rem;border:1px solid #3a3a4e;color:#a3a3b8;border-radius:.75rem;font-weight:600;transition:all .15s}
.btn-secondary:hover{border-color:#4a4a5e;color:#fff}
.cta-note{color:#4a4a5e;font-size:.75rem;margin-top:1rem}

/* Footer */
footer{border-top:1px solid #2a2a3e;background:rgba(10,10,15,.8);padding:2rem 0;margin-top:3rem}
footer .inner{max-width:56rem;margin:0 auto;padding:0 1rem;display:flex;flex-direction:column;align-items:center;gap:1rem;font-size:.75rem}
@media(min-width:640px){footer .inner{padding:0 1.5rem}}
.footer-links{display:flex;flex-wrap:wrap;align-items:center;justify-content:center;gap:.25rem 1.25rem;color:#6b6b80}
.footer-links a{color:#6b6b80;transition:color .15s}
.footer-links a:hover{color:#fff}
.footer-sep{color:#2a2a3e}
.footer-copy{color:#4a4a5e}
.footer-copy a{color:#6b6b80}
.footer-copy a:hover{color:#fff}
</style>
</head>
<body>

<header>
<div class="inner">
  <a href="/" class="logo">
    <div class="logo-icon">
      <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z"/></svg>
    </div>
    LLM Optimizer
  </a>
  <a href="/" class="back">
    <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18"/></svg>
    Back to App
  </a>
</div>
</header>

<main>
<h2>Research Citations</h2>
<p class="subtitle">The research underpinning LLM Optimizer's analysis methodology. All scoring frameworks, dimension weights, and recommendations are derived from peer-reviewed academic work and validated practitioner research.</p>

<!-- Research Digest -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z"/></svg>
  Research Digest
</h3>
<div class="digest">
  <p><strong>Brand Recognition vs. Discovery.</strong> A key framework throughout LLM Optimizer is the distinction between <em>brand recognition</em> &#8212; how well AI represents your brand when people search for it by name &#8212; and <em>inbound discovery</em> &#8212; how often AI surfaces your brand when people search your category without prior knowledge of you. Both matter, but they require different strategies. Brand recognition improves through authority signals, earned media, and training data presence. Discovery requires appearing in category-level content, answering the questions your audience asks before they know you exist, and being present in the YouTube videos, Wikipedia pages, review sites, comparison lists and community threads that AI assistants draw on for category queries.</p>
  <p>The emerging science of LLM visibility distinguishes two ways a brand becomes known to AI. <em>Parametric</em> visibility is what a model learned in training; <em>dynamic</em> visibility is what it finds when it searches the web while answering. <a href="#nanoknow">NanoKnow (SIGIR 2026)</a> shows, on small open models, that how often an answer appears in the training data strongly affects whether a model knows it, and that retrieved evidence and training knowledge complement each other. Both matter in practice: ChatGPT searched the web for <a href="#semrush-chatgpt-2026">about 34.5% of US prompts</a> in early 2026 and answered the rest from what it had learned. The dual imperative holds: be published widely enough to be learned, and be retrievable and citable when assistants search.</p>
  <p>What a model remembers shapes even its searched answers. In a <a href="#chen-koudas">University of Toronto study of 158,000 answers</a> collected in mid-2026, ChatGPT wrote a brand into its own search queries 67% of the time when the brand was in its prior knowledge, against 11% otherwise, and 46% of its unprompted brand mentions had no supporting cited page. Familiarity compounds: <a href="#monaco-popular">the most prominent brands were cited 1.4-2.8 times as often</a> as the average brand, web retrieval didn't correct this, and fact-preserving rewrites gave negligible gains. In <a href="#kumar-geo-scale">one vendor panel</a>, household names appeared in 73% of relevant category answers, mid-market brands in 44% and niche brands in 11%, so compare a brand with peers of similar size, not with category leaders. <a href="#unusual-prominence">Category leaders</a> reach nearly every retrieval but win only 25-41% of the recommendation slots they reach, so they need differentiation; about half of specialist and regional brands never surfaced at all, so they first need to be found. When two providers both left a brand out, they <a href="#unusual-failure-modes">agreed 95% of the time on why</a>: it never reached the model, reached it but wasn't mentioned, or was mentioned but not positioned as the pick. For challengers, third-party signals that build familiarity matter more than rewrites: with identical specifications, well-known brands won every recommendation <a href="#incumbent-advantage">until a competitor had even a small rating advantage</a> (under 0.1 stars).</p>
  <p>Across the research, AI answers cite third parties far more than a brand's own pages: independent sources made up <a href="#geo-toronto">72-92% of citations</a> in the Toronto study, and <a href="#zatuchin-reputation">85.7% of citations in brand answers</a> across 13 languages pointed to sites the brand doesn't own. Much of that third-party share is other companies' commercial pages and ranked "best X" listicles, not only editorial media: in one vendor panel the tracked brand's own domain drew <a href="#kumar-geo-scale">2.9% of citations</a>. Owned pages still matter. In <a href="#tannenbaum-stage">another vendor panel</a>, a brand was named in about 3% of searched answers when neither it nor its site came up, about half the time when one of its own pages was cited, and almost always when the engine also searched its name. Self-ranked lists are a weak lever: self-promotional listicles drew <a href="#peec-listicles">about 11% of citations</a> in software reviews (3.6% in ChatGPT), but when a brand's own "best X" list was cited in AI Overviews, <a href="#ray-listicles">the brand was left out of the recommendation 69% of the time</a>; third-party lists are what count. Video is part of third-party coverage: YouTube is the <a href="#ahrefs-most-cited-aio">most-cited domain in Google AI Overviews (24.5%)</a> and leads in aggregate across assistants, <a href="#meltwater-oct-2026">mainly through Google's AI surfaces</a>, while Wikipedia leads in Claude and Copilot and is <a href="#answer-bubbles">overrepresented in AI summaries</a>; per-engine rankings depend on the prompt set. Because <a href="#livecc">video LLMs learn from transcripts</a>, not pictures, what is said in a video matters far more than its production value.</p>
  <p>What has not held up is the 2023 headline that adding quotations raises visibility <a href="#geo-princeton">by 41%</a>. It was a lab result: <a href="#c-seo-bench">C-SEO Bench</a>, a <a href="#geo-survey-2026">2026 survey of 45 studies</a> and a <a href="#bajemon-rochet">re-test on ten current engine families</a> (by vendor-affiliated authors, with code and data released) found the quotation, statistics and cite-sources levers moved citation on none of them. Page features that look helpful when comparing websites &#8212; FAQ blocks, summary boxes, schema markup &#8212; showed <a href="#chen-koudas">no effect when comparing pages of the same website</a>. What held up was fit to the question: pages whose title and key paragraph match the question people ask were cited about 14-16% more per standard deviation, and which pages an engine's own search retrieved explained far more of the variation than any page feature. Being cited is also not the same as shaping the answer: pages containing figures, definitions, comparisons and how-to steps had <a href="#zhang-absorption">markedly more influence on the answer</a> than pages without them, while Q&amp;A formatting had slightly less, and explicit prices and recent dates <a href="#vishwakarma-cited">helped in a controlled test</a>. The levers that reproduce are relevance, prominence and specific, usable evidence, so a page score measures quality and fit for the question, not a predicted citation rate.</p>
  <p>Reddit's role is shifting, and how much it counts depends on how it is measured. It was foundational training data through <a href="#reddit-webtext">WebText</a> and the <a href="#reddit-common-crawl">Common Crawl</a>, and Google and OpenAI signed <a href="#reddit-data-deals">data licensing deals with it in 2024</a>, which are reported to be under renegotiation. Its upvoted consensus, especially in "best X for Y" threads, still <a href="#reddit-community-consensus">shapes how models describe a category</a>. But its share of AI citations <a href="#conductor-reddit">fell about 50% between October 2025 and January 2026</a>, and per-engine figures disagree widely: <a href="#ahrefs-most-cited-chatgpt">Ahrefs' November 2026 prompt set</a> put Reddit second in ChatGPT at 6.2%, while <a href="#meltwater-oct-2026">Meltwater's October 2026 data</a> had ChatGPT near 0.4% Reddit and Claude at none, with Reddit Gemini's top domain until Gemini's Reddit citations fell 88% in late October. Citation share understates its influence: in <a href="#ahrefs-chatgpt-cites">Ahrefs' study of 1.4 million ChatGPT prompts</a>, Reddit made up 67.8% of the pages ChatGPT retrieved but did not cite. Genuine participation is worth it; over-investing in Reddit at the expense of other channels is not.</p>
  <p>Google AI Overviews and standalone assistants draw on different sources, and AI Overviews lean less on rankings than they did. About <a href="#ahrefs-aio-top10-2026">38% of AI Overview citations</a> come from top-10 organic pages (Ahrefs, March 2026; 76% in its 2025 study, partly because of a change in how results are parsed), on average <a href="#kirsten-acl26">53% of the domains AI Overviews consult</a> are outside the organic top 10, and Google's results, AI Overviews and Gemini <a href="#sigir26-disrupts">share few sources</a>. Gemini 3 has <a href="#techcrunch-aio-mode">powered AI Overviews since January 2026</a>, with a handoff into AI Mode, which passed <a href="#google-io-2026">1 billion monthly users</a>. Clicks fall where AI answers appear: the top organic result loses <a href="#ahrefs-ctr-2026">about 58% of its clicks</a> under an AI Overview, a field experiment found AI Overviews <a href="#agarwal-sen-aio">cut organic clicks by 38%</a>, and in a <a href="#wang-ai-mode-rct">preregistered experiment</a> an AI Mode-only search cut click-through by 18.8 percentage points (21.2 points for clicks to Reddit). Visibility increasingly shows up as being named, not as visits. For standalone assistants, <a href="#ahrefs-ai-search-overlap">only 12% of cited URLs rank in Google's top 10</a>. Across 75,000 brands, <a href="#ahrefs-75k-2026">YouTube mentions (about 0.74) and branded web mentions (0.66-0.71)</a> correlate most strongly with AI visibility and Domain Rating far less (0.27-0.33) &#8212; correlation, not causation. Freshness matters too: AI assistants cite content <a href="#ahrefs-freshness">25.7% newer</a> than search results do, and <a href="#seer-recency-2026">how recently key commercial and comparison pages were updated</a> matters more than a site's average content age.</p>
  <p>Crawlers now differ by purpose. OpenAI's <a href="#openai-crawlers">GPTBot collects training data only</a>; OAI-SearchBot fetches pages for ChatGPT search, and ChatGPT-User, which fetches pages a user asks about, may not follow robots.txt. Blocking training crawlers is a business choice, but blocking search crawlers hides a site from AI answers. Google-Extended controls whether Google may use a site for Gemini training and grounding; <a href="#google-crawlers">Google says it does not affect inclusion or ranking in Google Search</a>, and appearing in AI Overviews needs only <a href="#google-ai-features">an indexed page eligible for a snippet</a>. A SIGIR 2026 study found that sites blocking Google's AI crawler appear <a href="#sigir26-disrupts">less in AI Overview retrieval</a>, but that is an association: the sites that block AI crawlers are <a href="#robots-gatekeeping">disproportionately large, reputable publishers</a>. Google intends to support the AIPREF training opt-out signal in early 2027. robots.txt states intent but doesn't enforce it: in a <a href="#robots-respect">controlled 2026 study</a> Claude and Mistral followed it, while Gemini and Copilot fetched disallowed pages. Training is now <a href="#cloudflare-bot-report-2026">52% of crawler requests and mixed-use crawlers over 36%</a>, which makes clean training-only blocks hard, and since 15 September 2026 <a href="#cloudflare-2026">Cloudflare blocks AI crawlers by default</a> on new domains, so a site can be hidden from AI without its owner choosing it. Two popular tactics showed no effect on live engines in 2026: <a href="#ahrefs-schema-2026">schema markup gave no meaningful citation lift</a> (it still helps rich results and entity understanding, and <a href="#sageo-arena">helped retrieval in one simulated pipeline</a>), and <a href="#ahrefs-llmstxt">97% of llms.txt files got no requests</a> from AI crawlers; Google says no special files or schema are needed.</p>
  <p>AI answers are not reliably grounded, and how often depends on the topic. <a href="#imc26-aio">11% of AI Overview claims</a> in general queries were not supported by the sources they cited, against <a href="#agnoli-health-aio">about 43% of cited statements in health answers</a>, and <a href="#ebu-bbc">45% of AI assistant answers about news</a> had at least one significant issue. Errors about brands cluster where they hurt: in <a href="#profound-inaccuracy">one vendor dataset of 158,000 checked claims</a>, pricing and billing were 12% of claims but 24% of the inaccurate ones, and 54% of brands had at least one error that cited their own pages, so explicit, current pricing pages and an audit of old owned pages are worth the effort. Citation still concentrates: the <a href="#news-citing-patterns">top 20 sources capture 28-67% of citations</a>, and LLMs attend most to the <a href="#lost-in-the-middle">beginning and end of their context</a>, so front-loading answers still pays.</p>
  <p>Answers also vary from run to run. Even at temperature 0, <a href="#kirsten-acl26">9-27% of yes/no answers flipped within five minutes</a>, and only 18% of the pages AI Overviews used were the same two months later, against 45% for organic results; asked the same question repeatedly, AI tools returned the <a href="#sparktoro-inconsistency">same list of brands less than 1% of the time</a>. Brand-level rates averaged over many questions are reliable after a few runs, but a <a href="#dice-roll">difference on a single question</a> is only indicative at five runs &#8212; detecting it reliably takes 15-25 runs. Sentiment is the least stable measure: it flipped <a href="#kumar-geo-scale">about 6.7 times as often as mention</a> in one vendor panel, and automated sentiment scores varied far more with <a href="#zatuchin-sentiment">the query's language (26% of variance) than with the brand (1.5%)</a>. The interface matters too: for the same question, a chatbot's <a href="#uberti-product-audit">API and its consumer app shared only 12-15% of cited domains</a>, and providers recommend from memory at very different rates. Visibility is therefore best measured as a <a href="#schulte-measure">distribution over repeated runs</a>, per engine &#8212; how often a brand is mentioned, not where it ranks in a list &#8212; and the figures here describe associations, not causes.</p>
  <p>Model updates add volatility. <a href="#resoneo-citation-collapse">When GPT-5.3 replaced GPT-4o</a> as ChatGPT's default, unique domains cited per response dropped 20.5% overnight, and in October 2026 alone <a href="#meltwater-oct-2026">Gemini cut its links per answer and Perplexity trimmed its listed sources</a>, so per-engine shares are cited here with dates, not built into scores. This reinforces the value of parametric visibility alongside dynamic visibility; research into <a href="#dejan-brand-authority">LLM parametric memory</a> suggests that being associated with category leaders can matter more than raw mention frequency. Together, these findings inform LLM Optimizer's scoring frameworks across answer optimization, video authority, Reddit authority and search visibility. The LLM Visibility Score weighs the LLM Test &#8212; the direct measure of what assistants answer &#8212; as heavily as answer optimization (25% each; video 20%, Reddit 15%, search 15%). The LLM Test asks each question several times through the providers' APIs, reports sentiment separately from its score, and with web search on shows why a brand is missed: never found, found but not named, or named but not recommended.</p>
</div>
</section>

<!-- Source Papers -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25"/></svg>
  Source Papers
</h3>

<a id="lost-in-the-middle" class="paper" href="https://arxiv.org/abs/2307.03172" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Lost in the Middle: How Language Models Use Long Contexts</div>
  <div class="paper-venue">TACL 2024</div>
  <div class="paper-desc">Position bias in LLM context windows &#8212; U-shaped attention curve where content at the beginning and end is reliably used while middle content is ignored.</div>
</a>

<a id="geo-princeton" class="paper" href="https://arxiv.org/abs/2311.09735" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">GEO: Generative Engine Optimization</div>
  <div class="paper-venue">Princeton / KDD 2024</div>
  <div class="paper-desc">Tested 9 content rewriting strategies on 10,000 queries in a lab setting (2023). Quotations (+41%), statistics (+33%) and fluency (+29%) helped most there; 2025-2026 replications (C-SEO Bench, a survey of 45 studies) found these gains don't reliably hold on live engines.</div>
</a>

<a id="nanoknow" class="paper" href="https://arxiv.org/abs/2602.20122" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">NanoKnow: How to Know What Your Language Model Knows</div>
  <div class="paper-venue">SIGIR 2026 (Gu, Jedidi, Lin)</div>
  <div class="paper-desc">Links benchmark questions to a fully open pre-training corpus. On small open models, closed-book accuracy depends strongly on how often the answer appears in the pre-training data; external evidence reduces that dependence, and models stay more accurate on answers they saw in training, so training and retrieval complement each other.</div>
</a>

<a id="geo-toronto" class="paper" href="https://arxiv.org/abs/2509.08919" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">GEO: How to Dominate AI Search &#8212; Source Preferences</div>
  <div class="paper-venue">U of Toronto 2025</div>
  <div class="paper-desc">AI search engines cite earned media 72-92% of the time vs. 18-27% for brand-owned content. AI citations overlap with Google results only 15-50%.</div>
</a>

<a id="youtube-citations" class="paper" href="https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">YouTube vs Reddit AI Citations</div>
  <div class="paper-venue">Adweek / Bluefish / Emberos / Goodie AI, 2025</div>
  <div class="paper-desc">YouTube appears in 16% of LLM answers (vs. 10% for Reddit). YouTube's social citation share doubled from 18.9% to 39.2% between Aug-Dec 2024.</div>
</a>

<a id="news-citing-patterns" class="paper" href="https://arxiv.org/abs/2507.05301" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">News Source Citing Patterns in AI Search Systems</div>
  <div class="paper-venue">2025</div>
  <div class="paper-desc">Citation concentration and gatekeeping dynamics across 366K citations. Top 20 sources capture 28-67% of all citations (Gini 0.69-0.83).</div>
</a>

<a id="livecc" class="paper" href="https://arxiv.org/abs/2504.16030" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">LiveCC: Learning Video LLM with Streaming Speech Transcription</div>
  <div class="paper-venue">CVPR 2025</div>
  <div class="paper-desc">How video LLMs are trained from ASR transcripts. A 7B model trained on YouTube transcripts surpassed 72B models, proving transcript quality matters more than model size.</div>
</a>

<a id="false-promise" class="paper" href="https://arxiv.org/abs/2410.22349" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">The False Promise of Factual and Verifiable Source-Cited Responses</div>
  <div class="paper-venue">2024</div>
  <div class="paper-desc">Citation accuracy ranged 49-68% across 2024 answer engines, and 23-32% of claims had no source backing. See IMC 2026 and EBU/BBC 2025 for current figures.</div>
</a>

<a id="reddit-webtext" class="paper" href="https://cdn.openai.com/better-language-models/language_models_are_unsupervised_multitask_learners.pdf" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Language Models are Unsupervised Multitask Learners</div>
  <div class="paper-venue">OpenAI, 2019 (Radford et al.)</div>
  <div class="paper-desc">Introduced WebText, built from about 8 million pages linked from Reddit posts with 3+ karma, as GPT-2's training corpus. Reddit's community voting served as a quality filter for training data.</div>
</a>

<a id="reddit-common-crawl" class="paper" href="https://dl.acm.org/doi/10.1145/3630106.3659033" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Consent in Crisis: The Rapid Decline of the AI Data Commons</div>
  <div class="paper-venue">ACM FAccT 2024 (Longpre et al.)</div>
  <div class="paper-desc">Comprehensive audit of AI training data sources documenting Reddit's persistent prominence in Common Crawl and other web corpora. Found that robots.txt restrictions increased 25%+ from 2023-2024 as sites restricted AI crawling, while Reddit data remained broadly available through licensing agreements.</div>
</a>

<a id="reddit-data-deals" class="paper" href="https://www.reuters.com/technology/reddit-ai-content-licensing-deal-google-2024-02-22/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Reddit Data Licensing: Google and OpenAI Deals</div>
  <div class="paper-venue">Reuters / The Verge, 2024</div>
  <div class="paper-desc">Google and OpenAI signed paid data licensing deals with Reddit in 2024; they are reported to be under renegotiation. Reddit's API was locked down in 2023, and Reddit has sued AI companies over scraping.</div>
</a>

<a id="reddit-community-consensus" class="paper" href="https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Community Consensus as LLM Authority Signal</div>
  <div class="paper-venue">Bluefish Labs / Emberos Research, 2025</div>
  <div class="paper-desc">Reddit's multi-user validation (upvotes, comment consensus) creates credibility signals single-author content cannot match. "Best X for Y" recommendation threads are among the most influential for LLM comparison queries.</div>
</a>

<a id="ahrefs-aio-citations" class="paper" href="https://ahrefs.com/blog/search-rankings-ai-citations/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Overview Citations and Search Rankings</div>
  <div class="paper-venue">Ahrefs, 2025</div>
  <div class="paper-desc">2025 study: 76% of AI Overview citations came from top-10 organic pages, with a median position of 3. Ahrefs' March 2026 update puts the top-10 share at about 38%.</div>
</a>

<a id="ahrefs-ai-search-overlap" class="paper" href="https://ahrefs.com/blog/ai-search-overlap/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Search Overlap: How AI Citations Differ from Google</div>
  <div class="paper-venue">Ahrefs, 2025</div>
  <div class="paper-desc">Only 12% of standalone LLM citations overlap with Google's top 10. Perplexity shows 28.6% overlap. 80%+ of ChatGPT/Claude/Gemini citations come from pages not ranking in Google at all.</div>
</a>

<a id="ahrefs-75k-brands" class="paper" href="https://ahrefs.com/blog/ai-brand-visibility-correlations/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Brand Visibility Correlations (75K Brands)</div>
  <div class="paper-venue">Ahrefs, 2025</div>
  <div class="paper-desc">2025 study: YouTube mentions (0.737) and web mentions (0.664) correlated most strongly with AI visibility, brand search volume 0.334, backlinks far less. Top 25% brands got 12x more AIO mentions. Correlation, not causation.</div>
</a>

<a id="ahrefs-freshness" class="paper" href="https://ahrefs.com/blog/do-ai-assistants-prefer-to-cite-fresh-content/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Do AI Assistants Prefer to Cite Fresh Content?</div>
  <div class="paper-venue">Ahrefs, 2025 (17M citations)</div>
  <div class="paper-desc">AI assistants cite content 25.7% newer than traditional search. ChatGPT: avg 1,023 days old. Perplexity pulls ~50% from current year. Google AIOs counter-trend: prefer older authoritative content.</div>
</a>

<a id="seer-recency" class="paper" href="https://www.seerinteractive.com/insights/study-ai-brand-visibility-and-content-recency" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Brand Visibility and Content Recency</div>
  <div class="paper-venue">Seer Interactive, 2025</div>
  <div class="paper-desc">65% of AI bot crawl hits target content published within the past year. 85% of AIO citations from last 2 years. 94% from last 5 years.</div>
</a>

<a id="llm-recency-bias" class="paper" href="https://arxiv.org/html/2509.11353v1" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Do Large Language Models Favor Recent Content?</div>
  <div class="paper-venue">arXiv, September 2025</div>
  <div class="paper-desc">LLMs consistently promote "fresh" passages. Top-10 mean publication year shifts forward by up to 4.78 years. Individual items move up to 95 ranking positions based on recency signals alone.</div>
</a>

<a id="cloudflare-ai-crawlers" class="paper" href="https://blog.cloudflare.com/from-googlebot-to-gptbot-whos-crawling-your-site-in-2025/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">From Googlebot to GPTBot: Who's Crawling Your Site</div>
  <div class="paper-venue">Cloudflare, 2025</div>
  <div class="paper-desc">GPTBot grew 305% YoY. OpenAI crawl-to-referral ratio: 1,700:1. Anthropic: 73,000:1. ~21% of top-1000 sites block GPTBot. Training crawls = 80% of AI bot activity.</div>
</a>

<a id="semrush-aio" class="paper" href="https://www.semrush.com/blog/semrush-ai-overviews-study/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Overviews Study: 200,000 Keywords</div>
  <div class="paper-venue">Semrush, 2025</div>
  <div class="paper-desc">80% of AI Overview responses targeted informational queries, and 82% appeared for keywords with &lt;1,000 monthly searches (2025 data).</div>
</a>

<a id="ahrefs-aio-top10-2026" class="paper" href="https://ahrefs.com/blog/ai-overview-citations-top-10/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Overview Citations and the Top 10 (2026 update)</div>
  <div class="paper-venue">Ahrefs, March 2026 (vendor study, 863K keywords)</div>
  <div class="paper-desc">About 38% of AI Overview citations now come from pages in the top 10 organic results, down from 76% in Ahrefs' 2025 study, partly because of a change in how results are parsed.</div>
</a>

<a id="imc26-aio" class="paper" href="https://arxiv.org/abs/2605.14021" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Measuring Google AI Overviews</div>
  <div class="paper-venue">IMC 2026 (Xu, Iqbal, Montgomery)</div>
  <div class="paper-desc">About 30% of the domains AI Overviews cite are not on the first page of search results, and 11% of AI Overview claims are not supported by the sources they cite.</div>
</a>

<a id="sigir26-disrupts" class="paper" href="https://arxiv.org/abs/2604.27790" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">How Generative AI Disrupts Search</div>
  <div class="paper-venue">SIGIR 2026 (Grossman et al.)</div>
  <div class="paper-desc">Google's results, AI Overviews and Gemini draw on largely different sources (Jaccard overlap below 0.2). Sites blocking Google's AI crawler are associated with less AI Overview retrieval; Google says Google-Extended doesn't affect Search inclusion, and blocking sites are disproportionately large publishers, so treat this as an association.</div>
</a>

<a id="c-seo-bench" class="paper" href="https://arxiv.org/abs/2506.11097" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">C-SEO Bench</div>
  <div class="paper-venue">NeurIPS 2025 Datasets &amp; Benchmarks (Puerto et al.)</div>
  <div class="paper-desc">A benchmark of conversational search optimization methods: most content rewrites meant to raise AI citation, including the 2023 GEO techniques, were largely ineffective in realistic settings.</div>
</a>

<a id="geo-survey-2026" class="paper" href="https://arxiv.org/abs/2607.14035" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Critical Survey of GEO, 2023-2026</div>
  <div class="paper-venue">arXiv, 2026 (Martinez)</div>
  <div class="paper-desc">Reviews 45 GEO studies: the large 2023 gains, such as +41% from adding quotations, hold in lab settings but don't reliably carry over to live engines. Relevance to the query and prominence of the answer are the levers that reproduce.</div>
</a>

<a id="schulte-measure" class="paper" href="https://arxiv.org/abs/2604.07585" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Don't Measure Once</div>
  <div class="paper-venue">arXiv, 2026 (Schulte, Bleeker, Kaufmann)</div>
  <div class="paper-desc">AI answers vary between runs, so visibility should be measured as a distribution over repeated runs, not from a single answer.</div>
</a>

<a id="sparktoro-inconsistency" class="paper" href="https://sparktoro.com/blog/new-research-ais-are-highly-inconsistent-when-recommending-brands-or-products-marketers-should-take-care-when-tracking-ai-visibility/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AIs Are Highly Inconsistent When Recommending Brands</div>
  <div class="paper-venue">SparkToro / Gumshoe, January 2026</div>
  <div class="paper-desc">Asked the same question repeatedly, AI tools returned the same list of brands less than 1% of the time. Track how often a brand appears, not where it ranks in a list.</div>
</a>

<a id="tannenbaum-engine" class="paper" href="https://arxiv.org/abs/2609.22655" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Scoring With the Engine: Retrieval Exposure, Cross-Engine Divergence, and the Limits of Engine-Agnostic GEO Scores</div>
  <div class="paper-venue">arXiv, 2026 (Tannenbaum; small sample)</div>
  <div class="paper-desc">Different engines cited almost entirely different URLs for the same prompt, and one engine's set of cited URLs turned over by 67% from one day to the next.</div>
</a>

<a id="ahrefs-75k-2026" class="paper" href="https://thenextweb.com/news/ahrefs-youtube-mentions-ai-visibility-brand-search" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">YouTube Mentions and AI Visibility (75K Brands, 2026)</div>
  <div class="paper-venue">Ahrefs data via The Next Web, May 2026 (vendor data)</div>
  <div class="paper-desc">YouTube mentions (about 0.737) and branded web mentions (0.66-0.71) correlate most strongly with AI visibility; Domain Rating correlates far less (0.27-0.33). These are correlations, not causes.</div>
</a>

<a id="ahrefs-most-cited-aio" class="paper" href="https://ahrefs.com/blog/most-cited-domains-ai-overviews/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Most-Cited Domains in AI Overviews</div>
  <div class="paper-venue">Ahrefs, November 2026 (vendor data)</div>
  <div class="paper-desc">YouTube is the most-cited domain in Google AI Overviews, at 24.5% of citations.</div>
</a>

<a id="ahrefs-most-cited-chatgpt" class="paper" href="https://ahrefs.com/blog/most-cited-domains-in-chatgpt/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Most-Cited Domains in ChatGPT</div>
  <div class="paper-venue">Ahrefs, November 2026 (vendor data)</div>
  <div class="paper-desc">In Ahrefs' November 2026 prompt set, Wikipedia was ChatGPT's most-cited domain (10.1%) and Reddit second (6.2%). Meltwater's October 2026 data put NIH first in ChatGPT and Reddit near 0.4%: per-engine rankings depend on the prompt set.</div>
</a>

<a id="digiday-youtube" class="paper" href="https://digiday.com/marketing/in-graphic-detail-llms-keep-citing-youtube-in-search-results/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">LLMs Keep Citing YouTube in Search Results</div>
  <div class="paper-venue">Digiday, September 2026 (Meltwater and Scrunch data)</div>
  <div class="paper-desc">YouTube was the most-cited domain across AI assistants in Meltwater's August 2026 data, mainly through Google's AI Overviews.</div>
</a>

<a id="conductor-reddit" class="paper" href="https://www.conductor.com/academy/reddit-ai-citation-decline/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Reddit's AI Citation Decline</div>
  <div class="paper-venue">Conductor, 2026 (vendor data)</div>
  <div class="paper-desc">Reddit's share of AI citations fell about 50% between October 2025 and January 2026.</div>
</a>

<a id="ahrefs-schema-2026" class="paper" href="https://ahrefs.com/blog/schema-ai-citations/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Schema Markup and AI Citations</div>
  <div class="paper-venue">Ahrefs, May 2026 (controlled test, vendor)</div>
  <div class="paper-desc">Adding schema markup gave no meaningful lift in AI citations on live engines. Schema still supports rich results and entity understanding.</div>
</a>

<a id="ahrefs-llmstxt" class="paper" href="https://ahrefs.com/blog/llmstxt-study/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">llms.txt Study</div>
  <div class="paper-venue">Ahrefs, June 2026 (vendor)</div>
  <div class="paper-desc">97% of llms.txt files received no requests from AI crawlers.</div>
</a>

<a id="seer-recency-2026" class="paper" href="https://www.seerinteractive.com/insights/study-content-recencys-impact-on-ai-visibility-in-2026" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Content Recency's Impact on AI Visibility in 2026</div>
  <div class="paper-venue">Seer Interactive, 2026</div>
  <div class="paper-desc">How recently key commercial and comparison pages were updated matters more for AI visibility than a site's average content age.</div>
</a>

<a id="semrush-chatgpt-2026" class="paper" href="https://www.semrush.com/blog/chatgpt-search-insights/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">ChatGPT Search Insights (Clickstream)</div>
  <div class="paper-venue">Semrush, 2026 (vendor data)</div>
  <div class="paper-desc">ChatGPT ran a web search for about 34.5% of US prompts (February 2026 data).</div>
</a>

<a id="openai-crawlers" class="paper" href="https://developers.openai.com/api/docs/bots" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">OpenAI Crawlers</div>
  <div class="paper-venue">OpenAI documentation</div>
  <div class="paper-desc">GPTBot collects training data; OAI-SearchBot indexes pages for ChatGPT search; ChatGPT-User fetches pages a user asks about and may not follow robots.txt.</div>
</a>

<a id="cloudflare-2026" class="paper" href="https://blog.cloudflare.com/content-independence-day-ai-options/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Content Independence Day: AI Options</div>
  <div class="paper-venue">Cloudflare, July 2026</div>
  <div class="paper-desc">Cloudflare blocks AI crawlers by default on new domains from 15 September 2026, so a site can block AI crawlers without its owner choosing to.</div>
</a>

<a id="techcrunch-aio-mode" class="paper" href="https://techcrunch.com/2026/01/27/google-now-lets-users-jump-from-ai-overviews-into-ai-mode-conversations" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Google Lets Users Jump from AI Overviews into AI Mode</div>
  <div class="paper-venue">TechCrunch, January 2026</div>
  <div class="paper-desc">From January 2026, AI Overviews run on Gemini 3 and let users continue into AI Mode conversations.</div>
</a>

<a id="google-io-2026" class="paper" href="https://blog.google/products-and-platforms/products/search/search-io-2026/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Search at I/O 2026</div>
  <div class="paper-venue">Google, 2026 (self-reported)</div>
  <div class="paper-desc">AI Mode passed 1 billion monthly users.</div>
</a>

<a id="ahrefs-ctr-2026" class="paper" href="https://ahrefs.com/blog/ai-overviews-reduce-clicks-update" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Overviews Reduce Clicks (2026 update)</div>
  <div class="paper-venue">Ahrefs, February 2026 (vendor)</div>
  <div class="paper-desc">When an AI Overview appears, the top organic result's click-through rate falls by about 58%.</div>
</a>

<a id="ebu-bbc" class="paper" href="https://www.ebu.ch/research/open/report/news-integrity-in-ai-assistants" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">News Integrity in AI Assistants</div>
  <div class="paper-venue">EBU / BBC, October 2025</div>
  <div class="paper-desc">45% of AI assistant answers about news had at least one significant issue.</div>
</a>

<a id="metavert-state-2026" class="paper" href="https://metavert.io/state-of-ai-search-citations" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">State of AI Search Citations: Q4 2026</div>
  <div class="paper-venue">Metavert, 2026</div>
  <div class="paper-desc">A synthesis of the 2026 AI citation research behind these scoring frameworks.</div>
</a>

<a id="kirsten-acl26" class="paper" href="https://aclanthology.org/2026.findings-acl.526/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Characterizing Web Search in The Age of Generative AI</div>
  <div class="paper-venue">Findings of ACL 2026 (Kirsten, Große Perdekamp, Wu, Upadhyay, Gummadi, Zafar)</div>
  <div class="paper-desc">Even at temperature 0, 9-27% of yes/no answers flipped within five minutes. Rerun about two months apart, only 18% of AI Overview pages were the same, against 45% for organic results, and on average 53% of the domains AI Overviews consult are outside the organic top 10 (data from July-September 2025).</div>
</a>

<a id="chen-koudas" class="paper" href="https://arxiv.org/abs/2610.15033" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">What Drives AI Citations? Content, Retrieval and Model Knowledge in Generative Engine Optimization (GEO)</div>
  <div class="paper-venue">arXiv, October 2026 (Chen, Koudas, University of Toronto)</div>
  <div class="paper-desc">158,099 answers and 468,037 citations collected daily from June to September 2026. Comparing pages of the same website, FAQ blocks, summary boxes and schema had no significant effect; title and best-paragraph match to the question added about 14-16% per standard deviation. Retrieval explained 32% of within-website variation, page features 2.8%. ChatGPT wrote a brand into its searches 67% of the time when it knew the brand (11% otherwise), and 46% of its unprompted brand mentions had no supporting page.</div>
</a>

<a id="zhang-absorption" class="paper" href="https://arxiv.org/abs/2604.25707" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">From Citation Selection to Citation Absorption: A Measurement Framework for GEO Across AI Search Platforms</div>
  <div class="paper-venue">arXiv, April 2026 (Zhang, He, Yao; descriptive study)</div>
  <div class="paper-desc">ChatGPT cited 6.88 sources per prompt, against 12.06 for Google and 16.35 for Perplexity, but its cited pages had much higher influence on the answer. Pages with numbers (+61.6%), definitions (+57.3%), comparisons (+55.3%) or how-to steps (+41.2%) had more influence; Q&amp;A formatting had 5.7% less. Influence is a constructed proxy, so these are descriptive differences.</div>
</a>

<a id="bajemon-rochet" class="paper" href="https://arxiv.org/abs/2609.07559" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Scoring Without the Engine: Validating a Deterministic, Manipulation-Resistant Content Score for Generative Engines, End to End</div>
  <div class="paper-venue">arXiv, September 2026 (Bajemon, Rochet; vendor-affiliated, code and data released)</div>
  <div class="paper-desc">Re-tested the three strongest 2023 GEO levers (quotation, statistics, cite sources; published gains +42.6%, +32.8%, +27.7%) on ten current engine families: they moved citation on none. A query-agnostic content score correlated weakly with citation within a query (Spearman 0.11), so such scores are quality filters, not citation predictors.</div>
</a>

<a id="vishwakarma-cited" class="paper" href="https://arxiv.org/abs/2605.25517" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">What Gets Cited: Competitive GEO in AI Answer Engines</div>
  <div class="paper-venue">arXiv, May 2026 (Vishwakarma, Kumar, Jamidar; controlled testbed)</div>
  <div class="paper-desc">In 252,000 controlled trials across six LLMs, topical relevance and list position drove being cited first; explicit prices and a recent timestamp also helped, while formatting-only edits had little impact.</div>
</a>

<a id="sageo-arena" class="paper" href="https://arxiv.org/abs/2602.12187" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">SAGEO Arena: A Realistic Environment for Evaluating Search-Augmented Generative Engine Optimization</div>
  <div class="paper-venue">KDD 2026 (Kim, Jeong, Kim, Lee, Lee; simulated pipeline)</div>
  <div class="paper-desc">In a full retrieve-rerank-generate pipeline, existing GEO methods were largely impractical and often degraded retrieval and reranking; structural information, which benchmarks usually discard (such as schema markup), helped mitigate this.</div>
</a>

<a id="tannenbaum-stage" class="paper" href="https://arxiv.org/abs/2609.23162" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">From Prompt to Recommendation: A Fitted Stage Model of Brand Visibility in AI Search</div>
  <div class="paper-venue">arXiv, September 2026 (Tannenbaum; author's company data, Aiso Boost)</div>
  <div class="paper-desc">34,960 unbranded prompt-engine observations, June-September 2026. A brand was mentioned 2.8% (GPT) and 3.8% (Gemini) of the time when neither it nor its domain was in the retrieval path, 49.0% and 58.4% with an own-domain citation, and 91.4% and 100% when the engine also searched the brand's name.</div>
</a>

<a id="zatuchin-reputation" class="paper" href="https://arxiv.org/abs/2606.25787" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">How Large Language Models Source Brand Reputation Across Languages and Markets</div>
  <div class="paper-venue">arXiv, June 2026 (Zatuchin; author's company data, Rankfor.AI)</div>
  <div class="paper-desc">Across 128 brands, 12 markets and 13 languages, 85.7% of citations in brand answers pointed to sites the brand doesn't own and 14.3% to its own sites; Wikipedia was the most-cited domain in 11 of 12 languages.</div>
</a>

<a id="kumar-geo-scale" class="paper" href="https://arxiv.org/abs/2606.20065" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Generative Engine Optimization at Scale: Measuring Brand Visibility Across AI Search Engines</div>
  <div class="paper-venue">arXiv, June 2026 (Kumar; author is a Ranqo co-founder, vendor data)</div>
  <div class="paper-desc">On category prompts, household names appeared in 73% of relevant answers, mid-market brands in 44% and niche brands in 11%. Sentiment flipped about 6.7 times as often as mention. The tracked brand's own domain drew 2.9% of citations; other companies' pages 75.2%, with ranked "best X" listicles the most-cited format.</div>
</a>

<a id="monaco-popular" class="paper" href="https://arxiv.org/abs/2611.07614" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">The Popular Get More Popular: Auditing Brand Bias in Generative Search</div>
  <div class="paper-venue">arXiv, November 2026 (Monaco, Lenti, Pierri)</div>
  <div class="paper-desc">Across 1,176 market leaders, top-quintile brands were cited 1.4-2.8 times as often as the average brand, and web retrieval didn't mitigate it. Swapping real brand names for unfamiliar ones lowered recommendation rank, and fact-preserving content rewrites gave negligible gains.</div>
</a>

<a id="incumbent-advantage" class="paper" href="https://arxiv.org/abs/2606.17443" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Incumbent Advantage: Brand Bias and Cognitive Manipulation Dynamics in LLM Recommendation Systems</div>
  <div class="paper-venue">arXiv, October 2026 (Chu, Hou)</div>
  <div class="paper-desc">With identical specifications, well-known brands were recommended 100% of the time; that dominance disappeared when a competitor had a rating advantage of less than 0.1 stars.</div>
</a>

<a id="unusual-failure-modes" class="paper" href="https://arxiv.org/abs/2606.26116" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Divergent Recommendations, Convergent Diagnoses: Cross-Provider Failure-Mode Convergence in AI Commercial Recommendation</div>
  <div class="paper-venue">arXiv, May 2026 (Jack, Lehman, Maloney, Xu, unusual.ai; company-affiliated)</div>
  <div class="paper-desc">Providers disagreed on which brands to recommend about two-thirds of the time, and recommended from prior knowledge at very different rates (Anthropic 43-52%, OpenAI 8-29%). When both left a brand out, they agreed on the failure stage 95.1% of the time: the brand never reached the model, reached it but wasn't mentioned, or was mentioned but not positioned as the pick.</div>
</a>

<a id="unusual-prominence" class="paper" href="https://arxiv.org/abs/2605.27439" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Prominence-Stratified Failure Modes in Retrieval-Augmented Commercial Recommendation: A 37,000-Run Audit</div>
  <div class="paper-venue">arXiv, May 2026 (Jack, Lehman, Maloney, Xu, unusual.ai; company-affiliated)</div>
  <div class="paper-desc">Category leaders appeared in nearly every relevant retrieval but won only 25-41% of the recommendation slots they reached; 48-52% of specialist and regional brands never surfaced in any of the 37,000 runs.</div>
</a>

<a id="dice-roll" class="paper" href="https://arxiv.org/abs/2609.04047" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">The Dice Roll Method: A Standardized Protocol for Repeated-Query Auditing of Large Language Model Brand Recommendations</div>
  <div class="paper-venue">arXiv, October 2026, v2 (Zatuchin; implemented in a product of the author's company)</div>
  <div class="paper-desc">Aggregated over 250 questions, reliability reached 0.96 at five runs. But the power to detect even a large difference on a single question was only 0.15-0.19 at five runs; reaching 0.8 needs about 15-25 runs.</div>
</a>

<a id="zatuchin-sentiment" class="paper" href="https://arxiv.org/abs/2607.13304" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Query Language Sensitivity in Automated Sentiment Scores of LLM Responses</div>
  <div class="paper-venue">arXiv, November 2026, v2 (Zatuchin)</div>
  <div class="paper-desc">In 12,933 responses, query language accounted for 26.49% of the variance in automated sentiment scores and the brand for 1.46%. Don't compare sentiment across languages.</div>
</a>

<a id="uberti-product-audit" class="paper" href="https://arxiv.org/abs/2609.18729" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">"If I Had to Buy Just ONE: Galaxy S26 Ultra": Auditing AI-Generated Product Recommendations</div>
  <div class="paper-venue">arXiv, September 2026 (Uberti-Bona Marin et al.)</div>
  <div class="paper-desc">For the same product question, a chatbot's API and its consumer app shared on average only 12.0% (ChatGPT) and 14.8% (Gemini) of cited domains, and the ChatGPT and Gemini apps shared 5.4%. API tests describe the models, not exactly what app users see.</div>
</a>

<a id="answer-bubbles" class="paper" href="https://arxiv.org/abs/2603.16138" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Answer Bubbles: Information Exposure in AI-Mediated Search</div>
  <div class="paper-venue">arXiv, 2026, listed for EMNLP 2026 (Huang, Goyal, Saha, Chandrasekharan)</div>
  <div class="paper-desc">In AI summaries, Wikipedia and longer sources were disproportionately overrepresented, and cited social media content and negatively framed sources substantially underrepresented.</div>
</a>

<a id="meltwater-oct-2026" class="paper" href="https://www.meltwater.com/en/blog/ai-search-visibility-report-october-2026" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Search Visibility Report for October 2026</div>
  <div class="paper-venue">Meltwater, November 2026 (vendor data)</div>
  <div class="paper-desc">About 120,000 prompts, 12.7M answers and 88.6M citations across eight platforms. YouTube led overall, but the top domain differed by platform: YouTube in AI Overviews, Google in AI Mode, Wikipedia in Claude and Copilot, Reddit in Gemini, NIH in ChatGPT, X in Grok, Yahoo in Perplexity. ChatGPT stayed near 0.4% Reddit and Claude cited none; Gemini's Reddit citations fell 88% in late October.</div>
</a>

<a id="ahrefs-chatgpt-cites" class="paper" href="https://ahrefs.com/blog/why-chatgpt-cites-pages/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Why ChatGPT Cites One Page Over Another (Study of 1.4M Prompts)</div>
  <div class="paper-venue">Ahrefs, April 2026 (vendor data)</div>
  <div class="paper-desc">ChatGPT cited about half of the URLs it retrieved. Reddit pages it retrieved were cited only 1.93% of the time, and Reddit made up 67.8% of all retrieved-but-not-cited URLs. Cited pages' titles were closer to the prompt than non-cited ones (similarity 0.602 vs 0.484).</div>
</a>

<a id="agnoli-health-aio" class="paper" href="https://arxiv.org/abs/2610.39339" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Does the Citation Matter? Quality, Grounding, and Physician-Rubric Accuracy of Google's AI Overviews for Medical Information</div>
  <div class="paper-venue">arXiv, October 2026 (Agnoli, Reilly, Barna, Horvát)</div>
  <div class="paper-desc">Across 3,611 health questions, AI Overviews appeared 96.4% of the time; after correcting for classifier error, about 56.7% of cited statements were supported by the cited page (so about 43% were not), and a further 20.8% of statements were uncited factual claims.</div>
</a>

<a id="profound-inaccuracy" class="paper" href="https://www.tryprofound.com/blog/where-do-inaccurate-ai-claims-come-from" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Where Do Inaccurate AI Claims Come From?</div>
  <div class="paper-venue">Profound, August 2026 (vendor data)</div>
  <div class="paper-desc">Across 158,000+ checked claims, inaccuracy rates by engine ranged from 4.3% to 6.8%. Pricing and billing were 12% of claims but 24% of inaccurate ones, and 54% of brands had at least one inaccurate claim citing their own content.</div>
</a>

<a id="wang-ai-mode-rct" class="paper" href="https://arxiv.org/abs/2608.18352" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI in Search Reduces Publisher Referrals Without Improving User Experience: Experimental Evidence</div>
  <div class="paper-venue">arXiv, August 2026 (Wang, Gleason, Bart, Wilson, Metaxa; preregistered field experiment)</div>
  <div class="paper-desc">1,100 US Google users, March 2026. An AI Mode-only search cut click-through by 18.8 percentage points (Reddit 21.2, news 12.5, Wikipedia 9.9) and lowered trust in information found on Google; removing AI Overviews and AI Mode raised click-through by 8.8 points.</div>
</a>

<a id="agarwal-sen-aio" class="paper" href="https://www.searchenginejournal.com/ai-overviews-cut-organic-clicks-38-field-study-finds/573145/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Study Confirms Google AI Overviews Cut Organic Clicks 38%</div>
  <div class="paper-venue">Search Engine Journal, April 2026 (reporting Agarwal and Sen's SSRN working paper)</div>
  <div class="paper-desc">In a field experiment with 1,065 users, AI Overviews cut organic clicks by 38% on queries where they appeared (0.38 clicks per search against 0.61), and zero-click searches rose from 54% to 72%. A working paper, not yet peer-reviewed.</div>
</a>

<a id="google-crawlers" class="paper" href="https://developers.google.com/search/docs/crawling-indexing/google-common-crawlers" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Google's Common Crawlers</div>
  <div class="paper-venue">Google Search Central documentation</div>
  <div class="paper-desc">Google-Extended controls whether content may be used for Gemini training and grounding; it "does not impact a site's inclusion in Google Search nor is it used as a ranking signal in Google Search". Google intends to support the AIPREF training-preference signal in early 2027.</div>
</a>

<a id="google-ai-features" class="paper" href="https://developers.google.com/search/docs/appearance/ai-features" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Features and Your Website</div>
  <div class="paper-venue">Google Search Central documentation</div>
  <div class="paper-desc">To appear in AI Overviews or AI Mode a page must be indexed and eligible to be shown with a snippet; no new machine-readable files, AI text files, markup or special schema.org structured data are needed.</div>
</a>

<a id="robots-gatekeeping" class="paper" href="https://arxiv.org/abs/2510.10315" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Is Misinformation More Open? A Study of robots.txt Gatekeeping on the Web</div>
  <div class="paper-venue">WWW 2026 (Steinacker-Olsztyn, Gosain, Dao)</div>
  <div class="paper-desc">60.0% of reputable news sites disallowed at least one AI crawler, against 9.1% of misinformation sites, so what AI can retrieve is skewed toward sites that don't block.</div>
</a>

<a id="robots-respect" class="paper" href="https://arxiv.org/abs/2607.14447" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Do Generative AI Assistants Respect robots.txt? Tracing Web Access Beyond Visible Answers</div>
  <div class="paper-venue">arXiv, July 2026 (Lopez-Fonseca, Rodriguez, Bechtold, Del Alamo)</div>
  <div class="paper-desc">In 200 controlled trials across ten assistants, Claude and Mistral consistently followed robots.txt, while Gemini and Copilot accessed disallowed pages; robots.txt cannot be assumed to reliably govern inference-time web access.</div>
</a>

<a id="cloudflare-bot-report-2026" class="paper" href="https://blog.cloudflare.com/agentic-internet-bot-report/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Content Independence Day, One Year On: Building the Business Model for the Agentic Internet</div>
  <div class="paper-venue">Cloudflare, July 2026 (self-reported)</div>
  <div class="paper-desc">52% of crawler requests were for AI training in June 2026 (22% in spring 2025), mixed-use crawlers combining search, agent use and training were over 36% of activity, and Google sent about 88% of referral traffic.</div>
</a>

<a id="ray-listicles" class="paper" href="https://lilyraynyc.substack.com/p/why-calling-yourself-the-best-could" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Why Calling Yourself the "Best" Could Be Helping Your Competitors Win in AI Search</div>
  <div class="paper-venue">Lily Ray, June 2026 (practitioner analysis)</div>
  <div class="paper-desc">Across 100 B2B "best [category] software" queries in AI Overviews, when a brand's own self-ranked listicle was cited, the brand was left out of the recommendation 69% of the time (224 of 323 cases).</div>
</a>

<a id="peec-listicles" class="paper" href="https://docs.peec.ai/research/self-promotional-listicles" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Self-Promotional Listicles Still Get Cited: About 1 in 10 Citations</div>
  <div class="paper-venue">Peec, March 2026 (vendor data)</div>
  <div class="paper-desc">Across 232,000 citations in software reviews, about 11% came from self-promotional listicles: 3.6% in ChatGPT, 10.3% in Google AI Mode and 10.4% in Perplexity.</div>
</a>

<a id="resoneo-citation-collapse" class="paper" href="https://think.resoneo.com/chatgpt/5.3-5.4/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">ChatGPT Search Visibility: GPT-5.3/5.4 Citation Analysis</div>
  <div class="paper-venue">Resoneo, 2026</div>
  <div class="paper-desc">27,000 responses across 400 prompts over 14 weeks. After GPT-5.3 launched, unique domains cited per response dropped 20.5% (19.1 &#8594; 15.2) and unique URLs dropped 21.0% (24.1 &#8594; 19.1). Formalizes the distinction between parametric visibility (training data knowledge) and dynamic visibility (real-time web retrieval).</div>
</a>

<div id="dejan-brand-authority" class="paper">
  <div class="paper-title">Brand Authority Index: Network Centrality in LLM Parametric Memory</div>
  <div class="paper-venue">Dejan AI, 2026</div>
  <div class="paper-desc">Queried Gemini 200,000 times across ~20 million brand mentions, building a 2.9 million-node association graph. Found that being densely associated with high-authority brands can matter more than raw mention frequency for parametric visibility.</div>
</div>
</section>

<!-- Answer Optimization Framework -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 3v11.25A2.25 2.25 0 006 16.5h2.25M3.75 3h-1.5m1.5 0h16.5m0 0h1.5m-1.5 0v11.25A2.25 2.25 0 0118 16.5h-2.25m-7.5 0h7.5m-7.5 0l-1 3m8.5-3l1 3m0 0l.5 1.5m-.5-1.5h-9.5m0 0l-.5 1.5"/></svg>
  Answer Optimization Scoring Framework
</h3>
<p class="section-desc">Each optimization report scores how well a page fits and answers a buyer's question across four research-backed dimensions. It measures quality and fit for the question, not a predicted citation rate: in 2026 tests, which pages an engine's own search retrieves explained far more of the variation in citations than any page feature.</p>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Content Authority</span><span class="fw-weight">30%</span></div>
  <div class="fw-source">Source: Chen &amp; Koudas (2026) + Zhang et al. (2026) + C-SEO Bench (NeurIPS 2025) + Bajemon &amp; Rochet (2026)</div>
  <div class="fw-desc">Query fit and extractable evidence: do the page's title and a key paragraph match the question people actually ask, and does the page answer it directly and early with the evidence assistants use &#8212; figures, definitions, comparisons, steps and explicit prices, with named sources and dates? Within a website, title and paragraph match to the question was the page property tied to more citations (about +14-16% per standard deviation), and pages with figures, definitions and comparisons shaped answers more. The 2023 rewriting levers, such as adding quotations (+41% in a lab study), moved citation on none of ten current engine families. Penalizes keyword stuffing.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Structural Optimization</span><span class="fw-weight">20%</span></div>
  <div class="fw-source">Source: Lost in the Middle (TACL 2024) + Chen &amp; Koudas (2026) + Ahrefs schema study (2026)</div>
  <div class="fw-desc">Evaluates answer prominence (front-loaded vs. buried), conciseness, and whether the structure carries usable evidence &#8212; comparison tables, steps, clear headings &#8212; plus justification language that explains "why" rather than just "what." FAQ blocks, summary boxes and author bylines get no credit by themselves: comparing pages of the same website, they showed no citation effect in 2026, and Q&amp;A formatting slightly lowered a page's influence on the answer. Schema.org markup helps rich results and entity understanding but showed no measurable citation lift on live engines, so it counts little.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Source Authority</span><span class="fw-weight">30%</span></div>
  <div class="fw-source">Source: GEO (Toronto 2025) + Zatuchin (2026) + Ray (2026) + Meltwater (2026)</div>
  <div class="fw-desc">Assesses third-party coverage: independent reviews and ratings, industry publications, Wikipedia, YouTube videos from other creators, and third-party "best X" comparison lists. A brand's own self-ranked "best X" lists don't count: when a brand's own list was cited in AI Overviews, the brand was left out of the recommendation 69% of the time in a 2026 analysis. AI answers cite third-party sources 72-92% of the time (85.7% of citations in brand answers across 13 languages), much of it other companies' pages and listicles; YouTube leads in Google's AI Overviews, Wikipedia in Claude and Copilot. Evaluates cross-engine consistency since different AI providers cite substantially different sources.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Knowledge Persistence</span><span class="fw-weight">20%</span></div>
  <div class="fw-source">Source: NanoKnow (SIGIR 2026) + Monaco et al. (2026)</div>
  <div class="fw-desc">Estimates how likely the information is to be known to models from training. On small open models, accuracy depends strongly on how often an answer appears in the training data, and retrieved evidence and training knowledge complement each other. Familiarity compounds: the most prominent brands are cited 1.4-2.8 times as often as the average brand, and fact-preserving rewrites don't close the gap. Clear, self-contained answer passages that are widely repeated across quality sources score highest.</div>
</div>
</section>

<!-- Video Authority Framework -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z"/></svg>
  Video Authority Scoring Framework
</h3>
<p class="section-desc">Video analysis evaluates YouTube presence across four pillars, grounded in the finding that LLMs process video through transcripts, not visual content.</p>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Transcript Authority</span><span class="fw-weight">30%</span></div>
  <div class="fw-source">Source: LiveCC (CVPR 2025) + Ahrefs 75K Brands (2026)</div>
  <div class="fw-desc">Transcript quality is the dominant signal for LLM visibility. Evaluates keyword alignment, quotability (clear, standalone statements a model can lift into an answer), information density, and caption availability. Videos without captions are effectively invisible to LLMs. YouTube mentions correlate strongly with AI visibility (about 0.74 across 75,000 brands; correlation, not causation).</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Topical Dominance</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: AI Search Arena (2025) + GEO (Toronto 2025)</div>
  <div class="fw-desc">Measures topic coverage breadth and depth, share of voice across video content in the space, content gaps representing first-mover opportunities, and coverage depth (surface vs. in-depth treatment). Winner-take-all dynamics mean being first in a topic gap has outsized value.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Citation Network</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: AI Search Arena (2025) + YouTube Citation Analysis (Adweek 2025)</div>
  <div class="fw-desc">Analyzes who mentions the brand, their authority level, and concentration risk. Top 20 sources capture 28-67% of all AI citations. A mention by a high-authority channel outweighs dozens of small-channel mentions. Human engagement metrics (views, subscribers) do not predict AI citation.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Brand Narrative Quality</span><span class="fw-weight">20%</span></div>
  <div class="fw-source">Source: IMC 2026 + Agnoli et al. (2026) + EBU/BBC (2025) + Lost in the Middle (2024)</div>
  <div class="fw-desc">Evaluates sentiment, mention context and position (early mentions get priority per U-shaped attention), extractability (clear mentions are less likely to be misrepresented: 11% of AI Overview claims in general queries, and about 43% of cited statements in health answers, weren't supported by their sources), and narrative coherence. Includes a confidence discount reflecting how often AI answers misstate their sources.</div>
</div>
</section>

<!-- Reddit Authority Framework -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" style="color:#f97316"><path stroke-linecap="round" stroke-linejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155"/></svg>
  Reddit Authority Scoring Framework
</h3>
<p class="section-desc">Reddit analysis evaluates community discussion across four pillars, grounded in Reddit's unique role as a multi-user validation platform for LLM training data.</p>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Presence</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: Reddit Training Data Analysis (2024-2025) + GEO (Toronto 2025)</div>
  <div class="fw-desc">Volume and breadth of brand mentions across relevant subreddits. Measures total mentions, unique subreddits reached, and mention trend over time. High presence in topic-specific subreddits carries more weight than general discussion.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Sentiment &amp; Recommendations</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: Community Consensus Research (Bluefish/Emberos 2025)</div>
  <div class="fw-desc">Community tone and recommendation strength. Evaluates positive/negative sentiment balance, recommendation rate in "best X for Y" threads, and the specific praise/criticism themes that shape LLM perception.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Competitive Positioning</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: GEO (Toronto 2025) + Reddit Community Analysis</div>
  <div class="fw-desc">Head-to-head positioning against competitors in comparison threads. Measures win rate, cited differentiators, and competitor advantages not countered &#8212; these directly shape LLM comparison responses.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Training Signal Strength</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: NanoKnow (SIGIR 2026) + Reddit Data Licensing (2024) + Conductor, Ahrefs, Meltwater (2026)</div>
  <div class="fw-desc">Likelihood that Reddit discussions will influence LLM training and answers. High-upvote threads in authoritative subreddits with deep comment engagement create the strongest signals. Google and OpenAI licensed Reddit data in 2024 (deals reported to be under renegotiation). Reddit's share of AI citations fell about 50% from late 2025 to early 2026 and per-engine figures disagree (about 0.4% to 6.2% of ChatGPT citations in two 2026 datasets), but ChatGPT retrieves Reddit far more than it cites it, so citation share understates its influence. One channel among several.</div>
</div>
</section>

<!-- Search Visibility Framework -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z"/></svg>
  Search Visibility Scoring Framework
</h3>
<p class="section-desc">Search visibility analysis evaluates how search-related signals affect whether AI systems will discover, index, and cite your content &#8212; bridging traditional SEO signals with AI citation dynamics. When Brand Intelligence provides category data, a fifth pillar (Category Discovery) measures whether people searching your category &#8212; without knowing your brand &#8212; can find you.</p>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">AI Overview Readiness</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: Ahrefs AIO Citations (2026) + Kirsten et al. (ACL 2026) + IMC 2026 + Google documentation</div>
  <div class="fw-desc">About 38% of AI Overview citations now come from top-10 organic pages (Ahrefs, March 2026; 76% in 2025), on average 53% of the domains AI Overviews consult are outside the organic top 10, and about 30% of cited domains aren't on page one. Evaluates organic presence, content format alignment with AIO-preferred informational queries, and answer prominence (front-loaded concise answers). Google says a page needs only to be indexed and eligible for a snippet, with no special files or schema; structured data counts little, as schema showed no measurable citation lift on live engines. AIOs favor long-tail keywords: 82% appear for terms with &lt;1,000 monthly searches.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Crawl Accessibility</span><span class="fw-weight">20%</span></div>
  <div class="fw-source">Source: OpenAI crawler docs + Google documentation + Cloudflare (2026) + Lopez-Fonseca et al. (2026)</div>
  <div class="fw-desc">Crawlers differ by purpose: GPTBot and ClaudeBot collect training data; OAI-SearchBot, Claude-SearchBot, PerplexityBot, Googlebot and Bingbot fetch pages for AI answers and search; ChatGPT-User fetches pages for a user and may not follow robots.txt. Google-Extended controls Gemini training and grounding and, Google says, doesn't affect inclusion in Google Search, which AI Overviews draw on. Blocking training crawlers is a business choice and costs no points; blocking a major search crawler caps the score. robots.txt states intent but isn't enforcement &#8212; Gemini and Copilot fetched disallowed pages in a 2026 study &#8212; and mixed-use crawlers (over 36% of crawler activity) make clean training-only blocks hard. Cloudflare blocks AI crawlers by default on new domains since 15 September 2026, so check its setting; Google plans an AIPREF training opt-out in early 2027. Also evaluates sitemap completeness and render accessibility.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Brand Search Momentum</span><span class="fw-weight">30%</span></div>
  <div class="fw-source">Source: Ahrefs 75K-Brand Study (2025, 2026 update) + Google Trends</div>
  <div class="fw-desc">Across 75,000 brands, YouTube mentions (about 0.74) and branded web mentions (0.66-0.71) correlate most strongly with AI visibility, more than Domain Rating (0.27-0.33) or brand search volume (0.334). Correlation, not causation. Winner-takes-all: top 25% brands average 169 AIO mentions vs. 14 for the 50th-75th percentile. Evaluates brand search trends, web mentions, entity recognition, and competitive positioning.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Content Freshness</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: Ahrefs 17M Citations Study (2025) + Seer Interactive (2026) + arXiv Recency Bias (2025)</div>
  <div class="fw-desc">AI assistants cite content 25.7% newer than traditional search, and how recently key commercial and comparison pages (product, pricing, comparisons) were updated matters more than the site's average content age, so those pages weigh most. Freshness signals can move items up to 95 ranking positions in LLM reranking. Evaluates update dates of key pages, update frequency, freshness signals (dates, last-modified), and content decay risk. Note: Google AIOs counter-trend, preferring older authoritative content.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Category Discovery</span><span class="fw-weight">20% (when categories available)</span></div>
  <div class="fw-source">Source: Brand Intelligence categories + target queries</div>
  <div class="fw-desc">When Brand Intelligence provides category keywords and intent queries, a fifth pillar evaluates how visible the brand is in category-level searches &#8212; queries where users search the category without knowing the brand. This measures discovery potential: does the brand appear when someone searches "best [category] tools" or "[use case] solutions"? Sub-metrics include category visibility, intent coverage, competitor gap, and discovery potential. Weights rebalance to 20/15/25/20/20 across all five pillars.</div>
</div>
</section>

<!-- Key Research Findings -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 001.5-.189m-1.5.189a6.01 6.01 0 01-1.5-.189m3.75 7.478a12.06 12.06 0 01-4.5 0m3.75 2.383a14.406 14.406 0 01-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 10-7.517 0c.85.493 1.509 1.333 1.509 2.316V18"/></svg>
  Key Research Findings
</h3>

<a class="finding" href="#chen-koudas">
  <div class="finding-title">Fit to the question beats rewriting tricks</div>
  <div class="finding-detail">The 2023 GEO lab study found adding quotations (+41%) or statistics (+33%) raised visibility. C-SEO Bench (2025), a 2026 survey of 45 studies and a re-test on ten current engine families found these levers don't move citation on live engines. Within a website, pages whose title and key paragraph match the question people ask were cited about 14-16% more per standard deviation.</div>
  <div class="finding-source">C-SEO Bench 2025 + GEO survey + Bajemon &amp; Rochet + Chen &amp; Koudas, 2026</div>
</a>

<a class="finding" href="#chen-koudas">
  <div class="finding-title">FAQ blocks and summary boxes don't help by themselves</div>
  <div class="finding-detail">Across websites, FAQ blocks, summary boxes and schema all looked helpful; comparing pages of the same website, none had a significant effect. Which pages the engine's own search retrieved explained 32% of the variation in citations, page features 2.8%.</div>
  <div class="finding-source">Chen &amp; Koudas (University of Toronto), 2026</div>
</a>

<a class="finding" href="#zhang-absorption">
  <div class="finding-title">Being cited isn't the same as shaping the answer</div>
  <div class="finding-detail">ChatGPT cited fewer sources than Google or Perplexity (6.88 against 12.06 and 16.35 per prompt), but the pages it cited had far more influence on the answer. Pages with figures, definitions, comparisons or how-to steps had 41-62% more influence; Q&amp;A formatting had 5.7% less (a descriptive measure).</div>
  <div class="finding-source">Zhang, He, Yao, 2026</div>
</a>

<a class="finding" href="#tannenbaum-stage">
  <div class="finding-title">Third parties dominate citations, but owned pages still matter</div>
  <div class="finding-detail">Independent sources make up 72-92% of AI citations, and 85.7% of citations in brand answers across 13 languages; much of that is other companies' pages and listicles. Yet in one vendor panel a brand was named in about 3% of searched answers when neither it nor its site came up, and about half the time when one of its own pages was cited.</div>
  <div class="finding-source">GEO Toronto 2025 + Zatuchin + Tannenbaum (vendor data), 2026</div>
</a>

<a class="finding" href="#monaco-popular">
  <div class="finding-title">Assistants name brands they already know</div>
  <div class="finding-detail">ChatGPT wrote a brand into its own searches 67% of the time when it already knew the brand, against 11% otherwise, and 46% of its unprompted brand mentions had no supporting page. The most prominent brands are cited 1.4-2.8 times as often as the average brand, and rewrites don't close the gap. In one vendor panel, household names appeared in 73% of category answers, mid-market brands in 44% and niche brands in 11%: compare a brand with peers of similar size.</div>
  <div class="finding-source">Chen &amp; Koudas + Monaco et al. + Kumar (vendor), 2026</div>
</a>

<a class="finding" href="#unusual-failure-modes">
  <div class="finding-title">Brands miss at one of three stages</div>
  <div class="finding-detail">When two providers both left a brand out, they agreed 95% of the time on why: it never reached the model, reached it but wasn't mentioned, or was mentioned but not positioned as the pick. Category leaders reach nearly every retrieval but win only 25-41% of the slots; about half of specialist and regional brands never surface.</div>
  <div class="finding-source">Jack et al. (unusual.ai), 2026</div>
</a>

<a class="finding" href="#nanoknow">
  <div class="finding-title">Training data and retrieval work together</div>
  <div class="finding-detail">On small open models, accuracy depends strongly on how often an answer appears in the pre-training data; external evidence narrows the gap, and models stay more accurate on answers they saw in training.</div>
  <div class="finding-source">NanoKnow, SIGIR 2026</div>
</a>

<a class="finding" href="#meltwater-oct-2026">
  <div class="finding-title">YouTube leads in Google's AI surfaces</div>
  <div class="finding-detail">YouTube accounts for 24.5% of AI Overview citations (Ahrefs, November 2026) and leads in aggregate across assistants, mainly through Google's AI Overviews; Wikipedia leads in Claude and Copilot (Meltwater, October 2026). In 2025 data, views and subscriber counts did not predict AI citation.</div>
  <div class="finding-source">Ahrefs + Meltwater, 2026</div>
</a>

<a class="finding" href="#livecc">
  <div class="finding-title">Video LLMs are trained on transcripts, not visual content</div>
  <div class="finding-detail">A 7B model trained on YouTube transcripts outperformed 72B models. No captions = invisible to LLMs. Transcript quality is the dominant factor, not production value.</div>
  <div class="finding-source">LiveCC, CVPR 2025</div>
</a>

<a class="finding" href="#lost-in-the-middle">
  <div class="finding-title">Content position follows a U-shaped attention curve</div>
  <div class="finding-detail">LLMs reliably use content at the beginning and end of their context window but effectively ignore the middle. Front-loading key information is critical for citation.</div>
  <div class="finding-source">Lost in the Middle, TACL 2024</div>
</a>

<a class="finding" href="#imc26-aio">
  <div class="finding-title">AI answers aren't reliably grounded</div>
  <div class="finding-detail">11% of AI Overview claims in general queries weren't supported by their sources (IMC 2026), against about 43% of cited statements in health answers; 45% of AI assistant answers about news had a significant issue (EBU/BBC). In one vendor dataset, pricing and billing were 12% of checked claims but 24% of the inaccurate ones.</div>
  <div class="finding-source">IMC 2026 + Agnoli et al. 2026 + EBU/BBC + Profound</div>
</a>

<a class="finding" href="#ahrefs-chatgpt-cites">
  <div class="finding-title">Reddit is read far more than it is cited</div>
  <div class="finding-detail">Reddit's share of AI citations fell about 50% between October 2025 and January 2026, and per-engine figures disagree: 6.2% of ChatGPT citations in Ahrefs' November 2026 set, about 0.4% in Meltwater's October 2026 data, where it was Gemini's top domain until Gemini's Reddit citations fell 88%. But Reddit made up 67.8% of the pages ChatGPT retrieved and didn't cite, so citation share understates its influence.</div>
  <div class="finding-source">Conductor + Ahrefs + Meltwater, 2026</div>
</a>

<a class="finding" href="#reddit-community-consensus">
  <div class="finding-title">Community consensus creates unique credibility signals</div>
  <div class="finding-detail">Upvoted comment threads, especially "best X for Y" recommendation discussions, create multi-user validation that shapes how models describe a category. This multi-user signal cannot be replicated by single-author content.</div>
  <div class="finding-source">Bluefish Labs / Emberos, 2025</div>
</a>

<a class="finding" href="#ray-listicles">
  <div class="finding-title">Self-ranked "best X" lists can help competitors</div>
  <div class="finding-detail">Self-promotional listicles still drew about 11% of citations in software reviews (3.6% in ChatGPT), but when a brand's own "best X" list was cited in AI Overviews, the brand was left out of the recommendation 69% of the time. Third-party lists are what count.</div>
  <div class="finding-source">Peec (vendor) + Lily Ray, 2026</div>
</a>

<a class="finding" href="#ahrefs-aio-top10-2026">
  <div class="finding-title">AI Overviews rely less on top-ranked pages</div>
  <div class="finding-detail">About 38% of AI Overview citations come from top-10 organic pages (Ahrefs, March 2026; 76% in 2025), and on average 53% of the domains AI Overviews consult are outside the organic top 10 (ACL 2026). Google, AI Overviews and Gemini share few sources (SIGIR 2026), and standalone assistants overlap with Google's top 10 only 12% of the time.</div>
  <div class="finding-source">Ahrefs + Kirsten et al. + SIGIR, 2026</div>
</a>

<a class="finding" href="#ahrefs-75k-2026">
  <div class="finding-title">Brand mentions correlate most with AI visibility</div>
  <div class="finding-detail">Across 75,000 brands, YouTube mentions (about 0.74) and branded web mentions (0.66-0.71) correlate most strongly with AI visibility, Domain Rating far less (0.27-0.33). These are correlations, not causes.</div>
  <div class="finding-source">Ahrefs 75K Brands, 2026</div>
</a>

<a class="finding" href="#seer-recency-2026">
  <div class="finding-title">AI assistants prefer fresh content</div>
  <div class="finding-detail">Content cited by AI assistants is 25.7% newer on average than traditional search results, and how recently key commercial and comparison pages were updated matters more than a site's average content age.</div>
  <div class="finding-source">Ahrefs (17M citations) + Seer Interactive, 2026</div>
</a>

<a class="finding" href="#robots-respect">
  <div class="finding-title">Crawler purpose matters, and robots.txt is a request</div>
  <div class="finding-detail">GPTBot and ClaudeBot collect training data; OAI-SearchBot powers ChatGPT search; ChatGPT-User may not follow robots.txt. In a controlled study, Claude and Mistral obeyed robots.txt while Gemini and Copilot fetched disallowed pages. Training is 52% of crawler requests and mixed-use crawlers over 36%, and Cloudflare blocks AI crawlers by default on new domains.</div>
  <div class="finding-source">OpenAI docs + Lopez-Fonseca et al. + Cloudflare, 2026</div>
</a>

<a class="finding" href="#google-crawlers">
  <div class="finding-title">Google-Extended doesn't affect Google Search</div>
  <div class="finding-detail">Google says Google-Extended controls Gemini training and grounding and doesn't affect inclusion or ranking in Google Search; AI Overviews need only an indexed page eligible for a snippet, with no special files or schema. Sites blocking Google's AI crawler appear less in AI Overview retrieval (SIGIR 2026), but blocking sites are disproportionately large, reputable publishers. Google plans an AIPREF training opt-out in early 2027.</div>
  <div class="finding-source">Google documentation + SIGIR 2026 + WWW 2026</div>
</a>

<a class="finding" href="#ahrefs-schema-2026">
  <div class="finding-title">Schema and llms.txt don't drive AI citations</div>
  <div class="finding-detail">In a controlled test, schema markup gave no meaningful lift in AI citations on live engines (Ahrefs, May 2026), and 97% of llms.txt files got no requests from AI crawlers (Ahrefs, June 2026). Schema still helps rich results and entity understanding, and helped retrieval in one simulated pipeline (KDD 2026).</div>
  <div class="finding-source">Ahrefs + SAGEO Arena, 2026</div>
</a>

<a class="finding" href="#kirsten-acl26">
  <div class="finding-title">AI answers vary from run to run</div>
  <div class="finding-detail">Even at temperature 0, 9-27% of yes/no answers flipped within five minutes; AI tools returned the same list of brands less than 1% of the time; only 18% of AI Overview pages were the same two months later. Brand-level rates over many questions are reliable after a few runs, but a single-question difference needs about 15-25 runs to detect reliably.</div>
  <div class="finding-source">Kirsten et al. (ACL 2026) + SparkToro + Dice Roll Method, 2026</div>
</a>

<a class="finding" href="#zatuchin-sentiment">
  <div class="finding-title">Sentiment is the least stable measure</div>
  <div class="finding-detail">Sentiment flipped between positive and negative about 6.7 times as often as mention did (vendor panel), and automated sentiment scores varied far more with the query's language (26% of variance) than with the brand (1.5%). LLM Optimizer reports it separately from the LLM Test score.</div>
  <div class="finding-source">Kumar (vendor) + Zatuchin, 2026</div>
</a>

<a class="finding" href="#uberti-product-audit">
  <div class="finding-title">APIs and consumer apps cite different sources</div>
  <div class="finding-detail">For the same product question, a chatbot's API and its consumer app shared only 12% (ChatGPT) to 15% (Gemini) of cited domains, and the ChatGPT and Gemini apps shared 5.4%. API tests describe the models, not exactly what app users see.</div>
  <div class="finding-source">Uberti-Bona Marin et al., 2026</div>
</a>

<a class="finding" href="#wang-ai-mode-rct">
  <div class="finding-title">AI answers take clicks</div>
  <div class="finding-detail">When an AI Overview appears, the top organic result's click-through rate falls about 58% (Ahrefs). Field experiments found AI Overviews cut organic clicks by 38%, and an AI Mode-only search cut click-through by 18.8 percentage points (21.2 for clicks to Reddit). Visibility increasingly means being named, not visited.</div>
  <div class="finding-source">Ahrefs + Agarwal &amp; Sen + Wang et al., 2026</div>
</a>

<a class="finding" href="#resoneo-citation-collapse">
  <div class="finding-title">Model updates can collapse citation volume overnight</div>
  <div class="finding-detail">When GPT-5.3 replaced GPT-4o as the default model, unique domains cited per response dropped 20.5% and unique URLs dropped 21%. The study formalizes two visibility types: parametric (from training data) and dynamic (from real-time retrieval), and shows that model updates can sharply reduce the latter.</div>
  <div class="finding-source">Resoneo, 2026</div>
</a>

<a class="finding" href="#dejan-brand-authority">
  <div class="finding-title">Association with category leaders can outweigh mention frequency</div>
  <div class="finding-detail">A 200,000-query study of LLM parametric memory found that brands densely associated with high-authority peers can rank higher than brands with more raw mentions. Being mentioned alongside the right brands can matter more than being mentioned often.</div>
  <div class="finding-source">Dejan AI, 2026</div>
</a>
</section>

<!-- CTA -->
<section>
<div class="cta">
  <h3>Put this research to work</h3>
  <p>LLM Optimizer applies these research findings automatically to analyze and optimize your brand's visibility across AI search engines.</p>
  <div class="cta-buttons">
    <a href="/signup" class="btn-primary">
      Get Started
      <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3"/></svg>
    </a>
    <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noopener noreferrer" class="btn-secondary">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
      Self-Host Free
    </a>
  </div>
  <p class="cta-note">LLM Optimizer is open-source (MIT). Our hosted version supports ongoing development.</p>
</div>
</section>
</main>

<footer>
<div class="inner">
  <div class="footer-links">
    <a href="https://github.com/metavert-io/llmopt" target="_blank" rel="noopener noreferrer">GitHub (MIT License)</a>
    <span class="footer-sep">&middot;</span>
    <a href="https://www.metavert.io/privacy-policy" target="_blank" rel="noopener noreferrer">Privacy Policy</a>
    <span class="footer-sep">&middot;</span>
    <a href="https://www.metavert.io/terms-of-service" target="_blank" rel="noopener noreferrer">Terms of Service</a>
    <span class="footer-sep">&middot;</span>
    <a href="/docs">API Docs</a>
    <span class="footer-sep">&middot;</span>
    <a href="/api/v1/docs" target="_blank" rel="noopener noreferrer">API Reference (MD)</a>
  </div>
  <p class="footer-copy">&copy; 2026 Metavert LLC. All content licensed under <a href="https://creativecommons.org/licenses/by/4.0/" target="_blank" rel="noopener noreferrer">Creative Commons Attribution 4.0</a>.</p>
</div>
</footer>

</body>
</html>`
