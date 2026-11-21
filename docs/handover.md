# Handover: current state, decisions, and open work

Short version: `master` is green, there are no open pull requests, the only branch is
`master`, and the tracker holds three actionable backlog issues (#46 backup/restore
drill, #47 write `PATCH`/`DELETE`, #48 `/api/nodes` pagination). The last published
artifact is the official `v0.1.1` patch release on top of `v0.1.0`. Anything a new
maintainer needs to decide is listed under "Open work" and "Owner-only actions".

## Where things stand

- `master`: all gates green in GitHub Actions (gofmt, `go vet`, race tests with
  PostgreSQL integration tests, web build, hardened Compose stack, API/MCP/create
  API contract checks, image build).
- Tracker: 0 open PRs. The three remaining gaps below are filed as #46 (backup and
  restore drill), #47 (write `PATCH`/`DELETE`), and #48 (`/api/nodes` pagination and
  summary projection) so they are actionable rather than prose. Every closed issue or
  PR has a written reason (`not planned` items explain what would make them
  reviewable).
- Branches: only `master`. Old working branches were merged or superseded first,
  then deleted.
- Release: `v0.1.1`, published as a normal (non-pre-release) release with
  `my-known-world-image.tar` plus `.sha256`, pullable from GHCR. It supersedes
  `v0.1.0`, which reported a hardcoded `serverInfo.version` in the MCP handshake.

## What is delivered and working

- **Write path, single and batch**: `POST /api/write/nodes`, `/api/write/relations`,
  `/api/write/batch` (transactional, at most 500 nodes and 500 relations, 1 MiB
  body). Contract in `docs/write-api.md`, end-to-end script
  `scripts/write_api_smoke_test.py`.
- **Read path**: public read API plus read-only MCP over `/mcp` (streamable HTTP)
  and stdio (`cmd/myknownworld-mcp`). MCP registers query tools only and never
  receives a write token.
- **Web UI**: browse from Self, impact paths, typed details, history, sync history,
  and preview-then-confirm editing for every non-create change (with undo).
- **Secrets boundary**: application-layer CMDB catalog with the built-in encrypted
  file provider or an operator-supplied HTTP provider
  (`docs/secret-providers.md`). Secret values never enter the graph, history,
  assertions, MCP responses, logs, or Markdown exports.
- **Explicit export**: `myknownworld-materialize` writes deterministic Markdown with
  `manifest.yaml`, an unsupported-data report, and an explicit overwrite flag; no
  watcher and no Git automation.
- **Packaging**: multi-arch image, non-root UID 10001, read-only root filesystem,
  dropped capabilities, `no-new-privileges`, tmpfs `/tmp`, healthcheck; five
  bundled binaries (`myknownworld`, `-mcp`, `-markdown-import`, `-cmdb`,
  `-materialize`); container CLI usage documented in `docs/release-v0.1.0.md`.
- **Version reporting**: binaries report their build version, overridable with
  `MYKNOWNWORLD_VERSION` (`internal/version`).

## Decisions that are settled (do not re-litigate casually)

- **Scope of the 0.x line**: finish and stabilise the create / batch-create write
  API and the existing read/UI/export paths. No horizontal capability expansion;
  work should raise day-to-day usability of what already exists.
- **Issues #32 and #33 are closed as `not planned`** for this line, with reasons
  recorded in the issue threads. Both name a minimal slice that would be
  reviewable if someone wants to reopen them:
  - #32: a one-way `cmdbServiceId` reference on `SoftwareInstance` (no secret path,
    no multi-user identity work).
  - #33: come back with a concrete second materialization adapter; the current
    `myknownworld-materialize` behaviour is the supported path.
- **Vault providers are pluggable by rule, not by product**: the shipped encrypted
  file provider is the default, and a production vault is an operator choice behind
  the documented HTTP contract. A production vault is not an acceptance blocker.
- **Markdown is never a watcher.** Import and export are explicit, reviewed
  operations.
- **Closing something requires a written reason.** Every closed PR/issue in this
  repository carries a comment explaining why; keep doing that.

## Open work, in priority order

1. **[DELIVERED] Backup and restore are documented and engineered** (issue #46, closed).
   Delivered verified `scripts/backup.sh`, `scripts/restore.sh`, and `scripts/verify_restore.py`,
   with a fully automated restore drill step in CI and operator runbook `docs/backup-and-restore.md`.
2. **[DELIVERED] The write API supports update and delete** (issue #47, closed).
   Direct `PATCH /api/write/nodes/{id}`, `DELETE /api/write/nodes/{id}`, `PATCH /api/write/relations/{id}`,
   and `DELETE /api/write/relations/{id}` with optimistic revision locking, referential integrity
   protection, and undoable audit operation history.
3. **[DELIVERED] Paged read API and summary projection** (issue #48, closed).
   Added `limit` (max 500), stable ID-based `cursor`, and `offset` to `/api/nodes` and `/api/relations`,
   along with `view=summary` projection aligning with MCP `graph_query`, and internal paged streaming
   in `myknownworld-materialize`. Unified consistent response structures across all queries without historical compatibility branches.
4. **Single write token, no identity, no metrics.** No per-user authorization and
   no Prometheus/structured operational metrics. Both are architecture-level.
5. **CMDB and graph entities are still unlinked** (deferred with #32).

## How to work on the repository

```sh
# build, vet, race tests (PostgreSQL integration tests need a DSN)
export POSTGRES_TEST_DSN='postgres://postgres:postgres@localhost:5432/myknownworld_test?sslmode=disable'
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go vet ./... && go test -race ./...

# run it locally
go run ./cmd/myknownworld            # ADDR defaults to 127.0.0.1:8080
cd web && npm ci && npm run dev      # UI on 5173, proxies to 127.0.0.1:8081

# container stack
cp .env.example .env                 # replace both tokens and the DB password
docker compose up --build -d
python3 scripts/smoke_test.py --base-url http://127.0.0.1:8080
```

Release procedure (see `.github/workflows/ci.yml`):

1. Bump `VERSION`, `IMAGE_TAG`/`IMAGE_VERSION` in `.env.example`, and the Compose
   defaults; add `docs/release-v<tag>.md` (the workflow uses it as the release
   body, so it must exist).
2. The image `VERSION` build arg also stamps the binaries, so the MCP handshake
   reports the release version.
3. Merge, then push an annotated tag `v<version>`. A version containing `-` (for
   example `0.2.0-rc.1`) is published as a pre-release, a plain version as an
   official release.

## Owner-only actions (an agent cannot do these)

- **GitHub settings.** Repository settings need admin rights; the automation token
  gets `403 Resource not accessible by integration`. To enable branch
  auto-deletion: *Settings → General → "Automatically delete head branches"*.
  Consider also requiring the `test` and `web` checks on `master`.
- **Real data and credentials.** Keep write tokens, CMDB tokens, vault keys, real
  graph snapshots, and internal addresses out of this repository. The demo graph
  that a fresh install seeds is synthetic on purpose.

## Where to read next

- `docs/release-v0.1.1.md` — the current patch release (version metadata, sync
  preview fix, container CLI docs).
- `docs/release-v0.1.0.md` — what shipped, install, upgrade, limitations.
- `docs/write-api.md` — the create/batch contract, errors, limits.
- `docs/read-only-mcp.md` — MCP tools and their guarantees.
- `docs/secret-providers.md` — provider contract and the credential boundary.
- `docs/layered-adapters.md` — the materialize command and manifest semantics.
- `docs/database-authoritative-cutover.md` — moving an existing graph into
  PostgreSQL.
- `docs/requirements.md`, `docs/technology.md` — the original design constraints.
