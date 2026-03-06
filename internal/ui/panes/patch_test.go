package panes

import (
	"strings"
	"testing"

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
