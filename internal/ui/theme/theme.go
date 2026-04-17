// Package theme provides semantic color tokens for the scry TUI.
// Colors may use ANSI palette indices or explicit hex values where exact chrome
// matching matters.
package theme

import "github.com/charmbracelet/lipgloss"

// Semantic color tokens mapped to UI roles.
var (
	// Diff semantics.
	Added      = lipgloss.Color("2") // green
	Deleted    = lipgloss.Color("1") // red
	HunkHeader = lipgloss.Color("7") // light gray

	// Status semantics.
	Clean = Added               // green — same hue as diff additions
	Dirty = lipgloss.Color("3") // yellow
	Error = lipgloss.Color("1") // red

	// Chrome.
	Accent         = lipgloss.Color("7")       // soft bright gray
	Muted          = lipgloss.Color("8")       // dim / bright-black
	StatusBg       = lipgloss.Color("236")     // medium-dark gray
	StatusFg       = lipgloss.Color("252")     // light gray
	DividerFg      = lipgloss.Color("240")     // medium gray
	BrightText     = lipgloss.Color("255")     // white
	SelectedBg     = lipgloss.Color("237")     // subtle selected-row background
	ChromeFaint    = lipgloss.Color("239")     // lighter separators/borders
	PaneBorder     = lipgloss.Color("#373e47") // tmux pane border / divider chrome
	InactiveChrome = lipgloss.Color("#768390") // ai-party tmux inactive pane chrome
)
