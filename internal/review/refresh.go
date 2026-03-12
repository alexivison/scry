package review

import (
	"time"

	"github.com/alexivison/scry/internal/model"
)

// PrepareRefresh performs the synchronous start of a refresh cycle:
// generation bump, cache clear, and refresh-in-flight state update.
func PrepareRefresh(state *model.AppState) {
	BumpGeneration(state)
	ClearPatches(state)
	state.RefreshInFlight = true
}

// CompleteRefresh marks the refresh as finished and records the completion time.
func CompleteRefresh(state *model.AppState) {
	state.RefreshInFlight = false
	state.LastRefreshAt = time.Now()
}

// ReconcileSelection restores the selected file index after a file list update.
// Strategy: path-first match, then nearest-index clamp.
func ReconcileSelection(state *model.AppState, previousPath string) {
	if len(state.Files) == 0 {
		state.SelectedFile = -1
		return
	}

	if previousPath != "" {
		for i, f := range state.Files {
			if f.Path == previousPath {
				state.SelectedFile = i
				return
			}
		}
	}

	if state.SelectedFile >= len(state.Files) {
		state.SelectedFile = len(state.Files) - 1
	}
	if state.SelectedFile < 0 {
		state.SelectedFile = 0
	}
}

