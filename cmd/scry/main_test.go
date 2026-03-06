package main

import "testing"

func TestRunInvalidFlag(t *testing.T) {
	t.Parallel()

	code := runWith([]string{"--nonexistent"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestRunInvalidMode(t *testing.T) {
	t.Parallel()

	code := runWith([]string{"--mode", "invalid"})
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
