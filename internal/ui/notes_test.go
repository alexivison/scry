package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
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

func TestNoteReloadKeepsSelectedNoteReachableWhenItBecomesStale(t *testing.T) {
	m := notePatchModel()
	note := uiNote("note", "main.go", 2, notes.StateOpen, "body")
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID
	note.State = notes.StateStale

	updated, _ := m.Update(notesLoadedMsg{notes: []notes.Note{note}, generation: m.noteState.generation})
	m = updated.(Model)
	row, ok := m.currentFileTreeRow()
	if !ok || row.Kind != panes.FileTreeRowNotes || m.noteState.selectedID != note.ID {
		t.Fatalf("selected stale note became unreachable: row=%#v id=%q", row, m.noteState.selectedID)
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

func TestStaleNotesViewScrollsWithoutPatchViewport(t *testing.T) {
	state := sampleState()
	state.Files = nil
	state.SelectedFile = -1
	m := NewModel(state)
	note := uiNote("stale", "gone.go", 7, notes.StateStale, strings.Repeat("line\n", 20))
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID
	m.positionSelectedNote()
	m.State.FocusPane = model.PanePatch

	m, _ = sendKey(m, "j")
	if m.noteState.selectedID != note.ID || m.noteState.listScroll != 1 {
		t.Fatalf("stale scroll lost selection or offset: id=%q offset=%d", m.noteState.selectedID, m.noteState.listScroll)
	}
}

func TestNoteTargetsFollowFilesThenStale(t *testing.T) {
	m := NewModel(sampleState())
	first := uiNote("main-1", "main.go", 2, notes.StateOpen, "one")
	second := uiNote("main-2", "main.go", 2, notes.StateResolved, "two")
	second.CreatedAt = first.CreatedAt.Add(time.Second)
	other := uiNote("new-1", "new.go", 1, notes.StateOpen, "three")
	stale := uiNote("stale", "gone.go", 1, notes.StateStale, "four")
	m.noteState.items = []notes.Note{stale, other, second, first}

	targets := m.noteTargets()
	got := make([]string, len(targets))
	for i, note := range targets {
		got[i] = note.ID
	}
	if strings.Join(got, ",") != "main-1,main-2,new-1,stale" {
		t.Fatalf("note target order = %v", got)
	}
}

func TestNoteNavigationSelectsCardsAndStaleView(t *testing.T) {
	m := notePatchModel()
	first := uiNote("main-1", "main.go", 2, notes.StateOpen, "one")
	second := uiNote("main-2", "main.go", 2, notes.StateOpen, "two")
	second.CreatedAt = first.CreatedAt.Add(time.Second)
	stale := uiNote("stale", "gone.go", 1, notes.StateStale, "three")
	m.noteState.items = []notes.Note{stale, second, first}

	m, _ = sendKey(m, "}")
	if m.noteState.selectedID != first.ID {
		t.Fatalf("first } selected %q, want %q", m.noteState.selectedID, first.ID)
	}
	m, _ = sendKey(m, "}")
	if m.noteState.selectedID != second.ID {
		t.Fatalf("second } selected %q, want %q", m.noteState.selectedID, second.ID)
	}
	m, _ = sendKey(m, "}")
	row, ok := m.currentFileTreeRow()
	if m.noteState.selectedID != stale.ID || !ok || row.Kind != panes.FileTreeRowNotes {
		t.Fatalf("third } did not select stale view: id=%q row=%#v", m.noteState.selectedID, row)
	}
	m, _ = sendKey(m, "}")
	if m.noteState.selectedID != stale.ID {
		t.Fatalf("navigation wrapped past final note to %q", m.noteState.selectedID)
	}
}

func TestNoteNavigationMovesAcrossCachedFiles(t *testing.T) {
	m := notePatchModel()
	mainNote := uiNote("main", "main.go", 2, notes.StateOpen, "main note")
	newNote := uiNote("new", "new.go", 2, notes.StateOpen, "new note")
	m.noteState.items = []notes.Note{mainNote, newNote}
	newPatch := samplePatch()
	newPatch.Summary.Path = "new.go"
	m.State.Patches["new.go"] = model.PatchLoadState{Status: model.LoadLoaded, Patch: &newPatch, Generation: m.State.CacheGeneration}

	m, _ = sendKey(m, "}")
	m, cmd := sendKey(m, "}")
	if cmd != nil {
		t.Fatal("cached target unexpectedly reloaded")
	}
	if m.noteState.selectedID != newNote.ID || m.State.SelectedFile != 1 || m.patchViewport == nil || m.patchViewport.Patch.Summary.Path != "new.go" {
		t.Fatalf("cross-file navigation missed target: id=%q file=%d patch=%#v", m.noteState.selectedID, m.State.SelectedFile, m.patchViewport)
	}
}

func TestNoteComposerCreatesMultilineUserNote(t *testing.T) {
	store := noteActionStore(t)
	m := notePatchModel()
	m.noteState.store = store

	m, _ = sendKey(m, "C")
	if m.noteState.composer == nil {
		t.Fatal("C did not open the composer")
	}
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("first")})
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("second")})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Alt+S did not save the composer")
	}
	m = drainCmd(t, m, cmd)

	items, err := store.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Body != "first\nsecond" || items[0].Author != notes.AuthorUser || items[0].File != "main.go" || items[0].Line != 2 {
		t.Fatalf("created notes = %#v", items)
	}
	if m.noteState.composer != nil {
		t.Fatal("successful create left composer open")
	}
}

