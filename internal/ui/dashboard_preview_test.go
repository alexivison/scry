package ui

import (
	"context"
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
