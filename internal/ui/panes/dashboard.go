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

// LinesPerEntry is the number of terminal lines each worktree entry occupies.
const LinesPerEntry = 2

var (
	cleanStyle    = lipgloss.NewStyle().Foreground(theme.Clean)
	dirtyStyle    = lipgloss.NewStyle().Foreground(theme.Dirty)
	hashStyle     = lipgloss.NewStyle().Foreground(theme.Muted)
	staleStyle    = lipgloss.NewStyle().Foreground(theme.Error)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
)

// RenderDashboard renders the worktree dashboard list constrained to the given dimensions.
// Each entry occupies two lines; height is in terminal lines.
func RenderDashboard(worktrees []model.WorktreeInfo, selectedIdx, scrollOffset, width, height int) string {
	if len(worktrees) == 0 {
		return "No worktrees found."
	}

	visibleEntries := height / LinesPerEntry
	if visibleEntries < 1 {
		visibleEntries = 1
	}

	scrollOffset = EnsureVisible(selectedIdx, scrollOffset, visibleEntries, len(worktrees))

	end := scrollOffset + visibleEntries
	if end > len(worktrees) {
		end = len(worktrees)
	}

	truncStyle := lipgloss.NewStyle().MaxWidth(width)

	lines := make([]string, 0, (end-scrollOffset)*LinesPerEntry)
	for i := scrollOffset; i < end; i++ {
		primary, secondary := renderWorktreeEntry(worktrees[i], i, selectedIdx, width, truncStyle)
		lines = append(lines, primary, secondary)
	}
	return strings.Join(lines, "\n")
}

// renderWorktreeEntry returns two lines for a worktree entry:
//
//	Line 1: [prefix][status] [branch]              [count]
//	Line 2: [indent][staleness] [hash] [subject]   [activity]
func renderWorktreeEntry(wt model.WorktreeInfo, idx, selectedIdx, width int, truncStyle lipgloss.Style) (string, string) {
	selected := idx == selectedIdx

	// Status indicator.
	var status string
	if wt.Bare {
		status = hashStyle.Render("B")
	} else if wt.Dirty {
		status = dirtyStyle.Render("●")
	} else {
		status = cleanStyle.Render("●")
	}

	prefix := "  "
	if selected {
		prefix = "> "
	}

	// --- Primary line: [prefix][status] [branch] ... [count] ---
	// Fall back to path basename for bare/detached worktrees with no branch.
	branch := wt.Branch
	if branch == "" {
		branch = filepath.Base(wt.Path)
	}
	branchStyle := hashStyle
	if selected {
		branchStyle = selectedStyle
	}

	countStr := ""
	countPlain := ""
	if wt.ChangedFiles > 0 {
		label := "files"
		if wt.ChangedFiles == 1 {
			label = "file"
		}
		countPlain = fmt.Sprintf("%d %s", wt.ChangedFiles, label)
		countStr = dirtyStyle.Render(countPlain)
	}

	// Budget: prefix(2) + status(1) + space(1) + branch + gap(2) + count
	countWidth := len(countPlain)
	branchBudget := width - 2 - 1 - 1 - 2 - countWidth
	if branchBudget < 5 {
		branchBudget = 5
	}
	if lipgloss.Width(branch) > branchBudget {
		branch = truncatePath(branch, branchBudget)
	}

	var primary string
	if countStr != "" {
		gap := branchBudget - lipgloss.Width(branch)
		if gap < 2 {
			gap = 2
		}
		primary = prefix + status + " " + branchStyle.Render(branch) + strings.Repeat(" ", gap) + countStr
	} else {
		primary = prefix + status + " " + branchStyle.Render(branch)
	}
	primary = truncStyle.Render(primary)

	// --- Secondary line: [indent][staleness] [hash] [subject] ... [activity] ---
	indent := "    " // align with branch text (prefix + status + space)

	// Staleness badge from git commit age.
	stalenessLabel, stalenessStyle := StalenessBadge(wt.HeadCommittedAt)
	staleness := stalenessStyle.Render(stalenessLabel)
	stalenessWidth := lipgloss.Width(stalenessLabel)

	activity := RelativeTime(wt.LastActivityAt)
	activityPlain := activity
	if activity != "" {
		activity = hashStyle.Render(activity)
	}

	commitHash := hashStyle.Render(wt.CommitHash)
	commitHashWidth := lipgloss.Width(wt.CommitHash)

	// Budget: indent(4) + staleness + space(1) + hash + space(1) + subject + gap(2) + activity
	activityWidth := len(activityPlain)
	subjectBudget := width - 4 - stalenessWidth - 1 - commitHashWidth - 1 - 2 - activityWidth
	if subjectBudget < 5 {
		subjectBudget = 5
	}

	subject := wt.Subject
	if lipgloss.Width(subject) > subjectBudget {
		subject = truncatePath(subject, subjectBudget)
	}

	var secondary string
	if activityPlain != "" {
		gap := subjectBudget - lipgloss.Width(subject)
		if gap < 2 {
			gap = 2
		}
		secondary = indent + staleness + " " + commitHash + " " + subject + strings.Repeat(" ", gap) + activity
	} else {
		secondary = indent + staleness + " " + commitHash + " " + subject
	}
	secondary = truncStyle.Render(secondary)

	return primary, secondary
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
