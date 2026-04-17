package ui

import (
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/panes"
)

func TestMainViewsDoNotRenderOuterPaneCorners(t *testing.T) {
	t.Parallel()

	assertNoCorners := func(t *testing.T, view string) {
		t.Helper()
		if strings.ContainsAny(view, "┌┐└┘") {
			t.Fatalf("main browsing view should not contain outer pane corners:\n%s", view)
		}
	}

	t.Run("file list", func(t *testing.T) {
		t.Parallel()
		m := NewModel(sampleState())
		m.width = 100
		m.height = 30
		assertNoCorners(t, m.View())
	})

	t.Run("patch", func(t *testing.T) {
		t.Parallel()
		state := sampleState()
		state.FocusPane = model.PanePatch
		m := NewModel(state)
		m.width = 100
		m.height = 30
		m.patchViewport = panes.NewPatchViewport(samplePatch())
		assertNoCorners(t, m.View())
	})

	t.Run("split", func(t *testing.T) {
		t.Parallel()
		m := NewModel(splitState())
		m.width = 120
		m.height = 30
		m.patchViewport = panes.NewPatchViewport(samplePatch())
		assertNoCorners(t, m.View())
	})

	t.Run("dashboard", func(t *testing.T) {
		t.Parallel()
		m := NewModel(dashboardState())
		m.width = 80
		m.height = 30
		assertNoCorners(t, m.View())
	})

	t.Run("dashboard split", func(t *testing.T) {
		t.Parallel()
		state := dashboardState()
		state.DashboardState.PreviewFiles = previewFiles()
		m := NewModel(state)
		m.width = 120
		m.height = 30
		assertNoCorners(t, m.View())
	})
}
