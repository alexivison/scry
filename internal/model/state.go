package model

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
}
