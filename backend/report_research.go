package main

// Research digest and bibliography shown at the end of every report (both
// the Chromium report and the plain fallback PDF).

type bibEntry struct {
	ref   string
	title string
	venue string
	url   string
}

var reportResearchDigest = []string{
	"Brand Recognition vs. Discovery. A key framework throughout LLM Optimizer is the distinction between brand recognition -- how well AI represents your brand when people search for it by name -- and inbound discovery -- how often AI surfaces your brand when people search your category without prior knowledge of you. Both matter, but they require different strategies. Brand recognition improves through authority signals, earned media, and training data presence. Discovery requires appearing in category-level content, answering the questions your audience asks before they know you exist, and being present in the YouTube videos, Reddit threads, and web pages that LLMs cite for category queries.",
	"The emerging science of LLM visibility reveals a fundamental shift in how information gains authority online. The most significant recent finding comes from NanoKnow (2026), which demonstrates that content appearing frequently in training data more than doubles a model's accuracy on related questions -- and that the advantage compounds when content is both memorized during training and retrievable at inference time. This means the traditional SEO playbook of optimizing for a single ranking algorithm is being replaced by a dual imperative: getting into training corpora through widespread, high-quality publication, while simultaneously remaining citable through structured, authoritative web presence.",
	"Across the research, a consistent pattern emerges: AI search engines overwhelmingly favor earned media over brand-owned content, citing third-party sources 72-92% of the time. Content that includes quotations from authoritative sources gains +41% visibility -- the single most effective optimization technique identified. Meanwhile, YouTube has rapidly become the dominant social citation source for LLMs, with its share doubling to 39% between August and December 2024. Critically, video LLMs process content through transcripts, not visual analysis -- a 7B model trained on YouTube transcripts outperformed 72B models, proving that transcript quality matters far more than production value.",
	"Reddit has emerged as the #2 social citation source for LLMs, with unique authority dynamics. Reddit was foundational in LLM training through datasets like WebText and the Common Crawl, and continues through $60M (Google) and $70M (OpenAI) annual licensing deals. Unlike YouTube's channel-centric authority, Reddit's influence comes from multi-user validation -- upvoted comment consensus, especially in \"best X for Y\" recommendation threads, creates credibility signals that LLMs weight heavily. The Toronto GEO paper classifies Reddit as \"Social\" -- a category AI search engines suppress in direct citations -- yet Reddit's pervasive presence in training data means it heavily shapes baseline model knowledge even when not explicitly cited.",
	"A critical \"two-world\" split has emerged between Google AI Overviews and standalone LLMs. 76% of AI Overview citations pull from top-10 organic pages -- making traditional search rankings the primary signal for AIO inclusion. But for standalone LLMs like ChatGPT, only 12% of cited URLs rank in Google's top 10. The strongest predictor of AI citation across platforms is YouTube mentions (0.737 correlation), followed by web mentions (0.664) -- not backlinks. Meanwhile, content freshness has become a significant signal: AI assistants cite content that is 25.7% newer than traditional search results, and 65% of AI bot crawl hits target content less than a year old. The explosive growth of AI crawlers (GPTBot up 305% YoY) makes robots.txt policy a direct lever for AI visibility.",
	"However, this new landscape comes with important caveats. Citation accuracy across AI answer engines remains surprisingly poor (49-68%), with nearly a third of claims lacking any source backing. Citation concentration follows power-law dynamics, where the top 20 sources capture 28-67% of all citations. And LLMs exhibit strong positional bias, reliably attending to content at the beginning and end of context while ignoring the middle. Together, these findings inform LLM Optimizer's scoring frameworks across answer optimization, video authority, Reddit authority, and search visibility analysis.",
}

var reportBibliography = []bibEntry{
	{"[1]", "Lost in the Middle: How Language Models Use Long Contexts", "TACL 2024", "https://arxiv.org/abs/2307.03172"},
	{"[2]", "GEO: Generative Engine Optimization", "Princeton / KDD 2024", "https://arxiv.org/abs/2311.09735"},
	{"[3]", "NanoKnow: Probing LLM Knowledge by Linking Training Data to Answers", "2026", "https://arxiv.org/abs/2602.20122"},
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
}
