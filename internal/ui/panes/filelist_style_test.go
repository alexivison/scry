package panes

import (
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func TestStatusIcon_Colored(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		status model.FileStatus
		letter string
	}{
		"added":    {model.StatusAdded, "A"},
		"deleted":  {model.StatusDeleted, "D"},
		"modified": {model.StatusModified, "M"},
		"renamed":  {model.StatusRenamed, "R"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			icon := RenderIcon(tc.status, false)
			if !strings.Contains(icon, tc.letter) {
				t.Errorf("RenderIcon(%v) should contain %q, got %q", tc.status, tc.letter, icon)
			}
		})
	}
}

func TestFormatCounts_Colored(t *testing.T) {
	t.Parallel()

	f := model.FileSummary{Additions: 5, Deletions: 3}
	out := RenderCounts(f, false)
	if !strings.Contains(out, "+5") {
		t.Errorf("RenderCounts should contain '+5', got %q", out)
	}
	if !strings.Contains(out, "-3") {
		t.Errorf("RenderCounts should contain '-3', got %q", out)
	}
}

func TestFormatCounts_Binary(t *testing.T) {
	t.Parallel()

	f := model.FileSummary{IsBinary: true}
	out := RenderCounts(f, false)
	if !strings.Contains(out, "binary") {
		t.Errorf("RenderCounts should contain 'binary', got %q", out)
	}
}

func TestSelection_ReverseOnly(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 1, Deletions: 1},
	}
	output, _ := RenderFileList(files, 0, 0, 60, 10, true)
	// The output should contain the file — basic check that rendering works.
	if !strings.Contains(output, "main.go") {
		t.Error("output should contain main.go")
	}
}
