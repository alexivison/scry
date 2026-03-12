package model

import "testing"

func TestAppStateZeroValues(t *testing.T) {
	t.Parallel()

	var s AppState

	// Watch-mode fields exist and zero-initialize correctly.
	if s.WatchEnabled {
		t.Error("WatchEnabled zero value = true, want false")
	}
	if s.WatchInterval != 0 {
		t.Errorf("WatchInterval zero value = %v, want 0", s.WatchInterval)
	}
	if s.LastFingerprint != "" {
		t.Errorf("LastFingerprint zero value = %q, want empty", s.LastFingerprint)
	}
	if s.RefreshInFlight {
		t.Error("RefreshInFlight zero value = true, want false")
	}
	if !s.LastRefreshAt.IsZero() {
		t.Errorf("LastRefreshAt zero value = %v, want zero", s.LastRefreshAt)
	}

	// CommitState embedded struct exists and zero-initializes.
	if s.CommitState.InFlight {
		t.Error("CommitState.InFlight zero value = true, want false")
	}
	if s.CommitState.GeneratedMessage != "" {
		t.Errorf("CommitState.GeneratedMessage zero value = %q, want empty", s.CommitState.GeneratedMessage)
	}
}
