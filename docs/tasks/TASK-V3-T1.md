# V3-T1: Responsive Breakpoints and Minimal Mode

## Dependencies
V3-T0.

## Scope
- Define explicit width and height tiers with graceful layout transitions.
- Add a new 40–59 col "minimal" tier that truncates paths, hides gutter line numbers, and abbreviates the status bar instead of showing the "too small" error.
- Lower the "too small" threshold from 80×24 to 40×15.

## Files
- `internal/terminal/terminal.go`
- `internal/ui/model.go`
- `internal/ui/panes/filelist.go`
- `internal/ui/panes/patch.go`

## Deliverables
- [ ] Width tiers: ≥120 wide split, 80–119 compact split, 60–79 modal-only, 40–59 minimal, <40 too small.
- [ ] Height tiers: ≥30 footer visible, 24–29 standard, 15–23 compact (reduced padding), <15 too small.
- [ ] Minimal mode suppresses gutter and truncates paths sanely.
- [ ] Breakpoint constants defined in `terminal.go` and referenced by UI.
- [ ] Tests cover layout mode selection at each tier boundary.

## Test Strategy
- Table-driven tests exercising `WindowSizeMsg` at boundary widths/heights.
- Verify layout mode, gutter visibility, and footer visibility per tier.

## Out of Scope
- Pane borders (V3-T2).
- Status bar redesign (V3-T3).

## Verification
```
go test ./internal/ui/... ./internal/terminal/... -count=1
go vet ./...
```

## Complexity
L
