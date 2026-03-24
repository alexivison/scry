# Task 1 — Add Worktree Commit Age Data

**Dependencies:** none | **Issue:** N/A

---

## Goal

Extend the existing worktree commit metadata path so each non-bare worktree carries a raw HEAD commit committer timestamp alongside the already-rendered hash and subject, without adding another git round trip.

## Scope Boundary

**In scope:**
- Add a raw git commit timestamp field to `model.WorktreeInfo`
- Replace the hash/subject-only gitexec helper with a delimiter-safe commit metadata helper
- Add gitexec/model-level tests for the new parsed metadata

**Out of scope (handled by other tasks):**
- Wiring the new field into dashboard refresh/load state
- Rendering any new badge or changing dashboard layout
- Merge-status detection relative to `main`

**Cross-task consistency check:**
- `HeadCommittedAt` must be introduced here, then explicitly mapped in Task 2 and rendered in Task 3

## Reference

Files to study before implementing:

- `internal/gitexec/worktree.go:79-104` — existing commit metadata and status helpers
- `internal/model/worktree.go:5-15` — dashboard model shape
- `internal/app/bootstrap.go:252-257` — current commit metadata call site

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
| `internal/model/worktree.go` | Modify |
| `internal/gitexec/worktree.go` | Modify |
| `internal/gitexec/worktree_test.go` | Modify or create |

## Requirements

**Functionality:**
- Add a raw timestamp field such as `HeadCommittedAt time.Time` to `WorktreeInfo`
- Parse hash, committer timestamp, and subject from a single `git log -1` command
- Keep bare worktrees and missing commit metadata on the zero value path

**Key gotchas:**
- Do not parse a timestamp by splitting on spaces; use a delimiter-safe format so subjects remain intact
- Preserve the existing "best effort" loader behavior by returning structured errors from gitexec and letting the loader decide whether to ignore them

## Tests

Test cases:
- Happy path commit metadata parsing with a normal multi-word subject
- Subject parsing remains correct when the subject itself contains extra spaces
- Malformed or empty output returns an error instead of silently corrupting fields

## Acceptance Criteria

- [x] `WorktreeInfo` can represent a raw HEAD commit timestamp
- [x] Git metadata helper returns hash, subject, and committer time from one command
- [x] Tests pass
