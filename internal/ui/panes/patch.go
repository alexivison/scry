// Package panes implements individual UI pane components for scry.
package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/theme"
)

// PatchViewport tracks scroll position and hunk navigation for a loaded patch.
type PatchViewport struct {
	Patch        model.FilePatch
	CurrentHunk  int
	ScrollOffset int // line index at top of viewport
	Width        int
	Height       int

	SearchQuery   string // current search query for highlighting
	MatchLine     int    // viewport line of the current match (-1 = none)
	GutterVisible bool   // when false, suppress line number gutter (minimal mode)

	// Pre-computed flat line list for rendering.
	lines        []patchLine
	gutterDigits int // width of each line-number column (min 4)
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
	vp := &PatchViewport{Patch: patch, GutterVisible: true}
	vp.lines = vp.buildLines()
	vp.gutterDigits = vp.computeGutterDigits()
	return vp
}

// computeGutterDigits returns the number of digits needed for the largest
// line number in the patch (minimum 4 for visual consistency).
func (vp *PatchViewport) computeGutterDigits() int {
	maxLine := 0
	for _, h := range vp.Patch.Hunks {
		for _, dl := range h.Lines {
			if dl.OldNo != nil && *dl.OldNo > maxLine {
				maxLine = *dl.OldNo
			}
			if dl.NewNo != nil && *dl.NewNo > maxLine {
				maxLine = *dl.NewNo
			}
		}
	}
	digits := 4
	for n := maxLine; n >= 10000; n /= 10 {
		digits++
	}
	return digits
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

// ScrollDown moves the viewport one line down. No-op at the bottom.
func (vp *PatchViewport) ScrollDown() {
	if vp.ScrollOffset < len(vp.lines)-1 {
		vp.ScrollOffset++
		vp.SyncCurrentHunk()
	}
}

// ScrollUp moves the viewport one line up. No-op at the top.
func (vp *PatchViewport) ScrollUp() {
	if vp.ScrollOffset > 0 {
		vp.ScrollOffset--
		vp.SyncCurrentHunk()
	}
}

// PageDown moves the viewport one full page down.
func (vp *PatchViewport) PageDown() {
	vp.ScrollOffset += vp.Height
	if vp.ScrollOffset > len(vp.lines)-1 {
		vp.ScrollOffset = len(vp.lines) - 1
	}
	vp.SyncCurrentHunk()
}

// PageUp moves the viewport one full page up.
func (vp *PatchViewport) PageUp() {
	vp.ScrollOffset -= vp.Height
	if vp.ScrollOffset < 0 {
		vp.ScrollOffset = 0
	}
	vp.SyncCurrentHunk()
}

// HalfPageDown moves the viewport half a page down.
func (vp *PatchViewport) HalfPageDown() {
	vp.ScrollOffset += vp.Height / 2
	if vp.ScrollOffset > len(vp.lines)-1 {
		vp.ScrollOffset = len(vp.lines) - 1
	}
	vp.SyncCurrentHunk()
}

// HalfPageUp moves the viewport half a page up.
func (vp *PatchViewport) HalfPageUp() {
	vp.ScrollOffset -= vp.Height / 2
	if vp.ScrollOffset < 0 {
		vp.ScrollOffset = 0
	}
	vp.SyncCurrentHunk()
}

// ScrollToTop jumps to the beginning of the patch.
func (vp *PatchViewport) ScrollToTop() {
	vp.ScrollOffset = 0
	vp.CurrentHunk = 0
}

// ScrollToBottom jumps to the end of the patch.
func (vp *PatchViewport) ScrollToBottom() {
	max := len(vp.lines) - vp.Height
	if max < 0 {
		max = 0
	}
	vp.ScrollOffset = max
	vp.SyncCurrentHunk()
}

// SyncCurrentHunk derives CurrentHunk from ScrollOffset so that n/p
// navigate relative to the hunk the user is actually viewing.
func (vp *PatchViewport) SyncCurrentHunk() {
	for i := len(vp.Patch.Hunks) - 1; i >= 0; i-- {
		if vp.ScrollOffset >= vp.hunkLineOffset(i) {
			vp.CurrentHunk = i
			return
		}
	}
	vp.CurrentHunk = 0
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
	for i, pl := range visible {
		absLine := start + i
		switch pl.typ {
		case lineTypeHunkHeader:
			rendered = append(rendered, renderHunkSeparator(pl.header, vp.Width))
		case lineTypeDiff:
			isMatch := vp.SearchQuery != "" && absLine == vp.MatchLine
			rendered = append(rendered, renderDiffLineHL(pl.diff, vp.Width, vp.SearchQuery, isMatch, vp.GutterVisible, vp.gutterDigits))
		}
	}
	return strings.Join(rendered, "\n")
}

func renderDiffLineHL(dl model.DiffLine, width int, query string, highlight bool, gutterVisible bool, gutterDigits int) string {
	if dl.Kind == model.LineNoNewline {
		return noNewlineStyle.Render("\\ No newline at end of file")
	}

	prefix, style := diffLineStyle(dl.Kind)
	body := prefix + dl.Text

	if gutterVisible {
		gutter := formatGutter(dl.OldNo, dl.NewNo, gutterDigits)
		if width > 0 {
			bodyBudget := width - lipgloss.Width(gutter)
			if bodyBudget > 0 && lipgloss.Width(body) > bodyBudget {
				body = truncateToWidth(body, bodyBudget)
			}
		}
		if highlight && query != "" {
			return gutter + highlightMatch(body, query, style)
		}
		return gutter + style.Render(body)
	}

	if width > 0 && lipgloss.Width(body) > width {
		body = truncateToWidth(body, width)
	}

	if highlight && query != "" {
		return highlightMatch(body, query, style)
	}

	return style.Render(body)
}

func highlightMatch(line, query string, baseStyle lipgloss.Style) string {
	caseSensitive := strings.ToLower(query) != query
	var idx int
	if caseSensitive {
		idx = strings.Index(line, query)
	} else {
		idx = strings.Index(strings.ToLower(line), strings.ToLower(query))
	}
	if idx < 0 {
		return baseStyle.Render(line)
	}
	hlStyle := baseStyle.Reverse(true)
	before := line[:idx]
	match := line[idx : idx+len(query)]
	after := line[idx+len(query):]
	return baseStyle.Render(before) + hlStyle.Render(match) + baseStyle.Render(after)
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

func formatGutter(oldNo, newNo *int, digits int) string {
	old := strings.Repeat(" ", digits)
	if oldNo != nil {
		old = fmt.Sprintf("%*d", digits, *oldNo)
	}
	new := strings.Repeat(" ", digits)
	if newNo != nil {
		new = fmt.Sprintf("%*d", digits, *newNo)
	}
	return gutterStyle.Render(old+" "+new) + gutterStyle.Render(" │") + " "
}

// renderHunkSeparator renders a hunk header as a horizontal rule with the @@ text embedded.
// Example: ─── @@ -10,3 +11,4 @@ func main() ───────
func renderHunkSeparator(header string, width int) string {
	prefix := "── "
	middle := header
	suffix := " "

	core := prefix + middle + suffix
	coreW := lipgloss.Width(core)

	if width > 0 && coreW >= width {
		return hunkHeaderStyle.Render(truncateToWidth(core, width))
	}

	// Fill remaining width with ─.
	remaining := 0
	if width > 0 {
		remaining = width - coreW
	}
	line := core + strings.Repeat("─", remaining)

	return hunkHeaderStyle.Render(line)
}

// ScrollIndicatorPos returns the scroll position as a ratio (0.0–1.0) for
// rendering a scroll indicator on the border edge.
func (vp *PatchViewport) ScrollIndicatorPos() float64 {
	total := len(vp.lines)
	if total <= vp.Height || total == 0 {
		return 0
	}
	maxScroll := total - vp.Height
	if vp.ScrollOffset >= maxScroll {
		return 1.0
	}
	return float64(vp.ScrollOffset) / float64(maxScroll)
}

// truncateToWidth trims a string to fit within a terminal-cell width budget.
// Uses ANSI-aware truncation to preserve escape sequences.
func truncateToWidth(s string, maxWidth int) string {
	return ansi.Truncate(s, maxWidth, "")
}

// TotalLines returns the total number of rendered lines (headers + diff lines).
func (vp *PatchViewport) TotalLines() int {
	return len(vp.lines)
}

// IsHunkHeader reports whether the given viewport line is a hunk header.
func (vp *PatchViewport) IsHunkHeader(vpLine int) bool {
	if vpLine < 0 || vpLine >= len(vp.lines) {
		return false
	}
	return vp.lines[vpLine].typ == lineTypeHunkHeader
}

// DiffLineToViewportLine converts a DiffLine index (0-based across all hunks,
// headers excluded) to the corresponding viewport line index (headers included).
func (vp *PatchViewport) DiffLineToViewportLine(diffIdx int) int {
	count := 0
	for i, pl := range vp.lines {
		if pl.typ == lineTypeDiff {
			if count == diffIdx {
				return i
			}
			count++
		}
	}
	return 0
}

// ViewportLineToDiffLine converts a viewport line index to the DiffLine index.
// If the viewport line is a hunk header, it returns the index of the next DiffLine.
func (vp *PatchViewport) ViewportLineToDiffLine(vpLine int) int {
	if vpLine < 0 {
		vpLine = 0
	}
	if vpLine >= len(vp.lines) {
		vpLine = len(vp.lines) - 1
	}
	count := 0
	for i, pl := range vp.lines {
		if pl.typ == lineTypeDiff {
			if i >= vpLine {
				return count
			}
			count++
		}
	}
	if count > 0 {
		return count - 1
	}
	return 0
}

// Styles for patch rendering.
var (
	hunkHeaderStyle = lipgloss.NewStyle().
			Foreground(theme.HunkHeader).
			Bold(true)

	addedStyle = lipgloss.NewStyle().
			Foreground(theme.Added)

	deletedStyle = lipgloss.NewStyle().
			Foreground(theme.Deleted)

	contextStyle = lipgloss.NewStyle()

	noNewlineStyle = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true)

	gutterStyle = lipgloss.NewStyle().
			Foreground(theme.Muted)
)
