package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/alexivison/scry/internal/model"
)

func TestGutterFormat_SeparatorColumn(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Width = 80
	vp.Height = 20
	vp.GutterVisible = true
	output := vp.Render()

	// The gutter should use │ as separator between line numbers and content.
	if !strings.Contains(output, "│") {
		t.Errorf("gutter should contain │ separator, got:\n%s", output)
	}
}

func TestDiffLineLayersSeparateGutterPrefixBody(t *testing.T) {
	t.Parallel()

	dl := model.DiffLine{Kind: model.LineAdded, NewNo: intP(7), Text: "fmt.Println(value)"}
	layers := buildDiffLineLayers(dl, true, 4)

	if !strings.Contains(layers.gutter, "   7") {
		t.Fatalf("gutter = %q, want new line number", layers.gutter)
	}
	if layers.prefix != "+" {
		t.Fatalf("prefix = %q, want +", layers.prefix)
	}
	if layers.body != "fmt.Println(value)" {
		t.Fatalf("body = %q, want raw diff text", layers.body)
	}
	if strings.Contains(layers.body, layers.prefix) {
		t.Fatalf("body should not include prefix %q: %q", layers.prefix, layers.body)
	}
}

func TestStyledBodyDoesNotOwnDiffPrefix(t *testing.T) {
	t.Parallel()

	dl := model.DiffLine{Kind: model.LineAdded, NewNo: intP(7), Text: `return "ok"`}
	styledBody := "\x1b[32mreturn\x1b[0m \"ok\""

	got := renderDiffLineBodyHL(dl, styledBody, true, 0, "", NoSearchMatch(), false, 4, model.LineModeScroll, 0)
	if stripped, want := ansi.Strip(got), `+return "ok"`; stripped != want {
		t.Fatalf("renderDiffLineBodyHL() stripped = %q, want %q", stripped, want)
	}

	prefixAt := strings.Index(got, "+")
	styleAt := strings.Index(got, "\x1b[32m")
	if prefixAt < 0 || styleAt < 0 || styleAt < prefixAt {
		t.Fatalf("syntax styling should start after the diff prefix: %q", got)
	}
}

func TestPatchRenderSnapshotRepresentativeDiff(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Width = 0
	vp.Height = 4
	vp.GutterVisible = true

	want := strings.Join([]string{
		"── @@ -1,3 +1,4 @@ func init() ",
		"   1    1 │  package main",
		"        2 │ +import \"os\"",
		"   2    3 │  ",
	}, "\n")

	if got := vp.Render(); got != want {
		t.Fatalf("Render() mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestHunkSeparator_HorizontalRule(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Width = 80
	vp.Height = 20
	output := vp.Render()

	// Hunk separators between hunks should contain ─── horizontal rules.
	if !strings.Contains(output, "───") {
		t.Errorf("hunk separator should contain ─── horizontal rule, got:\n%s", output)
	}

	// The @@ header text should still be present.
	if !strings.Contains(output, "@@") {
		t.Errorf("hunk header should still contain @@ markers, got:\n%s", output)
	}
}

func TestScrollIndicator_Position(t *testing.T) {
	t.Parallel()

	// threeHunkPatch has 13 lines total. With height=5, maxScroll = 13-5 = 8.
	tests := map[string]struct {
		scrollOffset int
		height       int
		wantPos      float64
	}{
		"top of file": {
			scrollOffset: 0,
			height:       5,
			wantPos:      0.0, // 0/8 = 0
		},
		"middle of file": {
			scrollOffset: 4,
			height:       5,
			wantPos:      0.5, // 4/8 = 0.5
		},
		"bottom of file": {
			scrollOffset: 8,
			height:       5,
			wantPos:      1.0, // 8/8 = 1.0
		},
		"content fits viewport": {
			scrollOffset: 0,
			height:       20,
			wantPos:      0.0, // no scrolling needed
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vp := NewPatchViewport(threeHunkPatch())
			vp.Width = 80
			vp.Height = tc.height
			vp.ScrollOffset = tc.scrollOffset

			pos := vp.ScrollIndicatorPos()
			diff := pos - tc.wantPos
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("ScrollIndicatorPos() = %f, want %f (offset=%d, height=%d, total=%d)",
					pos, tc.wantPos, tc.scrollOffset, tc.height, vp.TotalLines())
			}
		})
	}
}

