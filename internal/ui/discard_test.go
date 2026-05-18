package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
)

type mockDiscarder struct {
	calls     []discardCall
	returnErr error
}

type discardCall struct {
	path      string
	untracked bool
}

func (d *mockDiscarder) Discard(_ context.Context, path string, untracked bool) error {
	d.calls = append(d.calls, discardCall{path: path, untracked: untracked})
	return d.returnErr
}

// discardState returns a state suitable for testing the discard flow:
// working-tree mode (WorkingTree=true), not in worktree dashboard, files present.
func discardState() model.AppState {
	s := model.AppState{
		Compare:      model.ResolvedCompare{WorkingTree: true, DiffRange: "origin/main"},
		CompareBasis: model.CompareBasisUpstream,
		Files: []model.FileSummary{
			{Path: "main.go", Status: model.StatusModified, Additions: 1, Deletions: 1},
			{Path: "junk.txt", Status: model.StatusUntracked, Additions: 1},
		},
		SelectedFile: 0,
		FocusPane:    model.PaneFiles,
		Patches:      make(map[string]model.PatchLoadState),
		Layout:       model.LayoutModal,
	}
	return s
}

func TestDiscard_TrackedFile_ConfirmExecutesAndRefreshes(t *testing.T) {
	t.Parallel()

	disc := &mockDiscarder{}
	meta := &mockMetadataLoader{files: []model.FileSummary{
		{Path: "junk.txt", Status: model.StatusUntracked, Additions: 1},
	}}
	m := NewModel(discardState(), WithFileDiscarder(disc), WithMetadataLoader(meta))
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)

	if !um.State.ConfirmDiscard {
		t.Fatal("ConfirmDiscard should be true after X")
	}
	if um.State.DiscardPath != "main.go" {
		t.Errorf("DiscardPath = %q, want %q", um.State.DiscardPath, "main.go")
	}
	if um.State.DiscardUntracked {
		t.Error("DiscardUntracked should be false for tracked file")
	}
	view := um.View()
	if !strings.Contains(view, "Discard changes?") {
		t.Errorf("expected discard dialog, got:\n%s", view)
	}
	if !strings.Contains(view, "main.go") {
		t.Errorf("dialog should show file path, got:\n%s", view)
	}

	updated2, cmd := um.Update(keyMsg('y'))
	um2 := updated2.(Model)

	if um2.State.ConfirmDiscard {
		t.Error("ConfirmDiscard should clear after y")
	}
	if !um2.State.DiscardInFlight {
		t.Error("DiscardInFlight should be true while async runs")
	}
	if cmd == nil {
		t.Fatal("expected async discard command")
	}

	msgs := execAndCollect(cmd)
	dm, ok := findMsg[FileDiscardedMsg](msgs)
	if !ok {
		t.Fatalf("expected FileDiscardedMsg in %+v", msgs)
	}
	if len(disc.calls) != 1 || disc.calls[0].path != "main.go" || disc.calls[0].untracked {
		t.Errorf("discarder calls = %+v, want one tracked main.go", disc.calls)
	}
	if dm.Err != nil {
		t.Fatalf("unexpected msg err: %v", dm.Err)
	}

	updated3, refreshCmd := um2.Update(dm)
	um3 := updated3.(Model)
	if um3.State.DiscardInFlight {
		t.Error("DiscardInFlight should be cleared after message")
	}
	if refreshCmd == nil {
		t.Fatal("expected refresh command after successful discard")
	}
}

func TestDiscard_UntrackedFile_FlagsUntracked(t *testing.T) {
	t.Parallel()

	disc := &mockDiscarder{}
	meta := &mockMetadataLoader{}
	state := discardState()
	state.SelectedFile = 1 // junk.txt (untracked)
	m := NewModel(state, WithFileDiscarder(disc), WithMetadataLoader(meta))
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)
	if !um.State.ConfirmDiscard {
		t.Fatal("ConfirmDiscard should be true")
	}
	if !um.State.DiscardUntracked {
		t.Error("DiscardUntracked should be true for untracked file")
	}
	view := um.View()
	if !strings.Contains(view, "(untracked — file will be deleted)") {
		t.Errorf("expected untracked warning, got:\n%s", view)
	}

	updated2, cmd := um.Update(keyMsg('y'))
	_ = updated2
	if cmd == nil {
		t.Fatal("expected async command")
	}
	msgs := execAndCollect(cmd)
	if _, ok := findMsg[FileDiscardedMsg](msgs); !ok {
		t.Fatalf("expected FileDiscardedMsg in %+v", msgs)
	}
	if len(disc.calls) != 1 || !disc.calls[0].untracked {
		t.Errorf("expected one untracked discard call, got %+v", disc.calls)
	}
}

