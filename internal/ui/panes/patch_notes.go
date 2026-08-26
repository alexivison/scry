package panes

import (
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/notes"
)

type noteVisualRow struct {
	id   string
	text string
}

type NoteDraftView struct {
	NoteID string
	File   string
	Line   int
	Body   string
}

func (vp *PatchViewport) SetNotes(items []notes.Note, selectedID string, draft *NoteDraftView) {
	changed := vp.selectedNoteID != selectedID || !sameNoteDraft(vp.noteDraft, draft)
	if !slices.Equal(vp.notes, items) {
		next := append([]notes.Note(nil), items...)
		sort.SliceStable(next, func(i, j int) bool {
			if next[i].File != next[j].File {
				return next[i].File < next[j].File
			}
			if next[i].Line != next[j].Line {
				return next[i].Line < next[j].Line
			}
			if !next[i].CreatedAt.Equal(next[j].CreatedAt) {
				return next[i].CreatedAt.Before(next[j].CreatedAt)
			}
			return next[i].ID < next[j].ID
		})
		if !slices.Equal(vp.notes, next) {
			vp.notes = next
			changed = true
		}
	}
	vp.selectedNoteID = selectedID
	vp.noteDraft = draft
	vp.visibilityDirty = vp.visibilityDirty || changed
}

func (vp *PatchViewport) NoteBodyWidth() int {
	return max(vp.Width-vp.noteIndent()-4, 1)
}

func (vp *PatchViewport) KeepScroll(offset int) {
	vp.ScrollOffset = offset
	vp.visibilityDirty = false
	vp.manualScroll = true
}

func sameNoteDraft(a, b *NoteDraftView) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (vp *PatchViewport) withNotes(rows []visualRow) []visualRow {
	if len(vp.notes) == 0 && vp.noteDraft == nil {
		return rows
	}

	type anchor struct {
		file string
		line int
	}
	byAnchor := make(map[anchor][]notes.Note, len(vp.notes))
	byLine := make(map[int][]notes.Note, len(vp.notes))
	for _, note := range vp.notes {
		if note.State != notes.StateOpen {
			continue
		}
		key := anchor{file: note.File, line: note.Line}
		byAnchor[key] = append(byAnchor[key], note)
		byLine[note.Line] = append(byLine[note.Line], note)
	}

	indent := vp.noteIndent()
	noteWidth := max(vp.Width-indent, 0)
	prefix := strings.Repeat(" ", indent)
	out := make([]visualRow, 0, len(rows)+len(vp.notes)*3)
	for i, row := range rows {
		out = append(out, row)
		line := row.newLineNo()
		continuesSourceLine := i+1 < len(rows) && sameSourceLine(row, rows[i+1])
		if line == nil || continuesSourceLine {
			continue
		}
		key := anchor{file: row.sourceFile(), line: *line}
		matching := byAnchor[key]
		if key.file == "" {
			matching = byLine[key.line]
		}
		draftAdded := false
		for _, note := range matching {
			isDraft := vp.noteDraft != nil && vp.noteDraft.NoteID == note.ID
			if isDraft {
				note.Body = vp.noteDraft.Body
				draftAdded = true
			}
			for _, text := range renderNoteCard(note, noteWidth, note.ID == vp.selectedNoteID || isDraft) {
				out = append(out, visualRow{note: &noteVisualRow{id: note.ID, text: prefix + text}})
			}
		}
		draftMatchesFile := false
		draftMatchesLine := false
		if vp.noteDraft != nil {
			draftMatchesFile = key.file == "" || vp.noteDraft.File == key.file
			draftMatchesLine = vp.noteDraft.Line == key.line
		}
		draftMatchesAnchor := draftMatchesFile && draftMatchesLine
		if draftMatchesAnchor && !draftAdded {
			draft := notes.Note{File: vp.noteDraft.File, Line: vp.noteDraft.Line, Body: vp.noteDraft.Body, Author: notes.AuthorUser, State: notes.StateOpen}
			for _, text := range renderNoteCard(draft, noteWidth, true) {
				out = append(out, visualRow{note: &noteVisualRow{text: prefix + text}})
			}
		}
	}
	return out
}

func (vp *PatchViewport) noteIndent() int {
	indent := 1
	if vp.DiffMode == model.PatchDiffModeSideBySide {
		left, _ := sideBySideColumnWidths(vp.Width)
		indent += left + lipgloss.Width(sideBySideSeparator())
		if vp.GutterVisible {
			indent += lipgloss.Width(formatSideGutter(nil, vp.gutterDigits))
		}
	} else if vp.GutterVisible {
		indent += lipgloss.Width(formatGutter(nil, nil, vp.gutterDigits))
	}
	return min(indent, max(vp.Width-4, 0))
}

func sameSourceLine(a, b visualRow) bool {
	aLine, bLine := a.newLineNo(), b.newLineNo()
	return aLine != nil && bLine != nil && a.sourceFile() == b.sourceFile() && *aLine == *bLine
}

func (vp *PatchViewport) ensureNoteVisible(rows []visualRow, id string) {
	first, last := -1, -1
	for i, row := range rows {
		if row.note == nil || row.note.id != id {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 || vp.Height <= 0 {
		return
	}
	if first < vp.ScrollOffset || last-first+1 > vp.Height {
		vp.ScrollOffset = first
	} else if last >= vp.ScrollOffset+vp.Height {
		vp.ScrollOffset = last - vp.Height + 1
	}
}

func (vp *PatchViewport) ensureActiveVisible(rows []visualRow) {
	switch {
	case vp.noteDraft != nil:
		vp.ensureNoteVisible(rows, vp.noteDraft.NoteID)
	case vp.selectedNoteID != "":
		vp.ensureNoteVisible(rows, vp.selectedNoteID)
	default:
		vp.ensureSourceCursorVisible(rows)
	}
}

func (vp *PatchViewport) ScrollToNote(id string) bool {
	for i, row := range vp.visualRows() {
		if row.note != nil && row.note.id == id {
			vp.ScrollOffset = i
			vp.manualScroll = false
			vp.SyncCurrentHunk()
			return true
		}
	}
	return false
}
