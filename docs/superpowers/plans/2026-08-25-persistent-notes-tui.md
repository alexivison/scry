# Persistent Notes TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Scry-themed TUI reader and editor for the existing durable per-worktree notes ledger.

**Architecture:** Keep `internal/notes.Store` as the only persistence boundary. The Bubble Tea model loads and mutates one active worktree ledger asynchronously, while `PatchViewport` owns inline card geometry and the file-list projection adds one non-Git stale-notes row. Reuse the installed Bubbles textarea and Scry's existing suspended `$EDITOR`, confirmation, status, and refresh patterns.

**Tech Stack:** Go 1.24.2+, Bubble Tea 1.3.10, Bubbles 1.0.0 textarea, Lip Gloss, existing `internal/notes` package.

**Spec:** `docs/superpowers/specs/2026-08-25-persistent-notes-tui-design.md`

## Global Constraints

- Notes remain private, local, and scoped to one canonical worktree ledger under the user configuration directory.
- Use the existing concrete `*notes.Store`; do not add another storage layer, repository file, Git integration, network service, or one-implementation interface.
- TUI creation always writes author `user`; TUI editing changes only `body`; TUI resolution changes only `state` to `resolved`.
- The TUI never changes anchors, reopens resolved notes, or resolves stale notes.
- Open cards use Scry's current theme tokens and appear after current-source lines without covering line-number gutters. Resolved and stale cards appear in the bottom Notes view.
- `{` and `}` replace their existing hunk aliases; `n` and `p` remain hunk navigation.
- `c` creates, `E` edits, `R` resolves, and `D` requests confirmed deletion. Composer controls are Enter for a newline, `Alt+Enter` to submit, `Ctrl+G`, and Esc.
- Note I/O must run in `tea.Cmd` functions. A note failure must not block diff review or discard the last good snapshot/draft.
- Do not add a note watcher, dashboard note counts, a note theme, timestamps, replies, or new dependencies.

---

### Task 1: Wire one concrete note store through diff and worktree modes

**Files:**

- Create: `internal/ui/notes.go`
- Create: `internal/ui/notes_test.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/dashboard.go`
- Modify: `internal/ui/dashboard_test.go`
- Modify: `internal/app/bootstrap.go`
- Modify: `internal/app/bootstrap_test.go`

**Interfaces:**

- Consumes: `notes.NewStore(worktree, configDir)`, `(*notes.Store).Sync()`, and `(*notes.Store).List(nil)`.
- Produces: `WithNoteStore(store *notes.Store, setupErr error) ModelOption`; `DrillDownResult.NoteStore *notes.Store`; `DrillDownResult.NoteErr error`; private `noteUIState`; asynchronous load messages used by later tasks.

- [ ] **Step 1: Write failing store-wiring and asynchronous-load tests.**

  In `internal/ui/notes_test.go`, create real stores with temporary worktree and config directories. Assert that `Init` schedules sync/list, a completed load installs the snapshot, a corrupt ledger keeps the previous snapshot and records an error, and a later successful load clears that error. In `dashboard_test.go`, assert a completed drill-down installs the result's store and leaving drill-down clears note state. In `bootstrap_test.go`, assert store creation uses the supplied worktree and config directory without touching Git.

- [ ] **Step 2: Run the focused tests and confirm the new behavior is absent.**

  Run:

  ```bash
  go test ./internal/ui ./internal/app -run 'TestNote|TestDrillDown.*Note'
  ```

  Expected: FAIL because the note model option, state, messages, and drill-down fields do not exist.

- [ ] **Step 3: Add the minimal UI-owned note state and load command.**

  Keep the state private in `internal/ui/notes.go`:

  ```go
  type noteUIState struct {
      store      *notes.Store
      items      []notes.Note
      selectedID string
      setupErr   error
      err        string
      generation int
      loading    bool
  }

  type notesLoadedMsg struct {
      notes      []notes.Note
      generation int
      err        error
  }

  func WithNoteStore(store *notes.Store, setupErr error) ModelOption
  func (m Model) loadNotes() tea.Cmd
  func (m Model) handleNotesLoaded(msg notesLoadedMsg) (tea.Model, tea.Cmd)
  ```

  `loadNotes` captures the store and generation, calls `Sync` and then `List(nil)` inside the command, and never clears `items` before success. Add the command to `Init` only when a store is available. Include note loading in `needsSpinner`.

