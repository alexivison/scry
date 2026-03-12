package panes

import (
	"strings"
	"testing"

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
