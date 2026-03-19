package model

import "time"

// WorktreeInfo holds the display state for a single worktree in the dashboard.
type WorktreeInfo struct {
	Path           string    // absolute path
	Branch         string    // short branch name (e.g. "main", "feature")
	CommitHash     string    // short commit hash
	Subject        string    // first line of commit message
	Dirty          bool      // true if worktree has uncommitted changes
	Bare           bool      // true if bare worktree
	ChangedFiles   int       // number of changed files from git status
	LastActivityAt time.Time // updated when snapshot state changes (dirty, count, commit)
}

// PreviewEntry holds cached preview data for a worktree.
type PreviewEntry struct {
	Files []FileSummary
}

// PaneDashboard is the focus pane for worktree dashboard mode.
const PaneDashboard Pane = "dashboard"

// DashboardState holds the state for the worktree dashboard view.
type DashboardState struct {
	Worktrees       []WorktreeInfo
	SelectedIdx     int
	ScrollOffset    int
	DrillDown       bool // true when viewing a worktree's diff
	DrillGeneration int  // monotonic counter to discard stale drill-down results

	// Preview pane state.
	PreviewCache map[string]PreviewEntry // cache key (snap) → preview data
	PreviewFiles []FileSummary           // current selection's preview (for rendering)

	// Deletion confirmation state.
	ConfirmDelete bool   // true when awaiting deletion confirmation
	DeletePath    string // path of worktree to delete
	DeleteDirty   bool   // true if the worktree is dirty (requires force)
	DeleteErr     string // error message from failed deletion
	DeleteIsMain  bool   // true if user tried to delete main worktree
}
