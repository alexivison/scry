# V3-T20: Optional Directory Grouping

## Dependencies
V3-T5, V3-T15.

## Scope
- Group file list rows by directory with dim directory headers.
- Opt-in via config (`filelist.group_by_directory = true`), disabled by default.
- When enabled, files grouped by directory prefix; directory header rows are non-selectable.
- When disabled, rendering unchanged from V3-T5.

## Files
- `internal/ui/panes/filelist.go`
- `internal/config/file.go`
- `internal/config/config.go`

## Deliverables
- [x] Directory grouping renders grouped headers when enabled.
- [x] Selection skips directory header rows.
- [x] Freshness markers (V3-T8) and flag markers (V3-T10) work correctly in grouped mode.
- [x] Feature disabled by default.
- [x] Ungrouped rendering identical to V3-T5 when disabled.

## Test Strategy
- Render tests for grouped and ungrouped modes.
- Selection navigation tests skipping header rows.
- Tests with freshness and flag markers in grouped mode.

## Out of Scope
- Collapsible directories.
- Custom grouping rules.

## Verification
```
go test ./internal/ui/panes/... ./internal/config/... -count=1
go vet ./...
```

## Complexity
M
