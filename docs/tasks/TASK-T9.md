# T9: Directional Search Index and UI Wiring

## Dependencies
T7 (patch pane).

## Scope
- `internal/search/` — Index implementation, smart-case matching.
- `internal/ui/` — Search input pane, `/` to enter search, `Enter`/`N` navigation, highlight matches.

## Deliverables
- [ ] `Index.Build(patch)` constructs searchable index from DiffLine text. Infallible.
- [ ] `Index.Find(query, fromLine, dir)` returns `(line, ok)`.
- [ ] Smart-case per-query: any uppercase rune → case-sensitive; all lowercase → case-insensitive.
- [ ] Plain substring matching (no regex).
- [ ] Forward search (`Enter`): starts from line after cursor, wraps around past end.
- [ ] Backward search (`N`): starts from line before cursor, wraps around past start.
- [ ] Empty query is a no-op.
- [ ] `/` enters search mode (PaneSearch): text input for query.
- [ ] `Enter` in search mode executes search and returns to patch pane.
- [ ] `Escape` in search mode cancels without searching.
- [ ] No-match state: status bar shows "Pattern not found: <query>".
- [ ] Match highlighting in patch pane (visual indication of matched text).

## Test Strategy
- Unit test Index with known line sets: forward, backward, wrap-around.
- Test smart-case: lowercase query matches mixed case; uppercase query is exact.
- Test empty query returns no match.
- Test single-match wrap-around (same match found after full cycle).

## Out of Scope
- Regex support (not in v0.1).
- Cross-file search.

## Verification
```
go test ./internal/search ./internal/ui -run TestDirectionalSearch
```
