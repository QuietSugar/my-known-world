# MyKnownWorld v0.0.9

An early-access release that distinguishes newly missing source records from records that were already marked stale, while preserving safe non-deleting synchronization behavior.

## Highlights

- Sync previews retain existing `missingNodes`, `missingRelations`, and `missingAssertions` fields for compatibility.
- Add `newlyMissingNodes`, `newlyMissingRelations`, and `newlyMissingAssertions` to identify first-time absence transitions.
- Add `alreadyStaleNodes`, `alreadyStaleRelations`, and `alreadyStaleAssertions` for records that remain absent after an earlier apply.
- Display the new/stale distinction in the UI and add synthetic regression coverage across initial preview, apply, and repeated preview.
- Include the migration replay fix for Markdown `relatedTo` and `hasInstance` predicates from PR #28.

Missing records are still not automatically deleted. PostgreSQL/MyKnownWorld remains authoritative. Markdown and source sync remain explicit and reviewed; no watcher or continuous synchronization is introduced. Synthetic tests do not establish private-corpus equivalence or production cutover readiness.
