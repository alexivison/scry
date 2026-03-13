# End-to-End Verification Checklist

Manual verification steps for scry v0.2 features. Run these against a real repository with an upstream branch configured.

## Watch Mode

- [ ] `scry --base origin/main --watch` — starts in file list (or idle if no changes)
- [ ] Make a local edit → scry auto-refreshes the file list within the polling interval
- [ ] Commit a change → scry detects the new HEAD and refreshes
- [ ] `git fetch` (if upstream advanced) → scry detects the base ref change

## Idle Screen

- [ ] `scry --base origin/main --head HEAD --watch` on a branch with no divergence → shows idle screen
- [ ] Make a commit that diverges from base → idle screen auto-transitions to file list
- [ ] `q` from idle screen exits cleanly
- [ ] `?` from idle screen shows help overlay

## Commit Flow

- [ ] `scry --base origin/main --commit` → `c` key generates a commit message
- [ ] Generated message follows conventional commit format
- [ ] `e` opens `$EDITOR` for editing the message
- [ ] `Enter` executes `git commit` and shows the short SHA
- [ ] `Esc` cancels and returns to file list
- [ ] `r` in commit pane regenerates the message
- [ ] Error case: no `ANTHROPIC_API_KEY` → shows clear error message
- [ ] Error case: no staged changes → shows error
- [ ] `scry --commit --commit-auto` → auto-commits after generation without confirmation

## Worktree Dashboard

- [ ] `scry --worktrees` → shows list of all worktrees with branch, commit, and dirty state
- [ ] `j`/`k` navigates the worktree list
- [ ] `l`/`Enter` drills into a worktree's diff
- [ ] `h`/`Esc` returns to the dashboard from drill-down
- [ ] `q` exits from the dashboard

## General

- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `go vet ./...` passes
- [ ] Terminal resize during any mode recovers cleanly
- [ ] Split layout (`Tab`) works in file list + patch view
