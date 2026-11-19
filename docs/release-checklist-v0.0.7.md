# v0.0.7 release checklist

Publish only after merge and successful tag CI; do not treat CI as a substitute for private-corpus validation.

## Before tagging

- [ ] Review the release candidate for private source data, credentials, generated batches/reports, internal addresses, and Markdown body leakage.
- [ ] Confirm all version metadata uses 0.0.7 / v0.0.7 consistently.
- [ ] Confirm notes document the relative-link subset and explicitly state that body text is not stored and Markdown is not authoritative.
- [ ] Review synthetic link tests: internal inline/wiki links resolve, duplicates deduplicate, broken targets are safely reported, and external/image/body content is omitted.
- [ ] Merge the candidate to `master`; confirm merge CI passes including PostgreSQL, Go race tests, Web build, container/API/UI tests.
- [ ] Run tag CI and verify GitHub Release assets and published GHCR image.
- [ ] In a disposable real deployment, back up/restore, generate a private report, compare real entity/field/relation sets with hall-mcp, preview before apply, and exercise MCP/UI workflows. Preserve hall-mcp for rollback until user signoff.

## Publish

After review and green merge checks, push annotated tag `v0.0.7` at the approved merge commit. The tag workflow publishes the container image, tar/checksum assets, and GitHub Release using `docs/release-v0.0.7.md`.

## After publish

- [ ] Confirm the image, tar, checksum, and release notes are available at the release page.
- [ ] Verify the image tar checksum before loading it.
- [ ] Record private real-corpus findings and any remaining differences before declaring hall-mcp replaced.
