package commit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

// --- mock git runner ---

type mockGitRunner struct {
	fn func(ctx context.Context, args ...string) (string, error)
}

func (m *mockGitRunner) RunGit(ctx context.Context, args ...string) (string, error) {
	return m.fn(ctx, args...)
}

// --- fixture data ---

const fixtureDiff = `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {
`

var fixtureFiles = []model.FileSummary{
	{Path: "main.go", Status: model.StatusModified, Additions: 1, Deletions: 0},
}

// --- BuildPrompt tests ---

func TestBuildPrompt_containsDiff(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(fixtureDiff, fixtureFiles)
	if !strings.Contains(prompt, fixtureDiff) {
		t.Error("prompt does not contain the diff")
	}
}

func TestBuildPrompt_containsFileSummaries(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "main.go", Status: model.StatusModified, Additions: 3, Deletions: 1},
		{Path: "new.go", Status: model.StatusAdded, Additions: 10, Deletions: 0},
	}
	prompt := BuildPrompt(fixtureDiff, files)
	if !strings.Contains(prompt, "main.go") {
		t.Error("prompt does not contain file path main.go")
	}
	if !strings.Contains(prompt, "new.go") {
		t.Error("prompt does not contain file path new.go")
	}
}

func TestBuildPrompt_containsCommitStyleInstructions(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(fixtureDiff, fixtureFiles)
	lower := strings.ToLower(prompt)
	if !strings.Contains(lower, "conventional commit") {
		t.Error("prompt missing conventional commit instruction")
	}
	if !strings.Contains(prompt, "72") {
		t.Error("prompt missing 72-char subject line limit")
	}
}

func TestBuildPrompt_emptyDiff(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt("", nil)
	if prompt == "" {
		t.Error("prompt is empty; should still contain instructions")
	}
}

// --- CheckStagingGuard tests ---

func TestCheckStagingGuard_onlyStaged(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if strings.Contains(key, "--cached") {
			return "", &gitexec.GitError{ExitCode: 1, Stderr: ""} // staged changes
		}
		return "", nil // no unstaged changes
	}}

	if err := CheckStagingGuard(context.Background(), git); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckStagingGuard_onlyUnstaged(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if strings.Contains(key, "--cached") {
			return "", nil // no staged changes
		}
		return "", &gitexec.GitError{ExitCode: 1, Stderr: ""} // unstaged changes
	}}

	if err := CheckStagingGuard(context.Background(), git); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckStagingGuard_bothStagedAndUnstaged(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		return "", &gitexec.GitError{ExitCode: 1, Stderr: ""} // both have changes
	}}

	err := CheckStagingGuard(context.Background(), git)
	if !errors.Is(err, ErrUnstagedChanges) {
		t.Fatalf("error = %v, want ErrUnstagedChanges", err)
	}
}

func TestCheckStagingGuard_noChanges(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		return "", nil // no changes
	}}

	if err := CheckStagingGuard(context.Background(), git); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckStagingGuard_gitError(t *testing.T) {
	t.Parallel()

	gitErr := fmt.Errorf("permission denied")
	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		return "", gitErr
	}}

	err := CheckStagingGuard(context.Background(), git)
	if err == nil {
		t.Fatal("expected error for git failure, got nil")
	}
	if !errors.Is(err, gitErr) {
		t.Errorf("error = %v, want wrapped gitErr", err)
	}
}

// --- CollectStagedSnapshot tests ---

func TestCollectStagedSnapshot_success(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case strings.Contains(key, "--name-status"):
			return "M\x00main.go\x00", nil
		case strings.Contains(key, "--numstat"):
			return "1\t0\tmain.go\x00", nil
		case strings.Contains(key, "diff --cached"):
			return fixtureDiff, nil
		default:
			return "", nil
		}
	}}

	diff, files, err := CollectStagedSnapshot(context.Background(), git)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != fixtureDiff {
		t.Errorf("diff mismatch:\ngot:  %q\nwant: %q", diff, fixtureDiff)
	}
	if len(files) != 1 {
		t.Fatalf("files count = %d, want 1", len(files))
	}
	if files[0].Path != "main.go" {
		t.Errorf("files[0].Path = %q, want %q", files[0].Path, "main.go")
	}
	if files[0].Status != model.StatusModified {
		t.Errorf("files[0].Status = %q, want %q", files[0].Status, model.StatusModified)
	}
	if files[0].Additions != 1 {
		t.Errorf("files[0].Additions = %d, want 1", files[0].Additions)
	}
}

func TestCollectStagedSnapshot_rename(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		switch {
		case strings.Contains(key, "--name-status"):
			return "R100\x00old.go\x00new.go\x00", nil
		case strings.Contains(key, "--numstat"):
			return "0\t0\t\x00old.go\x00new.go\x00", nil
		case strings.Contains(key, "diff --cached"):
			return "diff --git a/old.go b/new.go\n", nil
		default:
			return "", nil
		}
	}}

	diff, files, err := CollectStagedSnapshot(context.Background(), git)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff == "" {
		t.Error("diff is empty")
	}
	if len(files) != 1 {
		t.Fatalf("files count = %d, want 1", len(files))
	}
	if files[0].Path != "new.go" {
		t.Errorf("files[0].Path = %q, want %q", files[0].Path, "new.go")
	}
	if files[0].OldPath != "old.go" {
		t.Errorf("files[0].OldPath = %q, want %q", files[0].OldPath, "old.go")
	}
	if files[0].Status != model.StatusRenamed {
		t.Errorf("files[0].Status = %q, want %q", files[0].Status, model.StatusRenamed)
	}
}

func TestCollectStagedSnapshot_noStagedChanges(t *testing.T) {
	t.Parallel()

	git := &mockGitRunner{fn: func(_ context.Context, args ...string) (string, error) {
		return "", nil
	}}

	diff, files, err := CollectStagedSnapshot(context.Background(), git)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "" {
		t.Errorf("diff = %q, want empty", diff)
	}
	if len(files) != 0 {
		t.Errorf("files count = %d, want 0", len(files))
	}
}
