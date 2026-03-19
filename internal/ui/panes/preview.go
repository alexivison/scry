package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
)

// RenderPreview renders a compact file list for the dashboard preview pane.
// Shows up to 5 files with status icon and +/- counts.
func RenderPreview(files []model.FileSummary, width, height int) string {
	if len(files) == 0 {
		return "No changed files."
	}

	lines := make([]string, 0, len(files))
	for _, f := range files {
		if len(lines) >= height {
			break
		}
		icon := StatusIcon(f.Status)
		counts := FormatCounts(f)
		path := f.Path
		// Reserve: icon(1) + gap(1) + counts + gap(1) + path
		pathBudget := width - 1 - 1 - lipgloss.Width(counts) - 1
		if pathBudget < 5 {
			pathBudget = 5
		}
		if len(path) > pathBudget {
			path = truncatePath(path, pathBudget)
		}
		line := fmt.Sprintf("%s %s %s", icon, fmt.Sprintf("%-*s", pathBudget, path), counts)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
