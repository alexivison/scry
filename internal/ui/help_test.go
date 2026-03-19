package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewHelp_IsOverlay(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 80
	m.height = 24
	m.showHelp = true

	output := m.View()

	// Overlay should contain help sections.
	if !strings.Contains(output, "Navigation") {
		t.Error("help overlay should contain Navigation section")
	}
	if !strings.Contains(output, "Search") {
		t.Error("help overlay should contain Search section")
	}
}

func TestViewHelp_ContainsNewKeys(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 80
	m.height = 24
	m.showHelp = true

	output := m.View()

	for _, key := range []string{"gg", "ctrl+d/u", "ctrl+f/b"} {
		if !strings.Contains(output, key) {
			t.Errorf("help should contain %q", key)
		}
	}
}

func TestGGChord_JumpsToTop(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 80
	m.height = 24
	m.State.SelectedFile = 2

	// Press g then g
	updated, _ := m.Update(keyMsg('g'))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg('g'))
	m = updated.(Model)

	if m.State.SelectedFile != 0 {
		t.Errorf("after gg: SelectedFile = %d, want 0", m.State.SelectedFile)
	}
}

func TestGChord_CancelledByOtherKey(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 80
	m.height = 24
	m.State.SelectedFile = 2

	// g then k: should cancel g and do normal k
	updated, _ := m.Update(keyMsg('g'))
	m = updated.(Model)
	updated, _ = m.Update(keyMsg('k'))
	m = updated.(Model)

	if m.State.SelectedFile != 1 {
		t.Errorf("after g+k: SelectedFile = %d, want 1", m.State.SelectedFile)
	}
}

func TestGShift_JumpsToBottom(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 80
	m.height = 24
	m.State.SelectedFile = 0

	updated, _ := m.Update(keyMsg('G'))
	m = updated.(Model)

	want := len(m.State.Files) - 1
	if m.State.SelectedFile != want {
		t.Errorf("after G: SelectedFile = %d, want %d", m.State.SelectedFile, want)
	}
}

func TestCurlyBraces_JumpHunks(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	m = enterAndLoad(t, m)

	if m.patchViewport == nil {
		t.Fatal("expected non-nil patchViewport")
	}

	initial := m.patchViewport.CurrentHunk

	// } should go to next hunk
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	m = updated.(Model)

	if m.patchViewport.CurrentHunk != initial+1 {
		t.Errorf("after }: CurrentHunk = %d, want %d", m.patchViewport.CurrentHunk, initial+1)
	}

	// { should go back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}})
	m = updated.(Model)

	if m.patchViewport.CurrentHunk != initial {
		t.Errorf("after {: CurrentHunk = %d, want %d", m.patchViewport.CurrentHunk, initial)
	}
}
