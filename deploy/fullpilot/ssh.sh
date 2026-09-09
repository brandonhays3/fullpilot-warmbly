#!/bin/bash
# SSH to the control plane VM through IAP. With no args opens a shell;
# with args runs them remotely.
set -e
cd "$(dirname "$0")/../.." && source deploy/fullpilot/config.sh
if [ $# -eq 0 ]; then
  exec gcloud compute ssh "$VM" --project "$PROJECT" --zone "$ZONE" --tunnel-through-iap
else
  exec gcloud compute ssh "$VM" --project "$PROJECT" --zone "$ZONE" --tunnel-through-iap --quiet --command "$*"
fi
