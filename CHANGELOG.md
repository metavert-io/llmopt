# Changelog

## 2026-11-14

- **LLM Test / Claude**: answers from Claude 5-generation models (Opus 5.5, Sonnet 5.5, Haiku 5.5, Fable, Mythos) came back empty. These models think before answering and return thinking blocks ahead of the text, and the response reader took only the first block. It now joins all text blocks, adds output headroom for the thinking (as for the other providers' reasoning models), and reports a refusal or an answer cut off by the budget as a failure. Any blank answer from any provider now counts as a failed query instead of being scored as "brand not mentioned".
- **LLM Test / OpenAI**: the default model is now `chat-latest`, OpenAI's alias for the model ChatGPT's Chat tab answers with, instead of the newest API model (frontier models such as GPT-6.1 Terra aren't served in ChatGPT's chat). Without the alias, the newest `*-chat-latest` model is used. Other models can still be picked; the picker says why its default was chosen. The evaluation step still uses the best model.
- **LLM Test**: test queries are saved per domain (`GET`/`PUT /api/test/{domain}/queries`, collection `test_query_sets`) shortly after each edit, when the page is hidden, and when a test starts, so they are there again after leaving the page. Before anything is saved, a domain shows its latest test's queries; suggestions are generated only for a domain that has never had any. "Re-run Test" (and "Re-run or Change Settings") on a results page starts from exactly the queries shown in those results.
- **Hosting**: the machine now decides when to stop instead of Fly's proxy, which stopped it after a few idle minutes, sometimes mid-way through someone's work. With `LLMOPT_IDLE_SHUTDOWN=60m` (set in `fly.saas.toml`), it stays up until 60 minutes pass without a signed-in request (`LLMOPT_IDLE_SHUTDOWN_VISITOR`, 10 minutes, for anonymous visits and for the boot itself), never stops while a request is in flight, and ignores the health polls open tabs send every 5 minutes. It then asks supervisord to shut down; the machine stops cleanly (restart policy `on-failure`), and Fly's proxy starts it on the next request. `auto_stop_machines` is now `off`.

## 2026-11-13

- **LLM Test**: a provider that can't be used no longer stops the test. Providers without a key are skipped with a note. A provider whose key is rejected or out of credit is dropped after its first failure, and the rest of its queries are skipped. Rate-limit and overload errors are retried with backoff. A retired model makes the run switch to the provider's next model. Providers that answered nothing are listed in a new `unavailable_providers` field (shown on the results page and in the PDF) instead of counting as zero in the overall score. Per-provider rates are now computed over the queries that provider answered (`answered`/`failed`).
- **LLM Test**: the evaluation step falls back to another provider that just answered when the primary provider can't evaluate, and reads evaluator output wrapped in prose or code fences.
- **Models**: the model list for each provider is now fetched live from the provider (Anthropic, OpenAI, Gemini, xAI `models` endpoints) for the tenant's own key, cached for 6 hours, and ranked best first. The LLM Test defaults to the best available model. The built-in lists remain as a fallback.
- **API keys**: key checks list the provider's models first and test with the cheapest current one, so a retired model no longer makes a valid key look broken. The LLM Test page shows keys whose last check failed, with a Re-check button, instead of hiding them.
- **OpenAI**: requests to the Responses API now send `input` and `max_output_tokens` (they were sending the Chat Completions `messages`/`max_tokens` fields). Reasoning models (GPT-5, o-series), Gemini 2.5+ thinking models and Grok reasoning models get extra output budget so their reasoning doesn't use up the answer. Exhausted OpenAI quota is reported as a billing problem, not as an overload.
- **Gemini**: the API key is sent in the `x-goog-api-key` header instead of the URL, so it can't appear in error text. Multi-part answers are joined.
- **Cold starts**: Caddy now holds requests for up to 45s while the backends start (`lb_try_duration`), and buffers llmopt request bodies so a POST that arrives during startup is replayed instead of failing with 502. WARP connects in the background so the app starts immediately, and the WARP check is retried instead of being cached as unavailable. The frontend retries a test start on 502/503/504 with a "waking up" note, and explains other failures (rate limit, expired session) instead of showing "Failed to start test".
- **LLM Test**: fixed the model picker being ignored (stale `testModelSelections` in the run callback).

## 2026-06-18

- **LLM Test**: added a periodic SSE heartbeat to all 15 streaming endpoints, keeping the connection alive through long, silent LLM calls. Fixes the "Load Failed" error that dropped the Test stream mid-run behind the production proxy chain (Fly edge -> Caddy).
- **LLM Test**: Phase 1 provider queries now run concurrently (capped at 6) instead of sequentially, cutting a large provider×query matrix from the sum of all calls to roughly the slowest single call. The overall request timeout was raised from 5 to 15 minutes. Together these fix the "Could not resolve primary LLM for evaluation" error that occurred when a long run exhausted the request budget before the evaluation step.
- **API keys**: `resolveProviderKey`/`resolvePrimaryLLM` no longer map transient DB/context errors to `api_key_required`; a genuinely-missing key returns a sentinel error while real failures are wrapped and logged. The Test evaluation also falls back to a tested provider's key when the primary provider has none configured.
- **Report**: the aggregate PDF's "YouTube Video Authority" section now renders the brand narrative ("What LLMs Probably Believe About You") with sentiment, key themes, share of voice, content-gap opportunities, top creators, creator outreach targets, and the confidence note — matching the YouTube tab instead of showing only scores and scorecard titles.

## 2026-06-05

- **Video Authority**: added warning banner explaining that YouTube's bot detection frequently blocks transcript fetching from datacenter IPs (including the hosted instance at llmopt.fly.dev), and recommending self-hosting on a residential connection for reliable results.

## 2026-06-03

- **Video Authority**: added `yt-dlp` + `deno` based transcript-fetcher as the primary method, with the existing InnerTube clients (TVHTML5 embed, MWEB, WEB, ANDROID, watch-page scrape) retained as fallbacks. yt-dlp's EJS challenge solver is enabled via `--remote-components ejs:github` so the JS-challenge step succeeds when the egress IP is residential.
- **Video Authority**: sticky preference cache for transcript methods — once a method succeeds it floats to the front of the list for subsequent calls in the same process.
- **Video Discovery**: added opt-out checkbox for auto-generated brand-name search queries (`<brand> review`, `<brand> tutorial`, `<brand> vs <competitor>`) to avoid false positives when the brand name is a common phrase.
- **Video Discovery**: errors from individual YouTube search calls are now surfaced via the progress stream and logged, instead of being silently swallowed.
- **Rate limiting**: keyed by `(client IP, request path)` so per-endpoint limits don't share a counter. `VideoDiscoverLimit` and `VideoAnalyzeLimit` bumped from 3/min to 10/min.
