package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/alexivison/scry/internal/model"
)

func TestProjectFileTreeParentBeforeChildAndFileIndices(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "internal/ui/model_test.go"},
		{Path: "cmd/scry/main.go"},
		{Path: "README.md"},
		{Path: "internal/model/state.go"},
	}

	proj := ProjectFileTree(files, model.FileFilterAll, nil, 0, 0)

	assertRowBefore(t, proj.Rows, "cmd", "cmd/scry")
	assertRowBefore(t, proj.Rows, "cmd/scry", "cmd/scry/main.go")
	assertRowBefore(t, proj.Rows, "internal", "internal/model")
	assertRowBefore(t, proj.Rows, "internal", "internal/ui")

	gotIdx, ok := fileIndexForRow(proj.Rows, "internal/ui/model_test.go")
	if !ok {
		t.Fatal("missing file row for internal/ui/model_test.go")
	}
	if gotIdx != 0 {
		t.Fatalf("file row index = %d, want original index 0", gotIdx)
	}
}

func TestProjectFileTreeCollapsedDirectoryHidesDescendants(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "internal/ui/model.go"},
		{Path: "internal/model/state.go"},
		{Path: "README.md"},
	}

	proj := ProjectFileTree(files, model.FileFilterAll, map[string]bool{"internal": true}, 0, 0)

	if !hasRow(proj.Rows, "internal") {
		t.Fatal("collapsed directory row should remain visible")
	}
	if hasRow(proj.Rows, "internal/ui") || hasRow(proj.Rows, "internal/ui/model.go") {
		t.Fatalf("collapsed internal directory should hide descendants: %+v", proj.Rows)
	}
	if !hasRow(proj.Rows, "README.md") {
		t.Fatal("collapsed directory should not hide unrelated root files")
	}
}

func TestProjectFileTreeCursorMapsDirectoryAndFileRows(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "cmd/main.go"},
		{Path: "internal/app.go"},
	}

	proj := ProjectFileTree(files, model.FileFilterAll, nil, 0, 0)
	if proj.Rows[proj.Cursor].Kind != FileTreeRowDir {
		t.Fatalf("cursor row kind = %v, want dir", proj.Rows[proj.Cursor].Kind)
	}
	if proj.SelectedFile != -1 {
		t.Fatalf("SelectedFile = %d, want -1 on directory row", proj.SelectedFile)
	}

	fileCursor := rowIndex(proj.Rows, "internal/app.go")
	if fileCursor < 0 {
		t.Fatal("missing internal/app.go row")
	}
	proj = ProjectFileTree(files, model.FileFilterAll, nil, fileCursor, 0)
	if proj.SelectedFile != 1 {
		t.Fatalf("SelectedFile = %d, want original file index 1", proj.SelectedFile)
	}
}

func TestIsTestFileCommonPatterns(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"pkg/foo_test.go":                    true,
		"src/button.test.tsx":                true,
		"src/button.spec.ts":                 true,
		"src/__tests__/button.ts":            true,
		"test_api.py":                        true,
		"pkg/api_test.py":                    true,
		"tests/integration/api.py":           true,
		"crates/core/tests/parser.rs":        true,
		"src/UserTest.java":                  true,
		"src/UserTests.kt":                   true,
		"src/app.go":                         false,
		"src/contest.go":                     false,
		"src/latest.ts":                      false,
		"src/attestation.py":                 false,
		"src/TestData.java":                  false,
		"docs/tests-are-important/README.md": false,
	}

	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if got := IsTestFile(path); got != want {
				t.Fatalf("IsTestFile(%q) = %v, want %v", path, got, want)
			}
		})
	}
}

func TestProjectFileTreeFiltersTestsAndNonTests(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "cmd/main.go"},
		{Path: "cmd/main_test.go"},
		{Path: "web/button.spec.ts"},
	}

	tests := ProjectFileTree(files, model.FileFilterTests, nil, 0, 0)
	if tests.FilteredFiles != 2 {
		t.Fatalf("tests.FilteredFiles = %d, want 2", tests.FilteredFiles)
	}
	if hasRow(tests.Rows, "cmd/main.go") {
		t.Fatal("test filter should exclude non-test files")
	}

	nonTests := ProjectFileTree(files, model.FileFilterNonTests, nil, 0, 0)
	if nonTests.FilteredFiles != 1 {
		t.Fatalf("nonTests.FilteredFiles = %d, want 1", nonTests.FilteredFiles)
	}
	if hasRow(nonTests.Rows, "cmd/main_test.go") || hasRow(nonTests.Rows, "web/button.spec.ts") {
		t.Fatal("non-test filter should exclude detected test files")
	}

	empty := ProjectFileTree(files[:1], model.FileFilterTests, nil, 0, 0)
	if empty.FilteredFiles != 0 || len(empty.Rows) != 0 || empty.SelectedFile != -1 {
		t.Fatalf("empty test filter should be stable, got %+v", empty)
	}
}

func TestProjectFileTreeCollapsedFolderCanBeSelected(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "internal/app.go"},
	}

	proj := ProjectFileTree(files, model.FileFilterAll, map[string]bool{"internal": true}, 0, 0)
	if proj.Cursor != 0 {
		t.Fatalf("Cursor = %d, want collapsed folder row 0", proj.Cursor)
	}
	if proj.SelectedFile != -1 {
		t.Fatalf("SelectedFile = %d, want -1 on collapsed folder row", proj.SelectedFile)
	}
}

