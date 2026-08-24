# Persistent Notes CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add durable, private, JSON-first source-line notes for one local worktree, usable by people and agents while Scry is closed.

**Architecture:** Keep the feature outside the TUI and Git bootstrap path. A small `internal/notes` package owns the versioned per-worktree ledger, validation, locks, and lifecycle operations. `internal/notescli` parses `scry note` arguments and emits the stable JSON contract; `cmd/scry` only dispatches to it before the existing TUI configuration parser.

**Tech Stack:** Go 1.24.2+, standard library (`bufio`, `crypto/rand`, `crypto/sha256`, `encoding/json`, `os`, `path/filepath`, `syscall`, `time`); existing `spf13/pflag`.

**Spec:** `docs/superpowers/specs/2026-08-24-persistent-notes-cli-design.md`

## Global constraints

- Notes live only under `<user-config-dir>/scry/notes/v1`; never in Git, the repository, or a network service.
- The scope key is the canonical absolute directory supplied by `--worktree` or the current directory. Do not invoke Git to discover, validate, or persist note scope.
- Store only a clean repository-relative file path and a positive source line. Follow symlinks while validating an anchor so it cannot escape the selected worktree.
- `sync` uses the exact line-content SHA-256 fingerprint, transitions only `open` notes to `stale`, and never searches for a replacement anchor. `resolved` remains untouched.
- The ledger is intentionally one small, personal document per active worktree. Do not scan other ledgers or add indexing, pagination, sharing, or TUI code.
- Every normal command result is JSON on stdout. Every operational/validation failure is one JSON error object on stderr and a non-zero exit; `--help` may retain normal flag help.
- Release targets are Darwin and Linux. Use an advisory `flock` on those targets; provide a buildable unsupported-platform implementation that returns a structured error rather than weakening integrity.

## Stable command result shapes

```json
// scry note list
{"worktree":"/canonical/path","notes":[{"id":"…","file":"a.go","line":1,"lineFingerprint":"sha256:…","body":"…","author":"user","state":"open","createdAt":"…","updatedAt":"…"}]}

// scry note add, edit, remove
{"worktree":"/canonical/path","note":{"id":"…"}}

// scry note sync
{"worktree":"/canonical/path","checked":3,"staled":[{"id":"…"}]}

// all failures, on stderr
{"error":{"code":"invalid_anchor","message":"line 412 does not exist in internal/ui/model.go"}}
```

Use these error codes: `invalid_arguments`, `invalid_worktree`, `invalid_anchor`, `invalid_author`, `invalid_state`, `note_not_found`, `busy`, `corrupt_ledger`, `unsupported_platform`, and `storage`.

---

### Task 1: Add the local ledger, anchor validation, and safe persistence

**Files:**

- Create: `internal/notes/notes.go`
- Create: `internal/notes/store.go`
- Create: `internal/notes/lock_unix.go`
- Create: `internal/notes/lock_other.go`
- Create: `internal/notes/store_test.go`
- Create: `internal/notes/lock_unix_test.go`

- [ ] **Step 1: Write the failing package tests.**

  Cover a fresh ledger listing as empty without creating a file; add/list persistence after constructing a second store; both permitted authors; immutable author (the edit input has no author field); canonical-equivalent worktree paths sharing one ledger; different worktrees isolating ledgers; and exact rejection of empty bodies, invalid authors, non-positive/missing lines, absolute/traversal paths, symlink escapes, and nonexistent source lines. For every rejected mutation, assert the prior ledger bytes and list result are unchanged.

- [ ] **Step 2: Define the smallest exported domain surface in `internal/notes/notes.go`.**

  ```go
  type Author string
  const (
      AuthorUser  Author = "user"
      AuthorAgent Author = "agent"
  )

  type State string
  const (
      StateOpen     State = "open"
      StateResolved State = "resolved"
      StateStale    State = "stale"
  )

  type Note struct {
      ID              string    `json:"id"`
      File            string    `json:"file"`
      Line            int       `json:"line"`
      LineFingerprint string    `json:"lineFingerprint"`
      Body            string    `json:"body"`
      Author          Author    `json:"author"`
      State           State     `json:"state"`
      CreatedAt       time.Time `json:"createdAt"`
      UpdatedAt       time.Time `json:"updatedAt"`
  }
  type AddInput struct { File string; Line int; Body string; Author Author }
  type EditInput struct { Body *string; File *string; Line *int; State *State }
  type Store struct {
      worktree   string
      ledgerPath string
      lockPath   string
  }

  func NewStore(worktree, configDir string) (*Store, error)
  func (s *Store) List(filter *State) ([]Note, error)
  func (s *Store) Add(input AddInput) (Note, error)
  ```

  Include `Ledger` internally with `Version`, `Worktree`, and `Notes`; use `time.Time` for RFC 3339 JSON times. Generate opaque 128-bit hexadecimal IDs with `crypto/rand`; produce `sha256:<hex>` fingerprints with `crypto/sha256`.

