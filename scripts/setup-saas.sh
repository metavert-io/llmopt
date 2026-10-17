#!/bin/bash
# setup-saas.sh — Clone LastSaaS and prepare the SaaS frontend build.
#
# Run this once after cloning the llmopt repo, and again whenever you
# want to pull a newer version of LastSaaS.
#
# Usage:
#   ./scripts/setup-saas.sh          # clone or update
#   ./scripts/setup-saas.sh --clean  # remove and re-clone

set -euo pipefail
cd "$(dirname "$0")/.."

LASTSAAS_DIR="lastsaas"
REPO="https://github.com/metavert-io/lastsaas.git"
BRANCH="master"

if [ "${1:-}" = "--clean" ]; then
    echo "Removing existing LastSaaS clone..."
    rm -rf "$LASTSAAS_DIR"
fi

if [ -d "$LASTSAAS_DIR" ]; then
    echo "Updating existing LastSaaS clone..."
    cd "$LASTSAAS_DIR"
    git fetch origin
    git reset --hard "origin/$BRANCH"
    cd ..
else
    echo "Cloning LastSaaS from $REPO..."
    git clone --branch "$BRANCH" --single-branch "$REPO" "$LASTSAAS_DIR"
fi

echo ""
echo "LastSaaS is ready at ./$LASTSAAS_DIR/"
echo ""
echo "Next steps:"
echo "  1. Run ./scripts/build-frontend.sh to build the SaaS frontend"
echo "  2. Configure LLMOPT_SAAS_ENABLED=true in your environment"
echo "  3. Start the LastSaaS backend: cd lastsaas/backend && go run ./cmd/server"
echo "  4. Start the llmopt server:  cd backend && go run ."
