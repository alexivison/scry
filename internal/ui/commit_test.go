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
	message    string
	err        error
	calls      int
	generateFn func(ctx context.Context) (string, error) // optional override
}

func (m *mockCommitProvider) Generate(ctx context.Context) (string, error) {
	m.calls++
	if m.generateFn != nil {
		return m.generateFn(ctx)
	}
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
	if !strings.Contains(view, "C") {
		t.Errorf("status bar should contain C badge when commit enabled, got:\n%s", view)
	}
}

func TestCommitUI_StatusBarShowsBadgesWhenDisabled(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState()) // CommitEnabled = false
	m.width = 100
	m.height = 30

	view := m.View()
	// C badge is always present (dim when disabled, bright when enabled).
	if !strings.Contains(view, "C") {
		t.Errorf("status bar should always show C badge (dim when disabled), got:\n%s", view)
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

// --- Commit Execution tests (V2-T8) ---

type mockCommitExecutor struct {
	sha string
	err error
}

func (m *mockCommitExecutor) Execute(_ context.Context, _ string) (string, error) {
	return m.sha, m.err
}

func commitReadyState() model.AppState {
	s := sampleState()
	s.CommitEnabled = true
	s.FocusPane = model.PaneCommit
	s.CommitState = model.CommitState{
		GeneratedMessage: "feat: add feature",
		Generation:       1,
	}
	return s
}

func TestCommitExecution_enterExecutesCommit(t *testing.T) {
	t.Parallel()

	executor := &mockCommitExecutor{sha: "abc1234"}
	m := NewModel(commitReadyState(), WithCommitExecutor(executor))
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "enter")

	if !m.State.CommitState.Executing {
		t.Error("Executing should be true after Enter")
	}
	if cmd == nil {
		t.Fatal("expected a command to be returned for async commit execution")
	}

	// Execute the command (possibly batched with spinner tick) and feed results back.
	um := deepDrain(t, m, cmd)

	if um.State.CommitState.Executing {
		t.Error("Executing should be false after completion")
	}
	if um.State.CommitState.CommitSHA != "abc1234" {
		t.Errorf("CommitSHA = %q, want %q", um.State.CommitState.CommitSHA, "abc1234")
	}
}

func TestCommitExecution_errorShowsStderr(t *testing.T) {
	t.Parallel()

	executor := &mockCommitExecutor{err: fmt.Errorf("nothing to commit")}
	m := NewModel(commitReadyState(), WithCommitExecutor(executor))
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "enter")
	um := deepDrain(t, m, cmd)

	if um.State.CommitState.CommitErr == nil {
		t.Fatal("CommitErr should be set on failure")
	}
	view := um.View()
	if !strings.Contains(view, "nothing to commit") {
		t.Errorf("view should contain error message, got:\n%s", view)
	}
}

func TestCommitExecution_enterRetryAfterError(t *testing.T) {
	t.Parallel()

	// First attempt fails, second succeeds.
	executor := &mockCommitExecutor{err: fmt.Errorf("hook rejected")}
	s := commitReadyState()
	m := NewModel(s, WithCommitExecutor(executor))
	m.width = 100
	m.height = 30

	// Execute → fail.
	m, cmd := sendKey(m, "enter")
	m = deepDrain(t, m, cmd)

	if m.State.CommitState.CommitErr == nil {
		t.Fatal("CommitErr should be set after first failure")
	}

	// Fix executor for retry.
	executor.err = nil
	executor.sha = "retry1"

	// Press Enter again to retry.
	m, cmd = sendKey(m, "enter")
	if !m.State.CommitState.Executing {
		t.Error("Executing should be true after retry Enter")
	}
	if cmd == nil {
		t.Fatal("expected command for retry execution")
	}

	m = deepDrain(t, m, cmd)

	if m.State.CommitState.CommitSHA != "retry1" {
		t.Errorf("CommitSHA = %q, want %q after retry", m.State.CommitState.CommitSHA, "retry1")
	}
}

func TestCommitExecution_autoCommitSkipsConfirmation(t *testing.T) {
	t.Parallel()

	executor := &mockCommitExecutor{sha: "auto123"}
	provider := &mockCommitProvider{message: "feat: auto commit"}

	s := sampleState()
	s.CommitEnabled = true
	s.CommitAuto = true

	m := NewModel(s, WithCommitProvider(provider), WithCommitExecutor(executor))
	m.width = 100
	m.height = 30

	// Press "c" to start generation.
	m, cmd := sendKey(m, "c")
	if cmd == nil {
		t.Fatal("expected generation command")
	}

	// Complete generation — with CommitAuto, should auto-fire execution.
	// The cmd may be batched with spinner tick. Use deepDrain to follow
	// the full chain: generation → CommitGeneratedMsg → execution → CommitExecutedMsg.
	um := deepDrain(t, m, cmd)

	if um.State.CommitState.CommitSHA != "auto123" {
		t.Errorf("CommitAuto: CommitSHA = %q, want %q", um.State.CommitState.CommitSHA, "auto123")
	}
}

func TestCommitExecution_postCommitRefresh(t *testing.T) {
	t.Parallel()

	executor := &mockCommitExecutor{sha: "ref1234"}
	metaLoader := &mockMetadataLoader{files: sampleFiles()}

	s := commitReadyState()
	m := NewModel(s, WithCommitExecutor(executor), WithMetadataLoader(metaLoader))
	m.width = 100
	m.height = 30

	genBefore := m.State.CacheGeneration

	// Enter → execute → complete (handles BatchMsg from spinner).
	m, cmd := sendKey(m, "enter")
	um := deepDrain(t, m, cmd)

	if um.State.CacheGeneration <= genBefore {
		t.Error("CacheGeneration should bump after successful commit (refresh)")
	}
}

