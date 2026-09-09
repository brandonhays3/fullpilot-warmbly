#!/bin/bash
# Build Warmbly images from this checkout on Cloud Build and push them to
# Artifact Registry under us-docker.pkg.dev/data-286013/fullpilot/<service>:<TAG>.
#
#   npm run images                       # every service, tag fp-<short sha>
#   npm run images -- web admin          # only the dashboard and admin panel
#   TAG=fp-2026-09-10 npm run images -- backend worker
#
# Builds in the cloud on amd64 (the VM and Cloud Run are amd64), so nothing
# needs Docker locally and an Apple Silicon Mac is fine. Each image is also
# tagged :latest and rebuilt with --cache-from that tag, so an unchanged
# service is a cache hit and only what you touched really compiles.
#
# Partial builds: rollout.sh only switches the services whose image carries
# the tag, so building `web` alone and deploying that tag leaves the other
# services on their current images.
set -euo pipefail
cd "$(dirname "$0")/../.." && source deploy/fullpilot/config.sh

VERSION="${VERSION:-$TAG}"
WANT="${*:-$SERVICES}"

# Dockerfile + build context per service, mirroring upstream's release workflow.
ctx_of()  { case "$1" in tracking) echo tracking ;; web) echo web ;; admin) echo admin ;; *) echo . ;; esac; }
file_of() { case "$1" in tracking) echo tracking/Dockerfile ;; web) echo web/Dockerfile ;; admin) echo admin/Dockerfile ;; *) echo "deploy/docker/$1.Dockerfile" ;; esac; }

for s in $WANT; do
  case " $SERVICES " in *" $s "*) ;; *) echo "unknown service: $s (valid: $SERVICES)" >&2; exit 1 ;; esac
done

CFG=$(mktemp -t warmbly-cloudbuild).yaml
{
  echo "options:"
  echo "  machineType: E2_HIGHCPU_32"
  echo "  logging: CLOUD_LOGGING_ONLY"
  echo "timeout: 3600s"
  echo "steps:"
  for s in $WANT; do
    img="${IMAGE_PREFIX}/${s}"
    cat <<EOF
  - id: ${s}
    name: gcr.io/cloud-builders/docker
    waitFor: ["-"]
    env: ["DOCKER_BUILDKIT=1"]
    entrypoint: bash
    args:
      - -c
      - |
        docker pull ${img}:latest >/dev/null 2>&1 || true
        docker build -f $(file_of "$s") \\
          --cache-from ${img}:latest \\
          --build-arg BUILDKIT_INLINE_CACHE=1 \\
          --build-arg VERSION=${VERSION} \\
          --build-arg COMMIT=$(git rev-parse HEAD 2>/dev/null || echo unknown) \\
          -t ${img}:${TAG} -t ${img}:latest \\
          $(ctx_of "$s")
EOF
  done
  echo "images:"
  for s in $WANT; do
    echo "  - ${IMAGE_PREFIX}/${s}:${TAG}"
    echo "  - ${IMAGE_PREFIX}/${s}:latest"
  done
} > "$CFG"

echo "Building: ${WANT}"
echo "  -> ${IMAGE_PREFIX}/<service>:${TAG}  (VERSION=${VERSION})"

# .gcloudignore keeps node_modules and build output out of the upload.
gcloud builds submit . --project "$PROJECT" --config "$CFG"
rm -f "$CFG"

echo
echo "Pushed ${IMAGE_PREFIX}/{${WANT// /,}}:${TAG}"
echo "Roll it out with:  TAG=${TAG} npm run deploy"
