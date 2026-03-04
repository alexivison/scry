# T8: Async Lazy Patch Loading, Cache, Viewport Virtualization

## Dependencies
T7 (patch pane).

## Scope
- `internal/review/` — Review state management, patch cache, generation guard.
- `internal/ui/` — Async message handling, loading indicators, viewport virtualization.

## Deliverables
- [ ] Patch loading triggered asynchronously on file selection via Bubble Tea Cmd.
- [ ] `PatchLoadState` lifecycle: Idle → Loading → Loaded/Failed.
- [ ] Cache keyed by file path within current `CacheGeneration`.
- [ ] Revisiting a file in the same generation uses cached patch (no re-fetch).
- [ ] Generation guard: async responses include generation ID; stale responses discarded.
- [ ] Loading indicator shown while patch is in-flight.
- [ ] Viewport virtualization: only render visible lines, handle large patches without freezing.
- [ ] File list renders immediately (metadata-first paint) before any patches load.

## Test Strategy
- Test cache hit: load file, load same file again → no second GitRunner call.
- Test generation guard: simulate stale response with old generation ID → discarded.
- Test LoadStatus transitions.
- Test concurrent file selection while load is in-flight.

## Out of Scope
- Search (T9).
- Manual refresh / cache invalidation (T9a).
- Whitespace toggle (T10).

## Verification
```
go test ./internal/review ./internal/ui -run TestLazyLoad
```
