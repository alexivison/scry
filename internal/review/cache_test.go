package review

import (
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func samplePatch() model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{
			{OldStart: 1, OldLen: 3, NewStart: 1, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, Text: "package main"},
				}},
		},
	}
}

func TestCacheLookupMiss(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		state model.AppState
		path  string
	}{
		"empty cache": {
			state: model.AppState{
				Patches:         make(map[string]model.PatchLoadState),
				CacheGeneration: 1,
			},
			path: "main.go",
		},
		"different generation": {
			state: model.AppState{
				Patches: map[string]model.PatchLoadState{
					"main.go": {
						Status:     model.LoadLoaded,
						Patch:      ptrPatch(samplePatch()),
						Generation: 0,
					},
				},
				CacheGeneration: 1,
			},
			path: "main.go",
		},
		"loading status not a hit": {
			state: model.AppState{
				Patches: map[string]model.PatchLoadState{
					"main.go": {
						Status:     model.LoadLoading,
						Generation: 1,
					},
				},
				CacheGeneration: 1,
			},
			path: "main.go",
		},
		"failed status not a hit": {
			state: model.AppState{
				Patches: map[string]model.PatchLoadState{
					"main.go": {
						Status:     model.LoadFailed,
						Err:        model.ErrBinaryFile,
						Generation: 1,
					},
				},
				CacheGeneration: 1,
			},
			path: "main.go",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, hit := CacheLookup(tc.state, tc.path)
			if hit {
				t.Error("expected cache miss, got hit")
			}
		})
	}
}

func TestCacheLookupHit(t *testing.T) {
	t.Parallel()

	patch := samplePatch()
	state := model.AppState{
		Patches: map[string]model.PatchLoadState{
			"main.go": {
				Status:     model.LoadLoaded,
				Patch:      &patch,
				Generation: 1,
			},
		},
		CacheGeneration: 1,
	}

	got, hit := CacheLookup(state, "main.go")
	if !hit {
		t.Fatal("expected cache hit, got miss")
	}
	if got.Patch == nil {
		t.Fatal("cached patch should not be nil")
	}
	if got.Patch.Summary.Path != "main.go" {
		t.Errorf("cached path = %q, want %q", got.Patch.Summary.Path, "main.go")
	}
}

func TestCacheStore(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Patches:         make(map[string]model.PatchLoadState),
		CacheGeneration: 2,
	}
	patch := samplePatch()

	CacheStore(&state, "main.go", &patch, nil)

	got, ok := state.Patches["main.go"]
	if !ok {
		t.Fatal("expected entry in cache")
	}
	if got.Status != model.LoadLoaded {
		t.Errorf("Status = %q, want %q", got.Status, model.LoadLoaded)
	}
	if got.Generation != 2 {
		t.Errorf("Generation = %d, want 2", got.Generation)
	}
}

func TestCacheStoreContentHash(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Patches:         make(map[string]model.PatchLoadState),
		CacheGeneration: 1,
	}
	patch := samplePatch()

	CacheStore(&state, "main.go", &patch, nil)

	got := state.Patches["main.go"]
	if got.ContentHash == "" {
		t.Error("ContentHash should be populated after CacheStore")
	}

	// Different patch content → different hash.
	patch2 := model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{
			{OldStart: 1, OldLen: 3, NewStart: 1, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineAdded, Text: "different content"},
				}},
		},
	}
	CacheStore(&state, "other.go", &patch2, nil)
	got2 := state.Patches["other.go"]
	if got2.ContentHash == "" {
		t.Error("ContentHash should be populated for second store")
	}
	if got.ContentHash == got2.ContentHash {
		t.Error("different patch content should produce different hashes")
	}
}

func TestCacheStoreError(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Patches:         make(map[string]model.PatchLoadState),
		CacheGeneration: 1,
	}

	CacheStore(&state, "bad.go", nil, model.ErrBinaryFile)

	got, ok := state.Patches["bad.go"]
	if !ok {
		t.Fatal("expected entry in cache")
	}
	if got.Status != model.LoadFailed {
		t.Errorf("Status = %q, want %q", got.Status, model.LoadFailed)
	}
	if got.Err != model.ErrBinaryFile {
		t.Errorf("Err = %v, want ErrBinaryFile", got.Err)
	}
}

func TestMarkLoading(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Patches:         make(map[string]model.PatchLoadState),
		CacheGeneration: 3,
	}

	MarkLoading(&state, "file.go")

	got, ok := state.Patches["file.go"]
	if !ok {
		t.Fatal("expected entry in cache")
	}
	if got.Status != model.LoadLoading {
		t.Errorf("Status = %q, want %q", got.Status, model.LoadLoading)
	}
	if got.Generation != 3 {
		t.Errorf("Generation = %d, want 3", got.Generation)
	}
}

func TestIsStaleGeneration(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		msgGen   int
		stateGen int
		want     bool
	}{
		"same generation is not stale": {
			msgGen: 1, stateGen: 1, want: false,
		},
		"older generation is stale": {
			msgGen: 0, stateGen: 1, want: true,
		},
		"newer generation is stale": {
			msgGen: 2, stateGen: 1, want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := IsStaleGeneration(tc.msgGen, tc.stateGen)
			if got != tc.want {
				t.Errorf("IsStaleGeneration(%d, %d) = %v, want %v",
					tc.msgGen, tc.stateGen, got, tc.want)
			}
		})
	}
}

func TestBumpGeneration(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Patches:         make(map[string]model.PatchLoadState),
		CacheGeneration: 5,
	}

	BumpGeneration(&state)

	if state.CacheGeneration != 6 {
		t.Errorf("CacheGeneration = %d, want 6", state.CacheGeneration)
	}
}

func TestClearPatches(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		Patches: map[string]model.PatchLoadState{
			"a.go": {Status: model.LoadLoaded, Generation: 1},
			"b.go": {Status: model.LoadLoading, Generation: 1},
		},
		CacheGeneration: 1,
	}

	ClearPatches(&state)

	if len(state.Patches) != 0 {
		t.Errorf("Patches length = %d, want 0", len(state.Patches))
	}
	if state.Patches == nil {
		t.Error("Patches map should be empty, not nil")
	}
}

func ptrPatch(fp model.FilePatch) *model.FilePatch { return &fp }
