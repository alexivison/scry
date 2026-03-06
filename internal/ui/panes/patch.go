// Package panes implements individual UI pane components for scry.
package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
)

// PatchViewport tracks scroll position and hunk navigation for a loaded patch.
type PatchViewport struct {
	Patch        model.FilePatch
	CurrentHunk  int
	ScrollOffset int // line index at top of viewport
	Width        int
	Height       int

	// Pre-computed flat line list for rendering.
	lines []patchLine
}

type lineType int

const (
	lineTypeHunkHeader lineType = iota
	lineTypeDiff
)

type patchLine struct {
	typ    lineType
	header string         // only for hunkHeader
	diff   model.DiffLine // only for diff lines
}

// NewPatchViewport creates a viewport positioned at the first hunk.
func NewPatchViewport(patch model.FilePatch) *PatchViewport {
	vp := &PatchViewport{Patch: patch}
	vp.lines = vp.buildLines()
	return vp
}

func (vp *PatchViewport) buildLines() []patchLine {
	var lines []patchLine
	for _, h := range vp.Patch.Hunks {
		lines = append(lines, patchLine{typ: lineTypeHunkHeader, header: formatHunkHeader(h)})
		for _, dl := range h.Lines {
			lines = append(lines, patchLine{typ: lineTypeDiff, diff: dl})
		}
	}
	return lines
}

func formatHunkHeader(h model.Hunk) string {
	s := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldLen, h.NewStart, h.NewLen)
	if h.Header != "" {
		s += " " + h.Header
	}
	return s
}

// hunkLineOffset returns the line index of a given hunk's header.
func (vp *PatchViewport) hunkLineOffset(hunk int) int {
	if hunk <= 0 || len(vp.Patch.Hunks) == 0 {
		return 0
	}
	offset := 0
	for i := 0; i < hunk && i < len(vp.Patch.Hunks); i++ {
		offset += 1 + len(vp.Patch.Hunks[i].Lines) // header + lines
	}
	return offset
}

// NextHunk advances to the next hunk. No-op at the last hunk.
func (vp *PatchViewport) NextHunk() {
	if len(vp.Patch.Hunks) == 0 || vp.CurrentHunk >= len(vp.Patch.Hunks)-1 {
		return
	}
	vp.CurrentHunk++
	vp.ScrollOffset = vp.hunkLineOffset(vp.CurrentHunk)
}

// PrevHunk moves to the previous hunk. No-op at the first hunk.
func (vp *PatchViewport) PrevHunk() {
	if len(vp.Patch.Hunks) == 0 || vp.CurrentHunk <= 0 {
		return
	}
	vp.CurrentHunk--
	vp.ScrollOffset = vp.hunkLineOffset(vp.CurrentHunk)
}

// Render produces the visible portion of the patch for the current viewport.
func (vp *PatchViewport) Render() string {
	if len(vp.Patch.Hunks) == 0 {
		return "No changes."
	}

	if vp.Height <= 0 {
		return ""
	}

	end := vp.ScrollOffset + vp.Height
	if end > len(vp.lines) {
		end = len(vp.lines)
	}
	start := vp.ScrollOffset
	if start > len(vp.lines) {
		start = len(vp.lines)
	}

	visible := vp.lines[start:end]
	rendered := make([]string, 0, len(visible))
	for _, pl := range visible {
		switch pl.typ {
		case lineTypeHunkHeader:
			rendered = append(rendered, hunkHeaderStyle.Render(pl.header))
		case lineTypeDiff:
			rendered = append(rendered, renderDiffLine(pl.diff, vp.Width))
		}
	}
	return strings.Join(rendered, "\n")
}

func renderDiffLine(dl model.DiffLine, width int) string {
	if dl.Kind == model.LineNoNewline {
		return noNewlineStyle.Render("\\ No newline at end of file")
	}

	prefix, style := diffLineStyle(dl.Kind)
	gutter := formatGutter(dl.OldNo, dl.NewNo)
	line := gutter + prefix + dl.Text

	if width > 0 && lipgloss.Width(line) > width {
		// Truncate to fit; we don't wrap.
		line = line[:width]
	}

	return style.Render(line)
}

func diffLineStyle(kind model.LineKind) (string, lipgloss.Style) {
	switch kind {
	case model.LineAdded:
		return "+", addedStyle
	case model.LineDeleted:
		return "-", deletedStyle
	case model.LineContext:
		return " ", contextStyle
	default:
		return " ", contextStyle
	}
}

func formatGutter(oldNo, newNo *int) string {
	old := "    "
	if oldNo != nil {
		old = fmt.Sprintf("%4d", *oldNo)
	}
	new := "    "
	if newNo != nil {
		new = fmt.Sprintf("%4d", *newNo)
	}
	return old + " " + new + " "
}

// TotalLines returns the total number of rendered lines (headers + diff lines).
func (vp *PatchViewport) TotalLines() int {
	return len(vp.lines)
}

// Styles for patch rendering.
var (
	hunkHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	addedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	deletedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))

	contextStyle = lipgloss.NewStyle()

	noNewlineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
)
