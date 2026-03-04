# T9a: Manual Refresh Action

## Dependencies
T8 (async loading, cache, generation guard).

## Scope
- `internal/review/` — Refresh pipeline, generation bump, selection reconciliation.
- `internal/ui/` — `r` keybinding, refresh feedback.

## Deliverables
- [ ] `r` key triggers manual refresh from any pane.
- [ ] Refresh pipeline: increment CacheGeneration → clear all patch cache → reload metadata (re-run MetadataService.ListFiles) → reload selected file patch.
- [ ] Selection reconciliation: preserve selected file by path if still present; if removed, select nearest valid row by index.
- [ ] Stale async responses discarded by generation guard (same mechanism as T8).
- [ ] UI remains responsive during refresh (async metadata + patch reload).
- [ ] `r` appears in key-help text.
- [ ] Reuses same async loading pipeline as initial load (no separate refresh code path).

## Test Strategy
- Test generation bump clears cache.
- Test selection preserved when file still exists after refresh.
- Test selection moves to nearest when file removed.
- Test stale response from pre-refresh generation is discarded.

## Out of Scope
- Watch mode / auto-refresh (v0.2).
- Whitespace toggle (T10 — but shares cache-reset helper).

## Verification
```
go test ./internal/review ./internal/ui -run TestManualRefresh
```