- [ ] **Step 4: Supply stores from application bootstrap and drill-down.**

  Resolve `os.UserConfigDir()` once per launched TUI. In normal diff mode, build the store for `boot.Repo.WorktreeRoot` and pass both result and error through `WithNoteStore`. Give `drillDownProviderImpl` the config directory/setup error and return these fields without failing the diff load:

  ```go
  type DrillDownResult struct {
      Compare       model.ResolvedCompare
      Files         []model.FileSummary
      PatchLoader   PatchLoader
      FileDiscarder FileDiscarder
      NoteStore     *notes.Store
      NoteErr       error
  }
  ```

  A note setup error disables note commands and surfaces through note status; it does not abort `runDiff` or `LoadDrillDown`.

- [ ] **Step 5: Batch notes into existing refresh paths.**

  Normal `r` returns `tea.Batch(existingRefreshCmd, m.loadNotes())`. Drill-down refresh receives a fresh store in `DrillDownResult`, installs it, and starts its load. Leaving drill-down resets `noteUIState{}`. Do not load any ledger while the top-level dashboard is active.

- [ ] **Step 6: Run focused tests and commit.**

  Run `go test ./internal/ui ./internal/app`. Commit with `feat(ui): load worktree notes`.

### Task 2: Render Scry-themed note cards and the stale-notes file row

**Files:**

- Create: `internal/ui/panes/notes.go`
- Create: `internal/ui/panes/notes_test.go`
- Modify: `internal/ui/panes/patch.go`
- Modify: `internal/ui/panes/patch_render_test.go`
- Modify: `internal/ui/panes/filelist.go`
- Modify: `internal/ui/panes/filelist_test.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/notes.go`

**Interfaces:**

- Consumes: the loaded `[]notes.Note` snapshot from Task 1.
- Produces: `FileTreeRowNotes`; stale-count-aware file projection; `PatchViewport.SetNotes`; `PatchViewport.ScrollToNote`; shared card-row rendering for patches and the stale-notes view.

- [ ] **Step 1: Write failing pure rendering and projection tests.**

  Cover one open note after the final wrapped row of its matching `NewNo`, several cards on one line in creation/ID order, no card for an old-only row, gutter-aligned cards in both diff modes, resolved-note removal from inline diffs, and card height in `TotalLines`. Add file-list tests for a final `Notes (N)` row, including a worktree with zero changed files, and assert its `FileIndex` remains `-1`.

- [ ] **Step 2: Run the pane tests and verify they fail.**

  Run:

  ```bash
  go test ./internal/ui/panes -run 'Test.*Note|Test.*Stale'
  ```

  Expected: FAIL because note rows and the stale file-list row are not represented.

- [ ] **Step 3: Add a non-Git stale-notes row to the existing projection.**

  Extend the existing types instead of creating another file list:

  ```go
  const (
      FileTreeRowDir FileTreeRowKind = iota
      FileTreeRowFile
      FileTreeRowNotes
  )

  type FileListOpts struct {
      // existing fields
      NoteCount int
  }

  func ProjectFileTree(files []model.FileSummary, filter model.FileFilter, collapsed map[string]bool, cursor, staleNotes int) FileTreeProjection
  ```

  Append the notes row after every real row, render it with Scry's muted/error semantics, and allow it to be selected when the real file slice is empty. Update all projection call sites with `0` until the model supplies its stale count. Never add a sentinel `FileSummary`.

