# V3-T6: Zero-Config Defaults

## Dependencies
None.

## Scope
- `scry` with no args enables watch mode by default.
- Multi-worktree repos (>1 entry from `git worktree list`) auto-enter dashboard mode.
- Single-worktree repos stay in diff mode.
- New opt-out flags: `--no-watch`, `--no-dashboard`.
- Old positive flags (`--watch`, `--worktrees`) remain temporarily for compatibility until V3-T15.

## Files
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/app/bootstrap.go`
- `cmd/scry/main.go` (help text)

## Deliverables
- [x] Watch mode defaults to on; `--no-watch` disables it.
- [x] Dashboard mode auto-detected from worktree count; `--no-dashboard` forces diff mode.
- [x] `--watch` and `--worktrees` still work (no breakage) but are effectively no-ops when defaults already match.
- [x] Help text reflects the new defaults.
- [x] Tests cover all flag combinations.

## Test Strategy
- Config parsing tests for new defaults and opt-out flags.
- Bootstrap test verifying auto-dashboard detection logic.

## Out of Scope
- Config file (V3-T15).
- Removing old flags (V3-T15).

## Verification
```
go test ./internal/config/... ./internal/app/... ./cmd/scry/... -count=1
go vet ./...
```

## Complexity
M
