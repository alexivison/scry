package config

import (
	"testing"
	"time"

	"github.com/alexivison/scry/internal/model"
)

func TestParseDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{})
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
}

func TestParseAllFlags(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{
		"--base", "origin/main",
		"--head", "feature",
		"--mode", "two-dot",
		"--ignore-whitespace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseRef != "origin/main" {
		t.Errorf("BaseRef = %q, want %q", cfg.BaseRef, "origin/main")
	}
	if cfg.HeadRef != "feature" {
		t.Errorf("HeadRef = %q, want %q", cfg.HeadRef, "feature")
	}
	if cfg.Mode != model.CompareTwoDot {
		t.Errorf("Mode = %q, want %q", cfg.Mode, model.CompareTwoDot)
	}
	if !cfg.IgnoreWhitespace {
		t.Error("IgnoreWhitespace = false, want true")
	}
}

func TestParseInvalidMode(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--mode", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}
}

func TestParseInvalidFlag(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
}

func TestParseRejectsPositionalArgs(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"unexpected-arg"})
	if err == nil {
		t.Fatal("expected error for positional argument, got nil")
	}
}

// --- v0.2 flag tests ---

func TestParseWatchDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Watch {
		t.Error("Watch = true, want false")
	}
	if cfg.WatchInterval != 2*time.Second {
		t.Errorf("WatchInterval = %v, want 2s", cfg.WatchInterval)
	}
}

func TestParseWatchFlags(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{"--watch", "--watch-interval", "5s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Watch {
		t.Error("Watch = false, want true")
	}
	if cfg.WatchInterval != 5*time.Second {
		t.Errorf("WatchInterval = %v, want 5s", cfg.WatchInterval)
	}
}

func TestParseWatchIntervalMinimum(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--watch-interval", "100ms"})
	if err == nil {
		t.Fatal("expected error for watch-interval < 500ms, got nil")
	}
}

func TestParseWatchIntervalExactMinimum(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{"--watch-interval", "500ms"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WatchInterval != 500*time.Millisecond {
		t.Errorf("WatchInterval = %v, want 500ms", cfg.WatchInterval)
	}
}

func TestParseCommitDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestParseCommitFlags(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{
		"--commit",
		"--commit-provider", "claude",
		"--commit-model", "claude-sonnet-4-20250514",
		"--commit-auto",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Commit {
		t.Error("Commit = false, want true")
	}
	if cfg.CommitProvider != "claude" {
		t.Errorf("CommitProvider = %q, want %q", cfg.CommitProvider, "claude")
	}
	if cfg.CommitModel != "claude-sonnet-4-20250514" {
		t.Errorf("CommitModel = %q, want %q", cfg.CommitModel, "claude-sonnet-4-20250514")
	}
	if !cfg.CommitAuto {
		t.Error("CommitAuto = false, want true")
	}
}

func TestParseCommitAutoRequiresCommit(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--commit-auto"})
	if err == nil {
		t.Fatal("expected error for --commit-auto without --commit, got nil")
	}
}

func TestParseCommitProviderInvalid(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--commit", "--commit-provider", "unsupported"})
	if err == nil {
		t.Fatal("expected error for unsupported commit provider, got nil")
	}
}

func TestParseCommitProviderInvalidWithoutCommit(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"--commit-provider", "unsupported"})
	if err == nil {
		t.Fatal("expected error for unsupported commit provider without --commit, got nil")
	}
}

// --- worktree dashboard flag tests ---

func TestParseWorktreesDefault(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Worktrees {
		t.Error("Worktrees = true, want false")
	}
}

func TestParseWorktreesFlag(t *testing.T) {
	t.Parallel()

	cfg, err := Parse([]string{"--worktrees"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Worktrees {
		t.Error("Worktrees = false, want true")
	}
}
