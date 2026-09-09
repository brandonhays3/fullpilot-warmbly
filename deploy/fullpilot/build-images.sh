#!/bin/bash
# Build all Warmbly images from this checkout on Cloud Build and push them to
# Artifact Registry under us-docker.pkg.dev/data-286013/fullpilot/<service>:<TAG>.
#
#   npm run images              # tag = fp-<short sha>
#   TAG=fp-2026-09-10 npm run images
#
# Builds in the cloud on amd64 (the VM and Cloud Run are amd64), so nothing
# needs Docker locally and an Apple Silicon Mac is fine.
set -euo pipefail
cd "$(dirname "$0")/../.." && source deploy/fullpilot/config.sh

VERSION="${VERSION:-$TAG}"
echo "Building ${SERVICES}"
echo "  -> ${IMAGE_PREFIX}/<service>:${TAG}  (VERSION=${VERSION})"

# .gcloudignore keeps node_modules and build output out of the upload.
gcloud builds submit . \
  --project "$PROJECT" \
  --config deploy/fullpilot/cloudbuild.yaml \
  --substitutions "_PREFIX=${IMAGE_PREFIX},_TAG=${TAG},_VERSION=${VERSION},COMMIT_SHA=$(git rev-parse HEAD 2>/dev/null || echo unknown)"

echo
echo "Pushed ${IMAGE_PREFIX}/*:${TAG}"
echo "Roll it out with:  TAG=${TAG} npm run deploy"
