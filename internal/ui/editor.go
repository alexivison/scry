package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
)

type editorClosedMsg struct {
	err error
}

var buildEditorCommand = func(path string, line int, hasLine bool) *exec.Cmd {
	args := make([]string, 0, 3)
	if hasLine && line > 0 {
		args = append(args, fmt.Sprintf("+%d", line))
	}
	args = append(args, "--", path)
	return exec.Command("nvim", args...)
}

func (m Model) openInNeovim() (tea.Model, tea.Cmd) {
	file, ok := m.selectedEditorFile()
	if !ok {
		return m, nil
	}

	path := file.Path
	if file.Status == model.StatusDeleted && file.OldPath != "" {
		path = file.OldPath
	} else if path == "" {
		path = file.OldPath
	}
	if path == "" {
		m.refreshErr = "Cannot open file: no worktree path"
		return m, nil
	}

	absPath := path
	if !filepath.IsAbs(absPath) {
		root := m.State.Compare.Repo.WorktreeRoot
		if root == "" {
			root = "."
		}
		absPath = filepath.Join(root, filepath.FromSlash(path))
	}
	if _, err := os.Stat(absPath); err != nil {
		m.refreshErr = fmt.Sprintf("Cannot open missing file: %s", path)
		return m, nil
	}

	line, hasLine := 0, false
	if m.State.FocusPane == model.PanePatch && m.patchViewport != nil {
		line, hasLine = m.patchViewport.CurrentTargetLine()
	}

	cmd := buildEditorCommand(absPath, line, hasLine)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorClosedMsg{err: err}
	})
}

func (m Model) selectedEditorFile() (model.FileSummary, bool) {
	if m.State.SelectedFile < 0 || m.State.SelectedFile >= len(m.State.Files) {
		return model.FileSummary{}, false
	}
	return m.State.Files[m.State.SelectedFile], true
}
