package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/notes"
	"github.com/alexivison/scry/internal/ui/panes"
	"github.com/alexivison/scry/internal/ui/theme"
)

type noteUIState struct {
	store         *notes.Store
	items         []notes.Note
	selectedID    string
	err           string
	generation    int
	loading       bool
	mutating      bool
	composer      *noteComposer
	confirmDelete bool
	listScroll    int
	restoreView   *noteMutationView
}

type noteComposer struct {
	input  textarea.Model
	noteID string
	file   string
	line   int
}

type noteMutationView struct {
	path        string
	patchScroll int
	notes       bool
	listOffset  int
}

type notesLoadedMsg struct {
	notes           []notes.Note
	generation      int
	storeGeneration int
	err             error
}

type noteMutationMsg struct {
	selectedID      string
	storeGeneration int
	closeComposer   bool
	err             error
}

func WithNoteStore(store *notes.Store, setupErr error) ModelOption {
	return func(m *Model) {
		m.setNoteStore(store, setupErr)
	}
}

func (m *Model) setNoteStore(store *notes.Store, setupErr error) tea.Cmd {
	previous := m.noteState
	sameWorktree := previous.store != nil && store != nil && previous.store.Worktree() == store.Worktree()
	if sameWorktree {
		m.noteState.store = store
		m.noteState.generation++
	} else {
		m.noteStoreGeneration++
		m.noteState = noteUIState{store: store, generation: 1}
	}
	m.noteState.loading = false
	if setupErr != nil {
		m.noteState.err = fmt.Sprintf("notes unavailable: %v", setupErr)
		return nil
	}
	if store == nil {
		return nil
	}
	m.noteState.loading = true
	return m.loadNotes()
}

func (m Model) loadNotes() tea.Cmd {
	if m.noteState.store == nil {
		return nil
	}
	store := m.noteState.store
	generation := m.noteState.generation
	storeGeneration := m.noteStoreGeneration
	return func() tea.Msg {
		if _, err := store.Sync(); err != nil {
			return notesLoadedMsg{generation: generation, storeGeneration: storeGeneration, err: err}
		}
		items, err := store.List(nil)
		return notesLoadedMsg{notes: items, generation: generation, storeGeneration: storeGeneration, err: err}
	}
}

func (m *Model) startNoteLoad() tea.Cmd {
	if m.noteState.store == nil {
		return nil
	}
	m.noteState.generation++
	m.noteState.loading = true
	return m.loadNotes()
}

func (m Model) handleNotesLoaded(msg notesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.storeGeneration != m.noteStoreGeneration || msg.generation != m.noteState.generation {
		return m, nil
	}
	m.noteState.loading = false
	if msg.err != nil {
		m.noteState.restoreView = nil
		m.noteState.err = fmt.Sprintf("notes refresh failed: %v", msg.err)
		return m, nil
	}
	m.noteState.items = msg.notes
	m.noteState.err = ""
	if m.restoreNoteMutationView() {
		return m, nil
	}
	m.positionSelectedNote()
	if m.noteState.selectedID == "" {
		return m, nil
	}
	updated, cmd := m.loadSelectedPatch()
	m = updated.(Model)
	m.syncSelectedNoteViewport()
	return m, cmd
}

func (m *Model) restoreNoteMutationView() bool {
	view := m.noteState.restoreView
	m.noteState.restoreView = nil
	if view == nil {
		return false
	}
	if _, ok := m.selectedNote(); m.noteState.selectedID != "" && !ok {
		m.noteState.selectedID = ""
	}
	if view.notes {
		width, height := m.inactiveNoteListSize()
		base, _ := panes.NoteListOffset(m.inactiveNotes(), m.noteState.selectedID, width)
		listOffset := min(max(view.listOffset, 0), panes.NoteListMaxOffset(m.inactiveNotes(), width, height))
		m.noteState.listScroll = max(listOffset-base, 0)
		return true
	}
	current, ok := m.selectedPatchPath()
	if !ok || current != view.path || m.patchViewport == nil {
		return true
	}
	row, _ := m.currentFileTreeRow()
	m.patchViewport.SetNotes(m.attachedNotesForRow(row), m.noteState.selectedID, m.noteDraftView())
	m.patchViewport.KeepScroll(view.patchScroll)
	return true
}

