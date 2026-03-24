# Task 3 — Render Dashboard Staleness Badge

**Dependencies:** Task 2 | **Issue:** N/A

---

## Goal

Replace the current runtime-only activity text in the dashboard row with a compact, color-coded git staleness badge that helps users scan for old worktrees without widening the layout.

## Scope Boundary

**In scope:**
- Add a compact staleness formatter in the dashboard pane
- Replace the visible `RelativeTime(wt.LastActivityAt)` column with a badge derived from `HeadCommittedAt`
- Add renderer tests for label formatting and threshold colors/placement

**Out of scope (handled by other tasks):**
- Loading commit metadata from git
- Merge-status indicators such as `merged ✓`
- Any change to worktree delete confirmation rules

**Cross-task consistency check:**
- The renderer must consume the `HeadCommittedAt` field added in Task 1 and carried through Task 2; it must not fall back to `LastActivityAt`

## Reference

Files to study before implementing:

- `internal/ui/panes/dashboard.go:46-113` — current row layout and relative time helper
- `internal/ui/panes/dashboard_test.go:135-208` — current changed-file and relative-time render tests
- `internal/ui/theme/theme.go:15-25` — approved semantic ANSI colors

## Design References

- `docs/projects/worktree-staleness/dashboard-staleness-wireframe.svg`

## Data Transformation Checklist

- [ ] Proto definition — N/A (render-only change)
- [ ] Proto → Domain converter — N/A
- [ ] Domain model struct — read existing field from Task 1
- [ ] Params struct(s) — N/A
- [ ] Params conversion functions — N/A
- [ ] Any adapters between param types — N/A

## Files to Create/Modify

| File | Action |
|------|--------|
| `internal/ui/panes/dashboard.go` | Modify |
| `internal/ui/panes/dashboard_test.go` | Modify |
| `docs/projects/worktree-staleness/dashboard-staleness-wireframe.svg` | Reference only |

## Requirements

**Functionality:**
- Format staleness as `6h`, `3d`, `2w`, or `3mo` depending on age
- Use `theme.Clean` for `< 3d`, `theme.Dirty` for `3d-7d`, `theme.Error` for `> 7d`, and `theme.Muted` for placeholder values
- Keep the row width stable by replacing, not appending to, the old activity column

**Key gotchas:**
- Do not render `"ago"` strings; the dashboard row does not have room for prose timestamps
- Bare worktrees or zero timestamps should show a muted placeholder such as `--`
- Avoid introducing new theme tokens for a three-bucket badge

## Tests

Test cases:
- Recent, mid-age, and old timestamps render the expected compact labels
- Bare or zero timestamps render the placeholder
- Dashboard row output still includes branch/basename/commit info with the new badge inserted

## Acceptance Criteria

- [x] Dashboard row shows compact staleness instead of runtime activity
- [x] Threshold styling uses existing semantic theme colors only
- [x] Tests pass
