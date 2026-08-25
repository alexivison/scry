package panes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/notes"
	"github.com/alexivison/scry/internal/ui/theme"
)

func renderNoteCard(note notes.Note, width int, selected bool) []string {
	if width < 4 {
		return []string{padOrTruncateToWidth(note.Body, width)}
	}

	border := lipgloss.NewStyle().Foreground(theme.ChromeFaint)
	switch {
	case selected:
		border = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	case note.State == notes.StateStale:
		border = lipgloss.NewStyle().Foreground(theme.Dirty)
	case note.State == notes.StateResolved:
		border = lipgloss.NewStyle().Foreground(theme.Muted)
	}

	body := note.Body
	collapsed := note.State == notes.StateResolved && !selected
	if collapsed {
		body, _, _ = strings.Cut(body, "\n")
	}
	bodyStyle := lipgloss.NewStyle().Foreground(theme.BrightText)
	if note.State == notes.StateResolved {
		bodyStyle = bodyStyle.Foreground(theme.Muted).Faint(true)
	}

	lines := []string{noteCardTitle(note, width, border)}
	innerWidth := width - 4
	for _, logicalLine := range strings.Split(body, "\n") {
		segments := wrapBodySegments(logicalLine, innerWidth)
		if collapsed {
			segments = segments[:1]
		}
		for _, segment := range segments {
			content := padOrTruncateToWidth(segment.text, innerWidth)
			lines = append(lines, border.Render("│ ")+bodyStyle.Render(content)+border.Render(" │"))
		}
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return lines
}

func RenderNoteList(items []notes.Note, selectedID string, draft *NoteDraftView, width, height, offset int) string {
	var rows []string
	for _, note := range orderedNotes(items) {
		if draft != nil && draft.NoteID == note.ID {
			note.Body = draft.Body
		}
		selected := note.ID == selectedID
		card := renderNoteCard(note, width, selected)
		if selected {
			for i := range card {
				card[i] = renderCursorRow(card[i], width)
			}
		}
		rows = append(rows, card...)
	}
	if len(rows) == 0 || height <= 0 {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	maxOffset := max(len(rows)-height, 0)
	if offset > maxOffset {
		offset = maxOffset
	}
	end := min(offset+height, len(rows))
	return strings.Join(rows[offset:end], "\n")
}

func NoteListOffset(items []notes.Note, selectedID string, width int) (int, bool) {
	offset := 0
	for _, note := range orderedNotes(items) {
		if note.ID == selectedID {
			return offset, true
		}
		offset += len(renderNoteCard(note, width, false))
	}
	return 0, false
}

func NoteListMaxOffset(items []notes.Note, width, height int) int {
	rows := 0
	for _, note := range orderedNotes(items) {
		rows += len(renderNoteCard(note, width, false))
	}
	return max(rows-height, 0)
}

func orderedNotes(items []notes.Note) []notes.Note {
	ordered := append([]notes.Note(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].File != ordered[j].File {
			return ordered[i].File < ordered[j].File
		}
		if ordered[i].Line != ordered[j].Line {
			return ordered[i].Line < ordered[j].Line
		}
		if !ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
		}
		return ordered[i].ID < ordered[j].ID
	})
	return ordered
}

func noteCardTitle(note notes.Note, width int, style lipgloss.Style) string {
	author := string(note.Author)
	if author != "" {
		author = strings.ToUpper(author[:1]) + author[1:]
	}
	title := fmt.Sprintf("─ %s · %s ", author, note.State)
	if note.State != notes.StateOpen {
		title = fmt.Sprintf("─ %s · %s · %s:%d ", author, note.State, note.File, note.Line)
	}
	inner := width - 2
	if lipgloss.Width(title) > inner {
		title = truncateToWidth(title, inner)
	}
	return style.Render("╭" + title + strings.Repeat("─", inner-lipgloss.Width(title)) + "╮")
}
