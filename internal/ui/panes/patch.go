// Package panes implements individual UI pane components for scry.
package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/syntax"
	"github.com/alexivison/scry/internal/ui/theme"
)

// PatchViewport tracks scroll position and hunk navigation for a loaded patch.
type PatchViewport struct {
	Patch        model.FilePatch
	CurrentHunk  int
	ScrollOffset int // line index at top of viewport
	LineMode     model.LineMode
	XOffset      int // horizontal body-column offset in scroll mode
	Width        int
	Height       int

	SearchQuery   string      // current search query for highlighting
	SearchMatch   SearchMatch // current match anchored to a logical diff line
	GutterVisible bool        // when false, suppress line number gutter (minimal mode)

	// Pre-computed flat line list for rendering.
	lines             []patchLine
	gutterDigits      int // width of each line-number column (min 4)
	syntaxHighlighted *syntax.LineCache
}

const horizontalScrollStep = 8

type lineType int

const (
	lineTypeHunkHeader lineType = iota
	lineTypeDiff
)

type patchLine struct {
	typ       lineType
	header    string         // only for hunkHeader
	diff      model.DiffLine // only for diff lines
	diffIndex int            // logical diff line index, headers excluded
}

// SearchMatch identifies the current search match in logical diff-line space.
type SearchMatch struct {
	Line  int
	Start int
	End   int
}

// NoSearchMatch returns the sentinel used when no search match is active.
func NoSearchMatch() SearchMatch {
	return SearchMatch{Line: -1, Start: -1, End: -1}
}

// NewPatchViewport creates a viewport positioned at the first hunk.
func NewPatchViewport(patch model.FilePatch) *PatchViewport {
	vp := &PatchViewport{Patch: patch, LineMode: model.LineModeScroll, SearchMatch: NoSearchMatch(), GutterVisible: true}
	vp.lines = vp.buildLines()
	vp.gutterDigits = vp.computeGutterDigits()
	return vp
}

