package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/review"
	"github.com/alexivison/scry/internal/ui/panes"
	"github.com/alexivison/scry/internal/watch"
)

// maxPreviewCacheSize caps the number of entries in the preview cache.
const maxPreviewCacheSize = 50

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

// PreviewLoader loads the top changed files for a worktree preview.
type PreviewLoader interface {
	LoadPreview(ctx context.Context, worktreePath string) ([]model.FileSummary, error)
}

// WithPreviewLoader sets the PreviewLoader for dashboard preview pane.
func WithPreviewLoader(pl PreviewLoader) ModelOption {
	return func(m *Model) { m.previewLoader = pl }
}

// PreviewLoadedMsg is sent when an async preview load completes.
type PreviewLoadedMsg struct {
	Path  string
	Snap  string
	Files []model.FileSummary
	Err   error
}

// WorktreeRemover removes a worktree.
type WorktreeRemover interface {
	Remove(ctx context.Context, path string, force bool) error
}

// WithWorktreeRemover sets the WorktreeRemover for dashboard deletion.
func WithWorktreeRemover(wr WorktreeRemover) ModelOption {
	return func(m *Model) { m.worktreeRemover = wr }
}

// WorktreeRemovedMsg is sent when an async worktree removal completes.
type WorktreeRemovedMsg struct {
	Path string
	Err  error
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
	Worktrees  []model.WorktreeInfo
	Err        error
	Generation int // matches DashboardState.RefreshGeneration to detect stale results
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
	gen := m.State.DashboardState.RefreshGeneration
	return m, func() tea.Msg {
		wts, err := loader.LoadWorktrees(context.Background())
		return WorktreeRefreshedMsg{Worktrees: wts, Err: err, Generation: gen}
	}
}

// handleWorktreeRefreshed applies the refreshed worktree list to dashboard state.
func (m Model) handleWorktreeRefreshed(msg WorktreeRefreshedMsg) (tea.Model, tea.Cmd) {
	m.State.RefreshInFlight = false

	var nextTick tea.Cmd
	if m.State.WatchEnabled && m.State.WatchInterval > 0 {
		nextTick = watch.TickCmd(m.State.WatchInterval)
	}

	// Discard stale refresh results from before a deletion.
	if msg.Generation != m.State.DashboardState.RefreshGeneration {
		return m, nextTick
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

	// Reconcile LastActivityAt: compare old and new snapshots.
	reconcileActivity(ds.Worktrees, msg.Worktrees)
	ds.Worktrees = msg.Worktrees

	// Reconcile selection.
	found := false
	if prevBranch != "" {
		for i, wt := range ds.Worktrees {
			if wt.Branch == prevBranch {
				ds.SelectedIdx = i
				found = true
				break
			}
		}
	}
	if !found {
		// Clamp selection to valid range.
		if len(ds.Worktrees) == 0 {
			ds.SelectedIdx = 0
		} else {
			if ds.SelectedIdx >= len(ds.Worktrees) {
				ds.SelectedIdx = len(ds.Worktrees) - 1
			}
			if ds.SelectedIdx < 0 {
				ds.SelectedIdx = 0
			}
		}
	}

	// Trigger preview load for the (possibly new) selection.
	if previewCmd := m.maybeLoadPreview(); previewCmd != nil {
		return m, tea.Batch(nextTick, previewCmd)
	}
	return m, nextTick
}

func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ds := &m.State.DashboardState

	// Handle deletion confirmation prompts.
	if ds.ConfirmDelete {
		return m.updateDeleteConfirm(msg)
	}

	// Clear transient status messages on any key.
	ds.DeleteErr = ""
	ds.DeleteIsMain = false

	switch msg.String() {
	case "j", "down":
		if ds.SelectedIdx < len(ds.Worktrees)-1 {
			ds.SelectedIdx++
			m.syncDashboardScroll()
			if cmd := m.maybeLoadPreview(); cmd != nil {
				return m, cmd
			}
		}
	case "k", "up":
		if ds.SelectedIdx > 0 {
			ds.SelectedIdx--
			m.syncDashboardScroll()
			if cmd := m.maybeLoadPreview(); cmd != nil {
				return m, cmd
			}
		}
	case "l", "enter":
		if ds.SelectedIdx >= 0 && ds.SelectedIdx < len(ds.Worktrees) {
			return m.startDrillDown(ds.Worktrees[ds.SelectedIdx])
		}
	case "d":
		return m.startDeleteConfirm()
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
	}
	return m, nil
}

