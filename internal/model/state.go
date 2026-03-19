package model

import "time"

// Pane identifies a UI focus area.
type Pane string

const (
	PaneFiles  Pane = "files"
	PanePatch  Pane = "patch"
	PaneSearch Pane = "search"
	PaneCommit Pane = "commit"
	PaneIdle   Pane = "idle"
)

// LayoutMode controls the overall pane arrangement.
type LayoutMode string

const (
	LayoutModal LayoutMode = "modal"
	LayoutSplit LayoutMode = "split"
)

// LoadStatus tracks the lifecycle of an async patch load.
type LoadStatus string

const (
	LoadIdle    LoadStatus = "idle"
	LoadLoading LoadStatus = "loading"
	LoadLoaded  LoadStatus = "loaded"
	LoadFailed  LoadStatus = "failed"
)

// PatchLoadState holds the result of loading a single file's patch.
type PatchLoadState struct {
	Status      LoadStatus
	Patch       *FilePatch
	Err         error
	Generation  int
	ContentHash string // SHA-256 of patch content for scroll preservation
}

// CommitState holds the state of AI commit message generation and execution.
type CommitState struct {
	GeneratedMessage string
	Provider         string
	InFlight         bool
	Err              error
	Generation       int // monotonic counter to discard stale async results

	// Execution state (V2-T8).
	Executing bool
	CommitSHA string
	CommitErr error
}

// AppState is the top-level UI state threaded through the Bubble Tea model.
type AppState struct {
	Compare          ResolvedCompare
	Files            []FileSummary
	SelectedFile     int // Index into Files. -1 when Files is empty.
	Patches          map[string]PatchLoadState
	CacheGeneration  int
	IgnoreWhitespace bool
	SearchQuery      string
	FocusPane        Pane
	Layout           LayoutMode

	// Watch mode state (v0.2).
	WatchEnabled    bool
	WatchInterval   time.Duration
	LastFingerprint string
	RefreshInFlight bool
	LastRefreshAt   time.Time

	// Commit generation state (v0.2).
	CommitEnabled bool
	CommitAuto    bool
	CommitState   CommitState

	// Freshness tracking (v0.3).
	FileChangeGen map[string]int // path → CacheGeneration when file last changed

	// Worktree dashboard mode (v0.2).
	WorktreeMode   bool
	DashboardState DashboardState
}
