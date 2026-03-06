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
	n := len(idx.lines)
	if n == 0 || query == "" {
		return 0, false
	}

	caseSensitive := hasUppercase(query)
	matcher := substringMatcher(query, caseSensitive)

	// Wrap fromLine into [0, n-1] using modular arithmetic.
	fromLine = ((fromLine % n) + n) % n

	for i := 0; i < n; i++ {
		var candidate int
		if dir == SearchNext {
			candidate = (fromLine + i) % n
		} else {
			candidate = (fromLine - i%n + n) % n
		}
		if matcher(idx.lines[candidate]) {
			return candidate, true
		}
	}
	return 0, false
}

func hasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func substringMatcher(query string, caseSensitive bool) func(string) bool {
	if caseSensitive {
		return func(line string) bool {
			return strings.Contains(line, query)
		}
	}
	lower := strings.ToLower(query)
	return func(line string) bool {
		return strings.Contains(strings.ToLower(line), lower)
	}
}
