package notes

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListFreshLedgerDoesNotCreateFile(t *testing.T) {
	t.Parallel()

	store, root := newTestStore(t)
	notes, err := store.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %v, want empty", notes)
	}
	if _, err := os.Stat(store.ledgerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ledger stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(root, "source.go")); err != nil {
		t.Fatal(err)
	}
}

func TestAddPersistsForSecondStore(t *testing.T) {
	t.Parallel()

	store, root := newTestStore(t)
	added, err := store.Add(AddInput{File: "source.go", Line: 2, Body: "keep it simple", Author: AuthorUser})
	if err != nil {
		t.Fatal(err)
	}
	if added.ID == "" || added.LineFingerprint == "" {
		t.Fatalf("added = %#v, want ID and fingerprint", added)
	}

	again, err := NewStore(root, filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(store.ledgerPath)))))
	if err != nil {
		t.Fatal(err)
	}
	notes, err := again.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(notes, []Note{added}) {
		t.Fatalf("notes = %#v, want %#v", notes, []Note{added})
	}
}

func TestAddAcceptsBothPermittedAuthors(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t)
	for _, author := range []Author{AuthorUser, AuthorAgent} {
		if _, err := store.Add(AddInput{File: "source.go", Line: 1, Body: string(author), Author: author}); err != nil {
			t.Fatalf("Add(%q): %v", author, err)
		}
	}
}

func TestEditInputCannotChangeAuthor(t *testing.T) {
	t.Parallel()

	if _, ok := reflect.TypeOf(EditInput{}).FieldByName("Author"); ok {
		t.Fatal("EditInput must not allow author changes")
	}
}

func TestEditChangesOnlySuppliedFields(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t)
	note := mustAdd(t, store, AddInput{File: "source.go", Line: 1, Body: "before", Author: AuthorAgent})
	body := "after"
	edited, err := store.Edit(note.ID, EditInput{Body: &body})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Body != body || edited.File != note.File || edited.Line != note.Line || edited.LineFingerprint != note.LineFingerprint || edited.Author != note.Author || edited.State != note.State {
		t.Fatalf("body edit = %#v, want only body changed from %#v", edited, note)
	}
	if !edited.UpdatedAt.After(note.UpdatedAt) {
		t.Fatalf("updatedAt = %s, want after %s", edited.UpdatedAt, note.UpdatedAt)
	}

	state := StateResolved
	resolved, err := store.Edit(note.ID, EditInput{State: &state})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.State != StateResolved || resolved.Body != body || resolved.Author != note.Author || resolved.LineFingerprint != note.LineFingerprint {
		t.Fatalf("state edit = %#v, want only state changed", resolved)
	}
	if !resolved.UpdatedAt.After(edited.UpdatedAt) {
		t.Fatalf("updatedAt = %s, want after %s", resolved.UpdatedAt, edited.UpdatedAt)
	}
	before, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := store.Edit(note.ID, EditInput{State: &state})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unchanged, resolved) {
		t.Fatalf("unchanged edit = %#v, want %#v", unchanged, resolved)
	}
	after, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("unchanged edit rewrote ledger")
	}
}

func TestEditAnchorRequiresFileAndLineTogether(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t)
	note := mustAdd(t, store, AddInput{File: "source.go", Line: 1, Body: "before", Author: AuthorUser})
	file := "source.go"
	line := 2
	for name, input := range map[string]EditInput{
		"file only": {File: &file},
		"line only": {Line: &line},
	} {
		t.Run(name, func(t *testing.T) {
			before, err := os.ReadFile(store.ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Edit(note.ID, input); err == nil {
				t.Fatal("Edit succeeded, want invalid arguments error")
			}
			after, err := os.ReadFile(store.ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatal("invalid anchor edit changed ledger bytes")
			}
		})
	}
}

func TestEditRepairKeepsStaleUntilExplicitlyOpened(t *testing.T) {
	t.Parallel()

	store, root := newTestStore(t)
	note := mustAdd(t, store, AddInput{File: "source.go", Line: 2, Body: "before", Author: AuthorUser})
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("first\nchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Sync(); err != nil {
		t.Fatal(err)
	}

	file := "source.go"
	line := 1
	repaired, err := store.Edit(note.ID, EditInput{File: &file, Line: &line})
	if err != nil {
		t.Fatal(err)
	}
	if repaired.State != StateStale {
		t.Fatalf("repaired state = %q, want stale without explicit state", repaired.State)
	}

	state := StateOpen
	opened, err := store.Edit(note.ID, EditInput{File: &file, Line: &line, State: &state})
	if err != nil {
		t.Fatal(err)
	}
	if opened.State != StateOpen {
		t.Fatalf("explicitly opened state = %q, want open", opened.State)
	}
}

