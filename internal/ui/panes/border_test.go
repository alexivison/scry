package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBorderedPane_RoundedCorners(t *testing.T) {
	t.Parallel()

	out := BorderedPane("hello", "Title", "", 20, 5, true, true)
	if !strings.Contains(out, "╭") {
		t.Error("expected top-left rounded corner ╭")
	}
	if !strings.Contains(out, "╮") {
		t.Error("expected top-right rounded corner ╮")
	}
	if !strings.Contains(out, "╰") {
		t.Error("expected bottom-left rounded corner ╰")
	}
	if !strings.Contains(out, "╯") {
		t.Error("expected bottom-right rounded corner ╯")
	}
}

func TestBorderedPane_TitleInTopBorder(t *testing.T) {
	t.Parallel()

	out := BorderedPane("content", "Files", "", 30, 5, true, true)
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if !strings.Contains(lines[0], "Files") {
		t.Errorf("top border should contain title 'Files', got: %q", lines[0])
	}
}

func TestBorderedPane_FooterInBottomBorder(t *testing.T) {
	t.Parallel()

	out := BorderedPane("content", "Title", "3 files", 30, 5, true, true)
	lines := strings.Split(out, "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "3 files") {
		t.Errorf("bottom border should contain footer '3 files', got: %q", last)
	}
}

func TestBorderedPane_FooterHiddenWhenShowFooterFalse(t *testing.T) {
	t.Parallel()

	out := BorderedPane("content", "Title", "footer text", 30, 5, true, false)
	lines := strings.Split(out, "\n")
	last := lines[len(lines)-1]
	if strings.Contains(last, "footer text") {
		t.Errorf("footer should be hidden when showFooter=false, got: %q", last)
	}
}

func TestBorderedPane_ContentRendered(t *testing.T) {
	t.Parallel()

	out := BorderedPane("hello world", "T", "", 30, 5, true, true)
	if !strings.Contains(out, "hello world") {
		t.Error("bordered pane should contain the content")
	}
}

func TestBorderedPane_ActiveVsInactive(t *testing.T) {
	t.Parallel()

	active := BorderedPane("x", "T", "", 20, 5, true, true)
	inactive := BorderedPane("x", "T", "", 20, 5, false, true)
	// Both should render borders with same structure.
	if !strings.Contains(active, "╭") {
		t.Error("active pane missing border")
	}
	if !strings.Contains(inactive, "╭") {
		t.Error("inactive pane missing border")
	}
	// Both should contain content.
	if !strings.Contains(active, "x") {
		t.Error("active pane missing content")
	}
	if !strings.Contains(inactive, "x") {
		t.Error("inactive pane missing content")
	}
}

func TestBorderedPane_CorrectLineCount(t *testing.T) {
	t.Parallel()

	out := BorderedPane("line1\nline2", "T", "F", 30, 6, true, true)
	lines := strings.Split(out, "\n")
	if len(lines) != 6 {
		t.Errorf("expected 6 lines (1 top + 4 content + 1 bottom), got %d", len(lines))
	}
}

func TestBorderedPane_LongTitleTruncated(t *testing.T) {
	t.Parallel()

	longTitle := "very/long/path/that/exceeds/the/available/width/file.go"
	out := BorderedPane("x", longTitle, "", 20, 5, true, true)
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatal("expected output")
	}
	// Top border line must not exceed outer width.
	topWidth := lipgloss.Width(lines[0])
	if topWidth > 20 {
		t.Errorf("top border width = %d, want ≤ 20", topWidth)
	}
	// Should contain ellipsis for truncated title.
	if !strings.Contains(lines[0], "…") {
		t.Errorf("long title should be truncated with ellipsis, got: %q", lines[0])
	}
}

func TestBorderedPane_SideBorders(t *testing.T) {
	t.Parallel()

	out := BorderedPane("data", "T", "", 20, 5, true, true)
	lines := strings.Split(out, "\n")
	// Content lines (not first/last) should have │ side borders.
	for i := 1; i < len(lines)-1; i++ {
		if !strings.Contains(lines[i], "│") {
			t.Errorf("line %d should have side border │, got: %q", i, lines[i])
		}
	}
}
