package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/panes"
)

// splitState returns a state pre-configured for split layout testing.
func splitState() model.AppState {
	s := sampleState()
	s.Layout = model.LayoutSplit
	return s
}

func sendKey(m Model, key string) (Model, tea.Cmd) {
	var msg tea.Msg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	result, cmd := m.Update(msg)
	return result.(Model), cmd
}

func TestTabTogglesLayout(t *testing.T) {
	t.Parallel()

	t.Run("modal to split", func(t *testing.T) {
		t.Parallel()
		m := NewModel(sampleState())
		m.State.Layout = model.LayoutModal
		m.width = 120
		m.height = 30
		m, _ = sendKey(m, "tab")
		if m.State.Layout != model.LayoutSplit {
			t.Errorf("Layout = %q, want %q", m.State.Layout, model.LayoutSplit)
		}
	})

	t.Run("split to modal", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 120
		m.height = 30
		m, _ = sendKey(m, "tab")
		if m.State.Layout != model.LayoutModal {
			t.Errorf("Layout = %q, want %q", m.State.Layout, model.LayoutModal)
		}
	})

	t.Run("tab round trip", func(t *testing.T) {
		t.Parallel()
		m := NewModel(sampleState())
		m.State.Layout = model.LayoutModal
		m.width = 120
		m.height = 30
		m, _ = sendKey(m, "tab")
		m, _ = sendKey(m, "tab")
		if m.State.Layout != model.LayoutModal {
			t.Errorf("Layout = %q after round-trip, want %q", m.State.Layout, model.LayoutModal)
		}
	})

	t.Run("tab ignored during help", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 120
		m.height = 30
		m.showHelp = true
		m, _ = sendKey(m, "tab")
		if m.State.Layout != model.LayoutSplit {
			t.Errorf("Layout changed during help: %q", m.State.Layout)
		}
	})

	t.Run("tab ignored during search", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 120
		m.height = 30
		m.State.FocusPane = model.PaneSearch
		m, _ = sendKey(m, "tab")
		if m.State.Layout != model.LayoutSplit {
			t.Errorf("Layout changed during search: %q", m.State.Layout)
		}
	})
}

func TestSplitModeLKey(t *testing.T) {
	t.Parallel()

	t.Run("l in file list focuses patch and loads", func(t *testing.T) {
		t.Parallel()
		loader := &mockPatchLoader{patches: map[string]model.FilePatch{
			"main.go": samplePatch(),
		}}
		m := NewModel(splitState(), WithPatchLoader(loader))
		m.width = 120
		m.height = 30
		m, cmd := sendKey(m, "l")
		if m.State.FocusPane != model.PanePatch {
			t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PanePatch)
		}
		if cmd == nil {
			t.Error("expected async load cmd, got nil")
		}
	})

	t.Run("l works in modal mode too", func(t *testing.T) {
		t.Parallel()
		loader := &mockPatchLoader{patches: map[string]model.FilePatch{
			"main.go": samplePatch(),
		}}
		m := NewModel(sampleState(), WithPatchLoader(loader))
		m.State.Layout = model.LayoutModal
		m.width = 120
		m.height = 30
		m, _ = sendKey(m, "l")
		if m.State.FocusPane != model.PanePatch {
			t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PanePatch)
		}
	})
}

func TestSplitModeH_PreservesViewport(t *testing.T) {
	t.Parallel()

	t.Run("h in split mode keeps viewport", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 120
		m.height = 30
		m.State.FocusPane = model.PanePatch
		m.patchViewport = panes.NewPatchViewport(samplePatch())
		m, _ = sendKey(m, "h")
		if m.State.FocusPane != model.PaneFiles {
			t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneFiles)
		}
		if m.patchViewport == nil {
			t.Error("patchViewport was cleared in split mode; should be preserved")
		}
	})

	t.Run("h in modal mode clears viewport", func(t *testing.T) {
		t.Parallel()
		m := NewModel(sampleState())
		m.State.Layout = model.LayoutModal
		m.width = 120
		m.height = 30
		m.State.FocusPane = model.PanePatch
		m.patchViewport = panes.NewPatchViewport(samplePatch())
		m, _ = sendKey(m, "h")
		if m.patchViewport != nil {
			t.Error("patchViewport should be cleared in modal mode")
		}
	})

	t.Run("esc in split mode keeps viewport", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 120
		m.height = 30
		m.State.FocusPane = model.PanePatch
		m.patchViewport = panes.NewPatchViewport(samplePatch())
		m, _ = sendKey(m, "esc")
		if m.State.FocusPane != model.PaneFiles {
			t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneFiles)
		}
		if m.patchViewport == nil {
			t.Error("patchViewport was cleared in split mode; should be preserved")
		}
	})
}

