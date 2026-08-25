# Persistent Local Notes TUI Design

## Status

Approved for implementation. This document extends the persistent notes CLI
design with Scry's human-facing reader and editor surface.

## Goal

Let a person read and manage durable worktree notes without leaving Scry's diff
review. Agent and user notes appear beside current source lines, survive Scry
restarts, and remain interoperable with `scry note` commands.

## Scope

- Show attached notes inline in unified and side-by-side patches.
- Create `user` notes on current-source lines.
- Edit note bodies without changing authors, anchors, or states.
- Resolve open notes and delete any note through direct key bindings.
- Keep stale notes reachable when their file or line is absent from the diff.
- Reload CLI-created notes through Scry's existing refresh action.
- Support an inline multiline composer and an optional `$EDITOR` handoff.

## Out of Scope

- Watching note ledgers or refreshing them automatically.
- Note counts or indicators on the worktree dashboard.
- Creating notes on deleted-only lines.
- Reattaching notes or changing anchors through the TUI.
- Reopening resolved notes or changing arbitrary states through the TUI.
- Git, Hunk session, network, or repository-file persistence.
- Copying Hunk's visual theme.

## Navigation and Placement

Scry continues to render one selected file at a time. Attached notes become
first-class visual rows in `PatchViewport`, directly after the matching
current-source line. A card spans the patch width in both unified and
side-by-side layouts. Deleted-only rows cannot own cards because notes anchor
only to current source.

The viewport row plan owns card height, wrapping, scrolling, and visibility.
Cards are not ordinary code rows. This keeps source-line targeting and the
`n` / `p` hunk navigation unchanged while ensuring note height is included in
scroll math. The existing `{` / `}` hunk aliases are reassigned to notes.

`}` selects and reveals the next note, and `{` selects and reveals the previous
note. Navigation follows file-list order, then source line, creation time, and
note ID. It includes several notes on one line and the stale-notes entry, can
load another file when needed, and stops at the first or last note rather than
wrapping. Ordinary patch or file navigation clears the note selection so a
later action cannot target an off-screen card accidentally.

Scry renders a virtual `Stale notes (N)` entry after the real worktree files
when at least one stale note exists. It is a file-list presentation row, not a
fake `model.FileSummary`, so Git operations cannot target it. Selecting it
shows every stale note in the active worktree, including notes for files that
no longer appear in the diff. No stale entry appears on the worktree dashboard.

## Card Presentation

Cards use Scry's existing colors, borders, spacing, and text styles. Hunk is an
interaction reference only.

- The header identifies `User` or `Agent` and the note state.
- Open notes show their complete body.
- Resolved notes are muted and collapsed until selected.
- Stale notes show their last recorded `file:line` and remain readable.
- The selected card uses Scry's existing focused accent.
- Multiple cards on one line render in stable navigation order.

No timestamp, avatar, reply thread, or separate note theme is added in this
milestone.

## Key Bindings and Actions

The patch view adds these direct commands:

```text
C       create a user note on the current source line
{ / }   select the previous or next note
E       edit the selected note body
R       resolve the selected open note
D       delete the selected note after confirmation
```

The patch pane shows a gutter cursor on the selected current-source line.
`j` and `k` move it between context and added lines while skipping headers,
separators, note cards, and deleted-only rows. The cursor survives unified,
side-by-side, split, and modal layout changes. `C` without a selectable source
line leaves the UI unchanged and reports that a current-source line is
required. `E`, `R`, or `D` without a selected compatible note reports a short
status message.
Resolved and stale notes support `E` and `D`; `R` is only valid for open notes.

Deletion reuses Scry's confirmation-dialog pattern. Cancelling makes no ledger
change. The help view and patch footer advertise the new bindings.

## Composer and External Editor

`C` opens an inline multiline composer after the target source row. `E` opens
the same composer with the selected note body. The composer owns keyboard input
while active and uses the already-installed Bubbles textarea component.

```text
Enter     save
Alt+Enter insert a newline
Ctrl+G    continue editing in $EDITOR
Esc       cancel
```

`Ctrl+G` follows Scry's existing suspended-editor process pattern. Scry writes
the current draft to an owner-only temporary file, suspends the TUI, opens the
configured editor, and restores the edited text to the inline composer when
the editor exits. Returning from the editor does not save automatically.
Cancelling an edit leaves the saved note unchanged.

TUI-created notes always use author `user`. Editing changes only `body`.
Resolving changes only `state` to `resolved`.

## Architecture

The application creates the existing concrete `*notes.Store` for the active
worktree and supplies it to the UI model. No second notes service, storage
format, or single-implementation interface is introduced. Dashboard drill-down
results carry the selected worktree's store; returning to the dashboard clears
the loaded note and composer state.

The UI model owns:

- the active store and loaded `[]notes.Note` snapshot;
- the selected note ID;
- the textarea and create/edit target;
- note loading, mutation, confirmation, and error state.

`PatchViewport` receives presentation-ready notes for the selected file and
the active draft. It plans and renders cards but never reads or mutates the
ledger. The file-list projection owns the stale-notes presentation row without
adding it to the Git-backed file collection.

All storage operations run in `tea.Cmd` functions so file locking and source
fingerprinting do not block input or rendering. Existing store locking and
atomic writes preserve CLI/TUI concurrency.

## Data Flow

On initial diff entry and dashboard drill-down, Scry asynchronously calls
`Store.Sync` followed by `Store.List`. Refreshing with `r` batches the same note
reload with the existing diff refresh. `Sync` changes invalid open anchors to
`stale`; resolved notes remain resolved, preserving the distinction between an
explicitly handled concern and a displaced open concern.

Create, edit, resolve, and delete call the same store methods used by the CLI.
After a successful mutation, Scry reloads the active ledger and keeps the
selected note by ID when it still exists. Rebuilding viewport rows preserves
the current source target where possible and clamps the scroll position when
content disappears.

Only the active worktree ledger is opened. Scry never scans note ledgers for
dashboard rows or background updates.

## Failure Handling

Notes are supplementary to diff review. Store construction, loading, syncing,
or mutation failures never prevent Scry from opening or navigating a diff.

- A failed reload keeps the last successfully loaded note snapshot.
- A failed create or edit keeps the composer open with its text intact.
- A failed resolve or delete leaves the card unchanged.
- A failed `$EDITOR` launch or exit restores the inline composer and its
  pre-launch draft.
- Lock contention and corrupt-ledger errors are shown without retry loops or
  local optimistic changes.

Errors use Scry's existing status area. A later successful note operation or
refresh clears the note error.

## Verification

Focused tests cover:

- inline placement after current-source rows in unified and side-by-side views;
- card wrapping and viewport height accounting;
- no attachment to deleted-only rows;
- stable ordering and `{` / `}` navigation across files and same-line notes;
- open, resolved, selected, and stale presentation;
- the virtual stale-notes row without contaminating Git file operations;
- create and body-only edit composer behavior;
- `Enter`, `Alt+Enter`, `Ctrl+G`, and `Esc` draft lifecycle;
- direct `E`, `R`, and `D` actions and delete confirmation;
- note refresh, mutation failure preservation, and worktree store switching.

The implementation must pass `go test ./...`, `go test -race ./...`,
`go vet ./...`, and `go build ./cmd/scry`.

The manual acceptance pass must verify:

1. Add an agent note with the CLI and reveal it in the running TUI with `r`.
2. Create, edit, resolve, and delete user notes in both patch layouts.
3. Exit and reopen Scry and confirm persisted state.
4. Change an open note's source line, refresh, and find it under
   `Stale notes (N)`.
5. Drill into two worktrees and confirm their note ledgers remain isolated.
