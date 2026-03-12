# V2-T4: Watch Mode Bootstrap and Auto-Refresh Integration

## Dependencies
V2-T1, V2-T2, V2-T3.

## Scope
- `internal/app/` — wire watch services into bootstrap.
- `internal/ui/` — start initial watch tick in `Init`, handle fingerprint/tick messages, update help and status indicators.

## Deliverables
- [x] `--watch` starts periodic fingerprint checks after initial bootstrap.
- [x] Stable fingerprint causes no churn.
- [x] Changed fingerprint triggers the same shared refresh path as `r`.
- [x] Help/status text exposes watch mode, interval, and last refresh/check timestamps.
- [x] Watch loop stops cleanly on quit.

## Test Strategy
- Bubble Tea model tests for tick scheduling, no-refresh on stable fingerprint, and refresh trigger on changed fingerprint.
- App/bootstrap tests verifying watch mode wires dependencies only when enabled.

## Out of Scope
- Idle screen.
- Commit flow.

## Verification
```
go test ./internal/app ./internal/ui ./internal/watch -run 'TestWatch'
```
