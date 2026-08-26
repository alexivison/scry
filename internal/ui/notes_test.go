package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

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

func TestLateNoteLoadCannotCrossWorktrees(t *testing.T) {
	storeA, noteA := noteStoreWithOneNote(t)
	storeB, noteB := noteStoreWithOneNote(t)
	m := NewModel(sampleState(), WithNoteStore(storeA, nil))
	lateLoad := m.loadNotes()
	m.setNoteStore(storeB, nil)
	m.noteState.items = []notes.Note{noteB}

	updated, _ := m.Update(lateLoad())
	m = updated.(Model)
	if m.noteState.store != storeB || len(m.noteState.items) != 1 || m.noteState.items[0].ID != noteB.ID {
		t.Fatalf("late worktree A load replaced B: A=%q state=%#v", noteA.ID, m.noteState)
	}
}

func TestLateNoteMutationCannotCrossWorktrees(t *testing.T) {
	storeA, noteA := noteStoreWithOneNote(t)
	storeB, noteB := noteStoreWithOneNote(t)
	m := NewModel(sampleState(), WithNoteStore(storeA, nil))
	m.noteState.items = []notes.Note{noteA}
	m.noteState.selectedID = noteA.ID
	updated, mutation := m.resolveSelectedNote()
	m = updated.(Model)
	m.setNoteStore(storeB, nil)
	m.noteState.items = []notes.Note{noteB}

	updated, _ = m.Update(mutation())
	m = updated.(Model)
	if m.noteState.store != storeB || len(m.noteState.items) != 1 || m.noteState.items[0].ID != noteB.ID {
		t.Fatalf("late worktree A mutation altered B: %#v", m.noteState)
	}
}

