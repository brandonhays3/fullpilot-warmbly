# Fullpilot Warmbly deployment

Self-hosted Warmbly (open-source cold email + warmup) running on Google Cloud
project `data-286013`, set up 2026-09-09. This folder is the runbook. Secrets
are NOT in here; they live in `/opt/warmbly/.env` on the control plane VM and
in Secret Manager.

## Layout

```
Fullpilot-Warmbly/
  README.md            this runbook
  infra/               non-secret config used to build the GCP pieces
    worker-env.yaml    env vars for the Cloud Run worker jobs
    docker-startup.sh  VM startup script that installs Docker
  warmbly/             git clone of the fork (github.com/brandonhays3/fullpilot-warmbly)
```

## Architecture

```
                          Internet
                             |
   users ─────────────> control plane VM (warmbly-cp, us-central1-a)
                        34.45.45.44 public / 10.128.0.2 internal
                        docker compose: backend, consumer, web, admin,
                        realtime, tracking, forms, updater, postgres,
                        redis, nats (+ a local worker)
                             ^
                             |  NATS 4222 + Redis 6379 + backend 8080
                             |  over the VPC (internal IP only)
   Cloud Run Jobs "warmbly-worker" in us-central1, us-east1, us-west1,
   europe-west1. Each run = 2 fresh containers, 5 min lifetime, fresh
   Google egress IP every time. Cloud Scheduler fires each region every
   15 min, staggered.
                             |
   mailbox traffic (Gmail/M365 OAuth on 443, IMAP 993, SMTP 465/587)
   leaves from the worker's rotating IP. Sending IP seen by recipients
   is always the mailbox provider's, never ours.
```

Blob storage: Cloud Storage bucket `warmbly-blobs-data-286013` used through
its S3-compatible endpoint, so control plane and remote workers share files.

Images: Artifact Registry remote repo `ghcr-remote` (location `us`) mirrors
`ghcr.io/warmbly/warmbly/*` because Cloud Run cannot pull from ghcr.io.
Worker image path:
`us-docker.pkg.dev/data-286013/ghcr-remote/warmbly/warmbly/worker:v0.4.1`

## GCP inventory

| Thing | Name | Notes |
|---|---|---|
| Project | data-286013 | billing on |
| VM | warmbly-cp, us-central1-a, e2-standard-2, 50 GB | Ubuntu 24.04, Docker via startup script |
| Static IP | warmbly-cp-ip = 34.45.45.44 | us-central1 |
| Firewall | warmbly-cp-web | tcp 80,443 from anywhere, tag warmbly-cp |
| Firewall | warmbly-cp-direct-brandon | tcp 5173,5174,8080,4000,3000,8090 from Brandon's IP only. Remove once Caddy/TLS is in front |
| Bucket | warmbly-blobs-data-286013 | us-central1, uniform access |
| Service account | warmbly-blobs@ | HMAC key for S3 API, objectAdmin on bucket |
| Service account | warmbly-worker@ | runtime identity of Cloud Run jobs; secretAccessor on the 5 secrets, artifactregistry.reader on ghcr-remote |
| Service account | warmbly-scheduler@ | Cloud Scheduler identity; needs run.invoker on each job |
| Secrets | warmbly-internal-api-token, warmbly-kms-master-key, warmbly-credentials-key, warmbly-blob-access-key, warmbly-blob-secret-key | Secret Manager |
| Artifact Registry | ghcr-remote (us) | remote repo -> ghcr.io |
| Cloud Run jobs | warmbly-worker in 4 regions | see "Worker jobs" |
| Cloud Scheduler | warmbly-worker-<region> | */15 cron, staggered |

## Control plane VM

Install dir `/opt/warmbly`, installed with Warmbly's `install.sh` v0.4.1:

```
sudo env WARMBLY_S3_BUCKET=warmbly-blobs-data-286013 \
  WARMBLY_S3_ENDPOINT=https://storage.googleapis.com \
  WARMBLY_S3_REGION=us-central1 \
  WARMBLY_S3_ACCESS_KEY_ID=<hmac id> WARMBLY_S3_SECRET_ACCESS_KEY=<hmac secret> \
  sh install.sh --host 34.45.45.44 --tls none --blobs s3 --dir /opt/warmbly
```

Two fixes the v0.4.1 installer misses, appended to `.env` by hand:

```
GEODB_PATH=/app/data/GeoLite2-City.mmdb   # file may be absent; var must exist
EMAIL_NAME=Warmbly
```

`docker-compose.override.yml` publishes NATS and Redis on the internal IP only:

```yaml
services:
  nats:
    ports: ["10.128.0.2:4222:4222"]
  redis:
    ports: ["10.128.0.2:6379:6379"]
```

