# LLM Optimizer

[![Go](https://github.com/metavert-io/llmopt/actions/workflows/ci.yml/badge.svg)](https://github.com/metavert-io/llmopt/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/metavert-io/llmopt/branch/master/graph/badge.svg)](https://codecov.io/gh/metavert-io/llmopt)
[![Go Report Card](https://goreportcard.com/badge/github.com/metavert-io/llmopt)](https://goreportcard.com/report/github.com/metavert-io/llmopt)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

LLM Optimizer is an AI visibility intelligence platform. It analyzes how large language models and AI search engines perceive, cite, and recommend brands — then provides research-backed optimization strategies to improve that visibility.

https://github.com/user-attachments/assets/9ae56cae-e287-463f-ba18-c8e0aee916e9

ChatGPT, Claude, Gemini, Perplexity, and Google AI Overviews are replacing traditional search for millions of people. The signals that determine whether an AI recommends your brand are fundamentally different from traditional SEO: earned media coverage, transcript quality, content structure, training data frequency, and citation network dynamics matter more than backlinks and keyword density. LLM Optimizer measures these signals across five analysis dimensions and produces a composite AI Visibility Score (0-100) with prioritized, actionable recommendations.

## What It Analyzes

LLM Optimizer performs six types of analysis, each grounded in published research (peer-reviewed where available; vendor studies are flagged in [research.md](research.md)):

**Answer Engine Optimization** — Scores how well your pages fit and answer the questions buyers ask: whether a page's title and key paragraph match the question, whether it answers directly and early, and whether it carries the evidence assistants use (figures, definitions, comparisons, steps, explicit prices), plus third-party coverage and how widely the answer is known. These are quality and fit scores, not predicted citation rates. The 2023 [GEO](https://arxiv.org/abs/2311.09735) rewriting tactics (adding quotations or statistics) are no longer scored: 2025–2026 replications found they don't move citation on live engines, and FAQ blocks or summary boxes get no credit by themselves. Produces per-question scores with specific recommendations.

**Video Authority Analysis** — Two-phase analysis of YouTube presence. Phase 1 uses a fast model to assess individual videos for transcript quality, keyword alignment, and caption availability. Phase 2 feeds compact assessments into a reasoning model for four-pillar scoring: Transcript Authority, Topical Dominance, Citation Network, and Brand Narrative. Based on research showing YouTube is the [most-cited domain in Google AI Overviews](https://ahrefs.com/blog/most-cited-domains-ai-overviews/) and that video LLMs learn from transcripts, not pictures.

**Reddit Authority Analysis** — Finds Reddit discussions mentioning your brand and analyzes community sentiment, competitive positioning, and training data signal strength. Uses Reddit's public `.json` endpoints with a Cloudflare WARP proxy fallback; Reddit now refuses most anonymous searches, so with an OpenAI key, discovery falls back to OpenAI web search limited to reddit.com and keeps only threads that appear among the search's own sources. Scores four pillars: Presence, Sentiment, Competitive Position, and Training Signal. Reddit's share of AI citations fell in 2026, but assistants still [read it far more than they cite it](https://ahrefs.com/blog/why-chatgpt-cites-pages/).

**Search Visibility Analysis** — Evaluates your site's visibility across both Google AI Overviews and standalone LLMs. Checks AI crawler access by purpose (blocking training crawlers is a business choice; blocking the crawlers that fetch pages for AI answers hides a site), structured data, content freshness, brand search momentum, and earned media signals. Based on research showing only [12% overlap](https://ahrefs.com/blog/ai-search-traffic-study/) between Google top-10 results and ChatGPT/Perplexity citations.

**LLM Knowledge Testing** — Directly queries multiple LLM providers (Anthropic, OpenAI, Gemini, Grok) through their APIs with your brand's target queries and analyzes how each model responds. AI answers vary from run to run, so each query can be asked 1–5 times; scores are mention and recommendation rates over all runs, with the range between runs. An optional web-search pass asks each query once more with each provider's web search, shows what assistants find next to what they remember, and records which sites they cite (yours or third parties'). In the web-search pass, each miss is classified as not found (content, indexing or crawler access), found among the sources but not named (positioning and reputation), or named but not recommended (competitive standing), and the mention rate is compared between answers that cited your own site and answers that didn't. Sentiment is reported separately from the score, and results are API answers, which can differ from what consumer apps show. A slow provider can't stall a test: answers have their own deadline, and calls still unfinished then count as unanswered. Supports head-to-head competitor comparison against brands of similar size.

**Brand Intelligence** — Aggregates all analysis dimensions into a composite AI Visibility Score weighted across Optimization (25%), Video Authority (20%), Reddit Authority (15%), Search Visibility (15%), and LLM Test (25%); Search Visibility itself weighs AI Overview Readiness 25%, Crawl Accessibility 20%, Brand Momentum 30% and Content Freshness 25%. The weights are applied whenever a score is read, so saved analyses always use the current ones. An executive summary synthesizes every channel, the LLM Test and the weighted score. Generates prioritized action items that track through to completion.

**Keeping advice current** — When the research the scoring rests on changes, saved results can be brought up to date without re-running them: "Update advice" has your AI provider rewrite only the advice the current research no longer supports (for example FAQ or schema markup sold as an AI citation lever, or outdated figures), leaving scores and site findings as measured. Each updated result lists what changed, and the original is kept in a backup collection.

**Reports** — A downloadable report for each domain, laid out as an HTML page with score cards and charts and printed to PDF by headless Chromium (falling back to a plain PDF without a browser). It covers the LLM Visibility Score, every analysis, the LLM Test's runs and web-search diagnosis, prioritized recommendations, and the supporting research with its bibliography. The same report opens as a web page, including from public share links, and a cached PDF is regenerated whenever its inputs change.

## Research Foundation

The analysis methodology is grounded in published research, reviewed against 2026 findings in November 2026. Key findings that inform the scoring:

- **Fit to the question, not rewriting tricks**: the 2023 GEO lab gains from adding quotations (+41%) or statistics (+33%) didn't hold on live engines in 2025–2026 replications ([C-SEO Bench](https://arxiv.org/abs/2506.11097); [a re-test on ten engine families](https://arxiv.org/abs/2609.07559), vendor-affiliated). Comparing pages of the same website, FAQ blocks, summary boxes and schema showed no citation effect, while a title and key paragraph that match the question did ([Chen & Koudas, 2026](https://arxiv.org/abs/2610.15033)).
- **Third parties dominate citations, but owned pages count**: AI search engines cite independent sources 72-92% of the time ([GEO, Toronto 2025](https://arxiv.org/abs/2509.08919)), much of it other companies' pages and listicles, yet a brand is far more likely to be named when one of its own pages is retrieved ([Tannenbaum, 2026](https://arxiv.org/abs/2609.23162), author's company data).
- **Training data and familiarity**: on small open models, how often an answer appears in the training data strongly affects whether a model knows it ([NanoKnow, SIGIR 2026](https://arxiv.org/abs/2602.20122)), and well-known brands are cited and recommended disproportionately ([Monaco, Lenti, Pierri, 2026](https://arxiv.org/abs/2611.07614)), so brands are best compared with peers of similar size.
- **Video transcripts**: A 7B-parameter model trained on YouTube transcripts surpassed 72B models in commentary quality. Transcript quality is the dominant signal for video LLM influence — not production value or view counts ([LiveCC, CVPR 2025](https://arxiv.org/abs/2504.16030)).
- **Citation concentration**: Top 20 news sources capture 28-67% of all AI citations depending on provider. Different AI providers cite substantially different sources (cross-family similarity: 0.11-0.58) ([AI Search Arena, 2025](https://arxiv.org/abs/2507.05301)).
- **Variance**: AI answers change from run to run, even at temperature 0 ([Kirsten et al., Findings of ACL 2026](https://aclanthology.org/2026.findings-acl.526/)), so visibility is measured as rates over repeated runs; a few runs suffice for brand-level rates, but single-question differences need far more ([Dice Roll Method, 2026](https://arxiv.org/abs/2609.04047)).
- **Crawlers**: training crawlers and the crawlers that fetch pages for AI answers are separate ([OpenAI](https://developers.openai.com/api/docs/bots)); Google says Google-Extended doesn't affect Google Search ([Google](https://developers.google.com/search/docs/crawling-indexing/google-common-crawlers)); and robots.txt isn't always obeyed by assistants ([Lopez-Fonseca et al., 2026](https://arxiv.org/abs/2607.14447)).
- **Content freshness**: AI assistants cite content 25.7% newer than traditional search. Freshness signals can shift ranking by up to 95 positions ([Ahrefs, 2025](https://ahrefs.com/blog/freshness-seo/)).

For the complete research synthesis with methodology details, scoring frameworks, and prompt architecture, see [research.md](research.md).

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Caddy (port 8080)                   │
│                   TLS + reverse proxy                   │
└──────────┬────────────────────────────┬─────────────────┘
           │                            │
    /api/auth, /api/users,       Everything else
    /api/tenants, /api/billing,
    /api/admin, /api/bootstrap
           │                            │
           ▼                            ▼
┌─────────────────────┐    ┌──────────────────────────────┐
│  LastSaaS Backend   │    │   LLM Optimizer Backend      │
│    (port 8091)      │    │       (port 8090)            │
│                     │    │                              │
│  - Authentication   │    │  - Site analysis engine      │
│  - OAuth / SSO      │    │  - Video analysis (2-phase)  │
│  - Billing (Stripe) │    │  - Reddit analysis           │
│  - Tenant management│    │  - Search visibility         │
│  - API key mgmt     │    │  - LLM knowledge testing     │
│  - User management  │    │  - Brand intelligence        │
│                     │    │  - MCP server (OAuth 2.1)    │
│                     │    │  - REST API (v1)             │
│                     │    │  - PDF report generation     │
│                     │    │  - Health monitoring          │
│                     │    │  - Cloudflare WARP proxy     │
└─────────────────────┘    └──────────────────────────────┘
           │                            │
           └────────────┬───────────────┘
                        ▼
            ┌──────────────────────┐
            │    MongoDB Atlas     │
            │                      │
            │  - analyses          │
            │  - optimizations     │
            │  - brand_profiles    │
            │  - video_analyses    │
            │  - reddit_analyses   │
            │  - search_analyses   │
            │  - llm_tests         │
            │  - domain_summaries  │
            │  - todos             │
            │  - report_pdfs       │
            │  - guidance_backups  │
            │  - tenants / users   │
            └──────────────────────┘
```

**Backend** — Go 1.24, standard library `net/http` with `gorilla/mux`-style routing. No web framework. LLM provider abstraction supports Anthropic, OpenAI, Gemini, and Grok with streaming SSE responses. Each provider implements a common interface for `Call`, `Stream`, `VerifyKey`, and `BuildStreamBody`, plus a web-search call (each provider's own search tool) used by the LLM Test.

**Idle shutdown** — In the SaaS deployment the machine stops itself after a period without signed-in work and starts again on the next request. Caddy sends the LastSaaS routes through a small tracking proxy inside the llmopt process (port 8092, falling back to LastSaaS on 8091 directly), so sign-in and billing traffic count as activity.

**Frontend** — React 19 + TypeScript + Vite + Tailwind CSS. Single-page application with SSE streaming for real-time analysis progress. The SaaS deployment uses a frontend overlay system that extends the base [LastSaaS](https://github.com/metavert-io/lastsaas) frontend with product-specific pages.

**Multi-tenant SaaS** — Built on [LastSaaS](https://github.com/metavert-io/lastsaas), an open-source SaaS framework that provides authentication, billing (Stripe), tenant isolation, and user management. LLM Optimizer runs as a dependent application — LastSaaS handles the auth/billing plane while LLM Optimizer handles the product plane.

**MCP Server** — [Model Context Protocol](https://modelcontextprotocol.io) server using Streamable HTTP transport with OAuth 2.1 (PKCE + Dynamic Client Registration). Lets AI assistants like Claude access analysis data, visibility scores, and action items programmatically.

**Cloudflare WARP** — Integrated as a SOCKS5 proxy for Reddit scraping fallback (handles 429/403 rate limits).

## Prerequisites

- **Go 1.24+**
- **Node.js 20+** and npm
- **MongoDB** (Atlas recommended, local works for development)
- **Anthropic API key** (required — used for site analysis, optimization scoring, and brand intelligence; analyses run on Claude Opus 5.5, falling back to Sonnet 5.5)
- **[LastSaaS](https://github.com/metavert-io/lastsaas)** (required for SaaS mode with auth/billing; optional for standalone mode)

Optional API keys for additional providers and features:
- **OpenAI API key** — LLM knowledge testing with GPT models, and Reddit thread discovery through web search when Reddit blocks direct search
- **Google Gemini API key** — LLM knowledge testing with Gemini models
- **xAI Grok API key** — LLM knowledge testing with Grok models
- **YouTube Data API key** — Video authority analysis
- **Chrome or Chromium** — prints the PDF report (set `LLMOPT_CHROME_PATH` if it isn't on `PATH`)

## Setup

### Standalone Mode (single user, no auth)

1. Clone the repository:
   ```bash
   git clone https://github.com/metavert-io/llmopt.git
   cd llmopt
   ```

2. Copy the example environment file and fill in your values:
   ```bash
   cp .env.dev.example .env
   ```

   Required variables:
   ```
   ANTHROPIC_API_KEY=sk-ant-api03-your-key-here
   MONGODB_URI=mongodb+srv://user:pass@cluster.mongodb.net/?appName=Cluster0
   PORT=8080
   ```

3. Build and run the backend:
   ```bash
   cd backend
   go build -o llmopt .
   ./llmopt
   ```

4. Build and run the frontend (in a separate terminal):
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

5. Open `http://localhost:5173` in your browser.

### SaaS Mode (multi-tenant with auth and billing)

SaaS mode requires [LastSaaS](https://github.com/metavert-io/lastsaas) as a dependency. LastSaaS provides authentication (OAuth/SSO), Stripe billing, tenant management, and the admin interface.

1. Clone with the LastSaaS dependency:
   ```bash
   git clone https://github.com/metavert-io/llmopt.git
   cd llmopt
   git clone https://github.com/metavert-io/lastsaas.git lastsaas
   ```

2. Configure environment variables:
   ```bash
   cp .env.prod.example .env
   ```

   Additional variables required for SaaS mode:
   ```
   LLMOPT_SAAS_ENABLED=true
   LLMOPT_ENCRYPTION_KEY=<32-byte-hex-key-for-tenant-api-key-encryption>
   LLMOPT_JWT_ACCESS_SECRET=<random-secret-for-jwt-signing>
   ```

   See the [LastSaaS README](https://github.com/metavert-io/lastsaas#readme) for the full set of auth/billing environment variables (Google OAuth, Stripe keys, etc.).

3. For local development, use the start script:
   ```bash
   ./start-saas.sh
   ```

4. For production deployment on Fly.io:
   ```bash
   fly deploy -c fly.saas.toml
   ```
   See [deploy.md](deploy.md) for the full deployment guide.

### Local Copy of a Hosted Account

Some features work better from your own machine than from a hosted server. YouTube, for one, often blocks transcript requests from datacenter addresses but allows them from a home connection. Local account mode runs the app on your machine against the hosted service's database, as one account, with no LastSaaS and no sign-in. You see the same domains, reports and saved API keys, and whatever you create there shows up on the hosted service.

```bash
LLMOPT_LOCAL_USER_ID=<user _id>  LLMOPT_LOCAL_TENANT_ID=<tenant _id> \
MONGODB_URI=... LLMOPT_ENCRYPTION_KEY=... ANTHROPIC_API_KEY=... YOUTUBE_API_KEY=... \
STATIC_DIR=../frontend/dist PORT=4281 ./llmopt
```

- Use the hosted service's `MONGODB_URI` and `LLMOPT_ENCRYPTION_KEY`. The key is needed to read the account's saved provider keys.
- The user must be a member of the tenant (`tenant_memberships`). The server checks this at startup and on every request.
- Every request acts as that account, so the server listens on `127.0.0.1` and refuses any non-loopback `LLMOPT_LISTEN_HOST`. To reach it from your other devices, put a private tunnel in front of it, such as `tailscale serve`, and list the address you open it at in `LLMOPT_LOCAL_ORIGINS` (for example `https://my-mac.example.ts.net:8448`). Never expose it on your network.
- It answers only its own pages. It refuses requests addressed to other host names, which blocks DNS rebinding, and requests from other sites (a foreign `Origin`, or `Sec-Fetch-Site: cross-site`). It sends no CORS headers. Without these checks, any web page open in the same browser could act as the account.
- The hosted service owns the database. A local copy runs no migrations and none of the background jobs (cleanup, provider health checks, screenshots).

### Build Verification

```bash
cd backend && go build ./...
cd ../frontend && npx tsc --noEmit
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Anthropic API key for analysis engine |
| `MONGODB_URI` | Yes | MongoDB connection string |
| `PORT` | No | Server port (default: 8080) |
| `DATABASE_NAME` | No | MongoDB database name (default: llmopt) |
| `LLMOPT_SAAS_ENABLED` | SaaS only | Enable multi-tenant SaaS mode |
| `LLMOPT_ENCRYPTION_KEY` | SaaS only | AES key for encrypting tenant API keys |
| `LLMOPT_JWT_ACCESS_SECRET` | SaaS only | HMAC secret for JWT signing |
| `LLMOPT_LOCAL_USER_ID`, `LLMOPT_LOCAL_TENANT_ID` | Local account mode | Act as this account of a hosted database without signing in (see above) |
| `LLMOPT_LISTEN_HOST` | No | Address to listen on (default: all interfaces; `127.0.0.1` and loopback only in local account mode) |
| `LLMOPT_CHROME_PATH` | No | Chrome/Chromium used to print the PDF report (default: found on `PATH`). Without one, the report falls back to a plain PDF |
| `LLMOPT_LOCAL_ORIGINS` | Local account mode | Comma-separated `scheme://host[:port]` addresses the local copy is opened at besides `127.0.0.1`/`localhost` |
| `LLMOPT_IDLE_SHUTDOWN`, `LLMOPT_IDLE_SHUTDOWN_VISITOR` | No | Stop the machine after this long without signed-in work (and anonymous visits); see deploy.md |
| `LLMOPT_LASTSAAS_PROXY`, `LLMOPT_LASTSAAS_UPSTREAM` | SaaS deploy | Listen address for, and LastSaaS URL behind, the proxy that lets idle shutdown see LastSaaS traffic |
| `MCP_JWT_SECRET` | No | MCP OAuth token signing key (derived from encryption key if not set) |
| `BASE_URL` | No | Public URL for OAuth metadata (default: `https://llmopt.metavert.io`) |
| `YOUTUBE_API_KEY` | No | YouTube Data API key for video analysis |

## Project Structure

```
llmopt/
├── backend/
│   ├── main.go              # HTTP server, routes, handlers, health checks
│   ├── models.go            # MongoDB document models
│   ├── db.go                # Database connection, collections, indexes
│   ├── llm.go               # LLM provider abstraction, streaming helpers
│   ├── provider.go          # Provider interface and registry
│   ├── provider_anthropic.go # Anthropic Claude implementation
│   ├── provider_openai.go   # OpenAI GPT implementation
│   ├── provider_gemini.go   # Google Gemini implementation
│   ├── provider_grok.go     # xAI Grok implementation
│   ├── provider_websearch.go # Each provider's web-search call (LLM Test)
│   ├── scoring.go           # Score weights, applied when results are read
│   ├── llmtest_run.go       # LLM Test: per-provider lanes, retries, model switching
│   ├── llmtest_score.go     # LLM Test: runs, spread, web-search diagnosis
│   ├── test_queries.go      # Saved LLM Test queries per domain
│   ├── guidance_refresh.go  # "Update advice" for saved results
│   ├── youtube.go           # YouTube Data API client, video analysis
│   ├── reddit.go            # Reddit scraper with WARP fallback
│   ├── reddit_websearch.go  # Reddit discovery through OpenAI web search
│   ├── report_data.go       # Report data and the LLM Visibility Score
│   ├── report_html.go       # Report view model, Chromium PDF printing
│   ├── report_research.go   # Research digest and bibliography for reports
│   ├── reporttpl/           # Report HTML template and embedded fonts
│   ├── report_pdf.go        # Report endpoints, cache, plain PDF fallback
│   ├── research.go          # Public research/methodology page
│   ├── seo.go               # Public share pages
│   ├── idle.go              # Idle shutdown and the LastSaaS tracking proxy
│   ├── local_account.go     # Local account mode (loopback-only access guard)
│   ├── screenshot.go        # Chromedp-based page screenshots
│   ├── api_v1.go            # REST API v1 handlers
│   ├── crypto.go            # AES encryption for tenant API keys
│   ├── migration.go         # Database migrations
│   ├── telemetry.go         # Anonymous usage telemetry
│   └── internal/
│       ├── mcpserver/       # MCP protocol server (OAuth 2.1 + tools)
│       ├── saas/            # SaaS middleware (JWT, tenant context)
│       └── ratelimit/       # Rate limiting
├── frontend/                # React + TypeScript + Vite + Tailwind
│   └── src/
│       ├── App.tsx          # Main application (analysis UI, dashboards)
│       └── main.tsx         # Entry point
├── frontend-overlay/        # SaaS-specific frontend extensions
│   └── src/
│       ├── pages/           # Settings, API key management
│       └── components/      # SaaS-specific UI components
├── deploy/
│   ├── Caddyfile            # Caddy reverse proxy config
│   └── supervisord.conf     # Process manager config
├── lastsaas/                # LastSaaS dependency (git clone, gitignored)
├── research.md              # Full research synthesis with citations
├── deploy.md                # Production deployment guide
├── Dockerfile               # Standalone mode
├── Dockerfile.saas          # SaaS mode (Caddy + supervisord)
├── fly.toml                 # Fly.io config (standalone)
├── fly.saas.toml            # Fly.io config (SaaS)
└── start-saas.sh            # Local SaaS development script
```

## MCP Server

LLM Optimizer includes a [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that lets AI assistants like Claude access your brand intelligence and optimization data programmatically. The server uses **Streamable HTTP** transport with **OAuth 2.1** authentication.

**Server URL:** `https://llmopt.metavert.io/mcp`

Listed on [Smithery](https://smithery.ai/servers/jonradoff/llmopt) and [awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers) (Glama). Official MCP Registry submission in progress.

### Privacy

The LLM Optimizer MCP server accesses only data already stored in your LLM Optimizer account (domains, reports, visibility scores, and action items). No data is shared with third parties. Read-only tools (`llmopt_list_domains`, `llmopt_get_report`, `llmopt_get_visibility_score`, `llmopt_list_todos`) do not modify any data. The `llmopt_update_todo` tool mutates only todo status fields. API keys and OAuth tokens are transmitted over HTTPS and stored hashed (SHA-256). For full details see the [Privacy Policy](https://llmopt.metavert.io/privacy).

### Authentication

**Option 1 — Access Key (recommended):** Create a personal `lok_` access key at **Settings → Access Keys** in the app. Enter it in the OAuth prompt when connecting. Each key grants full access to your account's MCP tools.

**Option 2 — OAuth 2.1:** Full browser-based OAuth flow with PKCE. Claude Desktop handles this automatically when you add the server URL.

### Available Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `llmopt_list_domains` | List all domains tracked for your account | _(none)_ |
| `llmopt_get_report` | Get a specific analysis report for a domain | `domain` (required), `report_type` (required): analysis, optimizations, video, reddit, search, summary, tests, brand |
| `llmopt_get_visibility_score` | Get the composite AI visibility score (0-100) for a domain, weighted across 5 components | `domain` (required) |
| `llmopt_list_todos` | List action items from optimization analyses | `status` (optional): todo, completed, backlogged, archived; `domain` (optional) |
| `llmopt_update_todo` | Update a todo item's status (admin/owner only) | `id` (required), `status` (required): todo, completed, backlogged, archived |

### Working Examples

**1. Get a quick brand health summary**
> "What's the AI visibility score for acme.com and what are the top recommendations?"

Claude will call `llmopt_get_visibility_score` then `llmopt_get_report` (report_type: summary) to give you a snapshot of how well the brand appears in LLM outputs and what to fix first.

**2. Triage your optimization backlog**
> "Show me all my open todo items across all domains, prioritized by domain."

Claude calls `llmopt_list_todos` with `status: todo`, groups results by domain, and presents them as a prioritized task list — ready to assign or schedule.

**3. Competitive visibility comparison**
> "Compare the analysis reports for acme.com and competitorx.com and tell me where we're losing."

Claude fetches `llmopt_get_report` (report_type: analysis) for both domains, then synthesizes the differences — identifying topics where the competitor is mentioned and you're not.

**4. Morning brand briefing**
> "Give me a daily briefing: any visibility score changes, new todos, and Reddit sentiment for mycompany.com."

Claude chains `llmopt_get_visibility_score`, `llmopt_list_todos` (status: todo), and `llmopt_get_report` (report_type: reddit) to create a concise morning summary you can drop into Slack.

**5. Close out completed work**
> "I've finished all the meta-description updates. Mark all the meta-description todos for acme.com as completed."

Claude calls `llmopt_list_todos` filtered by domain to find relevant items, then calls `llmopt_update_todo` for each, confirming which were updated.

### MCP Configuration

#### Claude Code (HTTP native)

Add to `~/.claude.json` under `mcpServers`:

```json
{
  "mcpServers": {
    "llmopt-mcp": {
      "type": "http",
      "url": "https://llmopt.metavert.io/mcp"
    }
  }
}
```

On first use, Claude Code will open a browser to authorize. Enter your `lok_` access key (create one at **Settings → Access Keys**).

#### Claude Desktop

**Option A — OAuth (easiest, no key to manage):**

```bash
npm install -g mcp-remote
```

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "llmopt-mcp": {
      "command": "npx",
      "args": ["mcp-remote", "https://llmopt.metavert.io/mcp"]
    }
  }
}
```

On first launch, a browser window will open to authorize. Sign in at [llmopt.metavert.io](https://llmopt.metavert.io) and enter your `lok_` access key.

**Option B — Direct key (no browser auth, simpler for power users):**

```json
{
  "mcpServers": {
    "llmopt-mcp": {
      "command": "npx",
      "args": [
        "mcp-remote",
        "https://llmopt.metavert.io/mcp",
        "--header",
        "Authorization: Bearer lok_your_access_key_here"
      ]
    }
  }
}
```

Replace `lok_your_access_key_here` with a key from **Settings → Access Keys**.

#### Direct HTTP

```bash
# List available tools
curl -X POST https://llmopt.metavert.io/mcp \
  -H "Authorization: Bearer lok_your_access_key" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Get visibility score
curl -X POST https://llmopt.metavert.io/mcp \
  -H "Authorization: Bearer lok_your_access_key" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"llmopt_get_visibility_score","arguments":{"domain":"acme.com"}}}'
```

### OAuth Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /.well-known/oauth-protected-resource` | Protected Resource Metadata (RFC 9728) |
| `GET /.well-known/oauth-authorization-server` | Authorization Server Metadata (RFC 8414) |
| `POST /oauth/register` | Dynamic Client Registration (RFC 7591) |
| `GET /oauth/authorize` | Authorization endpoint |
| `POST /oauth/token` | Token endpoint (code exchange + refresh) |

## REST API

A REST API is also available at `/api/v1/`. See the [API Docs](https://llmopt.metavert.io/docs) page for a formatted guide.

## License

[MIT](LICENSE) - Copyright (c) 2026 Metavert LLC
