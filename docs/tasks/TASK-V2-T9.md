# V2-T9: v0.2 Fixtures, Docs, and Smoke Coverage

## Dependencies
V2-T5, V2-T8.

## Scope
- `testdata/` — watch/idle/commit fixtures and provider stubs.
- Docs: `README.md`, `CONTRIBUTING.md`, `docs/integrations/tmux-session.md`.
- End-to-end smoke coverage for watch mode, idle transition, and commit flow.

## Deliverables
- [ ] Fixture coverage: no-divergence watch startup, divergence after fingerprint change, commit-generation prompt fixtures, commit execution success/failure, linked-worktree watch behavior.
- [ ] README/CONTRIBUTING updated for new flags, API-key requirements, and minimum Go version.
- [ ] tmux integration doc updated from planned to supported.
- [ ] End-to-end verification checklist.

## Test Strategy
- Full-package tests across watch, ui, commit, and app.
- Race-detector run.
- Smoke command docs using temp repos and provider stubs.

## Out of Scope
- Release automation.
- Benchmarks beyond verifying existing responsiveness is not regressed.

## Verification
```
go test ./...
go test -race ./...
go vet ./...
```
