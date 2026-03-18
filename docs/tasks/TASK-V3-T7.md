# V3-T7: Diff-Aware Cache Invalidation

## Dependencies
None.

## Scope
- Replace the current "clear all patches on refresh" strategy with selective invalidation.
- On metadata refresh, compare old and new file summaries:
  - **Changed files** (different additions/deletions or status): evict from cache, reload if selected.
  - **Unchanged files**: keep cached patch.
  - **New files**: no cache entry, load on selection.
  - **Removed files**: evict from cache.
- Selected file path reconciled after refresh without forcing blanket reload.

## Files
- `internal/review/refresh.go`
- `internal/review/cache.go`
- `internal/ui/model.go`

## Deliverables
- [ ] Metadata refresh preserves cached patches for unchanged files.
- [ ] Removed files are evicted; changed files are evicted and reloaded if selected.
- [ ] New files load on first selection.
- [ ] Cache generation still increments on refresh (for stale-response guards).
- [ ] Selected file reconciliation works without blanket cache clear.

## Design Note
Summary-level comparison (additions/deletions/status) is sufficient for cache invalidation.
For exact scroll preservation on same-count edits, V3-T9 will compare actual patch content.

## Test Strategy
- Unit tests for selective cache invalidation logic.
- Integration test: refresh with mix of changed/unchanged/new/removed files.
- Verify selected file stays selected when path survives.

## Out of Scope
- Scroll preservation (V3-T9).
- Freshness tracking (V3-T8).

## Verification
```
go test ./internal/review/... ./internal/ui/... -count=1
go vet ./...
```

## Complexity
L
