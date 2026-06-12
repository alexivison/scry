package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/alexivison/scry/internal/model"
)

func treeState() model.AppState {
	return model.AppState{
		Compare:      sampleCompare(),
		CompareBasis: model.CompareBasisUpstream,
		Files: []model.FileSummary{
			{Path: "cmd/main.go", Status: model.StatusModified, Additions: 10, Deletions: 2},
			{Path: "internal/app_test.go", Status: model.StatusModified, Additions: 3, Deletions: 4},
			{Path: "internal/app.go", Status: model.StatusAdded, Additions: 7, Deletions: 1},
		},
		SelectedFile: 0,
		FocusPane:    model.PaneFiles,
		Patches:      make(map[string]model.PatchLoadState),
	}
}

func TestFileTreeNavigationMovesAcrossDirectoryRows(t *testing.T) {
	t.Parallel()

	m := NewModel(treeState())
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "j")
	if m.State.SelectedFile != -1 {
		t.Fatalf("SelectedFile = %d, want -1 on directory row", m.State.SelectedFile)
	}
	if got := selectedTreePath(m); got != "internal" {
		t.Fatalf("cursor path = %q, want internal", got)
	}

	m, _ = sendKey(m, "j")
	if m.State.SelectedFile != 2 {
		t.Fatalf("SelectedFile = %d, want original file index 2", m.State.SelectedFile)
	}
}

func TestFileTreeHOnFileCollapsesNearestParent(t *testing.T) {
	t.Parallel()

	state := treeState()
	state.SelectedFile = 2 // internal/app.go
	m := NewModel(state)
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "h")

	if !m.State.FileTreeCollapsed["internal"] {
		t.Fatal("internal directory should be collapsed")
	}
	if got := selectedTreePath(m); got != "internal" {
		t.Fatalf("cursor path = %q, want collapsed parent directory", got)
	}
	if m.State.SelectedFile != -1 {
		t.Fatalf("SelectedFile = %d, want -1 on collapsed directory", m.State.SelectedFile)
	}
}

func TestFileTreeRightExpandsSelectedCollapsedDirectory(t *testing.T) {
	t.Parallel()

	state := treeState()
	state.FileTreeCollapsed = map[string]bool{"internal": true}
	state.SelectedFile = -1
	state.FileTreeCursor = 2 // internal directory after cmd/, cmd/main.go
	m := NewModel(state)
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "right")
	if cmd != nil {
		t.Fatal("right on file list should not load a patch")
	}
	if m.State.FocusPane != model.PaneFiles {
		t.Fatalf("FocusPane = %q, want files", m.State.FocusPane)
	}
	if m.State.FileTreeCollapsed["internal"] {
		t.Fatal("right should expand selected collapsed directory")
	}
	if m.State.SelectedFile != -1 {
		t.Fatalf("SelectedFile = %d, want directory row to remain selected", m.State.SelectedFile)
	}
}

func TestFileFilterCycleUpdatesFooterAndSelection(t *testing.T) {
	t.Parallel()

	state := treeState()
	state.SelectedFile = 1 // internal/app_test.go
	m := NewModel(state)
	m.width = 100
	m.height = 31

	assertFilesHeader := func(t *testing.T, output, title, counts string) {
		t.Helper()
		header := strings.Split(ansi.Strip(output), "\n")[0]
		if !strings.Contains(header, title) {
			t.Fatalf("header should show %q, got:\n%s", title, output)
		}
		if !strings.HasSuffix(header, counts) {
			t.Fatalf("header should right-align aggregate counts %q, got header %q in:\n%s", counts, header, output)
		}
		if strings.Contains(header, "files ·") {
			t.Fatalf("header should not render old right-side file/filter context, got %q", header)
		}
	}

	assertFilesHeader(t, m.View(), "Files 3/3 (All)", "+20 -7")
	if meta := ansi.Strip(m.fileListFooter()); meta != "+20 -7" {
		t.Fatalf("file list right meta = %q, want only aggregate counts", meta)
	}

	m, _ = sendKey(m, "t")
	if m.State.FileFilter != model.FileFilterNonTests {
		t.Fatalf("FileFilter = %v, want non-tests", m.State.FileFilter)
	}
	if m.State.SelectedFile == 1 {
		t.Fatal("non-test filter should move selection off excluded test file")
	}
	assertFilesHeader(t, m.View(), "Files 2/3 (Non-Tests)", "+17 -3")

	m, _ = sendKey(m, "t")
	if m.State.FileFilter != model.FileFilterTests {
		t.Fatalf("FileFilter = %v, want tests", m.State.FileFilter)
	}
	if m.State.SelectedFile != 1 {
		t.Fatalf("SelectedFile = %d, want test file index 1", m.State.SelectedFile)
	}
	assertFilesHeader(t, m.View(), "Files 1/3 (Tests)", "+3 -4")

	m, _ = sendKey(m, "t")
	if m.State.FileFilter != model.FileFilterAll {
		t.Fatalf("FileFilter = %v, want all", m.State.FileFilter)
	}
	assertFilesHeader(t, m.View(), "Files 3/3 (All)", "+20 -7")
}

func TestFileFilterEmptyViewIsStable(t *testing.T) {
	t.Parallel()

	state := treeState()
	state.Files = []model.FileSummary{{Path: "cmd/main.go", Status: model.StatusModified}}
	m := NewModel(state)
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "t")
	m, _ = sendKey(m, "t")

	if m.State.SelectedFile != -1 {
		t.Fatalf("SelectedFile = %d, want -1 for empty filtered rows", m.State.SelectedFile)
	}
	if output := m.View(); !strings.Contains(output, "No test files changed.") {
		t.Fatalf("empty test filter message missing, got:\n%s", output)
	}
}

func TestSplitModeDirectoryCursorDoesNotLoadPatch(t *testing.T) {
	t.Parallel()

	state := treeState()
	state.Layout = model.LayoutSplit
	m := NewModel(state, WithPatchLoader(&mockPatchLoader{patches: map[string]model.FilePatch{
		"internal/app.go": samplePatch(),
	}}))
	m.width = 120
	m.height = 30

	m, cmd := sendKey(m, "j") // internal directory row
	if cmd != nil {
		t.Fatal("moving to a directory in split mode should not load a patch")
	}
	if got := selectedTreePath(m); got != "internal" {
		t.Fatalf("cursor path = %q, want internal directory", got)
	}
	if m.State.SelectedFile != -1 {
		t.Fatalf("SelectedFile = %d, want -1 on directory row", m.State.SelectedFile)
	}
	if len(m.State.Patches) != 0 {
		t.Fatalf("directory row should not mark patches loading: %+v", m.State.Patches)
	}
}

func selectedTreePath(m Model) string {
	rows := m.fileTreeRows()
	if m.State.FileTreeCursor < 0 || m.State.FileTreeCursor >= len(rows) {
		return ""
	}
	return rows[m.State.FileTreeCursor].Path
}
