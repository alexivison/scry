// Package panes implements individual UI pane components for scry.
package panes

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/notes"
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
	ColorEnabled  bool

	// Pre-computed flat line list for rendering.
	lines             []patchLine
	gutterDigits      int // width of each line-number column (min 4)
	syntaxHighlighted *syntax.LineCache
	notes             []notes.Note
	selectedNoteID    string
	noteDraft         *NoteDraftView
	sourceCursor      int
	renderWidth       int
	renderHeight      int
	visibilityDirty   bool
	manualScroll      bool
}

const bodyOffsetStep = 8

type lineType int

const (
	lineTypeFileHeader lineType = iota
	lineTypeHunkHeader
	lineTypeDiff
)

type patchLine struct {
	typ       lineType
	header    string         // only for hunkHeader
	diff      model.DiffLine // only for diff lines
	diffIndex int            // logical diff line index, headers excluded
	hunkIndex int

	changedSpans []bodySpan
	hasIntraline bool
}

type visualRow struct {
	line         patchLine
	segment      bodySegment
	continuation bool
	side         *sideBySideRow
	note         *noteVisualRow
}

type cursorTarget struct {
	diffIndex int
	noteID    string
}

type noteVisualRow struct {
	id   string
	text string
}

type NoteDraftView struct {
	NoteID string
	File   string
	Line   int
	Body   string
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

type bodySpan struct {
	start int
	end   int
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
		ColorEnabled:  true,
		sourceCursor:  -1,
		renderWidth:   -1,
		renderHeight:  -1,
	}
	vp.lines = vp.buildLines()
	vp.gutterDigits = vp.computeGutterDigits()
	return vp
}

// SetSyntaxHighlighter enables body-only syntax highlighting for diff lines.
func (vp *PatchViewport) SetSyntaxHighlighter(lines *syntax.LineCache) {
	vp.syntaxHighlighted = lines
}

func (vp *PatchViewport) SetNotes(items []notes.Note, selectedID string, draft *NoteDraftView) {
	changed := vp.selectedNoteID != selectedID || !sameNoteDraft(vp.noteDraft, draft)
	if !slices.Equal(vp.notes, items) {
		next := append([]notes.Note(nil), items...)
		sort.SliceStable(next, func(i, j int) bool {
			if next[i].Line != next[j].Line {
				return next[i].Line < next[j].Line
			}
			if !next[i].CreatedAt.Equal(next[j].CreatedAt) {
				return next[i].CreatedAt.Before(next[j].CreatedAt)
			}
			return next[i].ID < next[j].ID
		})
		if !slices.Equal(vp.notes, next) {
			vp.notes = next
			changed = true
		}
	}
	vp.selectedNoteID = selectedID
	vp.noteDraft = draft
	vp.visibilityDirty = vp.visibilityDirty || changed
}

func (vp *PatchViewport) NoteBodyWidth() int {
	return max(vp.Width-vp.noteIndent()-4, 1)
}

func (vp *PatchViewport) KeepScroll(offset int) {
	vp.ScrollOffset = offset
	vp.visibilityDirty = false
	vp.manualScroll = true
}

func sameNoteDraft(a, b *NoteDraftView) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
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
	filePath := ""
	for hunkIndex, h := range vp.Patch.Hunks {
		if h.FilePath != "" && h.FilePath != filePath {
			lines = append(lines, patchLine{typ: lineTypeFileHeader, header: h.FilePath, diffIndex: -1})
			filePath = h.FilePath
		}
		lines = append(lines, patchLine{typ: lineTypeHunkHeader, header: formatHunkHeader(h), diffIndex: -1, hunkIndex: hunkIndex})
		lines = append(lines, buildPatchLines(h.Lines, hunkIndex, &diffIndex)...)
	}
	return lines
}

func buildPatchLines(diffLines []model.DiffLine, hunkIndex int, diffIndex *int) []patchLine {
	lines := make([]patchLine, 0, len(diffLines))
	for _, dl := range diffLines {
		lines = append(lines, patchLine{typ: lineTypeDiff, diff: dl, diffIndex: *diffIndex, hunkIndex: hunkIndex})
		*diffIndex = *diffIndex + 1
	}
	annotateIntralineChanges(lines)
	return lines
}

func annotateIntralineChanges(lines []patchLine) {
	for i := 0; i < len(lines); {
		if lines[i].diff.Kind != model.LineDeleted {
			i++
			continue
		}

		deletedStart := i
		for i < len(lines) && lines[i].diff.Kind == model.LineDeleted {
			i++
		}
		addedStart := i
		for i < len(lines) && lines[i].diff.Kind == model.LineAdded {
			i++
		}
		if addedStart == i {
			continue
		}
		annotatePairedLineChanges(lines[deletedStart:addedStart], lines[addedStart:i])
	}
}

func annotatePairedLineChanges(deleted, added []patchLine) {
	for i := 0; i < min(len(deleted), len(added)); i++ {
		deletedSpans, addedSpans := intralineChangedSpans(deleted[i].diff.Text, added[i].diff.Text)
		deleted[i].changedSpans = deletedSpans
		deleted[i].hasIntraline = true
		added[i].changedSpans = addedSpans
		added[i].hasIntraline = true
	}
}

