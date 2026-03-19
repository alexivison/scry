package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/alexivison/scry/internal/model"
)

func TestStatusBar_Segments(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.IgnoreWhitespace = true // enables W badge → creates a second segment → separator appears
	m := NewModel(state)
	m.width = 100
	m.height = 30

	bar := m.viewStatusBar()
	// Should contain dim separator between segments.
	if !strings.Contains(bar, "│") {
		t.Error("status bar should contain │ segment separator")
	}
	// Should contain file count segment.
	if !strings.Contains(bar, "3 files") {
		t.Error("status bar should contain file count")
	}
	// Should contain compare context.
	if !strings.Contains(bar, "abc123...def456") {
		t.Error("status bar should contain compare range")
	}
}

func TestStatusBar_ModeBadges(t *testing.T) {
	t.Parallel()

	t.Run("whitespace badge when active", func(t *testing.T) {
		t.Parallel()
		state := sampleState()
		state.IgnoreWhitespace = true
		m := NewModel(state)
		m.width = 100
		m.height = 30
		bar := m.viewStatusBar()
		if !strings.Contains(bar, "W") {
			t.Error("status bar should show W badge when whitespace ignore is active")
		}
	})

	t.Run("commit badge when enabled", func(t *testing.T) {
		t.Parallel()
		state := sampleState()
		state.CommitEnabled = true
		m := NewModel(state)
		m.width = 100
		m.height = 30
		bar := m.viewStatusBar()
		if !strings.Contains(bar, "C") {
			t.Error("status bar should show C badge when commit is enabled")
		}
	})
}

func TestStatusBar_WatchIndicator(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	m := NewModel(state)
	m.width = 100
	m.height = 30
	m.lastCheckAt = time.Date(2026, 1, 1, 12, 34, 56, 0, time.UTC)

	bar := m.viewStatusBar()
	// Should contain watch interval.
	if !strings.Contains(bar, "2s") {
		t.Errorf("status bar should show watch interval, got: %q", bar)
	}
	// Should contain last check time.
	if !strings.Contains(bar, "12:34:56") {
		t.Errorf("status bar should show last check time, got: %q", bar)
	}
	// Should contain a dot indicator (● for watching).
	if !strings.Contains(bar, "●") {
		t.Errorf("status bar should show watch dot indicator, got: %q", bar)
	}
}

func TestStatusBar_DrillDownBreadcrumb(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.WorktreeMode = true
	state.DashboardState = model.DashboardState{
		DrillDown: true,
		Worktrees: []model.WorktreeInfo{
			{Path: "/project", Branch: "feature-x"},
		},
		SelectedIdx: 0,
	}
	m := NewModel(state)
	m.width = 100
	m.height = 30

	bar := m.viewStatusBar()
	// Breadcrumb should show Dashboard > branch.
	if !strings.Contains(bar, "Dashboard") {
		t.Errorf("drill-down breadcrumb should contain 'Dashboard', got: %q", bar)
	}
	if !strings.Contains(bar, "feature-x") {
		t.Errorf("drill-down breadcrumb should contain branch name, got: %q", bar)
	}
}

func TestStatusBar_WatchErrorRedDot(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	m := NewModel(state)
	m.width = 100
	m.height = 30
	m.watchErr = true

	bar := m.viewStatusBar()
	// Should still show the watch dot (red when error).
	if !strings.Contains(bar, "●") {
		t.Errorf("watch error should show dot indicator, got: %q", bar)
	}
}

func TestStatusBar_RefreshInFlightOverridesWatchErr(t *testing.T) {
	t.Parallel()

	state := sampleState()
	state.WatchEnabled = true
	state.WatchInterval = 2 * time.Second
	state.RefreshInFlight = true
	m := NewModel(state)
	m.width = 100
	m.height = 30
	m.watchErr = true // both error and in-flight set

	bar := m.viewStatusBar()
	// Should show the dot (refresh in-flight takes priority over error).
	if !strings.Contains(bar, "●") {
		t.Errorf("watch segment should show dot when refresh in-flight, got: %q", bar)
	}
}

func TestStatusBar_ErrorStillFullWidth(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 80
	m.height = 30
	m.refreshErr = "refresh failed: timeout"

	bar := m.viewStatusBar()
	if !strings.Contains(bar, "refresh failed: timeout") {
		t.Errorf("error bar should show full error message, got: %q", bar)
	}
	// Error messages should NOT contain segment separators.
	if strings.Contains(bar, "│") {
		t.Error("error bar should not contain segment separators")
	}
}

func TestStatusBar_NarrowTruncation(t *testing.T) {
	t.Parallel()

	m := NewModel(sampleState())
	m.width = 45 // minimal tier
	m.height = 30

	bar := m.viewStatusBar()
	// Should still contain file count.
	if !strings.Contains(bar, "3") {
		t.Errorf("narrow status bar should contain file count, got: %q", bar)
	}
	// Should not crash or be empty.
	if bar == "" {
		t.Error("narrow status bar should not be empty")
	}
}
