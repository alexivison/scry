package review

import (
	"time"

	"github.com/alexivison/scry/internal/model"
)

// PrepareRefresh performs the synchronous start of a refresh cycle:
// generation bump and refresh-in-flight state update.
// Patches are preserved for selective invalidation after metadata arrives.
func PrepareRefresh(state *model.AppState) {
	BumpGeneration(state)
	state.RefreshInFlight = true
}

// SelectiveInvalidate compares old and new file lists and preserves cached
// patches for unchanged files by promoting them to the current generation.
// Changed and removed files are evicted or left at the old generation.
func SelectiveInvalidate(state *model.AppState, oldFiles, newFiles []model.FileSummary) {
	type sig struct {
		Additions int
		Deletions int
		Status    model.FileStatus
	}

	old := make(map[string]sig, len(oldFiles))
	for _, f := range oldFiles {
		old[f.Path] = sig{f.Additions, f.Deletions, f.Status}
	}

	newPaths := make(map[string]struct{}, len(newFiles))
	for _, f := range newFiles {
		newPaths[f.Path] = struct{}{}

		prev, existed := old[f.Path]
		if !existed {
			continue // new file — no cache entry to preserve
		}
		if prev.Additions == f.Additions && prev.Deletions == f.Deletions && prev.Status == f.Status {
			// Unchanged — promote cache entry to current generation.
			if ps, ok := state.Patches[f.Path]; ok && ps.Status == model.LoadLoaded {
				ps.Generation = state.CacheGeneration
				state.Patches[f.Path] = ps
			}
			continue
		}
		// Changed: evict so stale patches can't be re-promoted on future refreshes.
		delete(state.Patches, f.Path)
	}

	// Evict removed files.
	for path := range state.Patches {
		if _, ok := newPaths[path]; !ok {
			delete(state.Patches, path)
		}
	}
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

