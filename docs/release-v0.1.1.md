# MyKnownWorld v0.1.1 — patch release

A small patch on top of [v0.1.0](release-v0.1.0.md). No schema change, no new
endpoint, no change to the write contract: this release fixes wrong version
metadata, carries one correctness fix that landed after v0.1.0, and documents
things operators were getting wrong.

Upgrade with the usual two steps — take a `pg_dump -Fc` backup first, then:

```sh
docker load -i my-known-world-image.tar   # or: docker pull ghcr.io/quietsugar/my-known-world:v0.1.1
docker compose up -d --no-build           # with IMAGE_TAG=v0.1.1 in .env
```

There is no manual migration step; the app applies schema migrations on startup.
Existing routes, tokens, vault keys, and Compose settings keep working.

## What changed

- **The MCP handshake reports the real version** (issue #40). The read-only MCP
  server used to answer `initialize` with a hardcoded
  `serverInfo.version: "0.0.7"` no matter which build was running. Binaries are now
  stamped at build time, so this image reports `0.1.1`, and an operator can override
  it with `MYKNOWNWORLD_VERSION` (used by Compose deployments) without rebuilding.
  `docker inspect` on the image shows the same version in the
  `org.opencontainers.image.version` label. This matters for agents that record
  which server they talked to, and for triage: previously every release looked
  identical to a client.
- **Sync preview no longer rewrites the identity mappings you submitted**
  (merged after v0.1.0). The preview endpoint used to merge the stored mappings into
  the request payload in place. A preview call must never mutate what the caller
  sent, and the confirmation step now compares exactly what the user reviewed.
- **Documentation for running the bundled CLI tools in the container** (issue #41).
  `docs/release-v0.1.0.md` documents the working `docker run` form: override the
  entrypoint with `--entrypoint /usr/local/bin/myknownworld-<tool>`, and use an
  output directory the container's UID can write (`chmod 777`, `chown 10001:10001`,
  or `--user $(id -u):$(id -g)`). The image entrypoint stays the web server, so
  appending a binary path to `docker run` does not run that binary; the section also
  notes that `-api http://127.0.0.1:8080` inside a container points at the container
  itself.
- **`docs/handover.md`** records the current state, the decisions that are settled
  (including why issues #32 and #33 are closed as not planned and what minimal slice
  would make each reviewable), the open work in priority order, and the release
  procedure. Trackers are at zero open issues and zero open pull requests.

## Verified before publishing

- `gofmt`, `go vet ./...`, `go test -race ./...` including PostgreSQL integration
  tests.
- Web build, hardened Compose stack (non-root, read-only root, dropped
  capabilities), API contract checks, CMDB synthetic secret lifecycle, create-API
  checks, and all five bundled commands.
- Release archive carries the `v0.1.1` tag; the published GHCR tag answers an
  anonymous pull request.

## Known limitations

Unchanged from v0.1.0. This is still a `0.x` single-operator deployment: keep the
listener private, keep the write token secret, and test restores before trusting it
with data you cannot recreate. There is still no write `PATCH`/`DELETE`, no
pagination on `/api/nodes`, and no automated backup/restore drill — see
`docs/handover.md` for the ordered list of remaining gaps. Issues #32 and #33
(CMDB-to-graph linking, layered materialization adapters) remain closed as not
planned for this line.
