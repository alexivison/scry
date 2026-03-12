# V2-T0: Working Tree Diff Mode

## Dependencies
None — this should be the first v0.2 task since it changes the default behavior.

## Scope
- `internal/source/resolve.go` — handle omitted head ref as working tree diff.
- `internal/model/model.go` — extend `ResolvedCompare` to represent working tree target.
- `internal/diff/metadata.go` — adjust diff commands when head is working tree.
- `internal/diff/patch.go` — adjust patch commands when head is working tree.
- `internal/ui/model.go` — status bar shows "(working tree)" when applicable.
- `cmd/scry/` — help text clarification.

## Deliverables
- [x] When `--head` is omitted, diff against working tree: `git diff <base>` (no head ref).
- [x] When `--head HEAD` is explicit, preserve v0.1 behavior (committed refs only).
- [x] `ResolvedCompare` gains a `WorkingTree bool` field (or `HeadRef` is empty to signal working tree mode).
- [x] Metadata commands: `git diff --name-status -z -M <base>` (no head ref).
- [x] Patch commands: `git diff --patch --no-color --no-ext-diff -M <base> -- <file>` (no head ref).
- [x] Status bar shows base ref + "(working tree)" instead of head SHA.
- [x] Refresh (`r`) re-reads working tree changes.
- [x] Help text updated.

## Test Strategy
- Compare resolver tests for omitted head vs explicit `HEAD`.
- Metadata/patch command tests verifying correct git args for working tree mode.
- Integration test with a fixture repo that has uncommitted changes.
- Regression tests for three-dot and two-dot committed-ref modes.

## Out of Scope
- Staged vs unstaged distinction (show both).
- Watch mode.
- Commit flow.

## Verification
```
go test ./internal/source ./internal/diff ./internal/ui ./cmd/scry -run 'Test(WorkingTree|Resolve|Metadata|Patch)'
go vet ./...
```
