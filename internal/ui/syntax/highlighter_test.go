package syntax

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/alexivison/scry/internal/model"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI)
	os.Exit(m.Run())
}

func TestDetectLexerByPath(t *testing.T) {
	t.Parallel()

	lexer := DetectLexer("main.go", "", "this sample should not decide the lexer")
	if got, want := lexer.Config().Name, "Go"; got != want {
		t.Fatalf("DetectLexer() = %q, want %q", got, want)
	}
}

func TestDetectLexerUsesOldPathForRenameOrDeletion(t *testing.T) {
	t.Parallel()

	lexer := DetectLexer("", "deleted.py", "this sample should not decide the lexer")
	if got, want := lexer.Config().Name, "Python"; got != want {
		t.Fatalf("DetectLexer() = %q, want %q", got, want)
	}
}

func TestDetectLexerUsesSampleWhenPathsDoNotMatch(t *testing.T) {
	t.Parallel()

	sample := "package main\n\nfunc main()\n{\n}\n"
	lexer := DetectLexer("", "", sample)
	if got := lexer.Config().Name; got == "fallback" {
		t.Fatalf("DetectLexer() = %q, want a lexer selected from sample analysis", got)
	}
}

func TestHighlightLineFallbackKeepsPlainText(t *testing.T) {
	t.Parallel()

	body := "plain text without a known language"
	got := HighlightLine("", "", "", body)
	if ansi.Strip(got) != body {
		t.Fatalf("HighlightLine() stripped = %q, want %q", ansi.Strip(got), body)
	}
}

func TestHighlightLineAddsANSIAndTruncatesSafely(t *testing.T) {
	t.Parallel()

	body := `func main() { return "ok" }`
	highlighted := HighlightLine("main.go", "", "", body)
	if !strings.Contains(highlighted, "\x1b[") {
		t.Fatalf("HighlightLine() should include ANSI styling for Go tokens: %q", highlighted)
	}
	if ansi.Strip(highlighted) != body {
		t.Fatalf("HighlightLine() stripped = %q, want %q", ansi.Strip(highlighted), body)
	}

	truncated := ansi.Truncate(highlighted, 12, "")
	if width := lipgloss.Width(truncated); width > 12 {
		t.Fatalf("ansi.Truncate(highlighted) width = %d, want <= 12; %q", width, truncated)
	}
	if !strings.HasPrefix(body, ansi.Strip(truncated)) {
		t.Fatalf("truncated output stripped = %q, want body prefix of %q", ansi.Strip(truncated), body)
	}
}

func TestHighlightSpanKeepsSearchMatchANSIValid(t *testing.T) {
	t.Parallel()

	body := `func main() { return "ok" }`
	start := strings.Index(body, "return")
	plain := HighlightLine("main.go", "", "", body)
	highlighted := NewHighlighter("main.go", "", "").HighlightSpan(body, start, start+len("return"))

	if highlighted == plain {
		t.Fatal("HighlightSpan() should add search-match styling on top of syntax styling")
	}
	if ansi.Strip(highlighted) != body {
		t.Fatalf("HighlightSpan() stripped = %q, want %q", ansi.Strip(highlighted), body)
	}
	truncated := ansi.Truncate(highlighted, 18, "")
	if width := lipgloss.Width(truncated); width > 18 {
		t.Fatalf("ansi.Truncate(search-highlighted) width = %d, want <= 18; %q", width, truncated)
	}
}

func TestLineCacheHitsOnRepeatedRender(t *testing.T) {
	t.Parallel()

	patch := model.FilePatch{
		Summary: model.FileSummary{Path: "main.go"},
		Hunks: []model.Hunk{{
			Lines: []model.DiffLine{
				{Kind: model.LineContext, Text: `func main() { return "ok" }`},
			},
		}},
	}
	cache := NewCache()
	lines := cache.ForPatch(patch, "hash-a")

	first := lines.HighlightLine(0, patch.Hunks[0].Lines[0].Text)
	lines.highlighter = nil
	second := lines.HighlightLine(0, patch.Hunks[0].Lines[0].Text)

	if second != first {
		t.Fatalf("cached highlight mismatch\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestCacheKeyIncludesPathAndContentHash(t *testing.T) {
	t.Parallel()

	patch := model.FilePatch{Summary: model.FileSummary{Path: "main.go"}}
	cache := NewCache()

	first := cache.ForPatch(patch, "hash-a")
	if got := cache.ForPatch(patch, "hash-a"); got != first {
		t.Fatal("cache should reuse the same entry for matching path and content hash")
	}
	if got := cache.ForPatch(patch, "hash-b"); got == first {
		t.Fatal("cache should create a new entry when content hash changes")
	}

	patch.Summary.Path = "other.go"
	if got := cache.ForPatch(patch, "hash-a"); got == first {
		t.Fatal("cache should create a new entry when path changes")
	}
}
