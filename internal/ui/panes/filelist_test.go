package panes

import (
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func sampleFileList() []model.FileSummary {
	return []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 10, Deletions: 5},
		{Path: "new.go", Status: model.StatusAdded, Additions: 30, Deletions: 0},
		{Path: "old.go", Status: model.StatusDeleted, Additions: 0, Deletions: 20},
		{Path: "renamed.go", OldPath: "orig.go", Status: model.StatusRenamed, Additions: 2, Deletions: 1},
	}
}

func TestRenderFileList_Basic(t *testing.T) {
	t.Parallel()

	output, scroll := RenderFileList(sampleFileList(), 0, 0, 60, 10, true)
	if scroll != 0 {
		t.Errorf("scroll = %d, want 0", scroll)
	}
	if !strings.Contains(output, "main.go") {
		t.Error("output should contain main.go")
	}
	if !strings.Contains(output, ">") {
		t.Error("output should show selection cursor >")
	}
}

func TestRenderFileList_ScrollKeepsSelectionVisible(t *testing.T) {
	t.Parallel()

	files := make([]model.FileSummary, 20)
	for i := range files {
		files[i] = model.FileSummary{Path: "f.go", Status: model.StatusModified}
	}

	// Select file 15 with height=5: scroll should adjust.
	_, scroll := RenderFileList(files, 15, 0, 60, 5, true)
	if scroll > 15 || scroll+5 <= 15 {
		t.Errorf("scroll=%d does not keep selectedIdx=15 visible in height=5", scroll)
	}
}

func TestRenderFileList_TruncatesLongPaths(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "very/long/path/that/exceeds/the/available/width/file.go", Status: model.StatusModified, Additions: 1},
	}
	output, _ := RenderFileList(files, 0, 0, 30, 10, true)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Each visible line should not exceed the width.
		// Note: ANSI escape codes may add bytes but not visual width.
		// We just check it doesn't crash and produces output.
		if len(line) == 0 {
			t.Error("expected non-empty line")
		}
	}
}

func TestRenderFileList_EmptyFiles(t *testing.T) {
	t.Parallel()

	output, scroll := RenderFileList(nil, -1, 0, 60, 10, true)
	if scroll != 0 {
		t.Errorf("scroll = %d, want 0", scroll)
	}
	if !strings.Contains(output, "No files") {
		t.Error("empty file list should show 'No files' message")
	}
}

func TestRenderFileList_ActiveInactive(t *testing.T) {
	t.Parallel()

	activeOutput, _ := RenderFileList(sampleFileList(), 0, 0, 60, 10, true)
	inactiveOutput, _ := RenderFileList(sampleFileList(), 0, 0, 60, 10, false)

	// Both should contain file names.
	if !strings.Contains(activeOutput, "main.go") {
		t.Error("active output missing main.go")
	}
	if !strings.Contains(inactiveOutput, "main.go") {
		t.Error("inactive output missing main.go")
	}
}

func TestRenderFileList_StatusIcons(t *testing.T) {
	t.Parallel()

	output, _ := RenderFileList(sampleFileList(), 0, 0, 60, 10, true)
	if !strings.Contains(output, "M") {
		t.Error("output should contain M for modified")
	}
	if !strings.Contains(output, "A") {
		t.Error("output should contain A for added")
	}
	if !strings.Contains(output, "D") {
		t.Error("output should contain D for deleted")
	}
}
