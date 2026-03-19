package ui

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

// countingPatchLoader tracks how many times LoadPatch is called.
type countingPatchLoader struct {
	patches map[string]model.FilePatch
	calls   atomic.Int32
	err     error
}

func (c *countingPatchLoader) LoadPatch(_ context.Context, _ model.ResolvedCompare, filePath string, _ model.FileStatus, _ bool) (model.FilePatch, error) {
	c.calls.Add(1)
	if c.err != nil {
		return model.FilePatch{}, c.err
	}
	if fp, ok := c.patches[filePath]; ok {
		return fp, nil
	}
	return model.FilePatch{}, nil
}

// --- Async loading tests ---

func TestLazyLoadEnterReturnsCmd(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{
		patches: map[string]model.FilePatch{
			"main.go": samplePatch(),
		},
	}
	m := NewModel(sampleState(), WithPatchLoader(loader))
	m.width = 100
	m.height = 30

	_, cmd := m.Update(enterMsg())
	if cmd == nil {
		t.Fatal("Enter should return a non-nil Cmd for async loading")
	}
	// The loader should NOT have been called synchronously during Update
	if loader.calls.Load() != 0 {
		t.Errorf("LoadPatch called %d times during Update, want 0 (async only)", loader.calls.Load())
	}
}

func TestLazyLoadEnterSetsLoadingState(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{
		patches: map[string]model.FilePatch{
			"main.go": samplePatch(),
		},
	}
	m := NewModel(sampleState(), WithPatchLoader(loader))
	m.width = 100
	m.height = 30

	updated, _ := m.Update(enterMsg())
	um := updated.(Model)

	ps, ok := um.State.Patches["main.go"]
	if !ok {
		t.Fatal("Patches should have an entry for main.go after Enter")
	}
	if ps.Status != model.LoadLoading {
		t.Errorf("Status = %q, want %q", ps.Status, model.LoadLoading)
	}
}

func TestLazyLoadPatchLoadedMsgUpdatesCacheAndViewport(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{}
	state := sampleState()
	state.CacheGeneration = 1
	m := NewModel(state, WithPatchLoader(loader))
	m.width = 100
	m.height = 30
	// Simulate: user pressed Enter, now in patch pane with Loading state.
	m.State.FocusPane = model.PanePatch
	m.State.Patches["main.go"] = model.PatchLoadState{
		Status:     model.LoadLoading,
		Generation: 1,
	}

	patch := samplePatch()
	msg := PatchLoadedMsg{
		Path:  "main.go",
		Patch: patch,
		Gen:   1,
	}

	updated, _ := m.Update(msg)
	um := updated.(Model)

	// Cache should be updated to Loaded.
	ps := um.State.Patches["main.go"]
	if ps.Status != model.LoadLoaded {
		t.Errorf("Status = %q, want %q", ps.Status, model.LoadLoaded)
	}
	if ps.Patch == nil {
		t.Fatal("cached Patch should not be nil")
	}

	// Viewport should be created.
	if um.patchViewport == nil {
		t.Error("patchViewport should be set after PatchLoadedMsg")
	}
}

func TestLazyLoadStaleGenerationDiscarded(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{}
	state := sampleState()
	state.CacheGeneration = 2
	m := NewModel(state, WithPatchLoader(loader))
	m.width = 100
	m.height = 30
	m.State.FocusPane = model.PanePatch
	m.State.Patches["main.go"] = model.PatchLoadState{
		Status:     model.LoadLoading,
		Generation: 2,
	}

	// Stale message from generation 1.
	msg := PatchLoadedMsg{
		Path:  "main.go",
		Patch: samplePatch(),
		Gen:   1,
	}

	updated, _ := m.Update(msg)
	um := updated.(Model)

	// Cache should still be Loading (stale msg discarded).
	ps := um.State.Patches["main.go"]
	if ps.Status != model.LoadLoading {
		t.Errorf("Status = %q, want %q (stale msg should be discarded)", ps.Status, model.LoadLoading)
	}
	if um.patchViewport != nil {
		t.Error("patchViewport should be nil (stale msg)")
	}
}

func TestLazyLoadCacheHitSkipsRefetch(t *testing.T) {
	t.Parallel()

	patch := samplePatch()
	loader := &countingPatchLoader{
		patches: map[string]model.FilePatch{
			"main.go": patch,
		},
	}
	state := sampleState()
	state.CacheGeneration = 1
	// Pre-populate cache with a loaded patch.
	state.Patches["main.go"] = model.PatchLoadState{
		Status:     model.LoadLoaded,
		Patch:      &patch,
		Generation: 1,
	}
	m := NewModel(state, WithPatchLoader(loader))
	m.width = 100
	m.height = 30

	updated, cmd := m.Update(enterMsg())
	um := updated.(Model)

	// Should use cache — no async Cmd needed.
	if cmd != nil {
		t.Error("cache hit should not return a Cmd")
	}
	if loader.calls.Load() != 0 {
		t.Errorf("LoadPatch called %d times, want 0 (cache hit)", loader.calls.Load())
	}
	// Should have viewport from cache.
	if um.patchViewport == nil {
		t.Error("patchViewport should be set from cache hit")
	}
	if um.State.FocusPane != model.PanePatch {
		t.Errorf("FocusPane = %q, want %q", um.State.FocusPane, model.PanePatch)
	}
}

