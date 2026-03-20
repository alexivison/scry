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

func TestRenderFileListGrouped_SelectedIdxMatchesFile(t *testing.T) {
	t.Parallel()

	// Files deliberately in unsorted order: root file first, then nested dirs.
	// After sortByDirectory, order becomes: cmd/main.go, internal/app.go, README.md.
	// If selectedIdx=2 targets README.md (original idx 2=internal/app.go),
	// the cursor must land on the file at sorted position 2 (README.md),
	// not the original position 2 (internal/app.go).
	files := []model.FileSummary{
		{Path: "README.md", Status: model.StatusModified, Additions: 1, Deletions: 0},
		{Path: "cmd/main.go", Status: model.StatusModified, Additions: 5, Deletions: 2},
		{Path: "internal/app.go", Status: model.StatusModified, Additions: 3, Deletions: 1},
	}
	// Select index 0 — in the original slice that's README.md.
	// After sorting: [cmd/main.go, internal/app.go, README.md].
	// The cursor on index 0 should highlight cmd/main.go (sorted[0]),
	// BUT the caller selected files[0] = README.md.
	// This mismatch is the bug: rendered cursor is on cmd/main.go
	// while the caller intended README.md.
	opts := FileListOpts{GroupByDirectory: true}
	output, _ := RenderFileList(files, 0, 0, 60, 20, true, opts)

	// The cursor line (containing ">") should show README.md (what index 0 IS),
	// not cmd/main.go (what sorted position 0 happens to be).
	lines := strings.Split(output, "\n")
	cursorFound := false
	for _, line := range lines {
		if strings.Contains(line, ">") {
			cursorFound = true
			if !strings.Contains(line, "README.md") {
				t.Errorf("cursor should be on README.md (files[0]), got: %q", line)
			}
			break
		}
	}
	if !cursorFound {
		t.Error("no cursor (>) found in output")
	}
}

// Ensure we can use FreshnessTier values in tests.
var _ = review.FreshnessHot
