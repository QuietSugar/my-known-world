# MyKnownWorld v0.1.0 — first official release

This is the first **non-pre-release** build. Earlier tags (`v0.0.2`–`v0.0.11`)
were tester previews; start here if you are adopting MyKnownWorld for your own
graph.

It is still a `0.x` release: the graph formats are stable enough to run with real
data and backups, but the API may still change in a minor release. Issues #32 and
#33 were later closed as not planned for this line; the reasoning and the minimal
slice that would make each one reviewable are recorded on those issues, and
`docs/handover.md` lists what remains open. PostgreSQL is
authoritative; Markdown is an explicit import/export format, never a watcher.

## What you can do with it

- **Own your data path.** Create objects one at a time or in atomic batches over
  HTTP, with no import tooling in the loop:

  ```sh
  curl -sS -X POST http://127.0.0.1:8080/api/write/batch \
    -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"nodes":[{"node":{"id":"machine-nas","type":"Machine","name":"NAS","modelVersion":1}}]}'
  ```

  `POST /api/write/nodes`, `/api/write/relations`, and `/api/write/batch` are
  documented in `docs/write-api.md`; batches are transactional, revocable
  through the existing undo endpoint, and protected by the write token.
- **Browse and review the graph.** The bundled Web UI shows the graph from Self,
  impact paths, typed details, change history, sync history, and a
  preview-then-confirm editor for every non-create change.
- **Query it from agents.** The read-only MCP endpoint (`/mcp`) and the read API
  expose the same graph without any write credential.
- **Keep secrets out of the graph.** The CMDB layer stores environment/service/
  account catalogs and talks to a secret provider that is either the built-in
  encrypted file provider or your own service behind a small documented HTTP
  contract (`docs/secret-providers.md`). Secret values never enter the graph,
  history, MCP responses, logs, or Markdown exports.
- **Get a local copy when you want one.** `myknownworld-materialize` writes a
  deterministic Markdown copy with a manifest and an unsupported-data report; it
  never overwrites without `--overwrite` and never runs Git.

## Install

1. Copy `.env.example` to `.env`; set `POSTGRES_PASSWORD`, `CMDB_APP_TOKEN`, and
   a stable `MYKNOWNWORLD_WRITE_TOKEN` (each at least 32 random characters,
   independent from each other).
2. Optionally enable credentials: `CMDB_SECRET_PROVIDER=file` with
   `CMDB_VAULT_KEY_BASE64` (see `docs/secret-providers.md`), or
   `CMDB_SECRET_PROVIDER=http` with your own endpoint.
3. Load the release archive and start:

   ```sh
   docker load -i my-known-world-image.tar
   docker compose up -d --no-build
   ```

   or pull `ghcr.io/quietsugar/my-known-world:v0.1.0` and run
   `docker compose up -d`. The archive contains the `v0.1.0` tag, so the default
   `.env.example` works without retagging.

Open `http://127.0.0.1:8080` (or `http://localhost:8080`); the port binds to
loopback by default.

## Running the bundled CLI tools in the container

The runtime image ends with
`ENTRYPOINT ["/sbin/tini","--","/usr/local/bin/myknownworld"]`. Appending a
binary path to `docker run` therefore starts the **web server** with that path as
an extra argument — it does not run the tool. Override the entrypoint:

```sh
mkdir -p ./export
# The image runs as 10001:10001; a directory created by your own user is not
# writable for that UID, which surfaces as "open /export/...: permission denied".
chmod 777 ./export        # or: sudo chown 10001:10001 ./export
docker run --rm \
  --entrypoint /usr/local/bin/myknownworld-materialize \
  -v "$PWD/export:/export" \
  ghcr.io/quietsugar/my-known-world:v0.1.0 \
  -api http://127.0.0.1:8080 -out /export
```

`-api http://127.0.0.1:8080` inside a container points at the container itself.
Reach the host API with `--network host` (Linux) or
`-api http://host.docker.internal:8080` (Docker Desktop), or use the Compose
service name when the tool runs on the stack network.

Alternative when the stack is already running: `docker compose exec` starts the
tool as UID 10001 inside the existing container, so the output directory must be
writable by that UID (`/tmp` is a tmpfs there, hence the documented
`--out /tmp/materialized`, which disappears with the container).

The same rule applies to every bundled binary: `myknownworld-mcp`,
`myknownworld-markdown-import`, `myknownworld-cmdb`, and
`myknownworld-materialize` all need `--entrypoint` (or `docker compose exec`)
plus a writable output path or mount.

## Upgrade from v0.0.11

- No manual database migration step: the app applies schema migrations on
  startup. Take a `pg_dump -Fc` backup first anyway.
- New endpoints only; existing routes, tokens, and Compose settings keep working.
- The `v0.0.11` image remains available, but it was published as a pre-release.

## Verified before publishing

- `gofmt`, `go vet ./...`, `go test -race ./...` including PostgreSQL
  integration tests.
- Web build, hardened Compose stack (read-only root, dropped capabilities,
  non-root), API contract checks, CMDB synthetic secret lifecycle, create-API
  checks, and the five bundled commands.
- Release archive carries the release tag; the published GHCR tag answers an
  anonymous pull request.

## Known limitations

- **#32 (CMDB, closed as not planned):** services and accounts are not linked to
  core graph entities; the environment → service → account workflow, per-user
  authorization and provider audit remain open. The built-in file provider is
  single-host.
- **#33 (materialization, closed as not planned):** only hosting, product
  instances, and work-item references become Markdown relations; other relations, provenance and history
  are reported as unsupported, and original Markdown bodies were never stored.
- **MCP version string:** the published v0.1.0 image reports
  `serverInfo.version: "0.0.7"` in the MCP `initialize` handshake; the fix (issue
  #40) is on `master` and ships in the next build. Browsing, querying, and the
  write API are unaffected.
- No multi-tenant or internet-facing deployment is supported. Keep the listener
  private, keep the write token secret, and test restores before trusting it
  with data you cannot recreate.
