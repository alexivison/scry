// Package ui implements the Bubble Tea TUI for scry.
package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/commit"
	"github.com/alexivison/scry/internal/diff"
	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/review"
	"github.com/alexivison/scry/internal/search"
	"github.com/alexivison/scry/internal/terminal"
	"github.com/alexivison/scry/internal/ui/panes"
	"github.com/alexivison/scry/internal/ui/theme"
	"github.com/alexivison/scry/internal/watch"
)

// PatchLoader loads a file's unified diff.
type PatchLoader interface {
	LoadPatch(ctx context.Context, cmp model.ResolvedCompare, filePath string, status model.FileStatus, ignoreWhitespace bool) (model.FilePatch, error)
}

// MetadataLoader lists changed files for a compare range.
type MetadataLoader interface {
	ListFiles(ctx context.Context, cmp model.ResolvedCompare) ([]model.FileSummary, error)
}

// CompareReResolver re-resolves the compare specification against current refs.
type CompareReResolver interface {
	Resolve(ctx context.Context, req model.CompareRequest) (model.ResolvedCompare, error)
}

// WatchFingerprinter computes a repo state fingerprint for watch mode.
type WatchFingerprinter interface {
	Fingerprint(ctx context.Context, baseRef string, workingTree bool) (string, error)
}

// CommitProvider generates commit messages. The implementation is responsible
// for collecting any diff/file data it needs (e.g. from the git index).
type CommitProvider interface {
	Generate(ctx context.Context) (string, error)
}

// CommitExecutor runs git commit with a message and returns the short SHA.
type CommitExecutor interface {
	Execute(ctx context.Context, message string) (string, error)
}

// Model is the top-level Bubble Tea model for scry.
type Model struct {
	State           model.AppState
	patchLoader     PatchLoader
	metadataLoader  MetadataLoader
	compareResolver CompareReResolver
	compareRequest  model.CompareRequest
	patchViewport   *panes.PatchViewport
	patchErr        string
	patchFallback   string // fallback message for binary/submodule/oversized files
	showHelp        bool
	width           int
	height          int
	quitting        bool
	tooSmall        bool // terminal below minimum dimensions
	sizeErr         string

	searchInput    string        // text being typed in search mode
	searchIndex    *search.Index // built when patch is loaded
	searchNotFound string        // "Pattern not found: <query>" message
	refreshErr     string        // shown in status bar when metadata reload fails
	fileListScroll int           // scroll offset for file list in split mode

	// Scroll preservation state: saved before refresh, restored if content unchanged.
	savedFilePath     string // path of the file whose scroll was saved
	savedContentHash  string
	savedScrollOffset int
	savedCurrentHunk  int
	savedSearchQuery  string

	commitProvider CommitProvider     // optional provider for AI commit messages
	commitExecutor CommitExecutor     // optional executor for git commit
	commitCancel   context.CancelFunc // cancels the in-flight commit generation request

	fingerprinter WatchFingerprinter // optional watch mode fingerprinter
	watchBaseRef  string             // symbolic base ref for fingerprint checks
	lastCheckAt   time.Time          // when the last fingerprint check completed
	watchErr      bool               // true when last fingerprint check failed

	worktreeLoader    WorktreeLoader    // optional loader for worktree dashboard
	drillDownProvider DrillDownProvider // optional provider for worktree drill-down
	worktreeRemover   WorktreeRemover   // optional remover for worktree deletion
}

