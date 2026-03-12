# V2-T2: Shared Refresh Orchestrator

## Dependencies
V2-T1.

## Scope
- `internal/review/` — extract refresh helpers so manual refresh, watch refresh, and post-commit refresh reuse the same path.
- `internal/ui/` — swap `startRefresh` internals to call the shared orchestrator while preserving current behavior.

## Deliverables
- [x] Shared refresh function: **compare re-resolution** → generation bump → cache clear → metadata reload → selection reconciliation → optional selected-file patch reload.
- [x] Compare re-resolution re-runs the `Resolver` so `AppState.Compare` reflects current HEAD/base SHAs and `DiffRange` after commits or ref movement.
- [x] Refresh state helpers for `RefreshInFlight` and `LastRefreshAt`.
- [x] Existing `r` key continues to use the shared path, with no regression from T9a.
- [x] Selection reconciliation remains path-first then nearest-index fallback.

## Test Strategy
- Unit tests for generation bump, cache reset, selection reconciliation, and refresh-in-flight state transitions.
- Regression tests proving the `r` path still behaves like T9a.

## Out of Scope
- Polling loop.
- Idle screen rendering.
- Commit execution.

## Verification
```
go test ./internal/review ./internal/ui -run 'Test(Refresh|Selection|ManualRefresh)'
```