func TestCommitExecution_editKeyOpensEditor(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 100
	m.height = 30

	m, cmd := sendKey(m, "e")

	// "e" in commit pane with a generated message should return a command.
	if cmd == nil {
		t.Error("expected a command for editor handoff on 'e' key")
	}
}

func TestCommitExecution_editedMessageUpdatesState(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 100
	m.height = 30

	editMsg := CommitEditedMsg{Message: "fix: edited message"}
	result, _ := m.Update(editMsg)
	um := result.(Model)

	if um.State.CommitState.GeneratedMessage != "fix: edited message" {
		t.Errorf("GeneratedMessage = %q, want %q", um.State.CommitState.GeneratedMessage, "fix: edited message")
	}
}

func TestCommitExecution_editorErrorClearsMessage(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 100
	m.height = 30

	if m.State.CommitState.GeneratedMessage == "" {
		t.Fatal("precondition: GeneratedMessage should be non-empty")
	}

	editMsg := CommitEditedMsg{Err: fmt.Errorf("editor crashed")}
	result, _ := m.Update(editMsg)
	um := result.(Model)

	if um.State.CommitState.GeneratedMessage != "" {
		t.Errorf("GeneratedMessage = %q, want empty after editor error", um.State.CommitState.GeneratedMessage)
	}
	if um.State.CommitState.Err == nil {
		t.Error("Err should be set after editor error")
	}
}

func TestCommitUI_EscCancelsContext(t *testing.T) {
	t.Parallel()

	var generateCtx context.Context
	provider := &mockCommitProvider{
		generateFn: func(ctx context.Context) (string, error) {
			generateCtx = ctx
			return "msg", nil
		},
	}
	s := commitState()
	m := NewModel(s, WithCommitProvider(provider))
	m.width = 100
	m.height = 30

	// Start generation — this stores a cancel func.
	m, cmd := sendKey(m, "c")
	if cmd == nil {
		t.Fatal("expected a command from commit generation")
	}

	// Execute the command (may be batched with spinner tick) to capture the context.
	msgs := execAndCollect(cmd)
	_ = msgs

	if generateCtx == nil {
		t.Fatal("generateCtx should have been set by the provider")
	}

	// Esc should cancel the context.
	m, _ = sendKey(m, "esc")
	if generateCtx.Err() == nil {
		t.Error("context should be cancelled after Esc")
	}
}

// --- V3-T14: Commit screen polish tests ---

func TestCommitUI_GeneratedMessageInBorderedArea(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	// Commit message should be inside a bordered area (rounded border).
	if !strings.Contains(view, "╭") || !strings.Contains(view, "╯") {
		t.Errorf("commit view should use bordered area for message, got:\n%s", view)
	}
	if !strings.Contains(view, "feat: add feature") {
		t.Errorf("commit view should show generated message, got:\n%s", view)
	}
}

func TestCommitUI_ActionKeyBadgesStyled(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	// Action hints: Enter, e, r, Esc should be present with labels.
	for _, action := range []string{"Enter", "commit", "edit", "regenerate", "Esc", "cancel"} {
		if !strings.Contains(view, action) {
			t.Errorf("commit view should show action %q, got:\n%s", action, view)
		}
	}
}

func TestCommitUI_SuccessViewBordered(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	s.CommitState.CommitSHA = "abc1234"
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "abc1234") {
		t.Errorf("commit success view should show SHA, got:\n%s", view)
	}
	if !strings.Contains(view, "╭") {
		t.Errorf("commit success view should be bordered, got:\n%s", view)
	}
}

func TestCommitUI_ErrorViewBordered(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	s.CommitState.Err = fmt.Errorf("API error")
	s.CommitState.GeneratedMessage = ""
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "API error") {
		t.Errorf("commit error view should show error, got:\n%s", view)
	}
	if !strings.Contains(view, "╭") {
		t.Errorf("commit error view should be bordered, got:\n%s", view)
	}
}

func TestCommitUI_NarrowWidthAdapts(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 50
	m.height = 24

	view := m.View()
	// Should render without panic at narrow width.
	if !strings.Contains(view, "feat: add feature") {
		t.Errorf("narrow commit view should still show message, got:\n%s", view)
	}
}

func TestCommitUI_NarrowWidthWrapsBadges(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 45 // narrow — badges must wrap to multiple lines
	m.height = 24

	view := m.View()
	// All action labels should still be visible (wrapped, not truncated).
	for _, action := range []string{"Enter", "commit", "Esc", "cancel"} {
		if !strings.Contains(view, action) {
			t.Errorf("narrow commit view should show action %q (wrapped), got:\n%s", action, view)
		}
	}
}

func TestCommitUI_CompactHeightAdapts(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	m := NewModel(s)
	m.width = 100
	m.height = 16

	view := m.View()
	if !strings.Contains(view, "feat: add feature") {
		t.Errorf("compact commit view should show message, got:\n%s", view)
	}
}

func TestCommitExecution_viewShowsSuccessSHA(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	s.CommitState.CommitSHA = "abc1234"
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "abc1234") {
		t.Errorf("view should show committed SHA, got:\n%s", view)
	}
}

func TestCommitExecution_viewShowsCommitError(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	s.CommitState.CommitErr = fmt.Errorf("pre-commit hook failed")
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "pre-commit hook failed") {
		t.Errorf("view should show commit error, got:\n%s", view)
	}
}

func TestCommitExecution_executingShowsProgress(t *testing.T) {
	t.Parallel()

	s := commitReadyState()
	s.CommitState.Executing = true
	m := NewModel(s)
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "ommit") {
		t.Errorf("view should indicate commit in progress, got:\n%s", view)
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
