package diff

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alexivison/scry/internal/model"
)

func patchCmd(file string) string {
	return "diff --patch --no-color --no-ext-diff -M " + diffRange + " -- " + file
}

func patchCmdW(file string) string {
	return "diff --patch --no-color --no-ext-diff -M -w " + diffRange + " -- " + file
}

func intP(n int) *int { return &n }

func intEq(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrStr(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *p)
}

func assertLines(t *testing.T, got, want []model.DiffLine) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("lines: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Kind != w.Kind {
			t.Errorf("line[%d].Kind = %q, want %q", i, g.Kind, w.Kind)
		}
		if g.Text != w.Text {
			t.Errorf("line[%d].Text = %q, want %q", i, g.Text, w.Text)
		}
		if !intEq(g.OldNo, w.OldNo) {
			t.Errorf("line[%d].OldNo = %v, want %v", i, ptrStr(g.OldNo), ptrStr(w.OldNo))
		}
		if !intEq(g.NewNo, w.NewNo) {
			t.Errorf("line[%d].NewNo = %v, want %v", i, ptrStr(g.NewNo), ptrStr(w.NewNo))
		}
	}
}

// --- golden diff fixtures ---

const modifyPatch = `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main

+import "os"
 func main() {
`

const addPatch = `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new.go
@@ -0,0 +1,2 @@
+package main
+func New() {}
`

const deletePatch = `diff --git a/old.go b/old.go
deleted file mode 100644
index 1234567..0000000
--- a/old.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package main
-func Old() {}
`

const renamePatch = `diff --git a/old.go b/new.go
similarity index 80%
rename from old.go
rename to new.go
index 1234567..abcdefg 100644
--- a/old.go
+++ b/new.go
@@ -1,2 +1,3 @@
 package main
 func Func() {}
+func New() {}
`

const noNewlinePatch = `diff --git a/file.txt b/file.txt
index 1234567..abcdefg 100644
--- a/file.txt
+++ b/file.txt
@@ -1,2 +1,2 @@
 first line
-old last
\ No newline at end of file
+new last
\ No newline at end of file
`

const binaryPatch = `diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
`

const submodulePatch = `diff --git a/sub b/sub
index abc1234..def5678 160000
--- a/sub
+++ b/sub
@@ -1 +1 @@
-Subproject commit abc1234
+Subproject commit def5678
`

// Regression: text diff whose content contains binary/submodule sentinel phrases.
const binaryWordsInContent = `diff --git a/readme.md b/readme.md
index 1234567..abcdefg 100644
--- a/readme.md
+++ b/readme.md
@@ -1,2 +1,3 @@
 some text
+Binary files can differ in many ways
 more text
`

const submoduleWordsInContent = `diff --git a/notes.txt b/notes.txt
index 1234567..abcdefg 100644
--- a/notes.txt
+++ b/notes.txt
@@ -1,2 +1,3 @@
 some text
+Subproject commit messages should be descriptive
 more text
`

// Regression: path containing "160000" should not trigger submodule detection.
const pathWith160000 = `diff --git a/dir 160000 name.txt b/dir 160000 name.txt
index 1234567..abcdefg 100644
--- a/dir 160000 name.txt
+++ b/dir 160000 name.txt
@@ -1,2 +1,3 @@
 some text
+new line
 more text
`

// Regression: hunk ending with a trailing blank context line.
const trailingBlankPatch = `diff --git a/f.go b/f.go
index 1234567..abcdefg 100644
--- a/f.go
+++ b/f.go
@@ -1,3 +1,4 @@
 line1
+added
 line3

`

