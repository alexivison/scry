# V3-T8: Freshness Indicators and Changed-File Jump

## Dependencies
V3-T5, V3-T7.

## Scope
- Track per-file "last changed" generation in `AppState` (parallel map: `FileChangeGen map[string]int`).
- On refresh, files whose summary changed get their generation set to current `CacheGeneration`.
- 3-tier visual freshness: hot (changed this refresh), warm (1–2 refreshes ago), cold (no marker).
- Jump-to-changed navigation key (see keybinding note below).
- Freshness markers render in the file list row prefix slot established by V3-T5.

## Keybinding
`]c` / `[c` for next/prev recently changed file (avoids collision with `gg`/`G` vim navigation from V3-T12).

## Files
- `internal/model/state.go`
- `internal/ui/model.go`
- `internal/ui/panes/filelist.go`
- `internal/review/refresh.go`

## Deliverables
- [ ] `FileChangeGen` map populated during refresh reconciliation.
- [ ] Hot/warm/cold markers render in file list (hot: bright marker, warm: dim marker, cold: none).
- [ ] Markers decay across subsequent refresh generations.
- [ ] `]c` / `[c` jump to next/prev hot or warm file, wrapping around.
- [ ] Normal selection/scroll behavior unaffected.

## Test Strategy
- Unit tests for generation tracking across multiple refreshes.
- Tests for tier decay (hot → warm → cold over 3 generations).
- Tests for jump navigation including wraparound and no-match cases.

## Out of Scope
- Scroll preservation (V3-T9).
- File flags (V3-T10).

## Verification
```
go test ./internal/ui/... ./internal/review/... -count=1
go vet ./...
```

## Complexity
M