// NewModel creates a Model from bootstrap data. Sets SelectedFile to -1
// when the file list is empty, 0 otherwise.
func NewModel(state model.AppState, opts ...ModelOption) Model {
	if len(state.Files) == 0 {
		state.SelectedFile = -1
	} else {
		if state.SelectedFile < 0 {
			state.SelectedFile = 0
		}
		if state.SelectedFile >= len(state.Files) {
			state.SelectedFile = len(state.Files) - 1
		}
	}
	if state.FocusPane == "" {
		state.FocusPane = model.PaneFiles
	}
	if state.Patches == nil {
		state.Patches = make(map[string]model.PatchLoadState)
	}
	m := Model{State: state}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// ModelOption configures optional Model dependencies.
type ModelOption func(*Model)

// WithPatchLoader sets the PatchLoader used to load file diffs on Enter.
func WithPatchLoader(pl PatchLoader) ModelOption {
	return func(m *Model) { m.patchLoader = pl }
}

// WithMetadataLoader sets the MetadataLoader used to reload file lists on refresh.
func WithMetadataLoader(ml MetadataLoader) ModelOption {
	return func(m *Model) { m.metadataLoader = ml }
}

// WithCompareResolver sets the resolver used to re-resolve compare refs on refresh.
func WithCompareResolver(cr CompareReResolver, req model.CompareRequest) ModelOption {
	return func(m *Model) {
		m.compareResolver = cr
		m.compareRequest = req
	}
}

// WithCommitProvider sets the CommitProvider used for AI commit message generation.
func WithCommitProvider(cp CommitProvider) ModelOption {
	return func(m *Model) { m.commitProvider = cp }
}

// WithCommitExecutor sets the CommitExecutor used to run git commit.
func WithCommitExecutor(ce CommitExecutor) ModelOption {
	return func(m *Model) { m.commitExecutor = ce }
}

// WithWatch sets the WatchFingerprinter and symbolic base ref for watch mode.
func WithWatch(fp WatchFingerprinter, baseRef string) ModelOption {
	return func(m *Model) {
		m.fingerprinter = fp
		m.watchBaseRef = baseRef
	}
}

// PatchLoadedMsg is sent when an async patch load completes.
type PatchLoadedMsg struct {
	Path  string
	Patch model.FilePatch
	Gen   int
	Err   error
}

// MetadataLoadedMsg is sent when an async metadata reload completes.
type MetadataLoadedMsg struct {
	Compare *model.ResolvedCompare // non-nil when compare was re-resolved
	Files   []model.FileSummary
	Gen     int
	Err     error
}

// CommitGeneratedMsg is sent when an async commit message generation completes.
type CommitGeneratedMsg struct {
	Message    string
	Err        error
	Generation int // matches CommitState.Generation to detect stale results
}

// CommitExecutedMsg is sent when an async git commit completes.
type CommitExecutedMsg struct {
	SHA        string
	Err        error
	Generation int
}

// CommitEditedMsg is sent when the user finishes editing a commit message in $EDITOR.
type CommitEditedMsg struct {
	Message string
	Err     error
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.State.WorktreeMode && m.State.WatchEnabled && m.worktreeLoader != nil {
		return watch.TickCmd(m.State.WatchInterval)
	}
	if m.State.WatchEnabled && m.fingerprinter != nil {
		return m.buildCheckCmd()
	}
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if err := terminal.CheckDimensions(msg.Width, msg.Height); err != nil {
			m.tooSmall = true
			m.sizeErr = err.Error()
		} else {
			m.tooSmall = false
			m.sizeErr = ""
		}
		return m, nil

	case PatchLoadedMsg:
		return m.handlePatchLoaded(msg)

	case MetadataLoadedMsg:
		return m.handleMetadataLoaded(msg)

	case CommitGeneratedMsg:
		return m.handleCommitGenerated(msg)

	case CommitExecutedMsg:
		return m.handleCommitExecuted(msg)

	case CommitEditedMsg:
		return m.handleCommitEdited(msg)

	case watch.FSEventMsg:
		// fsnotify detected a file change — trigger immediate fingerprint check
		// without rescheduling the polling timer (to avoid timer multiplication).
		if m.State.WatchEnabled && m.fingerprinter != nil && !m.State.WorktreeMode {
			return m, m.buildFSCheckCmd()
		}
		if m.State.WorktreeMode && m.State.WatchEnabled && m.worktreeLoader != nil {
			return m.handleDashboardTick()
		}
		return m, nil

	case watch.TickMsg:
		if m.State.WorktreeMode {
			return m.handleDashboardTick()
		}
		return m.handleWatchTick(msg)

	case watch.FingerprintMsg:
		return m.handleWatchFingerprint(msg)

	case WorktreeRefreshedMsg:
		return m.handleWorktreeRefreshed(msg)

	case DrillDownLoadedMsg:
		return m.handleDrillDownLoaded(msg)

	case WorktreeRemovedMsg:
		return m.handleWorktreeRemoved(msg)

	case tea.KeyMsg:
		if m.tooSmall {
			if msg.String() == "q" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}
		// r triggers refresh from any pane (except help/search/commit).
		if msg.String() == "r" && !m.showHelp && m.State.FocusPane != model.PaneSearch && m.State.FocusPane != model.PaneCommit {
			if m.State.WorktreeMode {
				if m.State.DashboardState.DrillDown {
					// In drill-down, re-load the current worktree's diff.
					ds := m.State.DashboardState
					if ds.SelectedIdx >= 0 && ds.SelectedIdx < len(ds.Worktrees) {
						return m.startDrillDown(ds.Worktrees[ds.SelectedIdx])
					}
				}
				// Top-level dashboard: no-op (auto-refresh handles updates).
				return m, nil
			}
			return m.startRefresh()
		}
		// W toggles whitespace ignore from any pane (except help/search/commit).
		if msg.String() == "W" && !m.showHelp && m.State.FocusPane != model.PaneSearch && m.State.FocusPane != model.PaneCommit {
			return m.toggleWhitespace()
		}
		// Tab toggles layout (except during help/search/commit).
		if msg.Type == tea.KeyTab && !m.showHelp && m.State.FocusPane != model.PaneSearch && m.State.FocusPane != model.PaneCommit {
			return m.toggleLayout()
		}
		if m.showHelp {
			return m.updateHelp(msg)
		}
		if m.State.FocusPane == model.PaneSearch {
			return m.updateSearch(msg)
		}
		if m.State.FocusPane == model.PaneCommit {
			return m.updateCommit(msg)
		}
		if m.State.FocusPane == model.PaneIdle {
			return m.updateIdle(msg)
		}
		if m.State.FocusPane == model.PaneDashboard {
			return m.updateDashboard(msg)
		}
		if m.State.FocusPane == model.PanePatch {
			return m.updatePatch(msg)
		}
		// In worktree drill-down, h/Esc returns to dashboard.
		if m.State.WorktreeMode && m.State.DashboardState.DrillDown {
			return m.updateDrillDown(msg)
		}
		return m.updateFiles(msg)
	}
	return m, nil
}

