package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

// --- mock commit provider ---

type mockCommitProvider struct {
	message string
	err     error
	calls   int
}

func (m *mockCommitProvider) Generate(_ context.Context) (string, error) {
	m.calls++
	return m.message, m.err
}

// commitState returns a state with commit mode enabled.
func commitState() model.AppState {
	s := sampleState()
	s.CommitEnabled = true
	return s
}

// --- c key binding tests ---

func TestCommitUI_CKeyTriggersGenerationWhenEnabled(t *testing.T) {
	t.Parallel()

	provider := &mockCommitProvider{message: "feat: add feature"}
	m := NewModel(commitState(), WithCommitProvider(provider))
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "c")

	if m.State.FocusPane != model.PaneCommit {
		t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneCommit)
	}
	if !m.State.CommitState.InFlight {
		t.Error("CommitState.InFlight = false, want true")
	}
	if cmd == nil {
		t.Fatal("expected async Cmd for commit generation, got nil")
	}
}

func TestCommitUI_CKeyIgnoredWhenDisabled(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState()) // CommitEnabled defaults to false
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "c")

	if m.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q (should not change)", m.State.FocusPane, model.PaneFiles)
	}
	if m.State.CommitState.InFlight {
		t.Error("CommitState.InFlight = true, want false")
	}
	if cmd != nil {
		t.Error("expected nil Cmd when commit disabled")
	}
}

func TestCommitUI_CKeyIgnoredWhenNoProvider(t *testing.T) {
	t.Parallel()

	s := commitState()
	m := NewModel(s) // CommitEnabled=true but no provider set
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "c")

	if m.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneFiles)
	}
	if cmd != nil {
		t.Error("expected nil Cmd when no provider")
	}
}

func TestCommitUI_CKeyIgnoredFromPatchPane(t *testing.T) {
	t.Parallel()

	provider := &mockCommitProvider{message: "feat: add feature"}
	s := commitState()
	s.FocusPane = model.PanePatch
	m := NewModel(s, WithCommitProvider(provider))
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "c")

	// c should not be handled in patch pane — only from file list
	if m.State.FocusPane == model.PaneCommit {
		t.Error("c should not trigger commit from patch pane")
	}
}

// --- in-flight view tests ---

func TestCommitUI_InFlightStatusShown(t *testing.T) {
	t.Parallel()

	s := commitState()
	s.FocusPane = model.PaneCommit
	s.CommitState.InFlight = true
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "Generating") {
		t.Errorf("View should show generating indicator, got:\n%s", view)
	}
}

// --- CommitGeneratedMsg handling ---

func TestCommitUI_GeneratedMessageDisplayed(t *testing.T) {
	t.Parallel()

	s := commitState()
	s.FocusPane = model.PaneCommit
	s.CommitState.InFlight = true
	s.CommitState.Generation = 1
	m := NewModel(s)
	m.width = 100
	m.height = 30

	msg := CommitGeneratedMsg{Message: "feat: add structured logging", Generation: 1}
	result, _ := m.Update(msg)
	um := result.(Model)

	if um.State.CommitState.InFlight {
		t.Error("InFlight should be false after generation completes")
	}
	if um.State.CommitState.GeneratedMessage != "feat: add structured logging" {
		t.Errorf("GeneratedMessage = %q, want %q", um.State.CommitState.GeneratedMessage, "feat: add structured logging")
	}
	if um.State.CommitState.Err != nil {
		t.Errorf("Err = %v, want nil", um.State.CommitState.Err)
	}

	view := um.View()
	if !strings.Contains(view, "feat: add structured logging") {
		t.Errorf("View should display generated message, got:\n%s", view)
	}
}

