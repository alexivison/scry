package review

import (
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func TestToggleFlag(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		FlaggedFiles: map[string]bool{},
	}

	ToggleFlag(&state, "a.go")
	if !state.FlaggedFiles["a.go"] {
		t.Error("a.go should be flagged after toggle")
	}

	ToggleFlag(&state, "a.go")
	if state.FlaggedFiles["a.go"] {
		t.Error("a.go should be unflagged after second toggle")
	}
}

func TestPruneFlags(t *testing.T) {
	t.Parallel()

	state := model.AppState{
		FlaggedFiles: map[string]bool{"a.go": true, "gone.go": true},
	}
	files := []model.FileSummary{{Path: "a.go"}, {Path: "b.go"}}

	PruneFlags(&state, files)

	if !state.FlaggedFiles["a.go"] {
		t.Error("a.go should survive pruning")
	}
	if state.FlaggedFiles["gone.go"] {
		t.Error("gone.go should be pruned")
	}
}

func TestNextFlaggedFile(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "a.go"},
		{Path: "b.go"},
		{Path: "c.go"},
		{Path: "d.go"},
	}
	flagged := map[string]bool{"b.go": true, "d.go": true}

	tests := map[string]struct {
		from   int
		want   int
		wantOK bool
	}{
		"from a finds b":       {from: 0, want: 1, wantOK: true},
		"from b finds d":       {from: 1, want: 3, wantOK: true},
		"from d wraps to b":    {from: 3, want: 1, wantOK: true},
		"from c finds d":       {from: 2, want: 3, wantOK: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := NextFlaggedFile(files, flagged, tc.from)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("NextFlaggedFile(from=%d) = (%d, %v), want (%d, %v)",
					tc.from, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestNextFlaggedFile_NoFlags(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{{Path: "a.go"}}
	_, ok := NextFlaggedFile(files, nil, 0)
	if ok {
		t.Error("expected no match when no flags")
	}
}

func TestNextFlaggedFile_Empty(t *testing.T) {
	t.Parallel()

	_, ok := NextFlaggedFile(nil, nil, 0)
	if ok {
		t.Error("expected no match for empty files")
	}
}

func TestNextFlaggedFile_SingleFlaggedFile(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{{Path: "only.go"}}
	flagged := map[string]bool{"only.go": true}

	idx, ok := NextFlaggedFile(files, flagged, 0)
	if !ok {
		t.Fatal("expected match")
	}
	if idx != 0 {
		t.Errorf("got %d, want 0 (wrap to self)", idx)
	}
}

func TestToggleFlag_NilMap(t *testing.T) {
	t.Parallel()

	state := model.AppState{} // FlaggedFiles is nil
	ToggleFlag(&state, "a.go")

	if state.FlaggedFiles == nil {
		t.Fatal("FlaggedFiles should be initialized")
	}
	if !state.FlaggedFiles["a.go"] {
		t.Error("a.go should be flagged")
	}
}
