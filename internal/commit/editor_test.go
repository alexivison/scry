package commit

import (
	"os"
	"testing"
)

func TestPrepareEditorCmd_usesEDITOR(t *testing.T) {
	t.Setenv("EDITOR", "nano")

	cmd, tmpPath, err := PrepareEditorCmd("test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpPath)

	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if cmd.Path == "" && len(cmd.Args) == 0 {
		t.Fatal("cmd has no path or args")
	}
	// The first arg should be the editor command name.
	if cmd.Args[0] != "nano" {
		t.Errorf("cmd.Args[0] = %q, want %q", cmd.Args[0], "nano")
	}
	// The second arg should be the temp file path.
	if len(cmd.Args) < 2 || cmd.Args[1] != tmpPath {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], tmpPath)
	}
}

func TestPrepareEditorCmd_fallbackToVi(t *testing.T) {
	t.Setenv("EDITOR", "")

	cmd, tmpPath, err := PrepareEditorCmd("test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpPath)

	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if cmd.Args[0] != "vi" {
		t.Errorf("cmd.Args[0] = %q, want %q (fallback)", cmd.Args[0], "vi")
	}
}

func TestPrepareEditorCmd_editorWithFlags(t *testing.T) {
	t.Setenv("EDITOR", "code --wait")

	cmd, tmpPath, err := PrepareEditorCmd("test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpPath)

	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if cmd.Args[0] != "code" {
		t.Errorf("cmd.Args[0] = %q, want %q", cmd.Args[0], "code")
	}
	if len(cmd.Args) < 3 {
		t.Fatalf("cmd.Args = %v, want at least 3 elements", cmd.Args)
	}
	if cmd.Args[1] != "--wait" {
		t.Errorf("cmd.Args[1] = %q, want %q", cmd.Args[1], "--wait")
	}
	if cmd.Args[2] != tmpPath {
		t.Errorf("cmd.Args[2] = %q, want %q", cmd.Args[2], tmpPath)
	}
}

func TestPrepareEditorCmd_writesMessageToTempFile(t *testing.T) {
	t.Setenv("EDITOR", "cat")

	msg := "feat: add new feature\n\nThis is the body."
	_, tmpPath, err := PrepareEditorCmd(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpPath)

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(content) != msg {
		t.Errorf("temp file content = %q, want %q", string(content), msg)
	}
}

func TestReadEditedMessage_roundTrip(t *testing.T) {
	f, err := os.CreateTemp("", "scry-test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	msg := "fix: repair something\n\nDetailed explanation."
	if _, err := f.WriteString("  " + msg + "  \n"); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	f.Close()

	result, err := ReadEditedMessage(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != msg {
		t.Errorf("result = %q, want %q (trimmed)", result, msg)
	}
}

func TestReadEditedMessage_fileNotFound(t *testing.T) {
	_, err := ReadEditedMessage("/tmp/nonexistent-scry-commit-test.txt")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
