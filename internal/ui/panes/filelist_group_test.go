package panes

import (
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/review"
)

func groupTestFiles() []model.FileSummary {
	return []model.FileSummary{
		{Path: "cmd/main.go", Status: model.StatusModified, Additions: 5, Deletions: 2},
		{Path: "cmd/util.go", Status: model.StatusAdded, Additions: 10, Deletions: 0},
		{Path: "internal/app.go", Status: model.StatusModified, Additions: 3, Deletions: 1},
		{Path: "README.md", Status: model.StatusModified, Additions: 1, Deletions: 0},
	}
}

func TestRenderFileListGrouped_ShowsDirectoryHeaders(t *testing.T) {
	t.Parallel()

	opts := FileListOpts{GroupByDirectory: true}
	output, _ := RenderFileList(groupTestFiles(), 0, 0, 60, 20, true, opts)

	// Should contain directory headers.
	if !strings.Contains(output, "cmd/") {
		t.Error("grouped output should contain 'cmd/' directory header")
	}
	if !strings.Contains(output, "internal/") {
		t.Error("grouped output should contain 'internal/' directory header")
	}
}

func TestRenderFileListGrouped_RootFilesNoHeader(t *testing.T) {
	t.Parallel()

	opts := FileListOpts{GroupByDirectory: true}
	output, _ := RenderFileList(groupTestFiles(), 3, 0, 60, 20, true, opts)

	// README.md is at root — should appear without a directory header prefix.
	if !strings.Contains(output, "README.md") {
		t.Error("root file README.md should be present")
	}
}

func TestRenderFileListUngrouped_NoHeaders(t *testing.T) {
	t.Parallel()

	// Default: grouping disabled.
	output, _ := RenderFileList(groupTestFiles(), 0, 0, 60, 20, true)

	// Should NOT contain directory headers as separate lines.
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "cmd/" || trimmed == "internal/" {
			t.Errorf("ungrouped output should not have directory header line: %q", line)
		}
	}
}

func TestRenderFileListGrouped_FreshnessAndFlagsWork(t *testing.T) {
	t.Parallel()

	opts := FileListOpts{
		GroupByDirectory: true,
		ChangeGen:        map[string]int{"cmd/main.go": 5},
		CurrentGen:       5,
		FlaggedFiles:     map[string]bool{"internal/app.go": true},
	}
	output, _ := RenderFileList(groupTestFiles(), 0, 0, 60, 20, true, opts)

	// Freshness marker (●) should appear for hot file.
	if !strings.Contains(output, "●") {
		t.Error("grouped output should show freshness marker for hot file")
	}
	// Flag marker (⚑) should appear for flagged file.
	if !strings.Contains(output, "⚑") {
		t.Error("grouped output should show flag marker for flagged file")
	}
}

func TestGroupedFileList_SelectionIndexMapsCorrectly(t *testing.T) {
	t.Parallel()

	// With grouping, selection index 0 should select the first FILE, not a header.
	files := groupTestFiles()
	opts := FileListOpts{GroupByDirectory: true}
	output, _ := RenderFileList(files, 0, 0, 60, 20, true, opts)

	// The first selectable item should have the > cursor.
	if !strings.Contains(output, ">") {
		t.Error("grouped output should show selection cursor")
	}
	// And should point to the first file (cmd/main.go), not a directory header.
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, ">") && !strings.Contains(line, "main.go") {
			// Check it's not a header line.
			if !strings.Contains(line, ".go") && !strings.Contains(line, ".md") {
				t.Errorf("selection cursor should be on a file, not a header: %q", line)
			}
		}
	}
}

// Ensure we can use FreshnessTier values in tests.
var _ = review.FreshnessHot
