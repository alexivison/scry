package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/terminal"
	"github.com/alexivison/scry/internal/ui/syntax"
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

	got := renderDiffLineBodyHL(dl, styledBody, true, 0, "", NoSearchMatch(), false, 4, model.LineModeWrap, 0)
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

func TestFolderPatchRendersFileSeparators(t *testing.T) {
	t.Parallel()

	patch := model.FilePatch{Hunks: []model.Hunk{
		{FilePath: "internal/a.go", OldStart: 0, OldLen: 0, NewStart: 1, NewLen: 1, Lines: []model.DiffLine{{Kind: model.LineAdded, NewNo: intP(1), Text: "a"}}},
		{FilePath: "internal/b.go", OldStart: 0, OldLen: 0, NewStart: 1, NewLen: 1, Lines: []model.DiffLine{{Kind: model.LineAdded, NewNo: intP(1), Text: "b"}}},
	}}
	for _, mode := range []model.PatchDiffMode{model.PatchDiffModeUnified, model.PatchDiffModeSideBySide} {
		vp := NewPatchViewport(patch)
		vp.DiffMode = mode
		vp.Width = 80
		vp.Height = 20

		output := ansi.Strip(vp.Render())
		for _, path := range []string{"File: internal/a.go", "File: internal/b.go"} {
			if !strings.Contains(output, path) {
				t.Fatalf("%v output missing %q:\n%s", mode, path, output)
			}
		}
	}
}

func TestChangedLinesUseBrightFullWidthBackground(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(oldProfile)

	tests := []struct {
		name   string
		line   model.DiffLine
		prefix string
		bg     string
	}{
		{
			name:   "added",
			line:   model.DiffLine{Kind: model.LineAdded, NewNo: intP(1), Text: "new()"},
			prefix: "+",
			bg:     "48;2;0;95;0",
		},
		{
			name:   "deleted",
			line:   model.DiffLine{Kind: model.LineDeleted, OldNo: intP(1), Text: "old()"},
			prefix: "-",
			bg:     "48;2;139;0;0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderDiffLineHL(tc.line, 16, "", NoSearchMatch(), false, 4, model.LineModeWrap, 0)
			if !strings.Contains(got, tc.bg) {
				t.Fatalf("rendered line missing diff background %s: %q", tc.bg, got)
			}
			stripped := ansi.Strip(got)
			if !strings.HasPrefix(stripped, tc.prefix+tc.line.Text) {
				t.Fatalf("stripped line = %q, want prefix %q", stripped, tc.prefix+tc.line.Text)
			}
			if width := lipgloss.Width(stripped); width != 16 {
				t.Fatalf("rendered line width = %d, want 16: %q", width, stripped)
			}
		})
	}
}

func TestWrappedPairedParagraphHighlightsOnlyChangedWords(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(oldProfile)

	oldText := "Alpha beta gamma delta epsilon zeta eta theta iota kappa lambda."
	newText := "Alpha beta gamma delta OMEGA zeta eta theta iota kappa lambda."
	patch := model.FilePatch{
		Summary: model.FileSummary{Path: "notes.md", Status: model.StatusModified},
		Hunks: []model.Hunk{{
			OldStart: 1, OldLen: 1, NewStart: 1, NewLen: 1,
			Lines: []model.DiffLine{
				{Kind: model.LineDeleted, OldNo: intP(1), Text: oldText},
				{Kind: model.LineAdded, NewNo: intP(1), Text: newText},
			},
		}},
	}

	tests := map[string]struct {
		mode  model.PatchDiffMode
		width int
	}{
		"unified":      {mode: model.PatchDiffModeUnified, width: 36},
		"side-by-side": {mode: model.PatchDiffModeSideBySide, width: 80},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			vp := NewPatchViewport(patch)
			vp.DiffMode = tc.mode
			vp.Width = tc.width
			vp.Height = 20
			vp.GutterVisible = false

			got := vp.Render()

			assertOnlyNeedleHasBackground(t, got, "epsilon", []string{"Alpha beta", "zeta"}, "48;2;139;0;0")
			assertOnlyNeedleHasBackground(t, got, "OMEGA", []string{"Alpha beta", "zeta"}, "48;2;0;95;0")
		})
	}
}