func TestCommitUI_ErrorDisplayed(t *testing.T) {
	t.Parallel()

	s := commitState()
	s.FocusPane = model.PaneCommit
	s.CommitState.InFlight = true
	s.CommitState.Generation = 1
	m := NewModel(s)
	m.width = 100
	m.height = 30

	msg := CommitGeneratedMsg{Err: fmt.Errorf("API key invalid"), Generation: 1}
	result, _ := m.Update(msg)
	um := result.(Model)

	if um.State.CommitState.InFlight {
		t.Error("InFlight should be false after error")
	}
	if um.State.CommitState.Err == nil {
		t.Fatal("Err should be set after generation error")
	}
	if um.State.CommitState.GeneratedMessage != "" {
		t.Errorf("GeneratedMessage = %q, want empty", um.State.CommitState.GeneratedMessage)
	}

	view := um.View()
	if !strings.Contains(view, "API key invalid") {
		t.Errorf("View should display error, got:\n%s", view)
	}
}

// --- Esc cancel tests ---

func TestCommitUI_EscCancelsWithNoSideEffects(t *testing.T) {
	t.Parallel()

	s := commitState()
	s.FocusPane = model.PaneCommit
	s.CommitState.InFlight = true
	m := NewModel(s)
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "esc")

	if m.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneFiles)
	}
	if m.State.CommitState.InFlight {
		t.Error("InFlight should be false after Esc")
	}
	if m.State.CommitState.GeneratedMessage != "" {
		t.Errorf("GeneratedMessage = %q, want empty after cancel", m.State.CommitState.GeneratedMessage)
	}
	if m.State.CommitState.Err != nil {
		t.Errorf("Err = %v, want nil after cancel", m.State.CommitState.Err)
	}
}

func TestCommitUI_EscFromResultClearsMessage(t *testing.T) {
	t.Parallel()

	s := commitState()
	s.FocusPane = model.PaneCommit
	s.CommitState.GeneratedMessage = "feat: something"
	m := NewModel(s)
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "esc")

	if m.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneFiles)
	}
	if m.State.CommitState.GeneratedMessage != "" {
		t.Errorf("GeneratedMessage = %q, want empty after Esc", m.State.CommitState.GeneratedMessage)
	}
}

// --- r regenerate tests ---

func TestCommitUI_RRegeneratesFromCommitPane(t *testing.T) {
	t.Parallel()

	provider := &mockCommitProvider{message: "fix: update handler"}
	s := commitState()
	s.FocusPane = model.PaneCommit
	s.CommitState.GeneratedMessage = "feat: old message"
	m := NewModel(s, WithCommitProvider(provider))
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "r")

	if !m.State.CommitState.InFlight {
		t.Error("InFlight should be true after regenerate")
	}
	if m.State.CommitState.GeneratedMessage != "" {
		t.Errorf("GeneratedMessage = %q, want empty during regeneration", m.State.CommitState.GeneratedMessage)
	}
	if m.State.CommitState.Err != nil {
		t.Errorf("Err = %v, want nil during regeneration", m.State.CommitState.Err)
	}
	if cmd == nil {
		t.Fatal("expected async Cmd for regeneration, got nil")
	}
	// Should stay in commit pane
	if m.State.FocusPane != model.PaneCommit {
		t.Errorf("FocusPane = %q, want %q", m.State.FocusPane, model.PaneCommit)
	}
}

func TestCommitUI_RFromCommitPaneDoesNotRefresh(t *testing.T) {
	t.Parallel()

	// r in commit pane should regenerate, not trigger a metadata refresh.
	provider := &mockCommitProvider{message: "feat: test"}
	s := commitState()
	s.FocusPane = model.PaneCommit
	s.CommitState.GeneratedMessage = "old"
	m := NewModel(s, WithCommitProvider(provider))
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "r")

	// If refresh was triggered, RefreshInFlight would be true
	if m.State.RefreshInFlight {
		t.Error("r in commit pane should regenerate, not trigger refresh")
	}
}

// --- q quit from commit pane ---

func TestCommitUI_QQuitsFromCommitPane(t *testing.T) {
	t.Parallel()

	s := commitState()
	s.FocusPane = model.PaneCommit
	m := NewModel(s)
	m.width = 100
	m.height = 30

	m, _ = sendKey(m, "q")

	if !m.quitting {
		t.Error("q in commit pane should quit")
	}
}

