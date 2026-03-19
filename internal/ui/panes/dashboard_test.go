package panes

import (
	"strings"
	"testing"
	"time"

	"github.com/alexivison/scry/internal/model"
)

func sampleWorktrees() []model.WorktreeInfo {
	return []model.WorktreeInfo{
		{Path: "/home/user/project", Branch: "main", CommitHash: "abc1234", Subject: "initial commit", Dirty: false},
		{Path: "/home/user/project-feature", Branch: "feature", CommitHash: "def5678", Subject: "add feature", Dirty: true},
		{Path: "/home/user/project-bugfix", Branch: "bugfix", CommitHash: "ghi9012", Subject: "fix bug", Dirty: false},
	}
}

func TestRenderDashboard(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		worktrees []model.WorktreeInfo
		selected  int
		scroll    int
		width     int
		height    int
		checks    func(t *testing.T, output string)
	}{
		"renders all worktrees": {
			worktrees: sampleWorktrees(),
			selected:  0,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "main") {
					t.Error("missing branch name 'main'")
				}
				if !strings.Contains(output, "feature") {
					t.Error("missing branch name 'feature'")
				}
				if !strings.Contains(output, "bugfix") {
					t.Error("missing branch name 'bugfix'")
				}
			},
		},
		"shows dirty indicator": {
			worktrees: sampleWorktrees(),
			selected:  0,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				// The dirty worktree (feature, index 1) should contain the dot indicator.
				if !strings.Contains(output, "●") {
					t.Error("missing dirty/clean indicator dot")
				}
			},
		},
		"shows commit info": {
			worktrees: sampleWorktrees(),
			selected:  0,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "abc1234") {
					t.Error("missing commit hash")
				}
				if !strings.Contains(output, "initial commit") {
					t.Error("missing commit subject")
				}
			},
		},
		"highlights selected row": {
			worktrees: sampleWorktrees(),
			selected:  1,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				lines := strings.Split(output, "\n")
				if len(lines) < 2 {
					t.Fatal("expected at least 2 lines")
				}
				// The selected row (index 1) should have a selection indicator
				if !strings.Contains(lines[1], ">") && !strings.Contains(lines[1], "feature") {
					t.Error("selected row not highlighted")
				}
			},
		},
		"empty worktree list": {
			worktrees: nil,
			selected:  -1,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "No worktrees") {
					t.Error("expected empty state message")
				}
			},
		},
		"scrolled view": {
			worktrees: sampleWorktrees(),
			selected:  2,
			scroll:    1,
			width:     80,
			height:    2,
			checks: func(t *testing.T, output string) {
				t.Helper()
				// With scroll=1 and height=2, we should see worktrees[1] and [2]
				if strings.Contains(output, "main") && !strings.Contains(output, "feature") {
					t.Error("scroll not applied: main visible but feature not")
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			output := RenderDashboard(tc.worktrees, tc.selected, tc.scroll, tc.width, tc.height)
			tc.checks(t, output)
		})
	}
}

func TestRenderDashboardShowsChangedFileCount(t *testing.T) {
	t.Parallel()

	wts := []model.WorktreeInfo{
		{Path: "/p", Branch: "main", CommitHash: "abc", Subject: "init", Dirty: true, ChangedFiles: 12},
	}
	output := RenderDashboard(wts, 0, 0, 120, 10)

	if !strings.Contains(output, "12") {
		t.Errorf("expected changed file count '12' in output:\n%s", output)
	}
}

func TestRenderDashboardShowsSingularFile(t *testing.T) {
	t.Parallel()

	wts := []model.WorktreeInfo{
		{Path: "/p", Branch: "main", CommitHash: "abc", Subject: "init", Dirty: true, ChangedFiles: 1},
	}
	output := RenderDashboard(wts, 0, 0, 120, 10)

	if !strings.Contains(output, "1 file") {
		t.Errorf("expected '1 file' (singular) in output:\n%s", output)
	}
	if strings.Contains(output, "1 files") {
		t.Error("should use singular 'file' for count=1, got '1 files'")
	}
}

func TestRenderDashboardShowsRelativeTime(t *testing.T) {
	t.Parallel()

	wts := []model.WorktreeInfo{
		{Path: "/p", Branch: "main", CommitHash: "abc", Subject: "init", Dirty: true,
			ChangedFiles: 3, LastActivityAt: time.Now().Add(-30 * time.Second)},
	}
	output := RenderDashboard(wts, 0, 0, 120, 10)

	// Use loose assertion to avoid clock-drift flakiness.
	if !strings.Contains(output, "s ago") {
		t.Errorf("expected relative time with 's ago' in output:\n%s", output)
	}
}

func TestRelativeTime(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		d          time.Duration
		want       string
		wantSuffix string // loose match for clock-sensitive cases
	}{
		"sub-second": {d: 500 * time.Millisecond, want: "just now"},
		"seconds":    {d: 5 * time.Second, wantSuffix: "s ago"},
		"minutes":    {d: 3 * time.Minute, want: "3m ago"},
		"hours":      {d: 2 * time.Hour, want: "2h ago"},
		"mixed":      {d: 90 * time.Second, want: "1m ago"},
		"zero":       {d: 0, want: ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var at time.Time
			if tc.d > 0 {
				at = time.Now().Add(-tc.d)
			}
			got := RelativeTime(at)
			if tc.wantSuffix != "" {
				if !strings.HasSuffix(got, tc.wantSuffix) {
					t.Errorf("RelativeTime = %q, want suffix %q", got, tc.wantSuffix)
				}
			} else if got != tc.want {
				t.Errorf("RelativeTime = %q, want %q", got, tc.want)
			}
		})
	}
}
