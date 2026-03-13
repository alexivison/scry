package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/review"
	"github.com/alexivison/scry/internal/ui/panes"
	"github.com/alexivison/scry/internal/watch"
)

// WorktreeLoader loads the worktree list with dirty state and commit info.
type WorktreeLoader interface {
	LoadWorktrees(ctx context.Context) ([]model.WorktreeInfo, error)
}

// DrillDownResult holds the resolved data for a worktree drill-down.
type DrillDownResult struct {
	Compare     model.ResolvedCompare
	Files       []model.FileSummary
	PatchLoader PatchLoader
}

// DrillDownProvider creates the diff context for a specific worktree.
type DrillDownProvider interface {
	LoadDrillDown(ctx context.Context, worktreePath string) (DrillDownResult, error)
}

// WithWorktreeLoader sets the WorktreeLoader used for dashboard auto-refresh.
func WithWorktreeLoader(wl WorktreeLoader) ModelOption {
	return func(m *Model) { m.worktreeLoader = wl }
}

// WithDrillDownProvider sets the provider for loading worktree diffs on drill-down.
func WithDrillDownProvider(dp DrillDownProvider) ModelOption {
	return func(m *Model) { m.drillDownProvider = dp }
}

// WorktreeRefreshedMsg is sent when an async worktree list reload completes.
type WorktreeRefreshedMsg struct {
	Worktrees []model.WorktreeInfo
	Err       error
}

// DrillDownLoadedMsg is sent when a worktree drill-down finishes loading.
type DrillDownLoadedMsg struct {
	Result     DrillDownResult
	Err        error
	Generation int // matches DashboardState.DrillGeneration to detect stale results
}

// handleDashboardTick fires an async worktree list refresh on each watch tick.
func (m Model) handleDashboardTick() (tea.Model, tea.Cmd) {
	if !m.State.WorktreeMode || m.worktreeLoader == nil {
		return m, nil
	}
	// Skip refresh during drill-down — the worktree list isn't visible, and
	// firing LoadWorktrees would set the shared RefreshInFlight flag, which
	// can conflict with drill-down operations. Keep scheduling ticks so
	// refresh resumes automatically when the user returns to the dashboard.
	if m.State.DashboardState.DrillDown {
		if m.State.WatchEnabled {
			return m, watch.TickCmd(m.State.WatchInterval)
		}
		return m, nil
	}
	if m.State.RefreshInFlight {
		// Skip this tick; schedule next one only if watch is still enabled.
		if m.State.WatchEnabled {
			return m, watch.TickCmd(m.State.WatchInterval)
		}
		return m, nil
	}
	m.State.RefreshInFlight = true
	loader := m.worktreeLoader
	return m, func() tea.Msg {
		wts, err := loader.LoadWorktrees(context.Background())
		return WorktreeRefreshedMsg{Worktrees: wts, Err: err}
	}
}

// handleWorktreeRefreshed applies the refreshed worktree list to dashboard state.
func (m Model) handleWorktreeRefreshed(msg WorktreeRefreshedMsg) (tea.Model, tea.Cmd) {
	m.State.RefreshInFlight = false

	var nextTick tea.Cmd
	if m.State.WatchEnabled && m.State.WatchInterval > 0 {
		nextTick = watch.TickCmd(m.State.WatchInterval)
	}

	if msg.Err != nil {
		m.refreshErr = fmt.Sprintf("worktree refresh failed: %v", msg.Err)
		return m, nextTick
	}
	m.refreshErr = ""

	// Preserve selection by branch name.
	var prevBranch string
	ds := &m.State.DashboardState
	if ds.SelectedIdx >= 0 && ds.SelectedIdx < len(ds.Worktrees) {
		prevBranch = ds.Worktrees[ds.SelectedIdx].Branch
	}
	ds.Worktrees = msg.Worktrees

	// Reconcile selection.
	if prevBranch != "" {
		for i, wt := range ds.Worktrees {
			if wt.Branch == prevBranch {
				ds.SelectedIdx = i
				return m, nextTick
			}
		}
	}
	// Clamp selection to valid range.
	if len(ds.Worktrees) == 0 {
		ds.SelectedIdx = 0
		return m, nextTick
	}
	if ds.SelectedIdx >= len(ds.Worktrees) {
		ds.SelectedIdx = len(ds.Worktrees) - 1
	}
	if ds.SelectedIdx < 0 {
		ds.SelectedIdx = 0
	}
	return m, nextTick
}

