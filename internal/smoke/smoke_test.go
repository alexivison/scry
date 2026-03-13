// Package smoke provides end-to-end smoke tests that exercise multiple
// packages together against real fixture Git repositories.
package smoke_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/commit"
	"github.com/alexivison/scry/internal/diff"
	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/watch"
)

var fixtureDir string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "scry-smoke-fixtures-*")
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

func fixtureRunner(t *testing.T, name string) gitexec.GitRunner {
	t.Helper()
	dir := filepath.Join(fixtureDir, name)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("fixture %q not found: %v", name, err)
	}
	return gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: dir})
}

// copyFixture copies a fixture repo to a temp directory via os.CopyFS
// and returns a GitRunner for the copy.
func copyFixture(t *testing.T, name string) gitexec.GitRunner {
	t.Helper()
	src := filepath.Join(fixtureDir, name)
	dst := filepath.Join(t.TempDir(), "repo")
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copy fixture %q: %v", name, err)
	}
	return gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: dst})
}

// ─── Watch: no-divergence startup ───────────────────────────────────────────

func TestSmoke_NoDivergenceWatchStartup(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "no-divergence")
	ctx := context.Background()

	// setup.sh pins initial branch to "main" and creates "feature" at the same commit.
	head, err := runner.RunGit(ctx, "rev-parse", "feature")
	if err != nil {
		t.Fatalf("rev-parse feature: %v", err)
	}
	base, err := runner.RunGit(ctx, "rev-parse", "main")
	if err != nil {
		t.Fatalf("rev-parse main: %v", err)
	}

	headSHA := strings.TrimSpace(head)
	baseSHA := strings.TrimSpace(base)

	// Verify topology: feature == main (no divergence).
	if headSHA != baseSHA {
		t.Fatalf("no-divergence: HEAD (%s) != base (%s), fixture is wrong", headSHA, baseSHA)
	}

	cmp := model.ResolvedCompare{
		BaseRef:   baseSHA,
		HeadRef:   headSHA,
		DiffRange: baseSHA + ".." + headSHA,
	}

	metaSvc := &diff.MetadataService{Runner: runner}
	files, err := metaSvc.ListFiles(ctx, cmp)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("no-divergence should produce 0 files, got %d", len(files))
	}
}

// ─── Watch: fingerprint divergence detection ────────────────────────────────

func TestSmoke_WatchFingerprintDivergence(t *testing.T) {
	t.Parallel()

	// Work on a copy so we can mutate the repo between fingerprint calls.
	runner := copyFixture(t, "watch-divergence")
	ctx := context.Background()

	fp := &watch.Fingerprinter{Runner: runner}

	// Compute fingerprint at current state.
	before, err := fp.Fingerprint(ctx, "HEAD~1", false)
	if err != nil {
		t.Fatalf("before fingerprint: %v", err)
	}
	if before == "" {
		t.Fatal("before fingerprint is empty")
	}

	// Mutate: create a new commit so HEAD advances.
	if _, err := runner.RunGit(ctx, "-c", "user.name=test", "-c", "user.email=test@test.com",
		"commit", "--allow-empty", "-m", "advance HEAD"); err != nil {
		t.Fatalf("create commit: %v", err)
	}

	// Re-fingerprint: should differ because HEAD changed.
	after, err := fp.Fingerprint(ctx, "HEAD~2", false)
	if err != nil {
		t.Fatalf("after fingerprint: %v", err)
	}

	if before == after {
		t.Errorf("fingerprint should change after new commit: both %q", before)
	}

	// Verify ShouldRefresh detects the change.
	state := model.AppState{LastFingerprint: before}
	if !watch.ShouldRefresh(&state, after) {
		t.Error("ShouldRefresh should return true after fingerprint change")
	}
}

func TestSmoke_WatchFingerprintWorkingTree(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "simple")
	ctx := context.Background()

	fp := &watch.Fingerprinter{Runner: runner}

	wt, err := fp.Fingerprint(ctx, "HEAD~1", true)
	if err != nil {
		t.Fatalf("working-tree fingerprint: %v", err)
	}

	committed, err := fp.Fingerprint(ctx, "HEAD~1", false)
	if err != nil {
		t.Fatalf("committed fingerprint: %v", err)
	}

	// Committed has 2 parts, working-tree has 3 (third is diff appendix).
	committedParts := strings.Split(committed, ":")
	wtParts := strings.Split(wt, ":")
	if len(committedParts) != 2 {
		t.Errorf("committed fingerprint parts = %d, want 2", len(committedParts))
	}
	if len(wtParts) != 3 {
		t.Errorf("working-tree fingerprint parts = %d, want 3", len(wtParts))
	}
}

// ─── Watch: linked-worktree fingerprint ─────────────────────────────────────

