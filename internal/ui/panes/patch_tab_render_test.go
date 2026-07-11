package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/alexivison/scry/internal/model"
)

// TestTabExpandedLongLineRendersFullWidthWithoutClipping is a render
// regression test for the tab-clipping bug: internal/diff now expands tabs
// to spaces before DiffLine.Text ever reaches the renderer, so the fixture
// here uses POST-expansion (space-indented) text, exactly as PatchViewport
// would receive it in production. It asserts the renderer's layout math
// (wrap segmentation, right-edge truncation, padding, side-by-side column
// widths) produces rows that exactly fill the viewport width and never
// silently drops the tail of a wrapped long line.
func TestTabExpandedLongLineRendersFullWidthWithoutClipping(t *testing.T) {
	t.Parallel()

	// Same shape as the reported bug: an 8-space (post-tab-expansion) indent
	// followed by a long trailing line comment.
	longText := strings.Repeat(" ", 8) +
		`result := compute(1, 2, 3) // now takes three arguments instead of two, which makes this line quite long`
	patch := model.FilePatch{
		Summary: model.FileSummary{Path: "app.go", Status: model.StatusModified},
		Hunks: []model.Hunk{{
			Header:   "func main()",
			OldStart: 1, OldLen: 0, NewStart: 1, NewLen: 1,
			Lines: []model.DiffLine{
				{Kind: model.LineAdded, NewNo: intP(1), Text: longText},
			},
		}},
	}

	modes := []struct {
		name string
		mode model.PatchDiffMode
	}{
		{"unified", model.PatchDiffModeUnified},
		{"side-by-side", model.PatchDiffModeSideBySide},
	}
	widths := []int{60, 100}

	for _, m := range modes {
		for _, width := range widths {
			t.Run(m.name, func(t *testing.T) {
				vp := NewPatchViewport(patch)
				vp.DiffMode = m.mode
				vp.Width = width
				vp.Height = 20
				vp.GutterVisible = true
				vp.LineMode = model.LineModeWrap

				got := vp.Render()
				rows := strings.Split(got, "\n")
				if len(rows) == 0 {
					t.Fatalf("%s width=%d: empty render", m.name, width)
				}

				for i, row := range rows {
					if w := ansi.StringWidth(row); w != width {
						t.Errorf("%s width=%d: row %d width = %d, want %d: %q", m.name, width, i, w, width, row)
					}
				}

				// The final wrap segment of the long line always contains its
				// tail (wrapBodySegments extends the last segment to len(body)),
				// so the last rendered row must contain the line's last word.
				last := ansi.Strip(rows[len(rows)-1])
				if !strings.Contains(last, "long") {
					t.Errorf("%s width=%d: wrapped render lost trailing word, last row = %q", m.name, width, last)
				}
			})
		}
	}
}
