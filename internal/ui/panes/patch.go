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
	DiffMode     model.PatchDiffMode
	XOffset      int // body-column offset used by non-wrapping layout
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

const bodyOffsetStep = 8

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
	side         *sideBySideRow
}

type sideBySideRow struct {
	old *sideBySideCell
	new *sideBySideCell
}

type sideBySideCell struct {
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
	vp := &PatchViewport{
		Patch:         patch,
		LineMode:      model.LineModeWrap,
		DiffMode:      model.PatchDiffModeUnified,
		SearchMatch:   NoSearchMatch(),
		GutterVisible: true,
	}
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

// ScrollRight moves the visible code body right by the fixed offset step.
func (vp *PatchViewport) ScrollRight() {
	if vp.LineMode != model.LineModeScroll {
		return
	}
	vp.XOffset += bodyOffsetStep
	vp.ClampXOffset()
}

// ScrollLeft moves the visible code body left by the fixed offset step.
func (vp *PatchViewport) ScrollLeft() {
	if vp.LineMode != model.LineModeScroll {
		return
	}
	vp.XOffset -= bodyOffsetStep
	vp.ClampXOffset()
}

// ResetXOffset returns the body-column offset to the beginning.
func (vp *PatchViewport) ResetXOffset() {
	vp.XOffset = 0
}

// ClampXOffset constrains the body-column offset to the currently visible range.
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

// MaxXOffset returns the largest useful body offset for visible rows.
func (vp *PatchViewport) MaxXOffset() int {
	if vp.LineMode != model.LineModeScroll || vp.Width <= 0 {
		return 0
	}

	if vp.DiffMode == model.PatchDiffModeSideBySide {
		return vp.sideBySideMaxXOffset()
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

func (vp *PatchViewport) sideBySideMaxXOffset() int {
	leftWidth, rightWidth := sideBySideColumnWidths(vp.Width)
	bodyWidth := min(
		sideBySideBodyWidth(leftWidth, vp.GutterVisible, vp.gutterDigits),
		sideBySideBodyWidth(rightWidth, vp.GutterVisible, vp.gutterDigits),
	)
	longestBody := 0
	for _, row := range vp.visibleRows() {
		if row.side == nil {
			continue
		}
		for _, cell := range []*sideBySideCell{row.side.old, row.side.new} {
			if cell == nil || cell.line.diff.Kind == model.LineNoNewline {
				continue
			}
			if w := lipgloss.Width(cell.line.diff.Text); w > longestBody {
				longestBody = w
			}
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

// SetLineMode switches line layout while keeping
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

// SetDiffMode switches between unified and side-by-side patch layout while
// keeping the same logical diff line at the top of the viewport when possible.
func (vp *PatchViewport) SetDiffMode(mode model.PatchDiffMode) {
	if vp.DiffMode == mode {
		return
	}

	diffLine, rowOffset, ok := vp.ScrollAnchor()
	headerHunk := vp.headerAnchor()
	vp.DiffMode = mode
	vp.XOffset = 0
	switch {
	case ok:
		if offset, found := vp.ScrollOffsetForDiffLine(diffLine, rowOffset); found {
			vp.ScrollOffset = offset
		}
	case headerHunk >= 0:
		vp.ScrollOffset = vp.hunkLineOffset(headerHunk)
	}
	vp.clampScrollOffset()
	vp.SyncCurrentHunk()
}

func (vp *PatchViewport) headerAnchor() int {
	rows := vp.visualRows()
	if len(rows) == 0 {
		return -1
	}
	idx := vp.ScrollOffset
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rows) {
		idx = len(rows) - 1
	}
	if rows[idx].line.typ != lineTypeHunkHeader {
		return -1
	}
	return rows[idx].line.hunkIndex
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
	return row.primaryDiffIndex(), row.segment.index, true
}

// ScrollOffsetForDiffLine returns the visual row offset for a logical diff line
// and wrapped-row offset. If the requested row no longer exists, it clamps to
// the last visual row for that logical line.
func (vp *PatchViewport) ScrollOffsetForDiffLine(diffLine int, rowOffset int) (int, bool) {
	last := -1
	for i, row := range vp.visualRows() {
		if !row.containsDiffIndex(diffLine) {
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
			if row.side != nil {
				rendered = append(rendered, vp.renderSideBySideVisualRow(row))
				continue
			}
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
	if vp.DiffMode == model.PatchDiffModeSideBySide {
		return vp.sideBySideVisualRows()
	}
	return vp.unifiedVisualRows()
}

func (vp *PatchViewport) unifiedVisualRows() []visualRow {
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

func (vp *PatchViewport) sideBySideVisualRows() []visualRow {
	rows := make([]visualRow, 0, len(vp.lines))
	diffIndex := 0
	for hunkIndex, h := range vp.Patch.Hunks {
		rows = append(rows, visualRow{
			line: patchLine{typ: lineTypeHunkHeader, header: formatHunkHeader(h), diffIndex: -1, hunkIndex: hunkIndex},
		})

		for i := 0; i < len(h.Lines); {
			switch h.Lines[i].Kind {
			case model.LineDeleted:
				var deleted []patchLine
				for i < len(h.Lines) && h.Lines[i].Kind == model.LineDeleted {
					deleted = append(deleted, patchLine{typ: lineTypeDiff, diff: h.Lines[i], diffIndex: diffIndex, hunkIndex: hunkIndex})
					i++
					diffIndex++
				}
				var added []patchLine
				for i < len(h.Lines) && h.Lines[i].Kind == model.LineAdded {
					added = append(added, patchLine{typ: lineTypeDiff, diff: h.Lines[i], diffIndex: diffIndex, hunkIndex: hunkIndex})
					i++
					diffIndex++
				}
				rows = append(rows, vp.pairedSideBySideRows(deleted, added)...)
			case model.LineAdded:
				var added []patchLine
				for i < len(h.Lines) && h.Lines[i].Kind == model.LineAdded {
					added = append(added, patchLine{typ: lineTypeDiff, diff: h.Lines[i], diffIndex: diffIndex, hunkIndex: hunkIndex})
					i++
					diffIndex++
				}
				rows = append(rows, vp.pairedSideBySideRows(nil, added)...)
			case model.LineContext:
				line := patchLine{typ: lineTypeDiff, diff: h.Lines[i], diffIndex: diffIndex, hunkIndex: hunkIndex}
				rows = append(rows, vp.pairedSideBySideRows([]patchLine{line}, []patchLine{line})...)
				i++
				diffIndex++
			default:
				line := patchLine{typ: lineTypeDiff, diff: h.Lines[i], diffIndex: diffIndex, hunkIndex: hunkIndex}
				rows = append(rows, visualRow{line: line, segment: fullBodySegment(line.diff.Text)})
				i++
				diffIndex++
			}
		}
	}
	return rows
}

func (vp *PatchViewport) pairedSideBySideRows(oldLines, newLines []patchLine) []visualRow {
	total := max(len(oldLines), len(newLines))
	rows := make([]visualRow, 0, total)
	for i := 0; i < total; i++ {
		var oldLine, newLine *patchLine
		if i < len(oldLines) {
			oldLine = &oldLines[i]
		}
		if i < len(newLines) {
			newLine = &newLines[i]
		}
		rows = append(rows, vp.sideBySideRowsForPair(oldLine, newLine)...)
	}
	return rows
}

func (vp *PatchViewport) sideBySideRowsForPair(oldLine, newLine *patchLine) []visualRow {
	leftWidth, rightWidth := sideBySideColumnWidths(vp.Width)
	oldSegments := sideBySideSegments(oldLine, sideBySideBodyWidth(leftWidth, vp.GutterVisible, vp.gutterDigits), vp.LineMode)
	newSegments := sideBySideSegments(newLine, sideBySideBodyWidth(rightWidth, vp.GutterVisible, vp.gutterDigits), vp.LineMode)
	total := max(len(oldSegments), len(newSegments))
	if total == 0 {
		total = 1
	}

	rows := make([]visualRow, 0, total)
	for i := 0; i < total; i++ {
		var oldCell, newCell *sideBySideCell
		if oldLine != nil && i < len(oldSegments) {
			oldCell = &sideBySideCell{line: *oldLine, segment: oldSegments[i], continuation: i > 0}
		}
		if newLine != nil && i < len(newSegments) {
			newCell = &sideBySideCell{line: *newLine, segment: newSegments[i], continuation: i > 0}
		}

		line := sideBySidePrimaryLine(oldCell, newCell)
		rows = append(rows, visualRow{
			line:    line,
			segment: bodySegment{index: i},
			side:    &sideBySideRow{old: oldCell, new: newCell},
		})
	}
	return rows
}

func sideBySideSegments(line *patchLine, width int, lineMode model.LineMode) []bodySegment {
	if line == nil {
		return nil
	}
	if lineMode == model.LineModeWrap && line.diff.Kind != model.LineNoNewline && width > 0 {
		return wrapBodySegments(line.diff.Text, width)
	}
	return []bodySegment{fullBodySegment(line.diff.Text)}
}

func sideBySidePrimaryLine(oldCell, newCell *sideBySideCell) patchLine {
	if oldCell != nil {
		return oldCell.line
	}
	if newCell != nil {
		return newCell.line
	}
	return patchLine{typ: lineTypeDiff, diffIndex: -1}
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

func (vp *PatchViewport) renderSideBySideVisualRow(row visualRow) string {
	leftWidth, rightWidth := sideBySideColumnWidths(vp.Width)
	left := vp.renderSideBySideCell(row.side.old, leftWidth, sideOld)
	right := vp.renderSideBySideCell(row.side.new, rightWidth, sideNew)
	line := left + sideBySideSeparator() + right
	if vp.Width > 0 {
		return truncateToWidth(line, vp.Width)
	}
	return line
}

type sideBySideSide int

const (
	sideOld sideBySideSide = iota
	sideNew
)

func (vp *PatchViewport) renderSideBySideCell(cell *sideBySideCell, width int, side sideBySideSide) string {
	if width <= 0 {
		return ""
	}
	if cell == nil {
		return strings.Repeat(" ", width)
	}

	dl := cell.line.diff
	body := cell.segment.text
	bodyStyled := false
	match := NoSearchMatch()
	if vp.SearchQuery != "" && vp.SearchMatch.Line == cell.line.diffIndex {
		match = vp.SearchMatch
	}
	segmentMatch := cell.segment.matchInSegment(match)

	if vp.syntaxHighlighted != nil && syntax.Highlightable(dl.Kind) {
		highlighted := vp.highlightBody(dl, cell.line.diffIndex, match)
		body = ansi.Cut(highlighted, cell.segment.displayStart, cell.segment.displayEnd)
		bodyStyled = true
		segmentMatch = NoSearchMatch()
	}

	gutter := ""
	if vp.GutterVisible {
		gutter = formatSideGutter(sideBySideLineNo(dl, side), vp.gutterDigits)
	}
	prefix, style, fill := diffLineStyle(dl.Kind)
	layers := diffLineLayers{
		gutter:     gutter,
		prefix:     prefix,
		body:       body,
		style:      style,
		fill:       fill,
		bodyStyled: bodyStyled,
	}

	rendered := emitDiffLine(layers, width, vp.SearchQuery, segmentMatch, vp.LineMode, vp.XOffset, cell.continuation)
	return padOrTruncateToWidth(rendered, width)
}

func (vp *PatchViewport) highlightBody(dl model.DiffLine, diffIndex int, match SearchMatch) string {
	if vp.syntaxHighlighted == nil || !syntax.Highlightable(dl.Kind) {
		return dl.Text
	}
	background, hasBackground := diffLineBackground(dl.Kind)
	if vp.SearchQuery != "" && match.validForBody(dl.Text) {
		if hasBackground {
			return vp.syntaxHighlighted.HighlightLineSpanBackground(diffIndex, dl.Text, match.Start, match.End, background)
		}
		return vp.syntaxHighlighted.HighlightLineSpan(diffIndex, dl.Text, match.Start, match.End)
	}
	if hasBackground {
		return vp.syntaxHighlighted.HighlightLineBackground(diffIndex, dl.Text, background)
	}
	return vp.syntaxHighlighted.HighlightLine(diffIndex, dl.Text)
}

type diffLineLayers struct {
	gutter string
	prefix string
	body   string
	style  lipgloss.Style
	fill   bool

	bodyStyled bool
}

func buildDiffLineLayers(dl model.DiffLine, gutterVisible bool, gutterDigits int) diffLineLayers {
	prefix, style, fill := diffLineStyle(dl.Kind)
	layers := diffLineLayers{
		prefix: prefix,
		body:   dl.Text,
		style:  style,
		fill:   fill,
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
	bodyPadding := ""
	if width > 0 {
		bodyWidth := diffLineBodyWidth(layers, width)
		if bodyWidth <= 0 {
			visibleBody = ""
		} else {
			if xOffset > 0 {
				visibleBody = ansi.TruncateLeft(visibleBody, xOffset, "")
			}
			visibleBody = ansi.Truncate(visibleBody, bodyWidth, "")
			bodyPadding = diffLinePadding(visibleBody, bodyWidth, layers.fill)
		}
	}

	body := layers.prefix + visibleBody + bodyPadding
	if layers.bodyStyled {
		body = layers.style.Render(layers.prefix) + visibleBody
		if bodyPadding != "" {
			body += layers.style.Render(bodyPadding)
		}
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

func diffLinePadding(body string, width int, fill bool) string {
	if !fill {
		return ""
	}
	padding := width - lipgloss.Width(lipgloss.NewStyle().Render(body))
	if padding <= 0 {
		return ""
	}
	return strings.Repeat(" ", padding)
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

func diffLineStyle(kind model.LineKind) (string, lipgloss.Style, bool) {
	switch kind {
	case model.LineAdded:
		return "+", addedStyle, true
	case model.LineDeleted:
		return "-", deletedStyle, true
	case model.LineContext:
		return " ", contextStyle, false
	default:
		return " ", contextStyle, false
	}
}

func diffLineBackground(kind model.LineKind) (lipgloss.Color, bool) {
	switch kind {
	case model.LineAdded:
		return theme.DiffAddedBg, true
	case model.LineDeleted:
		return theme.DiffDeletedBg, true
	default:
		return "", false
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

func sideBySideColumnWidths(width int) (int, int) {
	if width <= 0 {
		return 0, 0
	}
	separatorWidth := lipgloss.Width(sideBySideSeparator())
	if width <= separatorWidth {
		return width, 0
	}
	available := width - separatorWidth
	left := available / 2
	right := available - left
	return left, right
}

func sideBySideSeparator() string {
	return gutterStyle.Render(" │ ")
}

func sideBySideBodyWidth(columnWidth int, gutterVisible bool, gutterDigits int) int {
	width := columnWidth - 1 // diff prefix
	if gutterVisible {
		width -= gutterDigits + lipgloss.Width(" │ ")
	}
	if width < 0 {
		return 0
	}
	return width
}

func sideBySideLineNo(dl model.DiffLine, side sideBySideSide) *int {
	if side == sideOld {
		return dl.OldNo
	}
	return dl.NewNo
}

func formatSideGutter(no *int, digits int) string {
	lineNo := strings.Repeat(" ", digits)
	if no != nil {
		lineNo = fmt.Sprintf("%*d", digits, *no)
	}
	return gutterStyle.Render(lineNo) + gutterStyle.Render(" │") + " "
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

func padOrTruncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w >= width {
		return truncateToWidth(s, width)
	}
	return s + strings.Repeat(" ", width-w)
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

func (r visualRow) primaryDiffIndex() int {
	if r.side == nil {
		return r.line.diffIndex
	}
	if r.side.old != nil {
		return r.side.old.line.diffIndex
	}
	if r.side.new != nil {
		return r.side.new.line.diffIndex
	}
	return r.line.diffIndex
}

func (r visualRow) containsDiffIndex(diffIdx int) bool {
	if r.line.typ != lineTypeDiff {
		return false
	}
	if r.side == nil {
		return r.line.diffIndex == diffIdx
	}
	return (r.side.old != nil && r.side.old.line.diffIndex == diffIdx) ||
		(r.side.new != nil && r.side.new.line.diffIndex == diffIdx)
}

func (r visualRow) newLineNo() *int {
	if r.side == nil {
		return r.line.diff.NewNo
	}
	if r.side.new != nil {
		return r.side.new.line.diff.NewNo
	}
	if r.side.old != nil {
		return r.side.old.line.diff.NewNo
	}
	return nil
}

// DiffLineToViewportLine converts a DiffLine index (0-based across all hunks,
// headers excluded) to the corresponding viewport line index (headers included).
func (vp *PatchViewport) DiffLineToViewportLine(diffIdx int) int {
	for i, row := range vp.visualRows() {
		if row.containsDiffIndex(diffIdx) {
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
				return row.primaryDiffIndex()
			}
			lastDiff = row.primaryDiffIndex()
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
		if rows[i].line.typ == lineTypeDiff {
			if newNo := rows[i].newLineNo(); newNo != nil {
				return *newNo, true
			}
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
			Foreground(theme.DiffLineText).
			Background(theme.DiffAddedBg)

	deletedStyle = lipgloss.NewStyle().
			Foreground(theme.DiffLineText).
			Background(theme.DiffDeletedBg)

	contextStyle = lipgloss.NewStyle()

	noNewlineStyle = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true)

	gutterStyle = lipgloss.NewStyle().
			Foreground(theme.Muted)
)