func TestRenderFileListTreeShowsDisclosureMarkers(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "cmd/main.go", Status: model.StatusModified},
		{Path: "internal/app.go", Status: model.StatusAdded},
	}

	expanded, _ := RenderFileList(files, 0, 0, 60, 10, true)
	plainExpanded := ansi.Strip(expanded)
	if !strings.Contains(plainExpanded, "├─ [-] cmd/") {
		t.Fatalf("expanded directory marker missing, got:\n%s", plainExpanded)
	}
	if strings.Contains(plainExpanded, ">") {
		t.Fatalf("file list should not render a left selection gutter, got:\n%s", plainExpanded)
	}
	if !strings.HasPrefix(plainExpanded, "├─ [-] cmd/") {
		t.Fatalf("tree content should start at left edge, got:\n%s", plainExpanded)
	}

	collapsed, _ := RenderFileList(files, 0, 0, 60, 10, true, FileListOpts{
		Collapsed: map[string]bool{"cmd": true},
	})
	plainCollapsed := ansi.Strip(collapsed)
	if !strings.Contains(plainCollapsed, "├─ [+] cmd/") {
		t.Fatalf("collapsed directory marker missing, got:\n%s", plainCollapsed)
	}
}

func TestRenderFileListTreeConnectorGlyphs(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "cmd/scry/main.go", Status: model.StatusModified},
		{Path: "cmd/scry/util.go", Status: model.StatusAdded},
		{Path: "internal/app.go", Status: model.StatusModified},
		{Path: "README.md", Status: model.StatusModified},
	}

	output, _ := RenderFileList(files, 0, 0, 80, 20, true)
	plain := ansi.Strip(output)
	want := []string{
		"├─ [-] cmd/",
		"│ └─ [-] scry/",
		"│   ├─ M main.go",
		"│ └─ M app.go",
		"└─ M README.md",
	}
	for _, fragment := range want {
		if !strings.Contains(plain, fragment) {
			t.Fatalf("missing connector fragment %q, got:\n%s", fragment, plain)
		}
	}
}

func TestRenderFileListTreeSelectedConnectorUsesMutedStyle(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(oldProfile) })

	files := []model.FileSummary{
		{Path: "README.md", Status: model.StatusModified},
	}

	output, _ := RenderFileList(files, 0, 0, 60, 10, true)
	wantConnector := treeConnectorSelectStyle.Render("└─ ")
	if !strings.Contains(output, wantConnector) {
		t.Fatalf("selected connector should use muted connector style, got:\n%q\nwant connector segment %q", output, wantConnector)
	}
	if strings.Contains(output, fileSelectedStyle.Render("└─ ")) {
		t.Fatalf("selected connector should not use selected text foreground, got:\n%q", output)
	}
}

func TestRenderFileListTreeStatusIconAdjacentToFileName(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "cmd/main.go", Status: model.StatusModified},
	}

	output, _ := RenderFileList(files, 0, 0, 60, 10, true)
	plain := ansi.Strip(output)
	if !strings.Contains(plain, "M main.go") {
		t.Fatalf("status icon should be adjacent to basename, got:\n%s", plain)
	}
	if strings.Contains(plain, "M  main.go") {
		t.Fatalf("status icon should not be separated from basename, got:\n%s", plain)
	}
}

func TestRenderFileListTreeSelectedRowSpansWidthAndCountsAlignRight(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "cmd/main.go", Status: model.StatusModified, Additions: 12, Deletions: 3},
	}

	const width = 42
	output, _ := RenderFileList(files, 0, 0, width, 10, true)
	lines := strings.Split(ansi.Strip(output), "\n")
	var selected string
	for _, line := range lines {
		if strings.Contains(line, "main.go") {
			selected = line
			break
		}
	}
	if selected == "" {
		t.Fatalf("selected file row missing, got:\n%s", output)
	}
	if got := len([]rune(selected)); got != width {
		t.Fatalf("selected row width = %d, want %d: %q", got, width, selected)
	}
	if !strings.HasSuffix(selected, "+12 -3") {
		t.Fatalf("counts should align to right edge, got %q", selected)
	}
}

func assertRowBefore(t *testing.T, rows []FileTreeRow, before, after string) {
	t.Helper()
	beforeIdx := rowIndex(rows, before)
	afterIdx := rowIndex(rows, after)
	if beforeIdx < 0 {
		t.Fatalf("missing row %q", before)
	}
	if afterIdx < 0 {
		t.Fatalf("missing row %q", after)
	}
	if beforeIdx >= afterIdx {
		t.Fatalf("row %q at %d should be before %q at %d", before, beforeIdx, after, afterIdx)
	}
}

func rowIndex(rows []FileTreeRow, path string) int {
	for i, row := range rows {
		if row.Path == path {
			return i
		}
	}
	return -1
}

func hasRow(rows []FileTreeRow, path string) bool {
	return rowIndex(rows, path) >= 0
}

func fileIndexForRow(rows []FileTreeRow, path string) (int, bool) {
	for _, row := range rows {
		if row.Path == path && row.Kind == FileTreeRowFile {
			return row.FileIndex, true
		}
	}
	return -1, false
}