func TestSameWorktreeRefreshDoesNotLoseLateMutation(t *testing.T) {
	store, note := noteStoreWithOneNote(t)
	m := NewModel(sampleState(), WithNoteStore(store, nil))
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID
	updated, mutation := m.resolveSelectedNote()
	m = updated.(Model)
	refresh := m.setNoteStore(store, nil)
	m = deepDrain(t, m, refresh)

	updated, reload := m.Update(mutation())
	m = updated.(Model)
	m = deepDrain(t, m, reload)
	if len(m.noteState.items) != 1 || m.noteState.items[0].State != notes.StateResolved {
		t.Fatalf("late same-worktree mutation was lost: %#v", m.noteState)
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

func TestNoteReloadLoadsRepairedAnchorFile(t *testing.T) {
	m := notePatchModel()
	note := uiNote("note", "main.go", 2, notes.StateOpen, "body")
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID
	repaired := note
	repaired.File = "new.go"
	newPatch := samplePatch()
	newPatch.Summary.Path = "new.go"
	m.State.Patches["new.go"] = model.PatchLoadState{Status: model.LoadLoaded, Patch: &newPatch, Generation: m.State.CacheGeneration}

	updated, cmd := m.Update(notesLoadedMsg{notes: []notes.Note{repaired}, generation: m.noteState.generation})
	m = updated.(Model)
	if cmd != nil || m.patchViewport == nil || m.patchViewport.Patch.Summary.Path != "new.go" || m.State.SelectedFile != 1 {
		t.Fatalf("repaired anchor did not load new file: cmd=%v file=%d patch=%#v", cmd != nil, m.State.SelectedFile, m.patchViewport)
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
		uiNote("resolved", "main.go", 2, notes.StateResolved, "Resolved body"),
		uiNote("stale", "gone.go", 7, notes.StateStale, "Stale body"),
	}

	projection := m.fileTreeProjection()
	if row := projection.Rows[len(projection.Rows)-1]; row.Kind != panes.FileTreeRowNotes {
		t.Fatalf("last row kind = %v, want notes", row.Kind)
	}
	output := m.renderPatch(72, 30, 72)
	if !strings.Contains(output, "Attached body") || strings.Contains(output, "Other body") || strings.Contains(output, "Resolved body") || strings.Contains(output, "Stale body") {
		t.Fatalf("selected file received wrong notes:\n%s", output)
	}
	m.setFileTreeCursor(len(projection.Rows) - 1)
	output = m.renderPatch(72, 30, 72)
	if !strings.Contains(output, "Resolved body") || !strings.Contains(output, "main.go:2") || !strings.Contains(output, "Stale body") {
		t.Fatalf("bottom notes view missed inactive notes:\n%s", output)
	}
}

func TestNotesRowRendersWithoutGitTarget(t *testing.T) {
	state := sampleState()
	state.Files = nil
	state.SelectedFile = -1
	m := NewModel(state)
	m.noteState.items = []notes.Note{uiNote("stale", "gone.go", 7, notes.StateStale, "Stale body")}
	m.setFileTreeCursor(0)

	if _, ok := m.selectedPatchPath(); ok {
		t.Fatal("notes row exposed a Git patch target")
	}
	output := m.renderPatch(72, 30, 72)
	if !strings.Contains(output, "gone.go:7") || !strings.Contains(output, "Stale body") {
		t.Fatalf("notes view missing content:\n%s", output)
	}
}

func TestNotesViewCursorSelectsCards(t *testing.T) {
	state := sampleState()
	state.Files = nil
	state.SelectedFile = -1
	m := NewModel(state)
	resolved := uiNote("resolved", "a.go", 2, notes.StateResolved, "resolved")
	stale := uiNote("stale", "b.go", 7, notes.StateStale, "stale")
	m.noteState.items = []notes.Note{stale, resolved}
	m.setFileTreeCursor(0)
	m.State.FocusPane = model.PanePatch

	m, _ = sendKey(m, "j")
	if m.noteState.selectedID != resolved.ID {
		t.Fatalf("first notes-view cursor = %q, want %q", m.noteState.selectedID, resolved.ID)
	}
	m, _ = sendKey(m, "j")
	if m.noteState.selectedID != stale.ID {
		t.Fatalf("second notes-view cursor = %q, want %q", m.noteState.selectedID, stale.ID)
	}
	m, _ = sendKey(m, "k")
	if m.noteState.selectedID != resolved.ID {
		t.Fatalf("reverse notes-view cursor = %q, want %q", m.noteState.selectedID, resolved.ID)
	}
}

func TestNoteTargetsFollowOpenFilesThenInactiveNotes(t *testing.T) {
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
	if strings.Join(got, ",") != "main-1,new-1,stale,main-2" {
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

func TestSourceCursorNavigationSkipsPatchChromeAndDeletedLines(t *testing.T) {
	m := notePatchModel()
	m.patchViewport.ScrollOffset = 0

	assertSourceLine(t, m, 1)
	m, _ = sendKey(m, "j")
	assertSourceLine(t, m, 2)
	m, _ = sendKey(m, "j")
	assertSourceLine(t, m, 11)
	m, _ = sendKey(m, "j")
	assertSourceLine(t, m, 12)
	m, _ = sendKey(m, "k")
	assertSourceLine(t, m, 11)
}

func TestComposerOpensAfterDiffModeRoundTrip(t *testing.T) {
	m := notePatchModel()
	m.noteState.store = noteActionStore(t)
	m.patchViewport.ScrollOffset = 0
	for range 3 {
		m, _ = sendKey(m, "j")
	}

	m, _ = sendKey(m, "s")
	m, _ = sendKey(m, "s")
	m, _ = sendKey(m, "c")
	if m.noteState.composer == nil || m.noteState.composer.line != 12 {
		t.Fatalf("diff-mode round trip lost source anchor: %#v", m.noteState.composer)
	}
}

func TestNoteComposerCreatesMultilineUserNote(t *testing.T) {
	store := noteActionStore(t)
	m := notePatchModel()
	m.noteState.store = store

	m, _ = sendKey(m, "c")
	if m.noteState.composer == nil {
		t.Fatal("c did not open the composer")
	}
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("first")})
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("second")})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Alt+Enter did not save the composer")
	}
	m = deepDrain(t, m, cmd)

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
	before := m.patchViewport.TotalLines()
	m, _ = sendKey(m, "c")
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("inline draft")})

	patch := m.renderPatch(72, 25, 72)
	if !strings.Contains(patch, "inline draft") {
		t.Fatalf("composer was not rendered inline:\n%s", patch)
	}
	if status := m.viewStatusBar(); !strings.Contains(status, "Enter newline") || !strings.Contains(status, "Alt+Enter submit") || !strings.Contains(status, "Ctrl+G") {
		t.Fatalf("composer controls missing from status: %s", status)
	}
	if rows := m.patchViewport.TotalLines() - before; rows != 5 {
		t.Fatalf("composer card uses %d rows, want 5 (three editable rows plus chrome): %q", rows, m.noteDraftView().Body)
	}
}

