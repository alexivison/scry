package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/scry"
	build := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func TestRunInvalidFlag(t *testing.T) {
	t.Parallel()

	code := runWith([]string{"--nonexistent"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunDefaultsNonTTY(t *testing.T) {
	t.Parallel()

	// In a test process, stdin/stdout are not TTYs, so app.Run returns 128.
	code := runWith([]string{})
	if code != 128 {
		t.Errorf("exit code = %d, want 128 (non-TTY)", code)
	}
}

func TestRunHelpExitsZero(t *testing.T) {
	t.Parallel()

	code := runWith([]string{"--help"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunUnexpectedArg(t *testing.T) {
	t.Parallel()

	code := runWith([]string{"unexpected-arg"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunCommitAutoWithoutCommit(t *testing.T) {
	t.Parallel()

	code := runWith([]string{"--commit-auto"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestHelpDocumentsFinalCLISurface(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	cmd := exec.Command(bin, "--help")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Run()

	help := buf.String()

	finalFlags := []string{
		"--base",
		"--head",
		"--commit",
		"--commit-auto",
		"--no-dashboard",
	}
	for _, f := range finalFlags {
		if !strings.Contains(help, f) {
			t.Errorf("--help output missing flag %q", f)
		}
	}
}

func TestHelpDoesNotDocumentDeprecatedFlags(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	cmd := exec.Command(bin, "--help")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Run()

	help := buf.String()

	deprecated := []string{
		"--mode",
		"--ignore-whitespace",
		"--watch",
		"--no-watch",
		"--watch-interval",
		"--commit-provider",
		"--commit-model",
		"--worktrees",
	}
	for _, f := range deprecated {
		if strings.Contains(help, f) {
			t.Errorf("--help should NOT contain deprecated flag %q", f)
		}
	}
}

func TestExitCode2BadFlags(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	cmd := exec.Command(bin, "--nonexistent-flag")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit, got nil")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
}

func TestVersionFlagBinary(t *testing.T) {
	t.Parallel()

	bin := t.TempDir() + "/scry"
	build := exec.Command("go", "build", "-buildvcs=false", "-ldflags", "-X main.version=9.9.9 -X main.commit=deadbee", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "--version")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("run --version: %v\n%s", err, buf.String())
	}

	if got := buf.String(); got != "scry 9.9.9 (deadbee)\n" {
		t.Fatalf("output = %q, want %q", got, "scry 9.9.9 (deadbee)\n")
	}
}

func TestNoteCommandsPersistAndDefaultWorktreesAreIsolated(t *testing.T) {
	bin := buildBinary(t)
	config := t.TempDir()
	first := t.TempDir()
	second := t.TempDir()
	for _, worktree := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(worktree, "source.go"), []byte("line\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	add := noteCommand(t, bin, first, config, "add", "--file", "source.go", "--line", "1", "--body", "persisted", "--author", "user")
	if err := add.cmd.Run(); err != nil {
		t.Fatalf("note add: %v\n%s", err, add.stderr)
	}
	if !json.Valid(add.stdout.Bytes()) {
		t.Fatalf("note add stdout is not JSON: %q", add.stdout.String())
	}

	list := noteCommand(t, bin, first, config, "list")
	if err := list.cmd.Run(); err != nil {
		t.Fatalf("note list: %v\n%s", err, list.stderr)
	}
	var persisted struct {
		Notes []json.RawMessage `json:"notes"`
	}
	if err := json.Unmarshal(list.stdout.Bytes(), &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted.Notes) != 1 {
		t.Fatalf("persisted notes = %s, want one", list.stdout.String())
	}

	isolate := noteCommand(t, bin, second, config, "list")
	if err := isolate.cmd.Run(); err != nil {
		t.Fatalf("isolated note list: %v\n%s", err, isolate.stderr)
	}
	var isolated struct {
		Notes []json.RawMessage `json:"notes"`
	}
	if err := json.Unmarshal(isolate.stdout.Bytes(), &isolated); err != nil {
		t.Fatal(err)
	}
	if len(isolated.Notes) != 0 {
		t.Fatalf("isolated notes = %s, want empty", isolate.stdout.String())
	}
}

type commandBuffers struct {
	cmd            *exec.Cmd
	stdout, stderr *bytes.Buffer
}

func noteCommand(t *testing.T, bin, worktree, config string, args ...string) commandBuffers {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"note"}, args...)...)
	cmd.Dir = worktree
	cmd.Env = append(os.Environ(), "HOME="+filepath.Join(config, "home"), "XDG_CONFIG_HOME="+config)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return commandBuffers{cmd, stdout, stderr}
}
