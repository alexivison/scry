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
	if len(state.Patches) != 0 {
		t.Errorf("Patches length = %d, want 0", len(state.Patches))
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
