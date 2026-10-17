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
  <p><strong>Brand Recognition vs. Discovery.</strong> A key framework throughout LLM Optimizer is the distinction between <em>brand recognition</em> &#8212; how well AI represents your brand when people search for it by name &#8212; and <em>inbound discovery</em> &#8212; how often AI surfaces your brand when people search your category without prior knowledge of you. Both matter, but they require different strategies. Brand recognition improves through authority signals, earned media, and training data presence. Discovery requires appearing in category-level content, answering the questions your audience asks before they know you exist, and being present in the YouTube videos, Reddit threads, and web pages that LLMs cite for category queries.</p>
  <p>The emerging science of LLM visibility reveals a fundamental shift in how information gains authority online. The most significant recent finding comes from <a href="#nanoknow">NanoKnow (2026)</a>, which demonstrates that content appearing frequently in training data more than doubles a model's accuracy on related questions &#8212; and that the advantage compounds when content is both memorized during training and retrievable at inference time. This means the traditional SEO playbook of optimizing for a single ranking algorithm is being replaced by a dual imperative: getting into training corpora through widespread, high-quality publication, while simultaneously remaining citable through structured, authoritative web presence.</p>
  <p>Across the research, a consistent pattern emerges: AI search engines overwhelmingly favor earned media over brand-owned content, citing third-party sources <a href="#geo-toronto">72-92% of the time</a>. Content that includes quotations from authoritative sources gains <a href="#geo-princeton">+41% visibility</a> &#8212; the single most effective optimization technique identified. Meanwhile, YouTube has rapidly become the dominant social citation source for LLMs, with its <a href="#youtube-citations">share doubling to 39%</a> between August and December 2024. Critically, <a href="#livecc">video LLMs process content through transcripts</a>, not visual analysis &#8212; a 7B model trained on YouTube transcripts outperformed 72B models, proving that transcript quality matters far more than production value.</p>
  <p>Reddit has emerged as the <a href="#youtube-citations">#2 social citation source for LLMs</a>, with unique authority dynamics. Reddit was foundational in LLM training through datasets like <a href="#reddit-webtext">WebText</a> and the <a href="#reddit-common-crawl">Common Crawl</a>, and continues through <a href="#reddit-data-deals">$60M (Google) and $70M (OpenAI) annual licensing deals</a>. Unlike YouTube's channel-centric authority, Reddit's influence comes from <a href="#reddit-community-consensus">multi-user validation</a> &#8212; upvoted comment consensus, especially in "best X for Y" recommendation threads, creates credibility signals that LLMs weight heavily. The <a href="#geo-toronto">Toronto GEO paper</a> classifies Reddit as "Social" &#8212; a category AI search engines suppress in direct citations &#8212; yet Reddit's pervasive presence in training data means it heavily shapes baseline model knowledge even when not explicitly cited.</p>
  <p>A critical "two-world" split has emerged between Google AI Overviews and standalone LLMs. <a href="#ahrefs-aio-citations">76% of AI Overview citations</a> pull from top-10 organic pages &#8212; making traditional search rankings the primary signal for AIO inclusion. But for standalone LLMs like ChatGPT, <a href="#ahrefs-ai-search-overlap">only 12% of cited URLs rank in Google's top 10</a>. The <a href="#ahrefs-75k-brands">strongest predictor of AI citation across platforms is YouTube mentions (0.737 correlation)</a>, followed by web mentions (0.664) &#8212; not backlinks. Meanwhile, content freshness has become a significant signal: AI assistants cite content that is <a href="#ahrefs-freshness">25.7% newer</a> than traditional search results, and <a href="#seer-recency">65% of AI bot crawl hits target content less than a year old</a>. The <a href="#cloudflare-ai-crawlers">explosive growth of AI crawlers (GPTBot up 305% YoY)</a> makes robots.txt policy a direct lever for AI visibility.</p>
  <p>However, this new landscape comes with important caveats. Citation accuracy across AI answer engines remains <a href="#false-promise">surprisingly poor (49-68%)</a>, with nearly a third of claims lacking any source backing. Citation concentration follows power-law dynamics, where the <a href="#news-citing-patterns">top 20 sources capture 28-67% of all citations</a>. And LLMs exhibit strong <a href="#lost-in-the-middle">positional bias</a>, reliably attending to content at the beginning and end of context while ignoring the middle.</p>
  <p>Compounding these challenges, <a href="#resoneo-citation-collapse">model updates can sharply reduce citation volume</a>. When GPT-5.3 replaced GPT-4o as ChatGPT's default, unique domains cited per response dropped 20.5% overnight &#8212; meaning brands that had achieved dynamic visibility through real-time retrieval lost it without any change on their end. This volatility reinforces the importance of <em>parametric</em> visibility (being embedded in training data) alongside dynamic visibility (being citable at inference time). Research into <a href="#dejan-brand-authority">LLM parametric memory</a> reveals that network centrality &#8212; being densely associated with high-authority brands in a model's knowledge graph &#8212; outweighs raw mention frequency. A brand that appears alongside category leaders in training data gains disproportionate visibility, even if it is mentioned less often overall. Together, these findings inform LLM Optimizer's scoring frameworks across answer optimization, video authority, Reddit authority, and search visibility analysis.</p>
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
  <div class="paper-desc">Tested 9 content optimization strategies on 10,000 queries. Quotations (+41%), statistics (+33%), and fluency (+29%) are the most effective methods for improving LLM citation visibility.</div>
