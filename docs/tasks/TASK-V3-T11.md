# V3-T11: Dashboard Activity Summary

## Dependencies
None.

## Scope
- Add per-worktree changed-file count to dashboard rows (e.g., `● 12 files` instead of just a yellow dot).
- Add last-activity timestamp per worktree, displayed as relative time ("3s ago", "2m ago").
- Last-activity is tracked during dashboard refresh by comparing old and new worktree snapshots — not derived from git alone.

## Files
- `internal/model/worktree.go`
- `internal/gitexec/worktree.go`
- `internal/app/bootstrap.go`
- `internal/ui/dashboard.go`
- `internal/ui/panes/dashboard.go`

## Deliverables
- [x] `WorktreeInfo` gains `ChangedFiles int` and `LastActivityAt time.Time`.
- [x] Changed-file count discovered via `git -C <path> diff --name-only` or status porcelain count.
- [x] Last-activity updated when worktree snapshot state changes (dirty/clean transition, count change, new commit).
- [x] Dashboard rows show count and relative time.
- [x] Selection reconciliation remains stable across refreshes.

## Design Note
`LastActivityAt` is maintained in the reconciliation layer when comparing old and new snapshots,
not derived from gitexec alone. This avoids expensive history queries.

## Test Strategy
- Unit tests for snapshot comparison logic.
- Tests for relative time formatting.
- Dashboard render tests with counts and timestamps.

## Out of Scope
- Dashboard preview pane (V3-T17).
- Worktree deletion (V3-T18).

## Verification
```
go test ./internal/ui/... ./internal/gitexec/... ./internal/model/... -count=1
go vet ./...
```

## Complexity
M
