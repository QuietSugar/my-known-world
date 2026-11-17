# v0.0.5 release checklist

This checklist records release readiness. Do not tag until the candidate is merged to `master` and the merge commit's required CI passes.

## Before tagging

- [ ] Review the release candidate and confirm no private graph data, credentials, source configuration, internal addresses, or document bodies are included.
- [ ] Confirm `VERSION` is `0.0.5`, `.env.example` and Compose defaults use `v0.0.5`, package/MCP versions match, and notes describe the merged code.
- [ ] Confirm the fixes for issues #14–#19 are present and their GitHub issues are closed with resolution notes.
- [ ] Merge the approved candidate to `master` and verify the exact merge commit.
- [ ] Confirm CI passes on that merge commit: formatting, `go vet`, race-enabled tests, PostgreSQL integration tests, web production build, hardened container build, and API smoke test.
- [ ] Review the CI image artifact and checksum; verify the documented deployment works for the intended local/private single-user setup.

## Publish

After review, merge, and green checks, push annotated tag `v0.0.5` at the approved merge commit. The tag workflow rebuilds/tests the image, publishes version and commit-SHA images to GHCR, exports the image tar and SHA-256 checksum, and creates the GitHub Release using `docs/release-v0.0.5.md`.

## After publish

- [ ] Confirm `ghcr.io/quietsugar/my-known-world:v0.0.5` exists and the GitHub Release contains the image tar and checksum.
- [ ] Download the tar, verify the checksum, and run the documented smoke test.
- [ ] Keep the usage guidance explicit: local/private single-user early access; no multi-user collaboration or full valid-time/point-in-time graph history.
