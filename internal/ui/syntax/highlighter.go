// Package syntax turns Chroma tokens into Scry-owned terminal styling.
package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/terminal"
	"github.com/alexivison/scry/internal/ui/theme"
)

const sampleLimit = 8192

// CacheKey identifies the highlighted line cache for one loaded patch.
type CacheKey struct {
	Path        string
	ContentHash string
}

// Cache owns memoized syntax rendering for loaded patches.
type Cache struct {
	entries    map[CacheKey]*LineCache
	profile    terminal.ColorProfile
	profileSet bool
}

// NewCache creates an empty syntax cache.
func NewCache(profiles ...terminal.ColorProfile) *Cache {
	return &Cache{
		entries:    make(map[CacheKey]*LineCache),
		profile:    colorProfileOrDefault(profiles...),
		profileSet: true,
	}
}

// ForPatch returns the line cache keyed by path and PatchLoadState content hash.
func (c *Cache) ForPatch(patch model.FilePatch, contentHash string) *LineCache {
	if c == nil {
		return NewLineCache(patch.Summary.Path, patch.Summary.OldPath, sampleFromPatch(patch))
	}
	if c.entries == nil {
		c.entries = make(map[CacheKey]*LineCache)
	}

	key := CacheKey{Path: patch.Summary.Path, ContentHash: contentHash}
	if cached, ok := c.entries[key]; ok {
		return cached
	}
	lineCache := NewLineCache(patch.Summary.Path, patch.Summary.OldPath, sampleFromPatch(patch), c.colorProfile())
	c.entries[key] = lineCache
	return lineCache
}

func (c *Cache) colorProfile() terminal.ColorProfile {
	if c == nil || !c.profileSet {
		return terminal.ColorBasic
	}
	return normalizeColorProfile(c.profile)
}

// LineCache memoizes highlighted bodies by logical diff-line index.
type LineCache struct {
	highlighter *Highlighter
	lines       map[int]string
}

// NewLineCache creates a per-patch line highlighter.
func NewLineCache(path, oldPath, sample string, profiles ...terminal.ColorProfile) *LineCache {
	return &LineCache{
		highlighter: NewHighlighter(path, oldPath, sample, profiles...),
		lines:       make(map[int]string),
	}
}

// HighlightLine returns the cached highlighted body for a diff line.
func (lc *LineCache) HighlightLine(line int, body string) string {
	if lc == nil {
		return body
	}
	if highlighted, ok := lc.lines[line]; ok {
		return highlighted
	}
	if lc.highlighter == nil {
		return body
	}
	highlighted := lc.highlighter.Highlight(body)
	lc.lines[line] = highlighted
	return highlighted
}

// HighlightLineSpan highlights a transient search span without caching it.
func (lc *LineCache) HighlightLineSpan(line int, body string, start, end int) string {
	if start < 0 || end <= start || end > len(body) {
		return lc.HighlightLine(line, body)
	}
	if lc == nil || lc.highlighter == nil {
		return body
	}
	return lc.highlighter.HighlightSpan(body, start, end)
}

// Highlighter wraps a coalesced Chroma lexer.
type Highlighter struct {
	lexer   chroma.Lexer
	profile terminal.ColorProfile
}

// NewHighlighter creates a highlighter using Scry's lexer detection order.
func NewHighlighter(path, oldPath, sample string, profiles ...terminal.ColorProfile) *Highlighter {
	profile := colorProfileOrDefault(profiles...)
	if profile == terminal.ColorNone {
		return &Highlighter{profile: profile}
	}
	return &Highlighter{lexer: DetectLexer(path, oldPath, sample), profile: profile}
}

