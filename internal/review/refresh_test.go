package review

import (
	"testing"
	"time"

	"github.com/alexivison/scry/internal/model"
)

func TestPrepareRefresh(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		CacheGeneration: 3,
		Patches: map[string]model.PatchLoadState{
			"a.go": {Status: model.LoadLoaded, Generation: 3},
			"b.go": {Status: model.LoadLoading, Generation: 3},
		},
		RefreshInFlight: false,
	}

	PrepareRefresh(&state)

	if state.CacheGeneration != 4 {
		t.Errorf("CacheGeneration = %d, want 4", state.CacheGeneration)
	}
	// Patches are preserved for selective invalidation (no blanket clear).
	if len(state.Patches) != 2 {
		t.Errorf("Patches length = %d, want 2 (preserved for selective invalidation)", len(state.Patches))
	}
	if !state.RefreshInFlight {
		t.Error("RefreshInFlight should be true after PrepareRefresh")
	}
}

func TestCompleteRefresh(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		RefreshInFlight: true,
	}

	before := time.Now()
	CompleteRefresh(&state)
	after := time.Now()

	if state.RefreshInFlight {
		t.Error("RefreshInFlight should be false after CompleteRefresh")
	}
	if state.LastRefreshAt.Before(before) || state.LastRefreshAt.After(after) {
		t.Errorf("LastRefreshAt = %v, want between %v and %v", state.LastRefreshAt, before, after)
	}
}

func TestReconcileSelection_PathFound(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Files: []model.FileSummary{
			{Path: "a.go"},
			{Path: "b.go"},
			{Path: "c.go"},
		},
		SelectedFile: 0,
	}

	ReconcileSelection(&state, "b.go")

	if state.SelectedFile != 1 {
		t.Errorf("SelectedFile = %d, want 1", state.SelectedFile)
	}
}

func TestReconcileSelection_PathNotFound_ClampsIndex(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Files: []model.FileSummary{
			{Path: "a.go"},
			{Path: "c.go"},
		},
		SelectedFile: 5, // was beyond the new list
	}

	ReconcileSelection(&state, "removed.go")

	if state.SelectedFile != 1 {
		t.Errorf("SelectedFile = %d, want 1 (clamped to len-1)", state.SelectedFile)
	}
}

func TestReconcileSelection_EmptyFiles(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Files:        nil,
		SelectedFile: 2,
	}

	ReconcileSelection(&state, "old.go")

	if state.SelectedFile != -1 {
		t.Errorf("SelectedFile = %d, want -1", state.SelectedFile)
	}
}

func TestReconcileSelection_NegativeIndex_ClampsToZero(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Files: []model.FileSummary{
			{Path: "a.go"},
		},
		SelectedFile: -1,
	}

	ReconcileSelection(&state, "gone.go")

	if state.SelectedFile != 0 {
		t.Errorf("SelectedFile = %d, want 0", state.SelectedFile)
	}
}

func TestPrepareRefresh_PreservesOtherState(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		CacheGeneration:  1,
		Patches:          map[string]model.PatchLoadState{"x.go": {}},
		IgnoreWhitespace: true,
		Files:            []model.FileSummary{{Path: "x.go"}},
		SelectedFile:     0,
	}

	PrepareRefresh(&state)

	if !state.IgnoreWhitespace {
		t.Error("IgnoreWhitespace should be preserved")
	}
	if len(state.Files) != 1 {
		t.Error("Files should be preserved")
	}
	if state.SelectedFile != 0 {
		t.Error("SelectedFile should be preserved")
	}
}

func TestSelectiveInvalidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		gen      int
		patches  map[string]model.PatchLoadState
		oldFiles []model.FileSummary
		newFiles []model.FileSummary
		wantHit  map[string]bool // path → should CacheLookup hit at gen?
		wantGone []string        // paths that should be evicted entirely
	}{
		"unchanged file preserved": {
			gen: 2,
			patches: map[string]model.PatchLoadState{
				"a.go": {Status: model.LoadLoaded, Generation: 1, Patch: ptrPatch(samplePatch())},
			},
			oldFiles: []model.FileSummary{{Path: "a.go", Additions: 5, Deletions: 0, Status: model.StatusModified}},
			newFiles: []model.FileSummary{{Path: "a.go", Additions: 5, Deletions: 0, Status: model.StatusModified}},
			wantHit:  map[string]bool{"a.go": true},
		},
		"changed file evicted": {
			gen: 2,
			patches: map[string]model.PatchLoadState{
				"a.go": {Status: model.LoadLoaded, Generation: 1, Patch: ptrPatch(samplePatch())},
			},
			oldFiles: []model.FileSummary{{Path: "a.go", Additions: 5, Deletions: 0, Status: model.StatusModified}},
			newFiles: []model.FileSummary{{Path: "a.go", Additions: 10, Deletions: 3, Status: model.StatusModified}},
			wantHit:  map[string]bool{"a.go": false},
			wantGone: []string{"a.go"},
		},
		"removed file evicted": {
			gen: 2,
			patches: map[string]model.PatchLoadState{
				"gone.go": {Status: model.LoadLoaded, Generation: 1, Patch: ptrPatch(samplePatch())},
			},
			oldFiles: []model.FileSummary{{Path: "gone.go", Additions: 1, Deletions: 0, Status: model.StatusAdded}},
			newFiles: []model.FileSummary{},
			wantGone: []string{"gone.go"},
		},
		"new file no cache entry": {
			gen:      2,
			patches:  map[string]model.PatchLoadState{},
			oldFiles: []model.FileSummary{},
			newFiles: []model.FileSummary{{Path: "new.go", Additions: 8, Status: model.StatusAdded}},
			wantHit:  map[string]bool{"new.go": false},
		},
		"mixed scenario": {
			gen: 2,
			patches: map[string]model.PatchLoadState{
				"unchanged.go": {Status: model.LoadLoaded, Generation: 1, Patch: ptrPatch(samplePatch())},
				"changed.go":   {Status: model.LoadLoaded, Generation: 1, Patch: ptrPatch(samplePatch())},
				"removed.go":   {Status: model.LoadLoaded, Generation: 1, Patch: ptrPatch(samplePatch())},
			},
			oldFiles: []model.FileSummary{
				{Path: "unchanged.go", Additions: 5, Deletions: 0, Status: model.StatusModified},
				{Path: "changed.go", Additions: 3, Deletions: 2, Status: model.StatusModified},
				{Path: "removed.go", Additions: 1, Deletions: 0, Status: model.StatusAdded},
			},
			newFiles: []model.FileSummary{
				{Path: "unchanged.go", Additions: 5, Deletions: 0, Status: model.StatusModified},
				{Path: "changed.go", Additions: 10, Deletions: 5, Status: model.StatusModified},
				{Path: "new.go", Additions: 8, Deletions: 0, Status: model.StatusAdded},
			},
			wantHit:  map[string]bool{"unchanged.go": true, "changed.go": false, "new.go": false},
			wantGone: []string{"removed.go", "changed.go"},
		},
		"status change evicts": {
			gen: 2,
			patches: map[string]model.PatchLoadState{
				"a.go": {Status: model.LoadLoaded, Generation: 1, Patch: ptrPatch(samplePatch())},
			},
			oldFiles: []model.FileSummary{{Path: "a.go", Additions: 5, Deletions: 0, Status: model.StatusModified}},
			newFiles: []model.FileSummary{{Path: "a.go", Additions: 5, Deletions: 0, Status: model.StatusAdded}},
			wantHit:  map[string]bool{"a.go": false},
			wantGone: []string{"a.go"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			state := model.AppState{
				CacheGeneration: tc.gen,
				Patches:         tc.patches,
			}
			SelectiveInvalidate(&state, tc.oldFiles, tc.newFiles)

			for path, wantHit := range tc.wantHit {
				_, got := CacheLookup(state, path)
				if got != wantHit {
					t.Errorf("CacheLookup(%q) = %v, want %v", path, got, wantHit)
				}
			}
			for _, path := range tc.wantGone {
				if _, exists := state.Patches[path]; exists {
					t.Errorf("Patches[%q] should be evicted", path)
				}
			}
		})
	}
}