func intralineChangedSpans(oldText, newText string) ([]bodySpan, []bodySpan) {
	if oldText == "" || newText == "" {
		return fallbackIntralineChangedSpans(oldText, newText)
	}

	oldTokens := tokenizeBody(oldText)
	newTokens := tokenizeBody(newText)
	if len(oldTokens) == 0 || len(newTokens) == 0 || len(oldTokens)*len(newTokens) > 262144 {
		return fallbackIntralineChangedSpans(oldText, newText)
	}

	oldChanged, newChanged := changedTokenSpans(oldText, newText, oldTokens, newTokens)
	if len(oldChanged) == 0 && len(newChanged) == 0 {
		return fallbackIntralineChangedSpans(oldText, newText)
	}
	return oldChanged, newChanged
}

func fallbackIntralineChangedSpans(oldText, newText string) ([]bodySpan, []bodySpan) {
	prefix := commonPrefixLen(oldText, newText)
	oldSuffix, newSuffix := commonSuffixStart(oldText, newText, prefix)

	oldSpan := expandSpanToToken(oldText, bodySpan{start: prefix, end: oldSuffix})
	newSpan := expandSpanToToken(newText, bodySpan{start: prefix, end: newSuffix})

	return nonEmptySpan(oldSpan), nonEmptySpan(newSpan)
}

type bodyToken struct {
	text  string
	start int
	end   int
}

func tokenizeBody(body string) []bodyToken {
	var tokens []bodyToken
	for i := 0; i < len(body); {
		start := i
		r, size := utf8.DecodeRuneInString(body[i:])
		token := isTokenRune(r)
		i += size
		for i < len(body) {
			r, size = utf8.DecodeRuneInString(body[i:])
			if isTokenRune(r) != token {
				break
			}
			i += size
		}
		tokens = append(tokens, bodyToken{text: body[start:i], start: start, end: i})
	}
	return tokens
}

func changedTokenSpans(oldText, newText string, oldTokens, newTokens []bodyToken) ([]bodySpan, []bodySpan) {
	common := longestCommonTokenSubsequence(oldTokens, newTokens)
	return unmatchedTokenSpans(oldText, oldTokens, common, true), unmatchedTokenSpans(newText, newTokens, common, false)
}

type tokenMatch struct {
	old int
	new int
}

func longestCommonTokenSubsequence(oldTokens, newTokens []bodyToken) []tokenMatch {
	cols := len(newTokens) + 1
	table := make([]int, (len(oldTokens)+1)*cols)
	for i := len(oldTokens) - 1; i >= 0; i-- {
		for j := len(newTokens) - 1; j >= 0; j-- {
			idx := i*cols + j
			if oldTokens[i].text == newTokens[j].text {
				table[idx] = table[(i+1)*cols+j+1] + 1
				continue
			}
			table[idx] = max(table[(i+1)*cols+j], table[i*cols+j+1])
		}
	}

	var matches []tokenMatch
	for i, j := 0, 0; i < len(oldTokens) && j < len(newTokens); {
		if oldTokens[i].text == newTokens[j].text {
			matches = append(matches, tokenMatch{old: i, new: j})
			i++
			j++
			continue
		}
		if table[(i+1)*cols+j] >= table[i*cols+j+1] {
			i++
		} else {
			j++
		}
	}
	return matches
}

func unmatchedTokenSpans(body string, tokens []bodyToken, matches []tokenMatch, oldSide bool) []bodySpan {
	matched := make([]bool, len(tokens))
	for _, match := range matches {
		idx := match.new
		if oldSide {
			idx = match.old
		}
		matched[idx] = true
	}

	var spans []bodySpan
	for i := 0; i < len(tokens); {
		if matched[i] {
			i++
			continue
		}
		start := tokens[i].start
		end := tokens[i].end
		i++
		for i < len(tokens) && !matched[i] {
			end = tokens[i].end
			i++
		}
		spans = append(spans, bodySpan{start: start, end: end})
	}
	return compactSpans(spans)
}

func compactSpans(spans []bodySpan) []bodySpan {
	out := spans[:0]
	for _, span := range spans {
		if span.start < span.end {
			out = append(out, span)
		}
	}
	return out
}

func commonPrefixLen(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) {
		ar, as := utf8.DecodeRuneInString(a[i:])
		br, bs := utf8.DecodeRuneInString(b[i:])
		if ar != br {
			break
		}
		i += min(as, bs)
	}
	return i
}

func commonSuffixStart(a, b string, prefix int) (int, int) {
	ai, bi := len(a), len(b)
	for ai > prefix && bi > prefix {
		ar, as := utf8.DecodeLastRuneInString(a[:ai])
		br, bs := utf8.DecodeLastRuneInString(b[:bi])
		if ar != br || ai-as < prefix || bi-bs < prefix {
			break
		}
		ai -= as
		bi -= bs
	}
	return ai, bi
}

func expandSpanToToken(body string, span bodySpan) bodySpan {
	if span.start >= span.end {
		return span
	}

	for span.start > 0 {
		r, size := utf8.DecodeLastRuneInString(body[:span.start])
		if !isTokenRune(r) {
			break
		}
		span.start -= size
	}
	for span.end < len(body) {
		r, size := utf8.DecodeRuneInString(body[span.end:])
		if !isTokenRune(r) {
			break
		}
		span.end += size
	}
	return span
}

func isTokenRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func nonEmptySpan(span bodySpan) []bodySpan {
	if span.start >= span.end {
		return nil
	}
	return []bodySpan{span}
}

func formatHunkHeader(h model.Hunk) string {
	s := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldLen, h.NewStart, h.NewLen)
	if h.FilePath != "" {
		s += " " + h.FilePath
	}
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
	vp.sourceCursor = -1
}

// PrevHunk moves to the previous hunk. No-op at the first hunk.
func (vp *PatchViewport) PrevHunk() {
	if len(vp.Patch.Hunks) == 0 || vp.CurrentHunk <= 0 {
		return
	}
	vp.CurrentHunk--
	vp.ScrollOffset = vp.hunkLineOffset(vp.CurrentHunk)
	vp.sourceCursor = -1
}

func (vp *PatchViewport) MoveCursor(delta int) string {
	vp.manualScroll = false
	rows := vp.visualRows()
	vp.ensureSourceCursor(rows)
	targets := cursorTargets(rows)
	if len(targets) == 0 {
		return vp.selectedNoteID
	}
	position := -1
	for i, target := range targets {
		noteSelected := vp.selectedNoteID != "" && target.noteID == vp.selectedNoteID
		sourceSelected := vp.selectedNoteID == "" && target.noteID == "" && target.diffIndex == vp.sourceCursor
		if noteSelected || sourceSelected {
			position = i
			break
		}
	}
	target := position + delta
	if target < 0 || target >= len(targets) {
		return vp.selectedNoteID
	}
	selected := targets[target]
	if selected.noteID != "" {
		vp.selectedNoteID = selected.noteID
		vp.ensureNoteVisible(rows, selected.noteID)
	} else {
		vp.selectedNoteID = ""
		vp.sourceCursor = selected.diffIndex
		vp.ensureSourceCursorVisible(rows)
	}
	vp.SyncCurrentHunk()
	return vp.selectedNoteID
}

// ScrollDown moves the viewport one line down. No-op at the bottom.
func (vp *PatchViewport) ScrollDown() {
	if vp.ScrollOffset < vp.TotalLines()-1 {
		vp.ScrollOffset++
		vp.sourceCursor = -1
		vp.manualScroll = true
		vp.SyncCurrentHunk()
	}
}

