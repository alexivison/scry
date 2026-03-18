# V3-T12: Help Overlay and Page Navigation

## Dependencies
V3-T1, V3-T2.

## Scope
- Render help as a centered modal overlay (bordered, grouped sections) on top of the existing view with a dimmed background.
- Add vim-style page navigation keys.

## Navigation Keys
| Key | Action |
|-----|--------|
| `ctrl+d` / `ctrl+u` | Half-page down/up |
| `ctrl+f` / `ctrl+b` | Full page down/up |
| `gg` | Jump to top (first file / first line) |
| `G` | Jump to bottom (last file / last line) |
| `{` / `}` | Jump to prev/next hunk (alias for `p`/`n`) |

## `gg` Implementation
Two-key chord: first `g` sets a "pending g" flag, second `g` within 500ms executes jump-to-top. Any other key or timeout cancels the pending state.

## Files
- `internal/ui/model.go`
- `internal/ui/panes/patch.go`

## Deliverables
- [ ] Help renders as an overlay, not a full-screen replacement.
- [ ] Background dimmed when help is visible.
- [ ] Help text organized by section (Navigation, Search, Actions).
- [ ] All page navigation keys work in both file list and patch panes.
- [ ] `gg` chord implemented with timeout.
- [ ] `?` and `Esc` close the overlay.
- [ ] Help text reflects all currently bound keys.

## Test Strategy
- Overlay render test (overlay on top of content).
- Key handler tests for each new navigation key.
- `gg` chord test: `g` then `g` within timeout → top; `g` then `k` → cancel + normal `k`.

## Out of Scope
- Spinner animations (V3-T13).

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