// SetSyntaxHighlighter enables body-only syntax highlighting for diff lines.
func (vp *PatchViewport) SetSyntaxHighlighter(lines *syntax.LineCache) {
	vp.syntaxHighlighted = lines
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
	diffIndex := 0
	for _, h := range vp.Patch.Hunks {
		lines = append(lines, patchLine{typ: lineTypeHunkHeader, header: formatHunkHeader(h), diffIndex: -1})
		for _, dl := range h.Lines {
			lines = append(lines, patchLine{typ: lineTypeDiff, diff: dl, diffIndex: diffIndex})
			diffIndex++
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
	if len(vp.lines) == 0 {
		return
	}
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
	if len(vp.lines) == 0 {
		return
	}
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

// ScrollRight moves the visible code body right by the fixed horizontal step.
func (vp *PatchViewport) ScrollRight() {
	if vp.LineMode != model.LineModeScroll {
		return
	}
	vp.XOffset += horizontalScrollStep
	vp.ClampXOffset()
}

// ScrollLeft moves the visible code body left by the fixed horizontal step.
func (vp *PatchViewport) ScrollLeft() {
	if vp.LineMode != model.LineModeScroll {
		return
	}
	vp.XOffset -= horizontalScrollStep
	vp.ClampXOffset()
}

// ResetXOffset returns horizontal scroll to the beginning of the code body.
func (vp *PatchViewport) ResetXOffset() {
	vp.XOffset = 0
}

// ClampXOffset constrains horizontal scroll to the currently visible body range.
func (vp *PatchViewport) ClampXOffset() {
	if vp.XOffset < 0 {
		vp.XOffset = 0
		return
	}
	if max := vp.MaxXOffset(); vp.XOffset > max {
		vp.XOffset = max
	}
}

// MaxXOffset returns the largest useful horizontal body offset for visible rows.
func (vp *PatchViewport) MaxXOffset() int {
	if vp.LineMode != model.LineModeScroll || vp.Width <= 0 {
		return 0
	}

	bodyWidth := -1
	longestBody := 0
	for _, pl := range vp.visibleLines() {
		if pl.typ != lineTypeDiff || pl.diff.Kind == model.LineNoNewline {
			continue
		}
		layers := buildDiffLineLayers(pl.diff, vp.GutterVisible, vp.gutterDigits)
		if bodyWidth < 0 {
			bodyWidth = diffLineBodyWidth(layers, vp.Width)
		}
		if w := lipgloss.Width(layers.body); w > longestBody {
			longestBody = w
		}
	}
	if bodyWidth <= 0 {
		return 0
	}
	max := longestBody - bodyWidth
	if max < 0 {
		return 0
	}
	return max
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

	vp.ClampXOffset()
	visible := vp.visibleLines()
	rendered := make([]string, 0, len(visible))
	for _, pl := range visible {
		switch pl.typ {
		case lineTypeHunkHeader:
			rendered = append(rendered, renderHunkSeparator(pl.header, vp.Width))
		case lineTypeDiff:
			match := NoSearchMatch()
			if vp.SearchQuery != "" && vp.SearchMatch.Line == pl.diffIndex {
				match = vp.SearchMatch
			}
			rendered = append(rendered, vp.renderDiffLine(pl.diff, pl.diffIndex, match))
		}
	}
	return strings.Join(rendered, "\n")
}

func (vp *PatchViewport) visibleLines() []patchLine {
	end := vp.ScrollOffset + vp.Height
	if end > len(vp.lines) {
		end = len(vp.lines)
	}
	start := vp.ScrollOffset
	if start > len(vp.lines) {
		start = len(vp.lines)
	}

	if start > end {
		start = end
	}
	return vp.lines[start:end]
}

func (vp *PatchViewport) renderDiffLine(dl model.DiffLine, diffIndex int, match SearchMatch) string {
	return renderDiffLineBodyHL(
		dl,
		vp.highlightBody(dl, diffIndex, match),
		vp.syntaxHighlighted != nil && syntax.Highlightable(dl.Kind),
		vp.Width,
		vp.SearchQuery,
		match,
		vp.GutterVisible,
		vp.gutterDigits,
		vp.LineMode,
		vp.XOffset,
	)
}

func (vp *PatchViewport) highlightBody(dl model.DiffLine, diffIndex int, match SearchMatch) string {
	if vp.syntaxHighlighted == nil || !syntax.Highlightable(dl.Kind) {
		return dl.Text
	}
	if vp.SearchQuery != "" && match.validForBody(dl.Text) {
		return vp.syntaxHighlighted.HighlightLineSpan(diffIndex, dl.Text, match.Start, match.End)
	}
	return vp.syntaxHighlighted.HighlightLine(diffIndex, dl.Text)
}

type diffLineLayers struct {
	gutter string
	prefix string
	body   string
	style  lipgloss.Style

	bodyStyled bool
}

func buildDiffLineLayers(dl model.DiffLine, gutterVisible bool, gutterDigits int) diffLineLayers {
	prefix, style := diffLineStyle(dl.Kind)
	layers := diffLineLayers{
		prefix: prefix,
		body:   dl.Text,
		style:  style,
	}
	if gutterVisible {
		layers.gutter = formatGutter(dl.OldNo, dl.NewNo, gutterDigits)
	}
	return layers
}

func renderDiffLineHL(dl model.DiffLine, width int, query string, match SearchMatch, gutterVisible bool, gutterDigits int, lineMode model.LineMode, xOffset int) string {
	return renderDiffLineBodyHL(dl, dl.Text, false, width, query, match, gutterVisible, gutterDigits, lineMode, xOffset)
}

func renderDiffLineBodyHL(dl model.DiffLine, body string, bodyStyled bool, width int, query string, match SearchMatch, gutterVisible bool, gutterDigits int, lineMode model.LineMode, xOffset int) string {
	if dl.Kind == model.LineNoNewline {
		return noNewlineStyle.Render("\\ No newline at end of file")
	}

	layers := buildDiffLineLayers(dl, gutterVisible, gutterDigits)
	layers.body = body
	layers.bodyStyled = bodyStyled
	return emitDiffLine(layers, width, query, match, lineMode, xOffset)
}

func emitDiffLine(layers diffLineLayers, width int, query string, match SearchMatch, lineMode model.LineMode, xOffset int) string {
	if lineMode != model.LineModeScroll || xOffset < 0 {
		xOffset = 0
	}

	visibleBody := layers.body
	if width > 0 {
		bodyWidth := diffLineBodyWidth(layers, width)
		if bodyWidth <= 0 {
			visibleBody = ""
		} else {
			if xOffset > 0 {
				visibleBody = ansi.TruncateLeft(visibleBody, xOffset, "")
			}
			visibleBody = ansi.Truncate(visibleBody, bodyWidth, "")
		}
	}

	body := layers.prefix + visibleBody
	if layers.bodyStyled {
		body = layers.style.Render(layers.prefix) + visibleBody
	}

	if layers.gutter != "" {
		if !layers.bodyStyled && query != "" && match.validForBody(layers.body) {
			return layers.gutter + highlightSpan(body, bodySpanToVisibleContentSpan(match, layers.prefix, xOffset, visibleBody), layers.style)
		}
		if layers.bodyStyled {
			return layers.gutter + body
		}
		return layers.gutter + layers.style.Render(body)
	}

	if !layers.bodyStyled && query != "" && match.validForBody(layers.body) {
		return highlightSpan(body, bodySpanToVisibleContentSpan(match, layers.prefix, xOffset, visibleBody), layers.style)
	}

	if layers.bodyStyled {
		return body
	}
	return layers.style.Render(body)
}

func diffLineBodyWidth(layers diffLineLayers, width int) int {
	bodyWidth := width - lipgloss.Width(layers.gutter) - lipgloss.Width(layers.prefix)
	if bodyWidth < 0 {
		return 0
	}
	return bodyWidth
}

func (m SearchMatch) validForBody(body string) bool {
	return m.Line >= 0 && m.Start >= 0 && m.End > m.Start && m.End <= len(body)
}

func bodySpanToVisibleContentSpan(match SearchMatch, prefix string, xOffset int, visibleBody string) SearchMatch {
	prefixLen := len(prefix)
	start := match.Start - xOffset
	end := match.End - xOffset
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(visibleBody) {
		end = len(visibleBody)
	}
	return SearchMatch{
		Line:  match.Line,
		Start: prefixLen + start,
		End:   prefixLen + end,
	}
}

func highlightSpan(line string, span SearchMatch, baseStyle lipgloss.Style) string {
	if span.Start < 0 || span.End < span.Start || span.End > len(line) {
		return baseStyle.Render(line)
	}
	hlStyle := baseStyle.Reverse(true)
	return baseStyle.Render(line[:span.Start]) + hlStyle.Render(line[span.Start:span.End]) + baseStyle.Render(line[span.End:])
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
	for i, pl := range vp.lines {
		if pl.typ == lineTypeDiff && pl.diffIndex == diffIdx {
			return i
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
	lastDiff := 0
	for i, pl := range vp.lines {
		if pl.typ == lineTypeDiff {
			if i >= vpLine {
				return pl.diffIndex
			}
			lastDiff = pl.diffIndex
		}
	}
	return lastDiff
}

// CurrentTargetLine returns the most relevant new-file line number for the
// current viewport position or hunk, for opening the file in an editor.
func (vp *PatchViewport) CurrentTargetLine() (int, bool) {
	if len(vp.Patch.Hunks) == 0 || len(vp.lines) == 0 {
		return 0, false
	}

	hunk := vp.CurrentHunk
	if hunk < 0 {
		hunk = 0
	}
	if hunk >= len(vp.Patch.Hunks) {
		hunk = len(vp.Patch.Hunks) - 1
	}

	start := vp.ScrollOffset
	hunkStart := vp.hunkLineOffset(hunk)
	if start < hunkStart {
		start = hunkStart
	}
	hunkEnd := len(vp.lines)
	if hunk+1 < len(vp.Patch.Hunks) {
		hunkEnd = vp.hunkLineOffset(hunk + 1)
	}

	for i := start; i < hunkEnd; i++ {
		if vp.lines[i].typ == lineTypeDiff && vp.lines[i].diff.NewNo != nil {
			return *vp.lines[i].diff.NewNo, true
		}
	}
	for _, line := range vp.Patch.Hunks[hunk].Lines {
		if line.NewNo != nil {
			return *line.NewNo, true
		}
	}
	return 0, false
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
