# V3-T19: Export Flagged Files

## Dependencies
V3-T10.

## Scope
- `ctrl+e` exports the list of flagged file paths as newline-delimited text.
- Primary target: system clipboard (via `pbcopy` on macOS, `xclip`/`xsel` on Linux, `clip.exe` on WSL).
- Fallback: write to stdout on exit, or show error in status bar.
- Format: one path per line, plain text.

## Files
- `internal/ui/model.go`
- `internal/terminal/clipboard.go` (new — clipboard abstraction)

## Deliverables
- [x] `ctrl+e` copies flagged paths to clipboard.
- [x] Clipboard abstraction detects available tool and uses it.
- [x] Unsupported environments show clear status bar error, no crash.
- [x] Format is simple one-path-per-line, trivially pipeable.
- [x] Empty flagged set shows "No flagged files" in status bar.

## Test Strategy
- Unit test for clipboard command selection per platform.
- Test for empty flagged set handling.
- Integration test for export format.

## Out of Scope
- Exporting to file.
- Including line numbers or hunk ranges in export.

## Verification
```
go test ./internal/ui/... ./internal/terminal/... -count=1
go vet ./...
```

## Complexity
M
