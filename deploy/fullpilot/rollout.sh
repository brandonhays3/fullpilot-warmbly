#!/bin/bash
# Roll a built image tag out to production:
#   1. control plane VM: point .env at our registry + tag, pull, recreate
#   2. Cloud Run worker jobs in every region: switch to the new worker image
#
#   TAG=fp-abc1234 npm run deploy
#
# Requires: gcloud logged in as an owner of data-286013. The VM pulls from
# Artifact Registry with a short-lived access token minted here, so the VM
# itself needs no extra IAM scopes.
set -euo pipefail
cd "$(dirname "$0")/../.." && source deploy/fullpilot/config.sh

echo "Rolling out ${IMAGE_PREFIX}/*:${TAG}"

# Refuse to deploy a tag that was never pushed.
if ! gcloud artifacts docker images describe "${IMAGE_PREFIX}/worker:${TAG}" --project "$PROJECT" >/dev/null 2>&1; then
  echo "No image ${IMAGE_PREFIX}/worker:${TAG} in Artifact Registry. Run 'npm run images' first." >&2
  exit 1
fi

TOKEN=$(gcloud auth print-access-token)

echo "== control plane VM (${VM})"
gcloud compute ssh "$VM" --project "$PROJECT" --zone "$ZONE" --tunnel-through-iap --quiet --command "
  set -e
  cd /opt/warmbly
  echo '$TOKEN' | sudo docker login -u oauth2accesstoken --password-stdin ${AR_HOST} >/dev/null
  sudo cp .env .env.bak.\$(date +%Y%m%d%H%M%S)
  sudo sed -i 's|^WARMBLY_IMAGE_PREFIX=.*|WARMBLY_IMAGE_PREFIX=${IMAGE_PREFIX}|' .env
  sudo sed -i 's|^WARMBLY_TAG=.*|WARMBLY_TAG=${TAG}|' .env
  sudo docker compose -p warmbly pull --quiet
  sudo docker compose -p warmbly up -d --remove-orphans
  sleep 8
  sudo docker compose -p warmbly ps --format 'table {{.Name}}\t{{.Status}}'
"

echo "== Cloud Run worker jobs"
for region in $WORKER_REGIONS; do
  if gcloud run jobs describe warmbly-worker --region "$region" --project "$PROJECT" >/dev/null 2>&1; then
    gcloud run jobs update warmbly-worker --region "$region" --project "$PROJECT" \
      --image "${IMAGE_PREFIX}/worker:${TAG}" --quiet >/dev/null
    echo "  ${region}: warmbly-worker -> ${TAG}"
  else
    echo "  ${region}: no warmbly-worker job (skipped)"
  fi
done

echo
echo "Done. Dashboard: $(gcloud compute ssh "$VM" --project "$PROJECT" --zone "$ZONE" --tunnel-through-iap --quiet --command "sudo grep '^APP_URL=' /opt/warmbly/.env | cut -d= -f2-" 2>/dev/null)"
