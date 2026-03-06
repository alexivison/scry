# T4: Metadata Parser and Name-Status/Numstat Merge

## Dependencies
T2 (GitRunner), T3 (ResolvedCompare).

## Scope
- `internal/diff/metadata.go` — MetadataService implementation, NUL-delimited parsing, merge logic.

## Deliverables
- [x] `MetadataService.ListFiles()` returning `[]FileSummary` in `--name-status -z` emission order.
- [x] Parse `git diff --name-status -z -M <range>` for ordering, status, rename pairs.
- [x] Parse `git diff --numstat -z -M <range>` for additions/deletions/binary markers.
- [x] Merge by canonical key: `Path` for non-rename, `OldPath + "\x00" + Path` for rename/copy.
- [x] Missing counts default to `0/0` with non-fatal debug warning.
- [x] Binary detection from numstat (`-` values).
- [x] All FileStatus variants handled (A/M/D/R/C/T/U).

## Test Strategy
- Mock GitRunner returning known NUL-delimited output.
- Golden tests: simple diff, renames, copies, binary files, mixed.
- Test missing-stats warning path.
- Test empty diff (no files).

## Out of Scope
- Patch content parsing (T5).
- Submodule-specific handling (T11).

## Verification
```
go test ./internal/diff -count=1
```
