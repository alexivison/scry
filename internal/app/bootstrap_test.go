package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alexivison/scry/internal/config"
	"github.com/alexivison/scry/internal/model"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
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

func TestWorktreeCompareLoaderResolvesHeadDirty(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Scry Test")
	runGit(t, dir, "config", "user.email", "scry@example.com")
	if err := os.WriteFile(dir+"/main.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, dir, "add", "main.go")
	runGit(t, dir, "commit", "-m", "initial")
	if err := os.WriteFile(dir+"/main.go", []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmp, err := (&worktreeCompareLoaderImpl{}).LoadCompare(context.Background(), dir, model.CompareBasisHeadDirty)
	if err != nil {
		t.Fatalf("LoadCompare: %v", err)
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if cmp.Repo.WorktreeRoot != root {
		t.Fatalf("worktree root = %q, want %q", cmp.Repo.WorktreeRoot, root)
	}
	if !cmp.WorkingTree {
		t.Fatal("WorkingTree = false, want true")
	}
	if cmp.DiffRange != "HEAD" {
		t.Fatalf("diff range = %q, want %q", cmp.DiffRange, "HEAD")
	}
}
