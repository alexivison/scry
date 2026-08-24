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
