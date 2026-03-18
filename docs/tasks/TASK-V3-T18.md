# V3-T18: Worktree Deletion from Dashboard

## Dependencies
V3-T11.

## Scope
- `d` on a selected worktree opens a confirmation prompt.
- Dirty worktrees show a stronger warning and require force-delete confirmation.
- Main (non-linked) worktree cannot be deleted — `d` is a no-op with status message.
- Deletion uses `git worktree remove <path>` (force variant: `--force`).
- Post-deletion: refresh dashboard and reconcile selection.

## Files
- `internal/gitexec/worktree.go`
- `internal/model/worktree.go`
- `internal/ui/dashboard.go`
- `internal/ui/panes/dashboard.go`

## Deliverables
- [ ] `WorktreeRemove(ctx, runner, path, force)` added to gitexec.
- [ ] `ConfirmDelete` state + selected worktree path in `DashboardState`.
- [ ] `d` → confirm → execute → refresh flow wired in dashboard.
- [ ] Main worktree protected — `d` shows status message.
- [ ] Dirty worktree shows force-delete warning.
- [ ] Selection reconciles to nearest neighbor after deletion.

## Test Strategy
- Unit test for `WorktreeRemove` with mock runner.
- State machine test for confirm → execute → refresh flow.
- Test main worktree protection.
- Test dirty worktree force-delete path.

## Out of Scope
- Branch deletion after worktree removal.

## Verification
```
go test ./internal/ui/... ./internal/gitexec/... -count=1
go vet ./...
```

## Complexity
M