</a>

<a id="nanoknow" class="paper" href="https://arxiv.org/abs/2602.20122" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">NanoKnow: Probing LLM Knowledge by Linking Training Data to Answers</div>
  <div class="paper-venue">2026</div>
  <div class="paper-desc">Training data frequency more than doubles model accuracy. Even with oracle RAG, models score ~11 points higher on questions with answers in training data.</div>
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
  <div class="paper-desc">Citation accuracy ranges 49-68% across answer engines. 23-32% of claims have no source backing. Perplexity generates one-sided answers 83.4% of the time.</div>
</a>

<a id="reddit-webtext" class="paper" href="https://cdn.openai.com/better-language-models/language_models_are_unsupervised_multitask_learners.pdf" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Language Models are Unsupervised Multitask Learners</div>
  <div class="paper-venue">OpenAI, 2019 (Radford et al.)</div>
  <div class="paper-desc">Introduced WebText, a dataset of 8 million Reddit posts with 3+ karma score, as the foundational training corpus for GPT-2. Demonstrated that Reddit's community curation mechanism (karma voting) effectively serves as a quality filter for large-scale language model training data.</div>
</a>

<a id="reddit-common-crawl" class="paper" href="https://dl.acm.org/doi/10.1145/3630106.3659033" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Consent in Crisis: The Rapid Decline of the AI Data Commons</div>
  <div class="paper-venue">ACM FAccT 2024 (Longpre et al.)</div>
  <div class="paper-desc">Comprehensive audit of AI training data sources documenting Reddit's persistent prominence in Common Crawl and other web corpora. Found that robots.txt restrictions increased 25%+ from 2023-2024 as sites restricted AI crawling, while Reddit data remained broadly available through licensing agreements.</div>
</a>

<a id="reddit-data-deals" class="paper" href="https://www.reuters.com/technology/reddit-ai-content-licensing-deal-google-2024-02-22/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Reddit Data Licensing: Google and OpenAI Deals</div>
  <div class="paper-venue">Reuters / The Verge, 2024</div>
  <div class="paper-desc">Google pays $60M/year and OpenAI $70M/year for Reddit data access. Reddit's API was locked down in 2023. Active litigation: Reddit v. Anthropic, Reddit v. Perplexity (scraping claims).</div>
</a>

<a id="reddit-community-consensus" class="paper" href="https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Community Consensus as LLM Authority Signal</div>
  <div class="paper-venue">Bluefish Labs / Emberos Research, 2025</div>
  <div class="paper-desc">Reddit's multi-user validation (upvotes, comment consensus) creates credibility signals single-author content cannot match. "Best X for Y" recommendation threads are among the most influential for LLM comparison queries.</div>
