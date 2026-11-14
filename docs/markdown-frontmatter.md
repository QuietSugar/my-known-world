# Read-only Markdown frontmatter adapter

`cmd/myknownworld-markdown-import` scans a complete local wiki tree and writes one JSON `SourceBatch` report to stdout. It parses YAML frontmatter only; body text and Markdown links are not imported. A scan error aborts instead of emitting a partial complete batch. Missing/unclosed or invalid frontmatter is reported in `exclusions` and does not stop the scan.

Keep the root path and real configuration on the private machine. Begin with the synthetic example at [`markdown-adapter-config.example.yaml`](markdown-adapter-config.example.yaml):

```sh
go run ./cmd/myknownworld-markdown-import \
  --root /private/path/to/wiki \
  --config /private/path/to/wiki-source.yaml > /private/path/to/wiki-batch.json
```

The config requires an explicit `source` and `typeMappings`. Unknown and missing type values create `ProvisionalObject` candidates rather than guessed types. `identityMappings` is optional and must be deliberately updated after a rename, mapping the new source-relative path to its existing canonical node ID. The adapter never infers renames. Machine kind can be supplied in frontmatter (`machine_kind`, or the configured `machineKindField`) or by `machineKindMappings`; accepted values are physical, virtual, and shared. Shared-machine `constraints` remain a list in `MachineDetails.operationConstraints`.

The batch uses slash-normalized paths such as `machine/example.md` as source-side IDs. `parent` and `host` references are resolved against known Markdown paths (root-relative first, then document-relative); valid type-compatible references become `hosts` relations. Missing, unmapped, invalid, or incompatible references are reported. No relation is silently invented. Credentials are filtered recursively by default key-name patterns; add optional `secretKeyPatterns` to the private config. The report only gives a generic exclusion reason, never the credential key's value. Unknown keys are ignored rather than copied wholesale.

Supported typed fields in this initial slice are Machine hostname, IP list, OS, status, kind and operation constraints, plus SoftwareInstance deploy path, domains list and status. Values are present in the private batch because these are data-import records; never commit real batches. Field assertions contain supported-field metadata and a value hash. The adapter emits these fingerprints for mapped fields; hashes are not anonymization and may reveal low-entropy values by guessing. Treat the batch, graph, and hashes as sensitive private data, and do not use this adapter for secrets, document bodies, or any fields your deployment must not retain or fingerprint. Use only synthetic values in public fixtures.

The output is intended for `POST /api/sync/preview` and for the existing explicitly authenticated apply process. It does not write to the wiki or graph itself.
