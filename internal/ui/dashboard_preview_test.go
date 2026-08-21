package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/panes"
)

type mockPreviewLoader struct {
	files []model.FileSummary
	err   error
	bases []model.CompareBasis
}

func (m *mockPreviewLoader) LoadPreview(_ context.Context, _ string, basis model.CompareBasis) (PreviewResult, error) {
	m.bases = append(m.bases, basis)
	return PreviewResult{Files: m.files}, m.err
}

func previewFiles() []model.FileSummary {
	return []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 10, Deletions: 5},
		{Path: "new.go", Status: model.StatusAdded, Additions: 30, Deletions: 0},
		{Path: "old.go", Status: model.StatusDeleted, Additions: 0, Deletions: 20},
	}
}

func previewTreeFiles() []model.FileSummary {
	return []model.FileSummary{
		{Path: "cmd/scry/main.go", Status: model.StatusModified, Additions: 10, Deletions: 5},
		{Path: "cmd/scry/run.go", Status: model.StatusAdded, Additions: 30},
		{Path: "internal/app/bootstrap.go", Status: model.StatusModified, Additions: 2, Deletions: 1},
		{Path: "internal/model/state.go", Status: model.StatusModified, Additions: 4},
		{Path: "internal/ui/dashboard.go", Status: model.StatusModified, Additions: 8, Deletions: 3},
		{Path: "internal/ui/panes/preview.go", Status: model.StatusModified, Additions: 6},
		{Path: "README.md", Status: model.StatusDeleted, Deletions: 12},
	}
}

func TestDashboardPreview_VisibleBeforeFilesLoad(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state, WithPreviewLoader(&mockPreviewLoader{files: previewFiles()}))
	m.width = 120
	m.height = 30

	output := ansi.Strip(m.View())

	if !strings.Contains(output, "Preview") {
		t.Fatalf("wide dashboard should render preview pane before files load, got:\n%s", output)
	}
	if !strings.Contains(output, "Loading preview") {
		t.Fatalf("empty pending preview should show loading placeholder, got:\n%s", output)
	}
}

func TestDashboardPreview_LoadsAfterWideWindowSize(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state, WithPreviewLoader(&mockPreviewLoader{files: previewFiles()}))

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("wide window size should trigger initial preview load")
	}
	m = deepDrain(t, m, cmd)
	if len(m.State.DashboardState.PreviewFiles) == 0 {
		t.Fatal("preview files should be populated after wide resize load")
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
	snap := WorktreeSnapshotKey(state.DashboardState.Worktrees[0], state.CompareBasis)
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

func TestDashboardPreview_SnapshotKeyIncludesHeadDirtyBasis(t *testing.T) {
	t.Parallel()

	wt := dashboardWorktrees()[0]
	upstream := WorktreeSnapshotKey(wt, model.CompareBasisUpstream)
	local := WorktreeSnapshotKey(wt, model.CompareBasisLocalTrunk)
	headDirty := WorktreeSnapshotKey(wt, model.CompareBasisHeadDirty)

	if headDirty == upstream || headDirty == local {
		t.Fatalf("HEAD/dirty snapshot key should be distinct, got upstream=%q local=%q headDirty=%q", upstream, local, headDirty)
	}
}

func TestDashboardPreview_SnapshotKeyIncludesBranch(t *testing.T) {
	wt := dashboardWorktrees()[0]
	otherBranch := wt
	otherBranch.Branch = "release"

	if WorktreeSnapshotKey(wt, model.CompareBasisUpstream) == WorktreeSnapshotKey(otherBranch, model.CompareBasisUpstream) {
		t.Fatal("snapshot key should change when the worktree branch changes")
	}
}

func TestDashboardPreview_BareWorktreeDoesNotLoad(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.Worktrees = []model.WorktreeInfo{
		{Path: "/repo.git", Bare: true},
		{Path: "/repo-main", Branch: "main"},
	}
	state.DashboardState.SelectedIdx = 0
	loader := &mockPreviewLoader{files: previewFiles()}
	m := NewModel(state, WithPreviewLoader(loader))
	m.width = 120
	m.height = 30

	cmd := m.maybeLoadPreview()
	if cmd != nil {
		t.Fatal("bare worktree should not start a preview command")
	}
	if len(loader.bases) != 0 {
		t.Fatalf("preview loader called for bare worktree, bases=%v", loader.bases)
	}

	output := ansi.Strip(m.View())
	if strings.Contains(output, "Loading preview") {
		t.Fatalf("bare worktree should not show loading preview, got:\n%s", output)
	}
	if strings.Contains(output, "+0 -0") {
		t.Fatalf("bare worktree should not show empty aggregate counts, got:\n%s", output)
	}
	if !strings.Contains(output, "Bare repository has no working tree.") {
		t.Fatalf("bare worktree should show a no-working-tree message, got:\n%s", output)
	}
}

func TestDashboardPreview_RenderInSplitView(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state)
	m.width = 120
	m.height = 30
	m.State.DashboardState.PreviewFiles = previewTreeFiles()

	output := ansi.Strip(m.View())

	if !strings.Contains(output, "├─ [-] cmd/") || !strings.Contains(output, "│ └─ [-] scry/") {
		t.Fatalf("preview should use tree-style directory rows, got:\n%s", output)
	}
	if !strings.Contains(output, "preview.go") {
		t.Fatalf("preview should show more than five files when height allows, got:\n%s", output)
	}
	if !strings.Contains(output, "+10 -5") {
		t.Fatalf("preview should keep per-file counts, got:\n%s", output)
	}
}

func TestDashboardPreview_HeaderShowsAggregateCounts(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	m := NewModel(state)
	m.width = 120
	m.height = 30
	m.State.DashboardState.PreviewFiles = previewTreeFiles()

	output := m.View()
	header := strings.Split(ansi.Strip(output), "\n")[0]
	counts := panes.RenderCounts(model.FileSummary{Additions: 60, Deletions: 21}, false)

	if !strings.Contains(header, "Preview") {
		t.Fatalf("preview header missing title, got:\n%s", output)
	}
	if !strings.HasSuffix(header, "+60 -21") {
		t.Fatalf("preview header should right-align aggregate counts, got header %q", header)
	}
	if !strings.Contains(output, counts) {
		t.Fatalf("preview header should preserve styled aggregate counts, got:\n%s", output)
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
	currentSnap := WorktreeSnapshotKey(wt, state.CompareBasis)

	// Simulate a stale PreviewLoadedMsg with an outdated snapshot key
	// (e.g., worktree state changed between request and response).
	staleSnap := currentSnap + "|stale"
	staleMsg := PreviewLoadedMsg{
		Path:   wt.Path,
		Snap:   staleSnap,
		Result: PreviewResult{Files: previewFiles()},
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
	snap := WorktreeSnapshotKey(wt, state.CompareBasis)
	msg := PreviewLoadedMsg{Path: wt.Path, Snap: snap, Result: PreviewResult{Files: previewFiles()}}
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