func (m *Model) positionSelectedNote() {
	if m.noteState.selectedID == "" {
		return
	}
	note, ok := m.selectedNote()
	if !ok {
		m.noteState.selectedID = ""
		m.noteState.listScroll = 0
		return
	}
	if note.State != notes.StateOpen {
		proj := m.fileTreeProjection()
		m.setFileTreeCursor(len(proj.Rows) - 1)
		return
	}
	if row, ok := m.currentFileTreeRow(); ok && row.Kind == panes.FileTreeRowDir && strings.HasPrefix(note.File, row.Path+"/") {
		return
	}
	m.setFileTreeCursorByPath(note.File, m.State.FileTreeCursor)
}

func (m Model) attachedNotes(path string) []notes.Note {
	items := make([]notes.Note, 0)
	for _, note := range m.noteState.items {
		if note.File == path && note.State == notes.StateOpen {
			items = append(items, note)
		}
	}
	sortNotes(items)
	return items
}

func (m Model) attachedNotesForRow(row panes.FileTreeRow) []notes.Note {
	if row.Kind == panes.FileTreeRowFile && row.FileIndex >= 0 && row.FileIndex < len(m.State.Files) {
		return m.attachedNotes(m.State.Files[row.FileIndex].Path)
	}
	if row.Kind != panes.FileTreeRowDir {
		return nil
	}

	items := make([]notes.Note, 0)
	prefix := row.Path + "/"
	for _, note := range m.noteState.items {
		if note.State == notes.StateOpen && strings.HasPrefix(note.File, prefix) {
			items = append(items, note)
		}
	}
	sortNotes(items)
	return items
}

func (m Model) inactiveNotes() []notes.Note {
	items := make([]notes.Note, 0)
	for _, note := range m.noteState.items {
		if note.State != notes.StateOpen {
			items = append(items, note)
		}
	}
	return items
}

func (m Model) inactiveNoteCount() int {
	count := 0
	for _, note := range m.noteState.items {
		if note.State != notes.StateOpen {
			count++
		}
	}
	return count
}

func (m Model) noteTargets() []notes.Note {
	byFile := make(map[string][]notes.Note)
	var inactive []notes.Note
	for _, note := range m.noteState.items {
		if note.State == notes.StateOpen {
			byFile[note.File] = append(byFile[note.File], note)
		} else {
			inactive = append(inactive, note)
		}
	}

	var targets []notes.Note
	for _, row := range m.fileTreeProjection().Rows {
		if row.Kind != panes.FileTreeRowFile {
			continue
		}
		path := m.State.Files[row.FileIndex].Path
		items := byFile[path]
		sortNotes(items)
		targets = append(targets, items...)
	}
	sortNotes(inactive)
	return append(targets, inactive...)
}

func sortNotes(items []notes.Note) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		if items[i].Line != items[j].Line {
			return items[i].Line < items[j].Line
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
}

func (m Model) moveNoteSelection(delta int) (tea.Model, tea.Cmd) {
	targets := m.noteTargets()
	if len(targets) == 0 {
		m.noteState.err = "No notes"
		return m, nil
	}

	index := -1
	for i, note := range targets {
		if note.ID == m.noteState.selectedID {
			index = i
			break
		}
	}
	if index < 0 {
		if delta < 0 {
			index = len(targets) - 1
		} else {
			index = 0
		}
	} else {
		index += delta
		if index < 0 || index >= len(targets) {
			return m, nil
		}
	}

	target := targets[index]
	m.noteState.selectedID = target.ID
	m.noteState.err = ""
	m.State.FocusPane = model.PanePatch
	if target.State != notes.StateOpen {
		proj := m.fileTreeProjection()
		m.setFileTreeCursor(len(proj.Rows) - 1)
		return m, nil
	}

	m.setFileTreeCursorByPath(target.File, m.State.FileTreeCursor)
	if m.patchViewport != nil && m.patchViewport.Patch.Summary.Path == target.File {
		m.syncSelectedNoteViewport()
		return m, nil
	}
	updated, cmd := m.selectFile()
	m = updated.(Model)
	m.syncSelectedNoteViewport()
	return m, cmd
}

