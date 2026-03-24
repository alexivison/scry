package panes

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/theme"
)

var (
	cleanStyle        = lipgloss.NewStyle().Foreground(theme.Clean)
	dirtyStyle        = lipgloss.NewStyle().Foreground(theme.Dirty)
	dashSelectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	hashStyle         = lipgloss.NewStyle().Foreground(theme.Muted)
	staleStyle        = lipgloss.NewStyle().Foreground(theme.Error)
)

// RenderDashboard renders the worktree dashboard list constrained to the given dimensions.
func RenderDashboard(worktrees []model.WorktreeInfo, selectedIdx, scrollOffset, width, height int) string {
	if len(worktrees) == 0 {
		return "No worktrees found."
	}

	scrollOffset = EnsureVisible(selectedIdx, scrollOffset, height, len(worktrees))

	end := scrollOffset + height
	if end > len(worktrees) {
		end = len(worktrees)
	}

	// Hoist ANSI-safe truncation style — same width for all rows.
	truncStyle := lipgloss.NewStyle().MaxWidth(width)

	lines := make([]string, 0, end-scrollOffset)
	for i := scrollOffset; i < end; i++ {
		line := renderWorktreeEntry(worktrees[i], i, selectedIdx, truncStyle)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderWorktreeEntry(wt model.WorktreeInfo, idx, selectedIdx int, truncStyle lipgloss.Style) string {
	// Status indicator.
	var status string
	if wt.Bare {
		status = hashStyle.Render("B")
	} else if wt.Dirty {
		status = dirtyStyle.Render("●")
	} else {
		status = cleanStyle.Render("●")
	}

	// File count for dirty worktrees.
	countStr := ""
	if wt.ChangedFiles > 0 {
		label := "files"
		if wt.ChangedFiles == 1 {
			label = "file"
		}
		countStr = dirtyStyle.Render(fmt.Sprintf(" %d %s", wt.ChangedFiles, label))
	}

	prefix := "  "
	if idx == selectedIdx {
		prefix = "> "
	}

	basename := filepath.Base(wt.Path)
	commitInfo := fmt.Sprintf("%s %s", hashStyle.Render(wt.CommitHash), wt.Subject)

	// Staleness badge from git commit age (always rendered, "--" for bare/unknown).
	label, style := StalenessBadge(wt.HeadCommittedAt)
	styledStaleness := " " + style.Render(label)

	// Layout: [prefix][status][count] [branch]  [basename] [staleness]  [hash subject]
	branchWidth := 20
	basenameWidth := 20
	branch := wt.Branch
	if lipgloss.Width(branch) > branchWidth {
		branch = truncatePath(branch, branchWidth)
	}

	line := fmt.Sprintf("%s%s%s %-*s  %-*s%s  %s", prefix, status, countStr, branchWidth, branch, basenameWidth, basename, styledStaleness, commitInfo)

	if idx == selectedIdx {
		return dashSelectedStyle.Inherit(truncStyle).Render(line)
	}
	return truncStyle.Render(line)
}

// RelativeTime formats a timestamp as a relative duration string (e.g. "3s ago", "2m ago").
// Returns empty string for zero time.
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

// StalenessBadge returns both label and style for a commit-age badge.
// Computes time.Since once to avoid boundary divergence between label and color.
func StalenessBadge(t time.Time) (string, lipgloss.Style) {
	if t.IsZero() {
		return "--", hashStyle
	}
	d := time.Since(t)
	if d < 0 {
		return "0m", cleanStyle
	}

	var label string
	switch {
	case d < time.Hour:
		label = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		label = fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		label = fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 60*24*time.Hour:
		label = fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		label = fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}

	switch {
	case d < 3*24*time.Hour:
		return label, cleanStyle
	case d < 7*24*time.Hour:
		return label, dirtyStyle
	default:
		return label, staleStyle
	}
}
