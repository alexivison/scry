package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/watch"
)

// mockFingerprinter implements WatchFingerprinter for tests.
type mockFingerprinter struct {
	fingerprint string
	err         error
	calls       int
}

func (m *mockFingerprinter) Fingerprint(_ context.Context, _ string, _ bool) (string, error) {
	m.calls++
	return m.fingerprint, m.err
}

func watchState() model.AppState {
	return model.AppState{
		Compare: model.ResolvedCompare{
			BaseRef:     "abc123",
			HeadRef:     "def456",
			DiffRange:   "abc123...def456",
			WorkingTree: true,
		},
		Files:         sampleFiles(),
		SelectedFile:  0,
		FocusPane:     model.PaneFiles,
		Patches:       make(map[string]model.PatchLoadState),
		WatchEnabled:  true,
		WatchInterval: 2 * time.Second,
	}
}

// --- Init tests ---

func TestWatchInitReturnsCmd(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{fingerprint: "abc:def"}
	m := NewModel(watchState(), WithWatch(fp, "origin/main"))

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a non-nil Cmd when watch is enabled")
	}

	// Init returns a batch (spinner tick + watch cmd). Find the fingerprint.
	if !initContainsMsg[watch.FingerprintMsg](t, cmd) {
		t.Fatal("Init should contain a watch.FingerprintMsg cmd")
	}
}

func TestWatchInitNilWhenDisabled(t *testing.T) {
	t.Parallel()

	state := watchState()
	state.WatchEnabled = false
	m := NewModel(state)

	cmd := m.Init()
	if initContainsMsg[watch.FingerprintMsg](t, cmd) {
		t.Fatal("Init should not contain watch cmd when watch is disabled")
	}
}

func TestWatchInitNilWithoutFingerprinter(t *testing.T) {
	t.Parallel()

	// WatchEnabled but no fingerprinter wired.
	m := NewModel(watchState())

	cmd := m.Init()
	if initContainsMsg[watch.FingerprintMsg](t, cmd) {
		t.Fatal("Init should not contain watch cmd when no fingerprinter is set")
	}
}

// initContainsMsg checks if an Init cmd (possibly batched) produces a message of type T.
func initContainsMsg[T any](t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	msg := cmd()
	if _, ok := msg.(T); ok {
		return true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c != nil {
				if _, ok := c().(T); ok {
					return true
				}
			}
		}
	}
	return false
}

// --- TickMsg tests ---

func TestWatchTickMsgReturnsCheckCmd(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{fingerprint: "abc:def"}
	state := watchState()
	state.LastFingerprint = "abc:def" // seeded
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30

	_, cmd := m.Update(watch.TickMsg{At: time.Now()})
	if cmd == nil {
		t.Fatal("TickMsg should return a Cmd for fingerprint check")
	}

	// Execute the cmd and verify it produces a FingerprintMsg.
	msg := cmd()
	fpm, ok := msg.(watch.FingerprintMsg)
	if !ok {
		t.Fatalf("Cmd returned %T, want watch.FingerprintMsg", msg)
	}
	if fpm.Fingerprint != "abc:def" {
		t.Errorf("Fingerprint = %q, want %q", fpm.Fingerprint, "abc:def")
	}
}

func TestWatchTickMsgIgnoredWhenDisabled(t *testing.T) {
	t.Parallel()

	state := watchState()
	state.WatchEnabled = false
	m := NewModel(state)
	m.width = 100
	m.height = 30

	_, cmd := m.Update(watch.TickMsg{At: time.Now()})
	if cmd != nil {
		t.Error("TickMsg should be ignored when watch is disabled")
	}
}

// --- FingerprintMsg tests ---

func TestWatchSeedsFingerprintWithoutRefresh(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	// LastFingerprint is "" — not yet seeded.
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30

	updated, cmd := m.Update(watch.FingerprintMsg{Fingerprint: "initial:fp"})
	um := updated.(Model)

	if um.State.RefreshInFlight {
		t.Error("first fingerprint should seed without triggering refresh")
	}
	if um.State.LastFingerprint != "initial:fp" {
		t.Errorf("LastFingerprint = %q, want %q", um.State.LastFingerprint, "initial:fp")
	}
	// Should schedule first tick after seeding.
	if cmd == nil {
		t.Error("should schedule first tick after seeding")
	}
}

func TestWatchStableFingerprintNoRefresh(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	state.LastFingerprint = "abc:def"
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30

	updated, cmd := m.Update(watch.FingerprintMsg{Fingerprint: "abc:def"})
	um := updated.(Model)

	if um.State.RefreshInFlight {
		t.Error("stable fingerprint should not trigger refresh")
	}
	if um.State.LastFingerprint != "abc:def" {
		t.Errorf("LastFingerprint = %q, want %q", um.State.LastFingerprint, "abc:def")
	}
	// Should still schedule next tick.
	if cmd == nil {
		t.Error("should schedule next tick even on stable fingerprint")
	}
}

