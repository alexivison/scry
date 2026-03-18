# V3-T16: Event-Driven Refresh with Polling Fallback

## Dependencies
V3-T7.

## Scope
- Add `fsnotify`-based file change notifications for faster refresh than polling.
- Debounce strategy: sliding window, 150ms quiet period, collapses rapid agent writes into one refresh.
- Polling remains as automatic fallback when fsnotify fails (NFS, WSL edge cases, inotify limit hit).
- Detection: attempt `watcher.Add()` at startup — error triggers silent fallback.

## Design Note
Standard fsnotify does not recurse into subdirectories. Options:
1. Watch a computed set of active directories (those containing changed files).
2. Accept "best effort acceleration" — watch worktree root + `.git`, rely on polling for nested writes.
3. Use a recursive watcher dependency.

Decision should be made during implementation based on testing. Option 2 is the pragmatic starting point — the polling fallback ensures correctness regardless.

## Files
- `internal/watch/watch.go` (or new `internal/watch/fswatch.go`)
- `internal/app/bootstrap.go`
- `go.mod` (add `github.com/fsnotify/fsnotify`)

## Deliverables
- [ ] fsnotify watcher initialized at startup when supported.
- [ ] Debounced refresh triggered on file change events.
- [ ] Unsupported environments fall back to polling without error.
- [ ] Watcher resources cleaned up on quit.
- [ ] Existing polling path remains as-is for fallback.
- [ ] `watch.interval` config still applies to polling fallback.

## Test Strategy
- Unit test for debounce logic (mock event stream).
- Integration test verifying fallback when watcher.Add fails.
- Cleanup test verifying no leaked goroutines/watchers.

## Out of Scope
- Recursive directory watching (use computed set or best-effort root).

## Verification
```
go test ./internal/watch/... ./internal/app/... -count=1
go vet ./...
```

## Complexity
L
