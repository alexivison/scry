package gitexec

import (
	"context"
	"strings"
	"testing"
)

type mockRunner struct {
	fn func(ctx context.Context, args ...string) (string, error)
}

func (m *mockRunner) RunGit(ctx context.Context, args ...string) (string, error) {
	return m.fn(ctx, args...)
}

var _ GitRunner = (*mockRunner)(nil)

func routeGit(routes map[string]string) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if out, ok := routes[key]; ok {
			return out, nil
		}
		return "", &GitError{Args: args, ExitCode: 1, Stderr: "unexpected: " + key}
	}
}

func TestWorktreeList(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		output  string
		want    []WorktreeEntry
		wantErr bool
	}{
		"single main worktree": {
			output: "worktree /home/user/project\nHEAD abc123def456\nbranch refs/heads/main\n\n",
			want: []WorktreeEntry{
				{
					Path:   "/home/user/project",
					HEAD:   "abc123def456",
					Branch: "refs/heads/main",
				},
			},
		},
		"main plus linked worktree": {
			output: "worktree /home/user/project\nHEAD abc123\nbranch refs/heads/main\n\n" +
				"worktree /home/user/project-feature\nHEAD def456\nbranch refs/heads/feature\n\n",
			want: []WorktreeEntry{
				{Path: "/home/user/project", HEAD: "abc123", Branch: "refs/heads/main"},
				{Path: "/home/user/project-feature", HEAD: "def456", Branch: "refs/heads/feature"},
			},
		},
		"bare worktree": {
			output: "worktree /home/user/project.git\nHEAD abc123\nbare\n\n",
			want: []WorktreeEntry{
				{Path: "/home/user/project.git", HEAD: "abc123", Bare: true},
			},
		},
		"prunable worktree": {
			output: "worktree /home/user/project\nHEAD abc123\nbranch refs/heads/main\n\n" +
				"worktree /tmp/gone\nHEAD def456\nbranch refs/heads/gone\nprunable gitdir file points to non-existent location\n\n",
			want: []WorktreeEntry{
				{Path: "/home/user/project", HEAD: "abc123", Branch: "refs/heads/main"},
				{Path: "/tmp/gone", HEAD: "def456", Branch: "refs/heads/gone", Prunable: true},
			},
		},
		"detached HEAD": {
			output: "worktree /home/user/project\nHEAD abc123\ndetached\n\n",
			want: []WorktreeEntry{
				{Path: "/home/user/project", HEAD: "abc123"},
			},
		},
		"empty output": {
			output:  "",
			want:    nil,
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := &mockRunner{fn: routeGit(map[string]string{
				"worktree list --porcelain": tc.output,
			})}

			got, err := WorktreeList(t.Context(), runner)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for i, w := range tc.want {
				g := got[i]
				if g.Path != w.Path {
					t.Errorf("[%d] Path = %q, want %q", i, g.Path, w.Path)
				}
				if g.HEAD != w.HEAD {
					t.Errorf("[%d] HEAD = %q, want %q", i, g.HEAD, w.HEAD)
				}
				if g.Branch != w.Branch {
					t.Errorf("[%d] Branch = %q, want %q", i, g.Branch, w.Branch)
				}
				if g.Bare != w.Bare {
					t.Errorf("[%d] Bare = %v, want %v", i, g.Bare, w.Bare)
				}
				if g.Prunable != w.Prunable {
					t.Errorf("[%d] Prunable = %v, want %v", i, g.Prunable, w.Prunable)
				}
			}
		})
	}
}

func TestWorktreeListGitError(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{fn: func(_ context.Context, _ ...string) (string, error) {
		return "", &GitError{ExitCode: 128, Stderr: "fatal: not a git repository"}
	}}

	_, err := WorktreeList(t.Context(), runner)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCommitSubject(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		output      string
		wantHash    string
		wantSubject string
	}{
		"normal commit": {
			output:      "abc1234 initial commit\n",
			wantHash:    "abc1234",
			wantSubject: "initial commit",
		},
		"hash only no subject": {
			output:      "abc1234\n",
			wantHash:    "abc1234",
			wantSubject: "",
		},
		"subject with spaces": {
			output:      "def5678 fix: resolve merge conflict in main.go\n",
			wantHash:    "def5678",
			wantSubject: "fix: resolve merge conflict in main.go",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := &mockRunner{fn: routeGit(map[string]string{
				"-C /home/user/project log -1 --format=%h %s": tc.output,
			})}

			hash, subject, err := CommitSubject(t.Context(), runner, "/home/user/project")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hash != tc.wantHash {
				t.Errorf("hash = %q, want %q", hash, tc.wantHash)
			}
			if subject != tc.wantSubject {
				t.Errorf("subject = %q, want %q", subject, tc.wantSubject)
			}
		})
	}
}

func TestStatusCount(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		output string
		want   int
	}{
		"clean worktree": {
			output: "",
			want:   0,
		},
		"two changed files": {
			output: " M main.go\n?? new.go\n",
			want:   2,
		},
		"one changed file": {
			output: " M main.go\n",
			want:   1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runner := &mockRunner{fn: routeGit(map[string]string{
				"-C /home/user/project status --porcelain": tc.output,
			})}

			got, err := StatusCount(t.Context(), runner, "/home/user/project")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("count = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWorktreeRemove(t *testing.T) {
	t.Parallel()

	t.Run("normal remove", func(t *testing.T) {
		t.Parallel()
		runner := &mockRunner{fn: routeGit(map[string]string{
			"worktree remove /path/to/wt": "",
		})}
		err := WorktreeRemove(t.Context(), runner, "/path/to/wt", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("force remove", func(t *testing.T) {
		t.Parallel()
		runner := &mockRunner{fn: routeGit(map[string]string{
			"worktree remove --force /path/to/wt": "",
		})}
		err := WorktreeRemove(t.Context(), runner, "/path/to/wt", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("git error propagated", func(t *testing.T) {
		t.Parallel()
		runner := &mockRunner{fn: func(_ context.Context, _ ...string) (string, error) {
			return "", &GitError{ExitCode: 128, Stderr: "fatal: is the main worktree"}
		}}
		err := WorktreeRemove(t.Context(), runner, "/path/to/main", false)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestShortBranch(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  string
	}{
		"full ref":      {input: "refs/heads/main", want: "main"},
		"already short": {input: "main", want: "main"},
		"nested":        {input: "refs/heads/feature/foo", want: "feature/foo"},
		"empty":         {input: "", want: ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := ShortBranch(tc.input)
			if got != tc.want {
				t.Errorf("ShortBranch(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
