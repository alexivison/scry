# T2: GitExec Runner with Timeout

## Dependencies
T1 (project scaffold, model types).

## Scope
- `internal/gitexec/` — GitRunner implementation, GitRunnerConfig, structured errors.

## Deliverables
- [x] `GitRunner` interface implementation with `RunGit(ctx, args...)`.
- [x] `GitRunnerConfig` with `WorkDir` and `Timeout` (default 30s).
- [x] Structured error type wrapping stderr, exit code, and command args.
- [x] Context cancellation and timeout enforcement.
- [x] Constructor: `NewGitRunner(cfg GitRunnerConfig) GitRunner`.

## Design Notes
- Runner is the sole subprocess boundary. No other package may exec git.
- WorkDir is fixed at construction. Phase 1 (discovery) uses CWD; phase 2 (repo-scoped) uses WorktreeRoot.
- Timeout applies per-command via `context.WithTimeout`.

## Out of Scope
- Repo context resolution (T3).
- Any diff/patch parsing (T4/T5).

## Verification
```
go test ./internal/gitexec
```