func (m Model) updateFiles(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.State.SelectedFile < len(m.State.Files)-1 {
			m.State.SelectedFile++
			m.syncFileListScroll()
			if m.State.Layout == model.LayoutSplit {
				return m.selectFile()
			}
		}
	case "k", "up":
		if m.State.SelectedFile > 0 {
			m.State.SelectedFile--
			m.syncFileListScroll()
			if m.State.Layout == model.LayoutSplit {
				return m.selectFile()
			}
		}
	case "l", "enter":
		if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
			m.State.FocusPane = model.PanePatch
			return m.selectFile()
		}
	case "c":
		if m.State.CommitEnabled && m.commitProvider != nil {
			return m.startCommitGeneration()
		}
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
	}
	return m, nil
}

// cancelCommit cancels any in-flight commit generation request.
func (m *Model) cancelCommit() {
	if m.commitCancel != nil {
		m.commitCancel()
		m.commitCancel = nil
	}
}

// startCommitGeneration transitions to the commit pane and fires an async generation.
func (m Model) startCommitGeneration() (tea.Model, tea.Cmd) {
	m.cancelCommit()
	m.State.FocusPane = model.PaneCommit
	gen := m.State.CommitState.Generation + 1
	m.State.CommitState = model.CommitState{InFlight: true, Generation: gen}
	cmd, cancel := m.buildCommitCmd(gen)
	m.commitCancel = cancel
	return m, cmd
}

// handleCommitGenerated processes the result of an async commit generation.
func (m Model) handleCommitGenerated(msg CommitGeneratedMsg) (tea.Model, tea.Cmd) {
	if msg.Generation != m.State.CommitState.Generation {
		return m, nil // stale result from a cancelled/superseded generation
	}
	m.State.CommitState.InFlight = false
	if msg.Err != nil {
		m.State.CommitState.Err = msg.Err
		m.State.CommitState.GeneratedMessage = ""
		return m, nil
	}
	m.State.CommitState.GeneratedMessage = msg.Message
	m.State.CommitState.Err = nil

	// Auto-commit: skip confirmation and execute immediately.
	if m.State.CommitAuto && m.commitExecutor != nil {
		return m.startCommitExecution()
	}
	return m, nil
}

// handleCommitExecuted processes the result of an async git commit.
func (m Model) handleCommitExecuted(msg CommitExecutedMsg) (tea.Model, tea.Cmd) {
	if msg.Generation != m.State.CommitState.Generation {
		return m, nil
	}
	m.State.CommitState.Executing = false
	if msg.Err != nil {
		m.State.CommitState.CommitErr = msg.Err
		return m, nil
	}
	m.State.CommitState.CommitSHA = msg.SHA

	// Reuse the shared refresh orchestrator to clear stale diff state.
	return m.startRefresh()
}

// handleCommitEdited processes the result of an editor session.
func (m Model) handleCommitEdited(msg CommitEditedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.State.CommitState.Err = msg.Err
		m.State.CommitState.GeneratedMessage = "" // prevent committing stale pre-edit text
		return m, nil
	}
	m.State.CommitState.GeneratedMessage = msg.Message
	return m, nil
}

// updateCommit handles key events in the commit pane.
func (m Model) updateCommit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Block most actions while commit is executing (irreversible side effect).
	if m.State.CommitState.Executing {
		if msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "enter":
		if m.State.CommitState.GeneratedMessage != "" && m.State.CommitState.CommitSHA == "" && m.commitExecutor != nil {
			return m.startCommitExecution()
		}
	case "e":
		if m.State.CommitState.GeneratedMessage != "" && m.State.CommitState.CommitSHA == "" {
			return m.startEditor()
		}
	case "esc":
		m.cancelCommit()
		m.State.FocusPane = model.PaneFiles
		// Bump generation to invalidate any in-flight goroutine from the cancelled run.
		m.State.CommitState = model.CommitState{Generation: m.State.CommitState.Generation + 1}
	case "r":
		if m.commitProvider != nil && m.State.CommitState.CommitSHA == "" {
			m.cancelCommit()
			gen := m.State.CommitState.Generation + 1
			m.State.CommitState = model.CommitState{InFlight: true, Generation: gen}
			cmd, cancel := m.buildCommitCmd(gen)
			m.commitCancel = cancel
			return m, cmd
		}
	case "q":
		m.cancelCommit()
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// startCommitExecution begins async commit execution.
func (m Model) startCommitExecution() (tea.Model, tea.Cmd) {
	gen := m.State.CommitState.Generation
	m.State.CommitState.Executing = true
	m.State.CommitState.CommitErr = nil
	m.State.CommitState.CommitSHA = ""

	executor := m.commitExecutor
	message := m.State.CommitState.GeneratedMessage

	cmd := func() tea.Msg {
		sha, err := executor.Execute(context.Background(), message)
		return CommitExecutedMsg{SHA: sha, Err: err, Generation: gen}
	}
	return m, cmd
}

// startEditor opens $EDITOR with the commit message for user editing.
func (m Model) startEditor() (tea.Model, tea.Cmd) {
	message := m.State.CommitState.GeneratedMessage
	cmd, tmpPath, err := commit.PrepareEditorCmd(message)
	if err != nil {
		m.State.CommitState.Err = err
		return m, nil
	}

	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			os.Remove(tmpPath)
			return CommitEditedMsg{Err: err}
		}
		edited, readErr := commit.ReadEditedMessage(tmpPath)
		os.Remove(tmpPath)
		return CommitEditedMsg{Message: edited, Err: readErr}
	})
}

