package commit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/gitexec"
)

func TestCommitExecution_success(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(key, "commit"):
			return "", nil
		case strings.HasPrefix(key, "rev-parse"):
			return "abc1234\n", nil
		default:
			return "", nil
		}
	}}

	exec := &Executor{Git: git}
	sha, err := exec.Execute(context.Background(), "feat: add feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc1234" {
		t.Errorf("SHA = %q, want %q", sha, "abc1234")
	}
}

func TestCommitExecution_nothingToCommit(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if strings.HasPrefix(key, "commit") {
			return "", &gitexec.GitError{
				Args:     args,
				ExitCode: 1,
				Stderr:   "nothing to commit, working tree clean",
			}
		}
		return "", nil
	}}

	exec := &Executor{Git: git}
	_, err := exec.Execute(context.Background(), "feat: add feature")
	if err == nil {
		t.Fatal("expected error for nothing-to-commit, got nil")
	}

	var gitErr *gitexec.GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("error type = %T, want *gitexec.GitError", err)
	}
	if gitErr.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", gitErr.ExitCode)
	}
	if !strings.Contains(gitErr.Stderr, "nothing to commit") {
		t.Errorf("Stderr = %q, want to contain 'nothing to commit'", gitErr.Stderr)
	}
}

func TestCommitExecution_hookRejection(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if strings.HasPrefix(key, "commit") {
			return "", &gitexec.GitError{
				Args:     args,
				ExitCode: 1,
				Stderr:   "pre-commit hook rejected the commit",
			}
		}
		return "", nil
	}}

	exec := &Executor{Git: git}
	_, err := exec.Execute(context.Background(), "feat: add feature")
	if err == nil {
		t.Fatal("expected error for hook rejection, got nil")
	}

	var gitErr *gitexec.GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("error type = %T, want *gitexec.GitError", err)
	}
	if !strings.Contains(gitErr.Stderr, "pre-commit hook") {
		t.Errorf("Stderr = %q, want to contain 'pre-commit hook'", gitErr.Stderr)
	}
}

func TestCommitExecution_passesMessageToGit(t *testing.T) {
	t.Parallel()

	var capturedArgs []string
	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if strings.HasPrefix(key, "commit") {
			capturedArgs = args
			return "", nil
		}
		return "abc1234\n", nil
	}}

	exec := &Executor{Git: git}
	_, err := exec.Execute(context.Background(), "fix: repair bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedArgs) < 3 {
		t.Fatalf("capturedArgs = %v, want at least 3 elements", capturedArgs)
	}
	if capturedArgs[0] != "commit" || capturedArgs[1] != "-m" || capturedArgs[2] != "fix: repair bug" {
		t.Errorf("git args = %v, want [commit -m 'fix: repair bug']", capturedArgs)
	}
}
