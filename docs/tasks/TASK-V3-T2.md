# V3-T2: Pane Borders, Titles, and Footers

## Dependencies
V3-T0, V3-T1.

## Scope
- Wrap file list and patch panes in lipgloss borders.
- Active pane border uses `theme.Accent`; inactive uses `theme.Muted`.
- File list title: "Files"; patch title: current filename.
- File list footer: file count; patch footer: hunk position + scroll % (when height permits per V3-T1 tiers).
- Borders adapt to layout mode (split, modal, minimal).

## Files
- `internal/ui/model.go`
- `internal/ui/panes/filelist.go`
- `internal/ui/panes/patch.go`
- `internal/ui/panes/dashboard.go`

## Deliverables
- [x] Split and modal views render clear pane boundaries with `lipgloss.RoundedBorder()`.
- [x] Focused pane border is visually distinct from inactive panes.
- [x] File list footer shows file count; patch footer shows hunk N/M and scroll %.
- [x] Footers hidden when height < 30 rows (compact tier).
- [x] Dashboard pane also has consistent border styling.

## Test Strategy
- Render tests for bordered output at various dimensions.
- Verify footer content matches expected hunk/scroll state.

## Out of Scope
- Diff rendering changes (V3-T4).
- Status bar (V3-T3).

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