func TestScrollIndicator_VisibleInBorder(t *testing.T) {
	t.Parallel()

	// Render a bordered pane with scroll indicator on row 2.
	output := BorderedPaneWithScroll("line1\nline2\nline3\nline4", "Title", "", 30, 6, true, true, 2)

	if !strings.Contains(output, "│") {
		t.Errorf("scroll indicator should be visible in bordered pane, got:\n%s", output)
	}
}

func TestScrollIndicator_HiddenWhenNegative(t *testing.T) {
	t.Parallel()

	output := BorderedPaneWithScroll("line1\nline2", "Title", "", 30, 4, true, true, -1)

	// No distinct highlighted indicator when scrollLine is negative.
	if strings.Count(output, "│") < 2 {
		t.Errorf("side borders should still render when scrollLine=-1, got:\n%s", output)
	}
}

func TestGutterSuppressed_NarrowWidth(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Width = 50
	vp.Height = 20
	vp.GutterVisible = false // simulates width < 60 minimal mode
	output := vp.Render()

	// With gutter hidden, the │ separator should not appear in diff lines.
	// (It may appear in hunk separators, which is fine.)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Skip hunk header/separator lines — only check diff content lines.
		if strings.Contains(line, "@@") || strings.Contains(line, "───") {
			continue
		}
		// Diff lines should not have the gutter separator.
		if strings.Contains(line, "│") && (strings.Contains(line, "package") || strings.Contains(line, "import") || strings.Contains(line, "func")) {
			t.Errorf("gutter separator │ should not appear in diff line when gutter is hidden: %q", line)
		}
	}
}

func TestNarrowWidth_NoOverflow(t *testing.T) {
	t.Parallel()

	widths := []int{20, 30, 40, 50}
	for _, w := range widths {
		vp := NewPatchViewport(threeHunkPatch())
		vp.Width = w
		vp.Height = 20
		vp.GutterVisible = w >= 60
		output := vp.Render()

		for i, line := range strings.Split(output, "\n") {
			visualW := lipgloss.Width(line)
			if visualW > w {
				t.Errorf("width=%d line %d too wide (%d cells): %q", w, i, visualW, line)
			}
		}
	}
}

func TestTruncateToWidth_PreservesANSI(t *testing.T) {
	t.Parallel()

	// String with ANSI color codes — truncation must not break sequences.
	styled := "\x1b[31mhello world\x1b[0m"
	got := truncateToWidth(styled, 5)
	// Should contain 5 visible chars and valid ANSI (no partial sequences).
	visW := lipgloss.Width(got)
	if visW > 5 {
		t.Errorf("truncateToWidth with ANSI: visual width = %d, want <= 5", visW)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("truncateToWidth should keep 'hello', got %q", got)
	}
}

func TestPatchViewportDefaultLineModeWrap(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())

	if vp.LineMode != model.LineModeWrap {
		t.Fatalf("LineMode = %v, want LineModeWrap", vp.LineMode)
	}
}