// startDeleteConfirm initiates the worktree deletion confirmation flow.
func (m Model) startDeleteConfirm() (tea.Model, tea.Cmd) {
	ds := &m.State.DashboardState
	if ds.DeleteInFlight {
		return m, nil
	}
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) {
		return m, nil
	}
	wt := ds.Worktrees[ds.SelectedIdx]

	// Bare worktrees and the main worktree (index 0) cannot be deleted.
	if wt.Bare || ds.SelectedIdx == 0 {
		ds.DeleteIsMain = true
		return m, nil
	}

	ds.ConfirmDelete = true
	ds.DeletePath = wt.Path
	ds.DeleteDirty = wt.Dirty
	ds.DeleteErr = ""
	return m, nil
}

// updateDeleteConfirm handles key events during the deletion confirmation prompt.
func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ds := &m.State.DashboardState
	switch msg.String() {
	case "y", "Y":
		// Confirm deletion.
		ds.ConfirmDelete = false
		return m.executeWorktreeRemove(ds.DeletePath, ds.DeleteDirty)
	case "n", "N", "esc":
		// Cancel deletion.
		ds.ConfirmDelete = false
		ds.DeletePath = ""
		ds.DeleteDirty = false
	}
	return m, nil
}

// executeWorktreeRemove fires an async worktree removal command.
func (m Model) executeWorktreeRemove(path string, force bool) (tea.Model, tea.Cmd) {
	if m.worktreeRemover == nil {
		return m, nil
	}
	m.State.DashboardState.DeleteInFlight = true
	remover := m.worktreeRemover
	return m, func() tea.Msg {
		err := remover.Remove(context.Background(), path, force)
		return WorktreeRemovedMsg{Path: path, Err: err}
	}
}

// handleWorktreeRemoved processes the result of a worktree removal.
func (m Model) handleWorktreeRemoved(msg WorktreeRemovedMsg) (tea.Model, tea.Cmd) {
	ds := &m.State.DashboardState
	ds.DeleteInFlight = false
	ds.DeletePath = ""
	ds.DeleteDirty = false

	if msg.Err != nil {
		ds.DeleteErr = fmt.Sprintf("delete failed: %v", msg.Err)
		return m, nil
	}

	// Bump refresh generation so any in-flight refresh from before the delete
	// is discarded when it completes, preventing re-addition of the deleted entry.
	ds.RefreshGeneration++

	// Optimistically remove the deleted worktree from the list so it disappears
	// immediately.
	for i, wt := range ds.Worktrees {
		if wt.Path == msg.Path {
			ds.Worktrees = append(ds.Worktrees[:i], ds.Worktrees[i+1:]...)
			if ds.SelectedIdx >= len(ds.Worktrees) && ds.SelectedIdx > 0 {
				ds.SelectedIdx--
			}
			break
		}
	}
	// Clear stale preview from the deleted worktree.
	ds.PreviewFiles = nil

	// Schedule a refresh to get authoritative state and reload preview.
	return m.handleDashboardTick()
}

