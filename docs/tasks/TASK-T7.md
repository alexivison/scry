# T7: Patch Pane and Hunk Navigation

## Dependencies
T5 (PatchService), T6 (TUI shell).

## Deliverables
- [ ] Patch pane rendering unified diff with colored line types (context/added/deleted).
- [ ] Hunk headers displayed distinctly.
- [ ] `n` moves viewport to next hunk header. No-op at last hunk (no wrap).
- [ ] `p` moves viewport to previous hunk header. No-op at first hunk (no wrap).
- [ ] On file selection, viewport starts at the first hunk.
- [ ] `Enter` on file list loads patch synchronously (async deferred to T8) and switches focus to patch pane.
- [ ] `Escape` or `h` returns focus to file list pane.
- [ ] NoNewline marker rendered distinctly.

## Scope
- `internal/ui/panes/` — patch pane component.
- `internal/ui/` — key handling for n/p/Enter/Escape in patch context.

## Test Strategy
- Unit test hunk navigation logic with known FilePatch fixtures.
- Test boundary behavior: n at last hunk, p at first hunk.
- Test empty patch (no hunks).
- Test NoNewline line rendering.

## Out of Scope
- Async/lazy loading (T8).
- Search highlighting (T9).
- Edge-case fallback messages (T11).

## Verification
```
go test ./internal/ui -run TestHunkNavigation
```
