# T6: Bubble Tea Shell with Bootstrap Render

## Dependencies
T1 (CLI/Config), T3 (ResolvedCompare), T4 (MetadataService/file list).

## Scope
- `internal/ui/` — Bubble Tea model, shell layout, file list pane, status bar.
- `internal/app/` — Bootstrap wiring (Config → ResolvedCompare → file list → TUI).
- `internal/terminal/` — TTY detection, dimension validation (80x24 minimum), color capability.
- `go.mod` — Add Bubble Tea, Lipgloss, Bubbles dependencies.

## Deliverables
- [ ] Bubble Tea `Model` implementing `Init()`, `Update()`, `View()`.
- [ ] File list pane displaying `[]FileSummary` with status icons and add/delete counts.
- [ ] `j/k` navigation in file list, `Enter` to select file (switches to patch pane).
- [ ] Status bar showing active compare range (base...head or base..head).
- [ ] `q` exits cleanly, restores terminal state (exit code 0).
- [ ] `?` toggles help overlay. Initially shows keys available at T6 (`j/k`, `Enter`, `q`, `?`). Later tasks (T7/T9/T9a/T10) extend the help text with their own keybindings.
- [ ] TTY check: fail fast with actionable message if not a terminal.
- [ ] Dimension check: reject <80x24 with guidance message.
- [ ] Color capability detection: respect `NO_COLOR`, `COLORTERM`, terminfo fallback. Degrade styles gracefully.
- [ ] tmux detection and resize event handling without layout corruption.
- [ ] `AppState` initialized with `SelectedFile = -1` when Files is empty, `0` otherwise.
- [ ] App bootstrap in `internal/app/`: Config → phase 1 runner → RepoContext → phase 2 runner → resolve compare → list files → launch TUI.

## Out of Scope
- Patch rendering (T7).
- Async loading (T8).
- Search (T9).

## Verification
```
go test ./internal/ui -run TestShellRender
go test ./internal/app ./internal/terminal
```
