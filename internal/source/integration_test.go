package source_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/diff"
	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/source"
)

var fixtureDir string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "scry-source-fixtures-*")
	if err != nil {
		panic(err)
	}
	fixtureDir = tmp

	setupScript, err := filepath.Abs(filepath.Join("..", "..", "testdata", "repos", "setup.sh"))
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("bash", setupScript, fixtureDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("fixture setup failed: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(fixtureDir)
	os.Exit(code)
}

func TestLinkedWorktreeBootstrap(t *testing.T) {
	t.Parallel()

	wtDir := filepath.Join(fixtureDir, "linked-worktree", "wt")
	mainDir := filepath.Join(fixtureDir, "linked-worktree", "main")

	ctx := context.Background()

	// Bootstrap from linked worktree.
	wtResult, err := source.Bootstrap(ctx, wtDir)
	if err != nil {
		t.Fatalf("Bootstrap(wt): %v", err)
	}

	// Bootstrap from main worktree.
	mainResult, err := source.Bootstrap(ctx, mainDir)
	if err != nil {
		t.Fatalf("Bootstrap(main): %v", err)
	}

	// Linked worktree should have IsLinkedWorktree = true.
	if !wtResult.Repo.IsLinkedWorktree {
		t.Error("wt: IsLinkedWorktree = false, want true")
	}

	// Main worktree should have IsLinkedWorktree = false.
	if mainResult.Repo.IsLinkedWorktree {
		t.Error("main: IsLinkedWorktree = true, want false")
	}

	// GitCommonDir should be the same for both (the main .git dir).
	if wtResult.Repo.GitCommonDir != mainResult.Repo.GitCommonDir {
		t.Errorf("GitCommonDir mismatch:\n  wt:   %q\n  main: %q",
			wtResult.Repo.GitCommonDir, mainResult.Repo.GitCommonDir)
	}

	// GitDir should differ between main and linked worktree.
	if wtResult.Repo.GitDir == mainResult.Repo.GitDir {
		t.Errorf("GitDir should differ: both = %q", wtResult.Repo.GitDir)
	}

	// WorktreeRoot should point to the correct directories.
	if !strings.HasSuffix(wtResult.Repo.WorktreeRoot, "/wt") {
		t.Errorf("wt WorktreeRoot = %q, want suffix /wt", wtResult.Repo.WorktreeRoot)
	}
	if !strings.HasSuffix(mainResult.Repo.WorktreeRoot, "/main") {
		t.Errorf("main WorktreeRoot = %q, want suffix /main", mainResult.Repo.WorktreeRoot)
	}
}

func TestLinkedWorktreeDiffParity(t *testing.T) {
	t.Parallel()

	mainDir := filepath.Join(fixtureDir, "linked-worktree", "main")

	ctx := context.Background()

	mainResult, err := source.Bootstrap(ctx, mainDir)
	if err != nil {
		t.Fatalf("Bootstrap(main): %v", err)
	}

	// Compare HEAD~1..HEAD in the main worktree (the second commit).
	base, err := mainResult.Runner.RunGit(ctx, "rev-parse", "HEAD~1")
	if err != nil {
		t.Fatalf("rev-parse HEAD~1: %v", err)
	}
	head, err := mainResult.Runner.RunGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	cmp := model.ResolvedCompare{
		BaseRef:   strings.TrimSpace(base),
		HeadRef:   strings.TrimSpace(head),
		DiffRange: strings.TrimSpace(base) + ".." + strings.TrimSpace(head),
	}

	metaSvc := &diff.MetadataService{Runner: mainResult.Runner}
	files, err := metaSvc.ListFiles(ctx, cmp)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}
	if files[0].Path != "file.txt" {
		t.Errorf("path = %q, want %q", files[0].Path, "file.txt")
	}
	if files[0].Status != model.StatusModified {
		t.Errorf("status = %q, want %q", files[0].Status, model.StatusModified)
	}
}