func TestWatchChangedFingerprintTriggersRefresh(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	state.LastFingerprint = "old:fp"
	metaLoader := &mockMetadataLoader{files: sampleFiles()}
	m := NewModel(state, WithWatch(fp, "origin/main"), WithMetadataLoader(metaLoader))
	m.width = 100
	m.height = 30

	updated, cmd := m.Update(watch.FingerprintMsg{Fingerprint: "new:fp"})
	um := updated.(Model)

	if !um.State.RefreshInFlight {
		t.Error("changed fingerprint should trigger refresh")
	}
	if um.State.LastFingerprint != "new:fp" {
		t.Errorf("LastFingerprint = %q, want %q", um.State.LastFingerprint, "new:fp")
	}
	// Should return a batched Cmd (refresh + next tick).
	if cmd == nil {
		t.Fatal("should return Cmd for refresh + next tick")
	}
}

func TestWatchNoRefreshWhileInFlight(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	state.LastFingerprint = "old:fp"
	state.RefreshInFlight = true
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30

	updated, cmd := m.Update(watch.FingerprintMsg{Fingerprint: "new:fp"})
	um := updated.(Model)

	// ShouldRefresh returns false when RefreshInFlight is true.
	// LastFingerprint must NOT be updated — preserving the mismatch ensures
	// the change triggers a refresh once the in-flight one completes.
	if um.State.LastFingerprint != "old:fp" {
		t.Errorf("LastFingerprint = %q, want %q (should preserve old value during in-flight)", um.State.LastFingerprint, "old:fp")
	}
	// Should still schedule next tick.
	if cmd == nil {
		t.Error("should schedule next tick even when refresh is in flight")
	}
}

func TestWatchFingerprintErrorSchedulesTick(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	state.LastFingerprint = "old:fp"
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30

	_, cmd := m.Update(watch.FingerprintMsg{Err: fmt.Errorf("git error")})

	// Should still schedule next tick despite error.
	if cmd == nil {
		t.Error("should schedule next tick even on error")
	}
}

func TestWatchFingerprintMsgIgnoredWhenDisabled(t *testing.T) {
	t.Parallel()

	state := watchState()
	state.WatchEnabled = false
	m := NewModel(state)
	m.width = 100
	m.height = 30

	_, cmd := m.Update(watch.FingerprintMsg{Fingerprint: "abc"})
	if cmd != nil {
		t.Error("FingerprintMsg should be ignored when watch is disabled")
	}
}

// --- Help and status tests ---

func TestWatchHelpShowsWatchInfo(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30
	m.showHelp = true

	view := m.View()
	if !strings.Contains(view, "watch") {
		t.Errorf("help should mention watch mode, got:\n%s", view)
	}
}

func TestWatchStatusBarShowsIndicator(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30

	view := m.View()
	// New segmented status bar shows watch dot + interval.
	if !strings.Contains(view, "●") || !strings.Contains(view, "2s") {
		t.Errorf("status bar should show watch dot and interval, got:\n%s", view)
	}
}

func TestWatchStatusBarShowsCheckTime(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{}
	state := watchState()
	state.LastFingerprint = "old:fp"
	m := NewModel(state, WithWatch(fp, "origin/main"))
	m.width = 100
	m.height = 30

	// Send a FingerprintMsg to set lastCheckAt.
	updated, _ := m.Update(watch.FingerprintMsg{Fingerprint: "old:fp"})
	um := updated.(Model)

	view := um.View()
	// After a check, the status bar should include watch dot, interval, and a timestamp.
	if !strings.Contains(view, "●") || !strings.Contains(view, "2s") {
		t.Errorf("status bar should include watch dot and interval after fingerprint, got:\n%s", view)
	}
	// Timestamp should contain a colon (HH:MM:SS format).
	if !strings.Contains(view, ":") {
		t.Errorf("status bar should include check timestamp (HH:MM:SS) after fingerprint, got:\n%s", view)
	}
}

func TestWatchStatusBarHiddenWhenDisabled(t *testing.T) {
	t.Parallel()

	state := watchState()
	state.WatchEnabled = false
	m := NewModel(state)
	m.width = 100
	m.height = 30

	view := m.View()
	if strings.Contains(view, "●") {
		t.Errorf("status bar should not show watch dot when disabled, got:\n%s", view)
	}
}

// --- Bootstrap wiring test ---

func TestWatchBootstrapWiresDependencies(t *testing.T) {
	t.Parallel()

	fp := &mockFingerprinter{fingerprint: "test:fp"}
	state := watchState()
	m := NewModel(state, WithWatch(fp, "origin/main"))

	// Verify fingerprinter is wired.
	if m.fingerprinter == nil {
		t.Fatal("fingerprinter should be set by WithWatch")
	}
	if m.watchBaseRef != "origin/main" {
		t.Errorf("watchBaseRef = %q, want %q", m.watchBaseRef, "origin/main")
	}
}

func TestWatchBootstrapNotWiredWhenDisabled(t *testing.T) {
	t.Parallel()

	state := watchState()
	state.WatchEnabled = false
	m := NewModel(state)

	if m.fingerprinter != nil {
		t.Error("fingerprinter should be nil when watch not wired")
	}
}
