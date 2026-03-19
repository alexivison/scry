package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
)

func intP(n int) *int { return &n }

func threeHunkPatch() model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{
			{
				Header:   "func init()",
				OldStart: 1, OldLen: 3, NewStart: 1, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: "package main"},
					{Kind: model.LineAdded, NewNo: intP(2), Text: `import "os"`},
					{Kind: model.LineContext, OldNo: intP(2), NewNo: intP(3), Text: ""},
				},
			},
			{
				Header:   "func main()",
				OldStart: 10, OldLen: 3, NewStart: 11, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(10), NewNo: intP(11), Text: "func main() {"},
					{Kind: model.LineDeleted, OldNo: intP(11), Text: "\told()"},
					{Kind: model.LineAdded, NewNo: intP(12), Text: "\tnew()"},
					{Kind: model.LineContext, OldNo: intP(12), NewNo: intP(13), Text: "}"},
				},
			},
			{
				Header:   "func helper()",
				OldStart: 20, OldLen: 2, NewStart: 21, NewLen: 3,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(20), NewNo: intP(21), Text: "func helper() {"},
					{Kind: model.LineAdded, NewNo: intP(22), Text: "\tlog.Println()"},
					{Kind: model.LineContext, OldNo: intP(21), NewNo: intP(23), Text: "}"},
				},
			},
		},
	}
}

func emptyPatch() model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "empty.go", Status: model.StatusModified},
	}
}

func noNewlinePatch() model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "file.txt", Status: model.StatusModified},
		Hunks: []model.Hunk{
			{
				Header:   "",
				OldStart: 1, OldLen: 2, NewStart: 1, NewLen: 2,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: "first line"},
					{Kind: model.LineDeleted, OldNo: intP(2), Text: "old last"},
					{Kind: model.LineNoNewline},
					{Kind: model.LineAdded, NewNo: intP(2), Text: "new last"},
					{Kind: model.LineNoNewline},
				},
			},
		},
	}
}

func TestNewPatchViewport(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		patch     model.FilePatch
		wantHunk  int
		wantStart int
	}{
		"multi-hunk starts at first hunk": {
			patch:     threeHunkPatch(),
			wantHunk:  0,
			wantStart: 0,
		},
		"empty patch": {
			patch:     emptyPatch(),
			wantHunk:  0,
			wantStart: 0,
		},
		"single hunk with no-newline": {
			patch:     noNewlinePatch(),
			wantHunk:  0,
			wantStart: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vp := NewPatchViewport(tc.patch)
			if vp.CurrentHunk != tc.wantHunk {
				t.Errorf("CurrentHunk = %d, want %d", vp.CurrentHunk, tc.wantHunk)
			}
			if vp.ScrollOffset != tc.wantStart {
				t.Errorf("ScrollOffset = %d, want %d", vp.ScrollOffset, tc.wantStart)
			}
		})
	}
}

func TestNextHunk(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		patch    model.FilePatch
		initial  int
		wantHunk int
		wantLine int // expected scroll offset (line index of hunk header)
	}{
		"first to second": {
			patch:    threeHunkPatch(),
			initial:  0,
			wantHunk: 1,
			wantLine: 4, // hunk0 header + 3 lines = index 4
		},
		"second to third": {
			patch:    threeHunkPatch(),
			initial:  1,
			wantHunk: 2,
			wantLine: 9, // hunk0(4) + hunk1 header + 4 lines = index 9
		},
		"last hunk no-op": {
			patch:    threeHunkPatch(),
			initial:  2,
			wantHunk: 2,
			wantLine: 9,
		},
		"empty patch no-op": {
			patch:    emptyPatch(),
			initial:  0,
			wantHunk: 0,
			wantLine: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vp := NewPatchViewport(tc.patch)
			vp.CurrentHunk = tc.initial
			vp.ScrollOffset = vp.hunkLineOffset(tc.initial)
			vp.NextHunk()
			if vp.CurrentHunk != tc.wantHunk {
				t.Errorf("CurrentHunk = %d, want %d", vp.CurrentHunk, tc.wantHunk)
			}
			if vp.ScrollOffset != tc.wantLine {
				t.Errorf("ScrollOffset = %d, want %d", vp.ScrollOffset, tc.wantLine)
			}
		})
	}
}

