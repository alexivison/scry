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

	// Activity timestamp.
	activity := RelativeTime(wt.LastActivityAt)
	if activity != "" {
		activity = " " + hashStyle.Render(activity)
	}

	// Layout: [prefix][status][count] [branch]  [basename] [activity]  [hash subject]
	branchWidth := 20
	basenameWidth := 20
	branch := wt.Branch
	if lipgloss.Width(branch) > branchWidth {
		branch = truncatePath(branch, branchWidth)
	}

	line := fmt.Sprintf("%s%s%s %-*s  %-*s%s  %s", prefix, status, countStr, branchWidth, branch, basenameWidth, basename, activity, commitInfo)

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
