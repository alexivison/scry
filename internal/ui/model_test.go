package ui

import (
	"context"
	"fmt"
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

func escMsg() tea.Msg {
	return tea.KeyMsg{Type: tea.KeyEscape}
}

func intP(n int) *int { return &n }

type mockPatchLoader struct {
	patches map[string]model.FilePatch
	err     error
}

func (m *mockPatchLoader) LoadPatch(_ context.Context, _ model.ResolvedCompare, filePath string, _ bool) (model.FilePatch, error) {
	if m.err != nil {
		return model.FilePatch{}, m.err
	}
	if fp, ok := m.patches[filePath]; ok {
		return fp, nil
	}
	return model.FilePatch{}, nil
}

func samplePatch() model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{
			{
				Header: "func init()", OldStart: 1, OldLen: 3, NewStart: 1, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: "package main"},
					{Kind: model.LineAdded, NewNo: intP(2), Text: `import "os"`},
				},
			},
			{
				Header: "func main()", OldStart: 10, OldLen: 3, NewStart: 11, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(10), NewNo: intP(11), Text: "func main() {"},
					{Kind: model.LineDeleted, OldNo: intP(11), Text: "old()"},
					{Kind: model.LineAdded, NewNo: intP(12), Text: "new()"},
				},
			},
		},
	}
}

func modelWithLoader() Model {
	loader := &mockPatchLoader{
		patches: map[string]model.FilePatch{
			"main.go": samplePatch(),
		},
	}
	m := NewModel(sampleState(), WithPatchLoader(loader))
	m.width = 100
	m.height = 30
	return m
}

// enterAndLoad simulates pressing Enter and completing the async load cycle.
// It calls Update(enterMsg()), executes the returned Cmd, and feeds the
// resulting PatchLoadedMsg back through Update.
func enterAndLoad(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := m.Update(enterMsg())
	um := updated.(Model)
	if cmd == nil {
		return um
	}
	msg := cmd()
	updated2, _ := um.Update(msg)
	return updated2.(Model)
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

// --- Patch pane integration tests ---

func TestEnterLoadsPatchAndSwitchesFocus(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	um := enterAndLoad(t, m)

	if um.State.FocusPane != model.PanePatch {
		t.Errorf("FocusPane = %q, want %q", um.State.FocusPane, model.PanePatch)
	}
	if um.patchViewport == nil {
		t.Fatal("patchViewport should not be nil after Enter")
	}
}

func TestEnterWithLoadError(t *testing.T) {
	t.Parallel()

	loader := &mockPatchLoader{err: fmt.Errorf("git error")}
	m := NewModel(sampleState(), WithPatchLoader(loader))
	m.width = 100
	m.height = 30

	um := enterAndLoad(t, m)

	if um.State.FocusPane != model.PanePatch {
		t.Errorf("FocusPane = %q, want %q", um.State.FocusPane, model.PanePatch)
	}
	if um.patchErr == "" {
		t.Error("patchErr should be set on load error")
	}
	view := um.View()
	if !strings.Contains(view, "Error loading patch") {
		t.Errorf("View() should show error, got:\n%s", view)
	}
}

func TestPatchPaneEscReturnsToFiles(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	um := enterAndLoad(t, m)

	updated2, _ := um.Update(escMsg())
	um2 := updated2.(Model)

	if um2.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q after Esc", um2.State.FocusPane, model.PaneFiles)
	}
	if um2.patchViewport != nil {
		t.Error("patchViewport should be nil after Esc")
	}
}

func TestPatchPaneHReturnsToFiles(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	um := enterAndLoad(t, m)

	updated2, _ := um.Update(keyMsg('h'))
	um2 := updated2.(Model)

	if um2.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q after h", um2.State.FocusPane, model.PaneFiles)
	}
}

func TestPatchPaneHunkNavigation(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	um := enterAndLoad(t, m)

	if um.patchViewport.CurrentHunk != 0 {
		t.Fatalf("initial hunk = %d, want 0", um.patchViewport.CurrentHunk)
	}

	// n -> next hunk
	updated2, _ := um.Update(keyMsg('n'))
	um2 := updated2.(Model)
	if um2.patchViewport.CurrentHunk != 1 {
		t.Errorf("after n: hunk = %d, want 1", um2.patchViewport.CurrentHunk)
	}

	// n at last hunk -> no-op
	updated3, _ := um2.Update(keyMsg('n'))
	um3 := updated3.(Model)
	if um3.patchViewport.CurrentHunk != 1 {
		t.Errorf("n at last: hunk = %d, want 1", um3.patchViewport.CurrentHunk)
	}

	// p -> prev hunk
	updated4, _ := um3.Update(keyMsg('p'))
	um4 := updated4.(Model)
	if um4.patchViewport.CurrentHunk != 0 {
		t.Errorf("after p: hunk = %d, want 0", um4.patchViewport.CurrentHunk)
	}

	// p at first hunk -> no-op
	updated5, _ := um4.Update(keyMsg('p'))
	um5 := updated5.(Model)
	if um5.patchViewport.CurrentHunk != 0 {
		t.Errorf("p at first: hunk = %d, want 0", um5.patchViewport.CurrentHunk)
	}
}

func TestPatchPaneQQuits(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	um := enterAndLoad(t, m)

	updated2, cmd := um.Update(keyMsg('q'))
	um2 := updated2.(Model)
	if !um2.quitting {
		t.Error("q in patch pane should quit")
	}
	if cmd == nil {
		t.Error("q should return tea.Quit command")
	}
}

func TestPatchPaneLineScroll(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	um := enterAndLoad(t, m)

	if um.patchViewport.ScrollOffset != 0 {
		t.Fatalf("initial scroll = %d, want 0", um.patchViewport.ScrollOffset)
	}

	// j scrolls down
	updated2, _ := um.Update(keyMsg('j'))
	um2 := updated2.(Model)
	if um2.patchViewport.ScrollOffset != 1 {
		t.Errorf("after j: scroll = %d, want 1", um2.patchViewport.ScrollOffset)
	}

	// k scrolls back up
	updated3, _ := um2.Update(keyMsg('k'))
	um3 := updated3.(Model)
	if um3.patchViewport.ScrollOffset != 0 {
		t.Errorf("after k: scroll = %d, want 0", um3.patchViewport.ScrollOffset)
	}

	// k at top is no-op
	updated4, _ := um3.Update(keyMsg('k'))
	um4 := updated4.(Model)
	if um4.patchViewport.ScrollOffset != 0 {
		t.Errorf("k at top: scroll = %d, want 0", um4.patchViewport.ScrollOffset)
	}
}

func TestPatchPaneViewRendersDiff(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	um := enterAndLoad(t, m)

	view := um.View()
	if !strings.Contains(view, "func init()") {
		t.Errorf("patch view missing hunk header, got:\n%s", view)
	}
	if !strings.Contains(view, "package main") {
		t.Errorf("patch view missing context line, got:\n%s", view)
	}
}