func TestNoteComposerRendersInlineWithControls(t *testing.T) {
	m := notePatchModel()
	m.noteState.store = noteActionStore(t)
	m, _ = sendKey(m, "C")
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("inline draft")})

	patch := m.renderPatch(72, 25, 72)
	if !strings.Contains(patch, "inline draft") {
		t.Fatalf("composer was not rendered inline:\n%s", patch)
	}
	if status := m.viewStatusBar(); !strings.Contains(status, "Alt+S") || !strings.Contains(status, "Ctrl+G") {
		t.Fatalf("composer controls missing from status: %s", status)
	}
}

func TestNoteComposerRejectsHeaderAndEmptyBody(t *testing.T) {
	m := notePatchModel()
	m.noteState.store = noteActionStore(t)
	m.patchViewport.ScrollOffset = 0
	m, cmd := sendKey(m, "C")
	if cmd != nil || m.noteState.composer != nil || !strings.Contains(m.noteState.err, "source line") {
		t.Fatalf("C on header = composer %#v, err %q", m.noteState.composer, m.noteState.err)
	}

	m.patchViewport.ScrollOffset = 2
	m, _ = sendKey(m, "C")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	m = updated.(Model)
	if cmd != nil || m.noteState.composer == nil || !strings.Contains(m.noteState.err, "empty") {
		t.Fatalf("empty save = cmd %v, composer %#v, err %q", cmd != nil, m.noteState.composer, m.noteState.err)
	}
}

func TestNoteEditRequiresSelectionAndEscapeCancels(t *testing.T) {
	m := notePatchModel()
	m.noteState.store = noteActionStore(t)
	m, cmd := sendKey(m, "E")
	if cmd != nil || m.noteState.composer != nil || !strings.Contains(m.noteState.err, "Select a note") {
		t.Fatalf("E without selection = composer %#v, err %q", m.noteState.composer, m.noteState.err)
	}

	m, _ = sendKey(m, "C")
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("draft")})
	m, cmd = sendKey(m, "esc")
	if cmd != nil || m.noteState.composer != nil {
		t.Fatal("Esc did not cancel the composer")
	}
	items, err := m.noteState.store.List(nil)
	if err != nil || len(items) != 0 {
		t.Fatalf("cancel persisted a note: %#v, %v", items, err)
	}
}

func TestOrdinaryPatchMovementClearsNoteSelection(t *testing.T) {
	m := notePatchModel()
	note := uiNote("note", "main.go", 2, notes.StateOpen, "body")
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID

	m, _ = sendKey(m, "j")
	if m.noteState.selectedID != "" {
		t.Fatalf("patch movement kept note selection %q", m.noteState.selectedID)
	}
}

