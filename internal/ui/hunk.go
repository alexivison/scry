package ui

import (
	"context"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
)

type hunkClosedMsg struct {
	err error
}

var buildHunkCommand = func(worktreeRoot, diffRange string) *exec.Cmd {
	cmd := exec.Command("hunk", "diff", diffRange)
	cmd.Dir = worktreeRoot
	return cmd
}

func (m Model) openInHunk(cmp model.ResolvedCompare) (tea.Model, tea.Cmd) {
	if cmp.Repo.WorktreeRoot == "" {
		m.refreshErr = "Hunk unavailable: no worktree selected"
		return m, nil
	}
	if cmp.DiffRange == "" {
		m.refreshErr = "Hunk unavailable: no comparison resolved"
		return m, nil
	}

	cmd := buildHunkCommand(cmp.Repo.WorktreeRoot, cmp.DiffRange)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return hunkClosedMsg{err: err}
	})
}

func (m Model) openSelectedWorktreeInHunk() (tea.Model, tea.Cmd) {
	ds := m.State.DashboardState
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) || ds.Worktrees[ds.SelectedIdx].Bare {
		m.refreshErr = "Hunk unavailable: no worktree selected"
		return m, nil
	}

	snap := WorktreeSnapshotKey(ds.Worktrees[ds.SelectedIdx], m.State.CompareBasis)
	if entry, ok := ds.PreviewCache[snap]; ok && ds.PreviewSnap == snap {
		return m.openInHunk(entry.Compare)
	}

	if m.worktreeCompareLoader == nil {
		m.refreshErr = "Hunk unavailable: no comparison loader"
		return m, nil
	}

	path := ds.Worktrees[ds.SelectedIdx].Path
	basis := m.State.CompareBasis
	loader := m.worktreeCompareLoader
	return m, func() tea.Msg {
		cmp, err := loader.LoadCompare(context.Background(), path, basis)
		return WorktreeCompareLoadedMsg{Compare: cmp, Snap: snap, Err: err}
	}
}

func (m Model) handleWorktreeCompareLoaded(msg WorktreeCompareLoadedMsg) (tea.Model, tea.Cmd) {
	ds := m.State.DashboardState
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) {
		return m, nil
	}
	if WorktreeSnapshotKey(ds.Worktrees[ds.SelectedIdx], m.State.CompareBasis) != msg.Snap {
		return m, nil
	}
	if msg.Err != nil {
		m.refreshErr = "Hunk unavailable: " + msg.Err.Error()
		return m, nil
	}
	return m.openInHunk(msg.Compare)
}