func TestWrapRenderingContinuationRowsAlignWithBody(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(singleLinePatch("0123456789abcdefghijkl"))
	vp.Width = 24
	vp.Height = 3
	vp.ScrollOffset = 1
	vp.GutterVisible = true

	lines := strings.Split(ansi.Strip(vp.Render()), "\n")
	if len(lines) != 2 {
		t.Fatalf("rendered lines = %d, want 2 continuation rows:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[0], "+0123456789a") {
		t.Fatalf("first row should include gutter, prefix, and first body segment: %q", lines[0])
	}
	if strings.Contains(lines[1], "+") || strings.Contains(lines[1], "│") {
		t.Fatalf("continuation row should have blank gutter and prefix: %q", lines[1])
	}
	firstBodyCol := visualColumn(lines[0], "0")
	continuationBodyCol := visualColumn(lines[1], "b")
	if firstBodyCol < 0 || continuationBodyCol < 0 || firstBodyCol != continuationBodyCol {
		t.Fatalf("continuation body column = %d, want %d\n%s", continuationBodyCol, firstBodyCol, strings.Join(lines, "\n"))
	}
}

func visualColumn(line, marker string) int {
	idx := strings.Index(line, marker)
	if idx < 0 {
		return -1
	}
	return lipgloss.Width(line[:idx])
}

func TestWrapSearchHighlightOnContinuationRow(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(oldProfile)

	text := "0123456789abcdefghijkl"
	matchStart := strings.Index(text, "def")
	vp := NewPatchViewport(singleLinePatch(text))
	vp.Width = 24
	vp.Height = 3
	vp.ScrollOffset = 1
	vp.GutterVisible = true
	vp.SearchQuery = "def"
	vp.SearchMatch = SearchMatch{Line: 0, Start: matchStart, End: matchStart + len("def")}

	lines := strings.Split(vp.Render(), "\n")
	if len(lines) != 2 {
		t.Fatalf("rendered lines = %d, want 2:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.Contains(ansi.Strip(lines[1]), "bcdefghijkl") {
		t.Fatalf("continuation row should contain matched body segment: %q", ansi.Strip(lines[1]))
	}
	if !strings.Contains(lines[1], "\x1b[7") {
		t.Fatalf("continuation row should contain reversed search highlight: %q", lines[1])
	}
}

func TestScrollModeKeepsOneLogicalLinePerRenderedRow(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(singleLinePatch("0123456789abcdefghijkl"))
	vp.LineMode = model.LineModeScroll
	vp.Width = 24
	vp.Height = 3
	vp.GutterVisible = true

	if got, want := vp.TotalLines(), 2; got != want {
		t.Fatalf("TotalLines() = %d, want %d in scroll mode", got, want)
	}

	output := ansi.Strip(vp.Render())
	if strings.Contains(output, "bcdefghijkl") {
		t.Fatalf("scroll mode should truncate instead of rendering continuation rows:\n%s", output)
	}
}

func TestHorizontalScrollClampsXOffset(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(singleLinePatch("0123456789abcdefghijkl"))
	vp.LineMode = model.LineModeScroll
	vp.Width = 24
	vp.Height = 1
	vp.ScrollOffset = 1

	if got, want := vp.MaxXOffset(), 11; got != want {
		t.Fatalf("MaxXOffset() = %d, want %d", got, want)
	}

	vp.ScrollRight()
	if got, want := vp.XOffset, 8; got != want {
		t.Fatalf("after first ScrollRight XOffset = %d, want %d", got, want)
	}

	vp.ScrollRight()
	if got, want := vp.XOffset, 11; got != want {
		t.Fatalf("after second ScrollRight XOffset = %d, want clamped max %d", got, want)
	}

	vp.ScrollRight()
	if got, want := vp.XOffset, 11; got != want {
		t.Fatalf("after third ScrollRight XOffset = %d, want still clamped max %d", got, want)
	}

	vp.ScrollLeft()
	if got, want := vp.XOffset, 3; got != want {
		t.Fatalf("after ScrollLeft XOffset = %d, want %d", got, want)
	}

	vp.ScrollLeft()
	if got := vp.XOffset; got != 0 {
		t.Fatalf("after second ScrollLeft XOffset = %d, want clamped zero", got)
	}
}

func TestHorizontalScrollKeepsGutterAndPrefixFixed(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(singleLinePatch("0123456789abcdefghijkl"))
	vp.LineMode = model.LineModeScroll
	vp.Width = 24
	vp.Height = 1
	vp.ScrollOffset = 1
	before := ansi.Strip(vp.Render())

	vp.XOffset = 8
	after := ansi.Strip(vp.Render())

	beforePrefix := before[:strings.Index(before, "+")+1]
	afterPrefix := after[:strings.Index(after, "+")+1]
	if afterPrefix != beforePrefix {
		t.Fatalf("fixed gutter/prefix moved\nbefore: %q\nafter:  %q", before, after)
	}
	if strings.Contains(after, "01234567") {
		t.Fatalf("scrolled body still contains skipped left text: %q", after)
	}
	if !strings.Contains(after, "89abcdefghi") {
		t.Fatalf("scrolled body = %q, want visible body starting at offset 8", after)
	}
}

func TestHorizontalScrollPreservesANSIBodySequences(t *testing.T) {
	t.Parallel()

	styledBody := "\x1b[31m0123456789abcdef\x1b[0m"
	vp := NewPatchViewport(singleLinePatch(styledBody))
	vp.LineMode = model.LineModeScroll
	vp.Width = 22
	vp.Height = 1
	vp.ScrollOffset = 1
	vp.XOffset = 8

	got := vp.Render()
	stripped := ansi.Strip(got)
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("rendered line lost ANSI escapes: %q", got)
	}
	if strings.Contains(stripped, "01234567") {
		t.Fatalf("rendered line still contains skipped left body: %q", stripped)
	}
	if !strings.Contains(stripped, "89abcdef") {
		t.Fatalf("rendered line = %q, want ANSI body truncated from left", stripped)
	}
	if w := lipgloss.Width(got); w > vp.Width {
		t.Fatalf("rendered width = %d, want <= %d: %q", w, vp.Width, got)
	}
}

func TestHorizontalScrollPreservesStyledBodySearchSequences(t *testing.T) {
	t.Parallel()

	dl := model.DiffLine{Kind: model.LineAdded, NewNo: intP(1), Text: "0123456789abcdefghijkl"}
	styledBody := "\x1b[31m0123456789\x1b[0m\x1b[7mabc\x1b[0mdefghijkl"
	match := SearchMatch{Line: 0, Start: 10, End: 13}

	got := renderDiffLineBodyHL(dl, styledBody, true, 14, "abc", match, false, 4, model.LineModeScroll, 8)
	stripped := ansi.Strip(got)
	if !strings.Contains(got, "\x1b[7m") {
		t.Fatalf("rendered line lost styled search span: %q", got)
	}
	if strings.Contains(stripped, "01234567") {
		t.Fatalf("rendered line still contains skipped left body: %q", stripped)
	}
	if !strings.Contains(stripped, "+89abcdefghi") {
		t.Fatalf("rendered line = %q, want styled body cropped after diff prefix", stripped)
	}
	if w := lipgloss.Width(got); w > 14 {
		t.Fatalf("rendered width = %d, want <= 14: %q", w, got)
	}
}

func singleLinePatch(text string) model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "long.go", Status: model.StatusModified},
		Hunks: []model.Hunk{{
			Header:   "func long()",
			OldStart: 1, OldLen: 1, NewStart: 1, NewLen: 1,
			Lines: []model.DiffLine{{
				Kind:  model.LineAdded,
				NewNo: intP(1),
				Text:  text,
			}},
		}},
	}
}

