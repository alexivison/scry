# V3-T15: Config File and Final CLI Pruning

## Dependencies
V3-T6.

## Scope
- Introduce `~/.config/scry/config.toml` (user) and `.scry.toml` (repo) config files.
- Precedence: CLI flag > repo `.scry.toml` > user `~/.config/scry/config.toml` > built-in defaults.
- Move stable knobs to config-only: `diff.mode`, `diff.ignore_whitespace`, `watch.interval`, `commit.provider`, `commit.model`.
- Remove deprecated positive flags (`--watch`, `--worktrees`, `--mode`, `--watch-interval`, `--ignore-whitespace`, `--commit-provider`, `--commit-model`).
- Final CLI surface: `--base`, `--head`, `--commit`, `--commit-auto`, `--no-watch`, `--no-dashboard`.

## Files
- `internal/config/file.go` (new — TOML parsing + precedence merge)
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/scry/main.go`

## Deliverables
- [ ] TOML config loaded from both paths with correct precedence.
- [ ] Config values merged: CLI > repo > user > defaults.
- [ ] Deprecated flags removed.
- [ ] Final CLI surface matches plan.
- [ ] Missing config files are silently ignored.
- [ ] Malformed config files produce clear errors.

## Test Strategy
- Precedence tests with all layers present/absent.
- Parse tests for valid and malformed TOML.
- CLI integration tests for final flag surface.

## Out of Scope
- Config UI or `scry config` command.

## Verification
```
go test ./internal/config/... ./cmd/scry/... -count=1
go vet ./...
```

## Complexity
L