func TestNoteMutationFailureKeepsComposerText(t *testing.T) {
	store := noteActionStore(t)
	m := notePatchModel()
	m.noteState.store = store
	m, _ = sendKey(m, "C")
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("keep me")})
	if err := os.Remove(filepath.Join(store.Worktree(), "main.go")); err != nil {
		t.Fatal(err)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	m = updated.(Model)
	m = drainCmd(t, m, cmd)
	if m.noteState.composer == nil || m.noteState.composer.input.Value() != "keep me" || m.noteState.err == "" {
		t.Fatalf("failed save lost draft: composer %#v, err %q", m.noteState.composer, m.noteState.err)
	}
}

func TestNotePersistenceAcrossModels(t *testing.T) {
	worktree := t.TempDir()
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, "main.go"), []byte("package main\nimport \"os\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := notes.NewStore(worktree, configDir)
	if err != nil {
		t.Fatal(err)
	}
	m := notePatchModel()
	m.noteState.store = store
	m, _ = sendKey(m, "C")
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("survives restart")})
	m = saveNoteComposer(t, m)

	reopened, err := notes.NewStore(worktree, configDir)
	if err != nil {
		t.Fatal(err)
	}
	second := NewModel(sampleState(), WithNoteStore(reopened, nil))
	msg, ok := findMsg[notesLoadedMsg](execAndCollect(second.Init()))
	if !ok {
		t.Fatal("reopened model did not load notes")
	}
	updated, _ := second.Update(msg)
	second = updated.(Model)
	if len(second.noteState.items) != 1 || second.noteState.items[0].Body != "survives restart" {
		t.Fatalf("reopened notes = %#v", second.noteState.items)
	}
}

func TestNoteEditResolveAndDeleteUseNarrowMutations(t *testing.T) {
	store := noteActionStore(t)
	note, err := store.Add(notes.AddInput{File: "main.go", Line: 2, Body: "original", Author: notes.AuthorAgent})
	if err != nil {
		t.Fatal(err)
	}
	m := notePatchModel()
	m.noteState.store = store
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID

	m, _ = sendKey(m, "E")
	if m.noteState.composer == nil || m.noteState.composer.input.Value() != "original" {
		t.Fatal("E did not open selected body")
	}
	m.noteState.composer.input.SetValue("edited")
	m = saveNoteComposer(t, m)
	edited := m.noteState.items[0]
	if edited.Body != "edited" || edited.Author != note.Author || edited.File != note.File || edited.Line != note.Line || edited.State != note.State {
		t.Fatalf("edit changed fields beyond body: %#v", edited)
	}

	updated, cmd := m.Update(keyMsg('R'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("R did not resolve an open note")
	}
	m = drainCmd(t, m, cmd)
	if m.noteState.items[0].State != notes.StateResolved {
		t.Fatalf("resolved state = %q", m.noteState.items[0].State)
	}

	m, cmd = sendKey(m, "D")
	if cmd != nil || !m.noteState.confirmDelete {
		t.Fatal("D did not open confirmation")
	}
	m, cmd = sendKey(m, "y")
	if cmd == nil {
		t.Fatal("confirmed delete did not execute")
	}
	m = drainCmd(t, m, cmd)
	if len(m.noteState.items) != 0 || m.noteState.selectedID != "" {
		t.Fatalf("delete left note state: %#v", m.noteState)
	}
}

func notePatchModel() Model {
	m := NewModel(sampleState())
	m.width = 100
	m.height = 30
	m.State.FocusPane = "patch"
	m.patchViewport = panes.NewPatchViewport(samplePatch())
	m.patchViewport.Width = 72
	m.patchViewport.Height = 25
	m.patchViewport.ScrollOffset = 2
	return m
}

func noteActionStore(t *testing.T) *notes.Store {
	t.Helper()
	worktree := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, "main.go"), []byte("package main\nimport \"os\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := notes.NewStore(worktree, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func sendNoteKey(m Model, key tea.KeyMsg) Model {
	updated, _ := m.Update(key)
	return updated.(Model)
}

func saveNoteComposer(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: true})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Alt+S did not produce a save command")
	}
	return drainCmd(t, m, cmd)
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
