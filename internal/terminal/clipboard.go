package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// clipboardCmd returns the clipboard command name for the given OS and
// a lookup function that resolves tool availability (e.g. exec.LookPath).
// Returns "" if no supported clipboard tool is found.
func clipboardCmd(goos string, lookPath func(string) string) string {
	if goos == "darwin" {
		return "pbcopy"
	}
	// Linux / WSL: try tools in preference order.
	for _, tool := range []string{"xclip", "xsel", "wl-copy", "clip.exe"} {
		if lookPath(tool) != "" {
			return tool
		}
	}
	return ""
}

// clipboardArgs returns the full command + args for piping stdin to the clipboard.
func clipboardArgs(tool string) []string {
	switch tool {
	case "xclip":
		return []string{"xclip", "-selection", "clipboard"}
	case "xsel":
		return []string{"xsel", "--clipboard", "--input"}
	default:
		return []string{tool}
	}
}

// FormatPaths formats file paths as newline-delimited text.
func FormatPaths(paths []string) string {
	return strings.Join(paths, "\n")
}

// CopyToClipboard writes text to the system clipboard.
// Returns nil on success, or an error describing the failure.
func CopyToClipboard(text string) error {
	lookPath := func(name string) string {
		p, err := exec.LookPath(name)
		if err != nil {
			return ""
		}
		return p
	}
	tool := clipboardCmd(runtime.GOOS, lookPath)
	if tool == "" {
		return fmt.Errorf("no clipboard tool found (install xclip, xsel, or wl-copy)")
	}

	args := clipboardArgs(tool)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
