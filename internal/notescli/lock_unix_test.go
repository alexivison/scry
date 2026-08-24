//go:build darwin || linux

package notescli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRunBusyErrorIsOnlyJSON(t *testing.T) {
	options, worktree := testOptions(t)
	if err := os.WriteFile(filepath.Join(worktree, "source.go"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stop := holdLedgerLock(t, testLedgerPath(options.ConfigDir, worktree)+".lock")
	t.Cleanup(stop)

	result := run(t, []string{"add", "--file", "source.go", "--line", "1", "--body", "blocked", "--author", "user"}, options)
	assertJSONError(t, result, 1, "busy")
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
