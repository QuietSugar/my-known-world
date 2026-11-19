---
type: shared
hostname: shared-cloud.example.test
ip: 203.0.113.20
os: Ubuntu 24.04
status: active
constraints:
  - Read-only without explicit approval
  - Only owner-managed deployments may be changed
owned:
  - application deployments
---
# Shared cloud

A synthetic shared machine with explicit operational boundaries and no hardware parent.
