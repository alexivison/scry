package source

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

// mockRunner dispatches RunGit calls to a user-supplied function.
type mockRunner struct {
	fn func(ctx context.Context, args ...string) (string, error)
}

func (m *mockRunner) RunGit(ctx context.Context, args ...string) (string, error) {
	return m.fn(ctx, args...)
}

var _ gitexec.GitRunner = (*mockRunner)(nil)

// gitErr returns a *gitexec.GitError matching a non-zero git exit.
func gitErr(code int, stderr string, args ...string) error {
	return &gitexec.GitError{Args: args, ExitCode: code, Stderr: stderr}
}

// --- ResolveRepoContext ---------------------------------------------------

func TestResolveRepoContext(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		runner   func(ctx context.Context, args ...string) (string, error)
		want     model.RepoContext
		wantErr  bool
		errCheck func(t *testing.T, err error)
	}{
		"normal repo": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --show-toplevel":
					return "/home/user/project\n", nil
				case "rev-parse --absolute-git-dir":
					return "/home/user/project/.git\n", nil
				case "rev-parse --git-common-dir":
					return "/home/user/project/.git\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.RepoContext{
				WorktreeRoot:     "/home/user/project",
				GitDir:           "/home/user/project/.git",
				GitCommonDir:     "/home/user/project/.git",
				IsLinkedWorktree: false,
			},
		},
		"linked worktree": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --show-toplevel":
					return "/home/user/worktrees/feature\n", nil
				case "rev-parse --absolute-git-dir":
					return "/home/user/project/.git/worktrees/feature\n", nil
				case "rev-parse --git-common-dir":
					return "/home/user/project/.git\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.RepoContext{
				WorktreeRoot:     "/home/user/worktrees/feature",
				GitDir:           "/home/user/project/.git/worktrees/feature",
				GitCommonDir:     "/home/user/project/.git",
				IsLinkedWorktree: true,
			},
		},
		"relative git-common-dir in main worktree": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --show-toplevel":
					return "/home/user/project\n", nil
				case "rev-parse --absolute-git-dir":
					return "/home/user/project/.git\n", nil
				case "rev-parse --git-common-dir":
					return ".git\n", nil // relative — main worktree
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.RepoContext{
				WorktreeRoot:     "/home/user/project",
				GitDir:           "/home/user/project/.git",
				GitCommonDir:     "/home/user/project/.git", // canonicalized to gitDir
				IsLinkedWorktree: false,
			},
		},
		"not a git repo": {
			runner: func(_ context.Context, args ...string) (string, error) {
				return "", gitErr(128, "fatal: not a git repository")
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				var ge *gitexec.GitError
				if !errors.As(err, &ge) {
					t.Errorf("error type = %T, want *gitexec.GitError", err)
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			got, err := ResolveRepoContext(ctx, &mockRunner{fn: tc.runner})

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errCheck != nil {
					tc.errCheck(t, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("RepoContext =\n  got  %+v\n  want %+v", got, tc.want)
			}
		})
	}
}

// --- CompareResolver.Resolve ----------------------------------------------

func TestCompareResolverResolve(t *testing.T) {
	t.Parallel()

	stubRepo := model.RepoContext{
		WorktreeRoot: "/repo",
		GitDir:       "/repo/.git",
		GitCommonDir: "/repo/.git",
	}

	tests := map[string]struct {
		req      model.CompareRequest
		runner   func(ctx context.Context, args ...string) (string, error)
		want     model.ResolvedCompare
		wantErr  bool
		errCheck func(t *testing.T, err error)
	}{
		"three-dot with explicit refs": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "origin/main",
				HeadRef: "feature",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify origin/main":
					return "aaa111\n", nil
				case "rev-parse --verify feature":
					return "bbb222\n", nil
				case "merge-base aaa111 bbb222":
					return "ccc333\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "aaa111",
				HeadRef:   "bbb222",
				MergeBase: "ccc333",
				DiffRange: "aaa111...bbb222",
			},
		},
		"two-dot with explicit refs": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "origin/main",
				HeadRef: "feature",
				Mode:    model.CompareTwoDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify origin/main":
					return "aaa111\n", nil
				case "rev-parse --verify feature":
					return "bbb222\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "aaa111",
				HeadRef:   "bbb222",
				MergeBase: "",
				DiffRange: "aaa111..bbb222",
			},
		},
		"default head resolves to HEAD": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "origin/main",
				HeadRef: "", // default → HEAD
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify HEAD":
					return "head111\n", nil
				case "rev-parse --verify origin/main":
					return "base111\n", nil
				case "merge-base base111 head111":
					return "mb111\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "base111",
				HeadRef:   "head111",
				MergeBase: "mb111",
				DiffRange: "base111...head111",
			},
		},
		"default base resolves to upstream": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "", // default → @{upstream}
				HeadRef: "feature",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --symbolic-full-name --verify @{upstream}":
					return "refs/remotes/origin/main\n", nil
				case "rev-parse --verify refs/remotes/origin/main":
					return "up111\n", nil
				case "rev-parse --verify feature":
					return "feat111\n", nil
				case "merge-base up111 feat111":
					return "mb222\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "up111",
				HeadRef:   "feat111",
				MergeBase: "mb222",
				DiffRange: "up111...feat111",
			},
		},
		"missing upstream with empty base": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "",
				HeadRef: "feature",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --symbolic-full-name --verify @{upstream}":
					return "", gitErr(128, "fatal: no upstream configured for branch 'feature'")
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				if err == nil {
					t.Fatal("expected error")
				}
				// Should mention upstream in the error message
				if !strings.Contains(err.Error(), "upstream") {
					t.Errorf("error = %q, want mention of upstream", err.Error())
				}
			},
		},
		"unresolvable head ref": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "origin/main",
				HeadRef: "nonexistent",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify nonexistent":
					return "", gitErr(128, "fatal: Needed a single revision")
				case "rev-parse --verify origin/main":
					return "base111\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			wantErr: true,
		},
		"unresolvable base ref": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "nonexistent",
				HeadRef: "feature",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify nonexistent":
					return "", gitErr(128, "fatal: Needed a single revision")
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			cr := &CompareResolver{Runner: &mockRunner{fn: tc.runner}}
			got, err := cr.Resolve(ctx, tc.req)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errCheck != nil {
					tc.errCheck(t, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ResolvedCompare =\n  got  %+v\n  want %+v", got, tc.want)
			}
		})
	}
}

// --- Bootstrap ------------------------------------------------------------

func TestBootstrapSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: dir})
	ctx := context.Background()

	if _, err := r.RunGit(ctx, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	result, err := Bootstrap(ctx, dir)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	if result.Repo.WorktreeRoot == "" {
		t.Error("WorktreeRoot is empty")
	}
	if result.Repo.GitDir == "" {
		t.Error("GitDir is empty")
	}
	if result.Repo.IsLinkedWorktree {
		t.Error("IsLinkedWorktree = true, want false for main worktree")
	}
	if result.Runner == nil {
		t.Error("Runner is nil")
	}

	// Verify the repo-scoped runner works from WorktreeRoot.
	out, err := result.Runner.RunGit(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("repo runner rev-parse: %v", err)
	}
	if !strings.Contains(out, result.Repo.WorktreeRoot) {
		t.Errorf("repo runner toplevel = %q, want to contain %q", out, result.Repo.WorktreeRoot)
	}
}

func TestBootstrapNotARepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir() // no git init
	_, err := Bootstrap(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for non-repo directory, got nil")
	}
}
