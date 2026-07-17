package model

// Pane identifies a UI focus area.
type Pane string

const (
	PaneFiles  Pane = "files"
	PanePatch  Pane = "patch"
	PaneSearch Pane = "search"
	PaneCommit Pane = "commit"
)

// LayoutMode controls the overall pane arrangement.
type LayoutMode string

const (
	LayoutModal LayoutMode = "modal"
	LayoutSplit LayoutMode = "split"
)

// LineMode controls how long patch body lines are laid out.
type LineMode int

const (
	LineModeWrap LineMode = iota
	LineModeScroll
)

// PatchDiffMode controls whether a patch is rendered as unified or side-by-side.
type PatchDiffMode int

const (
	PatchDiffModeUnified PatchDiffMode = iota
	PatchDiffModeSideBySide
)

// LoadStatus tracks the lifecycle of an async patch load.
type LoadStatus string

const (
	LoadIdle    LoadStatus = "idle"
	LoadLoading LoadStatus = "loading"
	LoadLoaded  LoadStatus = "loaded"
	LoadFailed  LoadStatus = "failed"
)

// FileFilter controls which changed files are visible in the file tree.
type FileFilter int

const (
	FileFilterAll FileFilter = iota
	FileFilterTests
	FileFilterNonTests
)

// Next returns the next file filter in the in-app cycle.
func (f FileFilter) Next() FileFilter {
	switch f {
	case FileFilterAll:
		return FileFilterNonTests
	case FileFilterNonTests:
		return FileFilterTests
	default:
		return FileFilterAll
	}
}

// Label returns the footer label for the file filter.
func (f FileFilter) Label() string {
	switch f {
	case FileFilterTests:
		return "Tests"
	case FileFilterNonTests:
		return "Non-tests"
	default:
		return "All"
	}
}

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
	Compare           ResolvedCompare
	CompareBasis      CompareBasis
	Files             []FileSummary
	SelectedFile      int // Index into Files. -1 when the tree cursor is not on a file.
	FileFilter        FileFilter
	FileTreeCollapsed map[string]bool // directory path -> collapsed
	FileTreeCursor    int             // Index into visible file tree rows.
	Patches           map[string]PatchLoadState
	CacheGeneration   int
	IgnoreWhitespace  bool
	SearchQuery       string
	FocusPane         Pane
	Layout            LayoutMode
	PatchLineMode     LineMode
	PatchDiffMode     PatchDiffMode
	ShowLineNumbers   bool

	// Refresh state.
	RefreshInFlight bool

	// Commit generation state (v0.2).
	CommitEnabled bool
	CommitAuto    bool
	CommitState   CommitState

	// Freshness tracking (v0.3).
	GroupByDirectory bool           // config-driven directory grouping in file list
	FileChangeGen    map[string]int // path → CacheGeneration when file last changed

	// Worktree dashboard mode (v0.2).
	WorktreeMode   bool
	DashboardState DashboardState

	// Discard confirmation state.
	ConfirmDiscard   bool
	DiscardPath      string
	DiscardUntracked bool
	DiscardInFlight  bool
	DiscardErr       string
}
