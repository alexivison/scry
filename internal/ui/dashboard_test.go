package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/watch"
)

func dashboardWorktrees() []model.WorktreeInfo {
	return []model.WorktreeInfo{
		{Path: "/home/user/project", Branch: "main", CommitHash: "abc1234", Subject: "initial", Dirty: false},
		{Path: "/home/user/project-feat", Branch: "feature", CommitHash: "def5678", Subject: "add feat", Dirty: true},
		{Path: "/home/user/project-fix", Branch: "bugfix", CommitHash: "ghi9012", Subject: "fix bug", Dirty: false},
	}
}

func dashboardState() model.AppState {
	return model.AppState{
		FocusPane:      model.PaneDashboard,
		WorktreeMode:   true,
		DashboardState: model.DashboardState{
			Worktrees:   dashboardWorktrees(),
			SelectedIdx: 0,
		},
		Patches: make(map[string]model.PatchLoadState),
	}
}

func TestDashboardNavigateDown(t *testing.T) {
	t.Parallel()

	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	// j moves selection down
	updated, _ := m.Update(keyMsg('j'))
	um := updated.(Model)
	if um.State.DashboardState.SelectedIdx != 1 {
		t.Errorf("SelectedIdx = %d, want 1", um.State.DashboardState.SelectedIdx)
	}
}

func TestDashboardNavigateUp(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.SelectedIdx = 2
	m := NewModel(state)
	m.width = 80
	m.height = 24

	// k moves selection up
	updated, _ := m.Update(keyMsg('k'))
	um := updated.(Model)
	if um.State.DashboardState.SelectedIdx != 1 {
		t.Errorf("SelectedIdx = %d, want 1", um.State.DashboardState.SelectedIdx)
	}
}

func TestDashboardNavigateDownAtBottom(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.SelectedIdx = 2 // last item
	m := NewModel(state)
	m.width = 80
	m.height = 24

	updated, _ := m.Update(keyMsg('j'))
	um := updated.(Model)
	if um.State.DashboardState.SelectedIdx != 2 {
		t.Errorf("SelectedIdx = %d, want 2 (should not exceed last)", um.State.DashboardState.SelectedIdx)
	}
}

func TestDashboardNavigateUpAtTop(t *testing.T) {
	t.Parallel()

	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	updated, _ := m.Update(keyMsg('k'))
	um := updated.(Model)
	if um.State.DashboardState.SelectedIdx != 0 {
		t.Errorf("SelectedIdx = %d, want 0 (should not go below 0)", um.State.DashboardState.SelectedIdx)
	}
}

type mockDrillDownProvider struct {
	result DrillDownResult
	err    error
}

func (m *mockDrillDownProvider) LoadDrillDown(_ context.Context, _ string) (DrillDownResult, error) {
	return m.result, m.err
}

func TestDashboardDrillDown(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.SelectedIdx = 1
	provider := &mockDrillDownProvider{
		result: DrillDownResult{
			Compare: model.ResolvedCompare{BaseRef: "abc", WorkingTree: true, DiffRange: "abc"},
			Files: []model.FileSummary{
				{Path: "main.go", Status: model.StatusModified, Additions: 5, Deletions: 2},
			},
		},
	}
	m := NewModel(state, WithDrillDownProvider(provider))
	m.width = 80
	m.height = 24

	// l fires async drill-down loading.
	updated, cmd := m.Update(keyMsg('l'))
	um := updated.(Model)
	if !um.State.DashboardState.DrillDown {
		t.Error("DrillDown = false, want true after 'l'")
	}
	if cmd == nil {
		t.Fatal("expected async drill-down command, got nil")
	}

	// Execute the async command.
	msg := cmd()
	ddMsg, ok := msg.(DrillDownLoadedMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want DrillDownLoadedMsg", msg)
	}
	if ddMsg.Err != nil {
		t.Fatalf("drill-down error: %v", ddMsg.Err)
	}
	if len(ddMsg.Result.Files) != 1 {
		t.Errorf("files len = %d, want 1", len(ddMsg.Result.Files))
	}
}

func TestDashboardDrillDownEnter(t *testing.T) {
	t.Parallel()

	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	updated, _ := m.Update(enterMsg())
	um := updated.(Model)
	if !um.State.DashboardState.DrillDown {
		t.Error("DrillDown = false, want true after Enter")
	}
}

