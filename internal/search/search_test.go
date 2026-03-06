package search

import (
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func samplePatch() model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{
			{
				Header: "func init()", OldStart: 1, OldLen: 3, NewStart: 1, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, Text: "package main"},
					{Kind: model.LineAdded, Text: `import "os"`},
					{Kind: model.LineContext, Text: ""},
				},
			},
			{
				Header: "func main()", OldStart: 10, OldLen: 3, NewStart: 11, NewLen: 4,
				Lines: []model.DiffLine{
					{Kind: model.LineContext, Text: "func main() {"},
					{Kind: model.LineDeleted, Text: "\told()"},
					{Kind: model.LineAdded, Text: "\tnew()"},
					{Kind: model.LineContext, Text: "}"},
				},
			},
		},
	}
}

// Flat line layout (hunk headers are NOT indexed — only DiffLine text):
// 0: "package main"
// 1: `import "os"`
// 2: ""
// 3: "func main() {"
// 4: "\told()"
// 5: "\tnew()"
// 6: "}"

func TestBuild(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		patch     model.FilePatch
		wantLines int
	}{
		"sample patch has 7 lines": {
			patch:     samplePatch(),
			wantLines: 7,
		},
		"empty patch has 0 lines": {
			patch:     model.FilePatch{},
			wantLines: 0,
		},
		"single hunk single line": {
			patch: model.FilePatch{
				Hunks: []model.Hunk{
					{Lines: []model.DiffLine{{Kind: model.LineContext, Text: "hello"}}},
				},
			},
			wantLines: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			idx := Build(tc.patch)
			if idx.Len() != tc.wantLines {
				t.Errorf("Len() = %d, want %d", idx.Len(), tc.wantLines)
			}
		})
	}
}

func TestFindForward(t *testing.T) {
	t.Parallel()

	idx := Build(samplePatch())

	tests := map[string]struct {
		query    string
		from     int
		wantLine int
		wantOK   bool
	}{
		"find main from start": {
			query: "main", from: 0, wantLine: 0, wantOK: true,
		},
		"find main from line 1 skips line 0": {
			query: "main", from: 1, wantLine: 3, wantOK: true,
		},
		"find import from start": {
			query: "import", from: 0, wantLine: 1, wantOK: true,
		},
		"wrap around: find package from line 1": {
			query: "package", from: 1, wantLine: 0, wantOK: true,
		},
		"no match": {
			query: "nonexistent", from: 0, wantLine: 0, wantOK: false,
		},
		"empty query is no-op": {
			query: "", from: 0, wantLine: 0, wantOK: false,
		},
		"single match wraps to itself": {
			query: "import", from: 2, wantLine: 1, wantOK: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			line, ok := idx.Find(tc.query, tc.from, SearchNext)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && line != tc.wantLine {
				t.Errorf("line = %d, want %d", line, tc.wantLine)
			}
		})
	}
}

func TestFindBackward(t *testing.T) {
	t.Parallel()

	idx := Build(samplePatch())

	tests := map[string]struct {
		query    string
		from     int
		wantLine int
		wantOK   bool
	}{
		"find main backward from line 5": {
			query: "main", from: 5, wantLine: 3, wantOK: true,
		},
		"find main backward from line 3 matches line 3 itself": {
			query: "main", from: 3, wantLine: 3, wantOK: true,
		},
		"wrap around backward: find } from line 0": {
			query: "}", from: 0, wantLine: 6, wantOK: true,
		},
		"no match backward": {
			query: "nonexistent", from: 5, wantLine: 0, wantOK: false,
		},
		"empty query backward": {
			query: "", from: 5, wantLine: 0, wantOK: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			line, ok := idx.Find(tc.query, tc.from, SearchPrev)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && line != tc.wantLine {
				t.Errorf("line = %d, want %d", line, tc.wantLine)
			}
		})
	}
}

func TestSmartCase(t *testing.T) {
	t.Parallel()

	patch := model.FilePatch{
		Hunks: []model.Hunk{
			{
				Lines: []model.DiffLine{
					{Kind: model.LineContext, Text: "Hello World"},
					{Kind: model.LineContext, Text: "hello world"},
					{Kind: model.LineContext, Text: "HELLO WORLD"},
				},
			},
		},
	}
	idx := Build(patch)

	tests := map[string]struct {
		query    string
		from     int
		dir      SearchDirection
		wantLine int
		wantOK   bool
	}{
		"lowercase query matches all — finds first from 0": {
			query: "hello", from: 0, dir: SearchNext, wantLine: 0, wantOK: true,
		},
		"lowercase query matches all — finds second": {
			query: "hello", from: 1, dir: SearchNext, wantLine: 1, wantOK: true,
		},
		"lowercase query matches all — finds third": {
			query: "hello", from: 2, dir: SearchNext, wantLine: 2, wantOK: true,
		},
		"uppercase query is case-sensitive — Hello matches line 0 only": {
			query: "Hello", from: 0, dir: SearchNext, wantLine: 0, wantOK: true,
		},
		"uppercase query — Hello from line 1 wraps to line 0": {
			query: "Hello", from: 1, dir: SearchNext, wantLine: 0, wantOK: true,
		},
		"uppercase query HELLO matches only line 2": {
			query: "HELLO", from: 0, dir: SearchNext, wantLine: 2, wantOK: true,
		},
		"uppercase query that has no exact match": {
			query: "hELLO", from: 0, dir: SearchNext, wantLine: 0, wantOK: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			line, ok := idx.Find(tc.query, tc.from, tc.dir)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && line != tc.wantLine {
				t.Errorf("line = %d, want %d", line, tc.wantLine)
			}
		})
	}
}

func TestFindEmptyIndex(t *testing.T) {
	t.Parallel()

	idx := Build(model.FilePatch{})
	line, ok := idx.Find("anything", 0, SearchNext)
	if ok {
		t.Errorf("expected no match on empty index, got line %d", line)
	}
}

func TestFindFromOutOfBounds(t *testing.T) {
	t.Parallel()

	idx := Build(samplePatch())

	tests := map[string]struct {
		from     int
		dir      SearchDirection
		wantLine int
		wantOK   bool
	}{
		"from negative forward clamps to 0": {
			from: -1, dir: SearchNext, wantLine: 6, wantOK: true,
		},
		"from beyond end forward clamps to last": {
			from: 100, dir: SearchNext, wantLine: 6, wantOK: true,
		},
		"from negative backward": {
			from: -1, dir: SearchPrev, wantLine: 6, wantOK: true,
		},
		"from beyond end backward": {
			from: 100, dir: SearchPrev, wantLine: 6, wantOK: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			line, ok := idx.Find("}", tc.from, tc.dir)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && line != tc.wantLine {
				t.Errorf("line = %d, want %d", line, tc.wantLine)
			}
		})
	}
}
