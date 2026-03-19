package config

import (
	"testing"
	"time"

	"github.com/alexivison/scry/internal/model"
)

// noConfigFiles returns a ParseOption that prevents loading ambient config files.
func noConfigFiles() ParseOption {
	return WithConfigPaths("/tmp/scry-test-nonexistent-user-config.toml", "/tmp/scry-test-nonexistent-repo-config.toml")
}

func TestParseDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{}, noConfigFiles())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseRef != "" {
		t.Errorf("BaseRef = %q, want empty", cfg.BaseRef)
	}
	if cfg.HeadRef != "" {
		t.Errorf("HeadRef = %q, want empty", cfg.HeadRef)
	}
	if cfg.Mode != model.CompareThreeDot {
		t.Errorf("Mode = %q, want %q", cfg.Mode, model.CompareThreeDot)
	}
	if cfg.IgnoreWhitespace {
		t.Error("IgnoreWhitespace = true, want false")
	}
	if !cfg.Watch {
		t.Error("Watch = false, want true (default)")
	}
	if cfg.WatchInterval != 2*time.Second {
		t.Errorf("WatchInterval = %v, want 2s", cfg.WatchInterval)
	}
	if cfg.Commit {
		t.Error("Commit = true, want false")
	}
	if cfg.CommitProvider != "claude" {
		t.Errorf("CommitProvider = %q, want %q", cfg.CommitProvider, "claude")
	}
	if cfg.CommitModel != "" {
		t.Errorf("CommitModel = %q, want empty", cfg.CommitModel)
	}
	if cfg.CommitAuto {
		t.Error("CommitAuto = true, want false")
	}
}

func TestParseInvalidFlag(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--nonexistent"}, noConfigFiles())
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
}

func TestParseRejectsPositionalArgs(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"unexpected-arg"}, noConfigFiles())
	if err == nil {
		t.Fatal("expected error for positional argument, got nil")
	}
}

func TestParseCommitAutoRequiresCommit(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--commit-auto"}, noConfigFiles())
	if err == nil {
		t.Fatal("expected error for --commit-auto without --commit, got nil")
	}
}

func TestParseNoWatchFlag(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{"--no-watch"}, noConfigFiles())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Watch {
		t.Error("Watch = true, want false after --no-watch")
	}
}

func TestParseNoDashboardFlag(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{"--no-dashboard"}, noConfigFiles())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoDashboard {
		t.Error("NoDashboard = false, want true after --no-dashboard")
	}
}

// --- V3-T15: deprecated flags rejected ---

func TestParseDeprecatedFlagsRejected(t *testing.T) {
	t.Parallel()

	deprecated := []string{
		"--watch",
		"--worktrees",
		"--mode=three-dot",
		"--watch-interval=2s",
		"--ignore-whitespace",
		"--commit-provider=claude",
		"--commit-model=test",
	}
	for _, flag := range deprecated {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]string{flag}, noConfigFiles())
			if err == nil {
				t.Errorf("expected error for deprecated flag %s, got nil", flag)
			}
		})
	}
}

// --- V3-T15: final CLI surface ---

func TestParseFinalCLISurface(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{
		"--base", "origin/main",
		"--head", "HEAD",
		"--commit",
		"--commit-auto",
		"--no-watch",
		"--no-dashboard",
	}, noConfigFiles())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseRef != "origin/main" {
		t.Errorf("BaseRef = %q, want %q", cfg.BaseRef, "origin/main")
	}
	if cfg.HeadRef != "HEAD" {
		t.Errorf("HeadRef = %q, want %q", cfg.HeadRef, "HEAD")
	}
	if !cfg.Commit {
		t.Error("Commit = false, want true")
	}
	if !cfg.CommitAuto {
		t.Error("CommitAuto = false, want true")
	}
	if cfg.Watch {
		t.Error("Watch = true, want false after --no-watch")
	}
	if !cfg.NoDashboard {
		t.Error("NoDashboard = false, want true")
	}
}

// --- V3-T15: config file merge precedence ---

func TestMergeFileConfigs(t *testing.T) {
	t.Parallel()

	user := FileConfig{}
	user.Diff.Mode = strPtr("two-dot")
	user.Watch.Interval = strPtr("5s")

	repo := FileConfig{}
	repo.Diff.Mode = strPtr("three-dot") // overrides user
	repo.Commit.Provider = strPtr("claude")

	merged := MergeFileConfigs(user, repo)
	if merged.Diff.Mode == nil || *merged.Diff.Mode != "three-dot" {
		t.Errorf("Diff.Mode = %v, want %q (repo overrides user)", merged.Diff.Mode, "three-dot")
	}
	if merged.Watch.Interval == nil || *merged.Watch.Interval != "5s" {
		t.Errorf("Watch.Interval = %v, want %q (from user, repo empty)", merged.Watch.Interval, "5s")
	}
	if merged.Commit.Provider == nil || *merged.Commit.Provider != "claude" {
		t.Errorf("Commit.Provider = %v, want %q (from repo)", merged.Commit.Provider, "claude")
	}
}

func TestMergeFileConfigs_RepoOverridesToFalse(t *testing.T) {
	t.Parallel()

	tr := true
	fa := false
	user := FileConfig{}
	user.Diff.IgnoreWhitespace = &tr

	repo := FileConfig{}
	repo.Diff.IgnoreWhitespace = &fa // repo explicitly resets to false

	merged := MergeFileConfigs(user, repo)
	if merged.Diff.IgnoreWhitespace == nil || *merged.Diff.IgnoreWhitespace != false {
		t.Error("repo config should override user IgnoreWhitespace=true to false")
	}
}

func TestShouldUseDashboard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		worktreeCount int
		noDashboard   bool
		want          bool
	}{
		{"single worktree", 1, false, false},
		{"multiple worktrees, auto-detect", 2, false, true},
		{"multiple worktrees, no-dashboard", 2, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Config{NoDashboard: tt.noDashboard}
			got := cfg.ShouldUseDashboard(tt.worktreeCount)
			if got != tt.want {
				t.Errorf("ShouldUseDashboard(%d) = %v, want %v", tt.worktreeCount, got, tt.want)
			}
		})
	}
}
