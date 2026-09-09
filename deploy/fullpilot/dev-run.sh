#!/bin/bash
# Run the Go services locally (backend + consumer + worker, hot reload) with
# Fullpilot's extra settings layered on top of upstream's `make run`.
#
#   npm run dev        # once: postgres/redis/nats/mailpit in Docker + migrations
#   npm run dev:run    # this script, in one terminal
#   npm run dev:web    # dashboard on http://localhost:5173, in another
#
# Put anything Fullpilot-specific (Gmail client, redirect mode, ...) in
# .env.fullpilot.local at the repo root; it is gitignored. See
# deploy/fullpilot/dev.env.example for the keys.
set -euo pipefail
cd "$(dirname "$0")/../.."
if [ -f .env.fullpilot.local ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env.fullpilot.local
  set +a
  echo "loaded .env.fullpilot.local"
else
  echo "no .env.fullpilot.local (copy deploy/fullpilot/dev.env.example to create one)"
fi
exec make run
