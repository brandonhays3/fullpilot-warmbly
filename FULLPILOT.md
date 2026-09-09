# Fullpilot fork of Warmbly

This is Fullpilot's fork of [warmbly/warmbly](https://github.com/warmbly/warmbly).
Production runs on Google Cloud project `data-286013`; the full runbook is in
`~/Documents/Fullpilot-Warmbly/README.md` (copy in `deploy/fullpilot/gcp/RUNBOOK.md`).

## Branches

| Branch | Purpose |
|---|---|
| `main` | Mirror of upstream main. Never commit here. `npm run upstream:sync` fast-forwards it. |
| `fullpilot` | Our changes (theme, features, deploy scripts). Started from upstream tag `v0.4.1`, the version production launched on. |

To pick up a new upstream release: `npm run upstream:sync`, then on `fullpilot`
run `git merge v0.4.2` (or whatever tag), resolve conflicts, test locally, release.

## The loop: test locally, release in batches

Building all nine images on Cloud Build takes 10 to 20 minutes, so nothing
gets built to be tested. Everything is tried locally first, and a build only
happens when a batch of changes is ready.

### Local development

Prereqs, all already on this Mac: Docker Desktop (must be running), Go 1.25
(auto-downloaded by the toolchain), pnpm, Node 22. Elixir and Rust are only
needed if you touch `realtime/` or `tracking/`.

Three terminals:

```
npm run dev        # once per session: postgres, redis, nats, mailpit in Docker + migrations
npm run dev:run    # backend + consumer + worker natively, hot reload on Go changes
npm run dev:web    # dashboard on http://localhost:5173 (Vite picks 5174 if busy) with hot reload
```

`npm run dev:admin` adds the admin panel on :5174. `npm run dev:stop` stops the
Docker infra. Outgoing platform mail lands in Mailpit at http://localhost:8025.

Fullpilot-specific env (Gmail client, redirect mode) lives in
`.env.fullpilot.local` at the repo root, gitignored, loaded by `dev:run`.
Template: `deploy/fullpilot/dev.env.example`. The local stack uses the same
Thunderbird client and paste flow as production, so Gmail connections can be
tested end to end on localhost.

First run: the dashboard prints a claim link in the backend log, or run
`make claim`. Dev data is separate from production (database `warmbly_dev`).

Where things live: dashboard UI and theme in `web/src/` (React, Vite,
Tailwind); admin panel in `admin/`; API and services in Go under `cmd/` and
`internal/`; mail sending and sync in `cmd/worker` + `internal/`.

Before committing: `cd web && pnpm typecheck && pnpm lint`, and `go build ./...`
for Go changes.

### Shipping a batch

```
npm run images                   # all 9 images, tag fp-<short sha>
npm run images -- web admin      # only the services you changed (much faster)
TAG=fp-abc1234 npm run deploy    # roll that tag out
npm run release                  # images + deploy, all services
```

- Images go to `us-docker.pkg.dev/data-286013/fullpilot/<service>:<tag>` and
  also `:latest`. Builds use `--cache-from :latest` with BuildKit inline cache,
  so an unchanged service is mostly cache hits.
- `deploy` only switches services that have an image at that tag. Per-service
  images are recorded in `/opt/warmbly/images.env` on the VM and rendered into
  `docker-compose.images.yml`, which compose loads last via `COMPOSE_FILE` in
  `.env`. Services you didn't build stay on whatever they run now.
- The Cloud Run worker jobs are updated only when `worker` was in the build.
- Rollback: `TAG=<previous tag> npm run deploy`. To go back to upstream's image
  for one service, delete its line from `images.env` on the VM and re-run
  `deploy` (or edit `docker-compose.images.yml` by hand and `docker compose up -d`).

## Operating production

```
npm run prod:ps         # container status on the VM
npm run prod:logs       # last 200 log lines
npm run prod:ssh        # shell on the VM
npm run workers:status  # last Cloud Run executions per region
npm run workers:run     # fire a worker run in every region now
```

All scripts read `deploy/fullpilot/config.sh` for project, zone, regions and
registry. None of them contain secrets.

## Fullpilot changes so far

- **Gmail OAuth via a desktop-type client** (`BOX_GOOGLE_DESKTOP_CLIENT_ID/SECRET`,
  Thunderbird's public client in production): Google only lets a desktop client
  redirect to localhost, so the connect modal asks the user to paste the address
  the consent window landed on and finishes from the code in it. With only the
  desktop client configured it is the Gmail button; with a web client
  (`BOX_GOOGLE_CLIENT_*`) too, it becomes the "Connect through Thunderbird's
  client instead" link. `email_accounts_oauth.oauth_client` (migration 000135)
  records which client issued a mailbox's tokens so workers refresh with the
  same one. `BOX_GOOGLE_REDIRECT_URL` / `BOX_OUTLOOK_REDIRECT_URL` can force a
  registered redirect_uri; a loopback one triggers the paste flow too. Files:
  `internal/config/inbox.go`, `internal/app/email/{onboarding,reauth}.go`,
  `internal/models/{email,worker}.go`, `internal/repository/pg_email.go`,
  `internal/app/email/loader.go`, `internal/app/worker/mailmanager/`,
  `internal/api/handler/{email_onboarding,auth_config}.go`,
  `web/src/components/app/modals/AddEmailModal.tsx`,
  `web/src/lib/api/client/app/emails/onboardOAuthStart.ts`. The reconnect flow
  in the mailbox drawer (`web/src/lib/emails/emailOAuthPopup.ts`) is not adapted
  for the paste step yet.
- **Rebrand to Fullpilot** (commit "rebrand dashboard and admin to Fullpilot"):
  Fullpilot mark in `web/src/components/svg.tsx` and `admin/src/components/Logo.tsx`
  (source: `~/Documents/fullpilotv2/apps/dashboard/public/logo-blue.svg`);
  favicons/app icons regenerated from it with ImageMagick; brand blue applied by
  redefining the Tailwind `sky` scale around #0c58c6 in `web/src/global.css`
  (so `bg-sky-600` etc. everywhere are Fullpilot blue); font Rethink Sans;
  titles via `BRAND` in `web/src/hooks/useDocumentTitle.ts`; product name
  replaced everywhere it was capitalized (technical `warmbly_*`, `warmbly-*`,
  `__WARMBLY_ENV__`, URLs untouched). Self-hosted pill removed
  (`PlanPill.tsx` returns null without billing). Every Warmbly Cloud surface
  deleted: settings page + nav entry, mailbox page banners/panels, onboarding
  step, connect-modal info box, drawer warmup card, `/connect` route.
  `useCloudPool` and the cloud-link API client remain (harmless, always "not linked").
- **Deploy tooling** under `deploy/fullpilot/`.