func TestLongNoteComposerDoesNotLeakANSIFragments(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(oldProfile)

	m := notePatchModel()
	m.noteState.store = noteActionStore(t)
	m, _ = sendKey(m, "c")
	m.noteState.composer.input.SetValue(strings.Repeat("long note ", 20))

	plain := ansi.Strip(m.renderPatch(30, 25, 30))
	if strings.Contains(plain, ";") {
		t.Fatalf("long composer leaked an ANSI fragment:\n%s", plain)
	}
}

func TestEditingNotePreservesPatchScroll(t *testing.T) {
	store := noteActionStore(t)
	note, err := store.Add(notes.AddInput{File: "main.go", Line: 2, Body: "original", Author: notes.AuthorUser})
	if err != nil {
		t.Fatal(err)
	}
	m := notePatchModel()
	m.noteState.store = store
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID
	m.patchViewport.Height = 5
	m.patchViewport.SetNotes([]notes.Note{note}, note.ID, nil)
	m.renderPatch(72, 5, 72)
	m.patchViewport.ScrollOffset = 4
	m.renderPatch(72, 5, 72)

	m, _ = sendKey(m, "E")
	if m.patchViewport.ScrollOffset != 4 {
		t.Fatalf("opening editor moved scroll to %d, want 4", m.patchViewport.ScrollOffset)
	}
	m.noteState.composer.input.SetValue("edited")
	m = saveNoteComposer(t, m)
	m.renderPatch(72, 5, 72)
	if m.patchViewport == nil || m.patchViewport.ScrollOffset != 4 {
		t.Fatalf("saving edit changed patch scroll: %#v", m.patchViewport)
	}
}

func TestEditingNotePreservesNotesViewScroll(t *testing.T) {
	store := noteActionStore(t)
	items := addResolvedNotes(t, store, "first", "original", "third")
	m := notePatchModel()
	m.width = 40
	m.height = 6
	m.noteState.store = store
	m.noteState.items = items
	m.noteState.selectedID = items[1].ID
	projection := m.fileTreeProjection()
	m.setFileTreeCursor(len(projection.Rows) - 1)
	m.noteState.listScroll = 1

	m, _ = sendKey(m, "E")
	m.noteState.composer.input.SetValue("edited")
	m = saveNoteComposer(t, m)
	plain := ansi.Strip(m.renderPatch(40, 3, 40))
	if m.noteState.listScroll != 1 || !strings.Contains(plain, "edited") || strings.Contains(plain, "first") {
		t.Fatalf("editing note changed rendered Notes offset (scroll %d):\n%s", m.noteState.listScroll, plain)
	}
}

func TestDeletingLaterNotePreservesRenderedNotesOffset(t *testing.T) {
	store := noteActionStore(t)
	items := addResolvedNotes(t, store, "first", "second", "third")
	m := notePatchModel()
	m.width = 40
	m.height = 6
	m.noteState.store = store
	m.noteState.items = items
	m.noteState.selectedID = items[1].ID
	projection := m.fileTreeProjection()
	m.setFileTreeCursor(len(projection.Rows) - 1)
	m.noteState.listScroll = 1

	updated, cmd := m.deleteSelectedNote()
	m = updated.(Model)
	m = deepDrain(t, m, cmd)
	plain := ansi.Strip(m.renderPatch(40, 3, 40))
	if !strings.Contains(plain, "third") || strings.Contains(plain, "first") {
		t.Fatalf("deleting later card reset rendered Notes offset:\n%s", plain)
	}
}

func addResolvedNotes(t *testing.T, store *notes.Store, bodies ...string) []notes.Note {
	t.Helper()
	state := notes.StateResolved
	items := make([]notes.Note, 0, len(bodies))
	for _, body := range bodies {
		note, err := store.Add(notes.AddInput{File: "main.go", Line: 1, Body: body, Author: notes.AuthorUser})
		if err != nil {
			t.Fatal(err)
		}
		note, err = store.Edit(note.ID, notes.EditInput{State: &state})
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, note)
	}
	return items
}