// ScrollUp moves the viewport one line up. No-op at the top.
func (vp *PatchViewport) ScrollUp() {
	if vp.ScrollOffset > 0 {
		vp.ScrollOffset--
		vp.sourceCursor = -1
		vp.manualScroll = true
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
	vp.sourceCursor = -1
	vp.manualScroll = true
	vp.SyncCurrentHunk()
}

// PageUp moves the viewport one full page up.
func (vp *PatchViewport) PageUp() {
	vp.ScrollOffset -= vp.Height
	if vp.ScrollOffset < 0 {
		vp.ScrollOffset = 0
	}
	vp.sourceCursor = -1
	vp.manualScroll = true
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
	vp.sourceCursor = -1
	vp.manualScroll = true
	vp.SyncCurrentHunk()
}

// HalfPageUp moves the viewport half a page up.
func (vp *PatchViewport) HalfPageUp() {
	vp.ScrollOffset -= vp.Height / 2
	if vp.ScrollOffset < 0 {
		vp.ScrollOffset = 0
	}
	vp.sourceCursor = -1
	vp.manualScroll = true
	vp.SyncCurrentHunk()
}

// ScrollToTop jumps to the beginning of the patch.
func (vp *PatchViewport) ScrollToTop() {
	vp.ScrollOffset = 0
	vp.CurrentHunk = 0
	vp.sourceCursor = -1
	vp.manualScroll = true
}

// ScrollToBottom jumps to the end of the patch.
func (vp *PatchViewport) ScrollToBottom() {
	max := vp.TotalLines() - vp.Height
	if max < 0 {
		max = 0
	}
	vp.ScrollOffset = max
	vp.sourceCursor = -1
	vp.manualScroll = true
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
	vp.manualScroll = false
	vp.ensureActiveVisible(vp.visualRows())
	vp.SyncCurrentHunk()
}

// SetDiffMode switches between unified and side-by-side patch layout while
// keeping the same logical diff line at the top of the viewport when possible.
func (vp *PatchViewport) SetDiffMode(mode model.PatchDiffMode) {
	if vp.DiffMode == mode {
		return
	}

	vp.setLayout(mode, vp.GutterVisible)
}

// SetGutterVisible changes line-number visibility while preserving scroll position.
func (vp *PatchViewport) SetGutterVisible(visible bool) {
	if vp.GutterVisible == visible {
		return
	}

	vp.setLayout(vp.DiffMode, visible)
}

func (vp *PatchViewport) setLayout(mode model.PatchDiffMode, gutterVisible bool) {
	diffLine, rowOffset, ok := vp.ScrollAnchor()
	headerHunk := vp.headerAnchor()
	vp.DiffMode = mode
	vp.GutterVisible = gutterVisible
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
	vp.manualScroll = false
	vp.ensureActiveVisible(vp.visualRows())
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
	rows := vp.visualRows()
	vp.ensureSourceCursor(rows)
	resized := vp.Width != vp.renderWidth || vp.Height != vp.renderHeight
	if resized || vp.visibilityDirty && !vp.manualScroll {
		vp.ensureActiveVisible(rows)
	}
	vp.visibilityDirty = false
	vp.manualScroll = false
	vp.renderWidth = vp.Width
	vp.renderHeight = vp.Height
	visible := vp.visibleRows()
	rendered := make([]string, 0, len(visible))
	for _, row := range visible {
		if row.note != nil {
			line := row.note.text
			if vp.selectedNoteID != "" && row.note.id == vp.selectedNoteID {
				line = vp.renderCursorRow(line)
			}
			rendered = append(rendered, line)
			continue
		}
		switch row.line.typ {
		case lineTypeFileHeader:
			rendered = append(rendered, renderFileSeparator(row.line.header, vp.Width))
		case lineTypeHunkHeader:
			rendered = append(rendered, renderHunkSeparator(row.line.header, vp.Width))
		case lineTypeDiff:
			if row.side != nil {
				rendered = append(rendered, vp.renderSideBySideVisualRow(row, vp.sourceCursorOn(row)))
				continue
			}
			match := NoSearchMatch()
			if vp.SearchQuery != "" && vp.SearchMatch.Line == row.line.diffIndex {
				match = vp.SearchMatch
			}
			line := vp.renderDiffVisualRow(row, match)
			if vp.sourceCursorOn(row) {
				line = vp.renderCursorRow(line, vp.unifiedCursorMatch(row, match))
			}
			rendered = append(rendered, line)
		}
	}
	return strings.Join(rendered, "\n")
}

func (vp *PatchViewport) sourceCursorOn(row visualRow) bool {
	if vp.selectedNoteID != "" || vp.noteDraft != nil {
		return false
	}
	index, ok := row.sourceDiffIndex()
	return ok && index == vp.sourceCursor
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
	var rows []visualRow
	if vp.DiffMode == model.PatchDiffModeSideBySide {
		rows = vp.sideBySideVisualRows()
	} else {
		rows = vp.unifiedVisualRows()
	}
	return vp.withNotes(rows)
}

func (vp *PatchViewport) withNotes(rows []visualRow) []visualRow {
	if len(vp.notes) == 0 && vp.noteDraft == nil {
		return rows
	}

	byLine := make(map[int][]notes.Note, len(vp.notes))
	for _, note := range vp.notes {
		if note.State != notes.StateOpen {
			continue
		}
		byLine[note.Line] = append(byLine[note.Line], note)
	}

	indent := vp.noteIndent()
	noteWidth := max(vp.Width-indent, 0)
	prefix := strings.Repeat(" ", indent)
	out := make([]visualRow, 0, len(rows)+len(vp.notes)*3)
	for i, row := range rows {
		out = append(out, row)
		line := row.newLineNo()
		if line == nil || i+1 < len(rows) && sameSourceLine(row, rows[i+1]) {
			continue
		}
		draftAdded := false
		for _, note := range byLine[*line] {
			isDraft := vp.noteDraft != nil && vp.noteDraft.NoteID == note.ID
			if isDraft {
				note.Body = vp.noteDraft.Body
				draftAdded = true
			}
			for _, text := range renderNoteCard(note, noteWidth, note.ID == vp.selectedNoteID || isDraft) {
				out = append(out, visualRow{note: &noteVisualRow{id: note.ID, text: prefix + text}})
			}
		}
		if vp.noteDraft != nil && vp.noteDraft.Line == *line && !draftAdded {
			draft := notes.Note{File: vp.noteDraft.File, Line: vp.noteDraft.Line, Body: vp.noteDraft.Body, Author: notes.AuthorUser, State: notes.StateOpen}
			for _, text := range renderNoteCard(draft, noteWidth, true) {
				out = append(out, visualRow{note: &noteVisualRow{text: prefix + text}})
			}
		}
	}
	return out
}

func (vp *PatchViewport) noteIndent() int {
	indent := 1
	if vp.DiffMode == model.PatchDiffModeSideBySide {
		left, _ := sideBySideColumnWidths(vp.Width)
		indent += left + lipgloss.Width(sideBySideSeparator())
		if vp.GutterVisible {
			indent += lipgloss.Width(formatSideGutter(nil, vp.gutterDigits))
		}
	} else if vp.GutterVisible {
		indent += lipgloss.Width(formatGutter(nil, nil, vp.gutterDigits))
	}
	return min(indent, max(vp.Width-4, 0))
}

func sameSourceLine(a, b visualRow) bool {
	aLine, bLine := a.newLineNo(), b.newLineNo()
	return aLine != nil && bLine != nil && *aLine == *bLine
}

func (vp *PatchViewport) unifiedVisualRows() []visualRow {
	rows := make([]visualRow, 0, len(vp.lines))
	for _, line := range vp.lines {
		if line.typ != lineTypeDiff {
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
	filePath := ""
	for hunkIndex, h := range vp.Patch.Hunks {
		if h.FilePath != "" && h.FilePath != filePath {
			rows = append(rows, visualRow{line: patchLine{typ: lineTypeFileHeader, header: h.FilePath, diffIndex: -1}})
			filePath = h.FilePath
		}
		rows = append(rows, visualRow{
			line: patchLine{typ: lineTypeHunkHeader, header: formatHunkHeader(h), diffIndex: -1, hunkIndex: hunkIndex},
		})

		hunkLines := vp.diffLinesForHunk(hunkIndex)
		for i := 0; i < len(hunkLines); {
			switch hunkLines[i].diff.Kind {
			case model.LineDeleted:
				var deleted []patchLine
				for i < len(hunkLines) && hunkLines[i].diff.Kind == model.LineDeleted {
					deleted = append(deleted, hunkLines[i])
					i++
				}
				var added []patchLine
				for i < len(hunkLines) && hunkLines[i].diff.Kind == model.LineAdded {
					added = append(added, hunkLines[i])
					i++
				}
				rows = append(rows, vp.pairedSideBySideRows(deleted, added)...)
			case model.LineAdded:
				var added []patchLine
				for i < len(hunkLines) && hunkLines[i].diff.Kind == model.LineAdded {
					added = append(added, hunkLines[i])
					i++
				}
				rows = append(rows, vp.pairedSideBySideRows(nil, added)...)
			case model.LineContext:
				line := hunkLines[i]
				rows = append(rows, vp.pairedSideBySideRows([]patchLine{line}, []patchLine{line})...)
				i++
			default:
				line := hunkLines[i]
				rows = append(rows, visualRow{line: line, segment: fullBodySegment(line.diff.Text)})
				i++
			}
		}
	}
	return rows
}

func (vp *PatchViewport) diffLinesForHunk(hunkIndex int) []patchLine {
	var lines []patchLine
	for _, line := range vp.lines {
		if line.typ == lineTypeDiff && line.hunkIndex == hunkIndex {
			lines = append(lines, line)
		}
	}
	return lines
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

func (s bodySegment) spansInSegment(spans []bodySpan) []bodySpan {
	if len(spans) == 0 {
		return nil
	}
	segmentSpans := make([]bodySpan, 0, len(spans))
	for _, span := range spans {
		start := max(span.start, s.start)
		end := min(span.end, s.end)
		if start >= end {
			continue
		}
		segmentSpans = append(segmentSpans, bodySpan{
			start: start - s.start,
			end:   end - s.start,
		})
	}
	return segmentSpans
}

func (vp *PatchViewport) renderDiffVisualRow(row visualRow, match SearchMatch) string {
	dl := row.line.diff
	body := row.segment.text
	bodyStyled := false
	segmentMatch := row.segment.matchInSegment(match)
	changedSpans := row.segment.spansInSegment(row.line.changedSpans)

	switch {
	case row.line.hasIntraline && vp.syntaxHighlighted != nil && syntax.Highlightable(dl.Kind):
		highlighted := vp.highlightIntralineBody(dl, row.line.diffIndex, match, row.line.changedSpans)
		body = ansi.Cut(highlighted, row.segment.displayStart, row.segment.displayEnd)
		bodyStyled = true
		segmentMatch = NoSearchMatch()
		changedSpans = nil
	case vp.syntaxHighlighted != nil && syntax.Highlightable(dl.Kind):
		highlighted := vp.highlightBody(dl, row.line.diffIndex, match)
		body = ansi.Cut(highlighted, row.segment.displayStart, row.segment.displayEnd)
		bodyStyled = true
		segmentMatch = NoSearchMatch()
	}

	return renderDiffLineSegmentSpansHL(
		dl,
		body,
		bodyStyled,
		changedSpans,
		row.line.hasIntraline,
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

func (vp *PatchViewport) renderSideBySideVisualRow(row visualRow, sourceCursor bool) string {
	leftWidth, rightWidth := sideBySideColumnWidths(vp.Width)
	left := vp.renderSideBySideCell(row.side.old, leftWidth, sideOld)
	right := vp.renderSideBySideCell(row.side.new, rightWidth, sideNew)
	line := left + sideBySideSeparator() + right
	if sourceCursor {
		separatorWidth := lipgloss.Width(sideBySideSeparator())
		line = vp.renderCursorRow(line,
			vp.sideCursorMatch(row.side.old, leftWidth, sideOld, 0),
			vp.sideCursorMatch(row.side.new, rightWidth, sideNew, leftWidth+separatorWidth),
		)
	}
	if vp.Width > 0 {
		return truncateToWidth(line, vp.Width)
	}
	return line
}

func renderCursorRow(line string, width int) string {
	if width <= 0 || line == "" {
		return line
	}
	return selectedStyle.Render(cursorRowText(line, width))
}

type cursorSpan struct {
	start int
	end   int
}

func (vp *PatchViewport) renderCursorRow(line string, spans ...cursorSpan) string {
	if vp.Width <= 0 || line == "" {
		return line
	}
	text := cursorRowText(line, vp.Width)
	if !vp.ColorEnabled {
		return text
	}
	return renderCursorSpans(text, spans)
}

func renderCursorSpans(line string, spans []cursorSpan) string {
	width := lipgloss.Width(line)
	valid := spans[:0]
	for _, span := range spans {
		span.start = max(span.start, 0)
		span.end = min(span.end, width)
		if span.start < span.end {
			valid = append(valid, span)
		}
	}
	if len(valid) == 0 {
		return selectedStyle.Render(line)
	}

	sort.Slice(valid, func(i, j int) bool { return valid[i].start < valid[j].start })
	var rendered strings.Builder
	position := 0
	for _, span := range valid {
		span.start = max(span.start, position)
		if span.start >= span.end {
			continue
		}
		if position < span.start {
			rendered.WriteString(selectedStyle.Render(ansi.Cut(line, position, span.start)))
		}
		rendered.WriteString(selectedStyle.Reverse(true).Render(ansi.Cut(line, span.start, span.end)))
		position = span.end
	}
	if position < width {
		rendered.WriteString(selectedStyle.Render(ansi.Cut(line, position, width)))
	}
	return rendered.String()
}

func (vp *PatchViewport) unifiedCursorMatch(row visualRow, match SearchMatch) cursorSpan {
	gutterWidth := 0
	if vp.GutterVisible {
		gutterWidth = lipgloss.Width(formatGutter(row.line.diff.OldNo, row.line.diff.NewNo, vp.gutterDigits))
	}
	bodyStart := gutterWidth + 1
	return vp.cursorMatch(row.segment, match, bodyStart, vp.Width-bodyStart)
}

func (vp *PatchViewport) sideCursorMatch(cell *sideBySideCell, width int, side sideBySideSide, offset int) cursorSpan {
	if cell == nil || vp.SearchQuery == "" || vp.SearchMatch.Line != cell.line.diffIndex {
		return cursorSpan{}
	}
	gutterWidth := 0
	if vp.GutterVisible {
		gutterWidth = lipgloss.Width(formatSideGutter(sideBySideLineNo(cell.line.diff, side), vp.gutterDigits))
	}
	bodyStart := gutterWidth + 1
	span := vp.cursorMatch(cell.segment, vp.SearchMatch, bodyStart, width-bodyStart)
	span.start += offset
	span.end += offset
	return span
}

func (vp *PatchViewport) cursorMatch(segment bodySegment, match SearchMatch, bodyStart, bodyWidth int) cursorSpan {
	match = segment.matchInSegment(match)
	if vp.SearchQuery == "" || bodyWidth <= 0 || !match.validForBody(segment.text) {
		return cursorSpan{}
	}

	start := lipgloss.Width(segment.text[:match.Start])
	end := lipgloss.Width(segment.text[:match.End])
	if vp.LineMode == model.LineModeScroll {
		start -= vp.XOffset
		end -= vp.XOffset
	}
	start = max(start, 0)
	end = min(end, bodyWidth)
	if start >= end {
		return cursorSpan{}
	}
	return cursorSpan{start: bodyStart + start, end: bodyStart + end}
}

func cursorRowText(line string, width int) string {
	line = padOrTruncateToWidth(ansi.Strip(line), width)
	return "▌" + ansi.Cut(line, 1, width)
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
	changedSpans := cell.segment.spansInSegment(cell.line.changedSpans)

	switch {
	case cell.line.hasIntraline && vp.syntaxHighlighted != nil && syntax.Highlightable(dl.Kind):
		highlighted := vp.highlightIntralineBody(dl, cell.line.diffIndex, match, cell.line.changedSpans)
		body = ansi.Cut(highlighted, cell.segment.displayStart, cell.segment.displayEnd)
		bodyStyled = true
		segmentMatch = NoSearchMatch()
		changedSpans = nil
	case vp.syntaxHighlighted != nil && syntax.Highlightable(dl.Kind):
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

		changedSpans: changedSpans,
		hasIntraline: cell.line.hasIntraline,
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

func (vp *PatchViewport) highlightIntralineBody(dl model.DiffLine, diffIndex int, match SearchMatch, spans []bodySpan) string {
	if vp.syntaxHighlighted == nil || !syntax.Highlightable(dl.Kind) {
		return dl.Text
	}
	background, _ := diffLineBackground(dl.Kind)
	return vp.syntaxHighlighted.HighlightLineSpansBackground(
		diffIndex,
		dl.Text,
		syntaxSpans(spans),
		syntaxMatch(match),
		background,
	)
}

func syntaxSpans(spans []bodySpan) []syntax.Span {
	if len(spans) == 0 {
		return nil
	}
	out := make([]syntax.Span, 0, len(spans))
	for _, span := range spans {
		out = append(out, syntax.Span{Start: span.start, End: span.end})
	}
	return out
}

func syntaxMatch(match SearchMatch) syntax.Span {
	return syntax.Span{Start: match.Start, End: match.End}
}

type diffLineLayers struct {
	gutter string
	prefix string
	body   string
	style  lipgloss.Style
	fill   bool

	bodyStyled bool

	changedSpans []bodySpan
	hasIntraline bool
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
	return renderDiffLineSegmentSpansHL(dl, body, bodyStyled, nil, false, width, query, match, gutterVisible, gutterDigits, lineMode, xOffset, continuation)
}

func renderDiffLineSegmentSpansHL(dl model.DiffLine, body string, bodyStyled bool, changedSpans []bodySpan, hasIntraline bool, width int, query string, match SearchMatch, gutterVisible bool, gutterDigits int, lineMode model.LineMode, xOffset int, continuation bool) string {
	if dl.Kind == model.LineNoNewline {
		return noNewlineStyle.Render("\\ No newline at end of file")
	}

	layers := buildDiffLineLayers(dl, gutterVisible, gutterDigits)
	layers.body = body
	layers.bodyStyled = bodyStyled
	layers.changedSpans = changedSpans
	layers.hasIntraline = hasIntraline
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
	visibleSpans := layers.changedSpans
	visibleMatch := match
	bodyPadding := ""
	if width > 0 {
		bodyWidth := diffLineBodyWidth(layers, width)
		if bodyWidth <= 0 {
			visibleBody = ""
			visibleSpans = nil
			visibleMatch = NoSearchMatch()
		} else {
			if layers.hasIntraline && !layers.bodyStyled {
				segment := bodySegmentForDisplayRange(visibleBody, xOffset, bodyWidth)
				visibleBody = segment.text
				visibleSpans = segment.spansInSegment(visibleSpans)
				visibleMatch = segment.matchInSegment(match)
			} else {
				if xOffset > 0 {
					visibleBody = ansi.TruncateLeft(visibleBody, xOffset, "")
				}
				visibleBody = ansi.Truncate(visibleBody, bodyWidth, "")
			}
			bodyPadding = diffLinePadding(visibleBody, bodyWidth, layers.fill)
		}
	}

	body := layers.prefix + visibleBody + bodyPadding
	if layers.hasIntraline && !layers.bodyStyled {
		body = renderIntralineBody(layers.prefix, visibleBody, bodyPadding, visibleSpans, visibleMatch, layers.style, continuation)
	} else if layers.bodyStyled {
		body = layers.style.Render(layers.prefix) + visibleBody
		if bodyPadding != "" {
			body += layers.style.Render(bodyPadding)
		}
	}

	if layers.gutter != "" {
		if layers.hasIntraline && !layers.bodyStyled {
			return layers.gutter + body
		}
		if !layers.bodyStyled && query != "" && match.validForBody(layers.body) {
			return layers.gutter + highlightSpan(body, bodySpanToVisibleContentSpan(match, layers.prefix, xOffset, visibleBody), layers.style)
		}
		if layers.bodyStyled {
			return layers.gutter + body
		}
		return layers.gutter + layers.style.Render(body)
	}

	if layers.hasIntraline && !layers.bodyStyled {
		return body
	}

	if !layers.bodyStyled && query != "" && match.validForBody(layers.body) {
		return highlightSpan(body, bodySpanToVisibleContentSpan(match, layers.prefix, xOffset, visibleBody), layers.style)
	}

	if layers.bodyStyled {
		return body
	}
	return layers.style.Render(body)
}

func bodySegmentForDisplayRange(body string, displayStart, width int) bodySegment {
	if body == "" || width <= 0 {
		return bodySegment{text: "", start: 0, end: 0}
	}
	if displayStart < 0 {
		displayStart = 0
	}

	start, startDisplay := bodyStartByteAtDisplayColumn(body, displayStart)
	end, endDisplay := bodyEndByteAtDisplayColumn(body, startDisplay+width)
	return bodySegmentRange(body, start, end, startDisplay, endDisplay, 0)
}

func bodyStartByteAtDisplayColumn(body string, column int) (int, int) {
	idx, displayCol, exact := bodyByteAtDisplayColumn(body, column)
	if exact || idx >= len(body) {
		return idx, displayCol
	}
	cluster, clusterWidth := ansi.FirstGraphemeCluster(body[idx:], ansi.GraphemeWidth)
	return idx + len(cluster), displayCol + clusterWidth
}

func bodyEndByteAtDisplayColumn(body string, column int) (int, int) {
	idx, displayCol, _ := bodyByteAtDisplayColumn(body, column)
	return idx, displayCol
}

func bodyByteAtDisplayColumn(body string, column int) (idx int, displayCol int, exact bool) {
	if column <= 0 {
		return 0, 0, true
	}

	for i := 0; i < len(body); {
		cluster, clusterWidth := ansi.FirstGraphemeCluster(body[i:], ansi.GraphemeWidth)
		if cluster == "" {
			break
		}
		if displayCol+clusterWidth > column {
			return i, displayCol, false
		}
		i += len(cluster)
		displayCol += clusterWidth
		if displayCol == column {
			return i, displayCol, true
		}
	}
	return len(body), displayCol, true
}

func renderIntralineBody(prefix, body, padding string, spans []bodySpan, match SearchMatch, style lipgloss.Style, continuation bool) string {
	var b strings.Builder
	if continuation {
		b.WriteString(prefix)
	} else {
		b.WriteString(style.Render(prefix))
	}
	b.WriteString(renderBodySpans(body, spans, match, style))
	b.WriteString(padding)
	return b.String()
}

func renderBodySpans(body string, spans []bodySpan, match SearchMatch, style lipgloss.Style) string {
	hasMatch := match.validForBody(body)
	if len(spans) == 0 && !hasMatch {
		return body
	}

	boundaries := bodyStyleBoundaries(body, spans, match, hasMatch)
	var b strings.Builder
	for i := 0; i < len(boundaries)-1; i++ {
		start, end := boundaries[i], boundaries[i+1]
		if start >= end {
			continue
		}
		segment := body[start:end]
		changed := spansOverlap(spans, start, end)
		matched := hasMatch && start < match.End && end > match.Start
		switch {
		case changed && matched:
			b.WriteString(style.Reverse(true).Render(segment))
		case changed:
			b.WriteString(style.Render(segment))
		case matched:
			b.WriteString(lipgloss.NewStyle().Reverse(true).Render(segment))
		default:
			b.WriteString(segment)
		}
	}
	return b.String()
}

func bodyStyleBoundaries(body string, spans []bodySpan, match SearchMatch, hasMatch bool) []int {
	boundaries := []int{0, len(body)}
	for _, span := range spans {
		start := max(min(span.start, len(body)), 0)
		end := max(min(span.end, len(body)), start)
		if start < end {
			boundaries = append(boundaries, start, end)
		}
	}
	if hasMatch {
		boundaries = append(boundaries, match.Start, match.End)
	}
	sort.Ints(boundaries)
	return compactInts(boundaries)
}

func compactInts(values []int) []int {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

func spansOverlap(spans []bodySpan, start, end int) bool {
	for _, span := range spans {
		if start < span.end && end > span.start {
			return true
		}
	}
	return false
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
func renderFileSeparator(path string, width int) string {
	label := " File: " + path + " "
	if width > 0 {
		label = padOrTruncateToWidth(label, width)
	}
	return fileHeaderStyle.Render(label)
}

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

func (r visualRow) sourceLine() (int, bool) {
	if r.note != nil || r.line.typ != lineTypeDiff {
		return 0, false
	}
	line := r.newLineNo()
	if line == nil {
		return 0, false
	}
	return *line, true
}

func (r visualRow) sourceDiffIndex() (int, bool) {
	if _, ok := r.sourceLine(); !ok {
		return 0, false
	}
	if r.side == nil {
		return r.line.diffIndex, true
	}
	if r.side.new == nil {
		return 0, false
	}
	return r.side.new.line.diffIndex, true
}

func cursorTargets(rows []visualRow) []cursorTarget {
	targets := make([]cursorTarget, 0)
	lastDiff := -1
	lastNote := ""
	for _, row := range rows {
		if row.note != nil {
			if row.note.id != "" && row.note.id != lastNote {
				targets = append(targets, cursorTarget{diffIndex: -1, noteID: row.note.id})
				lastNote = row.note.id
			}
			continue
		}
		index, ok := row.sourceDiffIndex()
		if !ok || index == lastDiff {
			continue
		}
		targets = append(targets, cursorTarget{diffIndex: index})
		lastDiff = index
		lastNote = ""
	}
	return targets
}

func (vp *PatchViewport) ensureSourceCursor(rows []visualRow) {
	if vp.sourceCursor >= 0 {
		return
	}
	start := min(max(vp.ScrollOffset, 0), len(rows))
	for i := start; i < len(rows); i++ {
		if index, ok := rows[i].sourceDiffIndex(); ok {
			vp.sourceCursor = index
			return
		}
	}
	for i := start - 1; i >= 0; i-- {
		if index, ok := rows[i].sourceDiffIndex(); ok {
			vp.sourceCursor = index
			return
		}
	}
}

func (vp *PatchViewport) ensureSourceCursorVisible(rows []visualRow) {
	for i, row := range rows {
		index, ok := row.sourceDiffIndex()
		if !ok || index != vp.sourceCursor {
			continue
		}
		if i < vp.ScrollOffset {
			vp.ScrollOffset = i
		} else if vp.Height > 0 && i >= vp.ScrollOffset+vp.Height {
			vp.ScrollOffset = i - vp.Height + 1
		}
		return
	}
}

func (vp *PatchViewport) ensureNoteVisible(rows []visualRow, id string) {
	first, last := -1, -1
	for i, row := range rows {
		if row.note == nil || row.note.id != id {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 || vp.Height <= 0 {
		return
	}
	if first < vp.ScrollOffset || last-first+1 > vp.Height {
		vp.ScrollOffset = first
	} else if last >= vp.ScrollOffset+vp.Height {
		vp.ScrollOffset = last - vp.Height + 1
	}
}

func (vp *PatchViewport) ensureActiveVisible(rows []visualRow) {
	switch {
	case vp.noteDraft != nil:
		vp.ensureNoteVisible(rows, vp.noteDraft.NoteID)
	case vp.selectedNoteID != "":
		vp.ensureNoteVisible(rows, vp.selectedNoteID)
	default:
		vp.ensureSourceCursorVisible(rows)
	}
}

func (vp *PatchViewport) SyncSourceCursor() {
	vp.manualScroll = false
	vp.sourceCursor = -1
	vp.ensureSourceCursor(vp.visualRows())
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

// CurrentSourceLine returns the line selected by the current-source cursor.
func (vp *PatchViewport) CurrentSourceLine() (int, bool) {
	rows := vp.visualRows()
	vp.ensureSourceCursor(rows)
	for _, row := range rows {
		index, ok := row.sourceDiffIndex()
		if !ok || index != vp.sourceCursor {
			continue
		}
		return row.sourceLine()
	}
	return 0, false
}

func (vp *PatchViewport) ScrollToNote(id string) bool {
	for i, row := range vp.visualRows() {
		if row.note != nil && row.note.id == id {
			vp.ScrollOffset = i
			vp.manualScroll = false
			vp.SyncCurrentHunk()
			return true
		}
	}
	return false
}

// Styles for patch rendering.
var (
	fileHeaderStyle = lipgloss.NewStyle().
			Foreground(theme.BrightText).
			Background(theme.SelectedBg).
			Bold(true)

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