- [ ] **Step 4: Add shared note-card rows and integrate them into patch geometry.**

  In `panes/notes.go`, keep card presentation data small and reuse `notes.Note`:

  ```go
  type NoteDraftView struct {
      NoteID string
      File   string
      Line   int
      Body   string
  }

  func RenderNoteList(items []notes.Note, selectedID string, draft *NoteDraftView, width, height, offset int) string
  func NoteListOffset(items []notes.Note, selectedID string, width int) (int, bool)
  ```

  Add note data to `PatchViewport` through:

  ```go
  func (vp *PatchViewport) SetNotes(items []notes.Note, selectedID string, draft *NoteDraftView)
  func (vp *PatchViewport) ScrollToNote(id string) bool
  func (vp *PatchViewport) CurrentSourceLine() (int, bool)
  ```

  Represent each rendered card line as a `visualRow` variant. After unified or side-by-side base rows are built, insert card rows only after the last wrapped row for a matching current-source line. Track the current-source cursor by logical diff index so folder patches with repeated line numbers remain unambiguous. `CurrentSourceLine` returns the selected cursor line.

- [ ] **Step 5: Feed selected-file notes and stale count from the model.**

  Filter attached cards by selected real file and state `open`. Use resolved and stale items for the virtual view. Add the inactive count to `fileListOpts` and every model projection call. When the current row is `FileTreeRowNotes`, render `RenderNoteList` and bypass patch loading, source editing, discard, commit targeting, and `c`.

- [ ] **Step 6: Run focused tests and commit.**

  Run `go test ./internal/ui/panes ./internal/ui`. Commit with `feat(ui): render persistent note cards`.

### Task 3: Add note navigation, composer, direct actions, and `$EDITOR`

**Files:**

- Modify: `internal/ui/notes.go`
- Modify: `internal/ui/notes_test.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/editor.go`
- Modify: `internal/ui/editor_test.go`

**Interfaces:**

- Consumes: Task 1 note state/store and Task 2 card/viewport APIs.
- Produces: direct `c`, `j`/`k`, `{`, `}`, `E`, `R`, and `D` behavior; inline Bubbles textarea; confirmed deletion; mutation/editor completion messages.

- [ ] **Step 1: Write failing interaction tests before production handlers.**

  Test that `j`/`k` traverses source rows and inline cards, while `{`/`}` selects notes in file/source/creation/ID order across real-file boundaries and the Notes view. Test `c` only on `CurrentSourceLine`; `E` copies only the selected body into the composer; `R` submits only `StateResolved`; and `D` requires confirmation before removal. Assert incompatible or missing targets produce status without a command.

  Add composer tests for Enter newline, `Alt+Enter` submit, `Ctrl+G`, Esc, create author `user`, body-only edit, empty-body failure preservation, mutation failure preservation, and editor failure restoring the pre-launch draft.

- [ ] **Step 2: Run interaction tests and verify they fail.**

  Run:

  ```bash
  go test ./internal/ui -run 'TestNote|TestComposer|Test.*Editor'
  ```

  Expected: FAIL because selection, draft, mutation, and editor messages are absent.

- [ ] **Step 3: Extend private note state with one composer and mutation lifecycle.**

  Reuse `textarea.Model` and keep creation/editing explicit:

  ```go
  type noteDraft struct {
      noteID string
      file   string
      line   int
      before string
  }

  type noteMutationMsg struct {
      kind noteMutationKind
      note notes.Note
      err  error
  }

  type noteEditorClosedMsg struct {
      body string
      err  error
  }
  ```

  Add the textarea and draft/delete/loading flags to `noteUIState`. While a draft is active, route all keys to the composer before global commands: Esc cancels, Enter inserts a newline, `alt+enter` submits, `ctrl+g` starts the editor, and every other key updates the textarea. Keep the draft open until a mutation succeeds.

- [ ] **Step 4: Implement stable cross-file note selection.**

  Build cross-file navigation targets from attached open notes in file-list order, then append resolved and stale notes from the Notes view. Within a patch, `j`/`k` traverses source lines and whole open-note cards with one full-row cursor. Sort same-file notes by line, `CreatedAt`, and ID.

  Replace `{`/`}` hunk aliases in file and patch handlers with note navigation. Retain `n`/`p` hunk navigation. Ordinary movement calls one `clearNoteSelection` helper.

