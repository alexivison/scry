package review

import "github.com/alexivison/scry/internal/model"

// FreshnessTier indicates how recently a file changed.
type FreshnessTier int

const (
	FreshnessCold FreshnessTier = iota // no marker
	FreshnessWarm                      // 1–2 generations ago
	FreshnessHot                       // changed this refresh
)

// ComputeFreshness returns the freshness tier for a file given its last change
// generation and the current cache generation.
func ComputeFreshness(changeGen, currentGen int) FreshnessTier {
	age := currentGen - changeGen
	switch {
	case age <= 0:
		return FreshnessHot
	case age <= 2:
		return FreshnessWarm
	default:
		return FreshnessCold
	}
}

// UpdateFileChangeGen updates the FileChangeGen map during refresh reconciliation.
// Files whose summary changed (or are new) get their generation set to currentGen.
// Removed files are cleaned from the map.
func UpdateFileChangeGen(state *model.AppState, oldFiles, newFiles []model.FileSummary) {
	if state.FileChangeGen == nil {
		state.FileChangeGen = make(map[string]int)
	}

	old := make(map[string]model.FileSummary, len(oldFiles))
	for _, f := range oldFiles {
		old[f.Path] = f
	}

	newPaths := make(map[string]struct{}, len(newFiles))
	for _, f := range newFiles {
		newPaths[f.Path] = struct{}{}

		prev, existed := old[f.Path]
		if !existed {
			// New file.
			state.FileChangeGen[f.Path] = state.CacheGeneration
			continue
		}
		if prev != f {
			// Any summary field changed.
			state.FileChangeGen[f.Path] = state.CacheGeneration
		}
		// Unchanged: keep previous generation.
	}

	// Clean removed files.
	for path := range state.FileChangeGen {
		if _, ok := newPaths[path]; !ok {
			delete(state.FileChangeGen, path)
		}
	}
}

// NextChangedFile returns the index of the next hot or warm file after fromIdx,
// wrapping around. Returns (0, false) if no hot/warm files exist.
func NextChangedFile(files []model.FileSummary, changeGen map[string]int, currentGen, fromIdx int) (int, bool) {
	n := len(files)
	if n == 0 {
		return 0, false
	}
	for i := 1; i <= n; i++ {
		idx := (fromIdx + i) % n
		if tier := fileTier(files[idx].Path, changeGen, currentGen); tier >= FreshnessWarm {
			return idx, true
		}
	}
	return 0, false
}

// PrevChangedFile returns the index of the previous hot or warm file before fromIdx,
// wrapping around. Returns (0, false) if no hot/warm files exist.
func PrevChangedFile(files []model.FileSummary, changeGen map[string]int, currentGen, fromIdx int) (int, bool) {
	n := len(files)
	if n == 0 {
		return 0, false
	}
	for i := 1; i <= n; i++ {
		idx := (fromIdx - i + n) % n
		if tier := fileTier(files[idx].Path, changeGen, currentGen); tier >= FreshnessWarm {
			return idx, true
		}
	}
	return 0, false
}

func fileTier(path string, changeGen map[string]int, currentGen int) FreshnessTier {
	gen, ok := changeGen[path]
	if !ok {
		return FreshnessCold
	}
	return ComputeFreshness(gen, currentGen)
}