func TestSplitModeJK_AutoLoadsPatch(t *testing.T) {
	t.Parallel()

	t.Run("j in split mode triggers patch load", func(t *testing.T) {
		t.Parallel()
		loader := &mockPatchLoader{patches: map[string]model.FilePatch{}}
		m := NewModel(splitState(), WithPatchLoader(loader))
		m.width = 120
		m.height = 30
		m, cmd := sendKey(m, "j")
		if m.State.SelectedFile != 1 {
			t.Errorf("SelectedFile = %d, want 1", m.State.SelectedFile)
		}
		if cmd == nil {
			t.Error("expected async load cmd for auto-load, got nil")
		}
	})

	t.Run("k in split mode triggers patch load", func(t *testing.T) {
		t.Parallel()
		loader := &mockPatchLoader{patches: map[string]model.FilePatch{}}
		s := splitState()
		s.SelectedFile = 1
		m := NewModel(s, WithPatchLoader(loader))
		m.width = 120
		m.height = 30
		m, cmd := sendKey(m, "k")
		if m.State.SelectedFile != 0 {
			t.Errorf("SelectedFile = %d, want 0", m.State.SelectedFile)
		}
		if cmd == nil {
			t.Error("expected async load cmd for auto-load, got nil")
		}
	})

	t.Run("j in modal mode does NOT auto-load", func(t *testing.T) {
		t.Parallel()
		m := NewModel(sampleState())
		m.State.Layout = model.LayoutModal
		m.width = 120
		m.height = 30
		m, cmd := sendKey(m, "j")
		if m.State.SelectedFile != 1 {
			t.Errorf("SelectedFile = %d, want 1", m.State.SelectedFile)
		}
		if cmd != nil {
			t.Error("modal j should not trigger auto-load cmd")
		}
	})

	t.Run("j keeps focus on file list", func(t *testing.T) {
		t.Parallel()
		loader := &mockPatchLoader{patches: map[string]model.FilePatch{}}
		m := NewModel(splitState(), WithPatchLoader(loader))
		m.width = 120
		m.height = 30
		m, _ = sendKey(m, "j")
		if m.State.FocusPane != model.PaneFiles {
			t.Errorf("FocusPane = %q, want %q after j", m.State.FocusPane, model.PaneFiles)
		}
	})
}

func TestSplitView_RendersDivider(t *testing.T) {
	t.Parallel()

	m := NewModel(splitState())
	m.width = 120
	m.height = 30
	output := m.View()
	if !strings.Contains(output, "│") {
		t.Error("split view should contain │ divider")
	}
}

func TestSplitView_TwoColumns(t *testing.T) {
	t.Parallel()

	m := NewModel(splitState())
	m.width = 120
	m.height = 30
	// Pre-load a patch so right pane has content.
	m.patchViewport = panes.NewPatchViewport(samplePatch())
	m.patchViewport.Width = 80
	m.patchViewport.Height = 28

	output := m.View()
	lines := strings.Split(output, "\n")
	// Top border line should contain two adjacent rounded corners (left pane right + right pane left).
	if len(lines) > 0 && !strings.Contains(lines[0], "╮╭") {
		t.Errorf("line 0 missing adjacent pane borders ╮╭: %q", lines[0])
	}
	// Content lines (between borders) should contain side borders │.
	for i := 1; i < len(lines)-2; i++ {
		if i >= m.height-1 {
			break // status bar
		}
		if !strings.Contains(lines[i], "│") {
			t.Errorf("line %d missing side border: %q", i, lines[i])
		}
	}
}

func TestSplitGracefulDegradation(t *testing.T) {
	t.Parallel()

	t.Run("narrow terminal falls back to modal", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 79
		m.height = 30
		// Even though Layout is split, narrow width should render as modal.
		output := m.View()
		// In modal file-list view, there's a single bordered pane (no adjacent ╮╭).
		if strings.Contains(output, "╮╭") {
			t.Error("narrow terminal should fall back to modal (no split border adjacency)")
		}
	})

	t.Run("width exactly 80 uses split", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 80
		m.height = 30
		output := m.View()
		if !strings.Contains(output, "│") {
			t.Error("width=80 should render split layout with divider")
		}
	})

	t.Run("WindowSizeMsg below 80 degrades to modal not tooSmall", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		// Send real WindowSizeMsg — this triggers CheckDimensions.
		result, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
		m = result.(Model)
		// Should NOT be tooSmall — split degrades to modal.
		output := m.View()
		if strings.Contains(output, "too small") {
			t.Error("split mode at 60 cols should degrade to modal, not show 'too small'")
		}
		// Should show file list (modal fallback).
		if !strings.Contains(output, "main.go") {
			t.Error("modal fallback should show file list")
		}
	})

	t.Run("WindowSizeMsg truly tiny shows tooSmall", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		result, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 5})
		m = result.(Model)
		output := m.View()
		if !strings.Contains(output, "too small") && !strings.Contains(output, "Press q") {
			t.Error("truly tiny terminal should show 'too small' or 'Press q'")
		}
	})
}