- [ ] **Step 3: Implement scope, input validation, and fingerprinting in `internal/notes/store.go`.**

  Canonicalize a requested worktree with absolute path plus `filepath.EvalSymlinks`, require a directory, and derive the filename as `hex(sha256(canonical-worktree)) + ".json"` under `configDir/scry/notes/v1`. Require `filepath.IsLocal(file)`, normalize its stored representation with `filepath.ToSlash`, resolve the candidate path, then verify its canonical target remains under the canonical worktree using `filepath.Rel` plus `filepath.IsLocal`.

  Read source lines with a buffered stream and stop at the requested line; hash the line content as it streams instead of loading a potentially large source file. A missing file or target line is `invalid_anchor`. `Add` validates the input, fingerprints the exact source line, appends a note with UTC timestamps, and persists it.

- [ ] **Step 4: Implement one locked read-modify-write path.**

  `List` reads only this store's ledger. Missing ledger means an empty v1 ledger; malformed JSON, the wrong version, a mismatched worktree, or invalid stored records return `corrupt_ledger` without writing anything. Every mutation must:

  1. create the notes directory with `0700` permissions;
  2. acquire the per-ledger lock;
  3. load and validate the ledger;
  4. apply its one mutation;
  5. JSON-encode to a `0600` temporary file in the same directory, `Sync`, close, and atomically `Rename` it over the ledger.

  On Darwin/Linux, `lock_unix.go` uses non-blocking `syscall.Flock(LOCK_EX|LOCK_NB)` on a persistent `0600` sibling `.lock` file and maps lock contention to `busy`; closing the descriptor releases a crashed process's lock. `lock_other.go` returns `unsupported_platform`, so other targets build without offering unsafe multi-process writes.

- [ ] **Step 5: Run focused tests and commit.**

  Run `go test ./internal/notes`. Commit with `feat: add persistent note storage`.

### Task 2: Complete note lifecycle operations and stale synchronization

**Files:**

- Modify: `internal/notes/notes.go`
- Modify: `internal/notes/store.go`
- Modify: `internal/notes/store_test.go`

- [ ] **Step 1: Write failing lifecycle and contention tests.**

  Add tests for body-only edit, state-only edit, anchor edit requiring `file` and `line` together, and a repaired stale note becoming `open` only when the caller explicitly sets that state. Verify `remove` returns and removes the specified note, while a missing ID does not mutate the ledger. Test `sync` for changed content, a deleted file, and a deleted target line moving `open` to `stale`; matching notes stay `open`; resolved notes stay resolved. In the Darwin/Linux-tagged `lock_unix_test.go`, add a child-process helper that holds the actual ledger lock and assert a competing `Add` returns `busy` and leaves the ledger unchanged.

- [ ] **Step 2: Extend the store API and implement the operations through the existing mutation path.**

  ```go
  func (s *Store) Edit(id string, input EditInput) (Note, error)
  func (s *Store) Remove(id string) (Note, error)
  type SyncResult struct { Checked int; Staled []Note }
  func (s *Store) Sync() (SyncResult, error)
  ```

  `Edit` accepts only supplied body, anchor, and state fields; it never accepts or changes author. Every actual edit updates `updatedAt`; an anchor edit also recomputes its fingerprint. `Remove` returns the removed note. `Sync` counts examined `open` notes, updates the timestamp of each note it makes `stale`, and makes a single write only if one or more states change. It must not alter `resolved` notes, re-anchor a note, or infer resolution.

- [ ] **Step 3: Run focused tests and commit.**

  Run `go test ./internal/notes`. Commit with `feat: add note lifecycle commands`.

### Task 3: Add the JSON command interface and top-level dispatch

