package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/watch"
)

// idleState returns a base AppState for idle screen tests:
// watch enabled, no files, PaneIdle focus.
func idleState() model.AppState {
	return model.AppState{
		Compare: model.ResolvedCompare{
			BaseRef:     "abc123",
			HeadRef:     "def456",
			DiffRange:   "abc123...def456",
			WorkingTree: true,
		},
		Files:         nil,
		SelectedFile:  -1,
		FocusPane:     model.PaneIdle,
		Patches:       make(map[string]model.PatchLoadState),
		WatchEnabled:  true,
		WatchInterval: 2 * time.Second,
	}
}

// --- View content tests ---

func TestIdleViewShowsWatchingMessage(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "Watching") {
		t.Errorf("idle view should contain 'Watching', got:\n%s", view)
	}
}

func TestIdleViewShowsBaseRef(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "abc123") {
		t.Errorf("idle view should show base ref, got:\n%s", view)
	}
}

func TestIdleViewShowsKeyHints(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "quit") {
		t.Errorf("idle view should show quit hint, got:\n%s", view)
	}
}

// --- Transition tests ---

func TestIdleTransitionOnDivergence(t *testing.T) {
	t.Parallel()
	fp := &mockFingerprinter{}
	metaLoader := &mockMetadataLoader{files: sampleFiles()}
	state := idleState()
	state.LastFingerprint = "old:fp"
	m := NewModel(state, WithWatch(fp, "origin/main"), WithMetadataLoader(metaLoader))
	m.width = 80
	m.height = 24

	// Changed fingerprint triggers refresh.
	updated, _ := m.Update(watch.FingerprintMsg{Fingerprint: "new:fp"})
	um := updated.(Model)

	// Simulate MetadataLoadedMsg arriving with files.
	updated2, _ := um.Update(MetadataLoadedMsg{
		Files: sampleFiles(),
		Gen:   um.State.CacheGeneration,
	})
	um2 := updated2.(Model)

	if um2.State.FocusPane != model.PaneFiles {
		t.Errorf("FocusPane = %q, want %q after divergence detected", um2.State.FocusPane, model.PaneFiles)
	}
	if len(um2.State.Files) != 3 {
		t.Errorf("Files = %d, want 3", len(um2.State.Files))
	}
}

func TestIdleFirstFingerprintTriggersRefresh(t *testing.T) {
	t.Parallel()
	fp := &mockFingerprinter{}
	metaLoader := &mockMetadataLoader{files: sampleFiles()}
	state := idleState()
	// LastFingerprint is "" — not yet seeded. In idle mode, the first
	// fingerprint should trigger a refresh to catch changes between
	// bootstrap and the first fingerprint check.
	m := NewModel(state, WithWatch(fp, "origin/main"), WithMetadataLoader(metaLoader))
	m.width = 80
	m.height = 24

	updated, cmd := m.Update(watch.FingerprintMsg{Fingerprint: "first:fp"})
	um := updated.(Model)

	if !um.State.RefreshInFlight {
		t.Error("first fingerprint in idle should trigger refresh")
	}
	if um.State.LastFingerprint != "first:fp" {
		t.Errorf("LastFingerprint = %q, want %q", um.State.LastFingerprint, "first:fp")
	}
	if cmd == nil {
		t.Fatal("should return cmd for refresh + tick")
	}
}

func TestIdleStaysIdleOnStableFingerprint(t *testing.T) {
	t.Parallel()
	fp := &mockFingerprinter{}
	state := idleState()
	state.LastFingerprint = "stable:fp"
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 80
	m.height = 24

	updated, _ := m.Update(watch.FingerprintMsg{Fingerprint: "stable:fp"})
	um := updated.(Model)

	if um.State.FocusPane != model.PaneIdle {
		t.Errorf("FocusPane = %q, want %q (no change in fingerprint)", um.State.FocusPane, model.PaneIdle)
	}
}

func TestIdleNoTransitionOnEmptyMetadata(t *testing.T) {
	t.Parallel()
	metaLoader := &mockMetadataLoader{files: nil}
	m := NewModel(idleState(), WithMetadataLoader(metaLoader))
	m.width = 80
	m.height = 24

	// Metadata arrives but still empty — should stay idle.
	updated, _ := m.Update(MetadataLoadedMsg{
		Files: nil,
		Gen:   m.State.CacheGeneration,
	})
	um := updated.(Model)

	if um.State.FocusPane != model.PaneIdle {
		t.Errorf("FocusPane = %q, want %q (no files yet)", um.State.FocusPane, model.PaneIdle)
	}
}

// --- Key handling tests ---

func TestIdleQuitKey(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	updated, cmd := m.Update(keyMsg('q'))
	um := updated.(Model)
	if !um.quitting {
		t.Error("q should set quitting")
	}
	if cmd == nil {
		t.Error("q should return tea.Quit cmd")
	}
}

func TestIdleHelpKey(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	updated, _ := m.Update(keyMsg('?'))
	um := updated.(Model)
	if !um.showHelp {
		t.Error("? should show help")
	}
}

func TestIdleCommitKeyIgnored(t *testing.T) {
	t.Parallel()
	state := idleState()
	state.CommitEnabled = true
	m := NewModel(state, WithCommitProvider(&mockCommitProvider{message: "test"}))
	m.width = 80
	m.height = 24

	updated, cmd := m.Update(keyMsg('c'))
	um := updated.(Model)
	if cmd != nil {
		t.Error("c in idle should not fire any cmd")
	}
	if um.State.FocusPane != model.PaneIdle {
		t.Errorf("FocusPane = %q, want %q (should stay idle)", um.State.FocusPane, model.PaneIdle)
	}
}

