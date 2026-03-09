# V2-T6: Commit Prompt Builder and Claude Provider

## Dependencies
V2-T1.

## Scope
- New `internal/commit/` package for provider abstraction, prompt construction, full diff snapshot collection, and Claude client.
- `go.mod` — add Anthropic SDK.

## Deliverables
- [ ] `CommitMessageProvider` interface per spec.
- [ ] Deterministic prompt builder combining full unified diff, file summaries, and commit-style instructions.
- [ ] `ClaudeProvider` reading `ANTHROPIC_API_KEY` and optional model override.
- [ ] Clear typed errors for missing API key, provider request failure, and malformed responses.

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
