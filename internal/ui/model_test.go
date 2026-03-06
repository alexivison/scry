package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/alexivison/scry/internal/model"
)

func sampleFiles() []model.FileSummary {
	return []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 10, Deletions: 5},
		{Path: "new.go", Status: model.StatusAdded, Additions: 30, Deletions: 0},
		{Path: "old.go", Status: model.StatusDeleted, Additions: 0, Deletions: 20},
	}
}

func sampleCompare() model.ResolvedCompare {
	return model.ResolvedCompare{
		BaseRef:   "abc123",
		HeadRef:   "def456",
		DiffRange: "abc123...def456",
	}
}

func sampleState() model.AppState {
	return model.AppState{
		Compare:      sampleCompare(),
		Files:        sampleFiles(),
		SelectedFile: 0,
		FocusPane:    model.PaneFiles,
		Patches:      make(map[string]model.PatchLoadState),
	}
}

func keyMsg(r rune) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func enterMsg() tea.Msg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

// --- NewModel tests ---

func TestNewModelNonEmpty(t *testing.T) {
	t.Parallel()

	state := sampleState()
	m := NewModel(state)
	if m.State.SelectedFile != 0 {
		t.Errorf("SelectedFile = %d, want 0", m.State.SelectedFile)
	}
	if m.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneFiles)
	}
}

func TestNewModelEmpty(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Files:   nil,
		Patches: make(map[string]model.PatchLoadState),
	}
	m := NewModel(state)
	if m.State.SelectedFile != -1 {
		t.Errorf("SelectedFile = %d, want -1 for empty file list", m.State.SelectedFile)
	}
}

// --- Update key tests ---

func TestUpdateKeyJ(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	updated, _ := m.Update(keyMsg('j'))
	um := updated.(Model)
	if um.State.SelectedFile != 1 {
		t.Errorf("after j: SelectedFile = %d, want 1", um.State.SelectedFile)
	}
}

func TestUpdateKeyK(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.SelectedFile = 2
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(keyMsg('k'))
	um := updated.(Model)
	if um.State.SelectedFile != 1 {
		t.Errorf("after k: SelectedFile = %d, want 1", um.State.SelectedFile)
	}
}

func TestUpdateKeyJAtEnd(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.SelectedFile = 2 // last file
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(keyMsg('j'))
	um := updated.(Model)
	if um.State.SelectedFile != 2 {
		t.Errorf("j at end: SelectedFile = %d, want 2 (no-op)", um.State.SelectedFile)
	}
}

func TestUpdateKeyKAtStart(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState()) // SelectedFile = 0
	m.width = 100
	m.height = 30

	updated, _ := m.Update(keyMsg('k'))
	um := updated.(Model)
	if um.State.SelectedFile != 0 {
		t.Errorf("k at start: SelectedFile = %d, want 0 (no-op)", um.State.SelectedFile)
	}
}

func TestUpdateKeyEnter(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	updated, _ := m.Update(enterMsg())
	um := updated.(Model)
	if um.State.FocusPane != model.PanePatch {
		t.Errorf("after Enter: FocusPane = %q, want %q", um.State.FocusPane, model.PanePatch)
	}
}

func TestUpdateKeyQ(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	updated, cmd := m.Update(keyMsg('q'))
	um := updated.(Model)
	if !um.quitting {
		t.Error("after q: quitting should be true")
	}
	// q should return a tea.Quit command
	if cmd == nil {
		t.Error("after q: cmd should not be nil (expected tea.Quit)")
	}
}

func TestUpdateKeyQInHelp(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	// Open help
	updated, _ := m.Update(keyMsg('?'))
	um := updated.(Model)

	// q in help mode should quit, not just close help
	updated2, cmd := um.Update(keyMsg('q'))
	um2 := updated2.(Model)
	if !um2.quitting {
		t.Error("q in help: quitting should be true")
	}
	if cmd == nil {
		t.Error("q in help: cmd should not be nil (expected tea.Quit)")
	}
}

func TestUpdateKeyHelp(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	// Toggle on
	updated, _ := m.Update(keyMsg('?'))
	um := updated.(Model)
	if !um.showHelp {
		t.Error("after ?: showHelp should be true")
	}

	// Toggle off
	updated2, _ := um.Update(keyMsg('?'))
	um2 := updated2.(Model)
	if um2.showHelp {
		t.Error("after second ?: showHelp should be false")
	}
}

func TestUpdateWindowSize(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	um := updated.(Model)
	if um.width != 120 {
		t.Errorf("width = %d, want 120", um.width)
	}
	if um.height != 40 {
		t.Errorf("height = %d, want 40", um.height)
	}
}

func TestUpdateKeyJEmptyFiles(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Files:        nil,
		SelectedFile: -1,
		FocusPane:    model.PaneFiles,
		Patches:      make(map[string]model.PatchLoadState),
	}
	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, _ := m.Update(keyMsg('j'))
	um := updated.(Model)
	if um.State.SelectedFile != -1 {
		t.Errorf("j on empty: SelectedFile = %d, want -1", um.State.SelectedFile)
	}
}

// --- View tests ---

func TestViewFileList(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	view := m.View()

	// File list should contain file names
	for _, f := range sampleFiles() {
		if !strings.Contains(view, f.Path) {
			t.Errorf("View() missing file path %q", f.Path)
		}
	}
}

func TestViewFileListStatusIcons(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	view := m.View()

	// Should show addition/deletion counts
	if !strings.Contains(view, "+10") {
		t.Error("View() missing +10 additions for main.go")
	}
	if !strings.Contains(view, "-5") {
		t.Error("View() missing -5 deletions for main.go")
	}
}

func TestViewStatusBar(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	view := m.View()

	// Status bar should show the compare range
	if !strings.Contains(view, "abc123...def456") {
		t.Error("View() missing compare range in status bar")
	}
}

func TestViewHelpOverlay(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30

	// Toggle help on
	updated, _ := m.Update(keyMsg('?'))
	um := updated.(Model)
	helpView := um.View()

	// Help overlay should mention available keys
	if !strings.Contains(helpView, "j/k") {
		t.Error("help overlay missing j/k key binding")
	}
	if !strings.Contains(helpView, "q") {
		t.Error("help overlay missing q key binding")
	}
}

func TestViewRenameShowsArrow(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Compare: sampleCompare(),
		Files: []model.FileSummary{
			{Path: "new.go", OldPath: "old.go", Status: model.StatusRenamed, Additions: 5, Deletions: 3},
		},
		SelectedFile: 0,
		FocusPane:    model.PaneFiles,
		Patches:      make(map[string]model.PatchLoadState),
	}
	m := NewModel(state)
	m.width = 100
	m.height = 30

	view := m.View()

	// Renamed files should show old → new
	if !strings.Contains(view, "old.go") || !strings.Contains(view, "new.go") {
		t.Error("View() missing rename paths")
	}
}