func TestSmoke_LinkedWorktreeWatchBehavior(t *testing.T) {
	t.Parallel()

	mainRunner := fixtureRunner(t, "linked-worktree/main")
	wtRunner := fixtureRunner(t, "linked-worktree/wt")
	ctx := context.Background()

	mainFP := &watch.Fingerprinter{Runner: mainRunner}
	wtFP := &watch.Fingerprinter{Runner: wtRunner}

	mainFingerprint, err := mainFP.Fingerprint(ctx, "HEAD~1", false)
	if err != nil {
		t.Fatalf("main fingerprint: %v", err)
	}

	wtFingerprint, err := wtFP.Fingerprint(ctx, "HEAD", false)
	if err != nil {
		t.Fatalf("wt fingerprint: %v", err)
	}

	// Different worktrees at different HEADs should produce different fingerprints.
	if mainFingerprint == wtFingerprint {
		t.Errorf("main and linked-worktree fingerprints should differ: both %q", mainFingerprint)
	}
}

// ─── Commit: staged snapshot from real repo ─────────────────────────────────

func TestSmoke_CommitStagedSnapshot(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "staged-simple")
	ctx := context.Background()

	diff, files, err := commit.CollectStagedSnapshot(ctx, runner)
	if err != nil {
		t.Fatalf("CollectStagedSnapshot: %v", err)
	}

	if diff == "" {
		t.Fatal("staged diff is empty")
	}
	if len(files) == 0 {
		t.Fatal("staged files list is empty")
	}

	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.Path] = true
	}
	if !paths["main.go"] {
		t.Error("expected main.go in staged files")
	}
	if !paths["added.txt"] {
		t.Error("expected added.txt in staged files")
	}
}

// ─── Commit: execution success ──────────────────────────────────────────────

func TestSmoke_CommitExecutionSuccess(t *testing.T) {
	t.Parallel()

	runner := copyFixture(t, "staged-simple")
	ctx := context.Background()

	executor := &commit.Executor{Git: runner}
	sha, err := executor.Execute(ctx, "feat: smoke test commit")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if sha == "" {
		t.Fatal("commit SHA is empty")
	}

	// Verify the commit message was recorded.
	msg, err := runner.RunGit(ctx, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if strings.TrimSpace(msg) != "feat: smoke test commit" {
		t.Errorf("commit message = %q, want %q", strings.TrimSpace(msg), "feat: smoke test commit")
	}
}

// ─── Commit: execution failure (nothing staged) ─────────────────────────────

func TestSmoke_CommitExecutionNothingStaged(t *testing.T) {
	t.Parallel()

	// Use a copy to avoid mutating the shared fixture.
	runner := copyFixture(t, "no-divergence")
	ctx := context.Background()

	executor := &commit.Executor{Git: runner}
	_, err := executor.Execute(ctx, "feat: should fail")
	if err == nil {
		t.Fatal("expected error for commit with nothing staged, got nil")
	}
}

// ─── Full pipeline: diff metadata + patch for simple fixture ────────────────

func TestSmoke_FullDiffPipeline(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "simple")
	ctx := context.Background()

	base, err := runner.RunGit(ctx, "rev-parse", "HEAD~1")
	if err != nil {
		t.Fatalf("rev-parse HEAD~1: %v", err)
	}
	head, err := runner.RunGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	cmp := model.ResolvedCompare{
		BaseRef:   strings.TrimSpace(base),
		HeadRef:   strings.TrimSpace(head),
		DiffRange: strings.TrimSpace(base) + ".." + strings.TrimSpace(head),
	}

	metaSvc := &diff.MetadataService{Runner: runner}
	files, err := metaSvc.ListFiles(ctx, cmp)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if len(files) < 3 {
		t.Fatalf("expected at least 3 files in simple fixture, got %d", len(files))
	}

	patchSvc := &diff.PatchService{Runner: runner}
	for _, f := range files {
		if f.Status == model.StatusDeleted {
			continue
		}
		fp, err := patchSvc.LoadPatch(ctx, cmp, f.Path, false)
		if err != nil {
			t.Errorf("LoadPatch(%s): %v", f.Path, err)
			continue
		}
		if len(fp.Hunks) == 0 {
			t.Errorf("LoadPatch(%s): 0 hunks", f.Path)
		}
	}
}

// ─── Race detector: concurrent fingerprint calls ────────────────────────────

func TestSmoke_ConcurrentFingerprints(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "simple")
	fp := &watch.Fingerprinter{Runner: runner}
	ctx := context.Background()

	errs := make(chan error, 10)
	for range 10 {
		go func() {
			_, err := fp.Fingerprint(ctx, "HEAD~1", true)
			errs <- err
		}()
	}

	for range 10 {
		if err := <-errs; err != nil {
			t.Errorf("concurrent Fingerprint: %v", err)
		}
	}
}
