# V3-T9: Scroll Preservation on Refresh

## Dependencies
V3-T7.

## Scope
- When the selected file survives a refresh and its patch content is truly unchanged, preserve the viewport scroll offset and current hunk position.
- Use exact patch content comparison (not just summary-level) for the selected file to catch same-count edits.
- If patch content changed or file disappeared, reset viewport sanely.

## Files
- `internal/ui/model.go`
- `internal/ui/panes/patch.go`
- `internal/review/refresh.go`

## Deliverables
- [ ] Selected file's cached patch is compared by content hash (not just additions/deletions) before preserving scroll.
- [ ] Unchanged patch: scroll offset, hunk position, and search state preserved.
- [ ] Changed patch: viewport resets to first hunk, search cleared.
- [ ] Removed file: selection reconciles to nearest neighbor, viewport resets.

## Design Note
V3-T7 does summary-level cache invalidation for all files. This task adds an exact content
comparison only for the currently selected file, keeping the cost bounded (one comparison per refresh).

## Test Strategy
- Test: refresh with unchanged selected file → scroll preserved.
- Test: refresh with changed selected file (same counts, different content) → scroll reset.
- Test: refresh with removed selected file → selection reconciled.

## Out of Scope
- Preserving scroll for non-selected files (only the visible patch matters).

## Verification
```
go test ./internal/ui/... ./internal/review/... -count=1
go vet ./...
```

## Complexity
M
