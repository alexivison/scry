// Package terminal provides TTY detection, dimension validation,
// color capability detection, and tmux awareness.
package terminal

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// MinWidth is the minimum terminal width required.
const MinWidth = 80

// MinHeight is the minimum terminal height required.
const MinHeight = 24

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

// CheckDimensions returns an error if the terminal is smaller than 80x24.
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
