package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexivison/scry/internal/notes"
	"github.com/alexivison/scry/internal/ui/panes"
)

func TestNoteInitLoadsActiveWorktree(t *testing.T) {
	store, want := noteStoreWithOneNote(t)
	m := NewModel(sampleState(), WithNoteStore(store, nil))

	msg, ok := findMsg[notesLoadedMsg](execAndCollect(m.Init()))
	if !ok {
		t.Fatal("Init did not schedule a note load")
	}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if len(m.noteState.items) != 1 || m.noteState.items[0].ID != want.ID {
		t.Fatalf("loaded notes = %#v, want note %q", m.noteState.items, want.ID)
	}
	if m.noteState.loading {
		t.Fatal("note loading remained active after completion")
	}
	if m.noteState.err != "" {
		t.Fatalf("note error = %q, want empty", m.noteState.err)
	}
}

func TestNoteLoadFailureKeepsLastGoodSnapshot(t *testing.T) {
	_, note := noteStoreWithOneNote(t)
	m := NewModel(sampleState())
	m.noteState.items = []notes.Note{note}
	m.noteState.loading = true

	updated, _ := m.Update(notesLoadedMsg{
		generation: m.noteState.generation,
		err:        errors.New("ledger unavailable"),
	})
	m = updated.(Model)

	if len(m.noteState.items) != 1 || m.noteState.items[0].ID != note.ID {
		t.Fatalf("failed reload replaced last snapshot: %#v", m.noteState.items)
	}
	if m.noteState.err == "" {
		t.Fatal("failed reload did not surface an error")
	}
}

func TestNoteSuccessfulReloadClearsPriorError(t *testing.T) {
	_, note := noteStoreWithOneNote(t)
	m := NewModel(sampleState())
	m.noteState.err = "old failure"

	updated, _ := m.Update(notesLoadedMsg{
		notes:      []notes.Note{note},
		generation: m.noteState.generation,
	})
	m = updated.(Model)

	if m.noteState.err != "" {
		t.Fatalf("note error = %q, want empty", m.noteState.err)
	}
}

func TestNoteSetupFailureLeavesDiffUsable(t *testing.T) {
	m := NewModel(sampleState(), WithNoteStore(nil, errors.New("config unavailable")))

	if m.noteState.err == "" {
		t.Fatal("setup failure did not surface a note error")
	}
	if _, ok := findMsg[notesLoadedMsg](execAndCollect(m.Init())); ok {
		t.Fatal("setup failure scheduled a note load")
	}
	if len(m.State.Files) == 0 {
		t.Fatal("setup failure removed diff files")
	}
}

func TestNotesFeedSelectedFileAndStaleProjection(t *testing.T) {
	m := NewModel(sampleState())
	m.width = 100
	m.height = 30
	m.patchViewport = panes.NewPatchViewport(samplePatch())
	m.noteState.items = []notes.Note{
		uiNote("attached", "main.go", 2, notes.StateOpen, "Attached body"),
		uiNote("other", "new.go", 2, notes.StateOpen, "Other body"),
		uiNote("stale", "gone.go", 7, notes.StateStale, "Stale body"),
	}

	projection := m.fileTreeProjection()
	if row := projection.Rows[len(projection.Rows)-1]; row.Kind != panes.FileTreeRowNotes {
		t.Fatalf("last row kind = %v, want notes", row.Kind)
	}
	output := m.renderPatch(72, 30, 72)
	if !strings.Contains(output, "Attached body") || strings.Contains(output, "Other body") || strings.Contains(output, "Stale body") {
		t.Fatalf("selected file received wrong notes:\n%s", output)
	}
}

func TestStaleNotesRowRendersWithoutGitTarget(t *testing.T) {
	state := sampleState()
	state.Files = nil
	state.SelectedFile = -1
	m := NewModel(state)
	m.noteState.items = []notes.Note{uiNote("stale", "gone.go", 7, notes.StateStale, "Stale body")}
	m.setFileTreeCursor(0)

	if _, ok := m.selectedPatchPath(); ok {
		t.Fatal("stale notes row exposed a Git patch target")
	}
	output := m.renderPatch(72, 30, 72)
	if !strings.Contains(output, "gone.go:7") || !strings.Contains(output, "Stale body") {
		t.Fatalf("stale notes view missing content:\n%s", output)
	}
}

func uiNote(id, file string, line int, state notes.State, body string) notes.Note {
	return notes.Note{
		ID:        id,
		File:      file,
		Line:      line,
		State:     state,
		Body:      body,
		Author:    notes.AuthorAgent,
		CreatedAt: time.Unix(1, 0),
	}
}

func noteStoreWithOneNote(t *testing.T) (*notes.Store, notes.Note) {
	t.Helper()
	worktree := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := notes.NewStore(worktree, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	note, err := store.Add(notes.AddInput{
		File:   "main.go",
		Line:   1,
		Body:   "Keep this line.",
		Author: notes.AuthorAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, note
}
