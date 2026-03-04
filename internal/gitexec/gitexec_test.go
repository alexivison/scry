package gitexec

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewGitRunnerDefaultTimeout(t *testing.T) {
	t.Parallel()

	r := NewGitRunner(GitRunnerConfig{WorkDir: t.TempDir()})
	if r == nil {
		t.Fatal("NewGitRunner returned nil")
	}
}

func TestRunGitVersion(t *testing.T) {
	t.Parallel()

	r := NewGitRunner(GitRunnerConfig{WorkDir: t.TempDir()})
	out, err := r.RunGit(t.Context(), "version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(out, "git version") {
		t.Errorf("output = %q, want prefix %q", out, "git version")
	}
}

func TestRunGitUsesWorkDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := NewGitRunner(GitRunnerConfig{WorkDir: dir})

	// init a repo so rev-parse works
	if _, err := r.RunGit(t.Context(), "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	out, err := r.RunGit(t.Context(), "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Trim trailing newline for comparison.
	got := strings.TrimSpace(out)
	if !strings.Contains(got, dir) {
		t.Errorf("toplevel = %q, want to contain %q", got, dir)
	}
}

func TestRunGitNonZeroExit(t *testing.T) {
	t.Parallel()

	r := NewGitRunner(GitRunnerConfig{WorkDir: t.TempDir()})

	// rev-parse in a non-repo directory should fail
	_, err := r.RunGit(t.Context(), "rev-parse", "HEAD")
	if err == nil {
		t.Fatal("expected error for non-repo rev-parse, got nil")
	}

	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("error type = %T, want *GitError", err)
	}
	if gitErr.ExitCode == 0 {
		t.Error("ExitCode = 0, want non-zero")
	}
	if gitErr.Stderr == "" {
		t.Error("Stderr is empty, want error message")
	}
	if len(gitErr.Args) == 0 {
		t.Error("Args is empty, want command args")
	}
}

func TestGitErrorMessage(t *testing.T) {
	t.Parallel()

	e := &GitError{
		Args:     []string{"rev-parse", "HEAD"},
		ExitCode: 128,
		Stderr:   "fatal: not a git repository",
	}
	msg := e.Error()
	if !strings.Contains(msg, "128") {
		t.Errorf("Error() = %q, want to contain exit code", msg)
	}
	if !strings.Contains(msg, "rev-parse") {
		t.Errorf("Error() = %q, want to contain command args", msg)
	}
	if !strings.Contains(msg, "fatal: not a git repository") {
		t.Errorf("Error() = %q, want to contain stderr", msg)
	}
}

func TestRunGitContextCancellation(t *testing.T) {
	t.Parallel()

	r := NewGitRunner(GitRunnerConfig{
		WorkDir: t.TempDir(),
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel immediately

	_, err := r.RunGit(ctx, "version")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestRunGitTimeout(t *testing.T) {
	t.Parallel()

	r := NewGitRunner(GitRunnerConfig{
		WorkDir: t.TempDir(),
		Timeout: 1 * time.Millisecond,
	})

	_, err := r.RunGit(t.Context(), "version")
	if err == nil {
		t.Fatal("expected error for timed-out command, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestRunGitMultipleArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := NewGitRunner(GitRunnerConfig{WorkDir: dir})

	if _, err := r.RunGit(t.Context(), "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// git log with multiple flags in an empty repo should fail gracefully
	_, err := r.RunGit(t.Context(), "log", "--oneline", "-1")
	if err == nil {
		t.Fatal("expected error for git log in empty repo, got nil")
	}

	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("error type = %T, want *GitError", err)
	}
}

func TestNewGitRunnerAcceptsExplicitTimeout(t *testing.T) {
	t.Parallel()

	r := NewGitRunner(GitRunnerConfig{WorkDir: t.TempDir(), Timeout: 10 * time.Second})
	if r == nil {
		t.Fatal("NewGitRunner returned nil")
	}
}
