# V3-T5: File List Visual Polish

## Dependencies
V3-T0, V3-T1.

## Scope
- Colored status icons: `A` green, `D` red, `M` yellow, `R` cyan.
- Colored counts: `+N` green, `-N` red.
- Selection: reverse video only (drop bold).
- Row layout designed to host freshness markers and flag markers in later tasks without rewrite.

## Files
- `internal/ui/panes/filelist.go`

## Deliverables
- [ ] Status letters render in semantic theme colors.
- [ ] `+/-` counts render in green/red respectively.
- [ ] Selection uses `Reverse(true)` without bold.
- [ ] Row truncation still works for long paths and renames.
- [ ] Row format has a stable slot for prefix markers (to be used by V3-T8 and V3-T10).

## Test Strategy
- Render tests for each file status type with expected color styling.
- Truncation tests for long paths at various widths.

## Out of Scope
- Directory grouping (V3-T20).
- Freshness markers (V3-T8).
- Flag markers (V3-T10).

## Verification
```
go test ./internal/ui/panes/... -count=1
go vet ./...
```

## Complexity
M