func TestPairedLineHighlightsSeparateChangedWords(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(oldProfile)

	vp := NewPatchViewport(model.FilePatch{
		Summary: model.FileSummary{Path: "notes.md", Status: model.StatusModified},
		Hunks: []model.Hunk{{
			OldStart: 1, OldLen: 1, NewStart: 1, NewLen: 1,
			Lines: []model.DiffLine{
				{Kind: model.LineDeleted, OldNo: intP(1), Text: "alpha one middle two omega"},
				{Kind: model.LineAdded, NewNo: intP(1), Text: "alpha ONE middle TWO omega"},
			},
		}},
	})
	vp.Width = 0
	vp.Height = 3
	vp.GutterVisible = false

	got := vp.Render()

	assertOnlyNeedleHasBackground(t, got, "one", []string{"middle"}, "48;2;139;0;0")
	assertOnlyNeedleHasBackground(t, got, "two", []string{"middle"}, "48;2;139;0;0")
	assertOnlyNeedleHasBackground(t, got, "ONE", []string{"middle"}, "48;2;0;95;0")
	assertOnlyNeedleHasBackground(t, got, "TWO", []string{"middle"}, "48;2;0;95;0")
}

func TestPairedIntralineChangesPreserveSyntaxHighlighting(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(oldProfile)

	vp := NewPatchViewport(model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{{
			OldStart: 1, OldLen: 1, NewStart: 1, NewLen: 1,
			Lines: []model.DiffLine{
				{Kind: model.LineDeleted, OldNo: intP(1), Text: `return "old"`},
				{Kind: model.LineAdded, NewNo: intP(1), Text: `return "new"`},
			},
		}},
	})
	vp.Width = 0
	vp.Height = 3
	vp.GutterVisible = false
	vp.SetSyntaxHighlighter(syntax.NewLineCache("main.go", "", "", terminal.ColorANSI256))

	got := vp.Render()
	wantKeyword := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true).Render("return")
	if !strings.Contains(got, wantKeyword) {
		t.Fatalf("paired intraline lines should preserve syntax highlighting for unchanged code, got:\n%q", got)
	}
}

func TestIntralineHorizontalScrollDoesNotOverflowWideRunes(t *testing.T) {
	vp := NewPatchViewport(model.FilePatch{
		Summary: model.FileSummary{Path: "wide.txt", Status: model.StatusModified},
		Hunks: []model.Hunk{{
			OldStart: 1, OldLen: 1, NewStart: 1, NewLen: 1,
			Lines: []model.DiffLine{
				{Kind: model.LineDeleted, OldNo: intP(1), Text: "abcdefg界XYZold"},
				{Kind: model.LineAdded, NewNo: intP(1), Text: "abcdefg界XYZnew"},
			},
		}},
	})
	vp.LineMode = model.LineModeScroll
	vp.Width = 5
	vp.Height = 1
	vp.ScrollOffset = 1
	vp.XOffset = 8
	vp.GutterVisible = false

	got := vp.Render()
	if width := lipgloss.Width(ansi.Strip(got)); width > vp.Width {
		t.Fatalf("rendered line width = %d, want <= %d: %q", width, vp.Width, got)
	}
}

func TestSideBySideChangedCellsUseBrightFullWidthBackground(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(oldProfile)

	tests := []struct {
		name   string
		line   model.DiffLine
		side   sideBySideSide
		prefix string
		bg     string
	}{
		{
			name:   "added",
			line:   model.DiffLine{Kind: model.LineAdded, NewNo: intP(1), Text: "new()"},
			side:   sideNew,
			prefix: "+",
			bg:     "48;2;0;95;0",
		},
		{
			name:   "deleted",
			line:   model.DiffLine{Kind: model.LineDeleted, OldNo: intP(1), Text: "old()"},
			side:   sideOld,
			prefix: "-",
			bg:     "48;2;139;0;0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vp := NewPatchViewport(model.FilePatch{})
			vp.GutterVisible = false
			vp.LineMode = model.LineModeWrap
			cell := &sideBySideCell{
				line:    patchLine{typ: lineTypeDiff, diff: tc.line, diffIndex: 0},
				segment: fullBodySegment(tc.line.Text),
			}

			got := vp.renderSideBySideCell(cell, 16, tc.side)
			if !strings.Contains(got, tc.bg) {
				t.Fatalf("rendered side-by-side cell missing diff background %s: %q", tc.bg, got)
			}
			if !strings.HasSuffix(got, "\x1b[0m") {
				t.Fatalf("rendered side-by-side cell should keep trailing padding inside the diff background: %q", got)
			}
			stripped := ansi.Strip(got)
			if !strings.HasPrefix(stripped, tc.prefix+tc.line.Text) {
				t.Fatalf("stripped cell = %q, want prefix %q", stripped, tc.prefix+tc.line.Text)
			}
			if width := lipgloss.Width(stripped); width != 16 {
				t.Fatalf("rendered cell width = %d, want 16: %q", width, stripped)
			}
		})
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

func TestPatchViewportDefaultDiffModeUnified(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())

	if vp.DiffMode != model.PatchDiffModeUnified {
		t.Fatalf("DiffMode = %v, want PatchDiffModeUnified", vp.DiffMode)
	}
}