Files on the VM to back up: `/opt/warmbly/.env`, `/opt/warmbly/keys-backup.txt`
(both 0600 root), and Postgres. Losing KMS_LOCAL_MASTER_KEY or
CREDENTIALS_ENCRYPTION_KEY makes every stored mailbox credential unrecoverable.

Useful commands (via `gcloud compute ssh warmbly-cp --zone us-central1-a --tunnel-through-iap`):

```
cd /opt/warmbly
sudo docker compose -p warmbly ps
sudo docker compose -p warmbly logs backend --tail 100
sudo docker compose -p warmbly up -d            # apply .env changes
sudo docker compose -p warmbly exec backend warmblyctl --help        # v0.4.1 has no `fleet` subcommand yet; workers show under Instance in the admin panel
sudo docker compose -p warmbly exec backend warmblyctl setup-link   # new claim link if no accounts yet
sudo docker compose -p warmbly exec backend warmblyctl backup --out /data/blobs/warmbly.tar.gz
```

## Worker jobs (Cloud Run)

One job per region, identical except `WORKER_REGION` and the scheduler offset.

**How they were actually created:** `infra/create-worker-jobs.py`, which calls
the Cloud Run Admin API v2 directly, because the Homebrew gcloud on this Mac
(481.0.0, mid-2024) predates the `--network/--subnet/--vpc-egress` flags for
jobs. Re-run it to update all four jobs after editing `worker-env-<region>.yaml`
(`python3 infra/create-worker-jobs.py`, or pass a region). It also accepts
`WORKER_IMAGE=...` to point at a Fullpilot-built image. The gcloud equivalent
below is for reference and works on a current SDK. Note: `brew upgrade --cask
google-cloud-sdk` installed a newer SDK under
`/opt/homebrew/Caskroom/google-cloud-sdk/latest/` but the `gcloud` symlink still
points at 481.0.0; fix with `brew reinstall --cask google-cloud-sdk` when
convenient.

Verified 2026-09-09: a test execution started 2 containers, both joined NATS
and heartbeated to the backend from 10.128.0.16 / .17 (Direct VPC egress).

```
gcloud run jobs create warmbly-worker --region <REGION> \
  --image us-docker.pkg.dev/data-286013/ghcr-remote/warmbly/warmbly/worker:v0.4.1 \
  --service-account warmbly-worker@data-286013.iam.gserviceaccount.com \
  --command /bin/sh \
  --args='^|^-c|timeout -s TERM 290 /app/worker; rc=$?; [ $rc -eq 124 ] && exit 0; exit $rc' \
  --tasks 2 --parallelism 2 --task-timeout 330s --max-retries 0 \
  --cpu 1 --memory 512Mi \
  --env-vars-file infra/worker-env.yaml \
  --set-env-vars WORKER_REGION=<us-central|us-east|us-west|eu-west> \
  --set-secrets INTERNAL_API_TOKEN=warmbly-internal-api-token:latest,ENCRYPTED_KEYS_WORKER_TOKEN=warmbly-internal-api-token:latest,KMS_LOCAL_MASTER_KEY=warmbly-kms-master-key:latest,CREDENTIALS_ENCRYPTION_KEY=warmbly-credentials-key:latest,AWS_ACCESS_KEY_ID=warmbly-blob-access-key:latest,AWS_SECRET_ACCESS_KEY=warmbly-blob-secret-key:latest \
  --network default --subnet default --vpc-egress private-ranges-only
```

The `timeout` wrapper ends the worker cleanly at 290 s so the task reports
success; a real crash still reports failure. Each task gets a fresh Google
egress IP. `private-ranges-only` sends only 10.x traffic (NATS, Redis, backend)
through the VPC; mailbox traffic goes straight out.

Scheduler, staggered 4 min apart per region:

```
gcloud scheduler jobs create http warmbly-worker-<REGION> --location <REGION> \
  --schedule "<OFFSET>/15 * * * *" \
  --uri https://run.googleapis.com/v2/projects/data-286013/locations/<REGION>/jobs/warmbly-worker:run \
  --http-method POST \
  --oauth-service-account-email warmbly-scheduler@data-286013.iam.gserviceaccount.com
```

Offsets: us-central1 `0`, us-east1 `4`, us-west1 `8`, europe-west1 `12`.
The scheduler SA needs `roles/run.invoker` on each job:
`gcloud run jobs add-iam-policy-binding warmbly-worker --region <REGION> --member serviceAccount:warmbly-scheduler@data-286013.iam.gserviceaccount.com --role roles/run.invoker`

Scaling knobs: more IP spread = more regions or more `--tasks`; faster pickup =
tighter cron (`*/10`). Cost is about $60/month per always-on vCPU equivalent.

## Why rotation, and what it does and does not do

