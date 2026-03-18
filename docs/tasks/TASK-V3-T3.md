# V3-T3: Status Bar Redesign

## Dependencies
V3-T0, V3-T1.

## Scope
- Replace the single flat status bar with a segmented information strip.
- Segments: compare context | mode badges (W, C) | watch state (dot + interval + time) | file count.
- Dim `│` separators between segments.
- Mode badges highlighted when active, dim when off.
- Watch dot: green=watching, yellow=refreshing, red=error.
- Drill-down breadcrumb: `Dashboard > branch > file`.

## Files
- `internal/ui/model.go`

## Deliverables
- [ ] Status bar renders segmented layout with dim separators.
- [ ] Mode indicators (`W` for whitespace, `C` for commit) styled as badges.
- [ ] Watch indicator shows colored dot + interval + last check time.
- [ ] Breadcrumb appears during worktree drill-down.
- [ ] Refresh/error/search-not-found messages still occupy full bar width.
- [ ] Graceful truncation at narrow widths.

## Test Strategy
- Unit tests for status bar rendering in various states (watch on/off, drill-down, error).
- Width truncation tests at compact and minimal tiers.

## Out of Scope
- Spinner animation (V3-T13).

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
