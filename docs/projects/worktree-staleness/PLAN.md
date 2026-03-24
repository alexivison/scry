# Worktree Staleness Indicator Implementation Plan

> **Goal:** Show git-based staleness for each dashboard worktree so users can quickly identify old branches that are stronger delete candidates than the current runtime-only activity signal suggests.
>
> **Architecture:** Extend the existing commit metadata lookup to return a raw HEAD committer timestamp, store it on `WorktreeInfo`, and render a compact color-coded badge in the dashboard pane. Keep runtime `LastActivityAt` for snapshot reconciliation only; do not reuse it as the staleness source.
>
> **Tech Stack:** Go, Bubble Tea, Lip Gloss, git subprocesses via `internal/gitexec`
>
> **Specification:** [SPEC.md](./SPEC.md) | **Design:** [DESIGN.md](./DESIGN.md)

## Scope

This plan covers the single-repo dashboard path only: git metadata lookup, worktree model wiring, dashboard refresh compatibility, and dashboard row rendering/tests. It deliberately defers merged-to-main detection and user-configurable thresholds so the feature stays cheap to load and easy to reason about.

## Task Granularity

- [x] **Standard** — ~200 lines of implementation (tests excluded), split if >5 files (default)
- [ ] **Atomic** — 2-5 minute steps with checkpoints (for high-risk: auth, payments, migrations)

## Tasks

- [x] [Task 1](./tasks/TASK1-add-worktree-commit-age-data.md) — Add delimiter-safe commit age metadata to the gitexec/model layer without adding a new git command per worktree (deps: none)
- [x] [Task 2](./tasks/TASK2-wire-staleness-through-dashboard-state.md) — Populate the new timestamp in the dashboard loader while preserving runtime `LastActivityAt` reconciliation semantics (deps: Task 1)
- [x] [Task 3](./tasks/TASK3-render-dashboard-staleness-badge.md) — Replace the runtime activity text with a compact color-coded staleness badge and renderer tests (deps: Task 2)

## Coverage Matrix

| New Field/Endpoint | Added In | Code Paths Affected | Handled By | Converter Functions |
|--------------------|----------|---------------------|------------|---------------------|
| `WorktreeInfo.HeadCommittedAt` | Task 1 | git log parsing, dashboard loader, dashboard row render, pane tests | Task 2 (loader/state), Task 3 (render/tests) | `gitexec.CommitMeta()` parsing, `worktreeLoaderImpl.LoadWorktrees()` mapping |

**Validation:** The new timestamp is introduced once, mapped once in the loader, and consumed only in the renderer; runtime `LastActivityAt` remains a separate path.

## Dependency Graph

```text
Task 1 ---> Task 2 ---> Task 3
```

## Task Handoff State

| After Task | State |
|------------|-------|
| Task 1 | Git layer and model can represent HEAD commit age, but dashboard still renders runtime activity |
| Task 2 | Loader/refresh paths carry the new timestamp safely; runtime activity semantics still pass |
| Task 3 | Dashboard shows staleness badges end-to-end and renderer/test coverage is complete |

## External Dependencies

| Dependency | Status | Blocking |
|------------|--------|----------|
| Existing git CLI (`git log -1`) | Available | Task 1 |
| Existing ANSI theme tokens | Available | Task 3 |

## Plan Evaluation Record

PLAN_EVALUATION_VERDICT: PASS

Evidence:
- [x] Existing standards referenced with concrete paths
- [x] Data transformation points mapped
- [x] Tasks have explicit scope boundaries
- [x] Dependencies and verification commands listed per task
- [x] Requirements reconciled against source inputs
- [x] Whole-architecture coherence evaluated
- [x] UI/component tasks include design references

Source reconciliation: The Claude brief asked about merged-state detection; this plan intentionally defers it as a follow-up and implements commit-age staleness only.

## Definition of Done

- [ ] All task checkboxes complete
- [ ] All verification commands pass
- [ ] SPEC.md acceptance criteria satisfied
