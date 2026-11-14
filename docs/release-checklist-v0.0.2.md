# v0.0.2 release checklist

This checklist prepares the release but does not publish a tag or GitHub release.

## Before tagging

- [ ] Finish and review the changes on the active branch; ensure no private graph data, credentials, source configuration, internal addresses, or document bodies are present.
- [ ] Merge the release candidate to `master` through the normal repository process.
- [ ] Confirm `VERSION` is `0.0.2`, Compose defaults use image tag `v0.0.2`, and `docs/release-v0.0.2.md` matches the final product.
- [ ] Confirm CI passes on the merge commit: formatting, `go vet`, race tests, PostgreSQL integration tests, web production build, hardened container build, and smoke tests.
- [ ] Review the generated image artifact and checksum from CI; local container build is not a release gate in this environment.
- [ ] Verify write-token setup, PostgreSQL backup/restore process, and the synthetic-demo versus existing-database configuration for the intended deployment.

## Publish

After the above checks are green, push the version tag `v0.0.2` at the approved release commit. The tag workflow builds and tests the container, publishes the versioned image and commit-SHA image to GHCR, exports the Docker image tar and SHA-256 checksum, and creates the GitHub release with `docs/release-v0.0.2.md` and GitHub's source archive.

## After publish

- [ ] Confirm the GitHub release assets include the Docker image tar and `.sha256` file, and verify the checksum.
- [ ] Confirm `ghcr.io/quietsugar/my-known-world:v0.0.2` exists and run the documented smoke test against the deployed image.
- [ ] Announce the release as early access; do not imply incomplete temporal queries, broad real-source adapters, multi-user access, or all evidence editing are available.
