// Package ui implements the Bubble Tea TUI for scry.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/terminal"
)

// Model is the top-level Bubble Tea model for scry.
type Model struct {
	State    model.AppState
	showHelp bool
	width    int
	height   int
	quitting bool
	tooSmall bool // terminal below minimum dimensions
	sizeErr  string
}

// NewModel creates a Model from bootstrap data. Sets SelectedFile to -1
// when the file list is empty, 0 otherwise.
func NewModel(state model.AppState) Model {
	if len(state.Files) == 0 {
		state.SelectedFile = -1
	} else {
		if state.SelectedFile < 0 {
			state.SelectedFile = 0
		}
		if state.SelectedFile >= len(state.Files) {
			state.SelectedFile = len(state.Files) - 1
		}
	}
	if state.FocusPane == "" {
		state.FocusPane = model.PaneFiles
	}
	if state.Patches == nil {
		state.Patches = make(map[string]model.PatchLoadState)
	}
	return Model{State: state}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if err := terminal.CheckDimensions(msg.Width, msg.Height); err != nil {
			m.tooSmall = true
			m.sizeErr = err.Error()
		} else {
			m.tooSmall = false
			m.sizeErr = ""
		}
		return m, nil

	case tea.KeyMsg:
		if m.tooSmall {
			if msg.String() == "q" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}
		if m.showHelp {
			return m.updateHelp(msg)
		}
		return m.updateFiles(msg)
	}
	return m, nil
}

func (m Model) updateFiles(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.State.SelectedFile < len(m.State.Files)-1 {
			m.State.SelectedFile++
		}
	case "k", "up":
		if m.State.SelectedFile > 0 {
			m.State.SelectedFile--
		}
	case "enter":
		if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
			m.State.FocusPane = model.PanePatch
		}
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
	}
	return m, nil
}

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?", "esc":
		m.showHelp = false
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}
	if m.tooSmall {
		return m.sizeErr + "\nPress q to quit."
	}

	var b strings.Builder

	if m.showHelp {
		b.WriteString(m.viewHelp())
	} else {
		b.WriteString(m.viewFileList())
	}

	b.WriteString("\n")
	b.WriteString(m.viewStatusBar())

	return b.String()
}

func (m Model) viewFileList() string {
	if len(m.State.Files) == 0 {
		return "No files changed."
	}

	var lines []string
	for i, f := range m.State.Files {
		line := m.renderFileLine(i, f)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFileLine(idx int, f model.FileSummary) string {
	status := statusIcon(f.Status)
	path := f.Path
	if f.OldPath != "" {
		path = fmt.Sprintf("%s → %s", f.OldPath, f.Path)
	}

	counts := formatCounts(f)
	prefix := "  "
	if idx == m.State.SelectedFile {
		prefix = "> "
	}
	line := fmt.Sprintf("%s%s  %-40s %s", prefix, status, path, counts)

	if idx == m.State.SelectedFile {
		return selectedStyle.Render(line)
	}
	return line
}

func statusIcon(s model.FileStatus) string {
	switch s {
	case model.StatusAdded:
		return "A"
	case model.StatusModified:
		return "M"
	case model.StatusDeleted:
		return "D"
	case model.StatusRenamed:
		return "R"
	case model.StatusCopied:
		return "C"
	case model.StatusTypeChg:
		return "T"
	case model.StatusUnmerged:
		return "U"
	default:
		return "?"
	}
}

func formatCounts(f model.FileSummary) string {
	if f.IsBinary {
		return "binary"
	}
	return fmt.Sprintf("+%d -%d", f.Additions, f.Deletions)
}

func (m Model) viewStatusBar() string {
	left := fmt.Sprintf(" %s ", m.State.Compare.DiffRange)
	fileCount := fmt.Sprintf(" %d files ", len(m.State.Files))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(fileCount)
	if gap < 0 {
		gap = 0
	}
	bar := left + strings.Repeat(" ", gap) + fileCount
	return statusBarStyle.Width(m.width).Render(bar)
}

func (m Model) viewHelp() string {
	help := []string{
		"Key Bindings",
		"",
		"  j/k     navigate file list",
		"  Enter   select file",
		"  q       quit",
		"  ?       toggle help",
	}
	return strings.Join(help, "\n")
}

// Styles — kept minimal; will degrade gracefully when color is unavailable.
var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	statusBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252"))
)
