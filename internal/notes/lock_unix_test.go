//go:build darwin || linux

package notes

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAcquireLockCreatesOwnerOnlyPersistentFile(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t)
	if err := os.MkdirAll(filepath.Dir(store.lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireLock(store.lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("lock permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestAddReturnsBusyWhileAnotherProcessHoldsLedgerLock(t *testing.T) {
	store, _ := newTestStore(t)
	mustAdd(t, store, AddInput{File: "source.go", Line: 1, Body: "saved", Author: AuthorUser})
	before, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestLockHolderProcess$", "--", store.lockPath)
	command.Env = append(os.Environ(), "GO_WANT_NOTE_LOCK_HELPER=1")
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
	reader := bufio.NewReader(stdout)
	ready, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if ready != "ready\n" {
		t.Fatalf("lock helper = %q", ready)
	}

	_, err = store.Add(AddInput{File: "source.go", Line: 2, Body: "blocked", Author: AuthorUser})
	if errCode(err) != "busy" {
		t.Fatalf("Add error = %v, want busy", err)
	}
	after, err := os.ReadFile(store.ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("busy Add changed ledger")
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestLockHolderProcess(t *testing.T) {
	if os.Getenv("GO_WANT_NOTE_LOCK_HELPER") != "1" {
		return
	}
	path := os.Args[len(os.Args)-1]
	lock, err := acquireLock(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer lock.Close()
	fmt.Fprintln(os.Stdout, "ready")
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func errCode(err error) string {
	if err, ok := err.(*Error); ok {
		return err.Code
	}
	return ""
}
