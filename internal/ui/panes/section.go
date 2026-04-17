package panes

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/ui/theme"
)

var (
	sectionDividerStyle       = lipgloss.NewStyle().Foreground(theme.ChromeFaint)
	sectionMetaStyle          = lipgloss.NewStyle().Foreground(theme.Muted)
	sectionTitleActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	sectionTitleInactiveStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightText)
)

// UnboxedSectionDimensions returns the width and content height for a main
// browsing section with an inline header and faint separator.
func UnboxedSectionDimensions(outerWidth, outerHeight int) (int, int) {
	if outerWidth < 0 {
		outerWidth = 0
	}
	innerHeight := outerHeight - 2
	if innerHeight < 0 {
		innerHeight = 0
	}
	return outerWidth, innerHeight
}

// UnboxedSection renders a main browsing section without an outer box. It uses
// an inline header and a faint separator line, preserving the overall area.
func UnboxedSection(content, title, meta string, outerWidth, outerHeight int, active bool) string {
	if outerWidth < 1 || outerHeight < 1 {
		return ""
	}

	rows := make([]string, 0, outerHeight)
	rows = append(rows, renderSectionHeader(title, meta, outerWidth, active))

	if outerHeight == 1 {
		return rows[0]
	}

	rows = append(rows, sectionDividerStyle.Render(strings.Repeat("─", outerWidth)))
	contentLines := strings.Split(content, "\n")
	contentHeight := outerHeight - 2
	for i := 0; i < contentHeight; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}
		rows = append(rows, padOrTruncate(line, outerWidth))
	}
	return strings.Join(rows, "\n")
}

// JoinColumns joins two unboxed sections with a single faint divider.
func JoinColumns(left, right string, leftWidth, rightWidth int) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	height := len(leftLines)
	if len(rightLines) > height {
		height = len(rightLines)
	}

	divider := sectionDividerStyle.Render("│")
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		rows[i] = padOrTruncate(l, leftWidth) + divider + padOrTruncate(r, rightWidth)
	}
	return strings.Join(rows, "\n")
}

func renderSectionHeader(title, meta string, width int, active bool) string {
	if width < 1 {
		return ""
	}

	titleStyle := sectionTitleInactiveStyle
	if active {
		titleStyle = sectionTitleActiveStyle
	}

	title = fitSectionHeaderText(title, width)
	if meta == "" {
		return padOrTruncate(titleStyle.Render(title), width)
	}

	minTitleWidth := width / 3
	if minTitleWidth < 1 {
		minTitleWidth = 1
	}
	maxMetaWidth := width - minTitleWidth - 1
	if maxMetaWidth < 0 {
		maxMetaWidth = 0
	}
	meta = fitSectionHeaderText(meta, maxMetaWidth)
	titleWidth := width - lipgloss.Width(meta)
	if meta != "" {
		titleWidth--
	}
	if titleWidth < 1 {
		titleWidth = 1
		meta = ""
	}
	title = fitSectionHeaderText(title, titleWidth)

	header := titleStyle.Render(title)
	if meta == "" {
		return padOrTruncate(header, width)
	}

	gap := width - lipgloss.Width(title) - lipgloss.Width(meta)
	if gap < 1 {
		gap = 1
	}
	return header + strings.Repeat(" ", gap) + sectionMetaStyle.Render(meta)
}

func fitSectionHeaderText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	if maxWidth == 1 {
		return truncateToWidth(text, 1)
	}
	return truncateToWidth(text, maxWidth-1) + "…"
}
