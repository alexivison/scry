package notescli

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestRunLifecycleJSONContract(t *testing.T) {
	t.Parallel()

	options, worktree := testOptions(t)
	if err := os.WriteFile(filepath.Join(worktree, "source.go"), []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	add := run(t, []string{"add", "--file", "source.go", "--line", "2", "--body", "remember", "--author", "agent"}, options)
	if add.Code != 0 {
		t.Fatalf("add exit code = %d, stderr = %s", add.Code, add.Stderr)
	}
	added := decodeObject(t, []byte(add.Stdout))
	assertKeys(t, added, "worktree", "note")
	assertWorktree(t, added, worktree)
	note := decodeObject(t, added["note"])
	if decodeString(t, note["body"]) != "remember" || decodeString(t, note["author"]) != "agent" || decodeString(t, note["state"]) != "open" {
		t.Fatalf("added note = %s", added["note"])
	}
	id := decodeString(t, note["id"])
	if id == "" {
		t.Fatal("added note id is empty")
	}

	list := run(t, []string{"list", "--state", "open"}, options)
	if list.Code != 0 {
		t.Fatalf("list exit code = %d, stderr = %s", list.Code, list.Stderr)
	}
	listed := decodeObject(t, []byte(list.Stdout))
	assertKeys(t, listed, "worktree", "notes")
	assertWorktree(t, listed, worktree)
	var notes []map[string]any
	if err := json.Unmarshal(listed["notes"], &notes); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0]["id"] != id {
		t.Fatalf("listed notes = %s, want one added note", listed["notes"])
	}

	edit := run(t, []string{"edit", id, "--body", "updated", "--state", "resolved"}, options)
	if edit.Code != 0 {
		t.Fatalf("edit exit code = %d, stderr = %s", edit.Code, edit.Stderr)
	}
	edited := decodeObject(t, []byte(edit.Stdout))
	assertKeys(t, edited, "worktree", "note")
	if got := decodeString(t, decodeObject(t, edited["note"])["body"]); got != "updated" {
		t.Fatalf("edited body = %v, want updated", got)
	}
	filtered := run(t, []string{"list", "--state", "open"}, options)
	if filtered.Code != 0 {
		t.Fatalf("filtered list exit code = %d, stderr = %s", filtered.Code, filtered.Stderr)
	}
	var openNotes []json.RawMessage
	if err := json.Unmarshal(decodeObject(t, []byte(filtered.Stdout))["notes"], &openNotes); err != nil {
		t.Fatal(err)
	}
	if len(openNotes) != 0 {
		t.Fatalf("open notes = %s, want empty after resolution", filtered.Stdout)
	}

	sync := run(t, []string{"sync"}, options)
	if sync.Code != 0 {
		t.Fatalf("sync exit code = %d, stderr = %s", sync.Code, sync.Stderr)
	}
	synced := decodeObject(t, []byte(sync.Stdout))
	assertKeys(t, synced, "worktree", "checked", "staled")
	if decodeNumber(t, synced["checked"]) != 0 || string(synced["staled"]) != "[]" {
		t.Fatalf("sync result = %s, want no resolved-note changes", sync.Stdout)
	}

	remove := run(t, []string{"remove", id}, options)
	if remove.Code != 0 {
		t.Fatalf("remove exit code = %d, stderr = %s", remove.Code, remove.Stderr)
	}
	removed := decodeObject(t, []byte(remove.Stdout))
	assertKeys(t, removed, "worktree", "note")
	if got := decodeString(t, decodeObject(t, removed["note"])["id"]); got != id {
		t.Fatalf("removed note id = %v, want %s", got, id)
	}
}