// buildCommitCmd creates an async Cmd with a cancellable context and returns
// both the command and the cancel func for the caller to store on the model.
func (m Model) buildCommitCmd(gen int) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	provider := m.commitProvider
	cmd := func() tea.Msg {
		msg, err := provider.Generate(ctx)
		return CommitGeneratedMsg{Message: msg, Err: err, Generation: gen}
	}
	return cmd, cancel
}

// --- Watch mode handlers ---

// buildCheckCmd creates an async Cmd that computes the repo fingerprint (polling path).
func (m Model) buildCheckCmd() tea.Cmd {
	fp := m.fingerprinter
	baseRef := m.watchBaseRef
	wt := m.State.Compare.WorkingTree
	return func() tea.Msg {
		result, err := fp.Fingerprint(context.Background(), baseRef, wt)
		return watch.FingerprintMsg{Fingerprint: result, Err: err}
	}
}

// buildFSCheckCmd creates an async Cmd that computes the repo fingerprint (fsnotify path).
// The result has FromFS=true so handleWatchFingerprint won't reschedule the polling timer.
func (m Model) buildFSCheckCmd() tea.Cmd {
	fp := m.fingerprinter
	baseRef := m.watchBaseRef
	wt := m.State.Compare.WorkingTree
	return func() tea.Msg {
		result, err := fp.Fingerprint(context.Background(), baseRef, wt)
		return watch.FingerprintMsg{Fingerprint: result, Err: err, FromFS: true}
	}
}

// handleWatchTick fires a fingerprint check on each watch interval tick.
func (m Model) handleWatchTick(_ watch.TickMsg) (tea.Model, tea.Cmd) {
	if !m.State.WatchEnabled || m.fingerprinter == nil {
		return m, nil
	}
	return m, m.buildCheckCmd()
}

// handleWatchFingerprint processes a fingerprint check result.
func (m Model) handleWatchFingerprint(msg watch.FingerprintMsg) (tea.Model, tea.Cmd) {
	if !m.State.WatchEnabled {
		return m, nil
	}
	m.lastCheckAt = time.Now()

	// tickCmd returns the polling tick command, or nil for FS-triggered checks
	// (to avoid multiplying polling timers).
	tickCmd := func() tea.Cmd {
		if msg.FromFS {
			return nil
		}
		return watch.TickCmd(m.State.WatchInterval)
	}

	if msg.Err != nil {
		m.watchErr = true
		return m, tickCmd()
	}
	m.watchErr = false

	// First fingerprint: seed without refresh (bootstrap already loaded data).
	// Exception: in idle mode, bootstrap had no files — refresh to catch any
	// changes that occurred between bootstrap and this first fingerprint.
	if m.State.LastFingerprint == "" {
		m.State.LastFingerprint = msg.Fingerprint
		if m.State.FocusPane == model.PaneIdle {
			refreshed, refreshCmd := m.startRefresh()
			if tc := tickCmd(); tc != nil {
				return refreshed, tea.Batch(refreshCmd, tc)
			}
			return refreshed, refreshCmd
		}
		return m, tickCmd()
	}

	if watch.ShouldRefresh(&m.State, msg.Fingerprint) {
		m.State.LastFingerprint = msg.Fingerprint
		refreshed, refreshCmd := m.startRefresh()
		if tc := tickCmd(); tc != nil {
			return refreshed, tea.Batch(refreshCmd, tc)
		}
		return refreshed, refreshCmd
	}

	// When RefreshInFlight is true, LastFingerprint is intentionally NOT updated
	// so the mismatch triggers a refresh once the in-flight one completes.
	return m, tickCmd()
}

// selectFile checks the cache and either uses a cached result or fires an async load.
func (m Model) selectFile() (tea.Model, tea.Cmd) {
	file := m.State.Files[m.State.SelectedFile]
	path := file.Path

	// Cache hit: use cached result directly.
	if ps, hit := review.CacheLookup(m.State, path); hit {
		m.applyPatchResult(ps)
		return m, nil
	}

	if m.patchLoader == nil {
		return m, nil
	}

	// Cache miss: mark loading and fire async Cmd.
	review.MarkLoading(&m.State, path)
	m.patchViewport = nil
	m.patchErr = ""
	m.patchFallback = ""

	gen := m.State.CacheGeneration
	cmp := m.State.Compare
	ignoreWS := m.State.IgnoreWhitespace
	status := file.Status
	loader := m.patchLoader

	cmd := func() tea.Msg {
		fp, err := loader.LoadPatch(context.Background(), cmp, path, status, ignoreWS)
		return PatchLoadedMsg{Path: path, Patch: fp, Gen: gen, Err: err}
	}

	return m, cmd
}

func (m Model) handlePatchLoaded(msg PatchLoadedMsg) (tea.Model, tea.Cmd) {
	if review.IsStaleGeneration(msg.Gen, m.State.CacheGeneration) {
		return m, nil
	}

	var patch *model.FilePatch
	if msg.Err == nil {
		patch = &msg.Patch
	}
	review.CacheStore(&m.State, msg.Path, patch, msg.Err)

	// Only update viewport if this message is for the currently selected file.
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		selected := m.State.Files[m.State.SelectedFile].Path
		if selected == msg.Path {
			ps := m.State.Patches[msg.Path]
			m.applyPatchResult(ps)
			// Restore scroll if same file and content hash matches pre-refresh snapshot.
			if m.savedFilePath == msg.Path && m.savedContentHash != "" &&
				ps.ContentHash == m.savedContentHash && m.patchViewport != nil {
				m.patchViewport.ScrollOffset = m.savedScrollOffset
				m.patchViewport.CurrentHunk = m.savedCurrentHunk
				m.State.SearchQuery = m.savedSearchQuery
			} else {
				// Content changed or file changed — clear search state.
				m.State.SearchQuery = ""
			}
			// Always clear saved state after use (or mismatch) to prevent bleed.
			m.savedFilePath = ""
			m.savedContentHash = ""
			m.savedSearchQuery = ""
		}
	}

	return m, nil
}

