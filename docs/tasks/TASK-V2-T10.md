# V2-T10: Split-Pane Layout with Vim Navigation

## Dependencies
V2-T0 (working tree diff mode should land first so the split view renders working tree diffs correctly).

## Motivation
Currently scry uses a modal, full-screen navigation where only one pane is visible at a time (file list OR patch). This task adds a split-pane layout with the file explorer on the left and the diff on the right, plus vim-style `h`/`l` navigation between panes.

## Scope
- `internal/model/state.go` — add `LayoutSplit` mode enum, active-pane tracking for split view.
- `internal/ui/model.go` — `View()` renders side-by-side layout when in split mode; `Update()` routes keys to active pane.
- `internal/ui/panes/filelist.go` (new) — extract file list rendering into a reusable pane with fixed width and scroll support.
- `internal/ui/panes/patch.go` — adjust width to fill remaining space in split mode.
- `internal/ui/model.go` keybindings — `l` (or `Enter`) from file list focuses patch pane; `h` from patch pane focuses file list.
- `internal/ui/model.go` — `Tab` toggles layout mode (split ↔ full-screen modal).
- `internal/ui/styles.go` — border/separator styles for the split divider.
- `cmd/scry/` — help text updated with new keybindings.

## Deliverables
- [ ] Split-pane view: file list on left (fixed width, ~30% of terminal or min 25 cols), diff on right (remaining width).
- [ ] Active pane indicated visually (e.g. highlighted border or dimmed inactive pane).
- [ ] `l` from file list → focus patch pane (same as `Enter` but without toggling layout).
- [ ] `h` from patch pane → focus file list (already works in modal mode, must also work in split mode).
- [ ] `Enter` from file list in split mode → load patch for selected file, focus patch pane.
- [ ] `j`/`k` in file list scrolls through files; in split mode, patch auto-updates to show selected file's diff.
- [ ] `Tab` toggles between split layout and full-screen modal layout (persists for session).
- [ ] File list in split mode supports scrolling when files exceed viewport height.
- [ ] Graceful degradation: if terminal width < 80 cols, fall back to modal layout with a status message.
- [ ] Help overlay (`?`) updated with new keybindings.

## Key Bindings Summary

### File List Pane (split mode)
| Key | Action |
|-----|--------|
| `j`/`k` | Navigate files (patch auto-updates) |
| `l`/`Enter` | Focus patch pane |
| `Tab` | Toggle to full-screen modal |
| `q` | Quit |
| `?` | Help |

### Patch Pane (split mode)
| Key | Action |
|-----|--------|
| `j`/`k` | Scroll diff |
| `h` | Focus file list |
| `n`/`p` | Next/prev hunk |
| `/` | Search |
| `Tab` | Toggle to full-screen modal |

### Full-Screen Modal (existing, unchanged)
| Key | Action |
|-----|--------|
| `l`/`Enter` | Open patch (from file list) |
| `h`/`Esc` | Back to file list (from patch) |

## Design Notes
- In split mode, navigating files with `j`/`k` should auto-load the patch for the newly selected file (lazy, with loading indicator).
- The divider between panes should be a single vertical line (`│`).
- Active pane gets a brighter border; inactive pane is dimmed.
- File list width: `max(25, min(termWidth * 0.3, 50))` — responsive but bounded.

## Test Strategy
- Model tests for layout toggle (`Tab`), pane focus switching (`h`/`l`).
- View rendering tests: split layout produces two-column output with divider.
- `j`/`k` in split mode triggers patch load for newly selected file.
- Graceful fallback test: width < 80 → modal layout.
- `l` from file list behaves like `Enter` (loads patch + focuses).
- Existing modal navigation tests remain passing (regression).

## Out of Scope
- Resizable split ratio (drag to resize).
- Tree view / directory grouping in file list.
- Side-by-side diff (old vs new columns within the patch pane).

## Verification
```
go test ./internal/ui ./internal/model -run 'Test(Split|Layout|VimNav|PaneFocus)'
go vet ./...
```
