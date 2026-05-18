package gitexec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// initRepoWithCommit creates a git repo with a single committed file.
// Returns the repo dir and a runner rooted there.
func initRepoWithCommit(t *testing.T, fileName, content string) (string, GitRunner) {
	t.Helper()
	dir := t.TempDir()
	r := NewGitRunner(GitRunnerConfig{WorkDir: dir})
	ctx := context.Background()

	mustRun := func(args ...string) {
		if _, err := r.RunGit(ctx, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	mustRun("init")
	mustRun("config", "user.email", "test@example.com")
	mustRun("config", "user.name", "Test")
	mustRun("config", "commit.gpgsign", "false")

	path := filepath.Join(dir, fileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustRun("add", fileName)
	mustRun("commit", "-m", "initial")
	return dir, r
}

func TestDiscardFile_Tracked_RestoresCommittedContent(t *testing.T) {
	t.Parallel()

	dir, r := initRepoWithCommit(t, "hello.txt", "original\n")
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("dirty change\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := DiscardFile(context.Background(), r, dir, "hello.txt", false); err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "original\n" {
		t.Errorf("file content = %q, want %q", got, "original\n")
	}
}

func TestDiscardFile_Untracked_DeletesFromDisk(t *testing.T) {
	t.Parallel()

	dir, r := initRepoWithCommit(t, "tracked.txt", "x\n")
	untracked := filepath.Join(dir, "junk.txt")
	if err := os.WriteFile(untracked, []byte("transient\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := DiscardFile(context.Background(), r, dir, "junk.txt", true); err != nil {
		t.Fatalf("DiscardFile: %v", err)
	}

	if _, err := os.Stat(untracked); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected file removed, stat err = %v", err)
	}
}

func TestDiscardFile_Tracked_ErrorPropagates(t *testing.T) {
	t.Parallel()

	dir, r := initRepoWithCommit(t, "real.txt", "x\n")

	err := DiscardFile(context.Background(), r, dir, "no-such-file.txt", false)
	if err == nil {
		t.Fatal("expected error for unknown path, got nil")
	}
}

func TestDiscardFile_Untracked_MissingFileError(t *testing.T) {
	t.Parallel()

	dir, r := initRepoWithCommit(t, "real.txt", "x\n")

	err := DiscardFile(context.Background(), r, dir, "ghost.txt", true)
	if err == nil {
		t.Fatal("expected error when removing missing untracked file, got nil")
	}
}
