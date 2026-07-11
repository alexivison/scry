package app

import (
	"testing"
	"time"

	"github.com/alexivison/scry/internal/config"
	"github.com/alexivison/scry/internal/model"
)

func TestInitialDiffStateDefaultsToSplitLayout(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		IgnoreWhitespace: true,
		Watch:            true,
		WatchInterval:    3 * time.Second,
		Commit:           true,
		CommitAuto:       true,
		GroupByDirectory: true,
	}
	cmp := model.ResolvedCompare{Basis: model.CompareBasisUpstream}
	files := []model.FileSummary{{Path: "main.go"}}

	state := initialDiffState(cfg, cmp, model.CompareBasisUpstream, files)

	if state.Layout != model.LayoutSplit {
		t.Fatalf("Layout = %q, want %q", state.Layout, model.LayoutSplit)
	}
	if state.FocusPane != model.PaneFiles {
		t.Fatalf("FocusPane = %q, want %q", state.FocusPane, model.PaneFiles)
	}
	if state.PatchDiffMode != model.PatchDiffModeSideBySide {
		t.Fatalf("PatchDiffMode = %v, want %v", state.PatchDiffMode, model.PatchDiffModeSideBySide)
	}
	if state.Patches == nil {
		t.Fatal("Patches map is nil")
	}
}

func TestInitialDashboardStateDefaultsToSplitLayout(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Watch:            true,
		WatchInterval:    3 * time.Second,
		GroupByDirectory: true,
	}

	state := initialDashboardState(cfg)

	if state.Layout != model.LayoutSplit {
		t.Fatalf("Layout = %q, want %q", state.Layout, model.LayoutSplit)
	}
	if state.FocusPane != model.PaneDashboard {
		t.Fatalf("FocusPane = %q, want %q", state.FocusPane, model.PaneDashboard)
	}
	if !state.WorktreeMode {
		t.Fatal("WorktreeMode = false, want true")
	}
	if state.PatchDiffMode != model.PatchDiffModeSideBySide {
		t.Fatalf("PatchDiffMode = %v, want %v", state.PatchDiffMode, model.PatchDiffModeSideBySide)
	}
	if state.Patches == nil {
		t.Fatal("Patches map is nil")
	}
}
