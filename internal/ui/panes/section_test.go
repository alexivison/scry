package panes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

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

func TestSectionHeaderPreservesStyledRightMeta(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(oldProfile) })

	meta := statusAddedStyle.Render("+123") + " " + statusDeletedStyle.Render("-45")
	header := renderSectionHeader("Files 24/100 (Non-Tests)", meta, 34, true)
	plain := ansi.Strip(header)

	if !strings.HasSuffix(plain, "+123 -45") {
		t.Fatalf("right meta should keep aggregate counts untruncated, got %q", plain)
	}
	if !strings.Contains(header, meta) {
		t.Fatalf("right meta should preserve existing count colors, got %q; want segment %q", header, meta)
	}
	if strings.Contains(header, sectionMetaStyle.Render(meta)) {
		t.Fatalf("right meta should not be recolored by generic section meta style, got %q", header)
	}
	if width := lipgloss.Width(header); width != 34 {
		t.Fatalf("header width = %d, want 34: %q", width, plain)
	}
}