func TestDashboardDrillDownLoadsFiles(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.DrillDown = true
	state.FocusPane = model.PaneFiles
	m := NewModel(state)
	m.width = 80
	m.height = 24

	// Simulate receiving drill-down loaded message.
	files := []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 10, Deletions: 3},
		{Path: "util.go", Status: model.StatusAdded, Additions: 20, Deletions: 0},
	}
	result, _ := m.Update(DrillDownLoadedMsg{
		Result: DrillDownResult{
			Compare: model.ResolvedCompare{BaseRef: "abc", WorkingTree: true, DiffRange: "abc"},
			Files:   files,
		},
	})
	um := result.(Model)

	if len(um.State.Files) != 2 {
		t.Errorf("Files len = %d, want 2", len(um.State.Files))
	}
	if um.State.SelectedFile != 0 {
		t.Errorf("SelectedFile = %d, want 0", um.State.SelectedFile)
	}
	if !um.State.Compare.WorkingTree {
		t.Error("Compare.WorkingTree = false, want true")
	}
}

func TestDashboardDrillDownStaleGeneration(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.DrillDown = true
	state.DashboardState.DrillGeneration = 2 // current generation
	state.FocusPane = model.PaneFiles
	state.Files = []model.FileSummary{{Path: "existing.go", Status: model.StatusModified}}
	state.SelectedFile = 0
	m := NewModel(state)
	m.width = 80
	m.height = 24

	// Deliver a stale result with generation 1 (older than current 2).
	staleFiles := []model.FileSummary{
		{Path: "stale.go", Status: model.StatusAdded},
		{Path: "old.go", Status: model.StatusDeleted},
	}
	result, _ := m.Update(DrillDownLoadedMsg{
		Result: DrillDownResult{
			Compare: model.ResolvedCompare{BaseRef: "xyz", WorkingTree: true, DiffRange: "xyz"},
			Files:   staleFiles,
		},
		Generation: 1, // stale
	})
	um := result.(Model)

	// State should be unchanged — stale result discarded.
	if len(um.State.Files) != 1 {
		t.Errorf("Files len = %d, want 1 (stale result should be discarded)", len(um.State.Files))
	}
	if um.State.Files[0].Path != "existing.go" {
		t.Errorf("Files[0].Path = %q, want 'existing.go'", um.State.Files[0].Path)
	}
}

func TestDashboardDrillDownError(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.DrillDown = true
	state.FocusPane = model.PaneFiles
	m := NewModel(state)
	m.width = 80
	m.height = 24

	// Simulate drill-down error.
	result, _ := m.Update(DrillDownLoadedMsg{
		Err: fmt.Errorf("no upstream"),
	})
	um := result.(Model)

	// Should return to dashboard on error.
	if um.State.DashboardState.DrillDown {
		t.Error("DrillDown = true, want false after error")
	}
	if um.State.FocusPane != model.PaneDashboard {
		t.Errorf("FocusPane = %q, want %q", um.State.FocusPane, model.PaneDashboard)
	}
}

func TestDashboardRefreshAtTopLevel(t *testing.T) {
	t.Parallel()

	// r at top-level dashboard should be a no-op (not call startRefresh).
	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	result, cmd := m.Update(keyMsg('r'))
	um := result.(Model)

	// Should not set RefreshInFlight (startRefresh not called).
	if um.State.RefreshInFlight {
		t.Error("RefreshInFlight = true, want false — r at dashboard should be no-op")
	}
	if cmd != nil {
		t.Error("expected nil command from r at top-level dashboard")
	}
}

func TestDashboardRefreshInDrillDown(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.DrillDown = true
	state.DashboardState.SelectedIdx = 1
	state.FocusPane = model.PaneFiles
	provider := &mockDrillDownProvider{
		result: DrillDownResult{
			Compare: model.ResolvedCompare{BaseRef: "abc", WorkingTree: true, DiffRange: "abc"},
			Files:   []model.FileSummary{{Path: "main.go", Status: model.StatusModified}},
		},
	}
	m := NewModel(state, WithDrillDownProvider(provider))
	m.width = 80
	m.height = 24

	// r in drill-down should re-trigger drill-down load, not startRefresh.
	_, cmd := m.Update(keyMsg('r'))
	if cmd == nil {
		t.Fatal("expected async drill-down command from 'r' in drill-down, got nil")
	}

	// Execute and verify it returns DrillDownLoadedMsg.
	msg := cmd()
	if _, ok := msg.(DrillDownLoadedMsg); !ok {
		t.Errorf("cmd returned %T, want DrillDownLoadedMsg", msg)
	}
}