func TestPrevHunk(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		patch    model.FilePatch
		initial  int
		wantHunk int
		wantLine int
	}{
		"second to first": {
			patch:    threeHunkPatch(),
			initial:  1,
			wantHunk: 0,
			wantLine: 0,
		},
		"third to second": {
			patch:    threeHunkPatch(),
			initial:  2,
			wantHunk: 1,
			wantLine: 4,
		},
		"first hunk no-op": {
			patch:    threeHunkPatch(),
			initial:  0,
			wantHunk: 0,
			wantLine: 0,
		},
		"empty patch no-op": {
			patch:    emptyPatch(),
			initial:  0,
			wantHunk: 0,
			wantLine: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vp := NewPatchViewport(tc.patch)
			vp.CurrentHunk = tc.initial
			vp.ScrollOffset = vp.hunkLineOffset(tc.initial)
			vp.PrevHunk()
			if vp.CurrentHunk != tc.wantHunk {
				t.Errorf("CurrentHunk = %d, want %d", vp.CurrentHunk, tc.wantHunk)
			}
			if vp.ScrollOffset != tc.wantLine {
				t.Errorf("ScrollOffset = %d, want %d", vp.ScrollOffset, tc.wantLine)
			}
		})
	}
}

func TestRenderLines(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		patch       model.FilePatch
		height      int
		wantContain []string
	}{
		"renders hunk header": {
			patch:       threeHunkPatch(),
			height:      20,
			wantContain: []string{"@@ -1,3 +1,4 @@ func init()"},
		},
		"renders added line": {
			patch:       threeHunkPatch(),
			height:      20,
			wantContain: []string{`import "os"`},
		},
		"renders deleted line": {
			patch:       threeHunkPatch(),
			height:      20,
			wantContain: []string{"old()"},
		},
		"renders context line": {
			patch:       threeHunkPatch(),
			height:      20,
			wantContain: []string{"package main"},
		},
		"renders no-newline marker": {
			patch:       noNewlinePatch(),
			height:      20,
			wantContain: []string{"No newline at end of file"},
		},
		"empty patch shows message": {
			patch:       emptyPatch(),
			height:      20,
			wantContain: []string{"No changes"},
		},
		"respects viewport height": {
			patch:  threeHunkPatch(),
			height: 3,
			// Only first 3 lines visible
			wantContain: []string{"func init()"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vp := NewPatchViewport(tc.patch)
			vp.Width = 80
			vp.Height = tc.height
			output := vp.Render()
			for _, want := range tc.wantContain {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, output)
				}
			}
		})
	}
}

func TestRenderHunkHeaderTruncation(t *testing.T) {
	t.Parallel()

	patch := model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{
			{
				Header:   "func veryLongFunctionNameThatExceedsNarrowWidth(ctx context.Context, arg1, arg2 string)",
				OldStart: 1, OldLen: 3, NewStart: 1, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: "x"},
				},
			},
		},
	}

	vp := NewPatchViewport(patch)
	vp.Width = 40 // narrow split pane
	vp.Height = 5
	output := vp.Render()

	for _, line := range strings.Split(output, "\n") {
		w := lipgloss.Width(line)
		if w > 40 {
			t.Errorf("line exceeds width 40 (got %d): %q", w, line)
		}
	}
}

func TestHunkLineOffset(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())

	tests := map[string]struct {
		hunk       int
		wantOffset int
	}{
		"hunk 0": {hunk: 0, wantOffset: 0},
		"hunk 1": {hunk: 1, wantOffset: 4},  // 1 header + 3 lines
		"hunk 2": {hunk: 2, wantOffset: 9},   // 4 + 1 header + 4 lines
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := vp.hunkLineOffset(tc.hunk)
			if got != tc.wantOffset {
				t.Errorf("hunkLineOffset(%d) = %d, want %d", tc.hunk, got, tc.wantOffset)
			}
		})
	}
}

