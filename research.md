# LLM Optimization Research

Research findings informing how LLM Optimizer assesses whether a website's content will be surfaced and cited by AI search engines and language models.

> **November 2026 update.** Several 2023–2025 findings below have been superseded by 2026 research; each is marked where it appears, and [section 9](#9-2026-update-what-changed-and-why) summarizes what changed, with sources. A second review later in November 2026 added 30 sources; [section 10](#10-november-2026-addendum-further-findings) covers it. Figures from vendor studies are flagged as such, and correlations are associations, not causes.

## Source Papers

| Paper | Year | Key Contribution |
|-------|------|------------------|
| [Lost in the Middle](https://arxiv.org/abs/2307.03172) | 2024 | Position bias in LLM context windows |
| [GEO: Generative Engine Optimization](https://arxiv.org/abs/2311.09735) | 2024 | Content optimization methods for LLM citation (2023 lab results; see section 9) |
| [NanoKnow: How to Know What Your Language Model Knows](https://arxiv.org/abs/2602.20122) (Gu, Jedidi, Lin, SIGIR 2026) | 2026 | Training data frequency → model knowledge |
| [GEO: How to Dominate AI Search](https://arxiv.org/abs/2509.08919) | 2025 | Source type preferences in AI search engines |
| [YouTube vs Reddit AI Citations](https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations) | 2025 | YouTube #1 social citation source; 16% of LLM answers |
| [News Source Citing Patterns in AI Search](https://arxiv.org/abs/2507.05301) | 2025 | Citation concentration + gatekeeping across 366K citations |
| [LiveCC: Video LLM Training from ASR](https://arxiv.org/abs/2504.16030) | 2025 | How video LLMs train on transcripts — what actually gets extracted |
| [The False Promise of Source-Cited Responses](https://arxiv.org/abs/2410.22349) | 2024 | Citation accuracy (49-68% in 2024 systems), hallucination rates in answer engines |

---

## 1. Position Bias (Lost in the Middle)

LLMs exhibit a U-shaped attention curve over their input context — content at the beginning and end is reliably used, while content in the middle is effectively ignored.

**Key metrics:**
- GPT-3.5-Turbo with answer in middle of 20-30 docs: accuracy **below closed-book** (worse than no documents at all)
- Performance consistently declines as document count increases from 10 to 30
- Extended context windows (16K, 100K) do not fix this — the U-shaped pattern persists
- Encoder-decoder models (bidirectional) show more robustness within training-length windows

**Implications for content optimization:**
- Front-load key information in pages — opening paragraphs matter most for retrieval
- Concise, focused pages outperform comprehensive but sprawling ones
- In RAG systems, retrieval rank matters more than retrieval volume — top 10-20 results plateau quickly

---

## 2. Content Optimization Methods (GEO, Princeton/KDD 2024)

> **2026 status:** these are 2023 lab results. [C-SEO Bench](https://arxiv.org/abs/2506.11097) (NeurIPS 2025) found such content rewrites largely ineffective in realistic settings, and a [2026 survey of 45 GEO studies](https://arxiv.org/abs/2607.14035) found the 2023 gains hold only in the lab. LLM Optimizer no longer scores or recommends these rewrites; Content Authority now measures query fit and extractable evidence (section 5). The table is kept for reference.

The first GEO paper tested 9 optimization strategies on a benchmark of 10,000 queries. Results measured via Position-Adjusted Word Count (visibility weighted by citation position):

| Method | Improvement | Notes |
|--------|-------------|-------|
| Quotation Addition | **+41%** | Best single method — embed quotes from authoritative sources |
| Statistics Addition | **+33%** | Concrete numbers make content more citable |
| Fluency Optimization | +29% | Clean, well-written prose preferred |
| Cite Sources | +28% | Content that itself cites other sources is treated as more authoritative |
| Technical Terms | +19% | Domain-specific vocabulary signals expertise |
| Easy-to-Understand | +14% | Accessible language helps |
| Authoritative Tone | +12% | Confident, expert voice |
| Unique Words | +6% | Minimal impact |
| Keyword Stuffing | **-9%** | Actively harmful in generative engines |

**Combinations:** Fluency + Statistics outperformed any single method by 5.5%+.

**Democratization effect:** Lower-ranked sites benefit disproportionately. Rank-5 sites saw +115% visibility improvement from Cite Sources optimization, while rank-1 sites saw -30%.

**Domain-specific effectiveness:**
- Authoritative tone → debate, history, science
- Statistics → law & government, debate, opinion
- Quotations → people & society, explanation, history
- Cite Sources → factual statements, law & government
- Fluency → business, science, health

**Validated on Perplexity.ai** — Quotations: +22% position-adjusted, +30% subjective impression.

---

## 3. Training Data Frequency (NanoKnow)

*NanoKnow: How to Know What Your Language Model Knows* (Gu, Jedidi, Lin, SIGIR 2026). The paper's own summary: closed-book accuracy is strongly influenced by how often the answer appears in the pre-training data; external evidence reduces that dependence; and models stay more accurate on answers seen in pre-training even with external evidence, so parametric and retrieved knowledge complement each other. The measurements are on small open models (nanochat checkpoints); how far they carry to frontier models is untested.

Using fully open pre-training data (FineWeb-Edu, 100B tokens), this paper quantified the relationship between answer frequency in training data and model knowledge.

**Key findings:**
- Accuracy rises steeply from rare (1-5 documents) to high-frequency (51+ documents) answers
- ~66-71% of benchmark questions have answers present in training data
- Even with retrieved context provided, models stay more accurate on questions whose answers appeared in training data (parametric and retrieved knowledge complement each other)
- Each additional irrelevant document costs ~4-6 LLM-Judge percentage points
- Non-relevant context is **worse than no context** (distractor-only < closed-book)

**Implications:**
- Information widely repeated across high-quality educational sources is more likely encoded in model weights
- Being in training data and being retrievable at inference work together
- Irrelevant surrounding context hurts accuracy, depending on its position and amount — clear, self-contained answer passages matter
- Minimum model size for meaningful memorization: ~1.9B parameters

---

## 4. AI Search Source Preferences (GEO, Toronto 2025)

Empirical analysis of what types of sources AI search engines (ChatGPT, Perplexity, Gemini, Claude) actually cite.

**The earned media bias (central finding):**

| Source Type | Google | AI Search |
|-------------|--------|-----------|
| Earned media (reviews, independent analysis) | 45-54% | **72-92%** |
| Brand-owned content | 33-44% | 18-27% |
| Social (Reddit, YouTube, forums) | 10-15% | **~0%** |

> **2026 status:** the "~0% social" figure no longer holds for video. YouTube is the most-cited domain in Google AI Overviews ([24.5%, Ahrefs, November 2026](https://ahrefs.com/blog/most-cited-domains-ai-overviews/), vendor data) and across AI assistants ([Meltwater data via Digiday, September 2026](https://digiday.com/marketing/in-graphic-detail-llms-keep-citing-youtube-in-search-results/)). The earned-media preference still holds.

**Additional findings:**
- AI citations overlap with Google results only 15-50% — they cite fundamentally different sources
- Claude shows highest cross-language citation stability; ChatGPT switches entire site ecosystems by language
- AI citations are more robust to query paraphrasing than Google results
- When Google shows AI summaries, link clicks fell to 8% (from 15%); 26% of searches ended with zero clicks

**Four GEO imperatives proposed:**
1. Prioritize earned media — third-party reviews, analyst coverage, independent mentions
2. Structure content for machine extraction — comparison tables, explicit justification (Schema.org helps rich results, but showed no measurable AI citation lift in a [2026 Ahrefs test](https://ahrefs.com/blog/schema-ai-citations/))
3. Develop language-specific authority strategies
4. Create lifecycle content (awareness → consideration → decision → post-purchase)

---

## 5. Synthesis: Answer Optimization Scoring Framework

Based on the research above, we assess how well a page fits and answers a given question across several dimensions. These are quality and fit scores, not predicted citation rates: in 2026 tests, which pages an engine's own search retrieved explained far more of the variation in citations than any page feature (section 10.1).

### Dimension 1: Content Authority — query fit and extractable evidence
Updated in 2026 (C-SEO Bench, GEO survey): relevance and prominence are the levers that reproduce, not rewriting tricks.
- **Query fit** — Do the page's title and a key paragraph match the question people actually ask, and does the page answer it directly and early? Within a website, title and paragraph match were the page properties tied to more citations (about +14–16% per standard deviation, Chen & Koudas 2026)
- **Extractable evidence** — The evidence assistants use: figures, definitions, comparisons, steps and explicit prices, with named sources and dates. Pages with these had markedly more influence on answers (Zhang, He, Yao 2026)
- **Source citations** — Does the content cite its sources, so the evidence can be checked?
- **Precision** — Clear writing that uses the field's terminology correctly
- **Anti-patterns** — Keyword stuffing, marketing fluff, vague claims without evidence, quotations or statistics added for decoration

### Dimension 2: Structural Optimization
Derived from Lost in the Middle + GEO (Toronto):
- **Answer prominence** — Is the answer front-loaded or buried deep in the page?
- **Content conciseness** — Focused, direct answers vs. sprawling, padded content
- **Machine readability** — Clean HTML structure, and structure that carries usable evidence: comparison tables, steps, clear headings (Schema.org helps rich results and entity understanding, not AI citation on live engines)
- **No credit for packaging alone** — FAQ blocks, summary boxes and author bylines get no credit by themselves: comparing pages of the same website, they showed no citation effect (Chen & Koudas 2026), and Q&A formatting slightly lowered a page's influence on the answer (Zhang, He, Yao 2026)
- **Justification language** — Does content explain "why" not just "what"?

### Dimension 3: Source Authority
Derived from GEO (Toronto) source preference analysis:
- **Earned media coverage** — Is the site mentioned by independent third-party sources? YouTube videos from other creators, Wikipedia, review sites and third-party "best X" comparison lists all count; a brand's own self-ranked "best X" lists don't (when they were cited in AI Overviews, the brand was left out of the recommendation 69% of the time; Ray 2026)
- **Source type classification** — Brand-owned, earned media, or social content?
- **Domain authority in AI search** — Known authoritative domains for the topic area
- **Cross-engine consistency** — Would the site be cited across multiple AI engines?

### Dimension 4: Knowledge Persistence
Derived from NanoKnow training data analysis:
- **Information frequency** — How widely does this answer appear across the web?
- **Educational quality** — Is the content written in a clear, didactic style?
- **Retrieval complementarity** — Is the content both likely in training data AND easily retrievable?
- **Surrounding context quality** — Does the page provide clear, self-contained answer passages?

### Scoring Model

Each dimension contributes to an overall **LLM Visibility Score** (0-100):

| Dimension | Weight | Rationale |
|-----------|--------|-----------|
| Content Authority | 30% | Answering the question directly with extractable evidence is the most reproducible lever (2026 replications) |
| Structural Optimization | 20% | Position and structure gate whether content is used at all |
| Source Authority | 30% | AI search shows 72-92% earned media preference |
| Knowledge Persistence | 20% | Training data frequency strongly affects what models know; complements retrieval (NanoKnow) |

Sub-scores within each dimension are assessed on content analysis of the specific page(s) answering the question, combined with web search for third-party coverage signals.

---

## 6. Video Authority in LLM Search

YouTube has emerged as the dominant social citation source in AI search. Video content shapes what LLMs "know" about brands — not through visual understanding (LLMs can't watch video) but through transcripts, titles, descriptions, and metadata that get ingested into training data and retrieval systems.

### Source Papers

| Paper / Report | Year | Key Contribution |
|----------------|------|------------------|
| [YouTube vs Reddit: AI Citation Analysis](https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations) (Adweek, citing Bluefish / Emberos / Goodie AI) | 2025 | Empirical citation share data across AI search engines |
| [News Source Citing Patterns in AI Search Systems](https://arxiv.org/abs/2507.05301) | 2025 | Citation concentration, gatekeeping dynamics, provider divergence across 366K citations |
| [LiveCC: Learning Video LLM with Streaming Speech Transcription](https://arxiv.org/abs/2504.16030) | 2025 | How video LLMs are trained from ASR transcripts — reveals what actually gets extracted |
| [The False Promise of Factual and Verifiable Source-Cited Responses](https://arxiv.org/abs/2410.22349) | 2024 | Citation accuracy, hallucination rates, and misattribution in answer engines |

---

### 6.1 YouTube's Citation Dominance

**YouTube is now the #1 social citation source for LLMs:**
- YouTube appears in **16% of LLM answers** vs. 10% for Reddit (Bluefish, 2025)
- YouTube is cited **40% more frequently** than Reddit across ChatGPT, Gemini, and Perplexity (Emberos)
- YouTube's social citation share **doubled** from 18.9% (Aug 2024) to **39.2%** (Dec 2024); Reddit fell from 44.2% to 20.3% (Goodie AI, 6.1M citations across 66 brands)
- YouTube generates **18x more citations** than Instagram, **50x more** than TikTok, **500x more** than Vimeo (Profound)

**Platform-specific rates:**
- Google AI Overviews cite YouTube in 18-25% of eligible answers
- ChatGPT cites YouTube in ~18% of answers
- Social citations overall represent ~7% of all LLM citations, but growing rapidly

**2026 update:** YouTube is now the most-cited domain in Google AI Overviews (24.5%, Ahrefs, November 2026) and leads in aggregate across AI assistants, mainly through Google's AI surfaces (Meltwater, August and October 2026; Wikipedia leads in Claude and Copilot), and YouTube mentions have the strongest correlation with AI visibility across 75,000 brands (about 0.737, Ahrefs 2026; correlation, not causation).

**Critical insight:** "AI visibility is earned differently than human attention. Views, followers and creator influence don't reliably translate into AI influence." Human engagement metrics (subscriber count, view count) do not predict AI searchability — structural factors like indexable transcripts matter most.

---

### 6.2 What LLMs Actually Extract from Video

The LiveCC paper (CVPR 2025) reveals how video LLMs are trained, which tells us exactly what signals matter:

- Video LLMs are trained on **ASR transcripts** (automatic speech recognition), not visual frames for text understanding
- The training approach **densely interleaves ASR words and video frames** according to timestamps — meaning transcript quality directly determines what the model learns
- A 7B-parameter model trained on YouTube transcripts **surpassed 72B models** (Qwen2.5-VL-72B, LLaVA-Video-72B) in commentary quality — proving transcript quality > model size
- Training datasets: **Live-CC-5M** (5M YouTube videos with closed captions) and **Live-WhisperX-526K** (high-quality transcriptions)
- YouTube's auto-generated captions are the primary training data pipeline — if a video has no captions, it effectively doesn't exist to these models

**Implications for video optimization:**
- Transcripts are the #1 signal — clear, keyword-rich spoken content matters more than production quality
- Videos with human-edited captions/subtitles provide cleaner training signal than auto-generated ones
- Timestamp-aligned transcripts give models fine-grained understanding of what was said when
- A well-structured 10-minute explainer with clear transcripts > a viral 60-second clip with no captions

---

### 6.3 Citation Concentration and Gatekeeping

The AI Search Arena study (366K citations, 24K conversations across OpenAI, Perplexity, Google) reveals extreme citation concentration that applies equally to video:

**Winner-take-all dynamics:**
- OpenAI: top 20 news sources capture **67.3%** of all citations (Gini = 0.83)
- Google: top 20 capture **31.9%** (Gini = 0.69)
- Perplexity: top 20 capture **28.5%** (Gini = 0.77)

**Provider divergence:**
- Different AI providers cite substantially different sources (cross-family cosine similarity: 0.11-0.58)
- Intra-family similarity is high (0.82-0.99) — models from the same provider behave similarly
- OpenAI heavily favors Reuters (22.8%) and AP News (12.2%); Google favors Indian media; Perplexity favors BBC

**Quality filtering:**
- OpenAI cites high-quality sources 96.2% of the time; Google 92.2%; Perplexity 89.7%
- Low-credibility sources are rarely cited across all providers

**For video:** This means a small number of authoritative YouTube channels likely capture a disproportionate share of LLM citations. Getting mentioned by those high-authority channels provides outsized visibility.

---

### 6.4 Citation Accuracy and the Hallucination Problem

> **2026 figures:** [11% of AI Overview claims](https://arxiv.org/abs/2605.14021) weren't supported by the sources they cited (IMC 2026), and [45% of AI assistant answers about news](https://www.ebu.ch/research/open/report/news-integrity-in-ai-assistants) had at least one significant issue (EBU/BBC, October 2025). The 2024 study below measured earlier systems.

The answer engine evaluation study (You.com, BingChat, Perplexity) reveals serious reliability issues in how AI search engines use their sources:

**Citation accuracy is poor:**
- Perplexity: only **49%** citation accuracy (half its citations don't actually support the claim)
- You.com: 68.3% accuracy
- BingChat: 65.8% accuracy

**Thoroughness is uniformly bad:**
- All systems: only **20-24%** citation thoroughness (citations cover only ~1/4 of possible source-statement relationships)

**Unsupported claims are common:**
- 23-32% of relevant statements have **no source backing at all**
- Systems display more sources than they actually cite (Perplexity uses only 51% of displayed sources)

**Overconfidence compounds the problem:**
- Perplexity generates one-sided answers in **83.4%** of debate queries, with **81.6%** simultaneously overconfident
- Users verify sources less when answers align with existing beliefs (1.08 sources hovered vs. 2.95 for opposing views)
- Users interact with answer engine sources far less than search results (2 sources hovered vs. 11.8 for Google Search)

**For video analysis interpretation:**
- When LLMs cite video content, the citation may not accurately reflect what the video actually says
- Brand sentiment in LLM outputs may be a distorted version of actual video sentiment
- Videos that are clear, structured, and unambiguous in their claims are less likely to be misrepresented
- The "extractability" dimension in Level 2 analysis directly addresses this — measuring how clearly a video's content can be accurately cited

---

### 6.5 Implications for Video Report Interpretation

These findings reshape how we should interpret each analysis level:

**Level 1 (Channel Health) — informed by LiveCC + GEO:**
- **Transcript quality is the dominant signal.** A video's LLM influence is almost entirely determined by its transcript, not its visual content or production value.
- **Keyword alignment in transcripts** maps directly to how video LLMs are trained (ASR → model weights). If target keywords aren't spoken clearly in the video, the model won't learn the association.
- **Structural clarity** (clear topic sentences, explicit entity naming, standalone statements a model can lift into an answer) matters because answers are extracted from transcripts.
- **Caption availability** is binary and critical — no captions = invisible to LLMs.

**Level 2 (Brand Perception) — informed by citation accuracy research + concentration data:**
- **Sentiment classification needs a confidence discount.** AI answers often misstate their sources (11% of AI Overview claims unsupported in 2026; 23-32% of answer engine claims unsupported in 2024), so what LLMs "believe" about a brand from video may be a distorted signal.
- **Mention position matters more than mention frequency.** Per Lost in the Middle, content at the beginning of a video transcript gets disproportionate attention in LLM context windows.
- **Extractability is the key differentiator.** Videos where brand mentions are clear, contextual, and unambiguous will be cited more accurately than passing references.
- **High-authority creators dominate.** The winner-take-all citation pattern (Gini 0.69-0.83) means a mention by a top-authority channel in a niche is worth more than dozens of small-channel mentions.

**Level 3 (Competitive Landscape) — informed by provider divergence + gatekeeping:**
- **Share of voice varies by AI platform.** Different providers cite different sources (cross-family similarity only 0.11-0.58), so a brand's video share of voice in ChatGPT may differ substantially from Perplexity or Gemini.
- **Content gaps represent real opportunity.** The concentration data shows that being the first authoritative video voice in a gap query could capture disproportionate citation share.
- **Creator targeting should prioritize channels already cited by AI systems**, not just channels with high view counts. Human engagement metrics don't predict AI citation (Adweek finding).

---

### 6.6 Implications for Prompt Architecture

The research suggests several refinements to how we prompt Claude for video analysis:

**Level 1 prompt refinements:**
- Weight transcript analysis above all other signals — it's what LLMs actually ingest
- Score caption availability/quality as a binary gate (no captions = score cap)
- Assess "quotability" — does the transcript contain clear, standalone statements that could be extracted and cited?
- Check for entity clarity: is the brand name spoken explicitly or only implied/shown visually?

**Level 2 prompt refinements:**
- Include a confidence discount in sentiment assessment — note that AI answers often misstate their sources (see 6.4)
- Weight mention position within transcript (first 20% and last 20% matter most, per U-shaped attention)
- Assess extractability explicitly: could an LLM accurately represent what this video says about the brand?
- Flag cases where brand mentions are ambiguous or easily misattributed

**Level 3 prompt refinements:**
- Note that share-of-voice may differ across AI platforms — recommend checking multiple engines
- Prioritize content gap identification where no authoritative video exists (winner-take-all means first-mover advantage is large)
- Score creator targets by structural authority (transcript quality, caption availability, topical consistency) rather than human engagement metrics
- Include the "AI visibility ≠ human attention" principle as an explicit instruction

---

## 7. Implementation Guidance: Answer Optimization Feature

The Answer Optimization feature should analyze a specific question-answer pair from a website analysis and assess how likely an LLM is to surface and cite that answer.

### What to Analyze
For a given question + page URL(s) from the site analysis:
1. Fetch and analyze the actual page content for authority signals (quotes, stats, citations, fluency, structure)
2. Search for third-party coverage of the site for this topic area (earned media signals)
3. Assess how well the content is structured for machine extraction
4. Evaluate the answer's prominence on the page (front-loaded vs. buried)
5. Check how widely the answer appears across the web (frequency/persistence)

### LLM Prompt Strategy
Use Claude with web_search to:
- Visit the specific page(s) and analyze content characteristics
- Search for third-party mentions and reviews of the site in this topic area
- Search for competing answers to the same question from other sources
- Assess relative authority compared to competing sources

### Output Structure
- Overall LLM Visibility Score (0-100)
- Per-dimension scores with specific evidence
- Actionable recommendations for improving each dimension
- Competitive landscape — who else answers this question and how they compare
- Priority ranking of improvement actions by expected impact

### Caching Strategy
- Cache optimization results linked to the specific analysis + question index
- Same 30-day expiry as site analyses
- Force re-analysis option available

---

## 8. Search Visibility and AI Crawl Accessibility

Traditional SEO is fragmenting into two distinct optimization pathways: Google AI Overviews (which favor organically top-ranked pages less than they did: about 38% of citations from the top 10 in 2026) and standalone LLMs (which operate on fundamentally different signals). Understanding this split — and the role of crawl accessibility, content freshness, and brand search momentum — is critical for comprehensive LLM visibility assessment.

### Source Papers

| Paper / Report | Year | Key Contribution |
|----------------|------|------------------|
| [How LLMs & AI Agents Get Info From Your Site](https://ahrefs.com/blog/llms-and-ai-agents/) (Ahrefs, 1.9M citations) | 2025 | AI Overview citation patterns — 76% from Google's top-10 organic results |
| [Where Do AI Search Engines Get Results?](https://ahrefs.com/blog/ai-search-traffic-study/) (Ahrefs, 17M citations) | 2025 | The two-world split — only 12% overlap between Google top-10 and ChatGPT/Perplexity citations |
| [AI Brand Visibility Study: 75K Brands](https://ahrefs.com/blog/ai-brand-visibility-study/) (Ahrefs) | 2025 | Brand search volume, web mentions, and YouTube as strongest predictors of AI citation |
| [Content Freshness and AI Answers](https://ahrefs.com/blog/freshness-seo/) (Ahrefs) | 2025 | AI assistants cite content 25.7% newer on average than traditional search |
| [The Impact of Content Recency on AI Visibility](https://www.seerinteractive.com/insights/content-recency-ai-overview) (Seer Interactive) | 2025 | Freshness signals shift LLM ranking by up to 95 positions |
| [Recency Bias in AI: LLM Performance on Outdated Facts](https://arxiv.org/abs/2504.01248) | 2025 | Systematic recency bias in LLM knowledge — newer facts preferred even when wrong |
| [2025 AI Bot & Crawler Report](https://blog.cloudflare.com/ai-crawlers-update-2025) (Cloudflare) | 2025 | GPTBot traffic grew 305% YoY; 3-bot architecture per AI provider; robots.txt as visibility lever |
| [AIO Study: 200K Keywords](https://www.semrush.com/blog/ai-overviews/) (Semrush) | 2025 | AIOs are long-tail dominated and informational (its trigger-rate figure is outdated) |
| [AI Overview Citations and the Top 10](https://ahrefs.com/blog/ai-overview-citations-top-10/) (Ahrefs, 863K keywords, vendor) | 2026 | About 38% of AIO citations from the top 10, down from 76% (partly a parsing change) |
| [Measuring Google AI Overviews](https://arxiv.org/abs/2605.14021) (IMC 2026) | 2026 | ~30% of AIO-cited domains not on page one; 11% of AIO claims unsupported by their sources |
| [How Generative AI Disrupts Search](https://arxiv.org/abs/2604.27790) (SIGIR 2026) | 2026 | Google, AIO and Gemini sources overlap below 0.2 Jaccard; blocking Google's AI crawler associated with less AIO retrieval |
| [OpenAI crawler docs](https://developers.openai.com/api/docs/bots) | 2026 | GPTBot = training; OAI-SearchBot = ChatGPT search; ChatGPT-User may not honour robots.txt |
| [Content Independence Day](https://blog.cloudflare.com/content-independence-day-ai-options/) (Cloudflare) | 2026 | AI crawlers blocked by default on new domains from 15 September 2026 |
| [Content Recency's Impact on AI Visibility in 2026](https://www.seerinteractive.com/insights/study-content-recencys-impact-on-ai-visibility-in-2026) (Seer Interactive) | 2026 | Update dates of key commercial and comparison pages matter more than average site age |

---

### 8.1 The Two-World Split: AI Overviews vs. Standalone LLMs

The most critical finding for search visibility assessment is that **Google AI Overviews and standalone LLMs cite fundamentally different sources.**

**Google AI Overviews (AIO):**
- **About 38% of AIO citations** come from pages in Google's top-10 organic results (Ahrefs, March 2026, 863K keywords, vendor) — down from **76%** in Ahrefs' 2025 study (1.9M citations), partly because of a change in parsing
- About **30% of AIO-cited domains** are not on page one (IMC 2026), and Google's results, AIO and Gemini share few sources (Jaccard below 0.2, SIGIR 2026)
- Since January 2026 AIO runs on Gemini 3 and hands off into AI Mode, which passed 1 billion monthly users (Google, self-reported)
- When an AIO appears, the top organic result's click-through rate falls about **58%** (Ahrefs, February 2026)
- AIO strongly favors informational queries, how-to content, and educational material
- Organic ranking still helps AIO citation, but it is no longer close to a prerequisite

**Standalone LLMs (ChatGPT, Perplexity, Claude):**
- Only **12% overlap** between Google's top-10 organic results and ChatGPT/Perplexity citations (Ahrefs, 17M citations)
- LLMs cite from a much wider pool of sources, weighted by training data presence and earned media signals
- Brand authority and web mention frequency matter more than traditional link-based authority
- Clear, extractable content (tables, definitions, direct answers) plays a larger role than backlink profiles; schema markup showed no measurable AI citation lift in a 2026 controlled test (Ahrefs), and 97% of llms.txt files got no AI crawler requests (Ahrefs, June 2026)
- Per-engine domain shares depend on the prompt set and change within weeks: Ahrefs' November 2026 set put Wikipedia first in ChatGPT (10.1%) and Reddit second (6.2%), while Meltwater's October 2026 data put NIH first in ChatGPT and Reddit near 0.4% (section 10.4). ChatGPT searched the web for about 34.5% of US prompts in early 2026 (Semrush clickstream)

**Implication:** A site can rank #1 in Google and still be invisible to ChatGPT — and vice versa. Comprehensive search visibility assessment must evaluate both pathways independently.

---

### 8.2 AI Crawl Accessibility

AI companies now operate crawlers by **purpose**, each requiring separate robots.txt consideration (updated 2026 from OpenAI's crawler documentation):

| Bot Type | Purpose | Examples |
|---------|---------|---------|
| Training bots | Collect content for model training | GPTBot, ClaudeBot, Applebot-Extended, CCBot |
| Search bots | Index pages for AI search answers | OAI-SearchBot, Claude-SearchBot, PerplexityBot, Googlebot, Bingbot |
| User-fetch bots | Fetch pages live when a user asks | ChatGPT-User (may not follow robots.txt), Claude-User, Perplexity-User |
| Control tokens | Opt out of a use without blocking crawling | Google-Extended (Gemini training and grounding; Google says it doesn't affect inclusion or ranking in Google Search; see section 10.7) |

**Key metrics:**
- **GPTBot traffic grew 305% YoY** (Cloudflare, 2025) — AI crawlers are now a major share of web traffic; by June 2026, 52% of crawler requests were for AI training and mixed-use crawlers (search, agent and training combined) were over 36% of activity (Cloudflare, 2026), which makes clean training-only blocks hard
- **robots.txt is a request, not enforcement:** in a controlled 2026 study Claude and Mistral followed it, while Gemini and Copilot fetched disallowed pages (Lopez-Fonseca et al. 2026). Real blocking needs network-level enforcement
- Google intends to support the **AIPREF** training opt-out signal (`user-agent: __usage_ai-training`) in early 2027
- **ClaudeBot and Bytespider grew 246% and 76%** respectively
- Approximately **30-40% of high-traffic sites** now block at least one AI crawler
- Since **15 September 2026 Cloudflare blocks AI crawlers by default** on new domains, so a site can be blocked without its owner choosing it — check the CDN setting as well as robots.txt
- Blocking training bots prevents future knowledge acquisition; blocking user-fetch bots prevents real-time citation
- Sites that block user-fetch bots (e.g., ChatGPT-User) are **invisible in real-time AI conversations** regardless of training data presence

**robots.txt is now a direct AI visibility lever:**
- `Disallow: /` for GPTBot excludes a site from OpenAI's training only; ChatGPT search uses OAI-SearchBot
- Selective blocking is possible (e.g., allow search bots, block training bots); blocking training bots is a business choice and costs nothing in LLM Optimizer's crawl score, while blocking a major search bot caps it
- Many sites have overly broad blocks that unintentionally exclude themselves from AI search
- Sitemap presence and quality affect how efficiently AI crawlers can index content

---

### 8.3 Brand Search Momentum

The Ahrefs 75K brands study quantified which signals correlate with whether an AI assistant mentions a brand. Its 2026 figures ([via The Next Web, May 2026](https://thenextweb.com/news/ahrefs-youtube-mentions-ai-visibility-brand-search), vendor data): YouTube mentions about 0.737, branded web mentions 0.66–0.71, Domain Rating 0.27–0.33. These are correlations, not causes. The 2025 table:

| Signal | Correlation with AI Citation | Notes |
|--------|------------------------------|-------|
| YouTube mentions | **0.737** | Strongest single predictor |
| Web mentions (earned media) | **0.664** | Second strongest — independent coverage |
| Brand search volume | **0.334** | Google Trends searches for the brand name |
| Domain authority / backlinks | 0.12-0.18 | Surprisingly weak predictor |
| Social media followers | <0.1 | Nearly irrelevant |

**Key findings:**
- Brand search volume is a **proxy for real-world prominence** — brands that people actively search for are more likely to appear in AI answers
- Web mentions (being talked about by others) are **3.7x more predictive** than traditional domain authority
- YouTube mentions are the **strongest single signal** (0.737 correlation) — this connects to our Video Authority findings in Section 6
- Traditional SEO metrics (backlinks, DA) are **poor predictors** of AI citation — the signal landscape has shifted

**Entity recognition** also matters: brands that Google recognizes as named entities (appearing in Knowledge Panels) receive disproportionate AI citation, suggesting that entity databases (Wikidata, Google Knowledge Graph) influence LLM knowledge.

---

### 8.4 Content Freshness Bias

Multiple studies confirm that AI systems systematically prefer newer content:

**Ahrefs (17M citations study):**
- AI assistants cite content that is **25.7% newer** on average than content cited in traditional Google results
- The median age of AI-cited content is **~14 months** vs. **~18 months** for Google organic results
- For rapidly evolving topics (technology, health, policy), the freshness bias is even more extreme

**Seer Interactive:**
- Content freshness signals can shift an article's **position by up to 95 ranks** in AI search results
- Pages updated within the last 6 months receive **significantly higher** AIO citation rates than stale content
- The effect is strongest for queries with informational intent

**arXiv — LLM Recency Bias (2025):**
- LLMs show systematic recency bias: they prefer information from more recent sources even when older sources contain more accurate answers
- This bias is **inherent to training data distributions** — more recent web pages are overrepresented in training corpora
- The bias compounds in RAG systems: retrieval algorithms also favor recent documents, amplifying the preference

**Seer Interactive (2026):** how recently key commercial and comparison pages (product, pricing, comparisons) were updated matters more for AI visibility than a site's average content age.

**Implications for optimization:**
- Refresh key product, pricing and comparison pages at least yearly, with honest update dates
- Regular content updates provide an ongoing visibility advantage in AI search
- "Content decay" is real — pages that haven't been updated in 12+ months lose AI visibility even if the information remains accurate
- Publication and modification dates should be explicitly marked in structured data (Schema.org `dateModified`)
- Evergreen content strategies must include regular refresh cycles to maintain AI visibility

---

### 8.5 Search Visibility Scoring Framework

Based on the research above, we assess a website's search visibility for AI search engines across four dimensions (five when Brand Intelligence provides category data). The overall score is recomputed from the pillars with the weights below every time a result is read (`backend/scoring.go`).

#### Pillar 1: AI Overview Readiness (25%)
How well-positioned is the site to appear in Google AI Overviews?

- **Organic presence** — Does the site rank in Google's top-10 for relevant queries? (About 38% of AIO citations come from the top 10 in 2026)
- **Structured data** — Informational only (10% of the pillar): schema showed no measurable AI citation lift in 2026, though it helps rich results
- **Content format** — Clear, concise answers to common questions; comparison tables; step-by-step formats
- **Answer prominence** — Key answers front-loaded in content; not buried in long-form prose

#### Pillar 2: Crawl Accessibility (20%)
Can AI systems actually access and index the site's content?

- **robots.txt policy** — Which AI bots are allowed vs. blocked? Overly broad blocks? Does a CDN (e.g. Cloudflare's default AI-bot blocking) block bots that robots.txt allows?
- **AI bot access** — Access for the search and user-fetch bots; training bots don't count. Blocking a major search bot (Googlebot, Bingbot, OAI-SearchBot, PerplexityBot, Claude-SearchBot) caps the pillar
- **Sitemap quality** — Is there a valid, comprehensive sitemap? Is it submitted to search engines?
- **Render accessibility** — Can content be accessed without JavaScript rendering? Are there bot-specific barriers?

#### Pillar 3: Brand Search Momentum (30%)
How visible is the brand in the broader search and mention landscape?

- **Brand search trend** — Is brand search volume growing, stable, or declining? (Google Trends signal)
- **Competitor comparison** — How does brand search volume compare to key competitors?
- **Web mention strength** — Is the brand mentioned by independent third-party sources? (0.66–0.71 correlation with AI visibility in 2026; correlation, not causation)
- **Entity recognition** — Does Google recognize the brand as a named entity? (Knowledge Panel presence)

#### Pillar 4: Content Freshness (25%)
How current is the site's content, and is it being maintained?

- **Key page freshness** — When were the key commercial and comparison pages last updated? These count more than average site age (Seer 2026)
- **Average content age** — How old is the typical page on the site? (AI prefers content 25.7% newer)
- **Update frequency** — How often is content being refreshed or added?
- **Freshness signals** — Are `dateModified` and `datePublished` structured data present and accurate?
- **Content decay risk** — What percentage of content hasn't been updated in 12+ months?

#### Scoring

| Pillar | Weight | Rationale |
|--------|--------|-----------|
| AI Overview Readiness | 25% | AIO is a major discovery channel, but only ~38% of its citations now come from the organic top 10 |
| Crawl Accessibility | 20% | Gate — if search bots can't crawl, nothing else matters |
| Brand Search Momentum | 30% | Branded web mentions (0.66–0.71) and YouTube mentions (~0.74) correlate most strongly with AI visibility |
| Content Freshness | 25% | 25.7% newer content preference; key-page update dates matter most |

With Category Discovery the weights are 20 / 15 / 25 / 20 / 20.

### Implementation Notes

The Search Visibility analysis uses Claude with `web_search` to check:
- The site's robots.txt file for AI crawler policies
- Google search results for organic ranking signals
- Third-party mentions and earned media coverage
- Content publication dates and update patterns
- Structured data and Schema.org markup presence
- Sitemap availability and quality
- Brand search trends and competitor comparison

Results are cached for 30 days per domain, with force re-analysis option. Recommendations are auto-converted to actionable todos when priority is high or medium.

---

## 9. 2026 Update: What Changed and Why

A review of 2026 research (November 2026) changed parts of LLM Optimizer's claims, scoring and guidance. Confidence varies: peer-reviewed and primary sources are marked H, studies with a described method M, vendor or thin sources L or "vendor".

### 9.1 Claims retired or corrected
- **AIO and the organic top 10:** about 38%, not 76% ([Ahrefs, March 2026](https://ahrefs.com/blog/ai-overview-citations-top-10/), M, vendor); ~30% of AIO-cited domains are off page one ([IMC 2026](https://arxiv.org/abs/2605.14021), H); Google, AIO and Gemini sources overlap below 0.2 Jaccard ([SIGIR 2026](https://arxiv.org/abs/2604.27790), H).
- **"Quotations +41% = most effective technique":** a 2023 lab result; [C-SEO Bench](https://arxiv.org/abs/2506.11097) (H) and a [2026 survey of 45 studies](https://arxiv.org/abs/2607.14035) (M) found such rewrites don't reliably hold on live engines. Relevance and prominence reproduce.
- **"Social ≈ 0% of AI citations":** YouTube is the #1 domain in AIO ([24.5%, Ahrefs, November 2026](https://ahrefs.com/blog/most-cited-domains-ai-overviews/), M) and in aggregate across assistants ([Meltwater via Digiday](https://digiday.com/marketing/in-graphic-detail-llms-keep-citing-youtube-in-search-results/), M), mainly through Google's AI surfaces (section 10.4).
- **Reddit:** its AI citation share fell about 50% from October 2025 to January 2026 ([Conductor](https://www.conductor.com/academy/reddit-ai-citation-decline/), M); in Ahrefs' November 2026 prompt set, Wikipedia was #1 in ChatGPT (10.1%) and Reddit #2 (6.2%) ([Ahrefs](https://ahrefs.com/blog/most-cited-domains-in-chatgpt/), M), but other 2026 measurements differ by more than ten times; see section 10.4. The Google and OpenAI licensing deals date from 2024 and are reported to be under renegotiation. The "Reddit 40.1% of AIO citations" figure was dropped as misattributed.
- **Correlations:** YouTube mentions ~0.737, branded web mentions 0.66–0.71, Domain Rating 0.27–0.33 ([Ahrefs via The Next Web](https://thenextweb.com/news/ahrefs-youtube-mentions-ai-visibility-brand-search), M) — correlation, not causation.
- **Crawlers:** GPTBot is training only; OAI-SearchBot powers ChatGPT search; ChatGPT-User may not honour robots.txt ([OpenAI](https://developers.openai.com/api/docs/bots), H). Cloudflare blocks AI bots by default on new domains from 15 September 2026 ([Cloudflare](https://blog.cloudflare.com/content-independence-day-ai-options/), H).
- **Schema and llms.txt:** no meaningful AI citation lift from schema ([Ahrefs, May 2026](https://ahrefs.com/blog/schema-ai-citations/), M); 97% of llms.txt files got no requests ([Ahrefs, June 2026](https://ahrefs.com/blog/llmstxt-study/), M).
- **Citation accuracy:** 11% of AIO claims unsupported (IMC 2026, H); 45% of assistant news answers had a significant issue ([EBU/BBC, October 2025](https://www.ebu.ch/research/open/report/news-integrity-in-ai-assistants), H). These replace the 2024 "49–68%".
- **NanoKnow:** correct title *NanoKnow: How to Know What Your Language Model Knows* (Gu, Jedidi, Lin, SIGIR 2026); its claims are now stated as the paper states them, on small open models.

### 9.2 Variance: measure visibility as a distribution
AI answers vary from run to run. Asked the same question repeatedly, AI tools returned the same list of brands less than 1% of the time ([SparkToro/Gumshoe, January 2026](https://sparktoro.com/blog/new-research-ais-are-highly-inconsistent-when-recommending-brands-or-products-marketers-should-take-care-when-tracking-ai-visibility/), M), and in a small study one engine's cited URLs turned over 67% from one day to the next ([Tannenbaum 2026](https://arxiv.org/abs/2609.22655), L–M). Visibility should be measured as a distribution over repeated runs, per engine ([Schulte, Bleeker, Kaufmann 2026](https://arxiv.org/abs/2604.07585), M): how often a brand is mentioned or recommended, never its rank position in a list.

### 9.3 Platform premises
Gemini 3 has powered AI Overviews since January 2026, with handoff into AI Mode ([TechCrunch](https://techcrunch.com/2026/01/27/google-now-lets-users-jump-from-ai-overviews-into-ai-mode-conversations)); AI Mode passed 1 billion monthly users ([Google I/O 2026](https://blog.google/products-and-platforms/products/search/search-io-2026/), self-reported). ChatGPT used web search on about 34.5% of US prompts ([Semrush](https://www.semrush.com/blog/chatgpt-search-insights/), M, vendor). An AI Overview cuts the top organic result's click-through about 58% ([Ahrefs, February 2026](https://ahrefs.com/blog/ai-overviews-reduce-clicks-update), M).

### 9.4 Scoring changes
- **LLM Visibility Score weights:** Optimization 25%, Video 20%, Reddit 15%, Search 15%, LLM Test 25% (was 30/20/20/15/15). The LLM Test is the only direct measure of what assistants answer.
- **Search Visibility weights:** AI Overview Readiness 25%, Crawl 20%, Brand Momentum 30%, Freshness 25%; with Category Discovery 20/15/25/20/20.
- **Content Authority** measures query fit and extractable evidence; **Source Authority** counts YouTube, Wikipedia, review sites and "best X" lists as third-party coverage; **Freshness** weighs the update dates of key commercial and comparison pages above average site age.
- **Schema** is informational in AI Overview readiness; **crawl access** counts search and user-fetch bots only, and a blocked major search bot caps the pillar.
- Weights apply to every stored analysis when it is read; each result records the rubric version it was scored with.

### 9.5 Guidance
1. Answer buyers' real questions directly and early, with sourced facts. Don't add quotations or statistics for a visibility percentage.
2. llms.txt isn't needed. Schema helps rich results and entities, not AI citation.
3. Allow search and user-fetch crawlers; blocking training crawlers is a business choice; check Cloudflare's AI bot setting.
4. Third-party presence priorities: Wikipedia, comparison lists and review sites, and many YouTube creator mentions. Take part in Reddit genuinely, without over-investing.
5. Refresh key product, pricing and comparison pages at least yearly, with honest update dates.
6. Judge visibility by mention rate over repeated runs, per engine, and describe findings as "associated with", not as causes.

### 9.6 Unchanged
The brand recognition vs. discovery framing; video at 20% with its transcript focus; Source Authority at 30% and the earned-media emphasis; answer prominence and front-loading; the Freshness pillar; monthly domain shares cited with dates but not hard-wired into scoring. Synthesis: [Metavert, State of AI Search Citations: Q4 2026](https://metavert.io/state-of-ai-search-citations).

---

## 10. November 2026 Addendum: Further Findings

A second review of 2026 research (November 2026) added 30 sources. Confidence: H = peer-reviewed or primary, M = credible method described, L = vendor or thin. Several 2026 brand-visibility preprints are written by people at AI-visibility companies about their own data; those are flagged.

### 10.1 What changes citation
- **The 2023 levers don't move live engines.** A re-test of quotation, statistics and cite-sources (published gains +42.6%, +32.8%, +27.7%) on ten current engine families found they moved citation on none; a query-agnostic content score correlated with citation at only 0.11 within a query, so such scores are quality filters, not predictors ([Bajemon & Rochet 2026](https://arxiv.org/abs/2609.07559), L–M, vendor-affiliated, code and data released).
- **Fit to the question, not packaging.** Comparing pages of the same website, FAQ blocks (−3.4%), summary boxes (+6.9%) and schema (−5.3%) had no significant effect, though all looked helpful across websites. Title match (+14.1%) and best-paragraph match (+15.5%) to the question, per standard deviation, held up. Which pages ChatGPT's own search retrieved explained 32% of within-website variation; page features explained 2.8% ([Chen & Koudas, University of Toronto, 2026](https://arxiv.org/abs/2610.15033), M; 158,099 answers, June–September 2026).
- **Cited isn't the same as used.** ChatGPT cited 6.88 sources per prompt against 12.06 for Google and 16.35 for Perplexity, but its cited pages had far more influence on the answer (0.27 against about 0.06). Pages with numbers (+61.6%), definitions (+57.3%), comparisons (+55.3%) or how-to steps (+41.2%) had more influence; Q&A formatting had 5.7% less ([Zhang, He, Yao 2026](https://arxiv.org/abs/2604.25707), M, descriptive).
- In a controlled testbed, topical relevance and list position drove being cited first; explicit prices and a recent timestamp helped; formatting-only edits had little impact ([Vishwakarma, Kumar, Jamidar 2026](https://arxiv.org/abs/2605.25517), M).
- In a simulated retrieve-rerank-generate pipeline, existing GEO methods often degraded retrieval, and structural information such as schema helped ([SAGEO Arena, KDD 2026](https://arxiv.org/abs/2602.12187), H, simulation). So schema showed no citation lift *on live engines*; it isn't useless.

### 10.2 What models remember
- ChatGPT wrote a brand into its own search queries 67% of the time when the brand was in its prior knowledge, against 11% otherwise; 46% of its unprompted brand mentions had no supporting cited page ([Chen & Koudas 2026](https://arxiv.org/abs/2610.15033), M).
- Top-quintile brands were cited 1.4–2.8 times as often as the average brand; retrieval didn't mitigate it, and fact-preserving rewrites gave negligible gains ([Monaco, Lenti, Pierri 2026](https://arxiv.org/abs/2611.07614), M).
- On category prompts, household names appeared in 73% of relevant answers, mid-market brands 44%, niche brands 11% ([Kumar 2026](https://arxiv.org/abs/2606.20065), L, the author co-founded the vendor whose data is used).
- With identical specifications, well-known brands were recommended 100% of the time, until a competitor had a rating advantage of less than 0.1 stars ([Chu & Hou 2026](https://arxiv.org/abs/2606.17443), M).
- Category leaders appear in nearly every relevant retrieval but win only 25–41% of the recommendation slots they reach; 48–52% of specialist and regional brands never surfaced in 37,000 runs ([Jack et al. 2026](https://arxiv.org/abs/2605.27439), L–M, unusual.ai). When two providers both left a brand out, they agreed on the failure stage 95.1% of the time: never reached the model, reached it but not mentioned, or mentioned but not positioned as the pick. Anthropic models recommended from prior knowledge 43–52% of the time, OpenAI models 8–29% ([Jack et al. 2026](https://arxiv.org/abs/2606.26116), L–M, unusual.ai).

**Implication:** compare a brand with peers of similar size, not with category leaders. Leaders need differentiation; small brands first need to be found; challengers build familiarity through third-party ratings, reviews and comparison coverage more than through page rewrites.

### 10.3 Owned pages, third parties and listicles
- 85.7% of citations in brand answers pointed to sites the brand doesn't own; Wikipedia was the most-cited domain in 11 of 12 languages ([Zatuchin 2026](https://arxiv.org/abs/2606.25787), L–M, author's company data).
- The tracked brand's own domain drew 2.9% of citations and other companies' commercial pages 75.2%, with ranked "best X" listicles the most-cited format ([Kumar 2026](https://arxiv.org/abs/2606.20065), L).
- Owned pages still matter: a brand was mentioned 2.8% (GPT) and 3.8% (Gemini) of the time when neither it nor its domain was in the retrieval path, 49.0% and 58.4% with an own-domain citation, and 91.4% and 100% when the engine also searched the brand's name ([Tannenbaum 2026](https://arxiv.org/abs/2609.23162), L–M, author's company data).
- When a brand's own self-ranked "best X" list was cited in AI Overviews, the brand was left out of the recommendation 69% of the time (224 of 323 cases; [Ray 2026](https://lilyraynyc.substack.com/p/why-calling-yourself-the-best-could), L, practitioner). Self-promotional listicles still drew about 11% of citations in software reviews, 3.6% in ChatGPT ([Peec 2026](https://docs.peec.ai/research/self-promotional-listicles), L, vendor).

**Implication:** the earned-media emphasis stands, but "third-party" is mostly other companies' pages, and an owned page that answers the question is one of the strongest correlates of being named. Count third-party "best X" lists as coverage, never a brand's own.

### 10.4 Per-engine domain shares are dated and disputed
- Meltwater's October 2026 data (about 120,000 prompts, 88.6M citations, eight platforms): YouTube led overall, but the top domain was YouTube in AI Overviews, Google in AI Mode, Wikipedia in Claude and Copilot, Reddit in Gemini, NIH in ChatGPT, X in Grok and Yahoo in Perplexity. ChatGPT stayed near 0.4% Reddit, Claude cited none, and Gemini's Reddit citations fell 88% in late October ([Meltwater 2026](https://www.meltwater.com/en/blog/ai-search-visibility-report-october-2026), L–M, vendor).
- ChatGPT cited about half the URLs it retrieved; Reddit pages it retrieved were cited 1.93% of the time and made up 67.8% of retrieved-but-not-cited URLs ([Ahrefs, April 2026](https://ahrefs.com/blog/why-chatgpt-cites-pages/), M, vendor). Citation share understates Reddit's influence.
- In AI summaries, Wikipedia and longer sources are overrepresented; cited social media and negatively framed sources are underrepresented ([Huang et al. 2026](https://arxiv.org/abs/2603.16138), M, listed for EMNLP 2026).

**Corrected:** "Reddit is #2 in ChatGPT (6.2%)" is one prompt set's figure; "YouTube is #1 across assistants" holds in aggregate, mainly through Google's surfaces; "Wikipedia is #1 in ChatGPT" depends on the prompt set.

### 10.5 Measurement and variance (the LLM Test)
- Even at temperature 0, 9–27% of yes/no answers flipped within five minutes; rerun about two months apart, only 18% of AI Overview pages were the same, against 45% for organic results; on average 53% of the domains AI Overviews consult are outside the organic top 10 ([Kirsten et al., Findings of ACL 2026](https://aclanthology.org/2026.findings-acl.526/), H).
- Aggregated over 250 questions, reliability reached 0.96 at five runs, but the power to detect a large difference on a single question was 0.15–0.19 at five runs; 0.8 needs about 15–25 runs ([Zatuchin, Dice Roll Method 2026](https://arxiv.org/abs/2609.04047), L–M, implemented in a product of the author's company).
- Sentiment flipped about 6.7 times as often as mention ([Kumar 2026](https://arxiv.org/abs/2606.20065), L), and query language accounted for 26.49% of the variance in automated sentiment scores against 1.46% for the brand ([Zatuchin 2026](https://arxiv.org/abs/2607.13304), L–M).
- For the same product question, a chatbot's API and its consumer app shared 12.0% (ChatGPT) and 14.8% (Gemini) of cited domains; the two apps shared 5.4% ([Uberti-Bona Marin et al. 2026](https://arxiv.org/abs/2609.18729), M).

**Implication:** three to five runs are enough for brand-level rates aggregated across questions; differences on a single question at five runs or fewer are indicative only. Sentiment is reported separately from the score and shouldn't be compared across languages. LLM Test results are API answers, which can differ from what consumer apps show.

### 10.6 Grounding and accuracy
- In health questions, about 56.7% of cited statements in AI Overviews were supported by the cited page (so about 43% were not), and a further 20.8% of statements were uncited factual claims ([Agnoli et al. 2026](https://arxiv.org/abs/2610.39339), M). Present accuracy as a range by topic: 11% unsupported in general queries (IMC 2026), about 43% in health.
- Across 158,000+ checked claims, inaccuracy by engine ranged 4.3–6.8%; pricing and billing were 12% of claims but 24% of inaccurate ones, and 54% of brands had an inaccurate claim citing their own content ([Profound 2026](https://www.tryprofound.com/blog/where-do-inaccurate-ai-claims-come-from), L, vendor).

**Implication:** check pricing and plan claims first; keep pricing pages explicit and current, and audit old owned pages.

### 10.7 Crawlers and Google-Extended
- Google-Extended controls Gemini training and grounding and "does not impact a site's inclusion in Google Search nor is it used as a ranking signal in Google Search"; Google intends to support the AIPREF training-preference signal in early 2027 ([Google](https://developers.google.com/search/docs/crawling-indexing/google-common-crawlers), H, primary). AI Overviews and AI Mode need only an indexed page eligible for a snippet, with no special files, markup or schema ([Google](https://developers.google.com/search/docs/appearance/ai-features), H).
- The SIGIR 2026 association between blocking Google's AI crawler and less AI Overview retrieval is partly about which sites block: 60.0% of reputable news sites disallow at least one AI crawler, against 9.1% of misinformation sites ([Steinacker-Olsztyn et al., WWW 2026](https://arxiv.org/abs/2510.10315), H). Don't unblock Google-Extended expecting AI Overview gains.
- In 200 controlled trials across ten assistants, Claude and Mistral consistently followed robots.txt, while Gemini and Copilot accessed disallowed pages ([Lopez-Fonseca et al. 2026](https://arxiv.org/abs/2607.14447), M).
- In June 2026, 52% of crawler requests were for AI training (22% in spring 2025), mixed-use crawlers were over 36% of activity, and Google sent about 88% of referrals ([Cloudflare 2026](https://blog.cloudflare.com/agentic-internet-bot-report/), H, self-reported).

### 10.8 Clicks
- In a field experiment with 1,065 users, AI Overviews cut organic clicks by 38% where they appeared, and zero-click searches rose from 54% to 72% (Agarwal & Sen, SSRN working paper, as reported by [Search Engine Journal](https://www.searchenginejournal.com/ai-overviews-cut-organic-clicks-38-field-study-finds/573145/), M).
- In a preregistered field experiment with 1,100 US Google users (March 2026), an AI Mode-only search cut click-through by 18.8 percentage points (Reddit 21.2, news 12.5, Wikipedia 9.9) and lowered trust in information found on Google; removing AI Overviews and AI Mode raised click-through by 8.8 points ([Wang et al. 2026](https://arxiv.org/abs/2608.18352), M).

**Implication:** in AI answers, visibility increasingly means being named rather than visited, so measure mentions as well as traffic.

### 10.9 What this changed in LLM Optimizer
- **Answer optimization:** Content Authority scores whether the title and a key paragraph match the buyer's question and whether the page carries the evidence assistants use (figures, definitions, comparisons, steps, explicit prices). Structural Optimization no longer credits FAQ blocks, summary boxes or author bylines by themselves. Source Authority counts third-party "best X" lists, never a brand's own self-ranked lists. Page scores are described as quality and fit for the question, not predicted citation.
- **LLM Test:** sentiment is reported separately rather than folded into the score; results are labelled as API answers; with web search on, each miss is classified as never found, found but not named, or named but not recommended; differences on a single question are marked indicative.
- **Guidance:** Google-Extended doesn't affect Google Search or AI Overviews; robots.txt states intent but isn't enforcement; AIPREF is coming; keep pricing pages explicit and current; compare with peers of similar size.
- **Unchanged:** the LLM Visibility Score and pillar weights.
