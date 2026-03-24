# Worktree Staleness Indicator Specification

## Goal

Show git-based staleness for each dashboard worktree so users can quickly spot old branches that are stronger delete candidates than the current runtime-only activity signal suggests.

## Functional Requirements

- The dashboard row shows staleness derived from the worktree HEAD commit timestamp, not `LastActivityAt`.
- Staleness is sourced from the current HEAD commit committer timestamp in the existing commit metadata lookup path.
- The displayed label is compact and column-friendly: `6h`, `3d`, `2w`, `3mo`.
- The badge uses existing ANSI semantic colors only:
  - green for `< 3d`
  - yellow for `3d-7d`
  - red for `> 7d`
  - muted placeholder for bare worktrees or missing commit time
- Dashboard startup must not add another git command per worktree beyond the existing status count + commit metadata calls.
- Runtime `LastActivityAt` tracking stays in the model for snapshot reconciliation, but it is no longer the dashboard staleness display.

## Non-Goals

- Detecting whether a worktree branch is already merged into `main`
- User-configurable thresholds or label formats
- Adding asynchronous background enrichment just for staleness
- Changing worktree deletion rules or confirmation UX

## Acceptance Criteria

- `WorktreeInfo` carries a git-derived commit timestamp that the dashboard renderer can consume.
- `worktreeLoaderImpl.LoadWorktrees` populates that timestamp for non-bare worktrees in the same pass that already loads status count and commit subject.
- Dashboard rows replace the current runtime-relative activity text with the new compact staleness badge.
- Existing dashboard refresh/reconciliation semantics for `LastActivityAt` remain intact.
- Tests cover commit metadata parsing, loader population, staleness label formatting, and dashboard row rendering.
