package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
				Basis:   model.CompareBasisUpstream,
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
				Basis:     model.CompareBasisUpstream,
				HeadRef:   "bbb222",
				MergeBase: "ccc333",
				DiffRange: "aaa111...bbb222",
			},
		},
		"two-dot with explicit refs": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "origin/main",
				Basis:   model.CompareBasisUpstream,
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
				Basis:     model.CompareBasisUpstream,
				HeadRef:   "bbb222",
				MergeBase: "",
				DiffRange: "aaa111..bbb222",
			},
		},
		"working tree mode when head omitted": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "origin/main",
				Basis:   model.CompareBasisUpstream,
				HeadRef: "", // empty → working tree mode
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify origin/main":
					return "base111\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:        stubRepo,
				BaseRef:     "base111",
				Basis:       model.CompareBasisUpstream,
				HeadRef:     "",
				WorkingTree: true,
				MergeBase:   "",
				DiffRange:   "base111",
			},
		},
		"explicit HEAD preserves committed mode": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "origin/main",
				Basis:   model.CompareBasisUpstream,
				HeadRef: "HEAD", // explicit → committed ref, not working tree
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
				Basis:     model.CompareBasisUpstream,
				HeadRef:   "head111",
				MergeBase: "mb111",
				DiffRange: "base111...head111",
			},
		},
		"default base resolves to merge-base with upstream": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "", // default → merge-base(head, @{upstream})
				Basis:   model.CompareBasisUpstream,
				HeadRef: "feature",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --symbolic-full-name --verify @{upstream}":
					return "refs/remotes/origin/main\n", nil
				case "merge-base feature refs/remotes/origin/main":
					return "mbup111\n", nil
				case "rev-parse --verify mbup111":
					return "mbup111\n", nil
				case "rev-parse --verify feature":
					return "feat111\n", nil
				case "merge-base mbup111 feat111":
					return "mb222\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "mbup111",
				Basis:     model.CompareBasisUpstream,
				HeadRef:   "feat111",
				MergeBase: "mb222",
				DiffRange: "mbup111...feat111",
			},
		},
		"default base resolves to merge-base with local trunk when requested": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "",
				Basis:   model.CompareBasisLocalTrunk,
				HeadRef: "feature",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "mainsha\n", nil
				case "merge-base feature refs/heads/main":
					return "mbmain111\n", nil
				case "rev-parse --verify mbmain111":
					return "mbmain111\n", nil
				case "rev-parse --verify feature":
					return "feat111\n", nil
				case "merge-base mbmain111 feat111":
					return "mb222\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "mbmain111",
				Basis:     model.CompareBasisLocalTrunk,
				HeadRef:   "feat111",
				MergeBase: "mb222",
				DiffRange: "mbmain111...feat111",
			},
		},
		"local trunk working tree uses HEAD merge-base": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "",
				Basis:   model.CompareBasisLocalTrunk,
				HeadRef: "",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "mainsha\n", nil
				case "merge-base HEAD refs/heads/main":
					return "mbmain111\n", nil
				case "rev-parse --verify mbmain111":
					return "mbmain111\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:        stubRepo,
				BaseRef:     "mbmain111",
				Basis:       model.CompareBasisLocalTrunk,
				WorkingTree: true,
				DiffRange:   "mbmain111",
			},
		},
		"head dirty working tree uses HEAD as base": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "",
				Basis:   model.CompareBasisHeadDirty,
				HeadRef: "",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify HEAD":
					return "head111\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:        stubRepo,
				BaseRef:     "head111",
				Basis:       model.CompareBasisHeadDirty,
				WorkingTree: true,
				DiffRange:   "HEAD",
			},
		},
		"local trunk basis returns error when branch missing": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "",
				Basis:   model.CompareBasisLocalTrunk,
				HeadRef: "feature",
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "", gitErr(1, "missing main")
				case "rev-parse --verify --quiet refs/heads/master":
					return "", gitErr(1, "missing master")
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "no local default branch found") {
					t.Errorf("error = %q, want local default branch message", err.Error())
				}
			},
		},
		"missing upstream falls back to merge-base with origin/main using explicit head": {
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
				case "merge-base feature origin/HEAD":
					return "", gitErr(1, "not a valid ref")
				case "merge-base feature origin/main":
					return "mb333\n", nil
				case "rev-parse --verify mb333":
					return "mb333\n", nil
				case "rev-parse --verify feature":
					return "feat111\n", nil
				case "merge-base mb333 feat111":
					return "mb555\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "mb333",
				Basis:     model.CompareBasisUpstream,
				HeadRef:   "feat111",
				MergeBase: "mb555",
				DiffRange: "mb333...feat111",
			},
		},
		"missing upstream falls back to merge-base with origin/HEAD": {
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
				case "merge-base feature origin/HEAD":
					return "mb444\n", nil
				case "rev-parse --verify mb444":
					return "mb444\n", nil
				case "rev-parse --verify feature":
					return "feat111\n", nil
				case "merge-base mb444 feat111":
					return "mb666\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:      stubRepo,
				BaseRef:   "mb444",
				Basis:     model.CompareBasisUpstream,
				HeadRef:   "feat111",
				MergeBase: "mb666",
				DiffRange: "mb444...feat111",
			},
		},
		"missing upstream working tree uses HEAD for merge-base": {
			req: model.CompareRequest{
				Repo:    stubRepo,
				BaseRef: "",
				HeadRef: "", // working tree — no explicit head
				Mode:    model.CompareThreeDot,
			},
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --symbolic-full-name --verify @{upstream}":
					return "", gitErr(128, "fatal: no upstream configured")
				case "merge-base HEAD origin/HEAD":
					return "", gitErr(1, "not a valid ref")
				case "merge-base HEAD origin/main":
					return "mb777\n", nil
				case "rev-parse --verify mb777":
					return "mb777\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: model.ResolvedCompare{
				Repo:        stubRepo,
				BaseRef:     "mb777",
				Basis:       model.CompareBasisUpstream,
				WorkingTree: true,
				DiffRange:   "mb777",
			},
		},
		"all fallbacks exhausted": {
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
					return "", gitErr(128, "fatal: no upstream configured")
				case "merge-base feature origin/HEAD":
					return "", gitErr(1, "not a valid ref")
				case "merge-base feature origin/main":
					return "", gitErr(1, "not a valid ref")
				case "merge-base feature origin/master":
					return "", gitErr(1, "not a valid ref")
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "no upstream configured") {
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

// --- LocalDefaultBranch ---------------------------------------------------

func TestLocalDefaultBranch(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		runner  func(ctx context.Context, args ...string) (string, error)
		want    string
		wantErr bool
	}{
		"main exists": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "abc123\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "main",
		},
		"master exists when main absent": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "", gitErr(1, "")
				case "rev-parse --verify --quiet refs/heads/master":
					return "def456\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "master",
		},
		"prefers main when both exist": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "abc123\n", nil
				case "rev-parse --verify --quiet refs/heads/master":
					return "def456\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "main",
		},
		"neither exists": {
			runner: func(_ context.Context, args ...string) (string, error) {
				return "", gitErr(1, "")
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := LocalDefaultBranch(context.Background(), &mockRunner{fn: tc.runner})
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("LocalDefaultBranch = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- WorktreeBaseRef ------------------------------------------------------

func TestWorktreeBaseRef(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		runner func(ctx context.Context, args ...string) (string, error)
		want   string
	}{
		"main + merge-base succeeds": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "headSHA\n", nil
				case "merge-base HEAD refs/heads/main":
					return "mbSHA\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "mbSHA",
		},
		"master + merge-base succeeds": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "", gitErr(1, "")
				case "rev-parse --verify --quiet refs/heads/master":
					return "headSHA\n", nil
				case "merge-base HEAD refs/heads/master":
					return "mbSHA\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "mbSHA",
		},
		"no local default branch returns empty": {
			runner: func(_ context.Context, args ...string) (string, error) {
				return "", gitErr(1, "")
			},
			want: "",
		},
		"merge-base failure returns empty": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "headSHA\n", nil
				case "merge-base HEAD refs/heads/main":
					return "", gitErr(1, "no common ancestor")
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "",
		},
		// Regression: ensure the fully qualified refs/heads/<branch> form is
		// used so a tag named "main" cannot shadow the local branch in
		// merge-base resolution.
		"uses refs/heads/ to avoid tag collision": {
			runner: func(_ context.Context, args ...string) (string, error) {
				key := strings.Join(args, " ")
				switch key {
				case "rev-parse --verify --quiet refs/heads/main":
					return "headSHA\n", nil
				case "merge-base HEAD refs/heads/main":
					return "branchMB\n", nil
				// A plain "main" would match refs/tags/main here. If
				// WorktreeBaseRef ever regresses to the short form, the
				// mock returns a tag SHA and the assertion fails.
				case "merge-base HEAD main":
					return "tagMB\n", nil
				default:
					return "", gitErr(1, "unexpected: "+key)
				}
			},
			want: "branchMB",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := WorktreeBaseRef(context.Background(), &mockRunner{fn: tc.runner})
			if got != tc.want {
				t.Errorf("WorktreeBaseRef = %q, want %q", got, tc.want)
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

func TestBootstrapFromGitDirUsesMatchingWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	r := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: dir})
	if _, err := r.RunGit(ctx, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	commitFixture(t, ctx, r, dir)
	if _, err := r.RunGit(ctx, "branch", "-M", "main"); err != nil {
		t.Fatalf("git branch: %v", err)
	}
	if _, err := r.RunGit(ctx, "worktree", "add", filepath.Join(t.TempDir(), "feature"), "-b", "feature"); err != nil {
		t.Fatalf("git worktree add: %v", err)
	}

	result, err := Bootstrap(ctx, filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatalf("Bootstrap from .git: %v", err)
	}
	if canonicalPath(t, result.Repo.WorktreeRoot) != canonicalPath(t, dir) {
		t.Fatalf("WorktreeRoot = %q, want %q", result.Repo.WorktreeRoot, dir)
	}
	if result.Runner == nil {
		t.Fatal("Runner is nil")
	}
}

func TestBootstrapFromBareGitDirUsesMatchingWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	parent := t.TempDir()
	bareDir := filepath.Join(parent, "repo.git")
	bareRunner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: parent})
	if _, err := bareRunner.RunGit(ctx, "init", "--bare", bareDir); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	if _, err := bareRunner.RunGit(ctx, "--git-dir", bareDir, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		t.Fatalf("git symbolic-ref: %v", err)
	}

	seedDir := filepath.Join(parent, "seed")
	if _, err := bareRunner.RunGit(ctx, "clone", bareDir, seedDir); err != nil {
		t.Fatalf("git clone: %v", err)
	}
	seedRunner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: seedDir})
	commitFixture(t, ctx, seedRunner, seedDir)
	if _, err := seedRunner.RunGit(ctx, "push", "origin", "HEAD:main"); err != nil {
		t.Fatalf("git push: %v", err)
	}

	mainDir := filepath.Join(parent, "main")
	if _, err := bareRunner.RunGit(ctx, "--git-dir", bareDir, "worktree", "add", mainDir, "main"); err != nil {
		t.Fatalf("git worktree add: %v", err)
	}

	result, err := Bootstrap(ctx, bareDir)
	if err != nil {
		t.Fatalf("Bootstrap from bare git dir: %v", err)
	}
	if canonicalPath(t, result.Repo.WorktreeRoot) != canonicalPath(t, mainDir) {
		t.Fatalf("WorktreeRoot = %q, want %q", result.Repo.WorktreeRoot, mainDir)
	}
	if result.Repo.IsLinkedWorktree {
		t.Fatal("IsLinkedWorktree = true, want false for bare dashboard host")
	}
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("eval symlinks %s: %v", path, err)
	}
	return real
}

func commitFixture(t *testing.T, ctx context.Context, r gitexec.GitRunner, dir string) {
	t.Helper()
	if _, err := r.RunGit(ctx, "config", "user.email", "scry@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if _, err := r.RunGit(ctx, "config", "user.name", "Scry Test"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := r.RunGit(ctx, "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := r.RunGit(ctx, "commit", "-m", "fixture"); err != nil {
		t.Fatalf("git commit: %v", err)
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
