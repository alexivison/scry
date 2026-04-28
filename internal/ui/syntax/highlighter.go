// Package syntax turns Chroma tokens into Scry-owned terminal styling.
package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
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
	entries map[CacheKey]*LineCache
}

// NewCache creates an empty syntax cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[CacheKey]*LineCache)}
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
	lineCache := NewLineCache(patch.Summary.Path, patch.Summary.OldPath, sampleFromPatch(patch))
	c.entries[key] = lineCache
	return lineCache
}

// LineCache memoizes highlighted bodies by logical diff-line index.
type LineCache struct {
	highlighter *Highlighter
	lines       map[int]string
}

// NewLineCache creates a per-patch line highlighter.
func NewLineCache(path, oldPath, sample string) *LineCache {
	return &LineCache{
		highlighter: NewHighlighter(path, oldPath, sample),
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
	lexer chroma.Lexer
}

// NewHighlighter creates a highlighter using Scry's lexer detection order.
func NewHighlighter(path, oldPath, sample string) *Highlighter {
	return &Highlighter{lexer: DetectLexer(path, oldPath, sample)}
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
	if body == "" || h == nil || h.lexer == nil {
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
		style := styleFor(token.Type)
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

func styleFor(tokenType chroma.TokenType) lipgloss.Style {
	switch {
	case tokenType == chroma.Error || tokenType == chroma.GenericError:
		return errorStyle
	case tokenType.InCategory(chroma.Comment):
		return commentStyle
	case tokenType == chroma.KeywordType ||
		tokenType == chroma.NameClass ||
		tokenType == chroma.NameBuiltin ||
		tokenType == chroma.NameException:
		return typeStyle
	case tokenType.InCategory(chroma.Keyword):
		return keywordStyle
	case tokenType.InSubCategory(chroma.LiteralString):
		return stringStyle
	case tokenType.InSubCategory(chroma.LiteralNumber) ||
		tokenType == chroma.LiteralDate ||
		tokenType == chroma.LiteralOther ||
		tokenType == chroma.NameConstant ||
		tokenType == chroma.KeywordConstant:
		return constantStyle
	case tokenType.InSubCategory(chroma.NameFunction):
		return functionStyle
	default:
		return lipgloss.NewStyle()
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

var (
	commentStyle = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Faint(true)

	keywordStyle = lipgloss.NewStyle().
			Foreground(theme.Accent).
			Bold(true)

	stringStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	constantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5"))

	typeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6"))

	functionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6"))

	errorStyle = lipgloss.NewStyle().
			Foreground(theme.Error)
)