func TestRemoveReturnsAndDeletesSpecifiedNote(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t)
	first := mustAdd(t, store, AddInput{File: "source.go", Line: 1, Body: "first", Author: AuthorUser})
	second := mustAdd(t, store, AddInput{File: "source.go", Line: 2, Body: "second", Author: AuthorAgent})
	removed, err := store.Remove(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(removed, first) {
		t.Fatalf("removed = %#v, want %#v", removed, first)
	}
	remaining, err := store.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(remaining, []Note{second}) {
		t.Fatalf("remaining = %#v, want %#v", remaining, []Note{second})
	}

	before, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Remove("00000000000000000000000000000000"); err == nil {
		t.Fatal("Remove succeeded for missing ID")
	}
	after, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("missing Remove changed ledger bytes")
	}
}

func TestSyncStalesChangedAndMissingOpenAnchors(t *testing.T) {
	t.Parallel()

	store, root := newTestStore(t)
	writeTestSource(t, root, "changed.go", "before\n")
	writeTestSource(t, root, "deleted.go", "present\n")
	writeTestSource(t, root, "short.go", "one\ntwo\n")
	writeTestSource(t, root, "match.go", "same\n")
	writeTestSource(t, root, "resolved.go", "before\n")
	changed := mustAdd(t, store, AddInput{File: "changed.go", Line: 1, Body: "changed", Author: AuthorUser})
	deleted := mustAdd(t, store, AddInput{File: "deleted.go", Line: 1, Body: "deleted", Author: AuthorUser})
	short := mustAdd(t, store, AddInput{File: "short.go", Line: 2, Body: "short", Author: AuthorUser})
	matching := mustAdd(t, store, AddInput{File: "match.go", Line: 1, Body: "matching", Author: AuthorUser})
	resolved := mustAdd(t, store, AddInput{File: "resolved.go", Line: 1, Body: "resolved", Author: AuthorUser})
	state := StateResolved
	resolved, err := store.Edit(resolved.ID, EditInput{State: &state})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "changed.go"), []byte("after\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "deleted.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "short.go"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := store.Sync()
	if err != nil {
		t.Fatal(err)
	}
	if result.Checked != 4 {
		t.Fatalf("checked = %d, want 4", result.Checked)
	}
	if len(result.Staled) != 3 {
		t.Fatalf("staled = %#v, want three notes", result.Staled)
	}
	byID := notesByID(t, store)
	for _, note := range []Note{changed, deleted, short} {
		if byID[note.ID].State != StateStale || !byID[note.ID].UpdatedAt.After(note.UpdatedAt) {
			t.Fatalf("staled note %q = %#v", note.ID, byID[note.ID])
		}
	}
	if byID[matching.ID].State != StateOpen {
		t.Fatalf("matching state = %q, want open", byID[matching.ID].State)
	}
	if !byID[matching.ID].UpdatedAt.Equal(matching.UpdatedAt) {
		t.Fatalf("matching updatedAt = %s, want unchanged %s", byID[matching.ID].UpdatedAt, matching.UpdatedAt)
	}
	if byID[resolved.ID].State != StateResolved {
		t.Fatalf("resolved state = %q, want resolved", byID[resolved.ID].State)
	}
	if !byID[resolved.ID].UpdatedAt.Equal(resolved.UpdatedAt) {
		t.Fatalf("resolved updatedAt = %s, want unchanged %s", byID[resolved.ID].UpdatedAt, resolved.UpdatedAt)
	}
}

func TestSyncWithoutStateChangesDoesNotRewriteLedger(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t)
	mustAdd(t, store, AddInput{File: "source.go", Line: 1, Body: "matching", Author: AuthorUser})
	before, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Sync()
	if err != nil {
		t.Fatal(err)
	}
	if result.Checked != 1 || len(result.Staled) != 0 {
		t.Fatalf("sync result = %#v, want one checked and none staled", result)
	}
	after, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("unchanged sync rewrote ledger")
	}
}

