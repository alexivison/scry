package ui

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

type mockWorktreeCompareLoader struct {
	compare model.ResolvedCompare
	err     error
	path    string
	basis   model.CompareBasis
}

func (m *mockWorktreeCompareLoader) LoadCompare(_ context.Context, path string, basis model.CompareBasis) (model.ResolvedCompare, error) {
	m.path = path
	m.basis = basis
	return m.compare, m.err
}

func TestBuildHunkCommandUsesOnlyTheResolvedRange(t *testing.T) {
	cmd := buildHunkCommand("/repo/worktree", "base...head")

	if cmd.Dir != "/repo/worktree" {
		t.Fatalf("Dir = %q, want %q", cmd.Dir, "/repo/worktree")
	}
	if got, want := cmd.Args, []string{"hunk", "diff", "base...head"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("Args = %q, want %q", got, want)
	}
}

func TestKeyHOpensCurrentCompareInHunk(t *testing.T) {
	previousBuilder := buildHunkCommand
	defer func() { buildHunkCommand = previousBuilder }()

	var gotRoot, gotRange string
	buildHunkCommand = func(root, diffRange string) *exec.Cmd {
		gotRoot = root
		gotRange = diffRange
		return exec.Command("sh", "-c", "exit 0")
	}

	m := NewModel(sampleState())
	m.width = 100
	m.height = 30
	m.State.Compare.Repo.WorktreeRoot = "/repo/current"

	updated, cmd := m.Update(keyMsg('H'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("H should launch Hunk")
	}

	for _, msg := range execAndCollect(cmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}

	if gotRoot != "/repo/current" {
		t.Fatalf("worktree root = %q, want %q", gotRoot, "/repo/current")
	}
	if gotRange != "abc123...def456" {
		t.Fatalf("diff range = %q, want %q", gotRange, "abc123...def456")
	}
}

func TestKeyHFromDashboardUsesSelectedWorktreePreview(t *testing.T) {
	previousBuilder := buildHunkCommand
	defer func() { buildHunkCommand = previousBuilder }()

	var gotRoot, gotRange string
	buildHunkCommand = func(root, diffRange string) *exec.Cmd {
		gotRoot = root
		gotRange = diffRange
		return exec.Command("sh", "-c", "exit 0")
	}

	state := dashboardState()
	state.DashboardState.SelectedIdx = 1
	selected := state.DashboardState.Worktrees[1]
	snapshot := WorktreeSnapshotKey(selected, state.CompareBasis)
	state.DashboardState.PreviewCache = map[string]model.PreviewEntry{
		snapshot: {
			Compare: model.ResolvedCompare{
				Repo:      model.RepoContext{WorktreeRoot: selected.Path},
				DiffRange: "merge-base",
			},
		},
	}
	state.DashboardState.PreviewSnap = snapshot

	m := NewModel(state)
	m.width = 100
	m.height = 30

	updated, cmd := m.Update(keyMsg('H'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("H should launch Hunk for the selected worktree")
	}

	for _, msg := range execAndCollect(cmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}

	if gotRoot != selected.Path {
		t.Fatalf("worktree root = %q, want %q", gotRoot, selected.Path)
	}
	if gotRange != "merge-base" {
		t.Fatalf("diff range = %q, want %q", gotRange, "merge-base")
	}
}

func TestKeyHFromNarrowDashboardResolvesSelectedWorktree(t *testing.T) {
	previousBuilder := buildHunkCommand
	defer func() { buildHunkCommand = previousBuilder }()

	var gotRoot, gotRange string
	buildHunkCommand = func(root, diffRange string) *exec.Cmd {
		gotRoot = root
		gotRange = diffRange
		return exec.Command("sh", "-c", "exit 0")
	}

	state := dashboardState()
	state.DashboardState.SelectedIdx = 1
	selected := state.DashboardState.Worktrees[1]
	loader := &mockWorktreeCompareLoader{
		compare: model.ResolvedCompare{
			Repo:      model.RepoContext{WorktreeRoot: selected.Path},
			DiffRange: "merge-base",
		},
	}
	m := NewModel(state, WithWorktreeCompareLoader(loader))
	m.width = 80
	m.height = 24

	updated, cmd := m.Update(keyMsg('H'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("H should resolve the selected worktree without a preview")
	}
	m = deepDrain(t, m, cmd)

	if loader.path != selected.Path {
		t.Fatalf("loader path = %q, want %q", loader.path, selected.Path)
	}
	if loader.basis != model.CompareBasisUpstream {
		t.Fatalf("loader basis = %q, want %q", loader.basis, model.CompareBasisUpstream)
	}
	if gotRoot != selected.Path {
		t.Fatalf("worktree root = %q, want %q", gotRoot, selected.Path)
	}
	if gotRange != "merge-base" {
		t.Fatalf("diff range = %q, want %q", gotRange, "merge-base")
	}
}

func TestStaleWorktreeCompareErrorIsIgnored(t *testing.T) {
	state := dashboardState()
	state.DashboardState.SelectedIdx = 1
	m := NewModel(state)
	m.width = 80
	m.height = 24

	selected := state.DashboardState.Worktrees[1]
	staleSnap := WorktreeSnapshotKey(selected, state.CompareBasis) + "|stale"
	updated, _ := m.Update(WorktreeCompareLoadedMsg{Snap: staleSnap, Err: errors.New("no upstream")})
	m = updated.(Model)

	if m.refreshErr != "" {
		t.Fatalf("refresh error = %q, want stale error ignored", m.refreshErr)
	}
}

func TestWorktreeCompareErrorRendersInStatusBar(t *testing.T) {
	state := dashboardState()
	state.DashboardState.SelectedIdx = 1
	m := NewModel(state)
	m.width = 80
	m.height = 24

	selected := state.DashboardState.Worktrees[1]
	snapshot := WorktreeSnapshotKey(selected, state.CompareBasis)
	updated, _ := m.Update(WorktreeCompareLoadedMsg{Snap: snapshot, Err: errors.New("no upstream")})
	m = updated.(Model)

	if !strings.Contains(m.viewStatusBar(), "Hunk unavailable: no upstream") {
		t.Fatalf("status bar = %q, want Hunk resolver error", m.viewStatusBar())
	}
}

func TestHunkProcessErrorRendersInStatusBar(t *testing.T) {
	m := NewModel(sampleState())
	m.width = 80
	m.height = 24

	updated, _ := m.Update(hunkClosedMsg{err: errors.New("executable not found")})
	m = updated.(Model)

	if !strings.Contains(m.viewStatusBar(), "hunk failed: executable not found") {
		t.Fatalf("status bar = %q, want Hunk process error", m.viewStatusBar())
	}
}