// startRefresh uses the shared refresh orchestrator to bump generation,
// optionally re-resolve the compare, and fire an async metadata reload.
// Patch cache is selectively invalidated when metadata arrives (not blanket-cleared).
func (m Model) startRefresh() (tea.Model, tea.Cmd) {
	review.PrepareRefresh(&m.State)
	m.refreshErr = ""

	if m.metadataLoader == nil {
		m.State.RefreshInFlight = false
		return m, nil
	}

	gen := m.State.CacheGeneration
	cmp := m.State.Compare
	loader := m.metadataLoader
	resolver := m.compareResolver
	req := m.compareRequest

	cmd := func() tea.Msg {
		ctx := context.Background()

		// Re-resolve compare if a resolver is configured.
		var resolvedCmp *model.ResolvedCompare
		if resolver != nil {
			newCmp, err := resolver.Resolve(ctx, req)
			if err != nil {
				return MetadataLoadedMsg{Gen: gen, Err: err}
			}
			resolvedCmp = &newCmp
			cmp = newCmp
		}

		files, err := loader.ListFiles(ctx, cmp)
		return MetadataLoadedMsg{Compare: resolvedCmp, Files: files, Gen: gen, Err: err}
	}
	return m, cmd
}

// toggleWhitespace flips IgnoreWhitespace, resets the patch cache, and reloads
// the selected file's patch. Unlike startRefresh, it does NOT reload metadata.
func (m Model) toggleWhitespace() (tea.Model, tea.Cmd) {
	m.State.IgnoreWhitespace = !m.State.IgnoreWhitespace
	review.BumpGeneration(&m.State)
	review.ClearPatches(&m.State)
	m.patchViewport = nil
	m.patchErr = ""
	m.refreshErr = ""
	m.searchIndex = nil
	m.searchNotFound = ""

	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		return m.selectFile()
	}
	return m, nil
}

func (m Model) handleMetadataLoaded(msg MetadataLoadedMsg) (tea.Model, tea.Cmd) {
	if review.IsStaleGeneration(msg.Gen, m.State.CacheGeneration) {
		return m, nil
	}
	if msg.Err != nil {
		m.State.RefreshInFlight = false
		m.refreshErr = fmt.Sprintf("refresh failed: %v", msg.Err)
		return m, nil
	}
	m.refreshErr = ""

	// Apply re-resolved compare if present.
	if msg.Compare != nil {
		m.State.Compare = *msg.Compare
	}

	// Save scroll state for the selected file before invalidation.
	m.savedFilePath = ""
	m.savedContentHash = ""
	m.savedScrollOffset = 0
	m.savedCurrentHunk = 0
	m.savedSearchQuery = ""
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		path := m.State.Files[m.State.SelectedFile].Path
		m.savedFilePath = path
		if ps, ok := m.State.Patches[path]; ok && ps.ContentHash != "" {
			m.savedContentHash = ps.ContentHash
		}
		if m.patchViewport != nil {
			m.savedScrollOffset = m.patchViewport.ScrollOffset
			m.savedCurrentHunk = m.patchViewport.CurrentHunk
		}
		m.savedSearchQuery = m.State.SearchQuery
	}

	// Selectively invalidate cache: preserve unchanged files, evict changed/removed.
	review.SelectiveInvalidate(&m.State, m.State.Files, msg.Files)

	var selectedPath string
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		selectedPath = m.State.Files[m.State.SelectedFile].Path
		// Force-evict the selected file to get a fresh content-hash comparison.
		delete(m.State.Patches, selectedPath)
	}
	m.State.Files = msg.Files
	review.ReconcileSelection(&m.State, selectedPath)
	review.CompleteRefresh(&m.State)

	// Selected file was evicted for reload; clear viewport.
	m.patchViewport = nil
	m.patchErr = ""
	m.searchIndex = nil
	m.searchNotFound = ""

	// Transition from idle to review when first files arrive.
	if m.State.FocusPane == model.PaneIdle && len(m.State.Files) > 0 {
		m.State.FocusPane = model.PaneFiles
	}

	if m.State.SelectedFile < 0 {
		return m, nil
	}

	return m.loadSelectedPatch()
}

// loadSelectedPatch fires an async patch load for the currently selected file
// if in the patch pane or in split layout.
func (m Model) loadSelectedPatch() (tea.Model, tea.Cmd) {
	needsPatch := m.State.FocusPane == model.PanePatch || m.State.Layout == model.LayoutSplit
	if needsPatch && m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		return m.selectFile()
	}
	return m, nil
}

