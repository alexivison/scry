// Package config handles CLI flag parsing and validation.
package config

import (
	"fmt"
	"time"

	"github.com/alexivison/scry/internal/model"
	flag "github.com/spf13/pflag"
)

// Config is parsed from CLI flags and threaded into app bootstrap.
type Config struct {
	BaseRef          string            // --base; default: "" (resolved to @{upstream})
	HeadRef          string            // --head; default: "" (working tree mode; set to "HEAD" for committed-only)
	Mode             model.CompareMode // --mode; default: CompareThreeDot
	IgnoreWhitespace bool              // --ignore-whitespace; default: false

	// Watch mode (v0.2+).
	Watch         bool          // default: true; --no-watch disables
	WatchInterval time.Duration // --watch-interval; default: 2s, min: 500ms

	// Commit generation (v0.2).
	Commit         bool   // --commit; default: false
	CommitProvider string // --commit-provider; default: "claude"
	CommitModel    string // --commit-model; default: ""
	CommitAuto     bool   // --commit-auto; default: false (requires --commit)

	// Worktree dashboard (v0.2+).
	Worktrees   bool // --worktrees; default: false (kept for backward compat)
	NoDashboard bool // --no-dashboard; forces diff mode even with multiple worktrees
}

// supportedProviders is the set of valid --commit-provider values.
var supportedProviders = map[string]bool{
	"claude": true,
}

// Parse parses CLI args into a Config. Returns an error for unknown flags
// or invalid values (caller should exit with code 2).
func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("scry", flag.ContinueOnError)

	var (
		base           string
		head           string
		mode           string
		ignoreWS       bool
		watch          bool
		noWatch        bool
		watchInterval  time.Duration
		commit         bool
		commitProvider string
		commitModel    string
		commitAuto     bool
		worktrees      bool
		noDashboard    bool
	)

	fs.StringVar(&base, "base", "", "base ref for comparison (default: @{upstream})")
	fs.StringVar(&head, "head", "", "head ref for comparison (default: working tree; use --head HEAD for committed only)")
	fs.StringVar(&mode, "mode", "three-dot", "compare mode: three-dot (default) or two-dot")
	fs.BoolVar(&ignoreWS, "ignore-whitespace", false, "ignore whitespace changes in diffs")
	fs.BoolVar(&watch, "watch", false, "enable watch mode (on by default)")
	fs.BoolVar(&noWatch, "no-watch", false, "disable watch mode")
	fs.DurationVar(&watchInterval, "watch-interval", 2*time.Second, "polling interval for watch mode (min 500ms)")
	fs.BoolVar(&commit, "commit", false, "enable AI commit message generation")
	fs.StringVar(&commitProvider, "commit-provider", "claude", "LLM provider for commit messages (claude)")
	fs.StringVar(&commitModel, "commit-model", "", "override default model for the commit provider")
	fs.BoolVar(&commitAuto, "commit-auto", false, "skip confirmation and commit immediately (requires --commit)")
	fs.BoolVar(&worktrees, "worktrees", false, "show worktree dashboard (auto-detected from worktree count)")
	fs.BoolVar(&noDashboard, "no-dashboard", false, "force diff mode even with multiple worktrees")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected argument: %s", fs.Arg(0))
	}

	cm, err := parseCompareMode(mode)
	if err != nil {
		return Config{}, err
	}

	if watchInterval < 500*time.Millisecond {
		return Config{}, fmt.Errorf("--watch-interval %v is below minimum 500ms", watchInterval)
	}

	if commitAuto && !commit {
		return Config{}, fmt.Errorf("--commit-auto requires --commit")
	}

	if !supportedProviders[commitProvider] {
		return Config{}, fmt.Errorf("unsupported commit provider %q", commitProvider)
	}

	// Watch defaults to on; --no-watch overrides, --watch is a compat no-op.
	resolvedWatch := !noWatch

	return Config{
		BaseRef:          base,
		HeadRef:          head,
		Mode:             cm,
		IgnoreWhitespace: ignoreWS,
		Watch:            resolvedWatch,
		WatchInterval:    watchInterval,
		Commit:           commit,
		CommitProvider:   commitProvider,
		CommitModel:      commitModel,
		CommitAuto:       commitAuto,
		Worktrees:        worktrees,
		NoDashboard:      noDashboard,
	}, nil
}

// ShouldUseDashboard decides whether to enter dashboard mode based on
// worktree count and flag state.
func (c Config) ShouldUseDashboard(worktreeCount int) bool {
	if c.NoDashboard {
		return false
	}
	if c.Worktrees {
		return true
	}
	return worktreeCount > 1
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
