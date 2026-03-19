package review

import (
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func TestUpdateFileChangeGen_PopulatedOnRefresh(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		CacheGeneration: 5,
		FileChangeGen:   map[string]int{},
		Files: []model.FileSummary{
			{Path: "a.go", Additions: 5, Status: model.StatusModified},
		},
	}

	// Simulate refresh: b.go is new, a.go changed (different additions).
	newFiles := []model.FileSummary{
		{Path: "a.go", Additions: 10, Status: model.StatusModified},
		{Path: "b.go", Additions: 3, Status: model.StatusAdded},
	}

	UpdateFileChangeGen(&state, state.Files, newFiles)

	if gen, ok := state.FileChangeGen["a.go"]; !ok || gen != 5 {
		t.Errorf("FileChangeGen[a.go] = %d (ok=%v), want 5", gen, ok)
	}
	if gen, ok := state.FileChangeGen["b.go"]; !ok || gen != 5 {
		t.Errorf("FileChangeGen[b.go] = %d (ok=%v), want 5", gen, ok)
	}
}

func TestUpdateFileChangeGen_UnchangedFileKeepsPreviousGen(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		CacheGeneration: 5,
		FileChangeGen:   map[string]int{"a.go": 3},
		Files: []model.FileSummary{
			{Path: "a.go", Additions: 5, Status: model.StatusModified},
		},
	}

	// Same summary — no change.
	newFiles := []model.FileSummary{
		{Path: "a.go", Additions: 5, Status: model.StatusModified},
	}

	UpdateFileChangeGen(&state, state.Files, newFiles)

	if gen := state.FileChangeGen["a.go"]; gen != 3 {
		t.Errorf("FileChangeGen[a.go] = %d, want 3 (unchanged)", gen)
	}
}

func TestUpdateFileChangeGen_RemovedFileCleaned(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		CacheGeneration: 5,
		FileChangeGen:   map[string]int{"gone.go": 2},
		Files: []model.FileSummary{
			{Path: "gone.go", Additions: 1, Status: model.StatusAdded},
		},
	}

	UpdateFileChangeGen(&state, state.Files, []model.FileSummary{})

	if _, ok := state.FileChangeGen["gone.go"]; ok {
		t.Error("FileChangeGen[gone.go] should be removed")
	}
}

func TestFreshnessTier_Decay(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		changeGen  int
		currentGen int
		wantTier   FreshnessTier
	}{
		"hot: same generation":         {changeGen: 5, currentGen: 5, wantTier: FreshnessHot},
		"warm: 1 generation ago":       {changeGen: 4, currentGen: 5, wantTier: FreshnessWarm},
		"warm: 2 generations ago":      {changeGen: 3, currentGen: 5, wantTier: FreshnessWarm},
		"cold: 3+ generations ago":     {changeGen: 2, currentGen: 5, wantTier: FreshnessCold},
		"cold: very old":               {changeGen: 0, currentGen: 10, wantTier: FreshnessCold},
		"hot: first generation":        {changeGen: 1, currentGen: 1, wantTier: FreshnessHot},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := ComputeFreshness(tc.changeGen, tc.currentGen)
			if got != tc.wantTier {
				t.Errorf("ComputeFreshness(%d, %d) = %v, want %v",
					tc.changeGen, tc.currentGen, got, tc.wantTier)
			}
		})
	}
}

func TestFileFreshness_IntegrationOverMultipleRefreshes(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		CacheGeneration: 1,
		FileChangeGen:   map[string]int{},
		Files:           []model.FileSummary{},
	}

	// Refresh 1: a.go appears.
	oldFiles := state.Files
	newFiles := []model.FileSummary{{Path: "a.go", Additions: 5, Status: model.StatusModified}}
	UpdateFileChangeGen(&state, oldFiles, newFiles)
	state.Files = newFiles

	if ComputeFreshness(state.FileChangeGen["a.go"], state.CacheGeneration) != FreshnessHot {
		t.Error("a.go should be hot after first refresh")
	}

	// Refresh 2: a.go unchanged.
	state.CacheGeneration = 2
	oldFiles = state.Files
	newFiles = []model.FileSummary{{Path: "a.go", Additions: 5, Status: model.StatusModified}}
	UpdateFileChangeGen(&state, oldFiles, newFiles)
	state.Files = newFiles

	if ComputeFreshness(state.FileChangeGen["a.go"], state.CacheGeneration) != FreshnessWarm {
		t.Error("a.go should be warm 1 generation later")
	}

	// Refresh 3: a.go still unchanged.
	state.CacheGeneration = 3
	oldFiles = state.Files
	newFiles = []model.FileSummary{{Path: "a.go", Additions: 5, Status: model.StatusModified}}
	UpdateFileChangeGen(&state, oldFiles, newFiles)
	state.Files = newFiles

	if ComputeFreshness(state.FileChangeGen["a.go"], state.CacheGeneration) != FreshnessWarm {
		t.Error("a.go should still be warm 2 generations later")
	}

	// Refresh 4: a.go unchanged, now cold.
	state.CacheGeneration = 4
	oldFiles = state.Files
	newFiles = []model.FileSummary{{Path: "a.go", Additions: 5, Status: model.StatusModified}}
	UpdateFileChangeGen(&state, oldFiles, newFiles)
	state.Files = newFiles

	if ComputeFreshness(state.FileChangeGen["a.go"], state.CacheGeneration) != FreshnessCold {
		t.Error("a.go should be cold 3 generations later")
	}
}
