# V2-T3: Watch Fingerprint Service

## Dependencies
V2-T1, V2-T2.

## Scope
- New `internal/watch/` package for fingerprint calculation, tick scheduling, debounce logic, and messages.
- `internal/gitexec/` reuse only; no direct subprocess calls elsewhere.

## Deliverables
- [ ] Fingerprint service using baseline `git rev-parse HEAD <base-ref>` semantics.
- [ ] Watch tick command/message model suitable for Bubble Tea.
- [ ] Debounce/in-flight rule: skip refresh while one is already in flight, reevaluate on next tick.
- [ ] Linked-worktree documentation/tests covering shared remote refs and per-worktree `HEAD` behavior.

## Test Strategy
- Fake `GitRunner` tests for stable fingerprint, changed fingerprint, and in-flight skip behavior.
- Linked-worktree fixture tests that mutate shared remote-tracking refs.
- Tick-interval tests using fake clock or deterministic injected scheduler.

## Out of Scope
- Idle view rendering.
- Working-tree-sensitive optional fingerprint.
- tmux launcher changes beyond docs.

## Verification
```
go test ./internal/watch ./internal/source -run 'Test(Fingerprint|Watch|LinkedWorktree)'
```
