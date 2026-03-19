// Package terminal provides TTY detection, dimension validation,
// color capability detection, and tmux awareness.
package terminal

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// MinWidth is the minimum terminal width required (below this: "too small" error).
const MinWidth = 40

// MinHeight is the minimum terminal height required (below this: "too small" error).
const MinHeight = 15

// WidthTier classifies the terminal width into layout buckets.
type WidthTier int

const (
	WidthTooSmall    WidthTier = iota // <40
	WidthMinimal                      // 40–59: truncated paths, no gutter
	WidthModalOnly                    // 60–79: modal layout only
	WidthCompactSplit                 // 80–119: compact split
	WidthWideSplit                    // ≥120: wide split
)

// HeightTier classifies the terminal height into layout buckets.
type HeightTier int

const (
	HeightTooSmall      HeightTier = iota // <15
	HeightCompact                         // 15–23: reduced padding
	HeightStandard                        // 24–29: normal
	HeightFooterVisible                   // ≥30: footer visible
)

// LayoutTier returns the width and height tiers for the given terminal dimensions.
func LayoutTier(width, height int) (WidthTier, HeightTier) {
	var wt WidthTier
	switch {
	case width >= 120:
		wt = WidthWideSplit
	case width >= 80:
		wt = WidthCompactSplit
	case width >= 60:
		wt = WidthModalOnly
	case width >= 40:
		wt = WidthMinimal
	default:
		wt = WidthTooSmall
	}

	var ht HeightTier
	switch {
	case height >= 30:
		ht = HeightFooterVisible
	case height >= 24:
		ht = HeightStandard
	case height >= 15:
		ht = HeightCompact
	default:
		ht = HeightTooSmall
	}

	return wt, ht
}

// ColorProfile describes the terminal's color capability.
type ColorProfile int

const (
	ColorNone     ColorProfile = iota // NO_COLOR or dumb terminal
	ColorBasic                        // 16-color ANSI
	ColorANSI256                      // 256-color
	ColorTrueColor                    // 24-bit true color
)

// Env abstracts environment variable access for testability.
type Env struct {
	Getenv   func(string) string
	LookupEnv func(string) (string, bool)
}

// OSEnv returns an Env backed by the real OS environment.
func OSEnv() Env {
	return Env{Getenv: os.Getenv, LookupEnv: os.LookupEnv}
}

// IsTTY reports whether f is connected to a terminal.
func IsTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	return isatty.IsTerminal(f.Fd())
}

// CheckDimensions returns an error if the terminal is smaller than 40x15.
func CheckDimensions(width, height int) error {
	if width < MinWidth || height < MinHeight {
		return fmt.Errorf("terminal too small (%dx%d); scry requires at least %dx%d",
			width, height, MinWidth, MinHeight)
	}
	return nil
}

// DetectColorProfile determines the terminal's color capability.
// Per https://no-color.org/, NO_COLOR set to any value (including empty)
// disables color output.
func DetectColorProfile(env Env) ColorProfile {
	if _, ok := env.LookupEnv("NO_COLOR"); ok {
		return ColorNone
	}
	if ct := env.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		return ColorTrueColor
	}
	if strings.Contains(env.Getenv("TERM"), "256color") {
		return ColorANSI256
	}
	return ColorBasic
}

// IsTmux reports whether the session is running inside tmux.
func IsTmux(env Env) bool {
	return env.Getenv("TMUX") != ""
}

// String returns a human-readable name for the color profile.
func (c ColorProfile) String() string {
	switch c {
	case ColorNone:
		return "none"
	case ColorBasic:
		return "basic"
	case ColorANSI256:
		return "256-color"
	case ColorTrueColor:
		return "truecolor"
	default:
		return fmt.Sprintf("ColorProfile(%d)", int(c))
	}
}
