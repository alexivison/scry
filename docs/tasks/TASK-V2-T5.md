# V2-T5: Idle Screen and Auto-Transition

## Dependencies
V2-T4.

## Scope
- `internal/ui/` — idle-state rendering, transition logic, status/help updates.
- `internal/model/` — explicit idle/view-mode state if needed.

## Deliverables
- [ ] Startup idle screen when `--watch` is enabled and the initial compare has no divergence.
- [ ] Idle view includes compare summary, watch interval, last check time, and key hints.
- [ ] Idle mode does not trigger patch loads.
- [ ] First detected divergence transitions automatically into the normal review view without restarting the program.

## Test Strategy
- Model tests for idle-at-launch, no-patch-load in idle mode, and one-way transition on divergence.
- Golden-ish string tests for idle view content.

## Out of Scope
- Returning to idle after review begins.
- Commit-generation UI.

## Verification
```
go test ./internal/ui ./internal/watch -run 'TestIdle'
```
