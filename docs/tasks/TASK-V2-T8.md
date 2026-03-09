# V2-T8: Commit Execution, Editor Handoff, and Post-Commit Refresh

## Dependencies
V2-T2, V2-T7.

## Scope
- `internal/commit/` — editor handoff helper and commit executor.
- `internal/ui/` — accept/edit/auto flows and post-commit success/error handling.
- `internal/app/` wiring for commit execution service.

## Deliverables
- [ ] `Enter` executes `git commit` with the generated or user-edited message.
- [ ] `e` opens `$EDITOR`, persists edits, and resumes commit flow.
- [ ] `--commit-auto` skips confirmation and commits immediately after generation.
- [ ] Commit result surfaces SHA on success and git stderr on failure.
- [ ] Successful commit reuses the shared refresh orchestrator and clears stale diff state.

## Test Strategy
- Executor tests with fake `GitRunner` covering success, nothing-to-commit, and hook rejection.
- Editor helper tests with temp files and fake editor command.
- UI tests for post-commit status updates and refresh calls.

## Out of Scope
- Amending commits.
- Staging files.
- Multi-commit queues.

## Verification
```
go test ./internal/commit ./internal/ui ./internal/app -run 'TestCommitExecution'
```
