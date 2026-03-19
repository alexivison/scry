package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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

	// The highlighted scroll indicator uses ┃ instead of │ on the right border.
	if !strings.Contains(output, "┃") {
		t.Errorf("scroll indicator ┃ should be visible in bordered pane, got:\n%s", output)
	}
}

func TestScrollIndicator_HiddenWhenNegative(t *testing.T) {
	t.Parallel()

	output := BorderedPaneWithScroll("line1\nline2", "Title", "", 30, 4, true, true, -1)

	// No scroll indicator when scrollLine is negative.
	if strings.Contains(output, "┃") {
		t.Errorf("scroll indicator ┃ should not appear when scrollLine=-1, got:\n%s", output)
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
