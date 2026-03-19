package main

import (
	"bytes"
	"os/exec"
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
		"--no-watch",
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
