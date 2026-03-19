package diff_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/diff"
	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

var fixtureDir string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "scry-fixtures-*")
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

func resolvedCompare(t *testing.T, runner gitexec.GitRunner) model.ResolvedCompare {
	t.Helper()
	ctx := context.Background()
	base, err := runner.RunGit(ctx, "rev-parse", "HEAD~1")
	if err != nil {
		t.Fatalf("rev-parse HEAD~1: %v", err)
	}
	head, err := runner.RunGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return model.ResolvedCompare{
		BaseRef:   strings.TrimSpace(base),
		HeadRef:   strings.TrimSpace(head),
		DiffRange: strings.TrimSpace(base) + ".." + strings.TrimSpace(head),
	}
}

// rawGitDiffNameStatus runs git diff --name-status directly for golden comparison.
func rawGitDiffNameStatus(t *testing.T, runner gitexec.GitRunner, cmp model.ResolvedCompare) string {
	t.Helper()
	out, err := runner.RunGit(context.Background(), "diff", "--name-status", "-M", cmp.DiffRange)
	if err != nil {
		t.Fatalf("raw git diff --name-status: %v", err)
	}
	return out
}

func TestGoldenSimple(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "simple")
	cmp := resolvedCompare(t, runner)
	svc := &diff.MetadataService{Runner: runner}

	files, err := svc.ListFiles(context.Background(), cmp)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	// Verify against raw git diff.
	raw := rawGitDiffNameStatus(t, runner, cmp)

	// Parse expected statuses from raw output.
	wantStatuses := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		status := parts[0][:1]
		path := parts[len(parts)-1]
		wantStatuses[path] = status
	}

	if len(files) != len(wantStatuses) {
		t.Fatalf("file count = %d, want %d", len(files), len(wantStatuses))
	}

	for _, f := range files {
		want, ok := wantStatuses[f.Path]
		if !ok {
			t.Errorf("unexpected file %q in ListFiles output", f.Path)
			continue
		}
		if string(f.Status) != want {
			t.Errorf("file %q: status = %q, want %q", f.Path, f.Status, want)
		}
	}

	// Verify specific expectations.
	expectations := map[string]model.FileStatus{
		"greet.txt": model.StatusModified,
		"main.go":   model.StatusModified,
		"new.txt":   model.StatusAdded,
		"old.txt":   model.StatusDeleted,
	}
	for _, f := range files {
		if want, ok := expectations[f.Path]; ok {
			if f.Status != want {
				t.Errorf("file %q: status = %q, want %q", f.Path, f.Status, want)
			}
		}
	}
}

func TestGoldenSimplePatchParity(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "simple")
	cmp := resolvedCompare(t, runner)
	patchSvc := &diff.PatchService{Runner: runner}

	// Load patch for greet.txt and verify hunks match raw diff.
	fp, err := patchSvc.LoadPatch(context.Background(), cmp, "greet.txt", model.StatusModified, false)
	if err != nil {
		t.Fatalf("LoadPatch: %v", err)
	}

	if len(fp.Hunks) == 0 {
		t.Fatal("expected at least one hunk for greet.txt")
	}

	// Verify raw git diff --patch produces parseable output.
	rawPatch, err := runner.RunGit(context.Background(), "diff", "--patch", "--no-color", cmp.DiffRange, "--", "greet.txt")
	if err != nil {
		t.Fatalf("raw git diff --patch: %v", err)
	}
	if rawPatch == "" {
		t.Fatal("raw patch is empty")
	}

	// Verify hunk line counts match between raw and parsed.
	rawHunkCount := 0
	for _, line := range strings.Split(rawPatch, "\n") {
		if strings.HasPrefix(line, "@@") {
			rawHunkCount++
		}
	}
	if len(fp.Hunks) != rawHunkCount {
		t.Errorf("parsed hunks = %d, raw @@ count = %d", len(fp.Hunks), rawHunkCount)
	}
}

func TestGoldenRename(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "rename")
	cmp := resolvedCompare(t, runner)
	svc := &diff.MetadataService{Runner: runner}

	files, err := svc.ListFiles(context.Background(), cmp)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("file count = %d, want 1", len(files))
	}

	f := files[0]
	if f.Status != model.StatusRenamed {
		t.Errorf("status = %q, want %q", f.Status, model.StatusRenamed)
	}
	if f.OldPath != "util.go" {
		t.Errorf("OldPath = %q, want %q", f.OldPath, "util.go")
	}
	if f.Path != "helper.go" {
		t.Errorf("Path = %q, want %q", f.Path, "helper.go")
	}
}

func TestGoldenBinary(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "binary")
	cmp := resolvedCompare(t, runner)
	svc := &diff.MetadataService{Runner: runner}

	files, err := svc.ListFiles(context.Background(), cmp)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("file count = %d, want 2", len(files))
	}

	for _, f := range files {
		if !f.IsBinary {
			t.Errorf("file %q: IsBinary = false, want true", f.Path)
		}
	}
}

func TestGoldenSubmodule(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "submodule")
	cmp := resolvedCompare(t, runner)
	patchSvc := &diff.PatchService{Runner: runner}

	_, err := patchSvc.LoadPatch(context.Background(), cmp, "sub", model.StatusModified, false)
	if err == nil {
		t.Fatal("expected ErrSubmodule, got nil")
	}
	if !strings.Contains(err.Error(), "submodule") {
		t.Errorf("error = %q, want to contain 'submodule'", err.Error())
	}
}

func BenchmarkListFiles(b *testing.B) {
	dir := filepath.Join(fixtureDir, "simple")
	runner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: dir})
	ctx := context.Background()

	base, err := runner.RunGit(ctx, "rev-parse", "HEAD~1")
	if err != nil {
		b.Fatalf("rev-parse HEAD~1: %v", err)
	}
	head, err := runner.RunGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		b.Fatalf("rev-parse HEAD: %v", err)
	}

	cmp := model.ResolvedCompare{
		BaseRef:   strings.TrimSpace(base),
		HeadRef:   strings.TrimSpace(head),
		DiffRange: strings.TrimSpace(base) + ".." + strings.TrimSpace(head),
	}

	svc := &diff.MetadataService{Runner: runner}
	b.ResetTimer()
	for range b.N {
		if _, err := svc.ListFiles(ctx, cmp); err != nil {
			b.Fatalf("ListFiles: %v", err)
		}
	}
}

func TestGoldenLargeOversized(t *testing.T) {
	t.Parallel()

	runner := fixtureRunner(t, "large")
	cmp := resolvedCompare(t, runner)
	patchSvc := &diff.PatchService{Runner: runner}

	_, err := patchSvc.LoadPatch(context.Background(), cmp, "big.txt", model.StatusModified, false)
	if err == nil {
		t.Fatal("expected OversizedError, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds size threshold") {
		t.Errorf("error = %q, want oversized error", err.Error())
	}
}
