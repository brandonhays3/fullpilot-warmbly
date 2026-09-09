# Fullpilot fork of Warmbly

This is Fullpilot's fork of [warmbly/warmbly](https://github.com/warmbly/warmbly).
Production runs on Google Cloud project `data-286013`; the full runbook is in
`~/Documents/Fullpilot-Warmbly/README.md`.

## Branches

| Branch | Purpose |
|---|---|
| `main` | Mirror of upstream main. Never commit here. `npm run upstream:sync` fast-forwards it. |
| `fullpilot` | Our changes (theme, features, deploy scripts). Started from upstream tag `v0.4.1`, the version running in production. |

To pick up a new upstream release: `npm run upstream:sync`, then on `fullpilot`
run `git merge v0.4.2` (or whatever tag), resolve conflicts, test, release.

## Local development

Prereqs: Docker Desktop, Go 1.25+, pnpm, Node 20+. Elixir and Rust are only
needed if you touch `realtime/` or `tracking/`; the Makefile runs those in
containers otherwise.

```
npm run dev        # postgres, redis, nats, mailpit in Docker + migrations
npm run dev:run    # backend + consumer + worker natively with hot reload
npm run dev:web    # dashboard on http://localhost:5173 (Vite, hot reload)
npm run dev:admin  # admin panel on http://localhost:5174
npm run dev:stop
```

Theme and UI live in `web/` (React + Vite). Start with `web/src/` and the
Tailwind config there. Admin panel is `admin/`. Backend is Go under `cmd/`
and `internal/`. Mail sending and syncing is `cmd/worker` + `internal/`.

## Shipping to production

```
npm run images                 # build all 9 images on Cloud Build, push to Artifact Registry
TAG=fp-abc1234 npm run deploy  # roll that tag out to the VM and the Cloud Run worker jobs
npm run release                # both, using tag fp-<current sha>
```

- Images go to `us-docker.pkg.dev/data-286013/fullpilot/<service>:<tag>`.
- `deploy` rewrites `WARMBLY_IMAGE_PREFIX` and `WARMBLY_TAG` in `/opt/warmbly/.env`
  on the VM, pulls, recreates containers (migrations run on backend boot),
  then points every regional `warmbly-worker` Cloud Run job at the new worker
  image. Each `.env` change is backed up as `.env.bak.<timestamp>` first.
- Rollback: `TAG=<previous tag> npm run deploy`. To go back to upstream's
  images set `WARMBLY_IMAGE_PREFIX=ghcr.io/warmbly/warmbly` and
  `WARMBLY_TAG=v0.4.1` in `.env` by hand and `docker compose up -d`.

Builds take a while the first time (Rust tracking and Elixir realtime compile
from scratch). If you only changed the dashboard, you can still run the full
build; Docker layer caching on Cloud Build is not enabled, so expect 10 to 20
minutes.

## Operating production

```
npm run prod:ps         # container status on the VM
npm run prod:logs       # last 200 log lines
npm run prod:ssh        # shell on the VM
npm run fleet           # warmblyctl fleet list: which workers are live
npm run workers:status  # last Cloud Run executions per region
npm run workers:run     # fire a worker run in every region now
```

All scripts read `deploy/fullpilot/config.sh` for project, zone, regions and
registry. None of them contain secrets.
