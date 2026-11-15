package main

// Research digest and bibliography shown at the end of every report (both
// the Chromium report and the plain fallback PDF). Kept in step with the
// methodology page (research.go), the in-app methodology (App.tsx) and
// research.md. " -- " renders as a dash.

type bibEntry struct {
	ref   string
	title string
	venue string
	url   string
}

var reportResearchDigest = []string{
	"Brand Recognition vs. Discovery. A key framework throughout LLM Optimizer is the distinction between brand recognition -- how well AI represents your brand when people search for it by name -- and inbound discovery -- how often AI surfaces your brand when people search your category without prior knowledge of you. Both matter, but they require different strategies. Brand recognition improves through authority signals, earned media, and training data presence. Discovery requires appearing in category-level content, answering the questions your audience asks before they know you exist, and being present in the YouTube videos, Wikipedia pages, review sites, comparison lists and community threads that AI assistants draw on for category queries.",
	"The emerging science of LLM visibility distinguishes two ways a brand becomes known to AI. Parametric visibility is what a model learned in training; dynamic visibility is what it finds when it searches the web while answering. NanoKnow (SIGIR 2026) shows, on small open models, that how often an answer appears in the training data strongly affects whether a model knows it, and that retrieved evidence and training knowledge complement each other. Both matter in practice: ChatGPT searched the web for about 34.5% of US prompts in early 2026 and answered the rest from what it had learned. The dual imperative holds: be published widely enough to be learned, and be retrievable and citable when assistants search.",
	"Across the research, AI search engines favor earned media over brand-owned content, citing independent third-party sources 72-92% of the time. Third-party coverage now very much includes video: YouTube is the most-cited domain in Google AI Overviews (24.5%) and across AI assistants, and Wikipedia is ChatGPT's most-cited domain. What has not held up is the 2023 headline that adding quotations raises visibility by 41%: it was a lab result, and C-SEO Bench and a 2026 survey of 45 studies found such rewrites largely ineffective on live engines. The levers that reproduce are relevance -- answering the question that was actually asked -- and prominence -- putting that answer early and plainly. Because video LLMs learn from transcripts, not pictures, what is said in a video matters far more than its production value.",
	"Reddit's role is shifting. It was foundational training data through WebText and the Common Crawl, and Google and OpenAI signed data licensing deals with it in 2024, which are reported to be under renegotiation. Its upvoted consensus, especially in \"best X for Y\" threads, still shapes how models describe a category. But Reddit's share of AI citations fell about 50% between October 2025 and January 2026, and in ChatGPT it is the #2 cited domain (6.2%), behind Wikipedia (10.1%). Genuine participation is worth it; over-investing in Reddit at the expense of other channels is not.",
	"Google AI Overviews and standalone assistants draw on different sources, and AI Overviews now lean less on rankings. About 38% of AI Overview citations come from top-10 organic pages (Ahrefs, March 2026; 76% in its 2025 study, partly because of a change in how results are parsed), about 30% of cited domains are not on page one, and Google's results, AI Overviews and Gemini share few sources. Gemini 3 has powered AI Overviews since January 2026, with a handoff into AI Mode, which passed 1 billion monthly users; when an AI Overview appears, the top organic result loses about 58% of its clicks. For standalone assistants, only 12% of cited URLs rank in Google's top 10. Across 75,000 brands, YouTube mentions (about 0.74) and branded web mentions (0.66-0.71) correlate most strongly with AI visibility and Domain Rating far less (0.27-0.33) -- correlation, not causation. Freshness matters too: AI assistants cite content 25.7% newer than search results do, and how recently key commercial and comparison pages were updated matters more than a site's average content age.",
	"Crawlers now differ by purpose. OpenAI's GPTBot collects training data only; OAI-SearchBot fetches pages for ChatGPT search, and ChatGPT-User, which fetches pages a user asks about, may not follow robots.txt. Blocking training crawlers is a business choice, but blocking search crawlers hides a site from AI answers, and blocking Google's AI crawler is associated with less AI Overview retrieval. Since 15 September 2026 Cloudflare blocks AI crawlers by default on new domains, so a site can be hidden from AI without its owner choosing it. Two popular tactics showed no effect in 2026 tests: schema markup gave no meaningful citation lift, though it still helps rich results and entity understanding, and 97% of llms.txt files got no requests from AI crawlers.",
	"These measurements come with caveats. AI answers are not reliably grounded: 11% of AI Overview claims were not supported by the sources they cited, and 45% of AI assistant answers about news had at least one significant issue. Answers also vary from run to run: asked the same question repeatedly, AI tools returned the same list of brands less than 1% of the time, and in a small study one engine's cited URLs turned over 67% from one day to the next. Visibility is therefore best measured as a distribution over repeated runs, per engine -- how often a brand is mentioned, not where it ranks in a list -- and the figures here describe associations, not causes. Citation still concentrates: the top 20 sources capture 28-67% of citations, and LLMs attend most to the beginning and end of their context, so front-loading answers still pays.",
	"Model updates add volatility. When GPT-5.3 replaced GPT-4o as ChatGPT's default, unique domains cited per response dropped 20.5% overnight -- brands that had achieved dynamic visibility lost some of it without any change on their end. This reinforces the value of parametric visibility alongside dynamic visibility. Research into LLM parametric memory suggests that being associated with category leaders can matter more than raw mention frequency. Together, these findings inform LLM Optimizer's scoring frameworks across answer optimization, video authority, Reddit authority and search visibility, and the LLM Visibility Score weighs the LLM Test -- the direct measure of what assistants actually answer -- as heavily as answer optimization (25% each; video 20%, Reddit 15%, search 15%).",
}