func TestDiscard_CancelWithN(t *testing.T) {
	t.Parallel()

	disc := &mockDiscarder{}
	m := NewModel(discardState(), WithFileDiscarder(disc))
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)
	if !um.State.ConfirmDiscard {
		t.Fatal("should be in confirm state")
	}

	updated2, cmd := um.Update(keyMsg('n'))
	um2 := updated2.(Model)

	if um2.State.ConfirmDiscard {
		t.Error("n should cancel discard confirmation")
	}
	if cmd != nil {
		t.Error("cancel should not return a command")
	}
	if len(disc.calls) != 0 {
		t.Error("discarder should not have been called on cancel")
	}
}

func TestDiscard_CancelWithEsc(t *testing.T) {
	t.Parallel()

	disc := &mockDiscarder{}
	m := NewModel(discardState(), WithFileDiscarder(disc))
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)
	updated2, _ := um.Update(tea.KeyMsg{Type: tea.KeyEscape})
	um2 := updated2.(Model)
	if um2.State.ConfirmDiscard {
		t.Error("Esc should cancel discard confirmation")
	}
}

func TestDiscard_ErrorSurfaces(t *testing.T) {
	t.Parallel()

	disc := &mockDiscarder{returnErr: fmt.Errorf("git boom")}
	m := NewModel(discardState(), WithFileDiscarder(disc), WithMetadataLoader(&mockMetadataLoader{}))
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)
	updated2, cmd := um.Update(keyMsg('y'))
	um2 := updated2.(Model)
	if cmd == nil {
		t.Fatal("expected command")
	}
	msgs := execAndCollect(cmd)
	dm, ok := findMsg[FileDiscardedMsg](msgs)
	if !ok {
		t.Fatalf("expected FileDiscardedMsg in %+v", msgs)
	}
	updated3, _ := um2.Update(dm)
	um3 := updated3.(Model)

	if um3.State.DiscardErr == "" {
		t.Error("DiscardErr should be populated on failure")
	}
	view := um3.View()
	if !strings.Contains(view, "discard failed") {
		t.Errorf("expected discard error in view, got:\n%s", view)
	}
}

func TestDiscard_WithoutDiscarder_NoOp(t *testing.T) {
	t.Parallel()

	m := NewModel(discardState())
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)
	if um.State.ConfirmDiscard {
		t.Error("ConfirmDiscard should remain false when no discarder configured")
	}
}

func TestDiscard_BlockedInWorktreeMode(t *testing.T) {
	t.Parallel()

	disc := &mockDiscarder{}
	state := discardState()
	state.WorktreeMode = true
	state.FocusPane = model.PaneDashboard
	m := NewModel(state, WithFileDiscarder(disc))
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)
	if um.State.ConfirmDiscard {
		t.Error("X should not trigger discard in worktree dashboard")
	}
}

func TestDiscard_InFlightDrivesSpinner(t *testing.T) {
	t.Parallel()

	state := discardState()
	state.DiscardInFlight = true
	m := NewModel(state)
	if !m.needsSpinner() {
		t.Error("needsSpinner should return true while DiscardInFlight")
	}
}

func TestDiscard_BlockedWhenNotWorkingTree(t *testing.T) {
	t.Parallel()

	disc := &mockDiscarder{}
	state := discardState()
	state.Compare.WorkingTree = false
	m := NewModel(state, WithFileDiscarder(disc))
	m.width = 80
	m.height = 30

	updated, _ := m.Update(keyMsg('X'))
	um := updated.(Model)
	if um.State.ConfirmDiscard {
		t.Error("X should not trigger discard when not in working-tree mode")
	}
}
