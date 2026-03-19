package panes

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/ui/theme"
)

// BorderedPane wraps content in a rounded border with an optional title and footer.
// outerWidth and outerHeight are the total dimensions including borders.
// When active is true the border uses theme.Accent; otherwise theme.Muted.
// When showFooter is false the footer text is suppressed (used for compact tiers).
func BorderedPane(content, title, footer string, outerWidth, outerHeight int, active, showFooter bool) string {
	if outerWidth < 4 || outerHeight < 3 {
		return content
	}

	borderColor := theme.Muted
	if active {
		borderColor = theme.Accent
	}
	colorStyle := lipgloss.NewStyle().Foreground(borderColor)

	innerWidth := outerWidth - 2

	// Build top border with title.
	top := buildBorderLine("╭", "╮", "─", title, innerWidth, colorStyle)

	// Build bottom border with optional footer.
	footerText := ""
	if showFooter {
		footerText = footer
	}
	bottom := buildBorderLine("╰", "╯", "─", footerText, innerWidth, colorStyle)

	// Split content into lines and pad/truncate to fill inner area.
	innerHeight := outerHeight - 2
	contentLines := strings.Split(content, "\n")
	rows := make([]string, innerHeight)
	side := colorStyle.Render("│")
	for i := 0; i < innerHeight; i++ {
		var line string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		rows[i] = side + padOrTruncate(line, innerWidth) + side
	}

	parts := make([]string, 0, outerHeight)
	parts = append(parts, top)
	parts = append(parts, rows...)
	parts = append(parts, bottom)
	return strings.Join(parts, "\n")
}

// buildBorderLine constructs a top or bottom border line with an embedded label.
// Example: ╭─ Files ──────────╮
// Labels longer than available space are truncated with an ellipsis.
func buildBorderLine(left, right, fill, label string, innerWidth int, style lipgloss.Style) string {
	if label == "" {
		return style.Render(left + strings.Repeat(fill, innerWidth) + right)
	}

	// Reserve space for "─ " prefix and " " suffix around the label.
	maxLabel := innerWidth - 3 // fill + space + label + space
	if maxLabel < 1 {
		return style.Render(left + strings.Repeat(fill, innerWidth) + right)
	}
	if lipgloss.Width(label) > maxLabel {
		label = truncateToWidth(label, maxLabel-1) + "…"
	}

	decorated := fill + " " + label + " "
	remaining := innerWidth - lipgloss.Width(decorated)
	if remaining < 0 {
		remaining = 0
	}
	return style.Render(left + decorated + strings.Repeat(fill, remaining) + right)
}

// padOrTruncate ensures a string fits exactly within the given visual width.
func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w > width {
		return truncateToWidth(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// ContentDimensions returns the inner width and height available for content
// inside a bordered pane with the given outer dimensions. Values are clamped to ≥0.
func ContentDimensions(outerWidth, outerHeight int) (int, int) {
	w := outerWidth - 2
	h := outerHeight - 2
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w, h
}
