package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
)

// ctrlE returns a ctrl+e key message.
func ctrlE() tea.Msg {
	return tea.KeyMsg{Type: tea.KeyCtrlE}
}

// --- ctrl+e export tests ---

func TestExport_CtrlEWithFlaggedFiles(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.FlaggedFiles = map[string]bool{"main.go": true, "new.go": true}
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(ctrlE())
	um := updated.(Model)

	// exportMsg should be set (success or error depending on clipboard availability).
	if um.exportMsg == "" {
		t.Error("ctrl+e with flagged files should set exportMsg")
	}
	// Should show either "Copied" (success) or "Export failed" (no clipboard).
	hasResult := strings.Contains(um.exportMsg, "Copied") || strings.Contains(um.exportMsg, "Export failed")
	if !hasResult {
		t.Errorf("exportMsg = %q, want 'Copied...' or 'Export failed...'", um.exportMsg)
	}
}

func TestExport_CtrlEEmptyFlaggedSet(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.FlaggedFiles = map[string]bool{}
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(ctrlE())
	um := updated.(Model)

	view := um.View()
	if !strings.Contains(view, "No flagged files") {
		t.Errorf("ctrl+e with no flags should show 'No flagged files', got:\n%s", view)
	}
}

func TestExport_CtrlEIgnoredInSearchPane(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.FlaggedFiles = map[string]bool{"main.go": true}
	state.FocusPane = model.PaneSearch
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(ctrlE())
	um := updated.(Model)

	// Should not trigger export from search pane.
	view := um.View()
	if strings.Contains(view, "copied") || strings.Contains(view, "Copied") {
		t.Errorf("ctrl+e in search pane should not trigger export, got:\n%s", view)
	}
}

func TestExport_CtrlEIgnoredInCommitPane(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.FlaggedFiles = map[string]bool{"main.go": true}
	state.FocusPane = model.PaneCommit
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(ctrlE())
	um := updated.(Model)

	view := um.View()
	if strings.Contains(view, "copied") || strings.Contains(view, "Copied") {
		t.Errorf("ctrl+e in commit pane should not trigger export, got:\n%s", view)
	}
}

func TestExport_CtrlEIgnoredInIdlePane(t *testing.T) {
	t.Parallel()

	state := idleState()
	state.FlaggedFiles = map[string]bool{"main.go": true}
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(ctrlE())
	um := updated.(Model)

	view := um.View()
	if strings.Contains(view, "copied") || strings.Contains(view, "Copied") {
		t.Errorf("ctrl+e in idle pane should not trigger export, got:\n%s", view)
	}
}