// applyPatchResult sets the viewport or error from a PatchLoadState.
func (m *Model) applyPatchResult(ps model.PatchLoadState) {
	if ps.Err != nil {
		m.patchViewport = nil
		m.searchIndex = nil

		var summary model.FileSummary
		if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
			summary = m.State.Files[m.State.SelectedFile]
		}

		if fb := buildFallback(summary, ps.Err); fb != "" {
			m.patchFallback = fb
			m.patchErr = ""
			return
		}

		m.patchErr = ps.Err.Error()
		m.patchFallback = ""
		return
	}
	m.patchErr = ""
	m.patchFallback = ""
	if ps.Patch == nil {
		m.patchViewport = nil
		return
	}
	vp := panes.NewPatchViewport(*ps.Patch)
	vp.Width = m.width
	vp.Height = m.height - 1
	vp.GutterVisible = m.width >= 60
	m.patchViewport = vp
	m.searchIndex = search.Build(*ps.Patch)
	m.searchNotFound = ""
}

func isSentinelError(err error) bool {
	return errors.Is(err, model.ErrBinaryFile) ||
		errors.Is(err, model.ErrSubmodule) ||
		errors.Is(err, model.ErrOversized)
}

// buildFallback returns a user-facing fallback message for sentinel errors,
// or "" if the error is not a sentinel. Uses the FileSummary from the metadata
// pipeline (not from PatchService) for full status/path info.
func buildFallback(summary model.FileSummary, err error) string {
	if err == nil {
		return ""
	}

	path := summary.Path
	pathLine := fmt.Sprintf("  Path:   %s", path)
	if summary.OldPath != "" {
		pathLine = fmt.Sprintf("  Path:   %s -> %s", summary.OldPath, summary.Path)
	}
	statusLine := fmt.Sprintf("  Status: %s", summary.Status)

	switch {
	case errors.Is(err, model.ErrBinaryFile):
		return fmt.Sprintf("Binary file -- content not displayed\n\n%s\n%s", pathLine, statusLine)
	case errors.Is(err, model.ErrSubmodule):
		return fmt.Sprintf("Submodule change\n\n%s\n%s", pathLine, statusLine)
	case errors.Is(err, model.ErrOversized):
		var oe *diff.OversizedError
		if errors.As(err, &oe) {
			return fmt.Sprintf("Patch too large to display (%d lines, %d bytes).\nUse `git diff -- %s` to view.\n\n%s\n%s",
				oe.Lines, oe.Bytes, path, pathLine, statusLine)
		}
		return fmt.Sprintf("Patch too large to display.\nUse `git diff -- %s` to view.\n\n%s\n%s", path, pathLine, statusLine)
	default:
		return ""
	}
}

func (m Model) updatePatch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		m.State.FocusPane = model.PaneFiles
		if m.State.Layout != model.LayoutSplit {
			m.patchViewport = nil
			m.patchErr = ""
			m.patchFallback = ""
			m.searchIndex = nil
			m.State.SearchQuery = ""
			m.searchNotFound = ""
		}
	case "j", "down":
		if m.patchViewport != nil {
			m.patchViewport.ScrollDown()
		}
	case "k", "up":
		if m.patchViewport != nil {
			m.patchViewport.ScrollUp()
		}
	case "n":
		if m.patchViewport != nil {
			m.patchViewport.NextHunk()
		}
	case "N":
		m.executeSearch(search.SearchPrev)
	case "enter":
		m.executeSearch(search.SearchNext)
	case "/":
		m.State.FocusPane = model.PaneSearch
		m.searchInput = ""
		m.searchNotFound = ""
	case "p":
		if m.patchViewport != nil {
			m.patchViewport.PrevHunk()
		}
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
	}
	return m, nil
}

func (m *Model) executeSearch(dir search.SearchDirection) {
	if m.State.SearchQuery == "" || m.searchIndex == nil || m.patchViewport == nil {
		return
	}

	currentDiff := m.patchViewport.ViewportLineToDiffLine(m.patchViewport.ScrollOffset)
	onHeader := m.patchViewport.IsHunkHeader(m.patchViewport.ScrollOffset)
	var from int
	if dir == search.SearchNext {
		if onHeader {
			from = currentDiff
		} else {
			from = currentDiff + 1
		}
	} else {
		from = currentDiff - 1
	}

	m.searchFrom(from, dir)
}

func (m *Model) searchFrom(from int, dir search.SearchDirection) {
	if m.State.SearchQuery == "" || m.searchIndex == nil || m.patchViewport == nil {
		return
	}

	line, ok := m.searchIndex.Find(m.State.SearchQuery, from, dir)
	if !ok {
		m.searchNotFound = fmt.Sprintf("Pattern not found: %s", m.State.SearchQuery)
		m.patchViewport.SearchQuery = ""
		m.patchViewport.MatchLine = -1
		return
	}

	m.searchNotFound = ""
	vpLine := m.patchViewport.DiffLineToViewportLine(line)
	m.patchViewport.ScrollOffset = vpLine
	m.patchViewport.SyncCurrentHunk()
	m.patchViewport.MatchLine = vpLine
	m.patchViewport.SearchQuery = m.State.SearchQuery
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.State.FocusPane = model.PanePatch
		m.searchInput = ""
	case tea.KeyEnter:
		m.State.FocusPane = model.PanePatch
		if m.searchInput == "" {
			return m, nil
		}
		m.State.SearchQuery = m.searchInput
		m.searchInput = ""
		m.executeSearch(search.SearchNext)
	case tea.KeyBackspace:
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
	case tea.KeyRunes:
		m.searchInput += string(msg.Runes)
	}
	return m, nil
}

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?", "esc":
		m.showHelp = false
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}
	if m.tooSmall {
		return m.sizeErr + "\nPress q to quit."
	}

	var b strings.Builder

	if m.showHelp {
		b.WriteString(m.viewHelp())
	} else if m.State.FocusPane == model.PaneIdle {
		b.WriteString(m.viewIdle())
	} else if m.State.FocusPane == model.PaneDashboard {
		b.WriteString(m.viewDashboard())
	} else if m.State.FocusPane == model.PaneCommit {
		b.WriteString(m.viewCommit())
	} else if m.State.Layout == model.LayoutSplit && m.widthTierNow() >= terminal.WidthCompactSplit {
		b.WriteString(m.viewSplit())
	} else if m.State.FocusPane == model.PaneSearch || m.State.FocusPane == model.PanePatch {
		b.WriteString(m.viewPatch())
	} else {
		b.WriteString(m.viewFileList())
	}

	b.WriteString("\n")
	if m.State.FocusPane == model.PaneSearch {
		b.WriteString(m.viewSearchInput())
	} else {
		b.WriteString(m.viewStatusBar())
	}

	return b.String()
}

