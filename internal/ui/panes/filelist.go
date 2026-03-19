package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
)

var (
	fileSelectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	fileDimStyle      = lipgloss.NewStyle().Faint(true)
)

// RenderFileList renders a scrollable file list constrained to the given dimensions.
// It adjusts scrollOffset to keep selectedIdx visible and returns the rendered
// string along with the new scroll offset.
func RenderFileList(files []model.FileSummary, selectedIdx, scrollOffset, width, height int, active bool) (string, int) {
	if len(files) == 0 {
		return "No files changed.", 0
	}

	// Ensure selected item is visible.
	scrollOffset = EnsureVisible(selectedIdx, scrollOffset, height, len(files))

	// Determine visible window.
	end := scrollOffset + height
	if end > len(files) {
		end = len(files)
	}

	lines := make([]string, 0, end-scrollOffset)
	for i := scrollOffset; i < end; i++ {
		line := renderFileEntry(files[i], i, selectedIdx, width)
		if !active {
			line = fileDimStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), scrollOffset
}

// EnsureVisible adjusts scrollOffset so selectedIdx is within the visible window.
func EnsureVisible(selectedIdx, scrollOffset, height, total int) int {
	if selectedIdx < 0 || total == 0 {
		return 0
	}
	if selectedIdx < scrollOffset {
		return selectedIdx
	}
	if selectedIdx >= scrollOffset+height {
		return selectedIdx - height + 1
	}
	return scrollOffset
}

func renderFileEntry(f model.FileSummary, idx, selectedIdx, width int) string {
	status := StatusIcon(f.Status)
	path := f.Path
	if f.OldPath != "" {
		path = fmt.Sprintf("%s → %s", f.OldPath, f.Path)
	}
	counts := FormatCounts(f)

	prefix := "  "
	if idx == selectedIdx {
		prefix = "> "
	}

	// Reserve space: prefix(2) + status(1) + gap(2) + counts(~8) + gap(1) = ~14.
	pathWidth := width - 2 - 1 - 2 - len(counts) - 1
	if pathWidth < 5 {
		pathWidth = 5
	}

	// Truncate or pad path to fixed width.
	if lipgloss.Width(path) > pathWidth {
		path = truncatePath(path, pathWidth)
	}

	line := fmt.Sprintf("%s%s  %-*s %s", prefix, status, pathWidth, path, counts)

	if idx == selectedIdx {
		return fileSelectedStyle.Render(line)
	}
	return line
}

// truncatePath trims a path to fit within maxWidth, adding "…" as ellipsis.
func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 1 {
		return "…"
	}
	// Walk runes until we'd exceed maxWidth-1 (leaving room for …).
	w := 0
	for i, r := range path {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth-1 {
			return path[:i] + "…"
		}
		w += rw
	}
	return path
}

// StatusIcon returns a single-character icon for a file status.
func StatusIcon(s model.FileStatus) string {
	switch s {
	case model.StatusAdded:
		return "A"
	case model.StatusModified:
		return "M"
	case model.StatusDeleted:
		return "D"
	case model.StatusRenamed:
		return "R"
	case model.StatusCopied:
		return "C"
	case model.StatusTypeChg:
		return "T"
	case model.StatusUnmerged:
		return "U"
	case model.StatusUntracked:
		return "?"
	default:
		return "?"
	}
}

// FormatCounts formats addition/deletion counts for display.
func FormatCounts(f model.FileSummary) string {
	if f.IsBinary {
		return "binary"
	}
	return fmt.Sprintf("+%d -%d", f.Additions, f.Deletions)
}
