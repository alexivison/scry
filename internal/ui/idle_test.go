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
