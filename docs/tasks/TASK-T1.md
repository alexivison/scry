# T1: Repository Bootstrap, CLI, Config Model

## Dependencies
None.

## Scope
- `cmd/scry/main.go` — wire CLI flags via pflag, parse into Config, exit codes 2/128.
- `internal/config/config.go` — Config type, flag registration, validation.
- `internal/model/` — Core domain types from spec (RepoContext, CompareMode, CompareRequest, ResolvedCompare, FileStatus, FileSummary, LineKind, DiffLine, Hunk, FilePatch, Pane, LoadStatus, PatchLoadState, AppState, sentinel errors).
- `go.mod` — Fix Go version to match CI matrix, add pflag dependency.

## Deliverables
- [ ] `Config` struct with pflag bindings for `--base`, `--head`, `--mode`, `--ignore-whitespace`.
- [ ] `--help` output documents all v0.1 flags.
- [ ] Exit code 2 on invalid flag usage.
- [ ] All core domain types from spec defined in `internal/model/`.
- [ ] Sentinel errors (`ErrOversized`, `ErrBinaryFile`, `ErrSubmodule`) in model package.
- [ ] `go.mod` version aligned with CI matrix.

## Out of Scope
- Git command execution (T2).
- Source/ref resolution (T3).
- Any TUI rendering (T6).

## Verification
```
go build ./cmd/scry
go test ./cmd/scry ./internal/config ./internal/model
go vet ./...
```
