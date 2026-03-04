# T4: Metadata Parser and Name-Status/Numstat Merge

## Dependencies
T2 (GitRunner), T3 (ResolvedCompare).

## Scope
- `internal/diff/metadata.go` — MetadataService implementation, NUL-delimited parsing, merge logic.

## Deliverables
- [ ] `MetadataService.ListFiles()` returning `[]FileSummary` in `--name-status -z` emission order.
- [ ] Parse `git diff --name-status -z -M <range>` for ordering, status, rename pairs.
- [ ] Parse `git diff --numstat -z -M <range>` for additions/deletions/binary markers.
- [ ] Merge by canonical key: `Path` for non-rename, `OldPath + "\x00" + Path` for rename/copy.
- [ ] Missing counts default to `0/0` with non-fatal debug warning.
- [ ] Binary detection from numstat (`-` values).
- [ ] All FileStatus variants handled (A/M/D/R/C/T/U).

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
go test ./internal/diff -run TestMetadataMerge
```
