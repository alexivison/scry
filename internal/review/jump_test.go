package review

import (
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func TestNextChangedFile(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "a.go"},
		{Path: "b.go"},
		{Path: "c.go"},
		{Path: "d.go"},
		{Path: "e.go"},
	}
	// b.go is hot (gen 5), d.go is warm (gen 4).
	changeGen := map[string]int{
		"b.go": 5,
		"d.go": 4,
	}
	currentGen := 5

	tests := map[string]struct {
		from    int
		want    int
		wantOK  bool
	}{
		"from a.go finds b.go":           {from: 0, want: 1, wantOK: true},
		"from b.go finds d.go":           {from: 1, want: 3, wantOK: true},
		"from d.go wraps to b.go":        {from: 3, want: 1, wantOK: true},
		"from e.go wraps to b.go":        {from: 4, want: 1, wantOK: true},
		"from c.go finds d.go":           {from: 2, want: 3, wantOK: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := NextChangedFile(files, changeGen, currentGen, tc.from)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("NextChangedFile(from=%d) = %d, want %d", tc.from, got, tc.want)
			}
		})
	}
}

func TestPrevChangedFile(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{
		{Path: "a.go"},
		{Path: "b.go"},
		{Path: "c.go"},
		{Path: "d.go"},
		{Path: "e.go"},
	}
	changeGen := map[string]int{
		"b.go": 5,
		"d.go": 4,
	}
	currentGen := 5

	tests := map[string]struct {
		from    int
		want    int
		wantOK  bool
	}{
		"from d.go finds b.go":           {from: 3, want: 1, wantOK: true},
		"from b.go wraps to d.go":        {from: 1, want: 3, wantOK: true},
		"from e.go finds d.go":           {from: 4, want: 3, wantOK: true},
		"from a.go wraps to d.go":        {from: 0, want: 3, wantOK: true},
		"from c.go finds b.go":           {from: 2, want: 1, wantOK: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := PrevChangedFile(files, changeGen, currentGen, tc.from)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("PrevChangedFile(from=%d) = %d, want %d", tc.from, got, tc.want)
			}
		})
	}
}

func TestNextChangedFile_NoMatch(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{{Path: "a.go"}, {Path: "b.go"}}
	changeGen := map[string]int{} // all cold
	currentGen := 5

	_, ok := NextChangedFile(files, changeGen, currentGen, 0)
	if ok {
		t.Error("expected no match when all files are cold")
	}
}

func TestPrevChangedFile_NoMatch(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{{Path: "a.go"}, {Path: "b.go"}}
	changeGen := map[string]int{}
	currentGen := 5

	_, ok := PrevChangedFile(files, changeGen, currentGen, 0)
	if ok {
		t.Error("expected no match when all files are cold")
	}
}

func TestNextChangedFile_EmptyFiles(t *testing.T) {
	t.Parallel()

	_, ok := NextChangedFile(nil, nil, 5, 0)
	if ok {
		t.Error("expected no match for empty files")
	}
}

func TestNextChangedFile_SingleHotFile(t *testing.T) {
	t.Parallel()

	files := []model.FileSummary{{Path: "only.go"}}
	changeGen := map[string]int{"only.go": 5}

	// From the only hot file, wrapping should find itself.
	idx, ok := NextChangedFile(files, changeGen, 5, 0)
	if !ok {
		t.Fatal("expected match")
	}
	if idx != 0 {
		t.Errorf("got %d, want 0 (wrap to self)", idx)
	}
}
