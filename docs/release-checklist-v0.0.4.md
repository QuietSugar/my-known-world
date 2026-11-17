# v0.0.4 release checklist

This checklist prepares the release; it does not create or push a tag.

## Before tagging

- [ ] Review the active release candidate; confirm no private graph data, credentials, source configuration, internal addresses, or document bodies are included.
- [ ] Confirm every UI and HTTP write has an authenticated apply step and a matching server-side preview/revision check, or a documented domain-specific conditional guard.
- [ ] Verify that stale/tampered manual, batch, field-resolution, source-policy, provisional-review, and undo plans are rejected without partial writes.
- [ ] Confirm previews and errors do not return assertion values or value hashes.
- [ ] Merge the approved candidate to `master`; confirm `VERSION` is `0.0.4`, Compose defaults use `v0.0.4`, and the release notes match the merged product.
- [ ] Confirm CI passes on the merge commit: formatting, `go vet`, race tests, PostgreSQL integration tests, web production build, hardened container build, and smoke tests.
- [ ] Review the CI image artifact/checksum and verify deployment backup/restore and write-token procedures.

## Publish

After review, merge, and green checks, push tag `v0.0.4` at the approved merge commit. The tag workflow builds and tests the container, publishes version and commit-SHA images to GHCR, exports the image tar and SHA-256 checksum, and creates the GitHub release using `docs/release-v0.0.4.md`.

## After publish

- [ ] Confirm `ghcr.io/quietsugar/my-known-world:v0.0.4` exists; download the image tar and verify its checksum.
- [ ] Run the documented smoke test against the release image.
- [ ] Announce as early access; do not imply business-validity intervals or full point-in-time graph queries are available.
