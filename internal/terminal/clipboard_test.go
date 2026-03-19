package terminal

import (
	"testing"
)

func TestClipboardCommandDarwin(t *testing.T) {
	t.Parallel()
	cmd := clipboardCmd("darwin", func(string) string { return "" })
	if cmd == "" {
		t.Fatal("darwin should return a clipboard command")
	}
	if cmd != "pbcopy" {
		t.Errorf("darwin clipboard cmd = %q, want %q", cmd, "pbcopy")
	}
}

func TestClipboardCommandLinuxXclip(t *testing.T) {
	t.Parallel()
	lookup := func(name string) string {
		if name == "xclip" {
			return "/usr/bin/xclip"
		}
		return ""
	}
	cmd := clipboardCmd("linux", lookup)
	if cmd != "xclip" {
		t.Errorf("linux with xclip: cmd = %q, want %q", cmd, "xclip")
	}
}

func TestClipboardCommandLinuxXsel(t *testing.T) {
	t.Parallel()
	lookup := func(name string) string {
		if name == "xsel" {
			return "/usr/bin/xsel"
		}
		return ""
	}
	cmd := clipboardCmd("linux", lookup)
	if cmd != "xsel" {
		t.Errorf("linux with xsel: cmd = %q, want %q", cmd, "xsel")
	}
}

func TestClipboardCommandLinuxWlCopy(t *testing.T) {
	t.Parallel()
	lookup := func(name string) string {
		if name == "wl-copy" {
			return "/usr/bin/wl-copy"
		}
		return ""
	}
	cmd := clipboardCmd("linux", lookup)
	if cmd != "wl-copy" {
		t.Errorf("linux with wl-copy: cmd = %q, want %q", cmd, "wl-copy")
	}
}

func TestClipboardCommandWSL(t *testing.T) {
	t.Parallel()
	// WSL is detected as linux with clip.exe available.
	lookup := func(name string) string {
		if name == "clip.exe" {
			return "/mnt/c/Windows/System32/clip.exe"
		}
		return ""
	}
	cmd := clipboardCmd("linux", lookup)
	if cmd != "clip.exe" {
		t.Errorf("WSL clipboard cmd = %q, want %q", cmd, "clip.exe")
	}
}

func TestClipboardCommandUnsupported(t *testing.T) {
	t.Parallel()
	cmd := clipboardCmd("linux", func(string) string { return "" })
	if cmd != "" {
		t.Errorf("unsupported env should return empty, got %q", cmd)
	}
}

func TestCopyToClipboardFormat(t *testing.T) {
	t.Parallel()
	paths := []string{"main.go", "pkg/util.go", "README.md"}
	text := FormatPaths(paths)
	expected := "main.go\npkg/util.go\nREADME.md"
	if text != expected {
		t.Errorf("FormatPaths = %q, want %q", text, expected)
	}
}

func TestCopyToClipboardEmptyPaths(t *testing.T) {
	t.Parallel()
	text := FormatPaths(nil)
	if text != "" {
		t.Errorf("FormatPaths(nil) = %q, want empty", text)
	}
}
