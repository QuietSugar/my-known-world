# MyKnownWorld v0.0.6

An early-access release focused on explicit, database-authoritative Markdown bootstrap and improved read-only Agent discovery. PostgreSQL/MyKnownWorld remains authoritative; Markdown is imported only through a deliberate reviewed batch. No watcher or continuous Markdown replacement is introduced.

## Highlights

- Expanded Markdown frontmatter mapping for Hardware, Machine (including distinct `rig` and shared-machine `owned`), SoftwareProject, SoftwareInstance (`ports` and service fields), Product, Document, Runbook, WorkItem, Network, and AccountCredential metadata.
- Added typed `hasInstance` and `relatedTo` relationships and PostgreSQL migration 018, alongside host/reference validation and field assertions for modeled fields.
- Invalid field shapes, service ports, and network CIDRs are handled conservatively; invalid machine IP evidence is excluded without discarding otherwise valid machine details. Secret-like values and Markdown bodies are not imported.
- MCP graph queries can filter entities by hosting machine and case-insensitively search safe modeled fields including hostnames, hardware locations, service domains/ports, and source references.
- Added a database-authoritative deployment/cutover runbook for explicit bootstrap, guarded preview/apply, read-only MCP and human workflows, backup/restore, upgrades, and rollback.

## Authority and limitations

This release does not provide automatic Markdown sync, watcher/freshness monitoring against an external corpus, or Markdown body/link import. Make ongoing edits in MyKnownWorld. A later import must be deliberately generated, reviewed, backed up, previewed, and explicitly applied because it can change source-owned claims and mark absent records stale. Do not publish real source data, import batches, credentials, private configuration, or Markdown bodies.

This remains an early-access, single-operator application for private deployments. Full real-corpus compatibility and owner-run cutover verification are deployment-specific and must be confirmed before retiring any existing system. See `docs/database-authoritative-cutover.md`.

## Verification

GitHub Actions runs Go formatting/vet/race tests and PostgreSQL integration tests, the Web production build, and hardened container/API/UI checks. For installation and smoke testing, see `README.md`; verify the downloaded image tar against its published SHA-256 checksum before loading it.