func TestScrollDownUp(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup      func(vp *PatchViewport)
		action     func(vp *PatchViewport)
		wantOffset int
	}{
		"scroll down from top": {
			setup:      func(_ *PatchViewport) {},
			action:     func(vp *PatchViewport) { vp.ScrollDown() },
			wantOffset: 1,
		},
		"scroll up from offset 1": {
			setup:      func(vp *PatchViewport) { vp.ScrollOffset = 1 },
			action:     func(vp *PatchViewport) { vp.ScrollUp() },
			wantOffset: 0,
		},
		"scroll up at top is no-op": {
			setup:      func(_ *PatchViewport) {},
			action:     func(vp *PatchViewport) { vp.ScrollUp() },
			wantOffset: 0,
		},
		"scroll down at bottom is no-op": {
			setup: func(vp *PatchViewport) {
				vp.ScrollOffset = vp.TotalLines() - 1
			},
			action: func(vp *PatchViewport) { vp.ScrollDown() },
			wantOffset: -1, // sentinel: use TotalLines()-1
		},
		"multiple scroll downs": {
			setup: func(_ *PatchViewport) {},
			action: func(vp *PatchViewport) {
				vp.ScrollDown()
				vp.ScrollDown()
				vp.ScrollDown()
			},
			wantOffset: 3,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vp := NewPatchViewport(threeHunkPatch())
			tc.setup(vp)
			tc.action(vp)
			want := tc.wantOffset
			if want == -1 {
				want = vp.TotalLines() - 1
			}
			if vp.ScrollOffset != want {
				t.Errorf("ScrollOffset = %d, want %d", vp.ScrollOffset, want)
			}
		})
	}
}

func TestScrollSyncsCurrentHunk(t *testing.T) {
	t.Parallel()

	// threeHunkPatch layout:
	// line 0: hunk0 header
	// line 1-3: hunk0 lines (3)
	// line 4: hunk1 header
	// line 5-8: hunk1 lines (4)
	// line 9: hunk2 header
	// line 10-12: hunk2 lines (3)

	t.Run("scroll into hunk1 then p goes to hunk0", func(t *testing.T) {
		t.Parallel()
		vp := NewPatchViewport(threeHunkPatch())
		// Scroll to line 5 (inside hunk1)
		for i := 0; i < 5; i++ {
			vp.ScrollDown()
		}
		if vp.CurrentHunk != 1 {
			t.Fatalf("after scroll to line 5: CurrentHunk = %d, want 1", vp.CurrentHunk)
		}
		// p should go to hunk0
		vp.PrevHunk()
		if vp.CurrentHunk != 0 {
			t.Errorf("after p: CurrentHunk = %d, want 0", vp.CurrentHunk)
		}
		if vp.ScrollOffset != 0 {
			t.Errorf("after p: ScrollOffset = %d, want 0", vp.ScrollOffset)
		}
	})

	t.Run("scroll into hunk2 then n is no-op", func(t *testing.T) {
		t.Parallel()
		vp := NewPatchViewport(threeHunkPatch())
		// Scroll to line 10 (inside hunk2, the last hunk)
		for i := 0; i < 10; i++ {
			vp.ScrollDown()
		}
		if vp.CurrentHunk != 2 {
			t.Fatalf("after scroll to line 10: CurrentHunk = %d, want 2", vp.CurrentHunk)
		}
		// n at last hunk is no-op
		vp.NextHunk()
		if vp.CurrentHunk != 2 {
			t.Errorf("n at last: CurrentHunk = %d, want 2", vp.CurrentHunk)
		}
	})

	t.Run("scroll into hunk1 then n goes to hunk2", func(t *testing.T) {
		t.Parallel()
		vp := NewPatchViewport(threeHunkPatch())
		// Scroll to line 6 (inside hunk1)
		for i := 0; i < 6; i++ {
			vp.ScrollDown()
		}
		if vp.CurrentHunk != 1 {
			t.Fatalf("after scroll to line 6: CurrentHunk = %d, want 1", vp.CurrentHunk)
		}
		vp.NextHunk()
		if vp.CurrentHunk != 2 {
			t.Errorf("after n: CurrentHunk = %d, want 2", vp.CurrentHunk)
		}
		if vp.ScrollOffset != 9 {
			t.Errorf("after n: ScrollOffset = %d, want 9", vp.ScrollOffset)
		}
	})

	t.Run("scroll up back into hunk0 syncs correctly", func(t *testing.T) {
		t.Parallel()
		vp := NewPatchViewport(threeHunkPatch())
		// Scroll into hunk1
		for i := 0; i < 5; i++ {
			vp.ScrollDown()
		}
		if vp.CurrentHunk != 1 {
			t.Fatalf("CurrentHunk = %d, want 1", vp.CurrentHunk)
		}
		// Scroll back up into hunk0
		for i := 0; i < 3; i++ {
			vp.ScrollUp()
		}
		if vp.CurrentHunk != 0 {
			t.Errorf("after scroll up: CurrentHunk = %d, want 0", vp.CurrentHunk)
		}
	})
}

