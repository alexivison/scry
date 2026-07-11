package diff

import (
	"context"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func TestExpandTabs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in       string
		tabWidth int
		want     string
	}{
		"leading tab, width 8": {
			in:       "\tx",
			tabWidth: 8,
			want:     strings.Repeat(" ", 8) + "x",
		},
		"interior tab, width 8": {
			in:       "ab\tx",
			tabWidth: 8,
			want:     "ab" + strings.Repeat(" ", 6) + "x",
		},
		"tab lands exactly on a stop, width 8": {
			in:       "abcdefgh\tx",
			tabWidth: 8,
			want:     "abcdefgh" + strings.Repeat(" ", 8) + "x",
		},
		"multiple consecutive tabs": {
			in:       "\t\tx",
			tabWidth: 8,
			want:     strings.Repeat(" ", 16) + "x",
		},
		"tab after multiple consecutive tabs realigns": {
			in:       "\t\tab\tx",
			tabWidth: 8,
			want:     strings.Repeat(" ", 16) + "ab" + strings.Repeat(" ", 6) + "x",
		},
		"no tabs returned unchanged": {
			in:       "package main",
			tabWidth: 8,
			want:     "package main",
		},
		"empty string": {
			in:       "",
			tabWidth: 8,
			want:     "",
		},
		"leading tab, width 4": {
			in:       "\tx",
			tabWidth: 4,
			want:     strings.Repeat(" ", 4) + "x",
		},
		"interior tab, width 4": {
			in:       "ab\tx",
			tabWidth: 4,
			want:     "ab" + strings.Repeat(" ", 2) + "x",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := expandTabs(tc.in, tc.tabWidth)
			if got != tc.want {
				t.Errorf("expandTabs(%q, %d) = %q, want %q", tc.in, tc.tabWidth, got, tc.want)
			}
			if strings.ContainsRune(got, '\t') {
				t.Errorf("expandTabs(%q, %d) result still contains a tab: %q", tc.in, tc.tabWidth, got)
			}
		})
	}
}

// TestExpandTabsNoTabFastPath confirms the no-tab input is returned as-is
// (same underlying data), not rebuilt through the strings.Builder path.
func TestExpandTabsNoTabFastPath(t *testing.T) {
	t.Parallel()

	in := "no tabs here at all"
	got := expandTabs(in, tabWidth)
	if got != in {
		t.Fatalf("expandTabs(%q) = %q, want unchanged", in, got)
	}
}

// tabIndentedPatch is a unified diff whose hunk body has tab-indented
// context, deleted, and added lines, mirroring the tab-indented Go code
// scry itself ships (and therefore the shape most likely to trigger the
// clipping bug).
const tabIndentedPatch = "diff --git a/tabs.go b/tabs.go\n" +
	"index 1234567..abcdefg 100644\n" +
	"--- a/tabs.go\n" +
	"+++ b/tabs.go\n" +
	"@@ -1,3 +1,3 @@\n" +
	" \tbefore()\n" +
	"-\treturn a + b\n" +
	"+\treturn a + b + c\n" +
	" \tafter()\n"

// TestLoadPatchExpandsTabs feeds a raw unified diff with tab-indented body
// lines through the real parse path (PatchService.LoadPatch) and asserts
// the resulting DiffLine.Text values are tab-free with the expected
// leading-space expansion, so search/highlight byte offsets stay aligned
// with what the renderer measures.
func TestLoadPatchExpandsTabs(t *testing.T) {
	t.Parallel()

	svc := &PatchService{Runner: &mockRunner{fn: routeGit(map[string]string{
		patchCmd("tabs.go"): tabIndentedPatch,
	})}}

	fp, err := svc.LoadPatch(context.Background(), cmp(), "tabs.go", model.StatusModified, false)
	if err != nil {
		t.Fatalf("LoadPatch: %v", err)
	}
	if len(fp.Hunks) != 1 {
		t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
	}

	indent := strings.Repeat(" ", 8)
	wantLines := []model.DiffLine{
		{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: indent + "before()"},
		{Kind: model.LineDeleted, OldNo: intP(2), Text: indent + "return a + b"},
		{Kind: model.LineAdded, NewNo: intP(2), Text: indent + "return a + b + c"},
		{Kind: model.LineContext, OldNo: intP(3), NewNo: intP(3), Text: indent + "after()"},
	}
	assertLines(t, fp.Hunks[0].Lines, wantLines)

	for i, l := range fp.Hunks[0].Lines {
		if strings.ContainsRune(l.Text, '\t') {
			t.Errorf("line[%d].Text still contains a tab: %q", i, l.Text)
		}
	}
}
