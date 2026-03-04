package config

import (
	"testing"

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