func TestSideBySideRenderingSplitsOldAndNewColumns(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.DiffMode = model.PatchDiffModeSideBySide
	vp.Width = 90
	vp.Height = 20
	vp.GutterVisible = true

	output := ansi.Strip(vp.Render())
	lines := strings.Split(output, "\n")

	contextLine := findLineContaining(lines, "package main")
	if contextLine == "" {
		t.Fatalf("side-by-side output missing context line:\n%s", output)
	}
	if strings.Count(contextLine, "package main") != 2 {
		t.Fatalf("context should render in both columns, got %q", contextLine)
	}

	changeLine := findLineContaining(lines, "old()")
	if changeLine == "" || !strings.Contains(changeLine, "new()") {
		t.Fatalf("adjacent delete/add should share a row, got:\n%s", output)
	}
	oldIdx := strings.Index(changeLine, "old()")
	newIdx := strings.Index(changeLine, "new()")
	if oldIdx < 0 || newIdx < 0 || newIdx <= oldIdx {
		t.Fatalf("deleted content should be left of added content, got %q", changeLine)
	}
	if dashIdx := strings.Index(changeLine, "-"); dashIdx < 0 || dashIdx > oldIdx {
		t.Fatalf("deleted side should include '-' prefix before old content, got %q", changeLine)
	}
	if plusIdx := strings.Index(changeLine, "+"); plusIdx < 0 || plusIdx > newIdx {
		t.Fatalf("added side should include '+' prefix before new content, got %q", changeLine)
	}
	for _, want := range []string{"  11 │", "  12 │"} {
		if !strings.Contains(changeLine, want) {
			t.Fatalf("side-by-side change line missing line number %q: %q", want, changeLine)
		}
	}
}

func TestSideBySideDiffLineMappingFindsPairedAddedLine(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.DiffMode = model.PatchDiffModeSideBySide
	vp.Width = 90
	vp.Height = 20
	vp.GutterVisible = true

	if got, want := vp.DiffLineToViewportLine(5), 6; got != want {
		t.Fatalf("DiffLineToViewportLine(added paired line) = %d, want %d", got, want)
	}
	if got, ok := vp.ScrollOffsetForDiffLine(5, 0); !ok || got != 6 {
		t.Fatalf("ScrollOffsetForDiffLine(added paired line) = %d, %v; want 6, true", got, ok)
	}
}

func findLineContaining(lines []string, needle string) string {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func assertOnlyNeedleHasBackground(t *testing.T, output, changed string, unchanged []string, bg string) {
	t.Helper()
	lines := strings.Split(output, "\n")
	line := findLineContaining(lines, changed)
	if line == "" {
		t.Fatalf("missing rendered line for %q:\n%q", changed, output)
	}
	if !backgroundActiveAt(line, strings.Index(line, changed), bg) {
		t.Fatalf("changed text %q lacks background %s in line:\n%q", changed, bg, line)
	}

	for _, text := range unchanged {
		indexes := allIndexes(line, text)
		if len(indexes) == 0 {
			t.Fatalf("line for %q is missing unchanged text %q:\n%q", changed, text, line)
		}
		for _, index := range indexes {
			if backgroundActiveAt(line, index, bg) {
				t.Fatalf("unchanged text %q should not have background %s in line:\n%q", text, bg, line)
			}
		}
	}
}

func allIndexes(s, substr string) []int {
	var indexes []int
	for offset := 0; offset < len(s); {
		idx := strings.Index(s[offset:], substr)
		if idx < 0 {
			return indexes
		}
		indexes = append(indexes, offset+idx)
		offset += idx + len(substr)
	}
	return indexes
}

func backgroundActiveAt(line string, index int, bg string) bool {
	if index < 0 {
		return false
	}
	before := line[:index]
	bgAt := strings.LastIndex(before, bg)
	if bgAt < 0 {
		return false
	}
	return bgAt > strings.LastIndex(before, "\x1b[0m")
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
