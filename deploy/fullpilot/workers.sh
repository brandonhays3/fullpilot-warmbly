#!/bin/bash
# Cloud Run worker job helpers.
#   npm run workers:status   # last executions per region
#   npm run workers:run      # kick one execution in every region right now
set -euo pipefail
cd "$(dirname "$0")/../.." && source deploy/fullpilot/config.sh

case "${1:-status}" in
  status)
    for region in $WORKER_REGIONS; do
      echo "== $region"
      gcloud run jobs executions list --job warmbly-worker --region "$region" --project "$PROJECT" \
        --limit 3 --format "table(name.basename(),status.completionTime.date('%H:%M:%S'),status.succeededCount,status.failedCount,status.runningCount)" 2>&1 | tail -4
    done
    ;;
  run)
    for region in $WORKER_REGIONS; do
      gcloud run jobs execute warmbly-worker --region "$region" --project "$PROJECT" --quiet >/dev/null 2>&1 \
        && echo "started: $region" || echo "no job in $region"
    done
    ;;
  *) echo "usage: workers.sh [status|run]"; exit 1 ;;
esac