// DetectLexer selects a lexer by path, old path, content sample, then fallback.
func DetectLexer(path, oldPath, sample string) chroma.Lexer {
	lexer := lexers.Match(path)
	if lexer == nil && oldPath != "" {
		lexer = lexers.Match(oldPath)
	}
	if lexer == nil && sample != "" {
		lexer = lexers.Analyse(sample)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	return chroma.Coalesce(lexer)
}

// HighlightLine highlights one diff body without using a cache.
func HighlightLine(path, oldPath, sample, body string) string {
	return NewHighlighter(path, oldPath, sample).Highlight(body)
}

// Highlight renders body text with Scry-owned styles.
func (h *Highlighter) Highlight(body string) string {
	return h.highlight(body, span{start: -1, end: -1})
}

// HighlightSpan renders body text and reverses the provided raw byte span.
func (h *Highlighter) HighlightSpan(body string, start, end int) string {
	return h.highlight(body, span{start: start, end: end})
}

func (h *Highlighter) highlight(body string, match span) string {
	if body == "" || h == nil || h.profile == terminal.ColorNone || h.lexer == nil {
		return body
	}

	tokens, ok := h.tokens(body)
	if !ok {
		return body
	}

	var b strings.Builder
	offset := 0
	for _, token := range tokens {
		if token == chroma.EOF || token.Value == "" {
			continue
		}
		style := styleFor(token.Type, h.profile)
		b.WriteString(renderToken(style, token.Value, offset, match))
		offset += len(token.Value)
	}
	if b.Len() == 0 {
		return body
	}
	return b.String()
}

func (h *Highlighter) tokens(body string) (_ []chroma.Token, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	tokens, err := chroma.Tokenise(h.lexer, nil, body)
	if err != nil {
		return nil, false
	}
	return tokens, true
}

type span struct {
	start int
	end   int
}

func (s span) overlaps(start, end int) bool {
	return s.start >= 0 && s.end > s.start && start < s.end && end > s.start
}

func renderToken(style lipgloss.Style, value string, tokenStart int, match span) string {
	tokenEnd := tokenStart + len(value)
	if !match.overlaps(tokenStart, tokenEnd) {
		return style.Render(value)
	}

	overlapStart := max(match.start-tokenStart, 0)
	overlapEnd := min(match.end-tokenStart, len(value))
	return style.Render(value[:overlapStart]) +
		style.Reverse(true).Render(value[overlapStart:overlapEnd]) +
		style.Render(value[overlapEnd:])
}

func styleFor(tokenType chroma.TokenType, profile terminal.ColorProfile) lipgloss.Style {
	palette := paletteFor(profile)
	switch {
	case tokenType == chroma.Error || tokenType == chroma.GenericError:
		return lipgloss.NewStyle().Foreground(palette.error)
	case tokenType.InCategory(chroma.Comment):
		return lipgloss.NewStyle().Foreground(palette.comment).Faint(true)
	case tokenType == chroma.KeywordType ||
		tokenType == chroma.NameClass ||
		tokenType == chroma.NameBuiltin ||
		tokenType == chroma.NameException:
		return lipgloss.NewStyle().Foreground(palette.typeName)
	case tokenType.InCategory(chroma.Keyword):
		return lipgloss.NewStyle().Foreground(palette.keyword).Bold(true)
	case tokenType.InSubCategory(chroma.LiteralString):
		return lipgloss.NewStyle().Foreground(palette.string)
	case tokenType.InSubCategory(chroma.LiteralNumber) ||
		tokenType == chroma.LiteralDate ||
		tokenType == chroma.LiteralOther ||
		tokenType == chroma.NameConstant ||
		tokenType == chroma.KeywordConstant:
		return lipgloss.NewStyle().Foreground(palette.constant)
	case tokenType.InSubCategory(chroma.NameFunction):
		return lipgloss.NewStyle().Foreground(palette.function)
	default:
		return lipgloss.NewStyle()
	}
}

type tokenPalette struct {
	comment  lipgloss.Color
	keyword  lipgloss.Color
	string   lipgloss.Color
	constant lipgloss.Color
	typeName lipgloss.Color
	function lipgloss.Color
	error    lipgloss.Color
}

func paletteFor(profile terminal.ColorProfile) tokenPalette {
	switch normalizeColorProfile(profile) {
	case terminal.ColorANSI256:
		return tokenPalette{
			comment:  lipgloss.Color("244"),
			keyword:  lipgloss.Color("33"),
			string:   lipgloss.Color("114"),
			constant: lipgloss.Color("175"),
			typeName: lipgloss.Color("81"),
			function: lipgloss.Color("81"),
			error:    lipgloss.Color("196"),
		}
	case terminal.ColorTrueColor:
		return tokenPalette{
			comment:  lipgloss.Color("#6A737D"),
			keyword:  lipgloss.Color("#D73A49"),
			string:   lipgloss.Color("#22863A"),
			constant: lipgloss.Color("#B31D87"),
			typeName: lipgloss.Color("#00A4B8"),
			function: lipgloss.Color("#00A4B8"),
			error:    lipgloss.Color("#D1242F"),
		}
	default:
		return tokenPalette{
			comment:  theme.Muted,
			keyword:  theme.Accent,
			string:   lipgloss.Color("2"),
			constant: lipgloss.Color("5"),
			typeName: lipgloss.Color("6"),
			function: lipgloss.Color("6"),
			error:    theme.Error,
		}
	}
}

func colorProfileOrDefault(profiles ...terminal.ColorProfile) terminal.ColorProfile {
	if len(profiles) == 0 {
		return terminal.ColorBasic
	}
	return normalizeColorProfile(profiles[0])
}

func normalizeColorProfile(profile terminal.ColorProfile) terminal.ColorProfile {
	switch profile {
	case terminal.ColorNone, terminal.ColorBasic, terminal.ColorANSI256, terminal.ColorTrueColor:
		return profile
	default:
		return terminal.ColorBasic
	}
}

func sampleFromPatch(patch model.FilePatch) string {
	var b strings.Builder
	for _, hunk := range patch.Hunks {
		for _, line := range hunk.Lines {
			if !Highlightable(line.Kind) || line.Text == "" {
				continue
			}
			if b.Len()+len(line.Text)+1 > sampleLimit {
				return b.String()
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(line.Text)
		}
	}
	return b.String()
}

// Highlightable reports whether syntax highlighting should apply to a diff line.
func Highlightable(kind model.LineKind) bool {
	return kind == model.LineAdded || kind == model.LineDeleted || kind == model.LineContext
}