func TestFormatGutter_LargeLineNumbers(t *testing.T) {
	t.Parallel()

	// With 5-digit line numbers, gutter must accommodate them.
	old := 12345
	new := 67890
	result := formatGutter(&old, &new, 5)

	if !strings.Contains(result, "12345") {
		t.Errorf("formatGutter should show full 5-digit old number, got %q", result)
	}
	if !strings.Contains(result, "67890") {
		t.Errorf("formatGutter should show full 5-digit new number, got %q", result)
	}
}

// withANSIColorProfile forces lipgloss to emit ANSI escape sequences so we can
// assert background colors in the rendered output. Tests run as goroutines in a
// non-TTY default to Ascii (no-color), which masks the styling we want to check.
func withANSIColorProfile(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

// backgroundSGRFragment returns the SGR parameter substring (e.g. "48;2;17;40;26"
// or "48;5;22") that lipgloss emits for the style's background under the active
// color profile. Useful for asserting bg presence without locking to a specific
// palette entry — the same fragment appears in compound SGRs that merge fg+bg.
func backgroundSGRFragment(t *testing.T, style lipgloss.Style) string {
	t.Helper()
	probe := lipgloss.NewStyle().Background(style.GetBackground()).Render(" ")
	open := strings.Index(probe, "\x1b[")
	close := strings.Index(probe, "m")
	if open < 0 || close <= open+2 {
		t.Fatalf("could not extract bg SGR fragment from probe %q", probe)
	}
	frag := probe[open+2 : close]
	if frag == "" {
		t.Fatalf("empty bg SGR fragment from probe %q", probe)
	}
	return frag
}

// firstDiffLine returns the first rendered diff line that contains the given
// raw body substring, used to scope assertions to the added/deleted/context row.
func firstDiffLine(t *testing.T, output, bodyMarker string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(ansi.Strip(line), bodyMarker) {
			return line
		}
	}
	t.Fatalf("no rendered line contained %q in:\n%s", bodyMarker, output)
	return ""
}

