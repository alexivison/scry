# T3: Source Resolver with Worktree-Safe Repo Context

## Dependencies
T2 (GitRunner).

## Scope
- `internal/source/` — CompareResolver implementation, RepoContext resolution, bootstrap sequence.
- `internal/model/` — may add helper constructors if needed.

## Deliverables
- [ ] Two-phase bootstrap: discovery runner at CWD → resolve RepoContext → repo-scoped runner at WorktreeRoot.
- [ ] `RepoContext` resolution via `rev-parse` commands (--show-toplevel, --absolute-git-dir, --git-common-dir).
- [ ] `IsLinkedWorktree` detection (GitDir != GitCommonDir after canonicalization).
- [ ] `CompareResolver.Resolve()` implementing three-dot (default) and two-dot modes.
- [ ] Default ref resolution: `--head` → HEAD, `--base` → `@{upstream}`, fail-fast if unresolvable.
- [ ] `MergeBase` populated in three-dot mode, empty string in two-dot mode.
- [ ] `DiffRange` formatted as `"base...head"` or `"base..head"`.
- [ ] Fatal errors (exit 128) for unresolvable refs, missing git, not-a-repo.

## Test Strategy
- Mock GitRunner for unit tests.
- Test both compare modes.
- Test upstream-missing error path.
- Test linked worktree detection (mock rev-parse output where GitDir != GitCommonDir).

## Out of Scope
- Metadata parsing (T4).
- Patch loading (T5).
- TUI (T6+).

## Verification
```
go test ./internal/source ./internal/model
```
