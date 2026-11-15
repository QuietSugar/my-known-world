# v0.0.3 release checklist

This checklist prepares the release; it does not create or push a tag.

## Before tagging

- [ ] Review the release candidate on the active branch; confirm no private graph data, credentials, source configuration, internal addresses, or document bodies are included.
- [ ] Merge the approved release candidate into `master` through the normal repository process.
- [ ] Confirm `VERSION` is `0.0.3`, Compose and `.env.example` default to `v0.0.3`, and `docs/release-v0.0.3.md` accurately describes the merged product and limitations.
- [ ] Confirm CI passes on the merge commit: formatting, `go vet`, race tests, PostgreSQL integration tests, web production build, hardened container build, and smoke tests.
- [ ] Review the CI image artifact and checksum; verify write-token setup and PostgreSQL backup/restore for the intended deployment.

## Publish

Only after the merge commit is approved and all checks are green, push tag `v0.0.3` at that commit. The tag workflow builds and tests the container, publishes version and commit-SHA images to GHCR, exports the image tar and SHA-256 checksum, and creates the GitHub release from `docs/release-v0.0.3.md` plus GitHub's source archive.

## After publish

- [ ] Confirm release assets contain the Docker image tar and `.sha256` file; verify the checksum.
- [ ] Confirm `ghcr.io/quietsugar/my-known-world:v0.0.3` exists and run the documented smoke test against the deployed image.
- [ ] Announce this as early access; do not imply full temporal queries, broad real-source adapters, multi-user access, or complete preview coverage for field assertions/source policies.
