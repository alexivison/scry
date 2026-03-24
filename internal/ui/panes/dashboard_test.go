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
		{Path: "/home/user/project-feature", Branch: "feature", CommitHash: "def5678", Subject: "add feature", Dirty: true, ChangedFiles: 4},
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
				if !strings.Contains(output, "●") {
					t.Error("missing dirty/clean indicator dot")
				}
			},
		},
		"shows commit info on second line": {
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
		"two lines per entry": {
			worktrees: sampleWorktrees(),
			selected:  0,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				lines := strings.Split(output, "\n")
				// 3 worktrees × 2 lines = 6 lines
				if len(lines) != 6 {
					t.Errorf("expected 6 lines (3 entries × 2), got %d", len(lines))
				}
			},
		},
		"selected entry uses accent color not reverse video": {
			worktrees: sampleWorktrees(),
			selected:  1,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				lines := strings.Split(output, "\n")
				// Selected entry at index 1 starts at line 2 (0-indexed)
				if len(lines) < 4 {
					t.Fatal("expected at least 4 lines")
				}
				// Line 2 should have the > prefix and branch name
				if !strings.Contains(lines[2], ">") {
					t.Error("selected primary line missing '>' prefix")
				}
				if !strings.Contains(lines[2], "feature") {
					t.Error("selected primary line missing branch name")
				}
				// Should NOT contain reverse video escape (ESC[7m)
				if strings.Contains(lines[2], "\x1b[7m") {
					t.Error("selected line should not use reverse video")
				}
			},
		},
		"file count on primary line": {
			worktrees: sampleWorktrees(),
			selected:  0,
			scroll:    0,
			width:     80,
			height:    10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				lines := strings.Split(output, "\n")
				// Feature worktree (index 1) primary line is lines[2]
				if len(lines) < 3 {
					t.Fatal("expected at least 3 lines")
				}
				if !strings.Contains(lines[2], "4 files") {
					t.Errorf("file count should appear on primary line of dirty worktree, got: %q", lines[2])
				}
			},
		},
		"height limits visible entries": {
			worktrees: sampleWorktrees(),
			selected:  0,
			scroll:    0,
			width:     80,
			height:    4, // room for 2 entries (4 lines / 2 lines per entry)
			checks: func(t *testing.T, output string) {
				t.Helper()
				lines := strings.Split(output, "\n")
				if len(lines) > 4 {
					t.Errorf("expected at most 4 lines (2 entries), got %d", len(lines))
				}
				// Third worktree should not be visible
				if strings.Contains(output, "bugfix") {
					t.Error("third worktree should be clipped by height")
				}
			},
		},
		"scrolled view": {
			worktrees: sampleWorktrees(),
			selected:  2,
			scroll:    1,
			width:     80,
			height:    4, // room for 2 entries
			checks: func(t *testing.T, output string) {
				t.Helper()
				// With scroll=1 and height=4 (2 entries), should see worktrees[1] and [2]
				if strings.Contains(output, "main") && !strings.Contains(output, "feature") {
					t.Error("scroll not applied: main visible but feature not")
				}
				if !strings.Contains(output, "bugfix") {
					t.Error("bugfix should be visible in scrolled view")
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
		"activity time on secondary line": {
			worktrees: func() []model.WorktreeInfo {
				wts := sampleWorktrees()
				wts[0].LastActivityAt = time.Now().Add(-30 * time.Second)
				return wts
			}(),
			selected: 0,
			scroll:   0,
			width:    80,
			height:   10,
			checks: func(t *testing.T, output string) {
				t.Helper()
				lines := strings.Split(output, "\n")
				if len(lines) < 2 {
					t.Fatal("expected at least 2 lines")
				}
				// Secondary line (index 1) should have activity time
				if !strings.Contains(lines[1], "s ago") {
					t.Errorf("expected relative time on secondary line, got: %q", lines[1])
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

func TestRenderDashboardBareWorktreeFallsBackToBasename(t *testing.T) {
	t.Parallel()

	wts := []model.WorktreeInfo{
		{Path: "/home/user/project", Branch: "", Bare: true, CommitHash: "abc1234", Subject: "init"},
	}
	output := RenderDashboard(wts, 0, 0, 80, 10)

	if !strings.Contains(output, "project") {
		t.Errorf("bare worktree with empty branch should show path basename, got:\n%s", output)
	}
}

func TestRenderDashboardDetachedWorktreeFallsBackToBasename(t *testing.T) {
	t.Parallel()

	wts := []model.WorktreeInfo{
		{Path: "/home/user/detached-wt", Branch: "", CommitHash: "def5678", Subject: "detached HEAD"},
	}
	output := RenderDashboard(wts, 0, 0, 80, 10)

	if !strings.Contains(output, "detached-wt") {
		t.Errorf("detached worktree with empty branch should show path basename, got:\n%s", output)
	}
}

func TestRenderDashboardShowsStaleness(t *testing.T) {
	t.Parallel()

	wts := []model.WorktreeInfo{
		{Path: "/p", Branch: "main", CommitHash: "abc", Subject: "init", Dirty: true,
			ChangedFiles: 3, HeadCommittedAt: time.Now().Add(-2 * time.Hour)},
	}
	output := RenderDashboard(wts, 0, 0, 120, 10)

	if !strings.Contains(output, "2h") {
		t.Errorf("expected staleness label '2h' in output:\n%s", output)
	}
}

func TestRelativeTime(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		d          time.Duration
		want       string
		wantSuffix string
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

func TestStalenessBadge(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		age  time.Duration
		want string
	}{
		"hours":      {age: 6 * time.Hour, want: "6h"},
		"days":       {age: 3 * 24 * time.Hour, want: "3d"},
		"one week":   {age: 7 * 24 * time.Hour, want: "1w"},
		"two weeks":  {age: 14 * 24 * time.Hour, want: "2w"},
		"months":     {age: 90 * 24 * time.Hour, want: "3mo"},
		"sub-hour":   {age: 30 * time.Minute, want: "30m"},
		"future":     {age: -5 * time.Minute, want: "0m"},
		"zero":       {age: 0, want: "--"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var at time.Time
			if tc.age != 0 {
				at = time.Now().Add(-tc.age)
			}
			got, _ := StalenessBadge(at)
			if got != tc.want {
				t.Errorf("StalenessBadge label = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOverlayDialog(t *testing.T) {
	t.Parallel()

	// Build a simple base to overlay onto.
	base := strings.Repeat("base content line\n", 24)

	tests := map[string]struct {
		title  string
		body   string
		hint   string
		checks func(t *testing.T, output string)
	}{
		"contains title and body": {
			title: "Delete worktree?", body: "my-worktree", hint: "y confirm    n/Esc cancel",
			checks: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "Delete worktree?") {
					t.Error("missing title")
				}
				if !strings.Contains(output, "my-worktree") {
					t.Error("missing body text")
				}
				if !strings.Contains(output, "y confirm") {
					t.Error("missing hint")
				}
			},
		},
		"has border chars": {
			title: "Delete?", body: "wt", hint: "y/n",
			checks: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "╭") || !strings.Contains(output, "╯") {
					t.Error("missing border characters")
				}
			},
		},
		"preserves base content outside dialog": {
			title: "Delete?", body: "wt", hint: "y/n",
			checks: func(t *testing.T, output string) {
				t.Helper()
				lines := strings.Split(output, "\n")
				// First line should still be base content (dialog is centered, not at row 0).
				if !strings.Contains(lines[0], "base content") {
					t.Error("base content should be preserved above dialog")
				}
			},
		},
		"shows dirty warning": {
			title: "Delete worktree?", body: "my-wt\n\nDIRTY — uncommitted changes will be lost!", hint: "y/n",
			checks: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "DIRTY") {
					t.Error("missing dirty warning")
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			output := OverlayDialog(base, tc.title, tc.body, tc.hint, 80, 24)
			tc.checks(t, output)
		})
	}
}
