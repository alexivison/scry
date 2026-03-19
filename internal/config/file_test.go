package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileConfig_Valid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[diff]
mode = "two-dot"
ignore_whitespace = true

[watch]
interval = "5s"

[commit]
provider = "claude"
model = "claude-sonnet-4-20250514"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	fc, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Diff.Mode == nil || *fc.Diff.Mode != "two-dot" {
		t.Errorf("Diff.Mode = %v, want %q", fc.Diff.Mode, "two-dot")
	}
	if fc.Diff.IgnoreWhitespace == nil || !*fc.Diff.IgnoreWhitespace {
		t.Error("Diff.IgnoreWhitespace should be true")
	}
	if fc.Watch.Interval == nil || *fc.Watch.Interval != "5s" {
		t.Errorf("Watch.Interval = %v, want %q", fc.Watch.Interval, "5s")
	}
	if fc.Commit.Provider == nil || *fc.Commit.Provider != "claude" {
		t.Errorf("Commit.Provider = %v, want %q", fc.Commit.Provider, "claude")
	}
	if fc.Commit.Model == nil || *fc.Commit.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Commit.Model = %v, want %q", fc.Commit.Model, "claude-sonnet-4-20250514")
	}
}

func TestLoadFileConfig_MissingFile(t *testing.T) {
	t.Parallel()

	fc, err := LoadFileConfig("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	// Should return zero-value config.
	if fc.Diff.Mode != nil {
		t.Errorf("Diff.Mode = %v, want nil", fc.Diff.Mode)
	}
}

func TestLoadFileConfig_Malformed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("not valid toml [[["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFileConfig(path)
	if err == nil {
		t.Fatal("expected error for malformed TOML, got nil")
	}
}

func TestLoadFileConfig_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	fc, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fc.Diff.Mode != nil || fc.Diff.IgnoreWhitespace != nil || fc.Watch.Interval != nil {
		t.Error("empty file should produce zero-value FileConfig")
	}
}
