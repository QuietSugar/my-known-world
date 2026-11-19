# MyKnownWorld v0.0.8

An early-access release that strengthens explicit Markdown imports using configurable admission checks, tested against the fully synthetic hall-mcp replacement corpus.

## Highlights

- Add per-source-type `requiredFields`, `requiredAnyFields`, and `statusValues` rules to Markdown import configuration.
- Exclude invalid mapped entities from the import batch with actionable reasons; invalid or incompatible `parent`/`host` references exclude the child entity so they cannot create apparently valid host relations.
- Add corpus-level tests for all valid synthetic records and the five documented invalid examples, including expected relationship counts.
- Retain safe relative Markdown-link extraction, typed frontmatter mapping, and reviewed explicit imports. No watcher or continuous synchronization is introduced.

## Authority and limitations

PostgreSQL/MyKnownWorld remains authoritative. Markdown is for initial or explicitly triggered, reviewed imports only. This release was verified against the public synthetic fixtures; it does not establish equivalence with any private corpus, authorize production writes, or establish deployment/cutover readiness. Keep real data, generated reports/batches, credentials, private configuration, and Markdown body text out of public repositories.

## Verification

GitHub Actions passed Go race tests, Web production build, and container/API/UI checks for the synthetic-corpus implementation. See `internal/importer/testdata/hall-mcp-replacement/README.md` for fixture scope and documented import drills.