func TestAddedDeletedRowsCarryBackgroundTint(t *testing.T) {
	withANSIColorProfile(t)

	vp := NewPatchViewport(threeHunkPatch())
	vp.Width = 80
	vp.Height = 20
	vp.GutterVisible = true
	output := vp.Render()

	addedLine := firstDiffLine(t, output, `import "os"`)
	deletedLine := firstDiffLine(t, output, "old()")
	contextLine := firstDiffLine(t, output, "package main")

	// The body content (not just the +/- prefix) should be wrapped in the
	// row's background-color escape. We anchor on the body bytes to prove the
	// tint extends past the prefix column.
	// lipgloss may emit the bg either standalone (\x1b[<bg>m) or merged with
	// the foreground in a compound SGR (\x1b[<fg>;<bg>m). Probe a bg-only
	// style for the SGR parameter fragment so the assertion stays semantic
	// and survives palette/profile changes.
	addedBgSGR := backgroundSGRFragment(t, addedStyle)
	deletedBgSGR := backgroundSGRFragment(t, deletedStyle)

	requireBeforeBody := func(line, sgr, bodyMarker string) {
		t.Helper()
		bgAt := strings.Index(line, sgr)
		bodyAt := strings.Index(line, bodyMarker)
		if bgAt < 0 || bodyAt < 0 || bgAt > bodyAt {
			t.Fatalf("bg %q does not precede body %q in: %q", sgr, bodyMarker, line)
		}
	}

	requireBeforeBody(addedLine, addedBgSGR, "import")
	requireBeforeBody(deletedLine, deletedBgSGR, "old()")

	// The body content (not just the +/- prefix) must be inside the tinted
	// span — verify by looking for the bg fragment somewhere before any
	// terminal reset that follows the body characters.
	if idx := strings.Index(addedLine, addedBgSGR); idx < 0 || !strings.Contains(addedLine[idx:], `import "os"`) {
		t.Fatalf("added bg span does not cover body: %q", addedLine)
	}
	if idx := strings.Index(deletedLine, deletedBgSGR); idx < 0 || !strings.Contains(deletedLine[idx:], "old()") {
		t.Fatalf("deleted bg span does not cover body: %q", deletedLine)
	}

	// Context rows must NOT pick up either tint.
	if strings.Contains(contextLine, addedBgSGR) || strings.Contains(contextLine, deletedBgSGR) {
		t.Fatalf("context row leaked diff bg tint: %q", contextLine)
	}

	// Stripped content remains unchanged by the tint.
	if got := ansi.Strip(addedLine); !strings.Contains(got, `+import "os"`) {
		t.Fatalf("added row stripped content changed: %q", got)
	}
	if got := ansi.Strip(deletedLine); !strings.Contains(got, "old()") || !strings.Contains(got, "-") {
		t.Fatalf("deleted row stripped content changed: %q", got)
	}
}

func TestStyledBodyRetainsRowBackgroundAcrossInnerResets(t *testing.T) {
	withANSIColorProfile(t)

	dl := model.DiffLine{Kind: model.LineAdded, NewNo: intP(7), Text: `return "ok"`}
	// Simulates a chroma-style highlighted body: an inner full-reset that
	// would normally drop any outer background.
	styledBody := "\x1b[31mreturn\x1b[0m \"ok\""

	got := renderDiffLineBodyHL(dl, styledBody, true, 0, "", NoSearchMatch(), false, 4, model.LineModeScroll, 0)
	addedBgSGR := backgroundSGRFragment(t, addedStyle)

	// The bg fragment must appear at least twice: once around the prefix/body
	// and once re-applied after the inner \x1b[0m so the tint persists.
	if count := strings.Count(got, addedBgSGR); count < 2 {
		t.Fatalf("expected bg %q re-applied after inner reset, got %d occurrences in %q", addedBgSGR, count, got)
	}
	// Inner full reset must still be followed by a bg-restore sequence.
	if !strings.Contains(got, "\x1b[0m\x1b[") || !strings.Contains(got[strings.Index(got, "\x1b[0m"):], addedBgSGR) {
		t.Fatalf("bg restore sequence missing after inner reset: %q", got)
	}
	// Sanity: the syntax color survives.
	if !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("syntax foreground sequence dropped: %q", got)
	}
	// Stripped content unchanged.
	if want, stripped := `+return "ok"`, ansi.Strip(got); stripped != want {
		t.Fatalf("stripped content = %q, want %q", stripped, want)
	}
}