func TestLazyLoadLoadingIndicatorShown(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{}
	state := sampleState()
	state.CacheGeneration = 1
	m := NewModel(state, WithPatchLoader(loader))
	m.width = 100
	m.height = 30
	m.State.FocusPane = model.PanePatch
	m.State.Patches["main.go"] = model.PatchLoadState{
		Status:     model.LoadLoading,
		Generation: 1,
	}

	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("View() should show loading indicator, got:\n%s", view)
	}
}

func TestLazyLoadPatchLoadedMsgWithError(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{}
	state := sampleState()
	state.CacheGeneration = 1
	m := NewModel(state, WithPatchLoader(loader))
	m.width = 100
	m.height = 30
	m.State.FocusPane = model.PanePatch
	m.State.Patches["main.go"] = model.PatchLoadState{
		Status:     model.LoadLoading,
		Generation: 1,
	}

	msg := PatchLoadedMsg{
		Path: "main.go",
		Gen:  1,
		Err:  model.ErrBinaryFile,
	}

	updated, _ := m.Update(msg)
	um := updated.(Model)

	ps := um.State.Patches["main.go"]
	if ps.Status != model.LoadFailed {
		t.Errorf("Status = %q, want %q", ps.Status, model.LoadFailed)
	}
	// Sentinel errors (ErrBinaryFile) produce a fallback message, not patchErr.
	if um.patchFallback == "" {
		t.Error("patchFallback should be set for sentinel error")
	}
	if um.patchErr != "" {
		t.Errorf("patchErr should be empty for sentinel error, got %q", um.patchErr)
	}
}

func TestLazyLoadConcurrentSelectionDiscardsPrevious(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{
		patches: map[string]model.FilePatch{
			"main.go": samplePatch(),
			"new.go":  samplePatch(),
		},
	}
	state := sampleState()
	state.CacheGeneration = 1
	m := NewModel(state, WithPatchLoader(loader))
	m.width = 100
	m.height = 30

	// Select first file.
	updated, _ := m.Update(enterMsg())
	um := updated.(Model)

	// Navigate back and select second file.
	updated2, _ := um.Update(escMsg())
	um2 := updated2.(Model)
	updated3, _ := um2.Update(keyMsg('j'))
	um3 := updated3.(Model)
	updated4, _ := um3.Update(enterMsg())
	um4 := updated4.(Model)

	// Viewport should be nil — "new.go" is loading, no result yet.
	if um4.patchViewport != nil {
		t.Fatal("patchViewport should be nil while new.go is loading")
	}

	// A response arrives for main.go (not the currently selected file).
	// Same generation, so it updates the cache, but should NOT set the viewport
	// because the selected file is "new.go", not "main.go".
	mainMsg := PatchLoadedMsg{
		Path:  "main.go",
		Patch: samplePatch(),
		Gen:   1,
	}
	updated5, _ := um4.Update(mainMsg)
	um5 := updated5.(Model)

	if um5.State.SelectedFile != 1 {
		t.Errorf("SelectedFile = %d, want 1", um5.State.SelectedFile)
	}
	// Cache should have main.go's result stored.
	if ps, ok := um5.State.Patches["main.go"]; !ok || ps.Status != model.LoadLoaded {
		t.Error("main.go should be cached as Loaded")
	}
	// Viewport must NOT be set — the response was for a non-selected file.
	if um5.patchViewport != nil {
		t.Error("patchViewport should remain nil when PatchLoadedMsg is for a non-selected file")
	}
}

func TestLazyLoadRetryAfterFailure(t *testing.T) {
	t.Parallel()

	loader := &countingPatchLoader{
		patches: map[string]model.FilePatch{
			"main.go": samplePatch(),
		},
	}
	state := sampleState()
	state.CacheGeneration = 1
	// Pre-populate cache with a failed entry for main.go.
	state.Patches["main.go"] = model.PatchLoadState{
		Status:     model.LoadFailed,
		Err:        model.ErrOversized,
		Generation: 1,
	}
	m := NewModel(state, WithPatchLoader(loader))
	m.width = 100
	m.height = 30

	// Press Enter on main.go — should trigger a new async load (not use cached failure).
	_, cmd := m.Update(enterMsg())
	if cmd == nil {
		t.Fatal("Enter on a failed file should return a Cmd to retry the load")
	}
	if loader.calls.Load() != 0 {
		t.Errorf("LoadPatch should not be called synchronously, got %d calls", loader.calls.Load())
	}

	// Execute the Cmd to simulate async completion (may be batched with spinner tick).
	msgs := execAndCollect(cmd)
	loadedMsg, ok := findMsg[PatchLoadedMsg](msgs)
	if !ok {
		t.Fatal("expected PatchLoadedMsg in batch")
	}
	if loadedMsg.Err != nil {
		t.Fatalf("retry should succeed, got err: %v", loadedMsg.Err)
	}
	if loader.calls.Load() != 1 {
		t.Errorf("LoadPatch should be called once on retry, got %d", loader.calls.Load())
	}
}
