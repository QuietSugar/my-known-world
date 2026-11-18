# Database-authoritative deployment and cutover runbook

This runbook is for the owner-approved authority model: **PostgreSQL/MyKnownWorld is authoritative**. Markdown is only a bootstrap or deliberately triggered import format. MyKnownWorld does not watch a Markdown tree, run scheduled imports, or make an imported tree authoritative. After bootstrap, maintain facts in MyKnownWorld. Do not leave hall-mcp as a hidden runtime dependency.

## 1. Prepare and protect the deployment

1. Choose a private host and a supported release/container image. Pin the version; do not use a floating `latest` tag.
2. Create a private deployment directory and configure `.env` from `.env.example`. Set a strong, unique database password and a random `MYKNOWNWORLD_WRITE_TOKEN` of at least 32 bytes. Keep both out of Git, shell history, tickets, and client configuration.
3. Keep the API/MCP listener private. The default Compose publication is loopback-only; do not expose it publicly without a separately reviewed authenticated network boundary. Read-only MCP and API queries expose graph contents.
4. Start the service using the documented Compose deployment. Verify the app health check and database health before loading data.
5. Keep a protected backup location and test the backup/restore procedure before cutover.

## 2. Optional one-time Markdown bootstrap

Skip this section when no Markdown corpus is being imported.

1. Create a private, explicit importer config from `markdown-adapter-config.example.yaml`. Map only known frontmatter type spellings. Add identity mappings only for reviewed renames. Never place real paths, secrets, or real source data in public config or fixtures.
2. Take a database backup before importing. Generate a complete frontmatter-only batch with `myknownworld-markdown-import`; do not use a watcher or schedule it.
3. Inspect the batch and its exclusions locally. Confirm that it contains no secret values or Markdown bodies. Check IDs, type mappings, typed details, references, and field assertions. A complete batch may mark source-owned records absent from the scan stale; understand this impact before applying.
4. Use the Web Batch Sync preview or `myknownworld-sync` preview. Review the exact delta, conflicts, and stale/missing records. If the preview is not what was intended, stop and correct the mapping or batch.
5. Apply only with the configured write token and the exact preview-bound plan/revision confirmation. A stale preview must be regenerated. Verify the applied source hash and sync history.
6. Store the batch and importer config privately only if required by local retention policy; generated batches and field fingerprints can contain sensitive facts. Delete temporary exports securely when no longer needed.

The importer only emits a batch; it never writes directly to PostgreSQL. It does not read Markdown bodies, and secret-like keys are excluded. Import is explicit, not continuous. Subsequent changes belong in MyKnownWorld unless the owner deliberately approves another reviewed import with the same safeguards.

## 3. Validate application workflows and cut over clients

1. Load or review the intended graph in the MyKnownWorld UI. Confirm expected current nodes, types, relations, lifecycle states, provisional records, source policies, and source history. Resolve or document import exclusions before cutover.
2. Configure Agent clients to use the read-only MyKnownWorld MCP endpoint or the stdio executable documented in [read-only MCP](read-only-mcp.md). Do not provide a write token to an Agent MCP client. Test entity search, typed entity details, outgoing relations, hosted-service filtering, and graph history against representative synthetic/test records.
3. Confirm human users can browse/search entities, inspect typed details and relationships, see source/evidence and lifecycle, review provisionals, and see explicit batch-preview/apply history.
4. Switch Agent and human workflows to MyKnownWorld. Remove hall-mcp from active client configurations only after both workflows are verified. Keep the former service stopped but recoverable for the agreed rollback window; it must not be a runtime dependency.
5. Record the release version, database snapshot identifier, client configuration change, verification results, and rollback owner/date in the private operations log.

## 4. Normal operation and upgrades

- Make day-to-day graph edits in MyKnownWorld, using guarded previews and authenticated apply for writes. Review source ownership and field assertion conflicts; do not treat a source priority as automatic truth selection.
- Markdown is not a freshness mechanism. The UI and MCP show the MyKnownWorld database state, not whether an external Markdown tree has changed. If an explicit re-import is requested, treat it as a new planned change: back up, scan the complete tree, review exclusions and stale effects, preview, obtain approval, then apply.
- Before upgrades, back up PostgreSQL, pin the target release, review release notes/migrations, and test the upgrade against a restored backup or staging copy. Run health/API/MCP/UI checks before promoting.
- Restrict access to the database, backups, import batches, logs, and write token. Avoid including sensitive values in public fixtures or logs.

## 5. Backup, restore, and recovery

Use PostgreSQL-native backup tools suitable for the deployed version and storage; protect and encrypt backup files and retain them under the deployment's recovery policy. For example, from an authorized database host, create a custom-format dump with `pg_dump -Fc` and restore into an isolated empty database with `pg_restore`. Do not overwrite the only production copy during a restore drill.

For a recovery drill, restore to an isolated deployment, start MyKnownWorld against the restored database, and verify node/relation counts, source hashes, sync history, assertion/resolution history, and read-only MCP/UI queries. A database restart must preserve the graph and history. A new empty deployment can only be reconstructed from retained explicit source batches or deliberate re-imports plus any graph changes made after import; Markdown alone is not guaranteed to contain those later database-authoritative edits. Maintain backups accordingly.

## 6. Rollback

If cutover validation fails, stop routing clients to MyKnownWorld, restore the previous client configuration, and start the preserved hall-mcp deployment only if the rollback owner approves. Do not try to reverse an uncertain import by running another full import: inspect MyKnownWorld sync history and use the guarded undo operation only when its preconditions still match, or restore the pre-change database backup. Preserve logs and the failed release/database snapshot for diagnosis. Re-run the cutover checklist before attempting again.

## 7. Authority boundary and non-goals

This runbook does not promise automatic Markdown synchronization, freshness detection against external files, or live consistency with hall-mcp. Those capabilities are intentionally out of scope under the current authority decision. Any future external synchronization requires a separate explicit decision covering authority, conflicts, deletion/stale semantics, triggers, and rollback. PostgreSQL remains the source of truth in the meantime.
