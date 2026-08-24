# Persistent Local Notes CLI Design

## Status

Approved for design review. This document covers the CLI milestone only. TUI
rendering and editing are deliberately deferred.

## Goal

Let a person and an agent leave durable, private notes on source locations in
one Scry worktree. Notes remain local to Scry, survive process restarts, and do
not depend on a Hunk session, Git metadata, a network service, or a repository
file.

## Scope

- Notes are scoped to one canonical worktree path.
- Each note is attached to a repository-relative file and current source line.
- Authors are exactly `user` or `agent`.
- States are exactly `open`, `resolved`, or `stale`.
- Every command returns JSON by default.
- An agent can use the CLI while Scry is closed.

## Out of Scope

- TUI note rendering, creation, editing, resolution, or deletion.
- GitHub review threads, synchronization, sharing, or cloud storage.
- Hunk session integration.
- Automatic resolution or automatic anchor relocation.
- Git-backed persistence, branch scoping, or comparison-basis scoping.

## Storage

Scry stores one ledger per canonical worktree beneath the user configuration
directory:

```text
<user-config-dir>/scry/notes/v1/<sha256(canonical-worktree-path)>.json
```

On macOS, `os.UserConfigDir` resolves beneath Application Support. The ledger
directory and file use owner-only permissions where supported. The canonical
worktree path is stored in the ledger and checked on every read so a hash alone
cannot select another worktree's notes.

The v1 ledger is intentionally small and personal. Scry opens only the active
worktree's ledger; it never scans notes belonging to other worktrees.

```json
{
  "version": 1,
  "worktree": "/absolute/canonical/worktree/path",
  "notes": [
    {
      "id": "0198…",
      "file": "internal/ui/model.go",
      "line": 412,
      "lineFingerprint": "sha256:…",
      "body": "Keep this guard simple.",
      "author": "agent",
      "state": "open",
      "createdAt": "2026-08-24T00:00:00Z",
      "updatedAt": "2026-08-24T00:00:00Z"
    }
  ]
}
```

`lineFingerprint` is the SHA-256 digest of the exact source line at the time
the note is anchored. It detects edits and line displacement without inferring
whether a change resolved the note.

## Commands

All commands operate on the current worktree by default. `--worktree <path>`
selects another local worktree after canonicalization.

```text
scry note list [--worktree <path>] [--state open|resolved|stale]
scry note add --file <repo-relative-path> --line <positive-int> --body <text> --author user|agent [--worktree <path>]
scry note edit <id> [--body <text>] [--file <repo-relative-path> --line <positive-int>] [--state open|resolved|stale] [--worktree <path>]
scry note remove <id> [--worktree <path>]
scry note sync [--worktree <path>]
```

`add` requires an explicit author. An author never changes after creation.
`edit` changes only the body, anchor, or state fields supplied by the caller.
Changing an anchor requires `--file` and `--line` together. Setting a valid
anchor through `edit` does not implicitly resolve a note; callers set
`--state open` when that is their intent.

`list` returns the worktree identity and the matching notes. `add`, `edit`,
`remove`, and `sync` return the affected note or transition summary. Success
documents have stable top-level object keys; failures write a JSON error object
to stderr and exit nonzero:

```json
{
  "error": {
    "code": "invalid_anchor",
    "message": "line 412 does not exist in internal/ui/model.go"
  }
}
```

## Anchor and State Semantics

Anchors identify source code, not a diff side. Scry never records whether a
line was added or removed in a particular comparison.

`sync` reads each `open` note's current file and source line:

- The path exists, the line exists, and its fingerprint matches: leave the
  note `open`.
- Any of those checks fails: change the note to `stale`.
- `resolved` notes are never changed by `sync`.
- `sync` never searches nearby lines or moves an anchor.

A person or agent decides whether a code change resolves a concern. They set
`resolved` explicitly. To repair a misplaced or stale note, an agent uses
`edit` with a new `--file` and `--line`, and may set `--state open` in the same
command.

## Data Integrity

Mutations take a short local lock before reading the ledger. They write a
temporary owner-only file in the ledger directory and atomically rename it
over the previous ledger. A competing mutation reports a JSON `busy` error;
it never overwrites a newer ledger. A malformed ledger reports
`corrupt_ledger` and remains unchanged.

File paths must be repository-relative, clean, and remain inside the selected
worktree after canonicalization. Bodies must be non-empty. IDs are opaque,
cryptographically random values generated with the Go standard library.

## Verification

Tests cover:

- JSON output and JSON error contracts for every command.
- Explicit `user` and `agent` authors and immutable authorship.
- Persistence across independent CLI processes.
- Worktree isolation, including canonicalized paths.
- `open` to `stale` transitions when a file, line, or line content changes.
- `resolved` notes remaining resolved through `sync`.
- Explicit repair through `edit` of body, anchor, and state.
- Atomic mutation behavior and contention returning `busy`.
- Rejecting traversal paths, invalid lines, invalid states, missing IDs, and
  corrupt ledgers without mutating valid notes.

The implementation must pass `go test ./...`, `go test -race ./...`,
`go vet ./...`, and `go build ./cmd/scry`.