func TestFailedMutationReloadDoesNotAffectLaterRefresh(t *testing.T) {
	store := noteActionStore(t)
	note, err := store.Add(notes.AddInput{File: "main.go", Line: 2, Body: "body", Author: notes.AuthorUser})
	if err != nil {
		t.Fatal(err)
	}
	m := notePatchModel()
	m.noteState.store = store
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID

	updated, _ := m.Update(noteMutationMsg{selectedID: note.ID, storeGeneration: m.noteStoreGeneration})
	m = updated.(Model)
	updated, _ = m.Update(notesLoadedMsg{
		generation:      m.noteState.generation,
		storeGeneration: m.noteStoreGeneration,
		err:             errors.New("reload failed"),
	})
	m = updated.(Model)

	repaired := note
	repaired.File = "new.go"
	newPatch := samplePatch()
	newPatch.Summary.Path = "new.go"
	m.State.Patches["new.go"] = model.PatchLoadState{Status: model.LoadLoaded, Patch: &newPatch, Generation: m.State.CacheGeneration}
	m.startNoteLoad()
	updated, _ = m.Update(notesLoadedMsg{
		notes:           []notes.Note{repaired},
		generation:      m.noteState.generation,
		storeGeneration: m.noteStoreGeneration,
	})
	m = updated.(Model)
	if m.State.SelectedFile != 1 || m.patchViewport == nil || m.patchViewport.Patch.Summary.Path != "new.go" {
		t.Fatalf("failed mutation reload leaked restoration into later refresh: file=%d patch=%#v", m.State.SelectedFile, m.patchViewport)
	}
}

func TestNoteComposerScrollsIntoViewAtViewportBottom(t *testing.T) {
	lines := make([]model.DiffLine, 0, 12)
	for i := 1; i <= 12; i++ {
		lines = append(lines, model.DiffLine{Kind: model.LineAdded, NewNo: intP(i), Text: strings.Repeat("x", 50)})
	}
	patch := model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks:   []model.Hunk{{Lines: lines}},
	}
	m := notePatchModel()
	m.noteState.store = noteActionStore(t)
	m.patchViewport = panes.NewPatchViewport(patch)
	m.patchViewport.Width = 72
	m.patchViewport.Height = 5
	for range 4 {
		m.patchViewport.MoveCursor(1)
	}

	m, _ = sendKey(m, "c")
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bottom draft")})
	if output := m.renderPatch(30, 5, 30); !strings.Contains(output, "bottom draft") {
		t.Fatalf("composer opened below viewport:\n%s", output)
	}
}

func TestNoteComposerRejectsHeaderAndEmptyBody(t *testing.T) {
	m := notePatchModel()
	m.noteState.store = noteActionStore(t)
	m.patchViewport = panes.NewPatchViewport(model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusDeleted},
		Hunks: []model.Hunk{{
			OldStart: 1,
			OldLen:   1,
			Lines:    []model.DiffLine{{Kind: model.LineDeleted, OldNo: intP(1), Text: "removed"}},
		}},
	})
	m, cmd := sendKey(m, "c")
	if cmd != nil || m.noteState.composer != nil || !strings.Contains(m.noteState.err, "source line") {
		t.Fatalf("c without a current-source line = composer %#v, err %q", m.noteState.composer, m.noteState.err)
	}

	m.patchViewport = panes.NewPatchViewport(samplePatch())
	m.patchViewport.ScrollOffset = 2
	m, _ = sendKey(m, "c")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
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

	m, _ = sendKey(m, "c")
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

func TestPatchCursorSelectsInlineNote(t *testing.T) {
	m := notePatchModel()
	note := uiNote("note", "main.go", 1, notes.StateOpen, "body")
	m.noteState.items = []notes.Note{note}
	m.patchViewport.ScrollOffset = 0
	m.renderPatch(72, 25, 72)

	m, _ = sendKey(m, "j")
	if m.noteState.selectedID != note.ID {
		t.Fatalf("patch cursor selected %q, want note %q", m.noteState.selectedID, note.ID)
	}
	m, _ = sendKey(m, "j")
	if m.noteState.selectedID != "" {
		t.Fatalf("moving past note kept selection %q", m.noteState.selectedID)
	}
	if line, ok := m.patchViewport.CurrentSourceLine(); !ok || line != 2 {
		t.Fatalf("source cursor after note = %d, %v; want 2, true", line, ok)
	}
}

