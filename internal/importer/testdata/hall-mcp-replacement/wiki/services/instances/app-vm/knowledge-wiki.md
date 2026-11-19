---
type: service
name: knowledge-wiki
host: machine/lab/app-vm.md
domain:
  - wiki.example.test
  - knowledge.example.test
ports: [8080, 8443]
deploy_path: /srv/knowledge-wiki
credentials: vault://synthetic/knowledge-wiki-admin
status: active
---
# Knowledge wiki

Product notes live in [the product record](../../../runbooks/products/knowledge-wiki/README.md).

Recovery follows [the restore runbook](../../../runbooks/restore-knowledge-wiki.md), using [the lab network](../../../network/lab.md).

This [plain operations note](../../../notes/operations.md) is not an entity and must not create a graph edge.