func (m Model) moveInactiveNoteSelection(delta int) (tea.Model, tea.Cmd) {
	items := m.inactiveNotes()
	sortNotes(items)
	if len(items) == 0 {
		return m, nil
	}
	index := -1
	for i, note := range items {
		if note.ID == m.noteState.selectedID {
			index = i
			break
		}
	}
	if index < 0 {
		if delta < 0 {
			index = len(items) - 1
		} else {
			index = 0
		}
	} else if next := index + delta; next >= 0 && next < len(items) {
		index = next
	}
	m.noteState.selectedID = items[index].ID
	m.noteState.listScroll = 0
	m.noteState.err = ""
	return m, nil
}

func (m *Model) syncSelectedNoteViewport() {
	if m.patchViewport == nil || m.noteState.selectedID == "" {
		return
	}
	row, ok := m.currentFileTreeRow()
	if !ok || row.Kind == panes.FileTreeRowNotes {
		return
	}
	m.patchViewport.SetNotes(m.attachedNotesForRow(row), m.noteState.selectedID, m.noteDraftView())
	m.patchViewport.ScrollToNote(m.noteState.selectedID)
}

func (m Model) startNoteComposer(noteID string) (tea.Model, tea.Cmd) {
	if m.noteState.store == nil {
		m.noteState.err = "Notes are unavailable"
		return m, nil
	}

	file, line, body := "", 0, ""
	if noteID == "" {
		if m.noteState.selectedID != "" {
			m.noteState.err = "Select a current source line to add a note"
			return m, nil
		}
		row, ok := m.currentFileTreeRow()
		if !ok || row.Kind == panes.FileTreeRowNotes || m.patchViewport == nil {
			m.noteState.err = "Select a current source line to add a note"
			return m, nil
		}
		var sourceLine bool
		file, line, sourceLine = m.patchViewport.CurrentSourceAnchor()
		if !sourceLine {
			m.noteState.err = "Select a current source line to add a note"
			return m, nil
		}
	} else {
		note, ok := m.selectedNote()
		if !ok || note.ID != noteID {
			m.noteState.err = "Select a note to edit"
			return m, nil
		}
		file, line, body = note.File, note.Line, note.Body
	}

	input := textarea.New()
	input.Placeholder = "Write a note…"
	input.Prompt = ""
	input.ShowLineNumbers = false
	input.FocusedStyle.CursorLine = input.FocusedStyle.CursorLine.Background(theme.SelectedBg)
	input.SetWidth(max(m.noteComposerWidth(), 20))
	input.SetHeight(3)
	input.SetValue(body)
	m.noteState.composer = &noteComposer{input: input, noteID: noteID, file: file, line: line}
	m.noteState.err = ""
	focus := m.noteState.composer.input.Focus()
	if row, ok := m.currentFileTreeRow(); ok && row.Kind != panes.FileTreeRowNotes && m.patchViewport != nil {
		m.patchViewport.SetNotes(m.attachedNotesForRow(row), m.noteState.selectedID, m.noteDraftView())
	}
	return m, focus
}

func (m Model) editSelectedNote() (tea.Model, tea.Cmd) {
	if m.noteState.selectedID == "" {
		m.noteState.err = "Select a note to edit"
		return m, nil
	}
	return m.startNoteComposer(m.noteState.selectedID)
}

func (m Model) noteComposerWidth() int {
	if m.patchViewport != nil {
		return m.patchViewport.NoteBodyWidth()
	}
	width := m.width - 8
	if m.State.Layout == model.LayoutSplit {
		width = m.width/2 - 8
	}
	return width
}

func (m Model) updateNoteComposer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.noteState.composer == nil {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.noteState.composer = nil
		m.noteState.err = ""
		return m, nil
	case "alt+enter":
		return m.saveNoteComposer()
	case "ctrl+g":
		return m.startNoteEditor()
	}
	var cmd tea.Cmd
	m.noteState.composer.input, cmd = m.noteState.composer.input.Update(msg)
	return m, cmd
}

func (m Model) saveNoteComposer() (tea.Model, tea.Cmd) {
	if m.noteState.composer == nil || m.noteState.mutating {
		return m, nil
	}
	body := m.noteState.composer.input.Value()
	if body == "" {
		m.noteState.err = "Note body cannot be empty"
		return m, nil
	}
	store := m.noteState.store
	storeGeneration := m.noteStoreGeneration
	composer := *m.noteState.composer
	m.noteState.mutating = true
	m.noteState.err = ""
	return m, func() tea.Msg {
		if composer.noteID == "" {
			note, err := store.Add(notes.AddInput{File: composer.file, Line: composer.line, Body: body, Author: notes.AuthorUser})
			return noteMutationMsg{selectedID: note.ID, storeGeneration: storeGeneration, closeComposer: true, err: err}
		}
		_, err := store.Edit(composer.noteID, notes.EditInput{Body: &body})
		return noteMutationMsg{selectedID: composer.noteID, storeGeneration: storeGeneration, closeComposer: true, err: err}
	}
}