// --- help text tests ---

func TestCommitUI_HelpIncludesCommitBinding(t *testing.T) {
	t.Parallel()

	s := commitState()
	m := NewModel(s)
	m.width = 100
	m.height = 30
	m.showHelp = true

	view := m.View()
	if !strings.Contains(view, "c") || !strings.Contains(view, "commit") {
		t.Errorf("help text should mention c/commit when commit mode enabled, got:\n%s", view)
	}
}

func TestCommitUI_HelpOmitsCommitBindingWhenDisabled(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState()) // CommitEnabled = false
	m.width = 100
	m.height = 30
	m.showHelp = true

	view := m.View()
	// Should not mention commit key binding
	if strings.Contains(view, "generate commit") {
		t.Errorf("help should not mention commit when disabled, got:\n%s", view)
	}
}

// --- status bar tests ---

func TestCommitUI_StatusBarShowsCommitIndicator(t *testing.T) {
	t.Parallel()

	s := commitState()
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "[C]") {
		t.Errorf("status bar should contain [C] when commit enabled, got:\n%s", view)
	}
}

func TestCommitUI_StatusBarOmitsCommitIndicatorWhenDisabled(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState()) // CommitEnabled = false
	m.width = 100
	m.height = 30

	view := m.View()
	if strings.Contains(view, "[C]") {
		t.Errorf("status bar should not contain [C] when commit disabled, got:\n%s", view)
	}
}

// --- stale result guard tests ---

func TestCommitUI_StaleResultAfterEscIsDiscarded(t *testing.T) {
	t.Parallel()

	provider := &mockCommitProvider{message: "feat: stale"}
	s := commitState()
	m := NewModel(s, WithCommitProvider(provider))
	m.width = 100
	m.height = 30

	// Trigger generation (gen=1).
	m, _ = sendKey(m, "c")
	gen := m.State.CommitState.Generation

	// Cancel via Esc (bumps generation counter to invalidate in-flight goroutine).
	m, _ = sendKey(m, "esc")

	// Stale result arrives from the cancelled generation.
	staleMsg := CommitGeneratedMsg{Message: "feat: stale", Generation: gen}
	result, _ := m.Update(staleMsg)
	um := result.(Model)

	if um.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q", um.State.FocusPane, model.PaneFiles)
	}
	if um.State.CommitState.GeneratedMessage != "" {
		t.Errorf("GeneratedMessage = %q, want empty (stale result should be discarded)", um.State.CommitState.GeneratedMessage)
	}
	if um.State.CommitState.InFlight {
		t.Error("InFlight should remain false after stale result")
	}
}

func TestCommitUI_StaleResultAfterCancelAndRestart(t *testing.T) {
	t.Parallel()

	provider := &mockCommitProvider{message: "feat: fresh"}
	s := commitState()
	m := NewModel(s, WithCommitProvider(provider))
	m.width = 100
	m.height = 30

	// First generation (gen=1).
	m, _ = sendKey(m, "c")
	firstGen := m.State.CommitState.Generation

	// Cancel and restart (gen=2).
	m, _ = sendKey(m, "esc")
	m, _ = sendKey(m, "c")
	secondGen := m.State.CommitState.Generation

	if secondGen <= firstGen {
		t.Fatalf("second generation %d should be > first %d", secondGen, firstGen)
	}

	// Stale result from first generation arrives — must be discarded.
	staleMsg := CommitGeneratedMsg{Message: "feat: stale", Generation: firstGen}
	result, _ := m.Update(staleMsg)
	um := result.(Model)

	if !um.State.CommitState.InFlight {
		t.Error("InFlight should remain true (second generation still in flight)")
	}
	if um.State.CommitState.GeneratedMessage != "" {
		t.Errorf("GeneratedMessage = %q, want empty (stale result discarded)", um.State.CommitState.GeneratedMessage)
	}
}
