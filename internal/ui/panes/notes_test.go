package panes

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/notes"
)

func TestPatchViewportRendersNoteAfterCurrentSourceLine(t *testing.T) {
	for _, mode := range []model.PatchDiffMode{model.PatchDiffModeUnified, model.PatchDiffModeSideBySide} {
		vp := NewPatchViewport(notePatch())
		vp.Width = 72
		vp.Height = 30
		vp.DiffMode = mode
		vp.SetNotes([]notes.Note{noteFixture("note-1", 2, notes.StateOpen, "Review this import.")}, "", nil)

		output := ansi.Strip(vp.Render())
		lineAt := strings.Index(output, `import "os"`)
		noteAt := strings.Index(output, "Review this import.")
		if lineAt < 0 || noteAt <= lineAt {
			t.Fatalf("mode %v did not render note after source line:\n%s", mode, output)
		}
	}
}

func TestPatchViewportDoesNotAttachNoteToDeletedOnlyLine(t *testing.T) {
	vp := NewPatchViewport(model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusDeleted},
		Hunks: []model.Hunk{{
			OldStart: 1,
			OldLen:   1,
			Lines: []model.DiffLine{{
				Kind:  model.LineDeleted,
				OldNo: intP(1),
				Text:  "removed()",
			}},
		}},
	})
	vp.Width = 60
	vp.Height = 20
	vp.SetNotes([]notes.Note{noteFixture("note-1", 1, notes.StateOpen, "Must not appear.")}, "", nil)

	if output := ansi.Strip(vp.Render()); strings.Contains(output, "Must not appear.") {
		t.Fatalf("deleted-only line rendered a current-source note:\n%s", output)
	}
}

func TestPatchViewportResolvedNoteExpandsOnlyWhenSelected(t *testing.T) {
	note := noteFixture("note-1", 2, notes.StateResolved, "first line\nsecond line")
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 20
	vp.Height = 30
	vp.SetNotes([]notes.Note{note}, "", nil)

	if output := ansi.Strip(vp.Render()); strings.Contains(output, "second line") {
		t.Fatalf("unselected resolved note was expanded:\n%s", output)
	}

	vp.SetNotes([]notes.Note{note}, note.ID, nil)
	if output := ansi.Strip(vp.Render()); !strings.Contains(output, "second line") {
		t.Fatalf("selected resolved note remained collapsed:\n%s", output)
	}
}

func TestResolvedNoteCollapsesLongPreviewToOneRow(t *testing.T) {
	note := noteFixture("note-1", 2, notes.StateResolved, strings.Repeat("long preview ", 20))
	if rows := renderNoteCard(note, 32, false); len(rows) != 3 {
		t.Fatalf("collapsed resolved card has %d rows, want title, preview, border", len(rows))
	}
}

func TestPatchViewportOrdersNotesOnSameLine(t *testing.T) {
	first := noteFixture("note-b", 2, notes.StateOpen, "first")
	second := noteFixture("note-a", 2, notes.StateOpen, "second")
	second.CreatedAt = first.CreatedAt.Add(time.Second)
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 30
	vp.SetNotes([]notes.Note{second, first}, "", nil)

	output := ansi.Strip(vp.Render())
	if firstAt, secondAt := strings.Index(output, "first"), strings.Index(output, "second"); firstAt < 0 || secondAt <= firstAt {
		t.Fatalf("same-line notes rendered out of order:\n%s", output)
	}
}

func TestPatchViewportNoteRowsCountTowardHeight(t *testing.T) {
	vp := NewPatchViewport(notePatch())
	vp.Width = 44
	before := vp.TotalLines()
	vp.SetNotes([]notes.Note{noteFixture("note-1", 2, notes.StateOpen, "a body that wraps across several visual rows")}, "", nil)

	if after := vp.TotalLines(); after <= before+2 {
		t.Fatalf("TotalLines after note = %d, before = %d; card rows were not counted", after, before)
	}
}

func TestPatchViewportCurrentSourceLineRequiresExactDiffRow(t *testing.T) {
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 20

	if _, ok := vp.CurrentSourceLine(); ok {
		t.Fatal("hunk header unexpectedly produced a source line")
	}
	vp.ScrollOffset = 2
	if line, ok := vp.CurrentSourceLine(); !ok || line != 2 {
		t.Fatalf("CurrentSourceLine = %d, %v, want 2, true", line, ok)
	}
}

func TestPatchViewportScrollToNote(t *testing.T) {
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 20
	vp.SetNotes([]notes.Note{noteFixture("note-1", 2, notes.StateOpen, "Find me.")}, "", nil)

	if !vp.ScrollToNote("note-1") {
		t.Fatal("ScrollToNote did not find an attached note")
	}
	if rows := vp.visibleRows(); len(rows) == 0 || rows[0].note == nil || rows[0].note.id != "note-1" {
		t.Fatalf("viewport did not land on note: %#v", rows)
	}
}

func TestRenderNoteListShowsStaleAnchorAndRespectsOffset(t *testing.T) {
	first := noteFixture("note-1", 2, notes.StateStale, "first")
	second := noteFixture("note-2", 9, notes.StateStale, "second")
	second.File = "other.go"
	items := []notes.Note{first, second}

	offset, ok := NoteListOffset(items, second.ID, 60)
	if !ok || offset == 0 {
		t.Fatalf("NoteListOffset = %d, %v; want second card offset", offset, ok)
	}
	output := ansi.Strip(RenderNoteList(items, second.ID, nil, 60, 20, offset))
	if strings.Contains(output, "first") || !strings.Contains(output, "other.go:9") || !strings.Contains(output, "second") {
		t.Fatalf("stale note list rendered wrong slice:\n%s", output)
	}
}

func notePatch() model.FilePatch {
	return model.FilePatch{
		Summary: model.FileSummary{Path: "main.go", Status: model.StatusModified},
		Hunks: []model.Hunk{{
			OldStart: 1,
			OldLen:   1,
			NewStart: 1,
			NewLen:   2,
			Lines: []model.DiffLine{
				{Kind: model.LineContext, OldNo: intP(1), NewNo: intP(1), Text: "package main"},
				{Kind: model.LineAdded, NewNo: intP(2), Text: `import "os"`},
			},
		}},
	}
}

func noteFixture(id string, line int, state notes.State, body string) notes.Note {
	return notes.Note{
		ID:        id,
		File:      "main.go",
		Line:      line,
		Body:      body,
		Author:    notes.AuthorAgent,
		State:     state,
		CreatedAt: time.Unix(1, 0),
	}
}
