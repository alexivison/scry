# V3-T0: Theme Tokens and Style Centralization

## Dependencies
None.

## Scope
- New `internal/ui/theme/theme.go` — semantic ANSI color tokens mapped to roles (added, deleted, muted, accent, warning, error, hunk header, etc.).
- Replace scattered raw `lipgloss.Color("...")` literals in the file list, patch, dashboard, and top-level UI with theme references.
- No behavioral changes — pure refactor with identical visual output.

## Files
- `internal/ui/theme/theme.go` (new)
- `internal/ui/model.go`
- `internal/ui/panes/filelist.go`
- `internal/ui/panes/patch.go`
- `internal/ui/panes/dashboard.go`

## Deliverables
- [ ] Semantic theme roles exist for added/deleted/muted/accent/warning/error/hunk-header states.
- [ ] All hard-coded `lipgloss.Color("...")` usage removed from the above UI files (one documented fallback exception allowed for status bar background if needed).
- [ ] Existing rendering tests pass with no behavioral regressions.

## Test Strategy
- Existing test suite passes unchanged.
- Manual visual check that colors remain identical.

## Out of Scope
- Responsive breakpoints, borders, or layout changes.
- Custom theme configuration.

## Verification
```
go test ./internal/ui/... -count=1
go vet ./...
```

## Complexity
M