func TestNoteMutationFailureKeepsComposerText(t *testing.T) {
	store := noteActionStore(t)
	m := notePatchModel()
	m.noteState.store = store
	m, _ = sendKey(m, "c")
	m = sendNoteKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("keep me")})
	if err := os.Remove(filepath.Join(store.Worktree(), "main.go")); err != nil {
		t.Fatal(err)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(Model)
	m = deepDrain(t, m, cmd)
	if m.noteState.composer == nil || m.noteState.composer.input.Value() != "keep me" || m.noteState.err == "" {
		t.Fatalf("failed save lost draft: composer %#v, err %q", m.noteState.composer, m.noteState.err)
	}
}

func TestSuccessfulMutationReloadsConcurrentCLINotes(t *testing.T) {
	store := noteActionStore(t)
	note, err := store.Add(notes.AddInput{File: "main.go", Line: 2, Body: "original", Author: notes.AuthorAgent})
	if err != nil {
		t.Fatal(err)
	}
	m := notePatchModel()
	m.noteState.store = store
	m.noteState.items = []notes.Note{note}
	m.noteState.selectedID = note.ID
	updated, mutation := m.resolveSelectedNote()
	m = updated.(Model)
	mutationMsg := mutation()
	if _, err := store.Add(notes.AddInput{File: "main.go", Line: 1, Body: "from CLI", Author: notes.AuthorAgent}); err != nil {
		t.Fatal(err)
	}
	updated, reload := m.Update(mutationMsg)
	m = updated.(Model)
	if reload == nil {
		t.Fatal("successful mutation did not request an authoritative reload")
	}
	m = deepDrain(t, m, reload)
	if len(m.noteState.items) != 2 {
		t.Fatalf("authoritative reload lost concurrent note: %#v", m.noteState.items)
	}
}

func TestDeleteReturnsToSourceWithoutChangingScroll(t *testing.T) {
	store := noteActionStore(t)
	first, err := store.Add(notes.AddInput{File: "main.go", Line: 1, Body: "first", Author: notes.AuthorUser})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Add(notes.AddInput{File: "main.go", Line: 2, Body: "second", Author: notes.AuthorUser})
	if err != nil {
		t.Fatal(err)
	}
	m := notePatchModel()
	m.noteState.store = store
	m.noteState.items = []notes.Note{first, second}
	m.noteState.selectedID = first.ID
	m.patchViewport.Height = 5
	m.patchViewport.SetNotes([]notes.Note{first, second}, first.ID, nil)
	m.renderPatch(72, 5, 72)
	m.patchViewport.ScrollOffset = 4
	m.renderPatch(72, 5, 72)

	updated, cmd := m.deleteSelectedNote()
	m = updated.(Model)
	m = deepDrain(t, m, cmd)
	m.renderPatch(72, 5, 72)
	if m.noteState.selectedID != "" {
		t.Fatalf("delete kept note selection %q", m.noteState.selectedID)
	}
	if m.patchViewport == nil || m.patchViewport.ScrollOffset != 4 {
		t.Fatalf("delete changed patch scroll: %#v", m.patchViewport)
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
	m, _ = sendKey(m, "c")
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
	m = deepDrain(t, m, cmd)
	if m.noteState.items[0].State != notes.StateResolved {
		t.Fatalf("resolved state = %q", m.noteState.items[0].State)
	}
	if row, ok := m.currentFileTreeRow(); !ok || row.Kind != panes.FileTreeRowFile {
		t.Fatalf("resolving note left the patch: %#v", row)
	}
	if m.noteState.selectedID != "" {
		t.Fatalf("resolving note kept selection %q", m.noteState.selectedID)
	}

	m, _ = sendKey(m, "}")
	m, cmd = sendKey(m, "D")
	if cmd != nil || !m.noteState.confirmDelete {
		t.Fatal("D did not open confirmation")
	}
	m, cmd = sendKey(m, "y")
	if cmd == nil {
		t.Fatal("confirmed delete did not execute")
	}
	m = deepDrain(t, m, cmd)
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
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Alt+Enter did not produce a save command")
	}
	return deepDrain(t, m, cmd)
}

func assertSourceLine(t *testing.T, m Model, want int) {
	t.Helper()
	got, ok := m.patchViewport.CurrentSourceLine()
	if !ok || got != want {
		t.Fatalf("source line = %d, %v; want %d, true", got, ok, want)
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