// threeHunkPatch viewport line layout:
// 0: hunk0 header
// 1: "package main"   (DiffLine 0)
// 2: `import "os"`    (DiffLine 1)
// 3: ""               (DiffLine 2)
// 4: hunk1 header
// 5: "func main() {"  (DiffLine 3)
// 6: "\told()"        (DiffLine 4)
// 7: "\tnew()"        (DiffLine 5)
// 8: "}"              (DiffLine 6)
// 9: hunk2 header
// 10: "func helper()" (DiffLine 7)
// 11: "\tlog.Println()" (DiffLine 8)
// 12: "}"             (DiffLine 9)

func TestDiffLineToViewportLine(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())

	tests := map[string]struct {
		diffIdx int
		wantVP  int
	}{
		"first diff line":         {diffIdx: 0, wantVP: 1},
		"second diff line":        {diffIdx: 1, wantVP: 2},
		"last in hunk0":           {diffIdx: 2, wantVP: 3},
		"first in hunk1":          {diffIdx: 3, wantVP: 5},
		"last in hunk1":           {diffIdx: 6, wantVP: 8},
		"first in hunk2":          {diffIdx: 7, wantVP: 10},
		"last diff line":          {diffIdx: 9, wantVP: 12},
		"out of range returns 0":  {diffIdx: 99, wantVP: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := vp.DiffLineToViewportLine(tc.diffIdx)
			if got != tc.wantVP {
				t.Errorf("DiffLineToViewportLine(%d) = %d, want %d", tc.diffIdx, got, tc.wantVP)
			}
		})
	}
}

func TestRenderGutterSuppression(t *testing.T) {
	t.Parallel()

	patch := threeHunkPatch()
	// With gutter visible (default), output should contain line numbers.
	vpWith := NewPatchViewport(patch)
	vpWith.Width = 80
	vpWith.Height = 20
	vpWith.GutterVisible = true
	outWith := vpWith.Render()

	// Line numbers like "   1" should appear in the gutter.
	if !strings.Contains(outWith, "   1") {
		t.Errorf("gutter visible: expected line numbers in output, got:\n%s", outWith)
	}

	// With gutter hidden, line numbers should NOT appear.
	vpWithout := NewPatchViewport(patch)
	vpWithout.Width = 80
	vpWithout.Height = 20
	vpWithout.GutterVisible = false
	outWithout := vpWithout.Render()

	// Should NOT contain the "   1    1 " gutter pattern.
	if strings.Contains(outWithout, "   1") {
		t.Errorf("gutter hidden: line numbers should not appear, got:\n%s", outWithout)
	}

	// The actual diff content should still be present.
	if !strings.Contains(outWithout, "package main") {
		t.Errorf("gutter hidden: diff content missing, got:\n%s", outWithout)
	}
}

func TestViewportLineToDiffLine(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())

	tests := map[string]struct {
		vpLine   int
		wantDiff int
	}{
		"hunk header maps to next diff line": {vpLine: 0, wantDiff: 0},
		"first diff line":                    {vpLine: 1, wantDiff: 0},
		"second diff line":                   {vpLine: 2, wantDiff: 1},
		"hunk1 header maps to first hunk1 diff": {vpLine: 4, wantDiff: 3},
		"first diff in hunk1":                {vpLine: 5, wantDiff: 3},
		"last diff in hunk1":                 {vpLine: 8, wantDiff: 6},
		"hunk2 header":                       {vpLine: 9, wantDiff: 7},
		"last diff line":                     {vpLine: 12, wantDiff: 9},
		"negative clamps to 0":               {vpLine: -5, wantDiff: 0},
		"beyond end clamps to last":          {vpLine: 100, wantDiff: 9},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := vp.ViewportLineToDiffLine(tc.vpLine)
			if got != tc.wantDiff {
				t.Errorf("ViewportLineToDiffLine(%d) = %d, want %d", tc.vpLine, got, tc.wantDiff)
			}
		})
	}
}