func TestDashboardReturnFromDrillDown(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		key tea.Msg
	}{
		"esc": {key: tea.KeyMsg{Type: tea.KeyEscape}},
		"h":   {key: keyMsg('h')},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := dashboardState()
			state.DashboardState.DrillDown = true
			state.DashboardState.SelectedIdx = 1
			state.FocusPane = model.PaneFiles
			m := NewModel(state)
			m.width = 80
			m.height = 24

			updated, _ := m.Update(tc.key)
			um := updated.(Model)
			if um.State.DashboardState.DrillDown {
				t.Errorf("DrillDown = true, want false after %s", name)
			}
			if um.State.FocusPane != model.PaneDashboard {
				t.Errorf("FocusPane = %q, want %q", um.State.FocusPane, model.PaneDashboard)
			}
			if um.State.DashboardState.SelectedIdx != 1 {
				t.Errorf("SelectedIdx = %d, want 1 (preserved)", um.State.DashboardState.SelectedIdx)
			}
		})
	}
}

func TestDashboardReturnFromPatchView(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.DashboardState.DrillDown = true
	state.DashboardState.SelectedIdx = 1
	state.FocusPane = model.PanePatch // in drill-down, viewing a patch
	m := NewModel(state)
	m.width = 80
	m.height = 24

	// First Esc from patch view should return to file list (still in drill-down).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	um := updated.(Model)
	if !um.State.DashboardState.DrillDown {
		t.Error("DrillDown = false, want true after first Esc (back to file list)")
	}
	if um.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q after first Esc", um.State.FocusPane, model.PaneFiles)
	}

	// Second Esc from file list should return to dashboard.
	updated2, _ := um.Update(tea.KeyMsg{Type: tea.KeyEscape})
	um2 := updated2.(Model)
	if um2.State.DashboardState.DrillDown {
		t.Error("DrillDown = true, want false after second Esc")
	}
	if um2.State.FocusPane != model.PaneDashboard {
		t.Errorf("FocusPane = %q, want %q after second Esc", um2.State.FocusPane, model.PaneDashboard)
	}
	if um2.State.DashboardState.SelectedIdx != 1 {
		t.Errorf("SelectedIdx = %d, want 1 (preserved)", um2.State.DashboardState.SelectedIdx)
	}
}

func TestDashboardQuit(t *testing.T) {
	t.Parallel()

	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	_, cmd := m.Update(keyMsg('q'))
	if cmd == nil {
		t.Fatal("expected quit command, got nil")
	}
	// Execute the command and check it produces a quit message
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd returned %T, want tea.QuitMsg", msg)
	}
}

func TestDashboardHelp(t *testing.T) {
	t.Parallel()

	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	updated, _ := m.Update(keyMsg('?'))
	um := updated.(Model)
	if !um.showHelp {
		t.Error("showHelp = false, want true after '?'")
	}
}

func TestDashboardViewRenders(t *testing.T) {
	t.Parallel()

	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	// Should contain branch names from worktrees
	if !strings.Contains(view, "main") {
		t.Error("View missing 'main' branch")
	}
	if !strings.Contains(view, "feature") {
		t.Error("View missing 'feature' branch")
	}
}

func TestDashboardStatusBar(t *testing.T) {
	t.Parallel()

	m := NewModel(dashboardState())
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "Worktree") {
		t.Error("status bar missing 'Worktree'")
	}
	if !strings.Contains(view, "3 worktrees") {
		t.Error("status bar missing worktree count")
	}
}

// --- Auto-refresh tests ---

type mockWorktreeLoader struct {
	worktrees []model.WorktreeInfo
	err       error
}

func (m *mockWorktreeLoader) LoadWorktrees(_ context.Context) ([]model.WorktreeInfo, error) {
	return m.worktrees, m.err
}

func TestDashboardInitStartsTick(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	loader := &mockWorktreeLoader{worktrees: dashboardWorktrees()}
	m := NewModel(state, WithWorktreeLoader(loader))

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil, want tick command")
	}
}

func TestDashboardInitNoTickWithoutWatch(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.WatchEnabled = false
	m := NewModel(state)

	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() returned command, want nil when watch disabled")
	}
}

