package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/panes"
)

// startDiscardConfirm opens the discard confirmation modal for the selected file.
// No-op when the discarder is unconfigured, the view is not in working-tree mode,
// the worktree dashboard is active, or no file is selected.
func (m Model) startDiscardConfirm() (tea.Model, tea.Cmd) {
	if m.fileDiscarder == nil {
		return m, nil
	}
	if !m.State.Compare.WorkingTree {
		return m, nil
	}
	if m.State.WorktreeMode {
		return m, nil
	}
	if m.State.SelectedFile < 0 || m.State.SelectedFile >= len(m.State.Files) {
		return m, nil
	}
	if m.State.DiscardInFlight {
		return m, nil
	}
	f := m.State.Files[m.State.SelectedFile]
	m.State.ConfirmDiscard = true
	m.State.DiscardPath = f.Path
	m.State.DiscardUntracked = f.Status == model.StatusUntracked
	m.State.DiscardErr = ""
	return m, nil
}

// updateDiscardConfirm handles key events while the discard confirmation modal is open.
func (m Model) updateDiscardConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.State.ConfirmDiscard = false
		return m.executeDiscard()
	case "n", "N", "esc":
		m.State.ConfirmDiscard = false
		m.State.DiscardPath = ""
		m.State.DiscardUntracked = false
	}
	return m, nil
}

// executeDiscard fires an async discard command.
func (m Model) executeDiscard() (tea.Model, tea.Cmd) {
	if m.fileDiscarder == nil {
		return m, nil
	}
	discarder := m.fileDiscarder
	path := m.State.DiscardPath
	untracked := m.State.DiscardUntracked

	m.State.DiscardInFlight = true
	cmd := func() tea.Msg {
		err := discarder.Discard(context.Background(), path, untracked)
		return FileDiscardedMsg{Path: path, Err: err}
	}
	return m, tea.Batch(cmd, m.spinner.Tick)
}

// handleFileDiscarded processes the result of an async discard.
// On success it triggers a metadata refresh so the discarded file disappears
// (or its status updates) from the file list.
func (m Model) handleFileDiscarded(msg FileDiscardedMsg) (tea.Model, tea.Cmd) {
	m.State.DiscardInFlight = false
	m.State.DiscardPath = ""
	m.State.DiscardUntracked = false

	if msg.Err != nil {
		m.State.DiscardErr = fmt.Sprintf("discard failed: %v", msg.Err)
		return m, nil
	}
	return m.startRefresh()
}

// overlayDiscardConfirm renders the discard confirmation modal on top of the base view.
func (m Model) overlayDiscardConfirm(base string) string {
	outerHeight := m.height - 1
	if outerHeight < 3 {
		outerHeight = 3
	}
	body := m.State.DiscardPath
	if m.State.DiscardUntracked {
		body += "\n\n(untracked — file will be deleted)"
	} else {
		body += "\n\nChanges will be reverted to HEAD."
	}
	return panes.OverlayDialog(base, "Discard changes?", body, "y confirm    n/Esc cancel", m.width, outerHeight)
}
