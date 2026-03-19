# V3-T14: Idle and Commit Screen Polish

## Dependencies
V3-T0, V3-T1, V3-T2, V3-T13.

## Scope
- Style the idle screen: centered bordered box, pulsing watch indicator (`◉`/`○` alternating on tick), structured info layout.
- Style the commit screen: bordered text area for generated message, styled action key badges.

## Files
- `internal/ui/idle.go`
- `internal/ui/model.go`

## Deliverables
- [x] Idle view centered and bordered with `lipgloss.RoundedBorder()`.
- [x] Watch indicator pulses (alternates symbols on watch tick).
- [x] Idle shows: base ref, interval, last check time, status.
- [x] Commit view renders message in bordered area.
- [x] Action hints use styled key badges (contrasting background).
- [x] Existing commit actions (`Enter`, `e`, `r`, `Esc`) behavior unchanged.
- [x] Both screens adapt to responsive breakpoints.

## Test Strategy
- Render tests for idle screen at various sizes.
- Render tests for commit screen with generated message.
- Regression tests for commit action key handlers.

## Out of Scope
- Commit logic changes.
- Watch logic changes.

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
