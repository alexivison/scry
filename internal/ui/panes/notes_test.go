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

func TestPatchViewportCurrentSourceLineSkipsPatchChrome(t *testing.T) {
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 20

	if line, ok := vp.CurrentSourceLine(); !ok || line != 1 {
		t.Fatalf("CurrentSourceLine = %d, %v, want first source line after header", line, ok)
	}
}

func TestPatchViewportRendersSourceCursorOnCurrentLine(t *testing.T) {
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 20

	first := vp.Render()
	vp.MoveSourceCursor(1)
	second := vp.Render()
	firstCursor := renderedLine(ansi.Strip(first), "▌")
	secondCursor := renderedLine(ansi.Strip(second), "▌")
	if !strings.Contains(firstCursor, "package main") || !strings.Contains(secondCursor, `import "os"`) {
		t.Fatalf("source cursor did not move between current lines: before=%q after=%q", firstCursor, secondCursor)
	}
}

func TestPatchViewportSourceCursorDistinguishesFolderLines(t *testing.T) {
	patch := model.FilePatch{Hunks: []model.Hunk{
		{FilePath: "a.go", Lines: []model.DiffLine{{Kind: model.LineAdded, NewNo: intP(1), Text: "first"}}},
		{FilePath: "b.go", Lines: []model.DiffLine{{Kind: model.LineAdded, NewNo: intP(1), Text: "second"}}},
	}}
	vp := NewPatchViewport(patch)
	vp.Width = 60
	vp.Height = 20

	first := ansi.Strip(vp.Render())
	if strings.Count(first, "▌") != 1 || !strings.Contains(renderedLine(first, "▌"), "first") {
		t.Fatalf("initial folder cursor is ambiguous:\n%s", first)
	}
	vp.MoveSourceCursor(1)
	second := ansi.Strip(vp.Render())
	if strings.Count(second, "▌") != 1 || !strings.Contains(renderedLine(second, "▌"), "second") {
		t.Fatalf("folder cursor did not reach second file:\n%s", second)
	}
}

func TestPatchViewportKeepsSourceCursorVisibleAcrossDiffModes(t *testing.T) {
	lines := make([]model.DiffLine, 0, 24)
	for i := 1; i <= 12; i++ {
		lines = append(lines, model.DiffLine{Kind: model.LineDeleted, OldNo: intP(i), Text: "old"})
	}
	for i := 1; i <= 12; i++ {
		lines = append(lines, model.DiffLine{Kind: model.LineAdded, NewNo: intP(i), Text: "new"})
	}
	vp := NewPatchViewport(model.FilePatch{Hunks: []model.Hunk{{Lines: lines}}})
	vp.DiffMode = model.PatchDiffModeSideBySide
	vp.Width = 80
	vp.Height = 6
	for range 10 {
		vp.MoveSourceCursor(1)
	}

	vp.SetDiffMode(model.PatchDiffModeUnified)
	if output := ansi.Strip(vp.Render()); strings.Count(output, "▌") != 1 {
		t.Fatalf("source cursor left viewport after diff-mode change:\n%s", output)
	}
}

func TestPatchViewportKeepsSelectedNoteVisibleAcrossDiffModes(t *testing.T) {
	lines := make([]model.DiffLine, 0, 24)
	for i := 1; i <= 12; i++ {
		lines = append(lines, model.DiffLine{Kind: model.LineDeleted, OldNo: intP(i), Text: "old"})
	}
	for i := 1; i <= 12; i++ {
		lines = append(lines, model.DiffLine{Kind: model.LineAdded, NewNo: intP(i), Text: "new"})
	}
	vp := NewPatchViewport(model.FilePatch{Hunks: []model.Hunk{{Lines: lines}}})
	vp.DiffMode = model.PatchDiffModeSideBySide
	vp.Width = 80
	vp.Height = 6
	note := noteFixture("selected", 11, notes.StateOpen, "keep me visible")
	vp.SetNotes([]notes.Note{note}, note.ID, nil)
	vp.ScrollToNote(note.ID)

	vp.SetDiffMode(model.PatchDiffModeUnified)
	if output := ansi.Strip(vp.Render()); !strings.Contains(output, note.Body) {
		t.Fatalf("selected note left viewport after diff-mode change:\n%s", output)
	}
}

func TestPatchViewportPageScrollStaysInsideLongNote(t *testing.T) {
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 5
	items := []notes.Note{noteFixture("long", 1, notes.StateOpen, strings.Repeat("body row\n", 20))}
	vp.SetNotes(items, items[0].ID, nil)
	vp.Render()

	vp.PageDown()
	want := vp.ScrollOffset
	vp.SetNotes(items, "", nil)
	vp.Render()
	if vp.ScrollOffset != want {
		t.Fatalf("render moved page scroll from %d to %d", want, vp.ScrollOffset)
	}
}

func TestPatchViewportNoteRefreshKeepsSourceCursorVisible(t *testing.T) {
	vp := NewPatchViewport(notePatch())
	vp.Width = 60
	vp.Height = 3
	vp.MoveSourceCursor(1)
	vp.Render()

	vp.SetNotes([]notes.Note{noteFixture("inserted", 1, notes.StateOpen, strings.Repeat("new card row\n", 10))}, "", nil)
	if output := ansi.Strip(vp.Render()); strings.Count(output, "▌") != 1 {
		t.Fatalf("note refresh pushed source cursor offscreen:\n%s", output)
	}
}

func renderedLine(output, body string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, body) {
			return line
		}
	}
	return ""
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
	output := ansi.Strip(RenderNoteList(items, second.ID, nil, 60, 3, offset))
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