**Files:**

- Create: `internal/notescli/notescli.go`
- Create: `internal/notescli/notescli_test.go`
- Modify: `cmd/scry/main.go`
- Modify: `cmd/scry/main_test.go`

- [ ] **Step 1: Write failing command-contract tests.**

  Unit-test `notescli.Run` with a temporary worktree/config root and bytes buffers. Assert JSON success shapes for every command, JSON stderr error shapes for unknown commands/flags and each invalid domain value, the optional list state filter, and no non-JSON diagnostic leakage from `pflag`. Add an executable test in `cmd/scry/main_test.go`: build once, run `note add` and a separate `note list` child process with the same temporary `HOME`/`XDG_CONFIG_HOME`, and assert the second process reads the first note. Run child commands from two different worktree directories to prove default scopes do not mix.

- [ ] **Step 2: Implement `internal/notescli.Run` with explicit testable environment.**

  ```go
  type Options struct {
      WorkingDir string
      ConfigDir  string
      SetupErr   error
      Stdout     io.Writer
      Stderr     io.Writer
  }

  func Run(args []string, options Options) int
  ```

  With `pflag` flag sets whose output is discarded, implement exactly:

  ```text
  scry note list [--worktree <path>] [--state open|resolved|stale]
  scry note add --file <repo-relative-path> --line <positive-int> --body <text> --author user|agent [--worktree <path>]
  scry note edit <id> [--body <text>] [--file <repo-relative-path> --line <positive-int>] [--state open|resolved|stale] [--worktree <path>]
  scry note remove <id> [--worktree <path>]
  scry note sync [--worktree <path>]
  ```

  Select `options.WorkingDir` when `--worktree` is absent. Encode the result shapes defined above with `json.Encoder`; map package errors to the defined error codes and emit `{\"error\":{\"code\":…,\"message\":…}}` only to `options.Stderr`. Return exit code `2` for argument/validation errors and `1` for `busy`, ledger, storage, or platform errors.


- [ ] **Step 3: Dispatch before the existing interactive parser.**

  In `cmd/scry/main.go`, recognize a first positional argument of `note`, obtain `os.Getwd()` and `os.UserConfigDir()`, pass either failure as `Options.SetupErr`, and call the command package with `args[1:]`. `notescli.Run` maps `SetupErr` to the JSON `storage` error before parsing the command. Preserve existing `--version`, standard TUI flags, help, and non-TTY behavior for every non-`note` invocation.

- [ ] **Step 4: Run focused tests and commit.**

  Run `go test ./internal/notescli ./cmd/scry`. Commit with `feat: expose notes through the CLI`.

### Task 4: Document the CLI and repair the supported CI matrix

**Files:**

- Modify: `README.md`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add concise user documentation.**

  Add a `Persistent notes (CLI)` quick-start subsection after the existing quick-start examples. State that notes are private to Scry and the selected local worktree, stored outside the repository, JSON by default, and currently CLI-only. Include one add/list/edit/sync/remove sequence. Update the non-goal wording to distinguish unsupported GitHub review threads from these private local notes; do not promise TUI behavior.

- [ ] **Step 2: Align CI with the declared minimum supported Go version.**

  Replace the obsolete `1.22`/`1.23` matrix in `.github/workflows/ci.yml` with one `1.24.x` setup-go version. Keep the existing build, vet, normal-test, and race-test stages unchanged. The current `go.mod` declares `go 1.24.2`, so the existing matrix cannot build the repository.

- [ ] **Step 3: Run the required verification and commit.**

  Run:

  ```bash
  gofmt -w internal/notes/*.go internal/notescli/*.go cmd/scry/main.go cmd/scry/main_test.go
  go test ./...
  go test -race ./...
  go vet ./...
  go build ./cmd/scry
  GOOS=linux GOARCH=amd64 go build ./cmd/scry
  git diff --check
  ```

  Commit the documentation and CI correction with `docs: document persistent notes CLI`.

### Final review and handoff

- [ ] Review the complete branch against `origin/main` for the approved spec only: no TUI integration, no Hunk integration, no Git-backed note data, no scan of other worktrees, and no author edits.
- [ ] Run the configured Ponytail current-diff review; resolve any finding that compromises the minimal design.
- [ ] Push `persistent-notes-cli`, open a draft PR, and report its URL plus the verification output.
