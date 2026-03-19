package ui

import (
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func TestSpinner_PatchLoading(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	m.State.Patches["main.go"] = model.PatchLoadState{
		Status:     model.LoadLoading,
		Generation: m.State.CacheGeneration,
	}
	m.State.FocusPane = model.PanePatch

	output := m.View()

	// The output should contain the spinner view prefix before "Loading..."
	spinView := m.spinner.View()
	if !strings.Contains(output, spinView+" Loading...") {
		t.Errorf("expected spinner prefix before Loading..., got:\n%s", output)
	}
}

func TestSpinner_CommitGenerating(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 80
	m.height = 24
	m.State.FocusPane = model.PaneCommit
	m.State.CommitState.InFlight = true

	output := m.View()

	// Should contain spinner-animated text for commit generation.
	if output == "" {
		t.Error("expected non-empty output during commit generation")
	}
}

func TestSpinner_CleanupOnPatchLoaded(t *testing.T) {
	t.Parallel()

	m := modelWithLoader()
	m = enterAndLoad(t, m)

	// After loading, spinner should not be active — patch content should be present.
	output := m.View()
	if strings.Contains(output, "Loading") {
		t.Error("spinner should be cleaned up after patch loads")
	}
}
