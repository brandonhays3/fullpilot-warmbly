#!/bin/bash
# Shared settings for Fullpilot's Warmbly deploy scripts. Source this file.
# No secrets here: they live in /opt/warmbly/.env on the VM and in Secret Manager.

export PROJECT="data-286013"
export VM="warmbly-cp"
export ZONE="us-central1-a"
export AR_HOST="us-docker.pkg.dev"
export IMAGE_PREFIX="${AR_HOST}/${PROJECT}/fullpilot"

# Cloud Run worker jobs: one per region, all named warmbly-worker.
export WORKER_REGIONS="us-central1 us-east1 us-west1 europe-west1"

# All images that make up a release, in the order the compose file names them.
export SERVICES="backend consumer worker forms updater realtime tracking web admin"

# Tag defaults to fp-<short sha>[-dirty]. Override with TAG=... on the command line.
git_tag() {
  local sha dirty=""
  sha=$(git rev-parse --short HEAD 2>/dev/null || echo "nogit")
  [ -n "$(git status --porcelain 2>/dev/null)" ] && dirty="-dirty"
  echo "fp-${sha}${dirty}"
}
export TAG="${TAG:-$(git_tag)}"