func TestLoadPatch(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		runner     func(ctx context.Context, args ...string) (string, error)
		compare    *model.ResolvedCompare // nil defaults to cmp()
		filePath   string
		ignoreWS   bool
		wantErr    error // sentinel check via errors.Is
		wantAnyErr bool  // any non-nil error
		check      func(t *testing.T, fp model.FilePatch)
	}{
		"simple modify": {
			runner:   routeGit(map[string]string{patchCmd("main.go"): modifyPatch}),
			filePath: "main.go",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
				h := fp.Hunks[0]
				if h.OldStart != 1 || h.OldLen != 3 || h.NewStart != 1 || h.NewLen != 4 {
					t.Errorf("hunk range = -%d,%d +%d,%d, want -1,3 +1,4",
						h.OldStart, h.OldLen, h.NewStart, h.NewLen)
				}
				wantLines := []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: "package main"},
					{Kind: model.LineContext, OldNo: intP(2), NewNo: intP(2), Text: ""},
					{Kind: model.LineAdded, NewNo: intP(3), Text: `import "os"`},
					{Kind: model.LineContext, OldNo: intP(3), NewNo: intP(4), Text: "func main() {"},
				}
				assertLines(t, h.Lines, wantLines)
			},
		},
		"simple add": {
			runner:   routeGit(map[string]string{patchCmd("new.go"): addPatch}),
			filePath: "new.go",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
				h := fp.Hunks[0]
				if h.OldStart != 0 || h.OldLen != 0 || h.NewStart != 1 || h.NewLen != 2 {
					t.Errorf("hunk range = -%d,%d +%d,%d, want -0,0 +1,2",
						h.OldStart, h.OldLen, h.NewStart, h.NewLen)
				}
				wantLines := []model.DiffLine{
					{Kind: model.LineAdded, NewNo: intP(1), Text: "package main"},
					{Kind: model.LineAdded, NewNo: intP(2), Text: "func New() {}"},
				}
				assertLines(t, h.Lines, wantLines)
			},
		},
		"simple delete": {
			runner:   routeGit(map[string]string{patchCmd("old.go"): deletePatch}),
			filePath: "old.go",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
				h := fp.Hunks[0]
				if h.OldStart != 1 || h.OldLen != 2 || h.NewStart != 0 || h.NewLen != 0 {
					t.Errorf("hunk range = -%d,%d +%d,%d, want -1,2 +0,0",
						h.OldStart, h.OldLen, h.NewStart, h.NewLen)
				}
				wantLines := []model.DiffLine{
					{Kind: model.LineDeleted, OldNo: intP(1), Text: "package main"},
					{Kind: model.LineDeleted, OldNo: intP(2), Text: "func Old() {}"},
				}
				assertLines(t, h.Lines, wantLines)
			},
		},
		"rename with changes": {
			runner:   routeGit(map[string]string{patchCmd("new.go"): renamePatch}),
			filePath: "new.go",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
				h := fp.Hunks[0]
				if h.OldStart != 1 || h.OldLen != 2 || h.NewStart != 1 || h.NewLen != 3 {
					t.Errorf("hunk range = -%d,%d +%d,%d, want -1,2 +1,3",
						h.OldStart, h.OldLen, h.NewStart, h.NewLen)
				}
				wantLines := []model.DiffLine{
					{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: "package main"},
					{Kind: model.LineContext, OldNo: intP(2), NewNo: intP(2), Text: "func Func() {}"},
					{Kind: model.LineAdded, NewNo: intP(3), Text: "func New() {}"},
				}
				assertLines(t, h.Lines, wantLines)
			},
		},
		"no newline at EOF": {
			runner:   routeGit(map[string]string{patchCmd("file.txt"): noNewlinePatch}),
			filePath: "file.txt",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
				var noNLCount int
				for _, l := range fp.Hunks[0].Lines {
					if l.Kind == model.LineNoNewline {
						noNLCount++
						if l.OldNo != nil || l.NewNo != nil {
							t.Error("no-newline marker should have nil line numbers")
						}
					}
				}
				if noNLCount != 2 {
					t.Errorf("no-newline markers = %d, want 2", noNLCount)
				}
			},
		},
		"ignoreWhitespace appends -w flag": {
			runner:   routeGit(map[string]string{patchCmdW("main.go"): modifyPatch}),
			filePath: "main.go",
			ignoreWS: true,
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) == 0 {
					t.Fatal("expected hunks from parsed patch")
				}
			},
		},
		"binary file returns ErrBinaryFile": {
			runner:   routeGit(map[string]string{patchCmd("image.png"): binaryPatch}),
			filePath: "image.png",
			wantErr:  model.ErrBinaryFile,
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if fp.Summary.Path != "image.png" {
					t.Errorf("Summary.Path = %q, want %q", fp.Summary.Path, "image.png")
				}
				if !fp.Summary.IsBinary {
					t.Error("Summary.IsBinary = false, want true")
				}
				if fp.Hunks != nil {
					t.Error("expected nil Hunks for binary file")
				}
			},
		},
		"submodule returns ErrSubmodule": {
			runner:   routeGit(map[string]string{patchCmd("sub"): submodulePatch}),
			filePath: "sub",
			wantErr:  model.ErrSubmodule,
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if fp.Summary.Path != "sub" {
					t.Errorf("Summary.Path = %q, want %q", fp.Summary.Path, "sub")
				}
				if !fp.Summary.IsSubmodule {
					t.Error("Summary.IsSubmodule = false, want true")
				}
				if fp.Hunks != nil {
					t.Error("expected nil Hunks for submodule")
				}
			},
		},
		"oversized byte threshold": {
			runner: func(_ context.Context, _ ...string) (string, error) {
				return strings.Repeat("x", 9<<20), nil // 9 MiB exceeds 8 MiB gate
			},
			filePath: "huge.go",
			wantErr:  model.ErrOversized,
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if fp.Summary.Path != "huge.go" {
					t.Errorf("Summary.Path = %q, want %q", fp.Summary.Path, "huge.go")
				}
				if fp.Hunks != nil {
					t.Error("expected nil Hunks for oversized patch")
				}
			},
		},
		"oversized line count": {
			runner: func(_ context.Context, _ ...string) (string, error) {
				var b strings.Builder
				b.WriteString("diff --git a/big.go b/big.go\n")
				b.WriteString("new file mode 100644\n")
				b.WriteString("--- /dev/null\n")
				b.WriteString("+++ b/big.go\n")
				fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", 50001)
				for i := 0; i < 50001; i++ {
					fmt.Fprintf(&b, "+line %d\n", i)
				}
				return b.String(), nil
			},
			filePath: "big.go",
			wantErr:  model.ErrOversized,
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if fp.Summary.Path != "big.go" {
					t.Errorf("Summary.Path = %q, want %q", fp.Summary.Path, "big.go")
				}
				if fp.Hunks != nil {
					t.Error("expected nil Hunks for oversized line count")
				}
			},
		},
		"git error propagates": {
			runner: func(_ context.Context, _ ...string) (string, error) {
				return "", gitErr(128, "fatal: bad revision")
			},
			filePath:   "any.go",
			wantAnyErr: true,
		},
		"parse failure wraps error": {
			runner: routeGit(map[string]string{
				patchCmd("bad.go"): "diff --git a/bad.go b/bad.go\n--- a/bad.go\n+++ b/bad.go\n@@ bad header\n",
			}),
			filePath:   "bad.go",
			wantAnyErr: true,
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 0 {
					t.Errorf("hunks = %d, want 0 for parse failure", len(fp.Hunks))
				}
			},
		},
		"binary words in content not detected as binary": {
			runner:   routeGit(map[string]string{patchCmd("readme.md"): binaryWordsInContent}),
			filePath: "readme.md",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if fp.Summary.IsBinary {
					t.Error("false positive: text diff detected as binary")
				}
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
			},
		},
		"submodule words in content not detected as submodule": {
			runner:   routeGit(map[string]string{patchCmd("notes.txt"): submoduleWordsInContent}),
			filePath: "notes.txt",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if fp.Summary.IsSubmodule {
					t.Error("false positive: text diff detected as submodule")
				}
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
			},
		},
		"path containing 160000 not detected as submodule": {
			runner:   routeGit(map[string]string{patchCmd("dir 160000 name.txt"): pathWith160000}),
			filePath: "dir 160000 name.txt",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if fp.Summary.IsSubmodule {
					t.Error("false positive: path with 160000 detected as submodule")
				}
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
			},
		},
		"trailing blank context line preserved": {
			runner:   routeGit(map[string]string{patchCmd("f.go"): trailingBlankPatch}),
			filePath: "f.go",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
				lines := fp.Hunks[0].Lines
				if len(lines) != 4 {
					t.Fatalf("lines = %d, want 4", len(lines))
				}
				last := lines[3]
				if last.Kind != model.LineContext || last.Text != "" {
					t.Errorf("last line = %+v, want empty context", last)
				}
			},
		},
		"empty diff returns empty patch": {
			runner:   routeGit(map[string]string{patchCmd("unchanged.go"): ""}),
			filePath: "unchanged.go",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 0 {
					t.Errorf("hunks = %d, want 0", len(fp.Hunks))
				}
			},
		},
		"working tree mode uses base-only range": {
			runner: routeGit(map[string]string{
				"diff --patch --no-color --no-ext-diff -M aaa111 -- main.go": modifyPatch,
			}),
			compare:  &model.ResolvedCompare{DiffRange: "aaa111", WorkingTree: true},
			filePath: "main.go",
			check: func(t *testing.T, fp model.FilePatch) {
				t.Helper()
				if len(fp.Hunks) != 1 {
					t.Fatalf("hunks = %d, want 1", len(fp.Hunks))
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := cmp()
			if tc.compare != nil {
				c = *tc.compare
			}
			svc := &PatchService{Runner: &mockRunner{fn: tc.runner}}
			got, err := svc.LoadPatch(context.Background(), c, tc.filePath, model.StatusModified, tc.ignoreWS)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
			} else if tc.wantAnyErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}
