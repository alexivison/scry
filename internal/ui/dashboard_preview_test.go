package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

type mockPreviewLoader struct {
	files []model.FileSummary
	err   error
}

func (m *mockPreviewLoader) LoadPreview(_ context.Context, _ string) ([]model.FileSummary, error) {
	return m.files, m.err
}

func previewFiles() []model.FileSummary {
	return []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 10, Deletions: 5},
		{Path: "new.go", Status: model.StatusAdded, Additions: 30, Deletions: 0},
		{Path: "old.go", Status: model.StatusDeleted, Additions: 0, Deletions: 20},
	}
}

func TestDashboardPreview_LazyLoadOnSelectionChange(t *testing.T) {
	t.Parallel()

	loader := &mockPreviewLoader{files: previewFiles()}
	state := dashboardState()
	state.DashboardState.SelectedIdx = 0

	m := NewModel(state, WithWorktreeLoader(&mockWorktreeLoader{worktrees: state.DashboardState.Worktrees}), WithPreviewLoader(loader))
	m.width = 120
	m.height = 30

	// Move to second worktree — should trigger preview load.
	updated, cmd := m.Update(keyMsg('j'))
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("j should trigger preview load cmd")
	}

	// Execute and feed result.
	m = deepDrain(t, m, cmd)

	if m.State.DashboardState.PreviewFiles == nil {
		t.Error("PreviewFiles should be populated after load")
	}
}

func TestDashboardPreview_CacheHitOnReselect(t *testing.T) {
	t.Parallel()

	loader := &mockPreviewLoader{files: previewFiles()}
	state := dashboardState()
	m := NewModel(state, WithWorktreeLoader(&mockWorktreeLoader{worktrees: state.DashboardState.Worktrees}), WithPreviewLoader(loader))
	m.width = 120
	m.height = 30

	// Pre-populate cache for worktree 0.
	snap := WorktreeSnapshotKey(state.DashboardState.Worktrees[0])
	m.State.DashboardState.PreviewCache = map[string]model.PreviewEntry{
		snap: {Files: previewFiles()},
	}
	m.State.DashboardState.PreviewFiles = previewFiles()

	// Move away and back — should use cache, not trigger a new load.
	updated, _ := m.Update(keyMsg('j'))
	m = updated.(Model)
	updated, cmd := m.Update(keyMsg('k'))
	m = updated.(Model)

	// Cache hit — cmd may be nil or just a spinner tick (no preview load).
	if cmd != nil {
		msgs := execAndCollect(cmd)
		for _, msg := range msgs {
			if _, ok := msg.(PreviewLoadedMsg); ok {
				t.Error("expected cache hit, not a new preview load")
			}
		}
	}
}

func TestDashboardPreview_RenderInSplitView(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state)
	m.width = 120
	m.height = 30
	m.State.DashboardState.PreviewFiles = previewFiles()

	output := m.View()

	// Preview should show file names and +/- counts.
	if !strings.Contains(output, "main.go") {
		t.Error("preview should show main.go")
	}
}

// Bug #8b: handlePreviewLoaded should check snapshot key, not just path.
func TestDashboardPreview_StaleSnapshotDiscarded(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state, WithPreviewLoader(&mockPreviewLoader{files: previewFiles()}))
	m.width = 120
	m.height = 30

	// The worktree's current snapshot.
	wt := state.DashboardState.Worktrees[0]
	currentSnap := WorktreeSnapshotKey(wt)

	// Simulate a stale PreviewLoadedMsg with an outdated snapshot key
	// (e.g., worktree state changed between request and response).
	staleSnap := currentSnap + "|stale"
	staleMsg := PreviewLoadedMsg{
		Path:  wt.Path,
		Snap:  staleSnap,
		Files: previewFiles(),
	}

	updated, _ := m.handlePreviewLoaded(staleMsg)
	um := updated.(Model)

	// Stale snapshot should NOT be applied to the current view.
	if um.State.DashboardState.PreviewFiles != nil {
		t.Error("stale snapshot preview should not be applied to current view")
	}
}

// Bug #10: PreviewCache should evict entries when exceeding max size.
func TestDashboardPreview_CacheEviction(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state, WithPreviewLoader(&mockPreviewLoader{files: previewFiles()}))
	m.width = 120
	m.height = 30

	// Fill cache beyond the max cap (50).
	cache := make(map[string]model.PreviewEntry)
	for i := 0; i < 60; i++ {
		key := fmt.Sprintf("evict-test-%d", i)
		cache[key] = model.PreviewEntry{Files: previewFiles()}
	}
	m.State.DashboardState.PreviewCache = cache

	// Add one more entry via handlePreviewLoaded.
	wt := state.DashboardState.Worktrees[0]
	snap := WorktreeSnapshotKey(wt)
	msg := PreviewLoadedMsg{Path: wt.Path, Snap: snap, Files: previewFiles()}
	updated, _ := m.handlePreviewLoaded(msg)
	um := updated.(Model)

	if len(um.State.DashboardState.PreviewCache) > maxPreviewCacheSize {
		t.Errorf("PreviewCache size = %d, want <= %d", len(um.State.DashboardState.PreviewCache), maxPreviewCacheSize)
	}
	// Verify the new entry survived eviction.
	if _, ok := um.State.DashboardState.PreviewCache[snap]; !ok {
		t.Error("new entry should be present in cache after eviction")
	}
}

func TestDashboardPreview_HiddenNarrowWidth(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state)
	m.width = 60 // narrow — below 100 threshold
	m.height = 30
	m.State.DashboardState.PreviewFiles = previewFiles()

	output := m.View()

	// At narrow width (<100), the preview pane title should not appear.
	if strings.Contains(output, "Preview") {
		t.Error("preview pane should be hidden at narrow width, but 'Preview' title found")
	}
}
