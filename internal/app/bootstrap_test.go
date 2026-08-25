package app

import (
	"path/filepath"
	"testing"

	"github.com/alexivison/scry/internal/config"
	"github.com/alexivison/scry/internal/model"
)

func TestNoteStoreForWorktree(t *testing.T) {
	worktree := t.TempDir()
	store, err := noteStoreForWorktree(worktree, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if store.Worktree() != want {
		t.Fatalf("worktree = %q, want %q", store.Worktree(), want)
	}
}

func TestInitialDiffStateDefaultsToSplitLayout(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		IgnoreWhitespace: true,
		Commit:           true,
		CommitAuto:       true,
		GroupByDirectory: true,
	}
	cmp := model.ResolvedCompare{Basis: model.CompareBasisUpstream}
	files := []model.FileSummary{{Path: "main.go"}}

	state := initialDiffState(cfg, cmp, model.CompareBasisUpstream, files)

	if state.Layout != model.LayoutSplit {
		t.Fatalf("Layout = %q, want %q", state.Layout, model.LayoutSplit)
	}
	if state.FocusPane != model.PaneFiles {
		t.Fatalf("FocusPane = %q, want %q", state.FocusPane, model.PaneFiles)
	}
	if state.PatchDiffMode != model.PatchDiffModeSideBySide {
		t.Fatalf("PatchDiffMode = %v, want %v", state.PatchDiffMode, model.PatchDiffModeSideBySide)
	}
	if state.Patches == nil {
		t.Fatal("Patches map is nil")
	}
}

func TestInitialDashboardStateDefaultsToSplitLayout(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		GroupByDirectory: true,
	}

	state := initialDashboardState(cfg)

	if state.Layout != model.LayoutSplit {
		t.Fatalf("Layout = %q, want %q", state.Layout, model.LayoutSplit)
	}
	if state.FocusPane != model.PaneDashboard {
		t.Fatalf("FocusPane = %q, want %q", state.FocusPane, model.PaneDashboard)
	}
	if !state.WorktreeMode {
		t.Fatal("WorktreeMode = false, want true")
	}
	if state.PatchDiffMode != model.PatchDiffModeSideBySide {
		t.Fatalf("PatchDiffMode = %v, want %v", state.PatchDiffMode, model.PatchDiffModeSideBySide)
	}
	if state.Patches == nil {
		t.Fatal("Patches map is nil")
	}
}