func TestSplitView_ActivePaneIndicator(t *testing.T) {
	t.Parallel()

	t.Run("file list active shows indicator", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 120
		m.height = 30
		m.State.FocusPane = model.PaneFiles
		output := m.View()
		// The selected file should have the cursor prefix ">"
		if !strings.Contains(output, ">") {
			t.Error("active file list should show cursor >")
		}
	})
}

func TestSplitEnterFromFileList(t *testing.T) {
	t.Parallel()

	loader := &mockPatchLoader{patches: map[string]model.FilePatch{
		"main.go": samplePatch(),
	}}
	m := NewModel(splitState(), WithPatchLoader(loader))
	m.width = 120
	m.height = 30
	m, cmd := sendKey(m, "enter")
	if m.State.FocusPane != model.PanePatch {
		t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PanePatch)
	}
	if cmd == nil {
		t.Error("expected async load cmd from Enter")
	}
}

func TestSplitTabFromPatchPane(t *testing.T) {
	t.Parallel()

	m := NewModel(splitState())
	m.width = 120
	m.height = 30
	m.State.FocusPane = model.PanePatch
	m.patchViewport = panes.NewPatchViewport(samplePatch())
	m, _ = sendKey(m, "tab")
	if m.State.Layout != model.LayoutModal {
		t.Errorf("Layout = %q, want %q", m.State.Layout, model.LayoutModal)
	}
	// In modal mode with focus on patch, should show patch view.
	if m.State.FocusPane != model.PanePatch {
		t.Errorf("FocusPane = %q, want %q (should preserve)", m.State.FocusPane, model.PanePatch)
	}
}

func TestSplitToModalToSplit_LoadsPatch(t *testing.T) {
	t.Parallel()

	loader := &mockPatchLoader{patches: map[string]model.FilePatch{
		"main.go": samplePatch(),
	}}
	m := NewModel(sampleState(), WithPatchLoader(loader))
	m.State.Layout = model.LayoutModal
	m.width = 120
	m.height = 30

	// Toggle to split — should auto-load patch for selected file.
	m, cmd := sendKey(m, "tab")
	if m.State.Layout != model.LayoutSplit {
		t.Fatalf("Layout = %q, want %q", m.State.Layout, model.LayoutSplit)
	}
	if cmd == nil {
		t.Error("switching to split should auto-load patch for selected file")
	}
}

func TestSplitHelpOverlay(t *testing.T) {
	t.Parallel()

	m := NewModel(splitState())
	m.width = 120
	m.height = 30
	m, _ = sendKey(m, "?")
	output := m.View()
	if !strings.Contains(output, "Tab") {
		t.Error("help should mention Tab keybinding")
	}
	if !strings.Contains(output, "split") || !strings.Contains(output, "layout") {
		t.Error("help should describe split/layout toggle")
	}
}

func TestFileListScrollInSplitMode(t *testing.T) {
	t.Parallel()

	// Create a state with many files to force scrolling.
	files := make([]model.FileSummary, 50)
	for i := range files {
		files[i] = model.FileSummary{
			Path:      "file" + strings.Repeat("x", 5) + ".go",
			Status:    model.StatusModified,
			Additions: i,
		}
	}
	s := model.AppState{
		Compare:      sampleCompare(),
		Files:        files,
		SelectedFile: 0,
		FocusPane:    model.PaneFiles,
		Layout:       model.LayoutSplit,
		Patches:      make(map[string]model.PatchLoadState),
	}
	m := NewModel(s)
	m.width = 120
	m.height = 10 // Small height forces scrolling.

	// Navigate down past visible area.
	for i := 0; i < 15; i++ {
		m, _ = sendKey(m, "j")
	}
	if m.State.SelectedFile != 15 {
		t.Fatalf("SelectedFile = %d, want 15", m.State.SelectedFile)
	}

	// The view should still render without crashing and contain the selected file.
	output := m.View()
	if output == "" {
		t.Error("View() returned empty string")
	}
}
