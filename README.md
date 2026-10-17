# LLM Optimizer

[![Go](https://github.com/metavert-io/llmopt/actions/workflows/ci.yml/badge.svg)](https://github.com/metavert-io/llmopt/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/metavert-io/llmopt/branch/master/graph/badge.svg)](https://codecov.io/gh/metavert-io/llmopt)
[![Go Report Card](https://goreportcard.com/badge/github.com/metavert-io/llmopt)](https://goreportcard.com/report/github.com/metavert-io/llmopt)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

LLM Optimizer is an AI visibility intelligence platform. It analyzes how large language models and AI search engines perceive, cite, and recommend brands — then provides research-backed optimization strategies to improve that visibility.

ChatGPT, Claude, Gemini, Perplexity, and Google AI Overviews are replacing traditional search for millions of people. The signals that determine whether an AI recommends your brand are fundamentally different from traditional SEO: earned media coverage, transcript quality, content structure, training data frequency, and citation network dynamics matter more than backlinks and keyword density. LLM Optimizer measures these signals across five analysis dimensions and produces a composite AI Visibility Score (0-100) with prioritized, actionable recommendations.

## What It Analyzes

LLM Optimizer performs six types of analysis, each grounded in peer-reviewed research:

**Answer Engine Optimization** — Analyzes your website's content against the optimization strategies validated by the [GEO (Generative Engine Optimization)](https://arxiv.org/abs/2311.09735) research. Scores pages on quotation density (+41% visibility), statistical evidence (+33%), source citations (+28%), fluency, structural optimization, and machine readability. Produces per-question optimization scores with specific rewrite recommendations.

**Video Authority Analysis** — Two-phase analysis of YouTube presence. Phase 1 uses a fast model to assess individual videos for transcript quality, keyword alignment, and caption availability. Phase 2 feeds compact assessments into a reasoning model for four-pillar scoring: Transcript Authority, Topical Dominance, Citation Network, and Brand Narrative. Based on research showing YouTube is now the [#1 social citation source](https://www.adweek.com/media/youtube-reddit-ai-search-engine-citations) for LLMs, appearing in 16% of AI answers.

**Reddit Authority Analysis** — Scrapes Reddit discussions mentioning your brand and analyzes community sentiment, competitive positioning, and training data signal strength. Uses Reddit's public `.json` endpoints with Cloudflare WARP proxy fallback. Scores four pillars: Presence, Sentiment, Competitive Position, and Training Signal.

**Search Visibility Analysis** — Evaluates your site's visibility across both Google AI Overviews and standalone LLMs. Checks robots.txt AI crawler policies, structured data, content freshness, brand search momentum, and earned media signals. Based on research showing only [12% overlap](https://ahrefs.com/blog/ai-search-traffic-study/) between Google top-10 results and ChatGPT/Perplexity citations.

**LLM Knowledge Testing** — Directly queries multiple LLM providers (Anthropic, OpenAI, Gemini, Grok) with your brand's target queries and analyzes how each model responds. Compares your brand's presence, accuracy, and recommendation likelihood across providers. Supports head-to-head competitor comparison.

**Brand Intelligence** — Aggregates all analysis dimensions into a composite AI Visibility Score weighted across Optimization (30%), Video Authority (20%), Reddit Authority (20%), Search Visibility (15%), and LLM Test (15%). Generates prioritized action items that track through to completion.

## Research Foundation

The analysis methodology is grounded in published research. Key findings that inform the scoring:

- **Content optimization**: Embedding authoritative quotations improves AI citation visibility by +41%; adding statistics by +33%; keyword stuffing *reduces* visibility by -9% ([GEO, Princeton/KDD 2024](https://arxiv.org/abs/2311.09735)).
- **Training data frequency**: Answer accuracy more than doubles from rare (1-5 documents) to high-frequency (51+ documents) in training data. Being in training data AND being retrievable provides a compounding advantage ([NanoKnow, 2026](https://arxiv.org/abs/2602.20122)).
- **Source preferences**: AI search engines cite earned media 72-92% of the time vs. 18-27% for brand-owned content. Only 15-50% overlap with traditional Google results ([GEO, Toronto 2025](https://arxiv.org/abs/2509.08919)).
- **Video transcripts**: A 7B-parameter model trained on YouTube transcripts surpassed 72B models in commentary quality. Transcript quality is the dominant signal for video LLM influence — not production value or view counts ([LiveCC, CVPR 2025](https://arxiv.org/abs/2504.16030)).
- **Citation concentration**: Top 20 news sources capture 28-67% of all AI citations depending on provider. Different AI providers cite substantially different sources (cross-family similarity: 0.11-0.58) ([AI Search Arena, 2025](https://arxiv.org/abs/2507.05301)).
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
              ┌───────────────────┐
              │   MongoDB Atlas   │
              │                   │
              │  - analyses       │
              │  - brand_profiles │
              │  - video_analyses │
              │  - reddit_cache   │
              │  - health_checks  │
              │  - tenants / users│
              └───────────────────┘
```

**Backend** — Go 1.24, standard library `net/http` with `gorilla/mux`-style routing. No web framework. LLM provider abstraction supports Anthropic, OpenAI, Gemini, and Grok with streaming SSE responses. Each provider implements a common interface for `Call`, `Stream`, `VerifyKey`, and `BuildStreamBody`.

**Frontend** — React 19 + TypeScript + Vite + Tailwind CSS. Single-page application with SSE streaming for real-time analysis progress. The SaaS deployment uses a frontend overlay system that extends the base [LastSaaS](https://github.com/metavert-io/lastsaas) frontend with product-specific pages.

**Multi-tenant SaaS** — Built on [LastSaaS](https://github.com/metavert-io/lastsaas), an open-source SaaS framework that provides authentication, billing (Stripe), tenant isolation, and user management. LLM Optimizer runs as a dependent application — LastSaaS handles the auth/billing plane while LLM Optimizer handles the product plane.

**MCP Server** — [Model Context Protocol](https://modelcontextprotocol.io) server using Streamable HTTP transport with OAuth 2.1 (PKCE + Dynamic Client Registration). Lets AI assistants like Claude access analysis data, visibility scores, and action items programmatically.

**Cloudflare WARP** — Integrated as a SOCKS5 proxy for Reddit scraping fallback (handles 429/403 rate limits).

## Prerequisites

- **Go 1.24+**
- **Node.js 20+** and npm
- **MongoDB** (Atlas recommended, local works for development)
- **Anthropic API key** (required — used for site analysis, optimization scoring, and brand intelligence)
- **[LastSaaS](https://github.com/metavert-io/lastsaas)** (required for SaaS mode with auth/billing; optional for standalone mode)

Optional API keys for additional providers and features:
- **OpenAI API key** — LLM knowledge testing with GPT models
- **Google Gemini API key** — LLM knowledge testing with Gemini models
- **xAI Grok API key** — LLM knowledge testing with Grok models
- **YouTube Data API key** — Video authority analysis

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
│   ├── youtube.go           # YouTube Data API client, video analysis
│   ├── reddit.go            # Reddit scraper with WARP fallback
│   ├── report_pdf.go        # PDF report generation
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