</a>

<a id="ahrefs-aio-citations" class="paper" href="https://ahrefs.com/blog/search-rankings-ai-citations/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Overview Citations and Search Rankings</div>
  <div class="paper-venue">Ahrefs, 2025</div>
  <div class="paper-desc">76% of AI Overview citations pull from top-10 organic pages. Median organic ranking for a cited URL is position 3. 86% of citations come from within the top 100 organic results.</div>
</a>

<a id="ahrefs-ai-search-overlap" class="paper" href="https://ahrefs.com/blog/ai-search-overlap/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Search Overlap: How AI Citations Differ from Google</div>
  <div class="paper-venue">Ahrefs, 2025</div>
  <div class="paper-desc">Only 12% of standalone LLM citations overlap with Google's top 10. Perplexity shows 28.6% overlap. 80%+ of ChatGPT/Claude/Gemini citations come from pages not ranking in Google at all.</div>
</a>

<a id="ahrefs-75k-brands" class="paper" href="https://ahrefs.com/blog/ai-brand-visibility-correlations/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">AI Brand Visibility Correlations (75K Brands)</div>
  <div class="paper-venue">Ahrefs, 2025</div>
  <div class="paper-desc">YouTube mentions (0.737) and web mentions (0.664) are the strongest correlators with AI visibility. Brand search volume (0.334) outperforms backlinks (0.37). Top 25% brands get 12x more AIO mentions.</div>
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
  <div class="paper-desc">Reddit (40.1%) and Wikipedia (26.3%) dominate AIO citations. 80% of AIO responses target informational queries. 82% appear for keywords with &lt;1,000 monthly searches.</div>
</a>