Worker IPs are only seen by the mailbox provider at login. Recipients see the
provider's IP. Warmbly's own scheduler prefers a stable worker per mailbox
(their PR #395: "IP stability per mailbox beats IP diversity"). This deploy
overrides that on purpose per Brandon's decision. Residential/mobile proxy
providers (Decodo, IPRoyal, Rayobyte, Webshare, Bright Data) all refuse SMTP
ports, so Cloud Run's per-run IP is the rotation source.

## Domain and TLS (live: portal.fullpilot.com)

Done 2026-09-09. fullpilot.com DNS is on Cloudflare under the
brandonhays123@gmail.com Cloudflare account (NOT the brandon.hays@fullpilot.com
one); apex and www are on Vercel. Six A records point at 34.45.45.44 with the
Cloudflare proxy OFF (grey cloud) so Caddy on the VM can do ACME itself.
`infra/switch-to-domain.sh` (run as root on the VM, idempotent) rewrote `.env`,
wrote `/opt/warmbly/Caddyfile`, and added a `caddy` service to
`docker-compose.override.yml`. Certificates come from Let's Encrypt and renew
automatically. The temporary direct-port firewall rule was deleted; only 80/443
are open.

Dashboard: https://portal.fullpilot.com  Admin: https://admin.portal.fullpilot.com

| Host | Backs |
|---|---|
| portal.fullpilot.com | web :5173 |
| admin.portal.fullpilot.com | admin :5174 |
| api.portal.fullpilot.com | backend :8080 |
| ws.portal.fullpilot.com | realtime :4000 (HTTP/1.1 only) |
| track.portal.fullpilot.com | tracking :3000 |
| forms.portal.fullpilot.com | forms :8090 |

Worker env files (`infra/worker-env-*.yaml`) carry the same public URLs and
were re-applied with `create-worker-jobs.py`.

Deliverability note: `track.portal.fullpilot.com` is under the company domain.
Cold-email tracking links are usually put on a separate throwaway domain so a
spam complaint never touches fullpilot.com's reputation. To move it: add an A
record on the other domain, add a Caddy block for it, set `TRACKING_DOMAIN` in
`.env`, `docker compose up -d`.

## Mailbox OAuth: Thunderbird's client for Gmail, Fullpilot's Entra app for Outlook

Gmail path (decided 2026-09-09) = Thunderbird's public desktop-type client,
configured as `BOX_GOOGLE_DESKTOP_*` with NO `BOX_GOOGLE_CLIENT_*` set. With
only the desktop client present it IS the Gmail button (no alt link), and every
connect uses the paste-the-address flow. Its ID and secret are public in
Thunderbird's source (`mailnews/base/src/OAuth2Providers.sys.mjs`). Caveats:
Google only lets a desktop client redirect to localhost, and using another
product's client is against Google's API terms; Google could revoke it.

Why not Fullpilot's own "Fullpilot Sequencer" Google client (397999256581-…):
probing Google's authorize endpoint showed its only registered redirect URIs
are `http://localhost:300{0..5}/oauth/callback` (dev URIs); nothing on a
production host. Using it would still mean the paste flow, so Brandon chose
Thunderbird. To switch to it later with a clean redirect: register
`https://api.portal.fullpilot.com/addresses/google/callback` on that client in
the Google Cloud Console, set `BOX_GOOGLE_CLIENT_ID/SECRET` (secret is in
Secret Manager `warmbly-box-google-client-secret` v2), restart. The desktop
client then becomes the "Connect through Thunderbird's client instead" link.

Outlook (decided 2026-09-09, same reasoning) = Thunderbird's public Microsoft
client `9e5f94bc-e8a4-4e73-b8be-63364c29d753`, configured as
`BOX_OUTLOOK_DESKTOP_CLIENT_ID` with no secret and NO `BOX_OUTLOOK_CLIENT_*`.
Redirect `http://127.0.0.1` (Entra ignores the port on loopback), paste flow.
Warmbly requests Graph scopes (Mail.Send, Mail.ReadWrite, User.Read) on that
client via dynamic consent. The Fullpilot "Sequencer Outlook" Entra app
(2db46e08-…) is unused; its secret stays in `warmbly-box-outlook-client-secret`
in case its redirect URIs are ever registered.

Dead ends kept for the record: `app-engine.fullpilot.com` (old EmailEngine on
AWS App Runner, service gone) now has an A record to 34.45.45.44 and a Caddy
block forwarding `/oauth` to Warmbly's callbacks; harmless, unused, can be
removed. `BOX_GOOGLE_REDIRECT_URL` / `BOX_OUTLOOK_REDIRECT_URL` overrides exist
in the fork for the case where a client's registered redirect is on a host we
control.

Fork changes (branch `fullpilot`, migration 000135):
- `BOX_GOOGLE_DESKTOP_CLIENT_ID/SECRET` configure the second client:
  redirect `http://localhost:17777/warmbly/oauth`, scope `https://mail.google.com/`.
  `GET /auth/config` reports `gmail_desktop_client: true` when set.
- `POST /emails/onboarding/oauth/start` takes `client: "google_desktop"`; the
  response carries `manual_redirect: true` and the modal shows a paste field.
  The customer approves in the Google popup, lands on "localhost refused to
  connect", copies that address bar into the modal, and Warmbly finishes from
  the `code` + `state` in it.
- `email_accounts_oauth.oauth_client` ('default' | 'google_desktop') records
  which client issued a mailbox's tokens; the worker payload carries it and the
  worker refreshes with the matching client. Reauth reuses the stored client.
- Reconnect-from-drawer flow (`web/src/lib/emails/emailOAuthPopup.ts`) is NOT
  adapted for the paste step; a desktop-client mailbox that needs reconsent
  should be removed and re-added for now.

Env, in `/opt/warmbly/.env` and every `infra/worker-env-*.yaml` (secrets via
Secret Manager, see `infra/create-worker-jobs.py` SECRETS):
```
BOX_GOOGLE_CLIENT_ID / BOX_GOOGLE_CLIENT_SECRET            Fullpilot Sequencer (default)
BOX_GOOGLE_DESKTOP_CLIENT_ID / BOX_GOOGLE_DESKTOP_CLIENT_SECRET   Thunderbird's client
BOX_OUTLOOK_CLIENT_ID / BOX_OUTLOOK_CLIENT_SECRET          Fullpilot Sequencer Outlook
```

## Current production images

Since 2026-09-09 17:36 UTC every service runs Fullpilot-built images
(`us-docker.pkg.dev/data-286013/fullpilot/<svc>:fp-153d37bc`), not upstream's.
Per-service images are recorded in `/opt/warmbly/images.env` and rendered to
`docker-compose.images.yml` (loaded via `COMPOSE_FILE` in `.env`). Rollback to
upstream for one service: delete its line from `images.env`, regenerate with
`npm run deploy` or by hand, `docker compose up -d`.

Local dev on Brandon's Mac: Docker Desktop 24 ships Compose 2.23 and buildx
0.12, both too old for Warmbly's compose file; Homebrew `docker-compose` (5.x)
and `docker-buildx` (0.37) are installed and registered via
`cliPluginsExtraDirs` in `~/.docker/config.json`.

## Branding

Dashboard and admin are rebranded to Fullpilot (see `warmbly/FULLPILOT.md`,
"Rebrand to Fullpilot"). Brand source of truth: `~/Documents/fullpilotv2`
(`apps/dashboard/src/app/globals.css` tokens, `apps/dashboard/public/logo-blue.svg`).
Primary #0c58c6, text #02101e, Rethink Sans.

## AI, Instantly, and other keys (added 2026-09-09 evening)

`/opt/warmbly/.env` on the VM (backend + consumer): `AI_PROVIDER=openrouter`,
`AI_API_KEY` (Secret Manager `warmbly-openrouter-api-key`),
`AI_MODEL=anthropic/claude-sonnet-5`, `AI_MODEL_CLASSIFY=openai/gpt-5.6-luna`
(cheap reply classifier, fork feature), `INSTANTLY_API_KEY` (Secret Manager
`warmbly-instantly-api-key`, used by the fork's Instantly warmup integration).
Also `EMAIL_BRAND_NAME=Fullpilot`, `AWS_REQUEST_CHECKSUM_CALCULATION` /
`AWS_RESPONSE_CHECKSUM_VALIDATION=when_required` (Cloud Storage S3 compat; the
real fix is in the fork's storage client, which strips Accept-Encoding before
signing).

`warmbly-google-service-account` in Secret Manager holds the
cloud-768@data-286013 service account JSON from fullpilot_sequencer. Warmbly
only uses `GOOGLE_APPLICATION_CREDENTIALS_JSON` for Cloud Tasks webhook auth
(`TASKS_PROVIDER=gcloud`), which this deployment does not use, so it is NOT
wired into the env.

`warmblyctl mailbox ...` needs `WARMBLY_API_KEY` (create one in Settings > API
keys) to work; without it, mailbox removal was done with
`delete from email_accounts where id=...` (FKs cascade).

## Custom code and deploys

Fork: https://github.com/brandonhays3/fullpilot-warmbly
- `main` tracks upstream `warmbly/warmbly` main (sync: `git fetch upstream && git merge --ff-only upstream/main`)
- `fullpilot` is the branch for Fullpilot changes, started from tag `v0.4.1` to match production

See `warmbly/FULLPILOT.md` for the npm scripts that run dev locally, build
images, push them to Artifact Registry, and roll them out to the VM and the
Cloud Run jobs.