var reportBibliography = []bibEntry{
	{"[1]", "Lost in the Middle: How Language Models Use Long Contexts", "TACL 2024", "https://arxiv.org/abs/2307.03172"},
	{"[2]", "GEO: Generative Engine Optimization", "Princeton / KDD 2024 (2023 lab study)", "https://arxiv.org/abs/2311.09735"},
	{"[3]", "NanoKnow: How to Know What Your Language Model Knows", "SIGIR 2026 (Gu, Jedidi, Lin)", "https://arxiv.org/abs/2602.20122"},
	{"[4]", "GEO: How to Dominate AI Search -- Source Preferences", "U of Toronto 2025", "https://arxiv.org/abs/2509.08919"},
	{"[5]", "YouTube vs Reddit AI Citations", "Adweek / Bluefish / Emberos / Goodie AI, 2025", "https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations"},
	{"[6]", "News Source Citing Patterns in AI Search Systems", "2025", "https://arxiv.org/abs/2507.05301"},
	{"[7]", "LiveCC: Learning Video LLM with Streaming Speech Transcription", "CVPR 2025", "https://arxiv.org/abs/2504.16030"},
	{"[8]", "The False Promise of Factual and Verifiable Source-Cited Responses", "2024", "https://arxiv.org/abs/2410.22349"},
	{"[9]", "Language Models are Unsupervised Multitask Learners", "OpenAI, 2019 (Radford et al.)", "https://cdn.openai.com/better-language-models/language_models_are_unsupervised_multitask_learners.pdf"},
	{"[10]", "Consent in Crisis: The Rapid Decline of the AI Data Commons", "ACM FAccT 2024 (Longpre et al.)", "https://dl.acm.org/doi/10.1145/3630106.3659033"},
	{"[11]", "Reddit Data Licensing: Google and OpenAI Deals", "Reuters / The Verge, 2024", "https://www.reuters.com/technology/reddit-ai-content-licensing-deal-google-2024-02-22/"},
	{"[12]", "Community Consensus as LLM Authority Signal", "Bluefish Labs / Emberos Research, 2025", "https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations"},
	{"[13]", "AI Overview Citations and Search Rankings", "Ahrefs, 2025", "https://ahrefs.com/blog/search-rankings-ai-citations/"},
	{"[14]", "AI Search Overlap: How AI Citations Differ from Google", "Ahrefs, 2025", "https://ahrefs.com/blog/ai-search-overlap/"},
	{"[15]", "AI Brand Visibility Correlations (75K Brands)", "Ahrefs, 2025", "https://ahrefs.com/blog/ai-brand-visibility-correlations/"},
	{"[16]", "Do AI Assistants Prefer to Cite Fresh Content?", "Ahrefs, 2025 (17M citations)", "https://ahrefs.com/blog/do-ai-assistants-prefer-to-cite-fresh-content/"},
	{"[17]", "AI Brand Visibility and Content Recency", "Seer Interactive, 2025", "https://www.seerinteractive.com/insights/study-ai-brand-visibility-and-content-recency"},
	{"[18]", "Do Large Language Models Favor Recent Content?", "arXiv, September 2025", "https://arxiv.org/html/2509.11353v1"},
	{"[19]", "From Googlebot to GPTBot: Who's Crawling Your Site", "Cloudflare, 2025", "https://blog.cloudflare.com/from-googlebot-to-gptbot-whos-crawling-your-site-in-2025/"},
	{"[20]", "AI Overviews Study: 200,000 Keywords", "Semrush, 2025", "https://www.semrush.com/blog/semrush-ai-overviews-study/"},
	{"[21]", "ChatGPT Search Visibility: GPT-5.3/5.4 Citation Analysis", "Resoneo, 2026", "https://think.resoneo.com/chatgpt/5.3-5.4/"},
	{"[22]", "AI Overview Citations and the Top 10 (2026 update)", "Ahrefs, March 2026 (vendor study, 863K keywords)", "https://ahrefs.com/blog/ai-overview-citations-top-10/"},
	{"[23]", "Measuring Google AI Overviews", "IMC 2026 (Xu, Iqbal, Montgomery)", "https://arxiv.org/abs/2605.14021"},
	{"[24]", "How Generative AI Disrupts Search", "SIGIR 2026 (Grossman et al.)", "https://arxiv.org/abs/2604.27790"},
	{"[25]", "C-SEO Bench", "NeurIPS 2025 Datasets & Benchmarks (Puerto et al.)", "https://arxiv.org/abs/2506.11097"},
	{"[26]", "Critical Survey of GEO, 2023-2026", "arXiv, 2026 (Martinez)", "https://arxiv.org/abs/2607.14035"},
	{"[27]", "Don't Measure Once", "arXiv, 2026 (Schulte, Bleeker, Kaufmann)", "https://arxiv.org/abs/2604.07585"},
	{"[28]", "AIs Are Highly Inconsistent When Recommending Brands", "SparkToro / Gumshoe, January 2026", "https://sparktoro.com/blog/new-research-ais-are-highly-inconsistent-when-recommending-brands-or-products-marketers-should-take-care-when-tracking-ai-visibility/"},
	{"[29]", "Scoring With the Engine: Retrieval Exposure, Cross-Engine Divergence, and the Limits of Engine-Agnostic GEO Scores", "arXiv, 2026 (Tannenbaum; small sample)", "https://arxiv.org/abs/2609.22655"},
	{"[30]", "YouTube Mentions and AI Visibility (75K Brands, 2026)", "Ahrefs data via The Next Web, May 2026 (vendor data)", "https://thenextweb.com/news/ahrefs-youtube-mentions-ai-visibility-brand-search"},
	{"[31]", "Most-Cited Domains in AI Overviews", "Ahrefs, November 2026 (vendor data)", "https://ahrefs.com/blog/most-cited-domains-ai-overviews/"},
	{"[32]", "Most-Cited Domains in ChatGPT", "Ahrefs, November 2026 (vendor data)", "https://ahrefs.com/blog/most-cited-domains-in-chatgpt/"},
	{"[33]", "LLMs Keep Citing YouTube in Search Results", "Digiday, September 2026 (Meltwater and Scrunch data)", "https://digiday.com/marketing/in-graphic-detail-llms-keep-citing-youtube-in-search-results/"},
	{"[34]", "Reddit's AI Citation Decline", "Conductor, 2026 (vendor data)", "https://www.conductor.com/academy/reddit-ai-citation-decline/"},
	{"[35]", "Schema Markup and AI Citations", "Ahrefs, May 2026 (controlled test, vendor)", "https://ahrefs.com/blog/schema-ai-citations/"},
	{"[36]", "llms.txt Study", "Ahrefs, June 2026 (vendor)", "https://ahrefs.com/blog/llmstxt-study/"},
	{"[37]", "Content Recency's Impact on AI Visibility in 2026", "Seer Interactive, 2026", "https://www.seerinteractive.com/insights/study-content-recencys-impact-on-ai-visibility-in-2026"},
	{"[38]", "ChatGPT Search Insights (Clickstream)", "Semrush, 2026 (vendor data)", "https://www.semrush.com/blog/chatgpt-search-insights/"},
	{"[39]", "OpenAI Crawlers", "OpenAI documentation", "https://developers.openai.com/api/docs/bots"},
	{"[40]", "Content Independence Day: AI Options", "Cloudflare, July 2026", "https://blog.cloudflare.com/content-independence-day-ai-options/"},
	{"[41]", "Google Lets Users Jump from AI Overviews into AI Mode", "TechCrunch, January 2026", "https://techcrunch.com/2026/01/27/google-now-lets-users-jump-from-ai-overviews-into-ai-mode-conversations"},
	{"[42]", "Search at I/O 2026", "Google, 2026 (self-reported)", "https://blog.google/products-and-platforms/products/search/search-io-2026/"},
	{"[43]", "AI Overviews Reduce Clicks (2026 update)", "Ahrefs, February 2026 (vendor)", "https://ahrefs.com/blog/ai-overviews-reduce-clicks-update"},
	{"[44]", "News Integrity in AI Assistants", "EBU / BBC, October 2025", "https://www.ebu.ch/research/open/report/news-integrity-in-ai-assistants"},
	{"[45]", "State of AI Search Citations: Q4 2026", "Metavert, 2026", "https://metavert.io/state-of-ai-search-citations"},
}
