# Realistic hall-mcp replacement corpus

This directory is a **fully synthetic, public-safe** Markdown corpus modeled on the shape of an operational personal knowledge base. It contains no copied personal records, production names, internal addresses, credentials, or document bodies. Names use `example.test`; IP addresses come from documentation-only TEST-NET ranges; the vault URI is a non-resolvable pointer, not a secret.

The goal is to support [#22](https://github.com/QuietSugar/my-known-world/issues/22) development and acceptance without publishing the owner's real corpus.

## Layout

- `config.yaml`: explicit source-type and machine-kind mappings.
- `wiki/`: ten valid modeled documents plus one plain Markdown note.
- `invalid/`: five invalid records and two valid reference targets for schema/admission tests.

Run the current importer without applying anything:

```sh
go run ./cmd/myknownworld-markdown-import \
  --root internal/importer/testdata/hall-mcp-replacement/wiki \
  --config internal/importer/testdata/hall-mcp-replacement/config.yaml \
  > /tmp/hall-mcp-replacement-report.json
```

## Valid-corpus target behavior

The valid corpus covers every hall-mcp source type: `hardware`, `rig`, `shared`, `service`, `product`, `runbook`, `task`, `design`, and `network`.

The ten entities should produce five typed frontmatter relations:

- hardware `hosts` rig
- rig `hosts` the knowledge-wiki service
- shared machine `hosts` the status-site service
- product `hasInstance` knowledge-wiki
- task `relatedTo` knowledge-wiki

Markdown bodies contain eleven additional directed entity-to-entity links. They should become navigable relations when Markdown-link import is implemented:

| Source | Targets |
| --- | --- |
| knowledge-wiki service | product, restore runbook, network |
| status-site service | network |
| product | knowledge-wiki service, federation design |
| restore runbook | product, knowledge-wiki service |
| federation design | knowledge-wiki service, import task |
| network | knowledge-wiki service |

The service also links to `notes/operations.md`. That file deliberately has no frontmatter, so it must be excluded and must not produce a relation. Markdown prose itself must not enter the graph merely because links are parsed.

Useful acceptance queries include:

- Find services hosted by `machine/lab/app-vm.md` and by the shared machine.
- Find the wiki by `wiki.example.test`, port `8080`, deployment path, or product name.
- Find hardware by `fixed@home-lab` and distinguish `rig` from `shared` machine kinds.
- Read shared-machine constraints and owned responsibilities without treating them as a hardware parent.
- Follow product-instance and task-related references in both directions.
- Preserve `stopped` service state and `draft` design state rather than presenting either as active operational fact.
- Keep the runbook scope and network scope as typed details.

## Invalid-corpus target behavior

Run the same importer against `invalid/`. These five source records should be rejected as queryable current entities, with actionable exclusions:

- `services/missing-status.md`: missing required service status.
- `services/wrong-host-type.md`: service host points to a product.
- `services/broken-host.md`: service host does not exist.
- `machine/empty-shell.md`: rig has none of hostname, IP, or OS.
- `designs/invalid-status.md`: design uses `active` instead of a design lifecycle value.

The machine and product targets are valid support records. Invalid children must not create valid-looking host edges merely because their targets parse.

## Change and rename drills

This corpus can also exercise explicit, reviewed synchronization without a watcher:

1. Import the unchanged corpus twice; the second preview must be empty.
2. Change a domain, port, status, constraint, or scope and inspect the exact assertion delta.
3. Rename `machine/lab/app-vm.md` and supply an explicit identity mapping; hosting relations must retain stable identities.
4. Remove a complete service document and require preview to show the resulting stale/missing effects before apply.
5. Add an invalid document and ensure the rest of the complete scan remains reviewable without silently admitting the invalid entity.

Always apply generated batches only to a disposable test database. The fixture is input for development, not authorization to write to a real deployment.
