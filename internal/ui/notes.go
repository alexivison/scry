package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/notes"
)

type noteUIState struct {
	store      *notes.Store
	items      []notes.Note
	selectedID string
	err        string
	generation int
	loading    bool
}

type notesLoadedMsg struct {
	notes      []notes.Note
	generation int
	err        error
}

func WithNoteStore(store *notes.Store, setupErr error) ModelOption {
	return func(m *Model) {
		m.setNoteStore(store, setupErr)
	}
}

func (m *Model) setNoteStore(store *notes.Store, setupErr error) tea.Cmd {
	m.noteState = noteUIState{store: store}
	if setupErr != nil {
		m.noteState.err = fmt.Sprintf("notes unavailable: %v", setupErr)
		return nil
	}
	if store == nil {
		return nil
	}
	m.noteState.generation = 1
	m.noteState.loading = true
	return m.loadNotes()
}

func (m Model) loadNotes() tea.Cmd {
	if m.noteState.store == nil {
		return nil
	}
	store := m.noteState.store
	generation := m.noteState.generation
	return func() tea.Msg {
		if _, err := store.Sync(); err != nil {
			return notesLoadedMsg{generation: generation, err: err}
		}
		items, err := store.List(nil)
		return notesLoadedMsg{notes: items, generation: generation, err: err}
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
	if msg.generation != m.noteState.generation {
		return m, nil
	}
	m.noteState.loading = false
	if msg.err != nil {
		m.noteState.err = fmt.Sprintf("notes refresh failed: %v", msg.err)
		return m, nil
	}
	m.noteState.items = msg.notes
	m.noteState.err = ""
	return m, nil
}