func (m Model) viewFileList() string {
	outerHeight := m.height - 1 // reserve status bar
	if outerHeight < 3 {
		outerHeight = 3
	}
	innerW, innerH := panes.ContentDimensions(m.width, outerHeight)
	content, _ := panes.RenderFileList(
		m.State.Files, m.State.SelectedFile, 0,
		innerW, innerH, true,
	)
	footer := fmt.Sprintf("%d files", len(m.State.Files))
	return panes.BorderedPane(content, "Files", footer, m.width, outerHeight, true, m.showFooter())
}

func (m Model) viewPatch() string {
	outerHeight := m.height - 1
	if outerHeight < 3 {
		outerHeight = 3
	}
	innerW, innerH := panes.ContentDimensions(m.width, outerHeight)
	content := m.renderPatch(innerW, innerH, m.width)
	scrollLine := m.patchScrollLine(innerH)
	return panes.BorderedPaneWithScroll(content, m.patchTitle(), m.patchFooter(), m.width, outerHeight, true, m.showFooter(), scrollLine)
}

// renderPatch renders the patch pane content at the given dimensions.
// outerWidth is the pane's outer width (including borders) for gutter decisions.
func (m Model) renderPatch(width, height, outerWidth int) string {
	if m.patchErr != "" {
		return fmt.Sprintf("Error loading patch: %s", m.patchErr)
	}
	if m.patchFallback != "" {
		return m.patchFallback
	}

	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		path := m.State.Files[m.State.SelectedFile].Path
		if ps, ok := m.State.Patches[path]; ok && ps.Status == model.LoadLoading {
			return "Loading..."
		}
	}

	if m.patchViewport == nil {
		return "No patch loaded."
	}
	m.patchViewport.Width = width
	m.patchViewport.Height = height
	m.patchViewport.GutterVisible = outerWidth >= 60
	return m.patchViewport.Render()
}

// viewCommit renders the commit generation pane.
func (m Model) viewCommit() string {
	cs := m.State.CommitState

	if cs.InFlight {
		return "Generating commit message..."
	}
	if cs.Executing {
		return "Committing..."
	}
	if cs.CommitSHA != "" {
		return fmt.Sprintf("Committed: %s\n\n  Esc  back to files", cs.CommitSHA)
	}
	if cs.CommitErr != nil {
		return fmt.Sprintf("Commit failed: %v\n\n  Enter  retry\n  r  regenerate\n  Esc  cancel", cs.CommitErr)
	}
	if cs.Err != nil {
		return fmt.Sprintf("Error: %v\n\n  r  regenerate\n  Esc  cancel", cs.Err)
	}
	if cs.GeneratedMessage != "" {
		return fmt.Sprintf("%s\n\n  Enter  commit\n  e  edit in $EDITOR\n  r  regenerate\n  Esc  cancel", cs.GeneratedMessage)
	}
	return "No commit message."
}

func (m Model) viewHelp() string {
	if m.State.WorktreeMode && !m.State.DashboardState.DrillDown {
		help := []string{
			"Dashboard Key Bindings",
			"",
			"  j/k     navigate worktree list",
			"  l/Enter drill into worktree diff",
			"  q       quit",
			"  ?       toggle help",
		}
		return strings.Join(help, "\n")
	}

	help := []string{
		"Key Bindings",
		"",
		"  j/k     navigate file list / scroll diff",
		"  l/Enter select file / focus patch pane",
	}
	if m.State.WorktreeMode {
		help = append(help, "  h/Esc   back to dashboard")
	} else {
		help = append(help, "  h/Esc   back to file list")
	}
	help = append(help,
		"  n/p     next/previous hunk",
		"  /       search in patch",
		"  r       refresh",
		"  W       toggle whitespace ignore",
		"  Tab     toggle split/modal layout",
		"  q       quit",
		"  ?       toggle help",
	)
	if m.State.WatchEnabled {
		watchLine := fmt.Sprintf("  [watch]  auto-refresh every %s", m.State.WatchInterval)
		if !m.State.LastRefreshAt.IsZero() {
			watchLine += fmt.Sprintf(", last refresh %s", m.State.LastRefreshAt.Format("15:04:05"))
		}
		help = append(help, watchLine)
	}
	if m.State.CommitEnabled {
		help = append(help, "  c       generate commit message")
	}
	return strings.Join(help, "\n")
}

