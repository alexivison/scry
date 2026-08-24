package notes

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ledgerVersion = 1

func NewStore(worktree, configDir string) (*Store, error) {
	abs, err := filepath.Abs(worktree)
	if err != nil {
		return nil, noteError("invalid_worktree", err.Error())
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, noteError("invalid_worktree", err.Error())
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		if err != nil {
			return nil, noteError("invalid_worktree", err.Error())
		}
		return nil, noteError("invalid_worktree", "worktree is not a directory")
	}

	digest := sha256.Sum256([]byte(canonical))
	dir := filepath.Join(configDir, "scry", "notes", "v1")
	ledgerPath := filepath.Join(dir, hex.EncodeToString(digest[:])+".json")
	return &Store{
		worktree:   canonical,
		ledgerPath: ledgerPath,
		lockPath:   ledgerPath + ".lock",
	}, nil
}

func (s *Store) List(filter *State) ([]Note, error) {
	if filter != nil && !validState(*filter) {
		return nil, noteError("invalid_state", fmt.Sprintf("invalid state %q", *filter))
	}
	ledger, err := s.load()
	if err != nil {
		return nil, err
	}
	notes := make([]Note, 0, len(ledger.Notes))
	for _, note := range ledger.Notes {
		if filter == nil || note.State == *filter {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

func (s *Store) Add(input AddInput) (Note, error) {
	if input.Body == "" {
		return Note{}, noteError("invalid_arguments", "body must not be empty")
	}
	if !validAuthor(input.Author) {
		return Note{}, noteError("invalid_author", fmt.Sprintf("invalid author %q", input.Author))
	}
	file, fingerprint, err := s.fingerprint(input.File, input.Line)
	if err != nil {
		return Note{}, err
	}

	var note Note
	if err := s.mutate(func(ledger *ledger) (bool, error) {
		id, err := newID()
		if err != nil {
			return false, noteError("storage", err.Error())
		}
		now := time.Now().UTC()
		note = Note{
			ID:              id,
			File:            file,
			Line:            input.Line,
			LineFingerprint: fingerprint,
			Body:            input.Body,
			Author:          input.Author,
			State:           StateOpen,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		ledger.Notes = append(ledger.Notes, note)
		return true, nil
	}); err != nil {
		return Note{}, err
	}
	return note, nil
}

func (s *Store) Edit(id string, input EditInput) (Note, error) {
	if input.Body == nil && input.File == nil && input.Line == nil && input.State == nil {
		return Note{}, noteError("invalid_arguments", "no note fields supplied")
	}
	if input.Body != nil && *input.Body == "" {
		return Note{}, noteError("invalid_arguments", "body must not be empty")
	}
	if (input.File == nil) != (input.Line == nil) {
		return Note{}, noteError("invalid_arguments", "file and line must be supplied together")
	}
	if input.State != nil && !validState(*input.State) {
		return Note{}, noteError("invalid_state", fmt.Sprintf("invalid state %q", *input.State))
	}

	file, fingerprint := "", ""
	if input.File != nil {
		var err error
		file, fingerprint, err = s.fingerprint(*input.File, *input.Line)
		if err != nil {
			return Note{}, err
		}
	}

	var edited Note
	err := s.mutate(func(ledger *ledger) (bool, error) {
		for index := range ledger.Notes {
			note := &ledger.Notes[index]
			if note.ID != id {
				continue
			}
			changed := false
			if input.Body != nil && note.Body != *input.Body {
				note.Body = *input.Body
				changed = true
			}
			if input.File != nil && (note.File != file || note.Line != *input.Line || note.LineFingerprint != fingerprint) {
				note.File = file
				note.Line = *input.Line
				note.LineFingerprint = fingerprint
				changed = true
			}
			if input.State != nil && note.State != *input.State {
				note.State = *input.State
				changed = true
			}
			if changed {
				note.UpdatedAt = time.Now().UTC()
			}
			edited = *note
			return changed, nil
		}
		return false, noteError("note_not_found", fmt.Sprintf("note %q does not exist", id))
	})
	if err != nil {
		return Note{}, err
	}
	return edited, nil
}

func (s *Store) Remove(id string) (Note, error) {
	var removed Note
	err := s.mutate(func(ledger *ledger) (bool, error) {
		for index, note := range ledger.Notes {
			if note.ID != id {
				continue
			}
			removed = note
			ledger.Notes = append(ledger.Notes[:index], ledger.Notes[index+1:]...)
			return true, nil
		}
		return false, noteError("note_not_found", fmt.Sprintf("note %q does not exist", id))
	})
	if err != nil {
		return Note{}, err
	}
	return removed, nil
}

func (s *Store) Sync() (SyncResult, error) {
	var result SyncResult
	err := s.mutate(func(ledger *ledger) (bool, error) {
		changed := false
		for index := range ledger.Notes {
			note := &ledger.Notes[index]
			if note.State != StateOpen {
				continue
			}
			result.Checked++
			_, fingerprint, err := s.fingerprint(note.File, note.Line)
			if err != nil && !hasErrorCode(err, "invalid_anchor") {
				return false, err
			}
			if err == nil && fingerprint == note.LineFingerprint {
				continue
			}
			note.State = StateStale
			note.UpdatedAt = time.Now().UTC()
			result.Staled = append(result.Staled, *note)
			changed = true
		}
		return changed, nil
	})
	if err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

func (s *Store) mutate(change func(*ledger) (bool, error)) error {
	if err := os.MkdirAll(filepath.Dir(s.ledgerPath), 0o700); err != nil {
		return noteError("storage", err.Error())
	}
	if err := os.Chmod(filepath.Dir(s.ledgerPath), 0o700); err != nil {
		return noteError("storage", err.Error())
	}
	lock, err := acquireLock(s.lockPath)
	if err != nil {
		return err
	}
	defer lock.Close()

	ledger, err := s.load()
	if err != nil {
		return err
	}
	changed, err := change(&ledger)
	if err != nil || !changed {
		return err
	}
	return s.save(ledger)
}

func hasErrorCode(err error, code string) bool {
	var noteErr *Error
	return errors.As(err, &noteErr) && noteErr.Code == code
}

func (s *Store) load() (ledger, error) {
	data, err := os.ReadFile(s.ledgerPath)
	if errors.Is(err, os.ErrNotExist) {
		return ledger{Version: ledgerVersion, Worktree: s.worktree, Notes: []Note{}}, nil
	}
	if err != nil {
		return ledger{}, noteError("storage", err.Error())
	}
	var loaded ledger
	if err := json.Unmarshal(data, &loaded); err != nil {
		return ledger{}, noteError("corrupt_ledger", "ledger is not valid JSON")
	}
	if err := validateLedger(loaded, s.worktree); err != nil {
		return ledger{}, noteError("corrupt_ledger", err.Error())
	}
	return loaded, nil
}

func (s *Store) save(ledger ledger) error {
	file, err := os.CreateTemp(filepath.Dir(s.ledgerPath), ".tmp-*")
	if err != nil {
		return noteError("storage", err.Error())
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return noteError("storage", err.Error())
	}
	if err := json.NewEncoder(file).Encode(ledger); err != nil {
		file.Close()
		return noteError("storage", err.Error())
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return noteError("storage", err.Error())
	}
	if err := file.Close(); err != nil {
		return noteError("storage", err.Error())
	}
	if err := os.Rename(tempPath, s.ledgerPath); err != nil {
		return noteError("storage", err.Error())
	}
	return nil
}

func (s *Store) fingerprint(file string, line int) (string, string, error) {
	if line <= 0 {
		return "", "", noteError("invalid_anchor", "line must be positive")
	}
	if !filepath.IsLocal(file) {
		return "", "", noteError("invalid_anchor", fmt.Sprintf("file %q must be repository-relative", file))
	}
	stored := filepath.ToSlash(filepath.Clean(file))
	candidate := filepath.Join(s.worktree, filepath.FromSlash(stored))
	canonical, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", noteError("invalid_anchor", fmt.Sprintf("file %s does not exist", stored))
		}
		return "", "", noteError("storage", err.Error())
	}
	rel, err := filepath.Rel(s.worktree, canonical)
	if err != nil {
		return "", "", noteError("storage", err.Error())
	}
	if !filepath.IsLocal(rel) {
		return "", "", noteError("invalid_anchor", fmt.Sprintf("file %s escapes worktree", stored))
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", "", noteError("storage", err.Error())
	}
	if !info.Mode().IsRegular() {
		return "", "", noteError("invalid_anchor", fmt.Sprintf("file %s is not a regular file", stored))
	}

	fingerprint, err := sourceLineFingerprint(canonical, line)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", "", noteError("invalid_anchor", fmt.Sprintf("line %d does not exist in %s", line, stored))
		}
		return "", "", noteError("storage", err.Error())
	}
	return stored, fingerprint, nil
}

func sourceLineFingerprint(path string, target int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return lineFingerprint(file, target)
}

func lineFingerprint(source io.Reader, target int) (string, error) {
	reader := bufio.NewReader(source)
	hash := sha256.New()
	pendingCR := false
	seenTarget := false
	for line := 1; ; {
		content, err := reader.ReadSlice('\n')
		if line != target {
			if err == nil {
				line++
				continue
			}
			if errors.Is(err, io.EOF) {
				return "", io.EOF
			}
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			return "", err
		}

		seenTarget = seenTarget || len(content) > 0
		lineEnded := err == nil
		if lineEnded {
			content = content[:len(content)-1]
		}
		if pendingCR {
			if !lineEnded || len(content) > 0 {
				hash.Write([]byte{'\r'})
			}
			pendingCR = false
		}
		if len(content) > 0 && content[len(content)-1] == '\r' {
			content = content[:len(content)-1]
			pendingCR = true
		}
		hash.Write(content)

		if lineEnded {
			return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
		}
		if errors.Is(err, io.EOF) {
			if !seenTarget {
				return "", io.EOF
			}
			if pendingCR {
				hash.Write([]byte{'\r'})
			}
			return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return "", err
		}
	}
}

func validateLedger(ledger ledger, worktree string) error {
	if ledger.Version != ledgerVersion {
		return fmt.Errorf("unsupported ledger version")
	}
	if ledger.Worktree != worktree {
		return fmt.Errorf("ledger worktree does not match store")
	}
	if ledger.Notes == nil {
		return fmt.Errorf("ledger notes are missing")
	}
	ids := make(map[string]struct{}, len(ledger.Notes))
	for _, note := range ledger.Notes {
		if err := validateNote(note); err != nil {
			return err
		}
		if _, exists := ids[note.ID]; exists {
			return fmt.Errorf("ledger contains duplicate note ID")
		}
		ids[note.ID] = struct{}{}
	}
	return nil
}

func validateNote(note Note) error {
	if len(note.ID) != 32 {
		return fmt.Errorf("invalid note ID")
	}
	if _, err := hex.DecodeString(note.ID); err != nil {
		return fmt.Errorf("invalid note ID")
	}
	if !filepath.IsLocal(note.File) || filepath.ToSlash(filepath.Clean(note.File)) != note.File {
		return fmt.Errorf("invalid note file")
	}
	if note.Line <= 0 || note.Body == "" || !validAuthor(note.Author) || !validState(note.State) {
		return fmt.Errorf("invalid note record")
	}
	if !strings.HasPrefix(note.LineFingerprint, "sha256:") || len(note.LineFingerprint) != len("sha256:")+64 {
		return fmt.Errorf("invalid note fingerprint")
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(note.LineFingerprint, "sha256:")); err != nil {
		return fmt.Errorf("invalid note fingerprint")
	}
	if note.CreatedAt.IsZero() || note.UpdatedAt.IsZero() {
		return fmt.Errorf("invalid note timestamps")
	}
	return nil
}

func validAuthor(author Author) bool {
	return author == AuthorUser || author == AuthorAgent
}

func validState(state State) bool {
	return state == StateOpen || state == StateResolved || state == StateStale
}

func newID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
