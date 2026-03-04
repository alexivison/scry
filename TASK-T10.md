# T10: Whitespace Toggle

## Dependencies
T8 (async loading, cache), T9a (cache-reset/refresh pipeline).

## Scope
- `internal/diff/` — Whitespace flag threading to patch loading command.
- `internal/ui/` — `W` keybinding, whitespace mode indicator.
- `internal/review/` — Reuse refresh pipeline from T9a.

## Deliverables
- [ ] `W` key toggles `AppState.IgnoreWhitespace`.
- [ ] Toggle triggers cache-reset: increment CacheGeneration, clear all patch cache, reload selected file patch. Unlike T9a manual refresh, whitespace toggle does NOT reload metadata (file list is unaffected by `-w`).
- [ ] Patch loading appends `-w` to git diff command when `IgnoreWhitespace` is true.
- [ ] Stale async responses from prior generation discarded.
- [ ] Whitespace mode indicator visible in status bar (e.g., `[W]` when active).
- [ ] `W` appears in key-help text.

## Test Strategy
- Test toggle flips IgnoreWhitespace and bumps generation.
- Test patch command includes `-w` when enabled, omits when disabled.
- Test cache is fully cleared on toggle.
- Test rapid toggle (W-W) produces consistent state.

## Out of Scope
- Per-file whitespace settings.
- Whitespace visualization modes.

## Verification
```
go test ./internal/diff ./internal/review ./internal/ui -run TestWhitespaceGeneration
```
