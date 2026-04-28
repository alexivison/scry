// Package search implements a directional substring search index for diff lines.
package search

import (
	"strings"
	"unicode"

	"github.com/alexivison/scry/internal/model"
)

// SearchDirection indicates which way to scan.
type SearchDirection int

const (
	SearchNext SearchDirection = iota
	SearchPrev
)

// Match identifies a query occurrence in a logical diff line.
type Match struct {
	Line  int
	Start int
	End   int
}

// Index holds a searchable list of line texts extracted from a FilePatch.
type Index struct {
	lines []string
}

// Build constructs an Index from all DiffLines in the patch's hunks.
// Hunk headers are not included in the index.
func Build(patch model.FilePatch) *Index {
	var lines []string
	for _, h := range patch.Hunks {
		for _, dl := range h.Lines {
			lines = append(lines, dl.Text)
		}
	}
	return &Index{lines: lines}
}

// Len returns the number of indexed lines.
func (idx *Index) Len() int {
	return len(idx.lines)
}

// Find searches for query starting from fromLine in the given direction.
// It returns the matching line index and true, or (0, false) if not found.
// Empty query always returns (0, false). Search wraps around.
func (idx *Index) Find(query string, fromLine int, dir SearchDirection) (int, bool) {
	match, ok := idx.FindMatch(query, fromLine, dir)
	if !ok {
		return 0, false
	}
	return match.Line, true
}

// FindMatch searches for query and returns its logical line and byte span.
// Empty query always returns false. Search wraps around.
func (idx *Index) FindMatch(query string, fromLine int, dir SearchDirection) (Match, bool) {
	n := len(idx.lines)
	if n == 0 || query == "" {
		return Match{}, false
	}

	caseSensitive := hasUppercase(query)

	// Wrap fromLine into [0, n-1] using modular arithmetic.
	fromLine = ((fromLine % n) + n) % n

	for i := 0; i < n; i++ {
		var candidate int
		if dir == SearchNext {
			candidate = (fromLine + i) % n
		} else {
			candidate = (fromLine - i%n + n) % n
		}
		start, end, ok := findMatchSpan(idx.lines[candidate], query, caseSensitive)
		if ok {
			return Match{Line: candidate, Start: start, End: end}, true
		}
	}
	return Match{}, false
}

func hasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func findMatchSpan(line, query string, caseSensitive bool) (int, int, bool) {
	if caseSensitive {
		start := strings.Index(line, query)
		if start < 0 {
			return 0, 0, false
		}
		return start, start + len(query), true
	}

	lineRunes := []rune(strings.ToLower(line))
	queryRunes := []rune(strings.ToLower(query))
	if len(queryRunes) == 0 || len(queryRunes) > len(lineRunes) {
		return 0, 0, false
	}
	for startRune := 0; startRune <= len(lineRunes)-len(queryRunes); startRune++ {
		if string(lineRunes[startRune:startRune+len(queryRunes)]) != string(queryRunes) {
			continue
		}
		start := byteOffsetForRune(line, startRune)
		end := byteOffsetForRune(line, startRune+len(queryRunes))
		return start, end, true
	}
	return 0, 0, false
}

func byteOffsetForRune(s string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	count := 0
	for byteIdx := range s {
		if count == runeIdx {
			return byteIdx
		}
		count++
	}
	return len(s)
}
