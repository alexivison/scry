package syntax

import (
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/terminal"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI)
	os.Exit(m.Run())
}

func TestHighlightLineStylesRepresentativeTokensByColorProfile(t *testing.T) {
	tests := []struct {
		name            string
		profile         terminal.ColorProfile
		lipglossProfile termenv.Profile
		commentStyle    lipgloss.Style
		keywordStyle    lipgloss.Style
		stringStyle     lipgloss.Style
		constantStyle   lipgloss.Style
		typeStyle       lipgloss.Style
		functionStyle   lipgloss.Style
		errorStyle      lipgloss.Style
	}{
		{
			name:            "basic",
			profile:         terminal.ColorBasic,
			lipglossProfile: termenv.ANSI,
			commentStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true),
			keywordStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Bold(true),
			stringStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
			constantStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
			typeStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
			functionStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
			errorStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		},
		{
			name:            "ansi256",
			profile:         terminal.ColorANSI256,
			lipglossProfile: termenv.ANSI256,
			commentStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Faint(true),
			keywordStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true),
			stringStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
			constantStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("175")),
			typeStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("81")),
			functionStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("81")),
			errorStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		},
		{
			name:            "truecolor",
			profile:         terminal.ColorTrueColor,
			lipglossProfile: termenv.TrueColor,
			commentStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#6A737D")).Faint(true),
			keywordStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#D73A49")).Bold(true),
			stringStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("#22863A")),
			constantStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#B31D87")),
			typeStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("#00A4B8")),
			functionStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#00A4B8")),
			errorStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("#D1242F")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withLipglossProfile(t, tc.lipglossProfile)

			h := NewHighlighter("main.go", "", "", tc.profile)
			code := h.Highlight(`func main() string { return "ok" }`)
			assertContainsStyledToken(t, code, tc.keywordStyle.Render("func"), "keyword")
			assertContainsStyledToken(t, code, tc.functionStyle.Render("main"), "function")
			assertContainsStyledToken(t, code, tc.typeStyle.Render("string"), "type")
			assertContainsStyledToken(t, code, tc.stringStyle.Render(`"ok"`), "string")

			constant := h.Highlight(`const answer = 42`)
			assertContainsStyledToken(t, constant, tc.constantStyle.Render("42"), "constant")

			comment := h.Highlight(`// note`)
			assertContainsStyledToken(t, comment, tc.commentStyle.Render("// note"), "comment")

			gotError := styleFor(chroma.Error, tc.profile).Render("!")
			if gotError != tc.errorStyle.Render("!") {
				t.Fatalf("error style = %q, want %q", gotError, tc.errorStyle.Render("!"))
			}
		})
	}
}

func TestColorNoneReturnsRawAndSkipsTokenization(t *testing.T) {
	body := `func main() { return "ok" }`
	h := &Highlighter{lexer: panicLexer{}, profile: terminal.ColorNone}

	got := h.Highlight(body)
	if got != body {
		t.Fatalf("Highlight() = %q, want raw body %q", got, body)
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("Highlight() for ColorNone included ANSI escapes: %q", got)
	}
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

func TestHighlightBackgroundAppliesBackgroundToEveryToken(t *testing.T) {
	withLipglossProfile(t, termenv.TrueColor)

	body := `func main() { return "ok" }`
	highlighted := NewHighlighter("main.go", "", "", terminal.ColorTrueColor).
		HighlightBackground(body, lipgloss.Color("#005F00"))

	if ansi.Strip(highlighted) != body {
		t.Fatalf("HighlightBackground() stripped = %q, want %q", ansi.Strip(highlighted), body)
	}
	if count := strings.Count(highlighted, "48;2;0;95;0"); count < 2 {
		t.Fatalf("HighlightBackground() should apply background to multiple tokens, got %d occurrences: %q", count, highlighted)
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

func TestLineCacheUsesColorProfile(t *testing.T) {
	body := `func main() { return "ok" }`
	lines := NewLineCache("main.go", "", "", terminal.ColorNone)

	if got := lines.HighlightLine(0, body); got != body {
		t.Fatalf("HighlightLine() with ColorNone = %q, want %q", got, body)
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

func withLipglossProfile(t *testing.T, profile termenv.Profile) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(profile)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previous)
	})
}

func assertContainsStyledToken(t *testing.T, got, want, token string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("highlighted output missing styled %s token %q\nwant token: %q\ngot:        %q", token, ansi.Strip(want), want, got)
	}
}

type panicLexer struct{}

func (panicLexer) Config() *chroma.Config {
	return &chroma.Config{Name: "panic"}
}

func (panicLexer) Tokenise(*chroma.TokeniseOptions, string) (chroma.Iterator, error) {
	panic("ColorNone should not tokenize")
}

func (p panicLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer {
	return p
}

func (p panicLexer) SetAnalyser(func(string) float32) chroma.Lexer {
	return p
}

func (panicLexer) AnalyseText(string) float32 {
	return 0
}
