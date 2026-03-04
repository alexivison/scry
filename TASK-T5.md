# T5: Patch Parser/Loader Service and Domain Mapping

## Dependencies
T2 (GitRunner), T3 (ResolvedCompare).

## Scope
- `internal/diff/patch.go` — PatchService implementation, unified diff parsing via go-diff, domain mapping.
- `go.mod` — Add `github.com/sourcegraph/go-diff` dependency.

## Deliverables
- [ ] `PatchService.LoadPatch(ctx, cmp, filePath, ignoreWhitespace bool)` executing `git diff --patch --no-color --no-ext-diff -M <range> -- <file>`. Note: signature adds `ignoreWhitespace` parameter since whitespace state lives in `AppState`, not `ResolvedCompare`.
- [ ] Append `-w` to git diff command when `ignoreWhitespace` is true.
- [ ] Parse unified diff into `FilePatch` with `[]Hunk` and `[]DiffLine`.
- [ ] Map diff lines to `LineKind` constants (Context/Added/Deleted/NoNewline).
- [ ] Populate `OldNo`/`NewNo` line numbers on each DiffLine.
- [ ] Oversized patch gate: check raw byte count (>8 MiB) before parsing. Line count check (>50k) after parsing.
- [ ] Sentinel error returns: `ErrOversized` (valid Summary, nil Hunks), `ErrBinaryFile`, `ErrSubmodule`.
- [ ] Parse failures return wrapped error with empty FilePatch.

## Test Strategy
- Mock GitRunner returning known patch output.
- Golden tests: simple add/modify/delete, rename, context lines, no-newline-at-EOF.
- Test oversized gate (byte threshold).
- Test binary file detection.
- Test parse failure error wrapping.

## Out of Scope
- Metadata listing (T4).
- Async loading / caching (T8).
- UI rendering (T7).

## Verification
```
go test ./internal/diff -run TestLoadPatch
```