<a id="resoneo-citation-collapse" class="paper" href="https://think.resoneo.com/chatgpt/5.3-5.4/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">ChatGPT Search Visibility: GPT-5.3/5.4 Citation Analysis</div>
  <div class="paper-venue">Resoneo, 2026</div>
  <div class="paper-desc">27,000 responses across 400 prompts over 14 weeks. After GPT-5.3 launched, unique domains cited per response dropped 20.5% (19.1 &#8594; 15.2) and unique URLs dropped 21.0% (24.1 &#8594; 19.1). Formalizes the distinction between parametric visibility (training data knowledge) and dynamic visibility (real-time web retrieval).</div>
</a>

<a id="dejan-brand-authority" class="paper" href="https://think.resoneo.com/chatgpt/5.3-5.4/" target="_blank" rel="noopener noreferrer">
  <div class="paper-title">Brand Authority Index: Network Centrality in LLM Parametric Memory</div>
  <div class="paper-venue">Dejan AI / Resoneo, 2026</div>
  <div class="paper-desc">Queried Gemini 200,000 times across ~20 million brand mentions, building a 2.9 million-node directed association graph. Found that network centrality &#8212; being densely associated with high-authority brands &#8212; outweighs raw mention frequency for parametric visibility. A brand with zero spontaneous recall ranked highest due to dense intersections with authority brands.</div>
</a>
</section>

<!-- Answer Optimization Framework -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 3v11.25A2.25 2.25 0 006 16.5h2.25M3.75 3h-1.5m1.5 0h16.5m0 0h1.5m-1.5 0v11.25A2.25 2.25 0 0118 16.5h-2.25m-7.5 0h7.5m-7.5 0l-1 3m8.5-3l1 3m0 0l.5 1.5m-.5-1.5h-9.5m0 0l-.5 1.5"/></svg>
  Answer Optimization Scoring Framework
</h3>
<p class="section-desc">Each optimization report scores how likely an LLM is to surface and cite a website's answer across four research-backed dimensions.</p>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Content Authority</span><span class="fw-weight">30%</span></div>
  <div class="fw-source">Source: GEO (Princeton/KDD 2024)</div>
  <div class="fw-desc">Measures the presence of quotations from authoritative sources (+41% visibility), statistical evidence (+33%), source citations (+28%), fluency (+29%), and technical terminology (+19%). Penalizes keyword stuffing (-9%).</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Structural Optimization</span><span class="fw-weight">20%</span></div>
  <div class="fw-source">Source: Lost in the Middle (TACL 2024) + GEO (Toronto 2025)</div>
  <div class="fw-desc">Evaluates answer prominence (front-loaded vs. buried), content conciseness, machine-readable structure (Schema.org, tables, comparison formats), and justification language that explains "why" rather than just "what."</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Source Authority</span><span class="fw-weight">30%</span></div>
  <div class="fw-source">Source: GEO (Toronto 2025)</div>
  <div class="fw-desc">Assesses third-party coverage and earned media presence. AI search engines cite earned media 72-92% of the time. Evaluates cross-engine consistency since different AI providers cite substantially different sources (similarity only 0.11-0.58).</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Knowledge Persistence</span><span class="fw-weight">20%</span></div>
  <div class="fw-source">Source: NanoKnow (2026)</div>
  <div class="fw-desc">Measures how deeply information is embedded in model training data. Answer frequency more than doubles accuracy. Content that is both in training data AND retrievable at inference compounds advantage by ~11 percentage points. Clear, educational writing outperforms natural text by 19+ points.</div>
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
  <div class="fw-source">Source: LiveCC (CVPR 2025) + GEO (Princeton 2024)</div>
  <div class="fw-desc">Transcript quality is the dominant signal for LLM visibility. Evaluates keyword alignment, quotability (standalone citable statements get +41% visibility per GEO), information density, and caption availability. Videos without captions are effectively invisible to LLMs.</div>
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
  <div class="fw-source">Source: False Promise of Source-Cited Responses (2024) + Lost in the Middle (2024)</div>
  <div class="fw-desc">Evaluates sentiment, mention context and position (early mentions get priority per U-shaped attention), extractability (clear mentions are less likely to be misrepresented given 49-68% citation accuracy), and narrative coherence. Includes a confidence discount reflecting known citation inaccuracy rates.</div>
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
  <div class="fw-source">Source: NanoKnow (2026) + Reddit Data Licensing (2024)</div>
  <div class="fw-desc">Likelihood that Reddit discussions will influence LLM training. High-upvote threads in authoritative subreddits with deep comment engagement create the strongest training signals. Reddit data is actively licensed to OpenAI and Google.</div>
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
  <div class="fw-header"><span class="fw-name">AI Overview Readiness</span><span class="fw-weight">30%</span></div>
  <div class="fw-source">Source: Ahrefs AIO Citations Study (2025) + Semrush AIO Study (2025)</div>
  <div class="fw-desc">76% of AI Overview citations pull from top-10 organic pages. Evaluates organic ranking presence, structured data (Schema.org, JSON-LD), content format alignment with AIO-preferred informational queries, and answer prominence (front-loaded concise answers). AIOs favor long-tail keywords &#8212; 82% appear for terms with &lt;1,000 monthly searches.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Crawl Accessibility</span><span class="fw-weight">20%</span></div>
  <div class="fw-source">Source: Cloudflare AI Crawler Report (2025) + Consent in Crisis (ACM FAccT 2024)</div>
  <div class="fw-desc">GPTBot grew 305% YoY with a crawl-to-referral ratio of 1,700:1. Evaluates robots.txt policy for AI crawlers (GPTBot, ClaudeBot, PerplexityBot and their SearchBot variants), sitemap completeness, and render accessibility. Blocking training bots while allowing search bots is a valid strategy; blocking everything eliminates AI visibility.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Brand Search Momentum</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: Ahrefs 75K-Brand Study (2025) + Google Trends API (2025)</div>
  <div class="fw-desc">Brand search volume has a 0.334 correlation with AI citation frequency &#8212; but web mentions (0.664) and YouTube mentions (0.737) are stronger. Winner-takes-all: top 25% brands average 169 AIO mentions vs. 14 for the 50th-75th percentile. Evaluates brand search trends, entity recognition, and competitive positioning.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Content Freshness</span><span class="fw-weight">25%</span></div>
  <div class="fw-source">Source: Ahrefs 17M Citations Study (2025) + Seer Interactive (2025) + arXiv Recency Bias (2025)</div>
  <div class="fw-desc">AI assistants cite content 25.7% newer than traditional search. 65% of AI bot hits target content &lt;1 year old. Freshness signals can move items up to 95 ranking positions in LLM reranking. Evaluates content age, update frequency, freshness signals (dates, last-modified), and content decay risk. Note: Google AIOs counter-trend, preferring older authoritative content.</div>
</div>

<div class="fw-card">
  <div class="fw-header"><span class="fw-name">Category Discovery</span><span class="fw-weight">20% (when categories available)</span></div>
  <div class="fw-source">Source: Brand Intelligence categories + target queries</div>
  <div class="fw-desc">When Brand Intelligence provides category keywords and intent queries, a fifth pillar evaluates how visible the brand is in category-level searches &#8212; queries where users search the category without knowing the brand. This measures discovery potential: does the brand appear when someone searches "best [category] tools" or "[use case] solutions"? Sub-metrics include category visibility, intent coverage, competitor gap, and discovery potential. Weights rebalance to 25/15/20/20/20 across all five pillars.</div>
</div>
</section>

<!-- Key Research Findings -->
<section>
<h3>
  <svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 001.5-.189m-1.5.189a6.01 6.01 0 01-1.5-.189m3.75 7.478a12.06 12.06 0 01-4.5 0m3.75 2.383a14.406 14.406 0 01-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 10-7.517 0c.85.493 1.509 1.333 1.509 2.316V18"/></svg>
  Key Research Findings
</h3>

<a class="finding" href="#geo-princeton">
  <div class="finding-title">Quotations are the single most effective optimization method</div>
  <div class="finding-detail">Adding quotes from authoritative sources improves LLM visibility by 41%, more than any other technique tested on 10,000 queries. Statistics (+33%) and fluency (+29%) follow.</div>
  <div class="finding-source">GEO, Princeton/KDD 2024</div>
</a>

<a class="finding" href="#geo-princeton">
  <div class="finding-title">Lower-ranked sites benefit disproportionately</div>
  <div class="finding-detail">Rank-5 sites saw +115% visibility improvement from citing sources, while rank-1 sites saw -30%. Generative engines can be more democratic than traditional search for well-optimized content.</div>
  <div class="finding-source">GEO, Princeton/KDD 2024</div>
</a>

<a class="finding" href="#geo-toronto">
  <div class="finding-title">AI search overwhelmingly favors earned media</div>
  <div class="finding-detail">AI search engines cite independent third-party sources 72-92% of the time, compared to only 18-27% for brand-owned content and virtually 0% for social content.</div>
  <div class="finding-source">GEO, Toronto 2025</div>
</a>

<a class="finding" href="#nanoknow">
  <div class="finding-title">Training data frequency more than doubles accuracy</div>
  <div class="finding-detail">Models are more than twice as accurate on questions whose answers appear frequently (51+ documents) in training data vs. rarely (1-5 documents). Being in training data AND retrievable compounds advantage.</div>
  <div class="finding-source">NanoKnow, 2026</div>
</a>

<a class="finding" href="#youtube-citations">
  <div class="finding-title">YouTube is the #1 social citation source for LLMs</div>
  <div class="finding-detail">YouTube's share of social citations doubled from 18.9% to 39.2% in just 5 months. It generates 18x more AI citations than Instagram and 50x more than TikTok. Views and subscriber counts do not predict AI citation.</div>
  <div class="finding-source">Adweek / Bluefish / Emberos / Goodie AI, 2025</div>
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

<a class="finding" href="#false-promise">
  <div class="finding-title">AI citation accuracy is surprisingly poor</div>
  <div class="finding-detail">Perplexity achieves only 49% citation accuracy; You.com 68%; BingChat 66%. 23-32% of relevant statements have no source backing. Systems display more sources than they actually use.</div>
  <div class="finding-source">False Promise of Source-Cited Responses, 2024</div>
</a>

<a class="finding" href="#reddit-webtext">
  <div class="finding-title">Reddit is the #2 social citation source and foundational training data</div>
  <div class="finding-detail">Reddit accounts for 10-40% of AI social citations depending on platform/timeframe. WebText (GPT-2 training) was built from 8M Reddit posts with 3+ karma. Reddit remains pervasive in Common Crawl and is actively licensed to Google ($60M/yr) and OpenAI ($70M/yr).</div>
  <div class="finding-source">Multiple sources, 2024-2025</div>
</a>

<a class="finding" href="#reddit-community-consensus">
  <div class="finding-title">Community consensus creates unique credibility signals</div>
  <div class="finding-detail">Upvoted comment threads, especially "best X for Y" recommendation discussions, create multi-user validation that LLMs weight heavily. This multi-user signal cannot be replicated by single-author content.</div>
  <div class="finding-source">Bluefish Labs / Emberos, 2025</div>
</a>

<a class="finding" href="#ahrefs-aio-citations">
  <div class="finding-title">AI Overviews strongly favor top-ranked pages</div>
  <div class="finding-detail">76% of Google AI Overview citations come from top-10 organic pages, with median cited position at rank 3. But standalone LLMs (ChatGPT, Claude, Gemini) show only 12% overlap &#8212; they cite fundamentally different sources.</div>
  <div class="finding-source">Ahrefs, 2025</div>
</a>

<a class="finding" href="#ahrefs-75k-brands">
  <div class="finding-title">Web mentions outperform backlinks for AI visibility</div>
  <div class="finding-detail">Brand web mentions (0.664 correlation) and YouTube mentions (0.737) are far stronger predictors of AI citation than backlinks (0.37). Top 25% brands by web mentions get 12x more AI Overview mentions than the 50-75th percentile.</div>
  <div class="finding-source">Ahrefs 75K Brands Study, 2025</div>
</a>

<a class="finding" href="#ahrefs-freshness">
  <div class="finding-title">AI assistants strongly prefer fresh content</div>
  <div class="finding-detail">Content cited by AI assistants is 25.7% newer on average than traditional search results. 65% of AI bot crawl hits target content less than 1 year old. Freshness signals can shift LLM ranking positions by up to 95 places.</div>
  <div class="finding-source">Ahrefs (17M citations) + arXiv, 2025</div>
</a>

<a class="finding" href="#cloudflare-ai-crawlers">
  <div class="finding-title">AI crawlers are growing explosively</div>
  <div class="finding-detail">GPTBot grew 305% YoY, with OpenAI's crawl-to-referral ratio at 1,700:1. Each major AI company now runs 3 separate bots (training, indexing, user-fetch). Blocking training bots while allowing search bots is a valid strategy.</div>
  <div class="finding-source">Cloudflare, 2025</div>
</a>

<a class="finding" href="#resoneo-citation-collapse">
  <div class="finding-title">Model updates can collapse citation volume overnight</div>
  <div class="finding-detail">When GPT-5.3 replaced GPT-4o as the default model, unique domains cited per response dropped 20.5% and unique URLs dropped 21%. The study formalizes two distinct visibility types: parametric (stable, from training data) and dynamic (volatile, from real-time retrieval) &#8212; and shows that model updates can sharply reduce the latter.</div>
  <div class="finding-source">Resoneo, 2026</div>
</a>

<a class="finding" href="#dejan-brand-authority">
  <div class="finding-title">Brand network centrality outweighs raw mention frequency</div>
  <div class="finding-detail">A 200,000-query study of LLM parametric memory found that brands densely associated with high-authority peers rank higher than brands with more raw mentions. A brand with zero spontaneous recall ranked #1 because of its network position among luxury category leaders. Being mentioned alongside the right brands matters more than being mentioned often.</div>
  <div class="finding-source">Dejan AI / Resoneo, 2026</div>
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
