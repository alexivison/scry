// Package config handles CLI flag parsing and validation.
package config

import (
	"fmt"

	"github.com/alexivison/scry/internal/model"
	flag "github.com/spf13/pflag"
)

// Config is parsed from CLI flags and threaded into app bootstrap.
type Config struct {
	BaseRef          string           // --base; default: "" (resolved to @{upstream})
	HeadRef          string           // --head; default: "" (resolved to HEAD)
	Mode             model.CompareMode // --mode; default: CompareThreeDot
	IgnoreWhitespace bool             // --ignore-whitespace; default: false
}

// Parse parses CLI args into a Config. Returns an error for unknown flags
// or invalid values (caller should exit with code 2).
func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("scry", flag.ContinueOnError)

	var (
		base       string
		head       string
		mode       string
		ignoreWS   bool
	)

	fs.StringVar(&base, "base", "", "base ref for comparison (default: @{upstream})")
	fs.StringVar(&head, "head", "", "head ref for comparison (default: HEAD)")
	fs.StringVar(&mode, "mode", "three-dot", "compare mode: three-dot (default) or two-dot")
	fs.BoolVar(&ignoreWS, "ignore-whitespace", false, "ignore whitespace changes in diffs")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cm, err := parseCompareMode(mode)
	if err != nil {
		return Config{}, err
	}

	return Config{
		BaseRef:          base,
		HeadRef:          head,
		Mode:             cm,
		IgnoreWhitespace: ignoreWS,
	}, nil
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