// startDrillDown begins loading the diff context for a worktree.
// When called as a fresh drill-down (from dashboard), it resets focus to PaneFiles
// and clears stale data so the previous worktree's files don't flash briefly.
// When called as a refresh (already in drill-down), it preserves the current focus pane.
func (m Model) startDrillDown(wt model.WorktreeInfo) (tea.Model, tea.Cmd) {
	isRefresh := m.State.DashboardState.DrillDown
	m.State.DashboardState.DrillDown = true
	// Always clear patch state so stale content doesn't linger.
	m.State.Patches = make(map[string]model.PatchLoadState)
	m.patchViewport = nil
	m.patchErr = ""
	m.patchFallback = ""
	if !isRefresh {
		m.State.FocusPane = model.PaneFiles
		m.State.Files = nil
		m.State.SelectedFile = -1
		// Clear freshness state so stale generations from a previous worktree don't leak.
		// FlaggedFiles are session-scoped bookmarks — intentionally preserved across drill-downs.
		m.State.FileChangeGen = make(map[string]int)
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
	outerHeight := m.height - 1 // reserve status bar
	_, innerH := panes.ContentDimensions(m.width, outerHeight)
	visibleEntries := innerH / panes.LinesPerEntry
	if visibleEntries < 1 {
		visibleEntries = 1
	}
	ds := &m.State.DashboardState
	ds.ScrollOffset = panes.EnsureVisible(ds.SelectedIdx, ds.ScrollOffset, visibleEntries, len(ds.Worktrees))
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

// WorktreeSnapshotKey returns a cache key for a worktree's mutable state.
func WorktreeSnapshotKey(wt model.WorktreeInfo) string {
	return fmt.Sprintf("%s|%s|%v|%d", wt.Path, wt.CommitHash, wt.Dirty, wt.ChangedFiles)
}

// maybeLoadPreview triggers a preview load for the selected worktree if not cached.
func (m *Model) maybeLoadPreview() tea.Cmd {
	if m.previewLoader == nil {
		return nil
	}
	ds := &m.State.DashboardState
	if m.width < 100 {
		ds.PreviewFiles = nil // clear stale preview at narrow width
		return nil
	}
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) {
		return nil
	}
	wt := ds.Worktrees[ds.SelectedIdx]
	snap := WorktreeSnapshotKey(wt)

	// Cache hit.
	if ds.PreviewCache != nil {
		if entry, ok := ds.PreviewCache[snap]; ok {
			ds.PreviewFiles = entry.Files
			return nil
		}
	}

	// Cache miss: clear stale preview and fire async load.
	ds.PreviewFiles = nil
	loader := m.previewLoader
	path := wt.Path
	return func() tea.Msg {
		files, err := loader.LoadPreview(context.Background(), path)
		return PreviewLoadedMsg{Path: path, Snap: snap, Files: files, Err: err}
	}
}

// handlePreviewLoaded applies the loaded preview data and caches it.
func (m Model) handlePreviewLoaded(msg PreviewLoadedMsg) (tea.Model, tea.Cmd) {
	ds := &m.State.DashboardState
	if msg.Err != nil {
		ds.PreviewFiles = nil
		return m, nil
	}

	// Limit to top 5 files.
	files := msg.Files
	if len(files) > 5 {
		files = files[:5]
	}

	// Store in cache.
	if ds.PreviewCache == nil {
		ds.PreviewCache = make(map[string]model.PreviewEntry)
	}
	// Evict excess entries when cache exceeds cap.
	if len(ds.PreviewCache) >= maxPreviewCacheSize {
		// Simple eviction: clear the entire cache. A full LRU is overkill
		// for a preview cache — the working set rebuilds quickly.
		ds.PreviewCache = make(map[string]model.PreviewEntry)
	}
	ds.PreviewCache[msg.Snap] = model.PreviewEntry{Files: files}

	// Apply to current view only if the selected worktree's snapshot still matches.
	if ds.SelectedIdx >= 0 && ds.SelectedIdx < len(ds.Worktrees) {
		currentSnap := WorktreeSnapshotKey(ds.Worktrees[ds.SelectedIdx])
		if msg.Snap == currentSnap {
			ds.PreviewFiles = files
		}
	}
	return m, nil
}

// reconcileActivity compares old and new worktree snapshots and updates
// LastActivityAt on new entries when state changes are detected.
func reconcileActivity(old, new []model.WorktreeInfo) {
	oldByPath := make(map[string]model.WorktreeInfo, len(old))
	for _, wt := range old {
		oldByPath[wt.Path] = wt
	}

	now := time.Now()
	for i := range new {
		prev, existed := oldByPath[new[i].Path]
		if !existed {
			// New worktree — mark as active now.
			new[i].LastActivityAt = now
			continue
		}
		// Carry forward previous activity timestamp.
		new[i].LastActivityAt = prev.LastActivityAt

		// Detect state changes: dirty/clean transition, count change, new commit.
		if new[i].Dirty != prev.Dirty ||
			new[i].ChangedFiles != prev.ChangedFiles ||
			new[i].CommitHash != prev.CommitHash {
			new[i].LastActivityAt = now
		}
	}
}

func (m Model) viewDashboard() string {
	outerHeight := m.height - 1 // reserve status bar
	if outerHeight < 3 {
		outerHeight = 3
	}
	ds := m.State.DashboardState

	// Render the base dashboard (list or split view).
	var base string
	showPreview := m.width >= 100 && len(ds.PreviewFiles) > 0
	if showPreview {
		base = m.viewDashboardSplit(outerHeight)
	} else {
		innerW, innerH := panes.ContentDimensions(m.width, outerHeight)
		var content string
		if len(ds.Worktrees) == 0 && m.State.RefreshInFlight {
			content = "Loading worktrees..."
		} else {
			content = panes.RenderDashboard(ds.Worktrees, ds.SelectedIdx, ds.ScrollOffset, innerW, innerH)
		}
		footer := m.dashboardFooter()
		showFoot := m.showFooter() || ds.DeleteInFlight || ds.DeleteErr != ""
		base = panes.BorderedPane(content, "Worktrees", footer, m.width, outerHeight, true, showFoot)
	}

	// Overlay the confirmation dialog on top of the dashboard.
	if ds.ConfirmDelete {
		var label string
		if idx := ds.SelectedIdx; idx >= 0 && idx < len(ds.Worktrees) {
			wt := ds.Worktrees[idx]
			if wt.Branch != "" {
				label = wt.Branch + "\n"
			}
			label += ds.DeletePath
		} else {
			label = filepath.Base(ds.DeletePath)
		}
		body := label
		if ds.DeleteDirty {
			body += "\n\nDIRTY — uncommitted changes will be lost!"
		}
		return panes.OverlayDialog(base, "Delete worktree?", body, "y confirm    n/Esc cancel", m.width, outerHeight)
	}

	return base
}

// viewDashboardSplit renders the dashboard with a side preview pane.
func (m Model) viewDashboardSplit(outerHeight int) string {
	ds := m.State.DashboardState
	showFoot := m.showFooter() || ds.DeleteInFlight || ds.DeleteErr != ""

	// Allocate 60% to worktree list, 40% to preview.
	listW := m.width * 6 / 10
	previewW := m.width - listW

	listInnerW, listInnerH := panes.ContentDimensions(listW, outerHeight)
	previewInnerW, previewInnerH := panes.ContentDimensions(previewW, outerHeight)

	listContent := panes.RenderDashboard(ds.Worktrees, ds.SelectedIdx, ds.ScrollOffset, listInnerW, listInnerH)
	previewContent := panes.RenderPreview(ds.PreviewFiles, previewInnerW, previewInnerH)

	listFooter := m.dashboardFooter()
	left := panes.BorderedPane(listContent, "Worktrees", listFooter, listW, outerHeight, true, showFoot)
	right := panes.BorderedPane(previewContent, "Preview", "", previewW, outerHeight, false, showFoot)

	// Join panes side by side.
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	rows := make([]string, len(leftLines))
	for i := range leftLines {
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		rows[i] = leftLines[i] + r
	}
	return strings.Join(rows, "\n")
}

// dashboardFooter returns the footer text for the worktree pane.
func (m Model) dashboardFooter() string {
	ds := m.State.DashboardState
	if ds.DeleteInFlight {
		return "Deleting..."
	}
	if ds.DeleteErr != "" {
		return ds.DeleteErr
	}
	return fmt.Sprintf("%d worktrees", len(ds.Worktrees))
}
