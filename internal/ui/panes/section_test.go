package panes

import (
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/ui/theme"
)

func TestSectionDividerUsesPaneBorderChrome(t *testing.T) {
	t.Parallel()

	fg, ok := sectionDividerStyle.GetForeground().(lipgloss.Color)
	if !ok {
		t.Fatalf("section divider foreground should be a lipgloss.Color, got %T", sectionDividerStyle.GetForeground())
	}
	if fg != theme.PaneBorder {
		t.Fatalf("section divider foreground = %q, want %q", fg, theme.PaneBorder)
	}
}

func TestSectionInactiveTitleKeepsInactiveChrome(t *testing.T) {
	t.Parallel()

	fg, ok := sectionTitleInactiveStyle.GetForeground().(lipgloss.Color)
	if !ok {
		t.Fatalf("section inactive title foreground should be a lipgloss.Color, got %T", sectionTitleInactiveStyle.GetForeground())
	}
	if fg != theme.InactiveChrome {
		t.Fatalf("section inactive title foreground = %q, want %q", fg, theme.InactiveChrome)
	}
}