func TestIdleNoPatchLoadOnEnter(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	updated, cmd := m.Update(enterMsg())
	um := updated.(Model)
	if cmd != nil {
		t.Error("Enter in idle should not fire any cmd")
	}
	if um.State.FocusPane != model.PaneIdle {
		t.Errorf("FocusPane = %q, want %q (should stay idle)", um.State.FocusPane, model.PaneIdle)
	}
}

// --- V3-T14: Idle screen polish tests ---

func TestIdleViewHasBorderedBox(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	// Bordered idle box uses rounded corners from lipgloss.
	if !strings.Contains(view, "╭") || !strings.Contains(view, "╯") {
		t.Errorf("idle view should use rounded border (╭/╯), got:\n%s", view)
	}
}

func TestIdleViewCenteredVertically(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 40

	view := m.View()
	lines := strings.Split(view, "\n")
	// Find first line with border top — should not be line 0 (centered).
	firstBorder := -1
	for i, l := range lines {
		if strings.Contains(l, "╭") {
			firstBorder = i
			break
		}
	}
	if firstBorder <= 0 {
		t.Errorf("idle box should be vertically centered (first border at line %d), got:\n%s", firstBorder, view)
	}
}

func TestIdleViewPulseIndicator(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	// Pulse indicator: either ◉ (filled) or ○ (hollow).
	hasFilled := strings.Contains(view, "◉")
	hasHollow := strings.Contains(view, "○")
	if !hasFilled && !hasHollow {
		t.Errorf("idle view should show pulse indicator (◉ or ○), got:\n%s", view)
	}
}

func TestIdleViewShowsInterval(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "2s") {
		t.Errorf("idle view should show interval '2s', got:\n%s", view)
	}
}

func TestIdleViewShowsLastCheckTime(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24
	m.lastCheckAt = time.Date(2026, 3, 19, 14, 30, 45, 0, time.UTC)

	view := m.View()
	if !strings.Contains(view, "14:30:45") {
		t.Errorf("idle view should show last check time, got:\n%s", view)
	}
}

func TestIdleViewShowsNoDivergenceStatus(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "No divergence") {
		t.Errorf("idle view should show 'No divergence' status, got:\n%s", view)
	}
}

func TestIdleViewShowsDiffRange(t *testing.T) {
	t.Parallel()
	state := idleState()
	state.Compare.WorkingTree = false
	m := NewModel(state)
	m.width = 80
	m.height = 24

	view := m.View()
	if !strings.Contains(view, "abc123...def456") {
		t.Errorf("idle view should show diff range for non-worktree, got:\n%s", view)
	}
}

func TestIdleViewAdaptsToNarrowWidth(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 45
	m.height = 24

	view := m.View()
	// Should still render without panic and contain key info.
	if !strings.Contains(view, "abc123") {
		t.Errorf("narrow idle view should still show base ref, got:\n%s", view)
	}
}

func TestIdleViewAdaptsToCompactHeight(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 16

	view := m.View()
	// Should still render essential info at compact height.
	if !strings.Contains(view, "Watching") || !strings.Contains(view, "abc123") {
		t.Errorf("compact idle view should show essential info, got:\n%s", view)
	}
}

func TestIdleViewStyledKeyBadges(t *testing.T) {
	t.Parallel()
	m := NewModel(idleState())
	m.width = 80
	m.height = 24

	view := m.View()
	// Key hints should contain action labels.
	if !strings.Contains(view, "q") || !strings.Contains(view, "quit") {
		t.Errorf("idle view should show styled key badges with actions, got:\n%s", view)
	}
	if !strings.Contains(view, "?") || !strings.Contains(view, "help") {
		t.Errorf("idle view should show help key badge, got:\n%s", view)
	}
	if !strings.Contains(view, "r") || !strings.Contains(view, "refresh") {
		t.Errorf("idle view should show refresh key badge, got:\n%s", view)
	}
}

func TestIdlePulseTogglesOnWatchFingerprint(t *testing.T) {
	t.Parallel()
	fp := &mockFingerprinter{fingerprint: "fp1"}
	state := idleState()
	state.LastFingerprint = "fp0"
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 80
	m.height = 24

	pulseBefore := m.idlePulse

	// Simulate a fingerprint message (watch tick result).
	updated, _ := m.Update(watch.FingerprintMsg{Fingerprint: "fp0"})
	um := updated.(Model)

	if um.idlePulse == pulseBefore {
		t.Error("idlePulse should toggle on watch fingerprint message")
	}

	// Second fingerprint toggles back.
	updated2, _ := um.Update(watch.FingerprintMsg{Fingerprint: "fp0"})
	um2 := updated2.(Model)
	if um2.idlePulse != pulseBefore {
		t.Error("idlePulse should toggle back on second fingerprint")
	}
}

// --- Init test ---

func TestIdleInitReturnsWatchCmd(t *testing.T) {
	t.Parallel()
	fp := &mockFingerprinter{fingerprint: "abc:def"}
	m := NewModel(idleState(), WithWatch(fp, "origin/main"))

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return non-nil cmd when watch is enabled in idle mode")
	}
	if !initContainsMsg[watch.FingerprintMsg](t, cmd) {
		t.Fatal("Init should contain a watch.FingerprintMsg cmd")
	}
}
