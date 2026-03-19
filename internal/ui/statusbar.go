package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/terminal"
	"github.com/alexivison/scry/internal/ui/theme"
)

func (m Model) renderBar(msg string, style lipgloss.Style) string {
	bar := " " + msg
	gap := m.width - lipgloss.Width(bar)
	if gap > 0 {
		bar += strings.Repeat(" ", gap)
	}
	return style.Width(m.width).Render(bar)
}

func (m Model) renderErrorBar(msg string) string {
	return m.renderBar(msg, searchNotFoundStyle)
}

func (m Model) viewStatusBar() string {
	// Full-width error/status messages take priority over segmented bar.
	if m.State.FocusPane == model.PaneDashboard {
		ds := m.State.DashboardState
		if ds.DeleteIsMain {
			return m.renderErrorBar("Cannot delete main worktree")
		}
		if ds.DeleteErr != "" {
			return m.renderErrorBar(ds.DeleteErr)
		}
	}
	if m.refreshErr != "" {
		return m.renderErrorBar(m.refreshErr)
	}
	if m.searchNotFound != "" {
		return m.renderErrorBar(m.searchNotFound)
	}
	if m.exportMsg != "" {
		return m.renderBar(m.exportMsg, statusBarStyle)
	}

	sep := segmentSepStyle.Render(" │ ")
	minimal := m.widthTierNow() <= terminal.WidthMinimal

	// Build segments left-to-right.
	var segments []string

	// Segment 1: Compare context / breadcrumb.
	if m.State.WorktreeMode && m.State.DashboardState.DrillDown {
		segments = append(segments, m.drillDownBreadcrumb())
	} else if m.State.FocusPane == model.PaneDashboard {
		segments = append(segments, "Worktree Dashboard")
	} else {
		segments = append(segments, m.compareSegment(minimal))
	}

	// Segment 2: Mode badges (non-dashboard).
	if m.State.FocusPane != model.PaneDashboard {
		segments = append(segments, m.modeBadges())
	}

	// Segment 3: Watch state (non-dashboard).
	if m.State.WatchEnabled && m.State.FocusPane != model.PaneDashboard {
		segments = append(segments, m.watchSegment())
	}

	left := " " + strings.Join(segments, sep) + " "

	// Right-aligned file/worktree count.
	var right string
	if m.State.FocusPane == model.PaneDashboard {
		right = fmt.Sprintf(" %d worktrees ", len(m.State.DashboardState.Worktrees))
	} else if minimal {
		right = fmt.Sprintf(" %d ", len(m.State.Files))
	} else {
		right = fmt.Sprintf(" %d files ", len(m.State.Files))
	}

	// Truncate left if it overflows.
	maxLeft := m.width - lipgloss.Width(right)
	if maxLeft > 0 && lipgloss.Width(left) > maxLeft {
		left = truncateToWidth(left, maxLeft)
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	bar := left + strings.Repeat(" ", gap) + right
	return statusBarStyle.Width(m.width).Render(bar)
}

// compareSegment returns the compare context string for the status bar.
func (m Model) compareSegment(minimal bool) string {
	if minimal {
		if m.State.Compare.WorkingTree {
			return m.State.Compare.BaseRef
		}
		return m.State.Compare.DiffRange
	}
	if m.State.Compare.WorkingTree {
		return m.State.Compare.BaseRef + " (working tree)"
	}
	return m.State.Compare.DiffRange
}

// modeBadges returns styled mode indicator badges (W for whitespace, C for commit).
// Active badges are bold/bright; inactive badges are dim.
func (m Model) modeBadges() string {
	wStyle := badgeDimStyle
	if m.State.IgnoreWhitespace {
		wStyle = badgeActiveStyle
	}
	cStyle := badgeDimStyle
	if m.State.CommitEnabled {
		cStyle = badgeActiveStyle
	}
	return wStyle.Render("W") + " " + cStyle.Render("C")
}

// watchSegment returns the watch state indicator: dot + interval + last check time.
// Always builds the full segment; left-side truncation handles narrow widths.
func (m Model) watchSegment() string {
	dot := watchDotStyle.Render("●")
	if m.watchErr {
		dot = watchErrorDotStyle.Render("●")
	}
	if m.State.RefreshInFlight {
		dot = m.spinner.View()
	}
	s := dot + " " + m.State.WatchInterval.String()
	if !m.lastCheckAt.IsZero() {
		s += " " + m.lastCheckAt.Format("15:04:05")
	}
	return s
}

// drillDownBreadcrumb returns "Dashboard > branch" (and "> file" if a file is selected).
func (m Model) drillDownBreadcrumb() string {
	ds := m.State.DashboardState
	branch := ""
	if ds.SelectedIdx >= 0 && ds.SelectedIdx < len(ds.Worktrees) {
		branch = ds.Worktrees[ds.SelectedIdx].Branch
	}
	crumb := "Dashboard > " + branch
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		crumb += " > " + m.State.Files[m.State.SelectedFile].Path
	}
	return crumb
}

func (m Model) viewSearchInput() string {
	prompt := "/" + m.searchInput
	gap := m.width - lipgloss.Width(prompt)
	if gap > 0 {
		prompt += strings.Repeat(" ", gap)
	}
	return statusBarStyle.Width(m.width).Render(prompt)
}

// Status bar styles.
var (
	segmentSepStyle = lipgloss.NewStyle().
			Foreground(theme.Muted)
	badgeActiveStyle = lipgloss.NewStyle().
				Foreground(theme.BrightText).
				Bold(true)
	badgeDimStyle = lipgloss.NewStyle().
			Foreground(theme.Muted)
	watchDotStyle = lipgloss.NewStyle().
			Foreground(theme.Clean)
	watchRefreshDotStyle = lipgloss.NewStyle().
				Foreground(theme.Dirty)
	watchErrorDotStyle = lipgloss.NewStyle().
				Foreground(theme.Error)
)