// widthTierNow returns the current width tier computed from the model dimensions.
func (m Model) widthTierNow() terminal.WidthTier {
	wt, _ := terminal.LayoutTier(m.width, m.height)
	return wt
}

// showFooter reports whether pane footers should be visible based on the height tier.
func (m Model) showFooter() bool {
	_, ht := terminal.LayoutTier(m.width, m.height)
	return ht >= terminal.HeightFooterVisible
}

// patchScrollLine returns the inner row index for the scroll indicator,
// or -1 if no indicator should be shown.
func (m Model) patchScrollLine(innerHeight int) int {
	if m.patchViewport == nil || innerHeight <= 0 {
		return -1
	}
	total := m.patchViewport.TotalLines()
	if total <= m.patchViewport.Height {
		return -1 // content fits — no scrollbar needed
	}
	pos := m.patchViewport.ScrollIndicatorPos()
	line := int(pos * float64(innerHeight-1))
	if line >= innerHeight {
		line = innerHeight - 1
	}
	return line
}

// patchTitle returns the title for the patch pane (current filename).
func (m Model) patchTitle() string {
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		return m.State.Files[m.State.SelectedFile].Path
	}
	return "Patch"
}

// patchFooter returns the footer for the patch pane (hunk position + scroll %).
func (m Model) patchFooter() string {
	if m.patchViewport == nil || len(m.patchViewport.Patch.Hunks) == 0 {
		return ""
	}
	hunkInfo := fmt.Sprintf("Hunk %d/%d", m.patchViewport.CurrentHunk+1, len(m.patchViewport.Patch.Hunks))
	pct := 100
	total := m.patchViewport.TotalLines()
	if maxScroll := total - m.patchViewport.Height; maxScroll > 0 {
		pct = m.patchViewport.ScrollOffset * 100 / maxScroll
		if pct > 100 {
			pct = 100
		}
	}
	return fmt.Sprintf("%s · %d%%", hunkInfo, pct)
}

// syncFileListScroll adjusts fileListScroll so the selected file stays visible
// in split mode. Must be called from Update paths (not View) to persist state.
func (m *Model) syncFileListScroll() {
	if m.State.Layout != model.LayoutSplit || m.height == 0 {
		return
	}
	outerHeight := m.height - 1 // reserve status bar
	_, innerH := panes.ContentDimensions(m.width, outerHeight)
	m.fileListScroll = panes.EnsureVisible(
		m.State.SelectedFile, m.fileListScroll, innerH, len(m.State.Files),
	)
}

// toggleLayout switches between split and modal layout modes.
func (m Model) toggleLayout() (tea.Model, tea.Cmd) {
	if m.State.Layout == model.LayoutSplit {
		m.State.Layout = model.LayoutModal
		return m, nil
	}
	m.State.Layout = model.LayoutSplit
	m.syncFileListScroll()
	// Auto-load selected file's patch so the right pane isn't empty.
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		return m.selectFile()
	}
	return m, nil
}

// fileListWidth computes the file list pane width: max(25, min(termWidth*0.3, 50)).
func fileListWidth(termWidth int) int {
	w := termWidth * 3 / 10
	if w < 25 {
		w = 25
	}
	if w > 50 {
		w = 50
	}
	return w
}

// viewSplit renders the split-pane layout with bordered file list on left and bordered patch on right.
func (m Model) viewSplit() string {
	outerHeight := m.height - 1 // reserve status bar
	if outerHeight < 3 {
		return ""
	}

	flOuterWidth := fileListWidth(m.width)
	patchOuterWidth := m.width - flOuterWidth

	flInnerW, flInnerH := panes.ContentDimensions(flOuterWidth, outerHeight)
	patchInnerW, patchInnerH := panes.ContentDimensions(patchOuterWidth, outerHeight)

	filesActive := m.State.FocusPane == model.PaneFiles

	leftContent, _ := panes.RenderFileList(
		m.State.Files, m.State.SelectedFile, m.fileListScroll,
		flInnerW, flInnerH, filesActive,
	)
	rightContent := m.renderPatch(patchInnerW, patchInnerH, patchOuterWidth)

	showFoot := m.showFooter()
	fileFooter := fmt.Sprintf("%d files", len(m.State.Files))
	left := panes.BorderedPane(leftContent, "Files", fileFooter, flOuterWidth, outerHeight, filesActive, showFoot)
	scrollLine := m.patchScrollLine(patchInnerH)
	right := panes.BorderedPaneWithScroll(rightContent, m.patchTitle(), m.patchFooter(), patchOuterWidth, outerHeight, !filesActive, showFoot, scrollLine)

	// Join bordered panes side by side, line by line.
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

// truncateToWidth trims a string to fit within a terminal-cell width budget.
func truncateToWidth(s string, maxWidth int) string {
	w := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth {
			return s[:i]
		}
		w += rw
	}
	return s
}

// Styles — kept minimal; will degrade gracefully when color is unavailable.
var (
	statusBarStyle = lipgloss.NewStyle().
			Background(theme.StatusBg).
			Foreground(theme.StatusFg)
	searchNotFoundStyle = lipgloss.NewStyle().
				Background(theme.Error).
				Foreground(theme.BrightText)
)
