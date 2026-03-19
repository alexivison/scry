// Package config handles CLI flag parsing, TOML config loading, and validation.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alexivison/scry/internal/model"
	flag "github.com/spf13/pflag"
)

// Config is the resolved configuration threaded into app bootstrap.
type Config struct {
	BaseRef          string            // --base
	HeadRef          string            // --head
	Mode             model.CompareMode // from config file; default: CompareThreeDot
	IgnoreWhitespace bool              // from config file; default: false

	Watch         bool          // default: true; --no-watch disables
	WatchInterval time.Duration // from config file; default: 2s, min: 500ms

	Commit         bool   // --commit
	CommitProvider string // from config file; default: "claude"
	CommitModel    string // from config file; default: ""
	CommitAuto     bool   // --commit-auto (requires --commit)

	NoDashboard      bool // --no-dashboard; forces diff mode even with multiple worktrees
	GroupByDirectory bool // from config file; default: false
}

// supportedProviders is the set of valid commit provider values.
var supportedProviders = map[string]bool{
	"claude": true,
}

// ParseOption configures Parse behavior.
type ParseOption func(*parseOpts)

type parseOpts struct {
	userConfigPath string
	repoConfigPath string
}

// WithConfigPaths overrides the default config file paths (for testing).
func WithConfigPaths(userPath, repoPath string) ParseOption {
	return func(o *parseOpts) {
		o.userConfigPath = userPath
		o.repoConfigPath = repoPath
	}
}

// Parse parses CLI args and merges with TOML config files.
// Precedence: CLI flag > repo .scry.toml > user ~/.config/scry/config.toml > defaults.
func Parse(args []string, opts ...ParseOption) (Config, error) {
	o := parseOpts{}
	for _, fn := range opts {
		fn(&o)
	}
	fs := flag.NewFlagSet("scry", flag.ContinueOnError)

	var (
		base        string
		head        string
		noWatch     bool
		commit      bool
		commitAuto  bool
		noDashboard bool
	)

	fs.StringVar(&base, "base", "", "base ref for comparison (default: @{upstream})")
	fs.StringVar(&head, "head", "", "head ref for comparison (default: working tree; use --head HEAD for committed only)")
	fs.BoolVar(&noWatch, "no-watch", false, "disable watch mode")
	fs.BoolVar(&commit, "commit", false, "enable AI commit message generation")
	fs.BoolVar(&commitAuto, "commit-auto", false, "skip confirmation and commit immediately (requires --commit)")
	fs.BoolVar(&noDashboard, "no-dashboard", false, "force diff mode even with multiple worktrees")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected argument: %s", fs.Arg(0))
	}

	if commitAuto && !commit {
		return Config{}, fmt.Errorf("--commit-auto requires --commit")
	}

	// Load config files: user then repo.
	fileConfig, err := loadAndMergeConfigs(o)
	if err != nil {
		return Config{}, err
	}

	// Apply file config with defaults.
	modeStr := derefStr(fileConfig.Diff.Mode, "three-dot")
	cm, err := parseCompareMode(modeStr)
	if err != nil {
		return Config{}, err
	}

	watchInterval := 2 * time.Second
	if fileConfig.Watch.Interval != nil {
		d, err := time.ParseDuration(*fileConfig.Watch.Interval)
		if err != nil {
			return Config{}, fmt.Errorf("invalid watch.interval %q: %w", *fileConfig.Watch.Interval, err)
		}
		if d < 500*time.Millisecond {
			return Config{}, fmt.Errorf("watch.interval %v is below minimum 500ms", d)
		}
		watchInterval = d
	}

	commitProvider := derefStr(fileConfig.Commit.Provider, "claude")
	if !supportedProviders[commitProvider] {
		return Config{}, fmt.Errorf("unsupported commit provider %q", commitProvider)
	}

	return Config{
		BaseRef:          base,
		HeadRef:          head,
		Mode:             cm,
		IgnoreWhitespace: derefBool(fileConfig.Diff.IgnoreWhitespace, false),
		Watch:            !noWatch,
		WatchInterval:    watchInterval,
		Commit:           commit,
		CommitProvider:   commitProvider,
		CommitModel:      derefStr(fileConfig.Commit.Model, ""),
		CommitAuto:       commitAuto,
		NoDashboard:      noDashboard,
		GroupByDirectory: derefBool(fileConfig.FileList.GroupByDirectory, false),
	}, nil
}

// ShouldUseDashboard decides whether to enter dashboard mode based on
// worktree count and flag state.
func (c Config) ShouldUseDashboard(worktreeCount int) bool {
	if c.NoDashboard {
		return false
	}
	return worktreeCount > 1
}

// loadAndMergeConfigs loads user and repo TOML configs and merges them.
func loadAndMergeConfigs(o parseOpts) (FileConfig, error) {
	up := o.userConfigPath
	if up == "" {
		up = userConfigPath()
	}
	userConfig, err := LoadFileConfig(up)
	if err != nil {
		return FileConfig{}, fmt.Errorf("user config %s: %w", up, err)
	}

	rp := o.repoConfigPath
	if rp == "" {
		rp = repoConfigPath()
	}
	repoConfig, err := LoadFileConfig(rp)
	if err != nil {
		return FileConfig{}, fmt.Errorf("repo config %s: %w", rp, err)
	}

	return MergeFileConfigs(userConfig, repoConfig), nil
}

// userConfigPath returns the path to the user config file.
func userConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "scry", "config.toml")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "scry", "config.toml")
}

func parseCompareMode(s string) (model.CompareMode, error) {
	switch model.CompareMode(s) {
	case model.CompareThreeDot:
		return model.CompareThreeDot, nil
	case model.CompareTwoDot:
		return model.CompareTwoDot, nil
	default:
		return "", fmt.Errorf("invalid compare mode %q: must be %q or %q", s, model.CompareThreeDot, model.CompareTwoDot)
	}
}
