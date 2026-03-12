# V2-T3: Watch Fingerprint Service

## Dependencies
V2-T1, V2-T2.

## Scope
- New `internal/watch/` package for fingerprint calculation, tick scheduling, debounce logic, and messages.
- `internal/gitexec/` reuse only; no direct subprocess calls elsewhere.

## Deliverables
- [x] Fingerprint service that is **compare-mode aware**:
  - Committed-ref mode: `git rev-parse HEAD <base-ref>` (fast, covers push/pull).
  - Working-tree mode: additionally incorporates `git diff --name-only` and/or index stat to detect staged/unstaged edits. Without this, the default `--head`-omitted mode would never trigger watch refreshes for the primary change class.
- [x] Watch tick command/message model suitable for Bubble Tea.
- [x] Debounce/in-flight rule: skip refresh while one is already in flight, reevaluate on next tick.
- [x] Linked-worktree documentation/tests covering shared remote refs and per-worktree `HEAD` behavior.

## Test Strategy
- Fake `GitRunner` tests for stable fingerprint, changed fingerprint, and in-flight skip behavior.
- Linked-worktree fixture tests that mutate shared remote-tracking refs.
- Tick-interval tests using fake clock or deterministic injected scheduler.

## Out of Scope
- Idle view rendering.
- Performance tuning of working-tree fingerprint (e.g. inotify/fswatch integration).
- tmux launcher changes beyond docs.

## Verification
```
go test ./internal/watch ./internal/source -run 'Test(Fingerprint|Watch|LinkedWorktree)'
```
