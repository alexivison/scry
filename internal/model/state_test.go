package model

import "testing"

func TestAppStateZeroValues(t *testing.T) {
	t.Parallel()

	var s AppState

	// Refresh state zero-initializes correctly.
	if s.RefreshInFlight {
		t.Error("RefreshInFlight zero value = true, want false")
	}

	// CommitState embedded struct exists and zero-initializes.
	if s.CommitState.InFlight {
		t.Error("CommitState.InFlight zero value = true, want false")
	}
	if s.CommitState.GeneratedMessage != "" {
		t.Errorf("CommitState.GeneratedMessage zero value = %q, want empty", s.CommitState.GeneratedMessage)
	}
}