func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ds := &m.State.DashboardState
	switch msg.String() {
	case "j", "down":
		if ds.SelectedIdx < len(ds.Worktrees)-1 {
			ds.SelectedIdx++
			m.syncDashboardScroll()
		}
	case "k", "up":
		if ds.SelectedIdx > 0 {
			ds.SelectedIdx--
			m.syncDashboardScroll()
		}
	case "l", "enter":
		if ds.SelectedIdx >= 0 && ds.SelectedIdx < len(ds.Worktrees) {
			return m.startDrillDown(ds.Worktrees[ds.SelectedIdx])
		}
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
	}
	return m, nil
}

// startDrillDown begins loading the diff context for a worktree.
// When called as a fresh drill-down (from dashboard), it resets focus to PaneFiles.
// When called as a refresh (already in drill-down), it preserves the current focus pane.
func (m Model) startDrillDown(wt model.WorktreeInfo) (tea.Model, tea.Cmd) {
	isRefresh := m.State.DashboardState.DrillDown
	m.State.DashboardState.DrillDown = true
	if !isRefresh {
		m.State.FocusPane = model.PaneFiles
	}

	if m.drillDownProvider == nil {
		return m, nil
	}

	// Bump generation to invalidate any in-flight drill-down load.
	m.State.DashboardState.DrillGeneration++
	gen := m.State.DashboardState.DrillGeneration

	path := wt.Path
	provider := m.drillDownProvider
	return m, func() tea.Msg {
		result, err := provider.LoadDrillDown(context.Background(), path)
		return DrillDownLoadedMsg{Result: result, Err: err, Generation: gen}
	}
}

// handleDrillDownLoaded applies the loaded worktree diff to the model.
func (m Model) handleDrillDownLoaded(msg DrillDownLoadedMsg) (tea.Model, tea.Cmd) {
	if !m.State.DashboardState.DrillDown {
		return m, nil // stale result; user already returned to dashboard
	}
	if msg.Generation != m.State.DashboardState.DrillGeneration {
		return m, nil // stale result from a superseded drill-down load
	}
	if msg.Err != nil {
		m.refreshErr = fmt.Sprintf("drill-down failed: %v", msg.Err)
		m.returnToDashboard()
		return m, nil
	}

	// Preserve selected file path for reconciliation.
	var prevPath string
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		prevPath = m.State.Files[m.State.SelectedFile].Path
	}

	// Bump cache generation to invalidate any in-flight patch loads from the old state.
	review.BumpGeneration(&m.State)

	m.State.Compare = msg.Result.Compare
	m.State.Files = msg.Result.Files
	m.State.Patches = make(map[string]model.PatchLoadState)
	m.patchLoader = msg.Result.PatchLoader

	// Reconcile selection: match by path, fallback to clamped index.
	review.ReconcileSelection(&m.State, prevPath)

	// If in patch view, reload the selected file's patch.
	return m.loadSelectedPatch()
}

// syncDashboardScroll adjusts the dashboard scroll offset so the selected worktree stays visible.
func (m *Model) syncDashboardScroll() {
	if m.height == 0 {
		return
	}
	contentHeight := m.height - 1 // reserve status bar
	ds := &m.State.DashboardState
	ds.ScrollOffset = panes.EnsureVisible(ds.SelectedIdx, ds.ScrollOffset, contentHeight, len(ds.Worktrees))
}

// returnToDashboard resets drill-down state and returns focus to the dashboard pane.
func (m *Model) returnToDashboard() {
	m.State.DashboardState.DrillDown = false
	m.State.FocusPane = model.PaneDashboard
	m.patchViewport = nil
	m.patchErr = ""
	m.patchFallback = ""
	m.searchIndex = nil
	m.State.SearchQuery = ""
	m.searchNotFound = ""
}

// updateDrillDown handles keys when in worktree drill-down (file/patch view for a single worktree).
func (m Model) updateDrillDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		m.returnToDashboard()
		return m, nil
	}
	return m.updateFiles(msg)
}

func (m Model) viewDashboard() string {
	contentHeight := m.height - 1 // reserve status bar
	ds := m.State.DashboardState
	return panes.RenderDashboard(ds.Worktrees, ds.SelectedIdx, ds.ScrollOffset, m.width, contentHeight)
}
