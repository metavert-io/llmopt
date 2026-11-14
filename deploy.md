# LLM Optimizer — Deployment Guide

## CRITICAL: Deploy Command

```bash
fly deploy -c fly.saas.toml
```

**NEVER use bare `fly deploy`.** This project depends on LastSaaS for authentication. See `lastsaas/CLAUDE.md` "Dependent Project Deployment" for the full explanation.

## Why This Matters

The production deployment runs **three processes** behind Caddy (managed by supervisord):

1. **Caddy** (port 8080) — reverse proxy, the only externally exposed port
2. **LLM Optimizer backend** (port 8090) — analysis, optimization, all `/api/*` product routes
3. **LastSaaS backend** (port 8091) — auth, OAuth, billing, admin, bootstrap

Caddy routes requests based on path:
- `/api/auth/*`, `/api/users/*`, `/api/tenants/*`, `/api/billing/*`, `/api/admin/*`, `/api/bootstrap/*`, `/health`, etc. → LastSaaS (:8091)
- Everything else → LLM Optimizer (:8090)

If you deploy with bare `fly deploy` (uses `Dockerfile` + `fly.toml`), only the llmopt backend runs. Auth endpoints return HTML from the SPA catch-all, OAuth providers disappear, and users get silently redirected to `/setup`.

## Pre-Deploy Checklist

1. **If lastsaas submodule changed**: push lastsaas to GitHub first, then pull into the submodule:
   ```bash
   cd /Users/jonradoff/lastsaas && git push origin master
   cd /Users/jonradoff/llmopt/lastsaas && git pull origin master
   ```
   The Docker build reads from the submodule directory, which tracks GitHub — not the local lastsaas working copy.

2. **Build check**:
   ```bash
   cd /Users/jonradoff/llmopt/backend && go build ./...
   cd /Users/jonradoff/llmopt/frontend && npx tsc --noEmit
   ```

3. **Deploy**:
   ```bash
   cd /Users/jonradoff/llmopt && fly deploy -c fly.saas.toml
   ```

## Post-Deploy Verification

- Visit https://llmopt.fly.dev/ — standalone frontend loads
- Visit https://llmopt.fly.dev/login — SaaS login page loads with OAuth buttons (Google)
- Check `fly logs -a llmopt` — should see startup lines from both llmopt and lastsaas backends

## Idle Shutdown (instead of Fly auto-stop)

`fly.saas.toml` sets `auto_stop_machines = "off"`; the app stops itself instead (`backend/idle.go`). It stays up for `LLMOPT_IDLE_SHUTDOWN` (60m) after the last signed-in request, or `LLMOPT_IDLE_SHUTDOWN_VISITOR` (10m) after anonymous visits and boot, never during a request, and ignores `/api/health*` polls. Then it sends SIGTERM to its parent supervisord, which exits 0, and the machine stops; the `[[restart]] policy = "on-failure"` keeps it stopped until the proxy starts it on the next request. Look for "Idle shutdown enabled" at startup and "Idle shutdown: ... stopping the machine" when it stops. To go back to Fly's auto-stop, set `auto_stop_machines = "stop"` and remove `LLMOPT_IDLE_SHUTDOWN`.

## Common Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| Redirects to `/setup` | Deployed with wrong Dockerfile (no LastSaaS backend) | Redeploy with `fly deploy -c fly.saas.toml` |
| Login page missing OAuth buttons | LastSaaS backend not running | Same as above |
| Credentials not working | LastSaaS backend not running (auth API returns HTML) | Same as above |
| "App not listening on expected address" warning | Normal — WARP startup delay, Caddy binds after a few seconds | Wait; health checks pass shortly after |

## Architecture Reference

```
fly.saas.toml          → Dockerfile.saas
                          ├── Stage 1: Build llmopt Go binary
                          ├── Stage 2: Build LastSaaS Go binary
                          ├── Stage 3: Build llmopt standalone frontend
                          ├── Stage 4: Build SaaS frontend (LastSaaS + overlay)
                          └── Stage 5: Production runtime (Debian)
                                ├── supervisord (PID 1)
                                │   ├── caddy (:8080)
                                │   ├── llmopt via start-saas.sh (:8090 + WARP)
                                │   └── lastsaas-server (:8091)
                                ├── deploy/Caddyfile (routing rules)
                                └── deploy/supervisord.conf (process config)
```

Fly app: `llmopt` | Region: `ewr` | URL: https://llmopt.fly.dev/
