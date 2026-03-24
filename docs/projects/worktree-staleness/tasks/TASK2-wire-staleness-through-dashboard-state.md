# Task 2 — Wire Staleness Through Dashboard State

**Dependencies:** Task 1 | **Issue:** N/A

---

## Goal

Populate the new commit-age field in the dashboard loader and preserve the existing runtime refresh behavior, so the model can carry both git staleness and session-local activity without confusing the two.

## Scope Boundary

**In scope:**
- Update `worktreeLoaderImpl.LoadWorktrees()` to map the new commit metadata field
- Keep `reconcileActivity()` and snapshot logic keyed to runtime state changes only
- Refresh test fixtures and mocks so the new field flows cleanly through dashboard state

**Out of scope (handled by other tasks):**
- The dashboard row badge/formatting itself
- Adding merged-state logic or main-branch comparisons
- Theme changes

**Cross-task consistency check:**
- Task 1 adds `HeadCommittedAt`; this task must ensure loader output includes it and does not accidentally replace `LastActivityAt`

## Reference

Files to study before implementing:

- `internal/app/bootstrap.go:226-263` — dashboard loader assembly
- `internal/ui/dashboard.go:396-497` — snapshot key + runtime activity reconciliation
- `internal/ui/dashboard_test.go:490-788` — dashboard refresh and activity tests

## Design References

N/A (non-UI task)

## Data Transformation Checklist

- [ ] Proto definition — N/A (internal-only dashboard metadata)
- [ ] Proto → Domain converter — N/A
- [ ] Domain model struct
- [ ] Params struct(s) — N/A
- [ ] Params conversion functions — N/A
- [ ] Any adapters between param types — N/A

## Files to Create/Modify

| File | Action |
|------|--------|
| `internal/app/bootstrap.go` | Modify |
| `internal/ui/dashboard.go` | Review and modify only if needed |
| `internal/ui/dashboard_test.go` | Modify |

## Requirements

**Functionality:**
- Each non-bare worktree in `LoadWorktrees()` receives the parsed commit timestamp from Task 1
- Runtime `LastActivityAt` remains the session-local "snapshot changed" signal
- Dashboard refresh tests continue to prove dirty/count/commit transitions update runtime activity independently of git age

**Key gotchas:**
- Do not add `HeadCommittedAt` to `WorktreeSnapshotKey`; commit hash already captures staleness changes at the state boundary
- Keep the loader best-effort: a missing commit timestamp should not fail the entire dashboard

## Tests

Test cases:
- Loader populates the new timestamp when commit metadata is available
- Refresh carries forward runtime `LastActivityAt` exactly as before
- New commit hash still updates runtime activity even with the new git-age field present

## Acceptance Criteria

- [x] Loader populates the new timestamp field
- [x] Runtime activity semantics remain unchanged
- [x] Tests pass