func TestRunErrorsAreOnlyJSON(t *testing.T) {
	t.Parallel()

	options, worktree := testOptions(t)
	if err := os.WriteFile(filepath.Join(worktree, "source.go"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, test := range map[string]struct {
		args []string
		code string
	}{
		"unknown command": {[]string{"wat"}, "invalid_arguments"},
		"unknown flag":    {[]string{"list", "--wat"}, "invalid_arguments"},
		"invalid author":  {[]string{"add", "--file", "source.go", "--line", "1", "--body", "x", "--author", "bot"}, "invalid_author"},
		"invalid state":   {[]string{"list", "--state", "done"}, "invalid_state"},
		"invalid anchor":  {[]string{"add", "--file", "source.go", "--line", "0", "--body", "x", "--author", "user"}, "invalid_anchor"},
		"missing id":      {[]string{"remove"}, "invalid_arguments"},
	} {
		t.Run(name, func(t *testing.T) {
			result := run(t, test.args, options)
			if result.Code != 2 {
				t.Fatalf("exit code = %d, want 2; stderr = %q", result.Code, result.Stderr)
			}
			if result.Stdout != "" {
				t.Fatalf("stdout = %q, want empty", result.Stdout)
			}
			errDoc := decodeObject(t, []byte(result.Stderr))
			assertKeys(t, errDoc, "error")
			if got := decodeString(t, decodeObject(t, errDoc["error"])["code"]); got != test.code {
				t.Fatalf("error code = %v, want %s; stderr = %q", got, test.code, result.Stderr)
			}
		})
	}
}

func TestRunOperationalErrorsAreOnlyJSON(t *testing.T) {
	options, worktree := testOptions(t)
	if err := os.WriteFile(filepath.Join(worktree, "source.go"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := testLedgerPath(options.ConfigDir, worktree) + ".lock"

	tests := []struct {
		name     string
		args     []string
		code     string
		exitCode int
		setup    func(t *testing.T)
	}{
		{
			name:     "note not found",
			args:     []string{"remove", "00000000000000000000000000000000"},
			code:     "note_not_found",
			exitCode: 2,
		},
		{
			name:     "corrupt ledger",
			args:     []string{"list"},
			code:     "corrupt_ledger",
			exitCode: 1,
			setup: func(t *testing.T) {
				t.Helper()
				ledgerPath := testLedgerPath(options.ConfigDir, worktree)
				if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(ledgerPath, []byte("not json"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:     "busy",
			args:     []string{"add", "--file", "source.go", "--line", "1", "--body", "blocked", "--author", "user"},
			code:     "busy",
			exitCode: 1,
			setup: func(t *testing.T) {
				t.Helper()
				stop := holdLedgerLock(t, lockPath)
				t.Cleanup(stop)
			},
		},
		{
			name:     "invalid worktree",
			args:     []string{"list", "--worktree", filepath.Join(t.TempDir(), "missing")},
			code:     "invalid_worktree",
			exitCode: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.setup != nil {
				test.setup(t)
			}
			result := run(t, test.args, options)
			assertJSONError(t, result, test.exitCode, test.code)
		})
	}
}

func TestRunSetupErrorIsStorageJSON(t *testing.T) {
	t.Parallel()

	result := run(t, []string{"list"}, Options{SetupErr: errors.New("no config"), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if result.Code != 1 {
		t.Fatalf("exit code = %d, want 1", result.Code)
	}
	errDoc := decodeObject(t, []byte(result.Stderr))
	if got := decodeString(t, decodeObject(t, errDoc["error"])["code"]); got != "storage" {
		t.Fatalf("error code = %v, want storage", got)
	}
}

func TestRunHelpStopsEveryCommandWithoutMutatingNotes(t *testing.T) {
	t.Parallel()

	options, worktree := testOptions(t)
	source := filepath.Join(worktree, "source.go")
	if err := os.WriteFile(source, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	added := run(t, []string{"add", "--file", "source.go", "--line", "1", "--body", "keep", "--author", "user"}, options)
	if added.Code != 0 {
		t.Fatalf("add exit code = %d, stderr = %s", added.Code, added.Stderr)
	}
	id := decodeString(t, decodeObject(t, decodeObject(t, []byte(added.Stdout))["note"])["id"])
	if err := os.WriteFile(source, []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, args := range map[string][]string{
		"list":   {"list", "--help"},
		"add":    {"add", "--help"},
		"edit":   {"edit", id, "--help"},
		"remove": {"remove", id, "--help"},
		"sync":   {"sync", "--help"},
	} {
		t.Run(name, func(t *testing.T) {
			result := run(t, args, options)
			if result.Code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", result.Code, result.Stderr)
			}
			if result.Stderr != "" {
				t.Fatalf("stderr = %q, want empty", result.Stderr)
			}
			if result.Stdout == "" || json.Valid([]byte(result.Stdout)) || strings.Contains(result.Stdout, "{") {
				t.Fatalf("stdout = %q, want normal non-JSON help", result.Stdout)
			}
		})
	}

	listed := run(t, []string{"list"}, options)
	if listed.Code != 0 {
		t.Fatalf("list exit code = %d, stderr = %s", listed.Code, listed.Stderr)
	}
	var notes []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(decodeObject(t, []byte(listed.Stdout))["notes"], &notes); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].ID != id || notes[0].State != "open" {
		t.Fatalf("notes after help = %s, want the unchanged open note", listed.Stdout)
	}
}

type runResult struct {
	Code   int
	Stdout string
	Stderr string
}

func run(t *testing.T, args []string, options Options) runResult {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	options.Stdout = stdout
	options.Stderr = stderr
	return runResult{Run(args, options), stdout.String(), stderr.String()}
}

func testOptions(t *testing.T) (Options, string) {
	t.Helper()
	worktree := t.TempDir()
	canonical, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	return Options{WorkingDir: worktree, ConfigDir: t.TempDir()}, canonical
}

func decodeObject(t *testing.T, document []byte) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(document, &object); err != nil {
		t.Fatalf("invalid JSON %q: %v", document, err)
	}
	return object
}

func decodeString(t *testing.T, document json.RawMessage) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(document, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func decodeNumber(t *testing.T, document json.RawMessage) int {
	t.Helper()
	var value int
	if err := json.Unmarshal(document, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func assertKeys(t *testing.T, object map[string]json.RawMessage, want ...string) {
	t.Helper()
	if len(object) != len(want) {
		t.Fatalf("keys = %v, want %v", mapsKeys(object), want)
	}
	for _, key := range want {
		if _, ok := object[key]; !ok {
			t.Fatalf("keys = %v, missing %q", mapsKeys(object), key)
		}
	}
}

func assertWorktree(t *testing.T, object map[string]json.RawMessage, want string) {
	t.Helper()
	var got string
	if err := json.Unmarshal(object["worktree"], &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("worktree = %q, want %q", got, want)
	}
}

func assertJSONError(t *testing.T, result runResult, wantExit int, wantCode string) {
	t.Helper()
	if result.Code != wantExit {
		t.Fatalf("exit code = %d, want %d; stderr = %q", result.Code, wantExit, result.Stderr)
	}
	if result.Stdout != "" {
		t.Fatalf("stdout = %q, want empty", result.Stdout)
	}
	errDoc := decodeObject(t, []byte(result.Stderr))
	assertKeys(t, errDoc, "error")
	if got := decodeString(t, decodeObject(t, errDoc["error"])["code"]); got != wantCode {
		t.Fatalf("error code = %q, want %q", got, wantCode)
	}
}

func testLedgerPath(configDir, worktree string) string {
	digest := sha256.Sum256([]byte(worktree))
	return filepath.Join(configDir, "scry", "notes", "v1", hex.EncodeToString(digest[:])+".json")
}

func holdLedgerLock(t *testing.T, path string) func() {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestNotesCLILockHolderProcess$", "--", path)
	command.Env = append(os.Environ(), "GO_WANT_NOTESCLI_LOCK_HELPER=1")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatalf("lock helper ready = %q, error = %v", ready, err)
	}
	return func() {
		if err := stdin.Close(); err != nil {
			t.Error(err)
		}
		if err := command.Wait(); err != nil {
			t.Error(err)
		}
	}
}

func TestNotesCLILockHolderProcess(t *testing.T) {
	if os.Getenv("GO_WANT_NOTESCLI_LOCK_HELPER") != "1" {
		return
	}
	file, err := os.OpenFile(os.Args[len(os.Args)-1], os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "ready")
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func mapsKeys(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}