- [ ] **Step 5: Implement direct mutations through the existing store.**

  Use exactly these operations inside asynchronous commands:

  ```go
  store.Add(notes.AddInput{File: file, Line: line, Body: body, Author: notes.AuthorUser})
  store.Edit(id, notes.EditInput{Body: &body})
  store.Edit(id, notes.EditInput{State: ptr(notes.StateResolved)})
  store.Remove(id)
  ```

  Do not optimistically edit `items`. On success, reload the ledger and preserve the ID for create/edit/resolve; after deletion select the nearest remaining note. On error, leave items and composer/confirmation state intact and set note status.

- [ ] **Step 6: Reuse Scry's external-editor process pattern.**

  Seed an owner-only temporary file from the textarea using `commit.PrepareEditorCmd`, suspend with `tea.ExecProcess`, read the result, remove the file, and return the body to the still-open composer. Trim only terminal line endings added by an editor; preserve internal lines. A launch/read/exit error leaves the original textarea value unchanged.

- [ ] **Step 7: Run focused tests and commit.**

  Run `go test ./internal/ui ./internal/ui/panes`. Commit with `feat(ui): manage persistent notes`.

### Task 4: Finish status/help integration and verify CLI interoperability

**Files:**

- Modify: `internal/ui/model.go`
- Modify: `internal/ui/statusbar.go`
- Modify: `internal/ui/statusbar_test.go`
- Modify: `internal/ui/help_test.go`
- Modify: `internal/ui/layout_chrome_test.go`
- Modify: `README.md`

**Interfaces:**

- Consumes: all prior tasks.
- Produces: discoverable bindings, stable error/status precedence, user documentation, and a testable binary.

- [ ] **Step 1: Write failing help, footer, and status tests.**

  Assert help includes the direct note commands and composer controls, patch footer advertises `c` and `{`/`}`, note errors appear without hiding higher-priority destructive-operation errors, and successful note actions clear stale note status. Add a model-level persistence test that creates through the TUI command, constructs a second store/model, and reads the same note.

- [ ] **Step 2: Run the focused tests and verify they fail.**

  Run:

  ```bash
  go test ./internal/ui -run 'TestHelp|TestStatus|TestPatchFooter|TestNotePersistence'
  ```

  Expected: FAIL until the new controls and status source are rendered.

- [ ] **Step 3: Add concise help, footer, and README documentation.**

  Document `c`, `j`/`k`, `{`/`}`, `E`, `R`, `D`, and composer keys in existing help sections. Add one README paragraph after the persistent notes CLI section explaining that Scry shows worktree notes inline, `r` reloads agent notes, and reattachment/state repair remains a CLI operation. Do not promise watch mode or dashboard counts.

- [ ] **Step 4: Run automated verification.**

  Run:

  ```bash
  gofmt -w internal/ui/*.go internal/ui/panes/*.go internal/app/*.go
  go test ./...
  go test -race ./...
  go vet ./...
  go build -o /tmp/scry-persistent-notes-tui ./cmd/scry
  GOOS=linux GOARCH=amd64 go build -o /tmp/scry-persistent-notes-tui-linux-amd64 ./cmd/scry
  git diff --check
  ```

- [ ] **Step 5: Run the manual acceptance pass with isolated config and worktrees.**

  Use the built binary and a temporary `XDG_CONFIG_HOME` to verify CLI add followed by TUI `r`, TUI create/edit/resolve/delete in unified and side-by-side modes, persistence after restart, stale transition after source-line change, `$EDITOR` return-to-composer behavior, and isolation between two worktrees. Preserve the resulting macOS binary at `/tmp/scry-persistent-notes-tui` for the user to test during handoff.

- [ ] **Step 6: Commit the completed integration.**

  Commit with `docs: document persistent notes TUI`.

### Final Review and Handoff

- [ ] Review the full diff against `origin/main` and the design spec; remove any note watcher, dashboard scan, fake Git file, duplicate storage abstraction, anchor mutation, or unrequested visual system.
- [ ] Run the configured Ponytail current-diff review and the repository's required Go checks again after any fix.
- [ ] Push `persistent-notes-tui`, open a draft PR, and provide the worktree binary path plus verification evidence.
