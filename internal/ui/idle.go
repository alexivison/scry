package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// viewIdle renders the idle screen shown when --watch is enabled and no
// divergence has been detected yet.
func (m Model) viewIdle() string {
	var lines []string

	lines = append(lines, "  Watching for changes...")
	lines = append(lines, "")

	if m.State.Compare.WorkingTree {
		lines = append(lines, fmt.Sprintf("  Base:     %s (working tree)", m.State.Compare.BaseRef))
	} else {
		lines = append(lines, fmt.Sprintf("  Range:    %s", m.State.Compare.DiffRange))
	}
	lines = append(lines, fmt.Sprintf("  Interval: every %s", m.State.WatchInterval))

	if !m.lastCheckAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  Last check: %s", m.lastCheckAt.Format("15:04:05")))
	}

	lines = append(lines, "")
	lines = append(lines, "  No divergence detected.")
	lines = append(lines, "")
	lines = append(lines, "  q quit  ? help  r refresh")

	return strings.Join(lines, "\n")
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
