#!/bin/bash
# Run an upstream Makefile dev target with Fullpilot's extra settings layered
# on top (Gmail client, redirect mode, ...).
#
#   npm run dev        # make dev: infra in Docker, migrations, seed, then every
#                      # service natively in this terminal (Ctrl-C stops the app)
#   npm run dev:run    # make run: backend + consumer + worker only
#   npm run dev:web    # make web: dashboard on http://localhost:5173
#
# Settings come from .env.fullpilot.local at the repo root (gitignored); see
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
exec make "${1:-dev}" "${@:2}"
