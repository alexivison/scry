# V3-T10: File Flags and Jump-Flagged

## Dependencies
V3-T5.

## Scope
- Session-scoped file bookmarks (do not persist across restarts).
- `m` toggles a flag on the selected file.
- Flagged files show a marker (`⚑` or `!`) in the file list prefix slot.
- `M` jumps to the next flagged file (wraps around).
- Flags survive refresh as long as the file path still exists; removed files lose their flag.

## Files
- `internal/model/state.go`
- `internal/ui/model.go`
- `internal/ui/panes/filelist.go`

## Deliverables
- [ ] `FlaggedFiles map[string]bool` (or `set`) in `AppState`.
- [ ] `m` toggles flag on selected file.
- [ ] Flag marker renders in file list.
- [ ] `M` cycles to next flagged file with wraparound.
- [ ] Flags pruned on refresh for files no longer in the list.
- [ ] Flags coexist with freshness markers (V3-T8) without collision.

## Test Strategy
- Toggle flag on/off.
- Jump-flagged with 0, 1, and multiple flagged files.
- Flag survival and pruning across refreshes.

## Out of Scope
- Exporting flagged files (V3-T19).
- Persisting flags across sessions.

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
