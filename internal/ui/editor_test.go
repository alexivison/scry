package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestKeyOFromPatchUsesCurrentLine(t *testing.T) {
	prevBuilder := buildEditorCommand
	defer func() { buildEditorCommand = prevBuilder }()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var gotPath string
	var gotLine int
	var gotHasLine bool
	buildEditorCommand = func(path string, line int, hasLine bool) *exec.Cmd {
		gotPath = path
		gotLine = line
		gotHasLine = hasLine
		return exec.Command("sh", "-c", "exit 0")
	}

	m := modelWithLoader()
	m.State.Compare.Repo.WorktreeRoot = tmpDir
	m = enterAndLoad(t, m)
	if m.patchViewport == nil {
		t.Fatal("expected patch viewport")
	}
	m.patchViewport.CurrentHunk = 1
	m.patchViewport.ScrollOffset = 4

	updated, cmd := m.Update(keyMsg('o'))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("o should return an ExecProcess cmd")
	}
	for _, msg := range execAndCollect(cmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}

	if gotPath != filePath {
		t.Fatalf("path = %q, want %q", gotPath, filePath)
	}
	if !gotHasLine {
		t.Fatal("expected line targeting from patch view")
	}
	if gotLine != 11 {
		t.Fatalf("line = %d, want 11", gotLine)
	}
}

func TestKeyOMissingFileSetsError(t *testing.T) {
	m := NewModel(sampleState())
	m.width = 100
	m.height = 30
	m.State.Compare.Repo.WorktreeRoot = t.TempDir()
	m.State.Files[m.State.SelectedFile].Path = "missing.go"

	updated, cmd := m.Update(keyMsg('o'))
	m = updated.(Model)

	if cmd != nil {
		t.Fatal("o should not launch editor for a missing file")
	}
	if m.refreshErr == "" {
		t.Fatal("missing file should set a visible error message")
	}
}
