# MyKnownWorld v0.0.7

An early-access release for validating database-authoritative MyKnownWorld imports against real Markdown archives. This version adds relationship extraction for internal relative Markdown links while retaining the authority boundary: PostgreSQL/MyKnownWorld is authoritative, and every Markdown import is deliberate and reviewed. No watcher or continuous Markdown replacement is introduced.

## Highlights

- Extracts relative inline Markdown and wiki-style links to mapped Markdown records as `relatedTo` graph relations. Link destinations are resolved within the scanned wiki; broken or unmapped targets produce safe path/line exclusions without echoing the raw destination.
- Scans body lines only to extract link targets. Body text, link labels, external URLs, and images are not stored or emitted. This is relationship extraction, not body import.
- Advances the Markdown parser version to `markdown-frontmatter-v3` so reports identify the new parsing semantics.
- Preserves existing frontmatter typed fields, `parent`/`host`, Product `instances`, Task `related`, guarded preview/apply, and read-only MCP/UI features.

## Authority and limitations

PostgreSQL/MyKnownWorld remains the source of truth. Markdown is an explicit bootstrap or reviewed later import only. Do not schedule or continuously run imports; make ongoing edits in MyKnownWorld. Full Markdown syntax is not interpreted: this release supports common inline links and simple wiki links, and deliberately ignores external/absolute links, images, and non-Markdown assets. Source-specific status vocabularies and private-corpus equivalence must be confirmed against the actual archive before cutover. Keep hall-mcp available for rollback until users complete their real-data trial.

Never publish real source data, generated import reports/batches, credentials, private configuration, or Markdown body text.

## Verification

GitHub Actions runs Go formatting/vet/race tests and PostgreSQL integration tests, the Web production build, and hardened container/API/UI checks. CI uses synthetic data; the private real-corpus comparison and cutover rehearsal remain deployment-owner validation steps. See `docs/database-authoritative-cutover.md` and `docs/markdown-frontmatter.md`.
