# V3-T17: Dashboard Preview Pane

## Dependencies
V3-T1, V3-T2, V3-T11.

## Scope
- When a worktree is selected (highlighted, not drilled into), show a side panel with the top 5 changed files and their `+/-` counts.
- Lazy-load preview data for the selected worktree only — no thundering herd on every refresh tick.
- Cache preview per worktree; invalidate when worktree snapshot changes.

## Files
- `internal/model/worktree.go`
- `internal/ui/dashboard.go`
- `internal/ui/panes/dashboard.go`
- `internal/app/bootstrap.go`

## Deliverables
- [ ] Preview pane shows top 5 changed files with `+/-` counts for selected worktree.
- [ ] Preview loads lazily on selection change, not on every tick.
- [ ] Preview cached per worktree; invalidated on snapshot change.
- [ ] Preview adapts to available width (hidden in narrow layouts).
- [ ] Drill-down behavior remains intact.

## Test Strategy
- Test lazy-load trigger on selection change.
- Test cache hit when re-selecting same unchanged worktree.
- Test cache invalidation when worktree state changes.
- Render test for preview pane at various widths.

## Out of Scope
- Full file list in preview (top 5 only).
- Worktree deletion (V3-T18).

## Verification
```
go test ./internal/ui/... ./internal/model/... -count=1
go vet ./...
```

## Complexity
L
