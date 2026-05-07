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
	hunkIndex int
}

type visualRow struct {
	line         patchLine
	segment      bodySegment
	continuation bool
}

type bodySegment struct {
	text         string
	start        int
	end          int
	displayStart int
	displayEnd   int
	index        int
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
	vp := &PatchViewport{Patch: patch, LineMode: model.LineModeWrap, SearchMatch: NoSearchMatch(), GutterVisible: true}
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
	for hunkIndex, h := range vp.Patch.Hunks {
		lines = append(lines, patchLine{typ: lineTypeHunkHeader, header: formatHunkHeader(h), diffIndex: -1, hunkIndex: hunkIndex})
		for _, dl := range h.Lines {
			lines = append(lines, patchLine{typ: lineTypeDiff, diff: dl, diffIndex: diffIndex, hunkIndex: hunkIndex})
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
	for i, row := range vp.visualRows() {
		if row.line.typ == lineTypeHunkHeader && row.line.hunkIndex == hunk {
			return i
		}
	}
	return 0
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
	if vp.ScrollOffset < vp.TotalLines()-1 {
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
	total := vp.TotalLines()
	if total == 0 {
		return
	}
	vp.ScrollOffset += vp.Height
	if vp.ScrollOffset > total-1 {
		vp.ScrollOffset = total - 1
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
	total := vp.TotalLines()
	if total == 0 {
		return
	}
	vp.ScrollOffset += vp.Height / 2
	if vp.ScrollOffset > total-1 {
		vp.ScrollOffset = total - 1
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
	max := vp.TotalLines() - vp.Height
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
	if vp.LineMode != model.LineModeScroll {
		return
	}
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
	for _, row := range vp.visibleRows() {
		pl := row.line
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

// SetLineMode switches between wrap and horizontal-scroll layout while keeping
// the same logical diff line at the top of the viewport when possible.
func (vp *PatchViewport) SetLineMode(mode model.LineMode) {
	if vp.LineMode == mode {
		if mode == model.LineModeScroll {
			vp.XOffset = 0
		}
		return
	}

	diffLine, rowOffset, ok := vp.ScrollAnchor()
	vp.LineMode = mode
	if mode == model.LineModeScroll {
		vp.XOffset = 0
	}
	if ok {
		if offset, found := vp.ScrollOffsetForDiffLine(diffLine, rowOffset); found {
			vp.ScrollOffset = offset
		}
	}
	vp.clampScrollOffset()
	vp.SyncCurrentHunk()
}

// ScrollAnchor returns the logical diff line and wrapped-row offset currently
// at the top of the viewport. Hunk headers do not have a diff-line anchor.
func (vp *PatchViewport) ScrollAnchor() (diffLine int, rowOffset int, ok bool) {
	rows := vp.visualRows()
	if len(rows) == 0 {
		return 0, 0, false
	}
	idx := vp.ScrollOffset
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rows) {
		idx = len(rows) - 1
	}
	row := rows[idx]
	if row.line.typ != lineTypeDiff {
		return 0, 0, false
	}
	return row.line.diffIndex, row.segment.index, true
}

// ScrollOffsetForDiffLine returns the visual row offset for a logical diff line
// and wrapped-row offset. If the requested row no longer exists, it clamps to
// the last visual row for that logical line.
func (vp *PatchViewport) ScrollOffsetForDiffLine(diffLine int, rowOffset int) (int, bool) {
	last := -1
	for i, row := range vp.visualRows() {
		if row.line.typ != lineTypeDiff || row.line.diffIndex != diffLine {
			continue
		}
		last = i
		if row.segment.index >= rowOffset {
			return i, true
		}
	}
	if last >= 0 {
		return last, true
	}
	return 0, false
}

func (vp *PatchViewport) clampScrollOffset() {
	total := vp.TotalLines()
	if total == 0 {
		vp.ScrollOffset = 0
		return
	}
	if vp.ScrollOffset < 0 {
		vp.ScrollOffset = 0
		return
	}
	if vp.ScrollOffset >= total {
		vp.ScrollOffset = total - 1
	}
}

// Render produces the visible portion of the patch for the current viewport.
func (vp *PatchViewport) Render() string {
	if len(vp.Patch.Hunks) == 0 {
		return "No changes."
	}

	if vp.Height <= 0 {
		return ""
	}

	vp.clampScrollOffset()
	vp.ClampXOffset()
	visible := vp.visibleRows()
	rendered := make([]string, 0, len(visible))
	for _, row := range visible {
		switch row.line.typ {
		case lineTypeHunkHeader:
			rendered = append(rendered, renderHunkSeparator(row.line.header, vp.Width))
		case lineTypeDiff:
			match := NoSearchMatch()
			if vp.SearchQuery != "" && vp.SearchMatch.Line == row.line.diffIndex {
				match = vp.SearchMatch
			}
			rendered = append(rendered, vp.renderDiffVisualRow(row, match))
		}
	}
	return strings.Join(rendered, "\n")
}

func (vp *PatchViewport) visibleRows() []visualRow {
	rows := vp.visualRows()
	end := vp.ScrollOffset + vp.Height
	if end > len(rows) {
		end = len(rows)
	}
	start := vp.ScrollOffset
	if start > len(rows) {
		start = len(rows)
	}

	if start > end {
		start = end
	}
	return rows[start:end]
}

func (vp *PatchViewport) visualRows() []visualRow {
	rows := make([]visualRow, 0, len(vp.lines))
	for _, line := range vp.lines {
		if line.typ == lineTypeHunkHeader {
			rows = append(rows, visualRow{line: line})
			continue
		}

		segments := []bodySegment{fullBodySegment(line.diff.Text)}
		if vp.LineMode == model.LineModeWrap && line.diff.Kind != model.LineNoNewline && vp.Width > 0 {
			bodyBudget := vp.bodyBudget(line.diff)
			if bodyBudget > 0 {
				segments = wrapBodySegments(line.diff.Text, bodyBudget)
			}
		}
		for i, segment := range segments {
			rows = append(rows, visualRow{
				line:         line,
				segment:      segment,
				continuation: i > 0,
			})
		}
	}
	return rows
}

func (vp *PatchViewport) bodyBudget(dl model.DiffLine) int {
	layers := buildDiffLineLayers(dl, vp.GutterVisible, vp.gutterDigits)
	return diffLineBodyWidth(layers, vp.Width)
}

func fullBodySegment(body string) bodySegment {
	return bodySegment{
		text:         body,
		start:        0,
		end:          len(body),
		displayStart: 0,
		displayEnd:   ansi.StringWidth(body),
		index:        0,
	}
}

func wrapBodySegments(body string, width int) []bodySegment {
	if body == "" || width <= 0 {
		return []bodySegment{fullBodySegment(body)}
	}

	var segments []bodySegment
	segmentStart := 0
	segmentDisplayStart := 0
	displayCol := 0
	segmentIndex := 0

	for i := 0; i < len(body); {
		cluster, clusterWidth := ansi.FirstGraphemeCluster(body[i:], ansi.GraphemeWidth)
		if cluster == "" {
			break
		}
		if i > segmentStart && displayCol-segmentDisplayStart+clusterWidth > width {
			segments = append(segments, bodySegmentRange(body, segmentStart, i, segmentDisplayStart, displayCol, segmentIndex))
			segmentStart = i
			segmentDisplayStart = displayCol
			segmentIndex++
		}

		i += len(cluster)
		displayCol += clusterWidth

		if displayCol-segmentDisplayStart == width {
			segments = append(segments, bodySegmentRange(body, segmentStart, i, segmentDisplayStart, displayCol, segmentIndex))
			segmentStart = i
			segmentDisplayStart = displayCol
			segmentIndex++
		}
	}

	if segmentStart < len(body) {
		segments = append(segments, bodySegmentRange(body, segmentStart, len(body), segmentDisplayStart, displayCol, segmentIndex))
	}
	if len(segments) == 0 {
		return []bodySegment{fullBodySegment(body)}
	}
	return segments
}

func bodySegmentRange(body string, start, end, displayStart, displayEnd, index int) bodySegment {
	return bodySegment{
		text:         body[start:end],
		start:        start,
		end:          end,
		displayStart: displayStart,
		displayEnd:   displayEnd,
		index:        index,
	}
}

func (s bodySegment) matchInSegment(match SearchMatch) SearchMatch {
	start := max(match.Start, s.start)
	end := min(match.End, s.end)
	if match.Line < 0 || start >= end {
		return NoSearchMatch()
	}
	return SearchMatch{
		Line:  match.Line,
		Start: start - s.start,
		End:   end - s.start,
	}
}

func (vp *PatchViewport) renderDiffVisualRow(row visualRow, match SearchMatch) string {
	dl := row.line.diff
	body := row.segment.text
	bodyStyled := false
	segmentMatch := row.segment.matchInSegment(match)

	if vp.syntaxHighlighted != nil && syntax.Highlightable(dl.Kind) {
		highlighted := vp.highlightBody(dl, row.line.diffIndex, match)
		body = ansi.Cut(highlighted, row.segment.displayStart, row.segment.displayEnd)
		bodyStyled = true
		segmentMatch = NoSearchMatch()
	}

	return renderDiffLineSegmentHL(
		dl,
		body,
		bodyStyled,
		vp.Width,
		vp.SearchQuery,
		segmentMatch,
		vp.GutterVisible,
		vp.gutterDigits,
		vp.LineMode,
		vp.XOffset,
		row.continuation,
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
	return renderDiffLineSegmentHL(dl, body, bodyStyled, width, query, match, gutterVisible, gutterDigits, lineMode, xOffset, false)
}

func renderDiffLineSegmentHL(dl model.DiffLine, body string, bodyStyled bool, width int, query string, match SearchMatch, gutterVisible bool, gutterDigits int, lineMode model.LineMode, xOffset int, continuation bool) string {
	if dl.Kind == model.LineNoNewline {
		return noNewlineStyle.Render("\\ No newline at end of file")
	}

	layers := buildDiffLineLayers(dl, gutterVisible, gutterDigits)
	layers.body = body
	layers.bodyStyled = bodyStyled
	return emitDiffLine(layers, width, query, match, lineMode, xOffset, continuation)
}

func emitDiffLine(layers diffLineLayers, width int, query string, match SearchMatch, lineMode model.LineMode, xOffset int, continuation bool) string {
	if lineMode != model.LineModeScroll || xOffset < 0 {
		xOffset = 0
	}
	if continuation {
		layers.gutter = strings.Repeat(" ", lipgloss.Width(layers.gutter))
		layers.prefix = strings.Repeat(" ", lipgloss.Width(layers.prefix))
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
		body = layers.style.Render(layers.prefix) + applyRowBackground(visibleBody, layers.style)
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

// applyRowBackground wraps a (possibly syntax-styled) body with the row's
// background so the tint persists across any inner full-reset ANSI sequences
// emitted by the syntax highlighter. Empty/no-bg inputs pass through.
func applyRowBackground(body string, style lipgloss.Style) string {
	if body == "" {
		return body
	}
	bgPrefix := backgroundPrefix(style)
	if bgPrefix == "" {
		return body
	}
	const reset = "\x1b[0m"
	// Re-emit the bg start after every inner reset so the tint stays active.
	restored := strings.ReplaceAll(body, reset, reset+bgPrefix)
	return bgPrefix + restored + reset
}

// backgroundPrefix probes lipgloss for the ANSI escape that activates only the
// background of the given style. Returns "" when no background is set or the
// active color profile is Ascii/no-color.
func backgroundPrefix(style lipgloss.Style) string {
	bg := style.GetBackground()
	if bg == nil {
		return ""
	}
	probe := lipgloss.NewStyle().Background(bg).Render(" ")
	idx := strings.Index(probe, " ")
	if idx <= 0 {
		return ""
	}
	return probe[:idx]
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
	total := vp.TotalLines()
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
	return len(vp.visualRows())
}

// IsHunkHeader reports whether the given viewport line is a hunk header.
func (vp *PatchViewport) IsHunkHeader(vpLine int) bool {
	rows := vp.visualRows()
	if vpLine < 0 || vpLine >= len(rows) {
		return false
	}
	return rows[vpLine].line.typ == lineTypeHunkHeader
}

// DiffLineToViewportLine converts a DiffLine index (0-based across all hunks,
// headers excluded) to the corresponding viewport line index (headers included).
func (vp *PatchViewport) DiffLineToViewportLine(diffIdx int) int {
	for i, row := range vp.visualRows() {
		if row.line.typ == lineTypeDiff && row.line.diffIndex == diffIdx {
			return i
		}
	}
	return 0
}

// ViewportLineToDiffLine converts a viewport line index to the DiffLine index.
// If the viewport line is a hunk header, it returns the index of the next DiffLine.
func (vp *PatchViewport) ViewportLineToDiffLine(vpLine int) int {
	rows := vp.visualRows()
	if len(rows) == 0 {
		return 0
	}
	if vpLine < 0 {
		vpLine = 0
	}
	if vpLine >= len(rows) {
		vpLine = len(rows) - 1
	}
	lastDiff := 0
	for i, row := range rows {
		if row.line.typ == lineTypeDiff {
			if i >= vpLine {
				return row.line.diffIndex
			}
			lastDiff = row.line.diffIndex
		}
	}
	return lastDiff
}

// CurrentTargetLine returns the most relevant new-file line number for the
// current viewport position or hunk, for opening the file in an editor.
func (vp *PatchViewport) CurrentTargetLine() (int, bool) {
	rows := vp.visualRows()
	if len(vp.Patch.Hunks) == 0 || len(rows) == 0 {
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
	hunkEnd := len(rows)
	if hunk+1 < len(vp.Patch.Hunks) {
		hunkEnd = vp.hunkLineOffset(hunk + 1)
	}

	for i := start; i < hunkEnd; i++ {
		if rows[i].line.typ == lineTypeDiff && rows[i].line.diff.NewNo != nil {
			return *rows[i].line.diff.NewNo, true
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
			Foreground(theme.Added).
			Background(theme.AddedBg)

	deletedStyle = lipgloss.NewStyle().
			Foreground(theme.Deleted).
			Background(theme.DeletedBg)

	contextStyle = lipgloss.NewStyle()

	noNewlineStyle = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true)

	gutterStyle = lipgloss.NewStyle().
			Foreground(theme.Muted)
)
