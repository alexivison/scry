package model

import "time"

// Pane identifies a UI focus area.
type Pane string

const (
	PaneFiles  Pane = "files"
	PanePatch  Pane = "patch"
	PaneSearch Pane = "search"
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
	Status     LoadStatus
	Patch      *FilePatch
	Err        error
	Generation int
}

// CommitState holds the state of AI commit message generation.
type CommitState struct {
	GeneratedMessage string
	Provider         string
	InFlight         bool
	Err              error
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

	// Watch mode state (v0.2).
	WatchEnabled    bool
	WatchInterval   time.Duration
	LastFingerprint string
	RefreshInFlight bool
	LastRefreshAt   time.Time

	// Commit generation state (v0.2).
	CommitState CommitState
}
