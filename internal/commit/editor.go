package commit

import (
	"os"
	"os/exec"
	"strings"
)

// PrepareEditorCmd writes message to a temp file and returns an exec.Cmd
// that opens $EDITOR (or "vi" as fallback) on that file.
// The caller must call cleanup() after reading the edited result.
func PrepareEditorCmd(message string) (cmd *exec.Cmd, tmpPath string, err error) {
	f, err := os.CreateTemp("", "scry-commit-*.txt")
	if err != nil {
		return nil, "", err
	}

	if _, err := f.WriteString(message); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, "", err
	}
	f.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Split on whitespace to support editors with flags (e.g. "code --wait").
	parts := strings.Fields(editor)
	args := append(parts[1:], f.Name())
	return exec.Command(parts[0], args...), f.Name(), nil
}

// ReadEditedMessage reads a file path and returns its trimmed content.
func ReadEditedMessage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
