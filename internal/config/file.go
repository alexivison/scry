package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileConfig represents the structure of a TOML config file.
// Pointer fields distinguish "not set" from zero-value, allowing higher-precedence
// layers to override lower-precedence values (including resetting to defaults).
type FileConfig struct {
	Diff   DiffFileConfig   `toml:"diff"`
	Watch  WatchFileConfig  `toml:"watch"`
	Commit CommitFileConfig `toml:"commit"`
}

// DiffFileConfig holds diff-related config file settings.
type DiffFileConfig struct {
	Mode             *string `toml:"mode"`
	IgnoreWhitespace *bool   `toml:"ignore_whitespace"`
}

// WatchFileConfig holds watch-related config file settings.
type WatchFileConfig struct {
	Interval *string `toml:"interval"`
}

// CommitFileConfig holds commit-related config file settings.
type CommitFileConfig struct {
	Provider *string `toml:"provider"`
	Model    *string `toml:"model"`
}

// LoadFileConfig reads and parses a TOML config file.
// Returns a zero-value FileConfig if the file does not exist.
func LoadFileConfig(path string) (FileConfig, error) {
	var fc FileConfig
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fc, nil
	}
	if err != nil {
		return fc, err
	}
	if _, err := toml.DecodeFile(path, &fc); err != nil {
		return FileConfig{}, err
	}
	return fc, nil
}

// MergeFileConfigs merges user and repo configs. Repo values override user
// values when explicitly set (non-nil pointer). Returns the merged result.
func MergeFileConfigs(user, repo FileConfig) FileConfig {
	merged := user
	if repo.Diff.Mode != nil {
		merged.Diff.Mode = repo.Diff.Mode
	}
	if repo.Diff.IgnoreWhitespace != nil {
		merged.Diff.IgnoreWhitespace = repo.Diff.IgnoreWhitespace
	}
	if repo.Watch.Interval != nil {
		merged.Watch.Interval = repo.Watch.Interval
	}
	if repo.Commit.Provider != nil {
		merged.Commit.Provider = repo.Commit.Provider
	}
	if repo.Commit.Model != nil {
		merged.Commit.Model = repo.Commit.Model
	}
	return merged
}

// repoConfigPath returns the path to .scry.toml at the git repository root.
// Falls back to ".scry.toml" in the current directory if git is unavailable.
func repoConfigPath() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ".scry.toml"
	}
	return filepath.Join(strings.TrimSpace(string(out)), ".scry.toml")
}

// Helper to dereference a *string with a fallback default.
func derefStr(p *string, fallback string) string {
	if p != nil {
		return *p
	}
	return fallback
}

// Helper to dereference a *bool with a fallback default.
func derefBool(p *bool, fallback bool) bool {
	if p != nil {
		return *p
	}
	return fallback
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }
