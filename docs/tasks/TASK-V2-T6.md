# V2-T6: Commit Prompt Builder and Claude Provider

## Dependencies
V2-T1.

## Scope
- New `internal/commit/` package for provider abstraction, prompt construction, full diff snapshot collection, and Claude client.
- `go.mod` — add Anthropic SDK.

## Deliverables
- [x] `CommitMessageProvider` interface per spec.
- [x] Deterministic prompt builder combining **staged-only diff** (`git diff --cached`), file summaries, and commit-style instructions. Must not include unstaged changes — the generated message must match what `git commit` will actually record.
- [x] Guard: block commit generation (return typed error) when unstaged changes are present alongside staged changes, until explicit staging semantics are designed.
- [x] `ClaudeProvider` reading `ANTHROPIC_API_KEY` and optional model override.
- [x] Clear typed errors for missing API key, provider request failure, and malformed responses.

## Test Strategy
- Prompt-builder tests using fixture diffs and file summaries.
- Provider tests with HTTP/client mocking for success, auth failure, and rate-limit/network errors.
- Config/provider-factory tests for default provider/model selection.

## Out of Scope
- Any UI confirmation pane.
- Running `git commit`.
- Additional providers beyond Claude.

## Verification
```
go test ./internal/commit ./internal/config -run 'Test(Commit|Claude|Prompt)'
```
