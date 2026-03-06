// Package ui implements the Bubble Tea TUI for scry.
package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/review"
	"github.com/alexivison/scry/internal/terminal"
	"github.com/alexivison/scry/internal/ui/panes"
)

// PatchLoader loads a file's unified diff.
type PatchLoader interface {
	LoadPatch(ctx context.Context, cmp model.ResolvedCompare, filePath string, ignoreWhitespace bool) (model.FilePatch, error)
}

// Model is the top-level Bubble Tea model for scry.
type Model struct {
	State         model.AppState
	patchLoader   PatchLoader
	patchViewport *panes.PatchViewport
	patchErr      string
	showHelp      bool
	width         int
	height        int
	quitting      bool
	tooSmall      bool // terminal below minimum dimensions
	sizeErr       string
}

// NewModel creates a Model from bootstrap data. Sets SelectedFile to -1
// when the file list is empty, 0 otherwise.
func NewModel(state model.AppState, opts ...ModelOption) Model {
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
	m := Model{State: state}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// ModelOption configures optional Model dependencies.
type ModelOption func(*Model)

// WithPatchLoader sets the PatchLoader used to load file diffs on Enter.
func WithPatchLoader(pl PatchLoader) ModelOption {
	return func(m *Model) { m.patchLoader = pl }
}

// PatchLoadedMsg is sent when an async patch load completes.
type PatchLoadedMsg struct {
	Path  string
	Patch model.FilePatch
	Gen   int
	Err   error
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

	case PatchLoadedMsg:
		return m.handlePatchLoaded(msg)

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
		if m.State.FocusPane == model.PanePatch {
			return m.updatePatch(msg)
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
			return m.selectFile()
		}
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
	}
	return m, nil
}

// selectFile checks the cache and either uses a cached result or fires an async load.
func (m Model) selectFile() (tea.Model, tea.Cmd) {
	file := m.State.Files[m.State.SelectedFile]
	path := file.Path

	// Cache hit: use cached result directly.
	if ps, hit := review.CacheLookup(m.State, path); hit {
		m.applyPatchResult(ps)
		return m, nil
	}

	if m.patchLoader == nil {
		return m, nil
	}

	// Cache miss: mark loading and fire async Cmd.
	review.MarkLoading(&m.State, path)
	m.patchViewport = nil
	m.patchErr = ""

	gen := m.State.CacheGeneration
	cmp := m.State.Compare
	ignoreWS := m.State.IgnoreWhitespace
	loader := m.patchLoader

	cmd := func() tea.Msg {
		fp, err := loader.LoadPatch(context.Background(), cmp, path, ignoreWS)
		return PatchLoadedMsg{Path: path, Patch: fp, Gen: gen, Err: err}
	}

	return m, cmd
}

func (m Model) handlePatchLoaded(msg PatchLoadedMsg) (tea.Model, tea.Cmd) {
	if review.IsStaleGeneration(msg.Gen, m.State.CacheGeneration) {
		return m, nil
	}

	var patch *model.FilePatch
	if msg.Err == nil {
		patch = &msg.Patch
	}
	review.CacheStore(&m.State, msg.Path, patch, msg.Err)

	// Only update viewport if this message is for the currently selected file.
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		selected := m.State.Files[m.State.SelectedFile].Path
		if selected == msg.Path {
			m.applyPatchResult(m.State.Patches[msg.Path])
		}
	}

	return m, nil
}

// applyPatchResult sets the viewport or error from a PatchLoadState.
func (m *Model) applyPatchResult(ps model.PatchLoadState) {
	if ps.Err != nil {
		m.patchErr = ps.Err.Error()
		m.patchViewport = nil
		return
	}
	m.patchErr = ""
	if ps.Patch == nil {
		m.patchViewport = nil
		return
	}
	vp := panes.NewPatchViewport(*ps.Patch)
	vp.Width = m.width
	vp.Height = m.height - 1
	m.patchViewport = vp
}

func (m Model) updatePatch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		m.State.FocusPane = model.PaneFiles
		m.patchViewport = nil
		m.patchErr = ""
	case "j", "down":
		if m.patchViewport != nil {
			m.patchViewport.ScrollDown()
		}
	case "k", "up":
		if m.patchViewport != nil {
			m.patchViewport.ScrollUp()
		}
	case "n":
		if m.patchViewport != nil {
			m.patchViewport.NextHunk()
		}
	case "p":
		if m.patchViewport != nil {
			m.patchViewport.PrevHunk()
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
	} else if m.State.FocusPane == model.PanePatch {
		b.WriteString(m.viewPatch())
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

func (m Model) viewPatch() string {
	if m.patchErr != "" {
		return fmt.Sprintf("Error loading patch: %s", m.patchErr)
	}

	// Show loading indicator when the selected file is still loading.
	if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
		path := m.State.Files[m.State.SelectedFile].Path
		if ps, ok := m.State.Patches[path]; ok && ps.Status == model.LoadLoading {
			return "Loading..."
		}
	}

	if m.patchViewport == nil {
		return "No patch loaded."
	}
	m.patchViewport.Width = m.width
	m.patchViewport.Height = m.height - 1 // reserve status bar
	return m.patchViewport.Render()
}

func (m Model) viewHelp() string {
	help := []string{
		"Key Bindings",
		"",
		"  j/k     navigate file list",
		"  Enter   select file",
		"  n/p     next/previous hunk",
		"  h/Esc   back to file list",
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