func TestCanonicalEquivalentWorktreesShareLedger(t *testing.T) {
	t.Parallel()

	store, root := newTestStore(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	viaAlias, err := NewStore(alias, filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(store.ledgerPath)))))
	if err != nil {
		t.Fatal(err)
	}
	if store.ledgerPath != viaAlias.ledgerPath || store.worktree != viaAlias.worktree {
		t.Fatalf("stores do not share canonical scope: %#v %#v", store, viaAlias)
	}
}

func TestDifferentWorktreesHaveDifferentLedgers(t *testing.T) {
	t.Parallel()

	first, _ := newTestStore(t)
	second, err := NewStore(t.TempDir(), filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(first.ledgerPath)))))
	if err != nil {
		t.Fatal(err)
	}
	if first.ledgerPath == second.ledgerPath {
		t.Fatalf("ledger path = %q for different worktrees", first.ledgerPath)
	}
}

func TestRejectedAddsLeaveLedgerUnchanged(t *testing.T) {
	t.Parallel()

	store, root := newTestStore(t)
	if _, err := store.Add(AddInput{File: "source.go", Line: 1, Body: "saved", Author: AuthorUser}); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.go")); err != nil {
		t.Fatal(err)
	}

	tests := map[string]AddInput{
		"empty body":     {File: "source.go", Line: 1, Body: "", Author: AuthorUser},
		"invalid author": {File: "source.go", Line: 1, Body: "bad", Author: "bot"},
		"negative line":  {File: "source.go", Line: -1, Body: "bad", Author: AuthorUser},
		"zero line":      {File: "source.go", Line: 0, Body: "bad", Author: AuthorUser},
		"missing line":   {File: "source.go", Line: 9, Body: "bad", Author: AuthorUser},
		"absolute path":  {File: filepath.Join(root, "source.go"), Line: 1, Body: "bad", Author: AuthorUser},
		"traversal path": {File: "../outside.go", Line: 1, Body: "bad", Author: AuthorUser},
		"symlink escape": {File: "escape.go", Line: 1, Body: "bad", Author: AuthorUser},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			beforeBytes, err := os.ReadFile(store.ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			beforeNotes, err := store.List(nil)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := store.Add(input); err == nil {
				t.Fatal("Add succeeded, want validation error")
			}
			afterBytes, err := os.ReadFile(store.ledgerPath)
			if err != nil {
				t.Fatal(err)
			}
			afterNotes, err := store.List(nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(afterBytes, beforeBytes) {
				t.Fatal("rejected Add changed ledger bytes")
			}
			if !reflect.DeepEqual(afterNotes, beforeNotes) {
				t.Fatal("rejected Add changed listed notes")
			}
		})
	}
}

func TestLineFingerprintStreamsTargetLine(t *testing.T) {
	t.Parallel()

	const lineLength = 8 << 20
	fingerprint, err := lineFingerprint(&longLineReader{remaining: lineLength, suffix: []byte("\nnext\n")}, 1)
	if err != nil {
		t.Fatal(err)
	}

	hash := sha256.New()
	chunk := make([]byte, 1024)
	for i := range chunk {
		chunk[i] = 'x'
	}
	for written := 0; written < lineLength; written += len(chunk) {
		if _, err := hash.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	want := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if fingerprint != want {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, want)
	}
}

type longLineReader struct {
	remaining int
	suffix    []byte
}

func (r *longLineReader) Read(p []byte) (int, error) {
	if r.remaining > 0 {
		n := min(min(len(p), 1024), r.remaining)
		for i := range p[:n] {
			p[i] = 'x'
		}
		r.remaining -= n
		return n, nil
	}
	if len(r.suffix) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.suffix)
	r.suffix = r.suffix[n:]
	return n, nil
}

func mustAdd(t *testing.T, store *Store, input AddInput) Note {
	t.Helper()
	note, err := store.Add(input)
	if err != nil {
		t.Fatal(err)
	}
	return note
}

func writeTestSource(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func notesByID(t *testing.T, store *Store) map[string]Note {
	t.Helper()
	notes, err := store.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Note, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
	}
	return byID
}

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store, root
}
