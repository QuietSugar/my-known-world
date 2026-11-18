# Explicit Markdown frontmatter import

Markdown is an **import format, not the authority for graph state**. PostgreSQL is the authoritative MyKnownWorld database. This adapter reads YAML frontmatter from a complete local Markdown tree and emits a JSON `SourceBatch`; it does not write to either the wiki or database. There is no background watcher or automatic synchronization.

Use this workflow to bootstrap a graph from an existing Markdown corpus, or to perform a deliberate later import after reviewing its consequences. Once imported, make ongoing edits in MyKnownWorld. A later import is an explicit source synchronization: it can update source-owned claims and remove records previously owned by that source when they are absent from the newly scanned complete batch. It does not merge Markdown bodies, and it can conflict with later database edits to imported fields. Always inspect the preview and exclusions, back up the database, and apply only when the proposed changes are intended. Do not run it as a scheduled job or treat the Markdown tree as a live mirror.

## Generate and review a batch

Keep the root path and real configuration on the private machine. Begin with the synthetic example at [`markdown-adapter-config.example.yaml`](markdown-adapter-config.example.yaml):

```sh
go run ./cmd/myknownworld-markdown-import \
  --root /private/path/to/wiki \
  --config /private/path/to/wiki-source.yaml > /private/path/to/wiki-batch.json
```

The same binary is included in the container image. To run the adapter with a read-only wiki mount (and keep its output on the host), use `docker run --rm --entrypoint /usr/local/bin/myknownworld-markdown-import -v "$WIKI_ROOT:/wiki:ro" -v "$WIKI_CONFIG:/config.yaml:ro" ghcr.io/quietsugar/my-known-world:${IMAGE_TAG} --root /wiki --config /config.yaml > "$BATCH_OUTPUT"`. The config requires an explicit `source` and `typeMappings`. Unknown and missing type values create `ProvisionalObject` candidates rather than guessed types. `identityMappings` is optional and must be deliberately updated after a rename, mapping the new source-relative path to its existing canonical node ID. The adapter never infers renames. Machine kind can be supplied in frontmatter (`machine_kind`, or the configured `machineKindField`) or by `machineKindMappings`; accepted values are physical, virtual, shared, and rig. A frontmatter `type: rig` mapped to Machine defaults to the distinct `machineKind: rig`; use `machineKindMappings` to override other source spellings deliberately. Shared-machine `owned` and `constraints` lists remain typed lists in `MachineDetails`.

The batch uses slash-normalized paths such as `machine/example.md` as source-side IDs. `parent` and `host` references are resolved against known Markdown paths (root-relative first, then document-relative); valid type-compatible references become `hosts` relations. Missing, unmapped, invalid, or incompatible references are reported. No relation is silently invented. Credentials are filtered recursively by default key-name patterns; add optional `secretKeyPatterns` to the private config. The report only gives a generic exclusion reason, never the credential key's value. Unknown keys are ignored rather than copied wholesale.

Supported typed fields in this initial slice include Hardware category, model, location and status; Machine hostname, IP list, OS, status, kind, shared-machine `owned` and operation constraints; plus SoftwareInstance deploy path, domains list and status. For compatibility, a `domain` key is accepted as an alias for `domains` and parsed with the same string-list rules. A file containing both keys is excluded with an explicit ambiguity reason; values are never silently merged or prioritized. Values are present in the private batch because these are data-import records; never commit real batches. Field assertions contain supported-field metadata and a value hash. The adapter emits these fingerprints for mapped fields; hashes are not anonymization and may reveal low-entropy values by guessing. Treat the batch, graph, and hashes as sensitive private data, and do not use this adapter for secrets, document bodies, or any fields your deployment must not retain or fingerprint. Use only synthetic values in public fixtures.

## Preview and explicitly apply

The output is intended for `POST /api/sync/preview` and for the existing explicitly authenticated apply process. Preview is non-mutating and reports the proposed delta. Apply is a separate, authenticated action guarded by a preview-bound plan hash and graph revision; it is not performed by the Markdown importer. See [incremental sync](incremental-sync.md) for the exact preview/apply procedure, write-token requirements, source ownership behavior, and backup cautions. A complete scan failure aborts instead of emitting a partial batch; invalid frontmatter is reported in `exclusions`.

## Current limitations

This is not a complete hall-mcp replacement. The adapter currently reads frontmatter only, supports a subset of typed fields and references, and does not import Markdown relative links or bodies. There is no automatic freshness monitoring, watcher, or sync-failure dashboard. PostgreSQL-backed graph data remains authoritative after import; these capabilities should be extended without making Markdown an implicit runtime dependency or authority.
