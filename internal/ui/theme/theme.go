// Package theme provides semantic color tokens for the scry TUI.
// All colors use standard ANSI codes so the terminal theme decides actual RGB.
package theme

import "github.com/charmbracelet/lipgloss"

// Semantic color tokens mapped to UI roles.
var (
	// Diff semantics.
	Added      = lipgloss.Color("2") // green
	Deleted    = lipgloss.Color("1") // red
	HunkHeader = lipgloss.Color("6") // cyan

	// Status semantics.
	Clean = Added                    // green — same hue as diff additions
	Dirty = lipgloss.Color("3")     // yellow
	Error = lipgloss.Color("1")     // red

	// Chrome.
	Muted      = lipgloss.Color("8")   // dim / bright-black
	StatusBg   = lipgloss.Color("235") // dark gray
	StatusFg   = lipgloss.Color("252") // light gray
	DividerFg  = lipgloss.Color("240") // medium gray
	BrightText = lipgloss.Color("15")  // white
)
