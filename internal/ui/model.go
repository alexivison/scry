// Package ui implements the Bubble Tea TUI for scry.
package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/diff"
	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/review"
	"github.com/alexivison/scry/internal/search"
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
	patchFallback string // fallback message for binary/submodule/oversized files
	showHelp      bool
	width         int
	height        int
	quitting      bool
	tooSmall      bool // terminal below minimum dimensions
	sizeErr       string

	searchInput    string        // text being typed in search mode
	searchIndex    *search.Index // built when patch is loaded
	searchNotFound string        // "Pattern not found: <query>" message
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
		if m.State.FocusPane == model.PaneSearch {
			return m.updateSearch(msg)
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
	m.patchFallback = ""

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
		m.patchViewport = nil
		m.searchIndex = nil

		var summary model.FileSummary
		if m.State.SelectedFile >= 0 && m.State.SelectedFile < len(m.State.Files) {
			summary = m.State.Files[m.State.SelectedFile]
		}

		if fb := buildFallback(summary, ps.Err); fb != "" {
			m.patchFallback = fb
			m.patchErr = ""
			return
		}

		m.patchErr = ps.Err.Error()
		m.patchFallback = ""
		return
	}
	m.patchErr = ""
	m.patchFallback = ""
	if ps.Patch == nil {
		m.patchViewport = nil
		return
	}
	vp := panes.NewPatchViewport(*ps.Patch)
	vp.Width = m.width
	vp.Height = m.height - 1
	m.patchViewport = vp
	m.searchIndex = search.Build(*ps.Patch)
	m.searchNotFound = ""
}

func isSentinelError(err error) bool {
	return errors.Is(err, model.ErrBinaryFile) ||
		errors.Is(err, model.ErrSubmodule) ||
		errors.Is(err, model.ErrOversized)
}

// buildFallback returns a user-facing fallback message for sentinel errors,
// or "" if the error is not a sentinel. Uses the FileSummary from the metadata
// pipeline (not from PatchService) for full status/path info.
func buildFallback(summary model.FileSummary, err error) string {
	if err == nil {
		return ""
	}

	path := summary.Path
	pathLine := fmt.Sprintf("  Path:   %s", path)
	if summary.OldPath != "" {
		pathLine = fmt.Sprintf("  Path:   %s -> %s", summary.OldPath, summary.Path)
	}
	statusLine := fmt.Sprintf("  Status: %s", summary.Status)

	switch {
	case errors.Is(err, model.ErrBinaryFile):
		return fmt.Sprintf("Binary file -- content not displayed\n\n%s\n%s", pathLine, statusLine)
	case errors.Is(err, model.ErrSubmodule):
		return fmt.Sprintf("Submodule change\n\n%s\n%s", pathLine, statusLine)
	case errors.Is(err, model.ErrOversized):
		var oe *diff.OversizedError
		if errors.As(err, &oe) {
			return fmt.Sprintf("Patch too large to display (%d lines, %d bytes).\nUse `git diff -- %s` to view.\n\n%s\n%s",
				oe.Lines, oe.Bytes, path, pathLine, statusLine)
		}
		return fmt.Sprintf("Patch too large to display.\nUse `git diff -- %s` to view.\n\n%s\n%s", path, pathLine, statusLine)
	default:
		return ""
	}
}

func (m Model) updatePatch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		m.State.FocusPane = model.PaneFiles
		m.patchViewport = nil
		m.patchErr = ""
		m.patchFallback = ""
		m.searchIndex = nil
		m.State.SearchQuery = ""
		m.searchNotFound = ""
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
	case "N":
		m.executeSearch(search.SearchPrev)
	case "enter":
		m.executeSearch(search.SearchNext)
	case "/":
		m.State.FocusPane = model.PaneSearch
		m.searchInput = ""
		m.searchNotFound = ""
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

func (m *Model) executeSearch(dir search.SearchDirection) {
	if m.State.SearchQuery == "" || m.searchIndex == nil || m.patchViewport == nil {
		return
	}

	currentDiff := m.patchViewport.ViewportLineToDiffLine(m.patchViewport.ScrollOffset)
	onHeader := m.patchViewport.IsHunkHeader(m.patchViewport.ScrollOffset)
	var from int
	if dir == search.SearchNext {
		if onHeader {
			from = currentDiff
		} else {
			from = currentDiff + 1
		}
	} else {
		from = currentDiff - 1
	}

	m.searchFrom(from, dir)
}

func (m *Model) searchFrom(from int, dir search.SearchDirection) {
	if m.State.SearchQuery == "" || m.searchIndex == nil || m.patchViewport == nil {
		return
	}

	line, ok := m.searchIndex.Find(m.State.SearchQuery, from, dir)
	if !ok {
		m.searchNotFound = fmt.Sprintf("Pattern not found: %s", m.State.SearchQuery)
		m.patchViewport.SearchQuery = ""
		m.patchViewport.MatchLine = -1
		return
	}

	m.searchNotFound = ""
	vpLine := m.patchViewport.DiffLineToViewportLine(line)
	m.patchViewport.ScrollOffset = vpLine
	m.patchViewport.SyncCurrentHunk()
	m.patchViewport.MatchLine = vpLine
	m.patchViewport.SearchQuery = m.State.SearchQuery
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.State.FocusPane = model.PanePatch
		m.searchInput = ""
	case tea.KeyEnter:
		m.State.FocusPane = model.PanePatch
		if m.searchInput == "" {
			return m, nil
		}
		m.State.SearchQuery = m.searchInput
		m.searchInput = ""
		m.executeSearch(search.SearchNext)
	case tea.KeyBackspace:
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
	case tea.KeyRunes:
		m.searchInput += string(msg.Runes)
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
	} else if m.State.FocusPane == model.PaneSearch || m.State.FocusPane == model.PanePatch {
		b.WriteString(m.viewPatch())
	} else {
		b.WriteString(m.viewFileList())
	}

	b.WriteString("\n")
	if m.State.FocusPane == model.PaneSearch {
		b.WriteString(m.viewSearchInput())
	} else {
		b.WriteString(m.viewStatusBar())
	}

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
	if m.searchNotFound != "" {
		bar := " " + m.searchNotFound
		gap := m.width - lipgloss.Width(bar)
		if gap > 0 {
			bar += strings.Repeat(" ", gap)
		}
		return searchNotFoundStyle.Width(m.width).Render(bar)
	}
	left := fmt.Sprintf(" %s ", m.State.Compare.DiffRange)
	fileCount := fmt.Sprintf(" %d files ", len(m.State.Files))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(fileCount)
	if gap < 0 {
		gap = 0
	}
	bar := left + strings.Repeat(" ", gap) + fileCount
	return statusBarStyle.Width(m.width).Render(bar)
}

func (m Model) viewSearchInput() string {
	prompt := "/" + m.searchInput
	gap := m.width - lipgloss.Width(prompt)
	if gap > 0 {
		prompt += strings.Repeat(" ", gap)
	}
	return statusBarStyle.Width(m.width).Render(prompt)
}

func (m Model) viewPatch() string {
	if m.patchErr != "" {
		return fmt.Sprintf("Error loading patch: %s", m.patchErr)
	}
	if m.patchFallback != "" {
		return m.patchFallback
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
	searchNotFoundStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("1")).
				Foreground(lipgloss.Color("15"))
)
