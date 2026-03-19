package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/ui/panes"
	"github.com/alexivison/scry/internal/ui/theme"
)

// viewIdle renders the idle screen shown when --watch is enabled and no
// divergence has been detected yet. Displays a centered bordered box with
// a pulsing watch indicator, structured info layout, and styled key badges.
func (m Model) viewIdle() string {
	contentH := m.height - 1 // reserve status bar

	// Pulse indicator alternates on spinner tick.
	indicator := "◉"
	if m.idlePulse {
		indicator = "○"
	}
	indicatorStyle := lipgloss.NewStyle().Foreground(theme.Clean)

	var lines []string
	lines = append(lines, indicatorStyle.Render(indicator)+" Watching for changes...")
	lines = append(lines, "")

	if m.State.Compare.WorkingTree {
		lines = append(lines, fmt.Sprintf("Base:     %s (working tree)", m.State.Compare.BaseRef))
	} else {
		lines = append(lines, fmt.Sprintf("Range:    %s", m.State.Compare.DiffRange))
	}
	lines = append(lines, fmt.Sprintf("Interval: every %s", m.State.WatchInterval))

	if !m.lastCheckAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Last:     %s", m.lastCheckAt.Format("15:04:05")))
	}

	lines = append(lines, "")

	statusStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	lines = append(lines, statusStyle.Render("No divergence detected."))
	lines = append(lines, "")

	// Determine box width from info lines (before badges).
	maxLineW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > maxLineW {
			maxLineW = w
		}
	}
	boxW := maxLineW + 6 // inner padding + borders
	if boxW > m.width-4 {
		boxW = m.width - 4
	}
	if boxW < 20 {
		boxW = 20
	}

	// Styled key badges (wrapped to fit box width).
	badges := wrapBadges([]string{keyBadge("q", "quit"), keyBadge("?", "help"), keyBadge("r", "refresh")}, boxW-2)
	for _, bl := range strings.Split(badges, "\n") {
		lines = append(lines, bl)
	}

	content := strings.Join(lines, "\n")
	boxH := len(lines) + 2 // top/bottom border
	box := panes.BorderedPane(content, "", "", boxW, boxH, true, false)

	return centerBox(box, contentH, m.width, boxW)
}

// centerBox centers a rendered box both vertically and horizontally within
// a content area of the given height and total width.
func centerBox(box string, contentH, totalW, boxW int) string {
	boxLines := strings.Split(box, "\n")
	startRow := (contentH - len(boxLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (totalW - boxW) / 2
	if startCol < 0 {
		startCol = 0
	}
	pad := strings.Repeat(" ", startCol)

	result := make([]string, contentH)
	for i := 0; i < contentH; i++ {
		boxIdx := i - startRow
		if boxIdx >= 0 && boxIdx < len(boxLines) {
			result[i] = pad + boxLines[boxIdx]
		}
	}
	return strings.Join(result, "\n")
}

// wrapBadges joins badge strings with "  " separators, wrapping to the next
// line when the accumulated width would exceed maxW.
func wrapBadges(badges []string, maxW int) string {
	if len(badges) == 0 {
		return ""
	}
	sep := "  "
	var lines []string
	line := badges[0]
	lineW := lipgloss.Width(line)
	for _, b := range badges[1:] {
		bw := lipgloss.Width(b)
		if lineW+lipgloss.Width(sep)+bw > maxW {
			lines = append(lines, line)
			line = b
			lineW = bw
		} else {
			line += sep + b
			lineW += lipgloss.Width(sep) + bw
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

// keyBadge renders a key name with a contrasting background followed by a label.
func keyBadge(key, label string) string {
	badgeStyle := lipgloss.NewStyle().
		Background(theme.StatusBg).
		Foreground(theme.BrightText).
		Padding(0, 1)
	labelStyle := lipgloss.NewStyle().Foreground(theme.Muted)
	return badgeStyle.Render(key) + " " + labelStyle.Render(label)
}

// updateIdle handles key events in idle mode. Only quit and help are active;
// all other keys are ignored to prevent accidental actions (e.g. commit
// generation) when no files are present.
func (m Model) updateIdle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
	}
	return m, nil
}
