package review

import "github.com/alexivison/scry/internal/model"

// ToggleFlag flips the flagged state of a file path.
func ToggleFlag(state *model.AppState, path string) {
	if state.FlaggedFiles == nil {
		state.FlaggedFiles = make(map[string]bool)
	}
	if state.FlaggedFiles[path] {
		delete(state.FlaggedFiles, path)
	} else {
		state.FlaggedFiles[path] = true
	}
}

// PruneFlags removes flags for files no longer in the list.
func PruneFlags(state *model.AppState, files []model.FileSummary) {
	if len(state.FlaggedFiles) == 0 {
		return
	}
	paths := make(map[string]struct{}, len(files))
	for _, f := range files {
		paths[f.Path] = struct{}{}
	}
	for path := range state.FlaggedFiles {
		if _, ok := paths[path]; !ok {
			delete(state.FlaggedFiles, path)
		}
	}
}

// NextFlaggedFile returns the index of the next flagged file after fromIdx,
// wrapping around. Returns (0, false) if no flagged files exist.
func NextFlaggedFile(files []model.FileSummary, flagged map[string]bool, fromIdx int) (int, bool) {
	n := len(files)
	if n == 0 || len(flagged) == 0 {
		return 0, false
	}
	for i := 1; i <= n; i++ {
		idx := (fromIdx + i) % n
		if flagged[files[idx].Path] {
			return idx, true
		}
	}
	return 0, false
}
