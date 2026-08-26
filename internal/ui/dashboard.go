package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/notes"
	"github.com/alexivison/scry/internal/review"
	"github.com/alexivison/scry/internal/ui/panes"
)

// maxPreviewCacheSize caps the number of entries in the preview cache.
const maxPreviewCacheSize = 50

// WorktreeLoader loads the worktree list with dirty state and commit info.
type WorktreeLoader interface {
	LoadWorktrees(ctx context.Context) ([]model.WorktreeInfo, error)
}

// DrillDownResult holds the resolved data for a worktree drill-down.
type DrillDownResult struct {
	Compare       model.ResolvedCompare
	Files         []model.FileSummary
	PatchLoader   PatchLoader
	FileDiscarder FileDiscarder
	NoteStore     *notes.Store
	NoteErr       error
}

// DrillDownProvider creates the diff context for a specific worktree.
type DrillDownProvider interface {
	LoadDrillDown(ctx context.Context, worktreePath string, basis model.CompareBasis) (DrillDownResult, error)
}

// PreviewLoader loads the top changed files for a worktree preview.
type PreviewLoader interface {
	LoadPreview(ctx context.Context, worktreePath string, basis model.CompareBasis) ([]model.FileSummary, error)
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

// WithWorktreeLoader sets the WorktreeLoader used for dashboard refresh.
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

// refreshDashboard reloads the worktree list on manual refresh.
func (m Model) refreshDashboard() (tea.Model, tea.Cmd) {
	if !m.State.WorktreeMode || m.worktreeLoader == nil || m.State.DashboardState.DrillDown || m.State.RefreshInFlight {
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

	// Discard stale refresh results from before a deletion.
	if msg.Generation != m.State.DashboardState.RefreshGeneration {
		return m.refreshDashboard()
	}

	if msg.Err != nil {
		m.refreshErr = fmt.Sprintf("worktree refresh failed: %v", msg.Err)
		return m, nil
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
		return m, previewCmd
	}
	return m, nil
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
	ds.DeleteBranch = wt.Branch
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
		ds.DeleteBranch = ""
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
	ds.DeleteBranch = ""
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
	ds.PreviewSnap = ""
	ds.PreviewErr = ""

	// Schedule a refresh to get authoritative state and reload preview.
	return m.refreshDashboard()
}

// startDrillDown begins loading the diff context for a worktree.
// When called as a fresh drill-down (from dashboard), it resets focus to PaneFiles
// and clears stale data so the previous worktree's files don't flash briefly.
// When called as a refresh (already in drill-down), it preserves the current focus pane.
func (m Model) startDrillDown(wt model.WorktreeInfo) (tea.Model, tea.Cmd) {
	if wt.Bare {
		return m, nil
	}
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
		m.State.FileTreeCursor = 0
		m.State.FileTreeCollapsed = make(map[string]bool)
		// Clear freshness state so stale generations from a previous worktree don't leak.
		m.State.FileChangeGen = make(map[string]int)
		m.setNoteStore(nil, nil)
	}

	if m.drillDownProvider == nil {
		return m, nil
	}

	// Bump generation to invalidate any in-flight drill-down load.
	m.State.DashboardState.DrillGeneration++
	gen := m.State.DashboardState.DrillGeneration

	path := wt.Path
	provider := m.drillDownProvider
	basis := m.State.CompareBasis
	return m, func() tea.Msg {
		result, err := provider.LoadDrillDown(context.Background(), path, basis)
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
	m.State.CompareBasis = msg.Result.Compare.Basis
	m.State.Files = msg.Result.Files
	m.State.Patches = make(map[string]model.PatchLoadState)
	m.patchLoader = msg.Result.PatchLoader
	m.fileDiscarder = msg.Result.FileDiscarder
	noteCmd := m.setNoteStore(msg.Result.NoteStore, msg.Result.NoteErr)

	// Reconcile selection: match by path, fallback to clamped index.
	review.ReconcileSelection(&m.State, prevPath)
	reconcileFileTreeToSelection(&m.State)

	// If in patch view, reload the selected file's patch.
	updated, patchCmd := m.loadSelectedPatch()
	return updated, tea.Batch(patchCmd, noteCmd)
}

// syncDashboardScroll adjusts the dashboard scroll offset so the selected worktree stays visible.
func (m *Model) syncDashboardScroll() {
	if m.height == 0 {
		return
	}
	outerHeight := m.height - 1 // reserve status bar
	_, innerH := panes.UnboxedSectionDimensions(m.width, outerHeight)
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
	m.setNoteStore(nil, nil)
}

// updateDrillDown handles keys when in worktree drill-down (file/patch view for a single worktree).
func (m Model) updateDrillDown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.returnToDashboard()
		return m, nil
	case "h", "left":
		row, ok := m.currentFileTreeRow()
		if !ok || (row.Kind == panes.FileTreeRowFile && parentDir(row.Path) == "") {
			m.returnToDashboard()
			return m, nil
		}
	}
	return m.updateFiles(msg)
}

