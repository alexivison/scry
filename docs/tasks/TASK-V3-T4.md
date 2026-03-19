# V3-T4: Diff Renderer Polish

## Dependencies
V3-T0, V3-T1, V3-T2.

## Scope
- Styled gutter: dim line numbers with `│` separator between gutter and content.
- Hunk separators: horizontal rule (`───`) with hunk header text between hunks.
- Scroll position indicator: highlighted segment on the right border edge.

## Files
- `internal/ui/panes/patch.go`
- `internal/ui/model.go`

## Deliverables
- [x] Gutter uses `theme.Muted` with a thin separator column.
- [x] Hunks separated by styled horizontal rules containing the `@@` header.
- [x] Scroll indicator visible as a highlighted border segment mapping to `scrollOffset / totalLines`.
- [x] All rendering adapts to minimal mode (no gutter when width < 60).
- [x] Overflowing patches don't break narrow layouts.

## Test Strategy
- Render tests for gutter format, hunk separators, and scroll indicator position.
- Tests at narrow widths verifying gutter suppression.

## Out of Scope
- Background tinting for added/deleted lines (deferred — terminal theme compatibility concern).

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
