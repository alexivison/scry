# T12: End-to-End Fixtures, Smoke Tests, Release Checklist

## Dependencies
T1-T11 (all prior tasks).

## Scope
- `testdata/repos/` — Fixture Git repositories.
- `testdata/golden/` — Golden output files for metadata and patch parity.
- `scripts/bench.sh` — Performance benchmark script.
- Integration tests across all packages.

## Deliverables
- [ ] Fixture repositories in `testdata/repos/`:
  - `simple` — basic add/modify/delete across a few files.
  - `rename` — file renames with content changes.
  - `binary` — binary file additions/modifications.
  - `submodule` — submodule pointer changes.
  - `large` — repo with a file exceeding 50k lines for oversize gate testing.
  - `linked-worktree` — fixture exercising linked worktree detection and correct RepoContext resolution.
- [ ] Golden tests verifying metadata and patch parity with raw `git diff` output.
- [ ] Linked worktree tests confirming identical diff output and correct GitDir/GitCommonDir distinction.
- [ ] `go test -race ./...` passes with no data races.
- [ ] `scripts/bench.sh`: measure file-list first-paint time on medium fixture (target <500ms).
- [ ] tmux smoke test: launch scry in tmux, verify resize handling without layout corruption.
- [ ] `--help` output verified to document all v0.1 flags.
- [ ] Clean exit (`q`) verified to restore terminal state.
- [ ] All exit codes verified: 0 (normal), 1 (runtime fatal after TUI start), 2 (bad flags), 128 (no repo).

## Release Checklist
- [ ] `go test ./...` passes.
- [ ] `go test -race ./...` passes.
- [ ] `go vet ./...` clean.
- [ ] `./scripts/bench.sh` first-paint under 500ms target.
- [ ] All MVP features F1-F8 (including F6a) satisfy acceptance criteria.
- [ ] No panics in any fixture or smoke test.
- [ ] Tag `v0.1.0`.

## Verification
```
go test ./... && go test -race ./... && ./scripts/bench.sh
```