// WorktreeSnapshotKey returns a cache key for a worktree's mutable state.
func WorktreeSnapshotKey(wt model.WorktreeInfo, basis model.CompareBasis) string {
	if basis != model.CompareBasisLocalTrunk && basis != model.CompareBasisHeadDirty {
		basis = model.CompareBasisUpstream
	}
	return fmt.Sprintf("%s|%s|%v|%d|%s", wt.Path, wt.CommitHash, wt.Dirty, wt.ChangedFiles, basis)
}

// maybeLoadPreview triggers a preview load for the selected worktree if not cached.
func (m *Model) maybeLoadPreview() tea.Cmd {
	if m.previewLoader == nil {
		return nil
	}
	ds := &m.State.DashboardState
	if m.width < 100 {
		ds.PreviewFiles = nil // clear stale preview at narrow width
		ds.PreviewSnap = ""
		ds.PreviewErr = ""
		return nil
	}
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) {
		ds.PreviewFiles = nil
		ds.PreviewSnap = ""
		ds.PreviewErr = ""
		return nil
	}
	wt := ds.Worktrees[ds.SelectedIdx]
	if wt.Bare {
		ds.PreviewFiles = nil
		ds.PreviewSnap = WorktreeSnapshotKey(wt, m.State.CompareBasis)
		ds.PreviewErr = ""
		return nil
	}
	snap := WorktreeSnapshotKey(wt, m.State.CompareBasis)

	// Cache hit.
	if ds.PreviewCache != nil {
		if entry, ok := ds.PreviewCache[snap]; ok {
			ds.PreviewFiles = entry.Files
			ds.PreviewSnap = snap
			ds.PreviewErr = ""
			return nil
		}
	}

	// Cache miss: clear stale preview and fire async load.
	ds.PreviewFiles = nil
	ds.PreviewSnap = ""
	ds.PreviewErr = ""
	loader := m.previewLoader
	path := wt.Path
	basis := m.State.CompareBasis
	return func() tea.Msg {
		files, err := loader.LoadPreview(context.Background(), path, basis)
		return PreviewLoadedMsg{Path: path, Snap: snap, Files: files, Err: err}
	}
}

// handlePreviewLoaded applies the loaded preview data and caches it.
func (m Model) handlePreviewLoaded(msg PreviewLoadedMsg) (tea.Model, tea.Cmd) {
	ds := &m.State.DashboardState
	if msg.Err != nil {
		if ds.SelectedIdx >= 0 && ds.SelectedIdx < len(ds.Worktrees) {
			currentSnap := WorktreeSnapshotKey(ds.Worktrees[ds.SelectedIdx], m.State.CompareBasis)
			if msg.Snap == currentSnap {
				ds.PreviewFiles = nil
				ds.PreviewSnap = msg.Snap
				ds.PreviewErr = msg.Err.Error()
			}
		}
		return m, nil
	}

	files := msg.Files

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
		currentSnap := WorktreeSnapshotKey(ds.Worktrees[ds.SelectedIdx], m.State.CompareBasis)
		if msg.Snap == currentSnap {
			ds.PreviewFiles = files
			ds.PreviewSnap = msg.Snap
			ds.PreviewErr = ""
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
		loadedActivity := new[i].LastActivityAt
		prev, existed := oldByPath[new[i].Path]
		if !existed {
			if len(old) == 0 {
				new[i].LastActivityAt = loadedActivity
				continue
			}
			// New worktree — mark as active now.
			if loadedActivity.IsZero() {
				new[i].LastActivityAt = now
			}
			continue
		}
		// Carry forward previous activity timestamp.
		new[i].LastActivityAt = prev.LastActivityAt

		// Detect state changes: dirty/clean transition, count change, new commit.
		if worktreeActivityChanged(prev, new[i]) {
			if loadedActivity.IsZero() {
				new[i].LastActivityAt = now
			} else {
				new[i].LastActivityAt = loadedActivity
			}
		}
	}
}

func worktreeActivityChanged(prev, next model.WorktreeInfo) bool {
	if next.Dirty != prev.Dirty || next.ChangedFiles != prev.ChangedFiles {
		return true
	}
	return prev.CommitHash != "" && next.CommitHash != "" && next.CommitHash != prev.CommitHash
}