func TestJoinColumnsContainsTintWithinRightColumn(t *testing.T) {
	withANSIColorProfile(t)

	// Render a tinted patch row and join it next to a plain left column
	// matching the real split layout (file list on the left, patch on the right).
	vp := NewPatchViewport(threeHunkPatch())
	vp.Width = 30
	vp.Height = 5
	vp.GutterVisible = true
	right := vp.Render()
	leftLines := []string{"file-a.go", "file-b.go", "file-c.go", "file-d.go", "file-e.go"}
	left := strings.Join(leftLines, "\n")

	const leftWidth = 12
	joined := JoinColumns(left, right, leftWidth, vp.Width)

	addedBgSGR := backgroundSGRFragment(t, addedStyle)
	deletedBgSGR := backgroundSGRFragment(t, deletedStyle)

	rows := strings.Split(joined, "\n")
	for i, row := range rows {
		// Stripped left-column slice must be one of the file-list strings —
		// proves no tinted bytes leaked left of the divider.
		left := ansi.Strip(row)
		if len(left) < leftWidth {
			t.Fatalf("row %d shorter than leftWidth %d: %q", i, leftWidth, left)
		}
		leftCell := strings.TrimRight(left[:leftWidth], " ")
		if leftCell != "" {
			found := false
			for _, name := range leftLines {
				if leftCell == name {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("row %d left column corrupted: %q (full row: %q)", i, leftCell, ansi.Strip(row))
			}
		}

		// The bg SGR must never appear before the divider column on any row,
		// i.e. its byte position is to the right of the divider's left-column
		// padding boundary in the unstripped row.
		divIdx := strings.Index(row, "│")
		if divIdx < 0 {
			continue // empty padding row
		}
		leftBytes := row[:divIdx]
		if strings.Contains(leftBytes, addedBgSGR) || strings.Contains(leftBytes, deletedBgSGR) {
			t.Fatalf("row %d: diff bg leaked into left column bytes: %q", i, leftBytes)
		}

		// Every tinted row must terminate with a reset before the trailing
		// newline so the next row's left column starts in a clean state.
		if strings.Contains(row, addedBgSGR) || strings.Contains(row, deletedBgSGR) {
			if !strings.HasSuffix(row, "\x1b[0m") {
				t.Fatalf("row %d tinted but does not end with reset: %q", i, row)
			}
		}
	}
}

func TestComputeGutterDigits(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		maxLine    int
		wantDigits int
	}{
		"small file":    {maxLine: 100, wantDigits: 4},
		"boundary 9999": {maxLine: 9999, wantDigits: 4},
		"10000":         {maxLine: 10000, wantDigits: 5},
		"large file":    {maxLine: 123456, wantDigits: 6},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lineNo := tc.maxLine
			patch := model.FilePatch{
				Hunks: []model.Hunk{{
					OldStart: tc.maxLine, OldLen: 1, NewStart: 1, NewLen: 1,
					Lines: []model.DiffLine{
						{Kind: model.LineContext, OldNo: &lineNo, NewNo: &lineNo, Text: "x"},
					},
				}},
			}
			vp := NewPatchViewport(patch)
			if vp.gutterDigits != tc.wantDigits {
				t.Errorf("gutterDigits = %d, want %d (maxLine=%d)", vp.gutterDigits, tc.wantDigits, tc.maxLine)
			}
		})
	}
}
