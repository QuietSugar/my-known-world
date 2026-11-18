# v0.0.6 release checklist

Do not publish until the candidate is merged to `master`, the merge commit's required CI passes, and the release artifact is verified.

## Before tagging

- [ ] Review the release candidate; confirm no private graph data, credentials, source configuration, internal addresses, generated batches, or Markdown bodies are included.
- [ ] Confirm `VERSION`, `.env.example`, Compose defaults, Dockerfile, MCP implementation, and Web package metadata use `0.0.6`/`v0.0.6` consistently.
- [ ] Confirm the release notes describe the authority boundary accurately: PostgreSQL is authoritative; Markdown import is explicit only; no watcher or continuous replacement.
- [ ] Merge the approved candidate to `master` and verify the exact merge commit.
- [ ] Confirm CI passes on that merge commit: format, `go vet`, race-enabled tests, PostgreSQL integration tests, Web production build, hardened container build, and API/UI smoke test.
- [ ] Review the CI image artifact and checksum. Run the container against a clean disposable PostgreSQL database and verify health, API, UI, and read-only MCP startup.
- [ ] Treat private-corpus and production cutover testing as deployment-owner responsibilities; do not claim these were performed by CI.

## Publish

After review, merge, and green checks, push annotated tag `v0.0.6` at the approved merge commit. The tag workflow rebuilds/tests the image, publishes version and commit-SHA images to GHCR, exports the image tar and SHA-256 checksum, and creates the GitHub Release using `docs/release-v0.0.6.md`.

## After publish

- [ ] Confirm `ghcr.io/quietsugar/my-known-world:v0.0.6` exists and the GitHub Release contains the image tar and checksum.
- [ ] Download the tar, verify its checksum, and run the documented smoke test.
- [ ] Before retiring hall-mcp in any real environment, complete the private-corpus comparison and end-to-end human/Agent cutover and rollback rehearsal described by `docs/database-authoritative-cutover.md`.
