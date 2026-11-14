#!/bin/bash
set -e

# Start dbus (required by warp-svc)
mkdir -p /run/dbus
dbus-daemon --system --fork 2>/dev/null || true

# Start WARP daemon
warp-svc &
WARP_PID=$!

# Connect WARP in the background so the app starts right away. WARP is only a
# fallback proxy for Reddit/YouTube fetches, and waiting for it here (up to
# ~20s) made every cold start of the machine slower; the app rechecks for the
# proxy when it needs it.
(
  # Wait for daemon to be ready
  for i in $(seq 1 10); do
    if warp-cli --accept-tos status >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done

  # Register (anonymous, no account needed) and set proxy mode
  warp-cli --accept-tos registration new 2>/dev/null || true
  warp-cli --accept-tos mode proxy 2>/dev/null || true
  warp-cli --accept-tos connect 2>/dev/null || true

  # Wait for connection
  for i in $(seq 1 10); do
    STATUS=$(warp-cli --accept-tos status 2>/dev/null | grep -i "status" | head -1 || true)
    if echo "$STATUS" | grep -qi "connected"; then
      echo "Cloudflare WARP connected (proxy on 127.0.0.1:40000)"
      break
    fi
    sleep 1
  done
) &

# Start the app with SaaS env vars
export PORT=8090
export LLMOPT_SAAS_ENABLED=true
export STATIC_DIR=/app/static
export LLMOPT_FRONTEND_DIR=/app/saas-frontend
exec ./llmopt
