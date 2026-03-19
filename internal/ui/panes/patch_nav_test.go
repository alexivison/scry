package panes

import "testing"

func TestPageDown(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Height = 5

	vp.PageDown()
	if vp.ScrollOffset != 5 { // half page = 5
		t.Errorf("after PageDown: ScrollOffset = %d, want 5", vp.ScrollOffset)
	}
}

func TestPageUp(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Height = 5
	vp.ScrollOffset = 8

	vp.PageUp()
	if vp.ScrollOffset != 3 { // 8 - 5 = 3
		t.Errorf("after PageUp: ScrollOffset = %d, want 3", vp.ScrollOffset)
	}
}

func TestHalfPageDown(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Height = 6

	vp.HalfPageDown()
	if vp.ScrollOffset != 3 { // half of 6 = 3
		t.Errorf("after HalfPageDown: ScrollOffset = %d, want 3", vp.ScrollOffset)
	}
}

func TestHalfPageUp(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Height = 6
	vp.ScrollOffset = 8

	vp.HalfPageUp()
	if vp.ScrollOffset != 5 { // 8 - 3 = 5
		t.Errorf("after HalfPageUp: ScrollOffset = %d, want 5", vp.ScrollOffset)
	}
}

func TestScrollToTop(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.ScrollOffset = 10

	vp.ScrollToTop()
	if vp.ScrollOffset != 0 {
		t.Errorf("after ScrollToTop: ScrollOffset = %d, want 0", vp.ScrollOffset)
	}
	if vp.CurrentHunk != 0 {
		t.Errorf("after ScrollToTop: CurrentHunk = %d, want 0", vp.CurrentHunk)
	}
}

func TestScrollToBottom(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Height = 5

	vp.ScrollToBottom()
	want := vp.TotalLines() - vp.Height
	if want < 0 {
		want = 0
	}
	if vp.ScrollOffset != want {
		t.Errorf("after ScrollToBottom: ScrollOffset = %d, want %d", vp.ScrollOffset, want)
	}
}

func TestPageDown_ClampsToBottom(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Height = 5
	vp.ScrollOffset = 10 // near bottom

	vp.PageDown()
	max := vp.TotalLines() - 1
	if vp.ScrollOffset > max {
		t.Errorf("PageDown overshot: ScrollOffset = %d, max = %d", vp.ScrollOffset, max)
	}
}

func TestPageUp_ClampsToTop(t *testing.T) {
	t.Parallel()

	vp := NewPatchViewport(threeHunkPatch())
	vp.Height = 5
	vp.ScrollOffset = 2

	vp.PageUp()
	if vp.ScrollOffset < 0 {
		t.Errorf("PageUp undershot: ScrollOffset = %d", vp.ScrollOffset)
	}
	if vp.ScrollOffset != 0 {
		t.Errorf("after PageUp from 2: ScrollOffset = %d, want 0", vp.ScrollOffset)
	}
}
