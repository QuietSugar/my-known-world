# v0.0.10 tester pre-release checklist

- [x] Increment `VERSION` to `0.0.10` and use matching `v0.0.10` image defaults.
- [x] Build and package `myknownworld-cmdb` in the container image; route same-origin `/cmdb` to its separate listener.
- [x] Keep the CMDB listener internal to the Compose network; use its app token independently of the graph write token.
- [x] Add encrypted file-vault lifecycle and HTTP tests; CI exercises create/read/update/rotate/disable using synthetic values.
- [x] Document this release as a pre-release, local/test only; make the tag workflow create a GitHub pre-release.
- [ ] Wait for CI on the release commit, including PostgreSQL/race tests, web build, hardened Compose smoke checks, and local-vault lifecycle.
- [ ] Push `v0.0.10` only after the release commit checks pass; confirm the GitHub Release and container tar/checksum artifacts were produced.
- [ ] Share the published pre-release with testers and collect only synthetic feedback (no private corpus, real credentials, or production cutover claims).