func TestDashboardTickTriggersRefresh(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	updated := []model.WorktreeInfo{
		{Path: "/home/user/project", Branch: "main", CommitHash: "new1234", Subject: "updated", Dirty: true},
	}
	loader := &mockWorktreeLoader{worktrees: updated}
	m := NewModel(state, WithWorktreeLoader(loader))
	m.width = 80
	m.height = 24

	// Send a tick — should fire async refresh.
	result, cmd := m.Update(watch.TickMsg{At: time.Now()})
	um := result.(Model)
	if !um.State.RefreshInFlight {
		t.Error("RefreshInFlight = false, want true after tick")
	}
	if cmd == nil {
		t.Fatal("expected async command after tick, got nil")
	}

	// Execute the async command to get the refresh result.
	msg := cmd()
	refreshMsg, ok := msg.(WorktreeRefreshedMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want WorktreeRefreshedMsg", msg)
	}
	if len(refreshMsg.Worktrees) != 1 {
		t.Errorf("worktrees len = %d, want 1", len(refreshMsg.Worktrees))
	}
}

func TestDashboardRefreshUpdatesState(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	state.RefreshInFlight = true
	m := NewModel(state)
	m.width = 80
	m.height = 24

	updated := []model.WorktreeInfo{
		{Path: "/home/user/project", Branch: "main", CommitHash: "new1234", Subject: "updated", Dirty: true},
		{Path: "/home/user/project-new", Branch: "new-branch", CommitHash: "xyz9999", Subject: "new work", Dirty: false},
	}

	result, cmd := m.Update(WorktreeRefreshedMsg{Worktrees: updated})
	um := result.(Model)

	if um.State.RefreshInFlight {
		t.Error("RefreshInFlight = true, want false after refresh completes")
	}
	if len(um.State.DashboardState.Worktrees) != 2 {
		t.Errorf("worktrees len = %d, want 2", len(um.State.DashboardState.Worktrees))
	}
	// Should schedule next tick.
	if cmd == nil {
		t.Error("expected next tick command, got nil")
	}
}

func TestDashboardRefreshPreservesSelection(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	state.DashboardState.SelectedIdx = 1 // "feature" selected
	state.RefreshInFlight = true
	m := NewModel(state)
	m.width = 80
	m.height = 24

	// Same worktrees, different order.
	updated := []model.WorktreeInfo{
		{Path: "/home/user/project-fix", Branch: "bugfix", CommitHash: "ghi9012", Subject: "fix bug", Dirty: false},
		{Path: "/home/user/project", Branch: "main", CommitHash: "abc1234", Subject: "initial", Dirty: false},
		{Path: "/home/user/project-feat", Branch: "feature", CommitHash: "def5678", Subject: "add feat", Dirty: false},
	}

	result, _ := m.Update(WorktreeRefreshedMsg{Worktrees: updated})
	um := result.(Model)

	// Selection should follow "feature" branch to index 2.
	if um.State.DashboardState.SelectedIdx != 2 {
		t.Errorf("SelectedIdx = %d, want 2 (followed 'feature' branch)", um.State.DashboardState.SelectedIdx)
	}
}

func TestDashboardTickSkipsWhenRefreshInFlight(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	state.RefreshInFlight = true
	loader := &mockWorktreeLoader{worktrees: dashboardWorktrees()}
	m := NewModel(state, WithWorktreeLoader(loader))
	m.width = 80
	m.height = 24

	result, cmd := m.Update(watch.TickMsg{At: time.Now()})
	um := result.(Model)

	// Should still be in flight (skipped).
	if !um.State.RefreshInFlight {
		t.Error("RefreshInFlight should stay true when refresh already in flight")
	}
	// Should schedule next tick anyway.
	if cmd == nil {
		t.Error("expected next tick command even when skipping, got nil")
	}
}

func TestDashboardTickSkipsDuringDrillDown(t *testing.T) {
	t.Parallel()

	state := dashboardState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	state.DashboardState.DrillDown = true
	state.FocusPane = model.PaneFiles
	loader := &mockWorktreeLoader{worktrees: dashboardWorktrees()}
	m := NewModel(state, WithWorktreeLoader(loader))
	m.width = 80
	m.height = 24

	result, cmd := m.Update(watch.TickMsg{At: time.Now()})
	um := result.(Model)

	// Should NOT set RefreshInFlight during drill-down.
	if um.State.RefreshInFlight {
		t.Error("RefreshInFlight = true during drill-down, want false (tick should be skipped)")
	}
	// Should still schedule next tick so refresh resumes when returning to dashboard.
	if cmd == nil {
		t.Error("expected next tick command during drill-down, got nil")
	}
}
