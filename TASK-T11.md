# T11: Edge-Case Hardening

## Dependencies
T5 (PatchService with sentinel errors), T8 (async loading).

## Scope
- `internal/diff/` — Binary/submodule detection hardening.
- `internal/ui/panes/` — Fallback rendering for edge cases.

## Deliverables
- [ ] Binary files: patch pane shows "Binary file — content not displayed" with file metadata (status, old/new path).
- [ ] Submodule changes: patch pane shows "Submodule change" with commit pointer info if available.
- [ ] Oversized patches (>50k lines or >8 MiB): patch pane shows metadata + "Patch too large to display (N lines, M bytes). Use `git diff -- <path>` to view."
- [ ] All three cases use sentinel errors from PatchService (ErrBinaryFile, ErrSubmodule, ErrOversized).
- [ ] UI checks `errors.Is()` on LoadPatch error to select fallback rendering.
- [ ] No panics on any edge case — defensive nil checks on Hunks.
- [ ] Empty diff (file in metadata but no patch content) handled gracefully.

## Test Strategy
- Mock PatchService returning each sentinel error with valid Summary.
- Test UI renders correct fallback message for each case.
- Test nil Hunks does not panic in patch pane.
- Test empty diff file.

## Out of Scope
- Syntax highlighting.
- Binary diff visualization.

## Verification
```
go test ./internal/diff ./internal/ui -run TestEdgeCases
```
