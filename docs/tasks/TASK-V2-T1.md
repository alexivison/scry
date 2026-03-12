# V2-T1: CLI, Config, and State Scaffolding

## Dependencies
None.

## Scope
- `internal/config/` — add and validate v0.2 flags.
- `internal/model/` — extend `AppState` and add `CommitState` / watch-state fields.
- `cmd/scry/` — help output and parse-path tests.

## Deliverables
- [x] `--watch` and `--watch-interval` flags with default `2s` and minimum `500ms`.
- [x] `--commit`, `--commit-provider`, `--commit-model`, and `--commit-auto` flags.
- [x] Validation: `--watch-interval < 500ms` returns exit code 2.
- [x] Validation: `--commit-auto` requires `--commit`.
- [x] Validation: unsupported provider values fail fast.
- [x] `model.AppState` additions for watch/idle/commit state (no runtime behavior yet).
- [x] Help text documents all new flags.

## Test Strategy
- Table-driven config-parse tests for valid/default/invalid flag combinations.
- Model zero-value tests for new state fields.
- CLI tests for exit code 2 on invalid v0.2 flag combinations.

## Out of Scope
- Starting watch ticks.
- Calling Claude.
- Executing git commits.

## Verification
```
go test ./cmd/scry ./internal/config ./internal/model -run 'Test(Parse|Run|AppState)'
go vet ./...
```
