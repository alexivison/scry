package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/notes"
	"github.com/alexivison/scry/internal/ui/panes"
)

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
	if m.noteState.mutating {
		return m, nil
	}
	switch msg.String() {
	case "y", "Y":
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
	m.noteState.confirmDelete = false
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
	return fmt.Sprintf("%s:%d\n\n%s", panes.SafeNoteText(note.File), note.Line, strings.ReplaceAll(panes.SafeNoteText(note.Body), "\n", " "))
}
