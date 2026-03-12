# V2-T11: Worktree Dashboard Mode

## Dependencies
V2-T0 (working tree diff mode), V2-T1 (CLI/config scaffolding for `--worktrees` flag), V2-T2 (shared refresh orchestrator), V2-T3 (watch fingerprint service).

## Motivation
Our party setup has a master session orchestrating worker sessions, each working in a linked worktree. The master needs visibility into what all workers are doing. When launched with `--worktrees` (or auto-detected from the main worktree), scry shows a dashboard of all git worktrees in the repo instead of the normal file-list + diff view.

## Scope
- `internal/gitexec/` — add `WorktreeList` and per-worktree `StatusPorcelain` commands.
- `internal/model/` — add `WorktreeInfo` type and dashboard state.
- `internal/ui/panes/` — new `worktree_dashboard.go` pane.
- `internal/ui/model.go` — route to dashboard view when in worktree mode.
- `cmd/scry/` — add `--worktrees` flag, help text.

## Deliverables
- [ ] Parse `git worktree list --porcelain` to discover all worktrees (bare, linked, prunable).
- [ ] `WorktreeInfo` type: path, branch name, commit hash, dirty/clean state.
- [ ] Dirty state detection: run `git -C <worktree> status --porcelain` per worktree (empty = clean).
- [ ] Dashboard pane displaying: worktree basename, branch name, dirty/clean indicator, last commit summary (short hash + subject).
- [ ] Visual indicators: green = clean, yellow = dirty.
- [ ] Auto-refresh via watch infra (V2-T3) — poll worktree list + per-worktree dirty state on each tick.
- [ ] `--worktrees` CLI flag to enter dashboard mode.
- [ ] Help text updated with dashboard keybindings.

## Key Bindings

### Dashboard Pane
| Key | Action |
|-----|--------|
| `j`/`k` | Navigate worktree list |
| `l`/`Enter` | Drill into selected worktree's diff (reuse V2-T0 working tree diff, scoped to that worktree) |
| `q` | Quit |
| `?` | Help |

### Drill-Down (Working Tree Diff)
| Key | Action |
|-----|--------|
| `h`/`Esc` | Return to dashboard |
| (all diff keys) | Normal diff view keybindings apply |

## Design Notes
- Worktree list column layout: `[status] branch-name  path-basename  short-hash subject`
- Status column: colored dot or icon (green/yellow).
- Selected row highlighted with accent background.
- Drill-down reuses the existing diff view but scoped to the selected worktree's working tree diff against its upstream.
- On return from drill-down, dashboard state (selection, scroll position) is preserved.

## Test Strategy
- Fake `GitRunner` tests for worktree list parsing (bare, linked, prunable entries).
- Dashboard navigation model tests (`j`/`k` selection, `Enter` drill-down, `Esc` return).
- Dirty state detection tests (clean vs dirty worktree).
- Integration test with fixture repo containing linked worktrees (reuse existing worktree fixtures).
- Auto-refresh tests: worktree list change triggers dashboard update.

## Out of Scope
- Sending commands to workers.
- Reading worker Claude pane output.
- Worktree creation/deletion from within scry.
- Network/remote operations.

## Verification
```
go test ./internal/gitexec ./internal/model ./internal/ui ./cmd/scry -run 'Test(Worktree|Dashboard)'
go vet ./...
```