func (m Model) viewDashboard() string {
	outerHeight := m.height - 1 // reserve status bar
	if outerHeight < 3 {
		outerHeight = 3
	}
	ds := m.State.DashboardState

	// Render the base dashboard (list or split view).
	var base string
	showPreview := m.dashboardPreviewVisible()
	if showPreview {
		base = m.viewDashboardSplit(outerHeight)
	} else {
		innerW, innerH := panes.UnboxedSectionDimensions(m.width, outerHeight)
		var content string
		if len(ds.Worktrees) == 0 && m.State.RefreshInFlight {
			content = "Loading worktrees..."
		} else {
			content = panes.RenderDashboard(ds.Worktrees, ds.SelectedIdx, ds.ScrollOffset, innerW, innerH)
		}
		meta := ""
		if m.showFooter() || ds.DeleteInFlight || ds.DeleteErr != "" {
			meta = m.dashboardFooter()
		}
		base = panes.UnboxedSection(content, "Worktrees", meta, m.width, outerHeight, true)
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

func (m Model) dashboardPreviewVisible() bool {
	if m.width < 100 {
		return false
	}
	ds := m.State.DashboardState
	return m.previewLoader != nil || len(ds.PreviewFiles) > 0 || ds.PreviewSnap != "" || ds.PreviewErr != ""
}

// viewDashboardSplit renders the dashboard with a side preview pane.
func (m Model) viewDashboardSplit(outerHeight int) string {
	ds := m.State.DashboardState
	meta := ""
	if m.showFooter() || ds.DeleteInFlight || ds.DeleteErr != "" {
		meta = m.dashboardFooter()
	}

	// Allocate 60% to worktree list, 40% to preview.
	availableWidth := m.width - 1 // reserve one column for the divider
	if availableWidth < 2 {
		return ""
	}
	listW := availableWidth * 6 / 10
	if listW >= availableWidth {
		listW = availableWidth - 1
	}
	previewW := availableWidth - listW

	listInnerW, listInnerH := panes.UnboxedSectionDimensions(listW, outerHeight)
	previewInnerW, previewInnerH := panes.UnboxedSectionDimensions(previewW, outerHeight)

	var listContent string
	if len(ds.Worktrees) == 0 && m.State.RefreshInFlight {
		listContent = "Loading worktrees..."
	} else {
		listContent = panes.RenderDashboard(ds.Worktrees, ds.SelectedIdx, ds.ScrollOffset, listInnerW, listInnerH)
	}
	previewContent := m.dashboardPreviewContent(previewInnerW, previewInnerH)
	previewMeta := m.dashboardPreviewMeta()

	left := panes.UnboxedSection(listContent, "Worktrees", meta, listW, outerHeight, true)
	right := panes.UnboxedSection(previewContent, "Preview", previewMeta, previewW, outerHeight, false)

	return panes.JoinColumns(left, right, listW, previewW)
}

func (m Model) dashboardPreviewMeta() string {
	ds := m.State.DashboardState
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) || ds.Worktrees[ds.SelectedIdx].Bare {
		return ""
	}
	if ds.PreviewErr != "" || !m.dashboardPreviewFilesCurrent() {
		return ""
	}
	counts := aggregateFileCounts(ds.PreviewFiles, model.FileFilterAll)
	return panes.RenderCounts(counts, false)
}

func (m Model) dashboardPreviewContent(width, height int) string {
	ds := m.State.DashboardState
	if len(ds.Worktrees) == 0 {
		if m.State.RefreshInFlight {
			return "Loading worktrees..."
		}
		return "No worktree selected."
	}
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) {
		return "Select a worktree."
	}
	if ds.Worktrees[ds.SelectedIdx].Bare {
		return "Bare repository has no working tree."
	}

	snap := WorktreeSnapshotKey(ds.Worktrees[ds.SelectedIdx], m.State.CompareBasis)
	if len(ds.PreviewFiles) > 0 && (ds.PreviewSnap == "" || ds.PreviewSnap == snap) {
		return panes.RenderPreview(ds.PreviewFiles, width, height)
	}
	if ds.PreviewSnap == snap {
		if ds.PreviewErr != "" {
			return fmt.Sprintf("Preview failed: %s", ds.PreviewErr)
		}
		return panes.RenderPreview(ds.PreviewFiles, width, height)
	}
	if m.previewLoader != nil {
		return "Loading preview..."
	}
	return "No preview available."
}

func (m Model) dashboardPreviewFilesCurrent() bool {
	ds := m.State.DashboardState
	if ds.SelectedIdx < 0 || ds.SelectedIdx >= len(ds.Worktrees) {
		return false
	}

	snap := WorktreeSnapshotKey(ds.Worktrees[ds.SelectedIdx], m.State.CompareBasis)
	return ds.PreviewSnap == snap || (ds.PreviewSnap == "" && len(ds.PreviewFiles) > 0)
}

// dashboardFooter returns the footer text for the worktree pane.
func (m Model) dashboardFooter() string {
	ds := m.State.DashboardState
	if ds.DeleteInFlight {
		if ds.DeleteBranch != "" {
			return fmt.Sprintf("Removing %s...", ds.DeleteBranch)
		}
		return "Removing worktree..."
	}
	if ds.DeleteErr != "" {
		return ds.DeleteErr
	}
	return fmt.Sprintf("%d worktrees", len(ds.Worktrees))
}
