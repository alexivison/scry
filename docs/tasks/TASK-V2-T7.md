# V2-T7: Commit Generation UI Flow

## Dependencies
V2-T1, V2-T6.

## Scope
- `internal/ui/` — keybinding `c`, in-flight state, confirmation pane/mode, regenerate/cancel behavior, help updates.
- `internal/model/` — commit UI state threading.

## Deliverables
- [ ] `c` triggers generation only when `--commit` is enabled.
- [ ] UI shows in-flight status, generated message, retry/regenerate/cancel affordances, and actionable errors.
- [ ] `Esc` cancels with no side effects.
- [ ] `r` regenerates using a fresh provider call.
- [ ] Help text and status bar expose commit mode appropriately.

## Test Strategy
- Model tests for key handling, in-flight states, cancel/regenerate flows, and error rendering.
- Provider stub tests proving no generation occurs when commit mode is disabled.

## Out of Scope
- Running `$EDITOR`.
- Executing `git commit`.

## Verification
```
go test ./internal/ui ./internal/commit -run 'TestCommitUI'
```