func (m Model) resolveSelectedNote() (tea.Model, tea.Cmd) {
	note, ok := m.selectedNote()
	if !ok {
		m.noteState.err = "Select an open note to resolve"
		return m, nil
	}
	if note.State != notes.StateOpen {
		m.noteState.err = "Only open notes can be resolved"
		return m, nil
	}
	if m.noteState.store == nil || m.noteState.mutating {
		return m, nil
	}
	store := m.noteState.store
	storeGeneration := m.noteStoreGeneration
	state := notes.StateResolved
	m.noteState.mutating = true
	m.noteState.err = ""
	return m, func() tea.Msg {
		_, err := store.Edit(note.ID, notes.EditInput{State: &state})
		return noteMutationMsg{storeGeneration: storeGeneration, err: err}
	}
}

func (m Model) startNoteDeleteConfirm() (tea.Model, tea.Cmd) {
	if _, ok := m.selectedNote(); !ok {
		m.noteState.err = "Select a note to delete"
		return m, nil
	}
	m.noteState.confirmDelete = true
	m.noteState.err = ""
	return m, nil
}

func (m Model) updateNoteDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.noteState.confirmDelete = false
		return m.deleteSelectedNote()
	case "n", "N", "esc":
		m.noteState.confirmDelete = false
	}
	return m, nil
}

func (m Model) deleteSelectedNote() (tea.Model, tea.Cmd) {
	note, ok := m.selectedNote()
	if !ok || m.noteState.store == nil || m.noteState.mutating {
		return m, nil
	}
	store := m.noteState.store
	storeGeneration := m.noteStoreGeneration
	m.noteState.mutating = true
	return m, func() tea.Msg {
		_, err := store.Remove(note.ID)
		return noteMutationMsg{storeGeneration: storeGeneration, err: err}
	}
}

func (m Model) selectedNote() (notes.Note, bool) {
	for _, note := range m.noteState.items {
		if note.ID == m.noteState.selectedID {
			return note, true
		}
	}
	return notes.Note{}, false
}

func (m Model) handleNoteMutation(msg noteMutationMsg) (tea.Model, tea.Cmd) {
	if msg.storeGeneration != m.noteStoreGeneration {
		return m, nil
	}
	m.noteState.mutating = false
	if msg.err != nil {
		m.noteState.err = fmt.Sprintf("note action failed: %v", msg.err)
		return m, nil
	}
	if row, ok := m.currentFileTreeRow(); ok {
		switch row.Kind {
		case panes.FileTreeRowFile, panes.FileTreeRowDir:
			if m.patchViewport != nil {
				path, _ := m.selectedPatchPath()
				m.noteState.restoreView = &noteMutationView{
					path:        path,
					patchScroll: m.patchViewport.ScrollOffset,
				}
			}
		case panes.FileTreeRowNotes:
			width, _ := m.inactiveNoteListSize()
			base, _ := panes.NoteListOffset(m.inactiveNotes(), m.noteState.selectedID, width)
			m.noteState.restoreView = &noteMutationView{notes: true, listOffset: base + m.noteState.listScroll}
		}
	}
	m.noteState.selectedID = msg.selectedID
	if msg.closeComposer {
		m.noteState.composer = nil
	}
	m.noteState.err = ""
	return m, m.startNoteLoad()
}

func (m Model) noteDraftView() *panes.NoteDraftView {
	if m.noteState.composer == nil {
		return nil
	}
	composer := m.noteState.composer
	return &panes.NoteDraftView{
		NoteID: composer.noteID,
		File:   composer.file,
		Line:   composer.line,
		Body:   composer.input.View(),
	}
}

func (m Model) noteDeleteBody() string {
	note, ok := m.selectedNote()
	if !ok {
		return "Delete this note?"
	}
	return fmt.Sprintf("%s:%d\n\n%s", note.File, note.Line, strings.ReplaceAll(note.Body, "\n", " "))
}
