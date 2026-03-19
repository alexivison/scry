# V3-T13: Loading Spinners

## Dependencies
V3-T3.

## Scope
- Integrate `charmbracelet/bubbles/spinner` for animated loading states.
- Spinner contexts: patch loading, commit generation/execution, watch refresh (status bar), initial load.
- All spinners driven through Bubble Tea message loop — no stray goroutines.

## Files
- `go.mod` (add `bubbles` spinner if not present)
- `internal/ui/model.go`

## Deliverables
- [x] `bubbles/spinner` dependency added.
- [x] Patch loading shows spinner in patch pane area.
- [x] Commit generation/execution shows spinner with descriptive text.
- [x] Watch refresh shows subtle spinner in status bar watch segment.
- [x] Initial load shows centered spinner.
- [x] Spinner state cleaned up on async completion or cancellation.
- [x] No spinner leak after quit.

## Test Strategy
- Verify spinner init and tick messages flow correctly.
- Test spinner cleanup on load completion and cancellation.

## Out of Scope
- Idle screen pulsing (V3-T14).

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
