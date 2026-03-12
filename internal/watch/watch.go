// Package watch provides fingerprint calculation and tick scheduling
// for watch mode. It detects repository state changes by computing
// fingerprints and comparing them across ticks.
package watch

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

// Fingerprinter computes a change fingerprint for the current repo state.
// The fingerprint is compare-mode aware:
//   - Committed-ref mode: concatenates rev-parse of HEAD and base ref.
//   - Working-tree mode: additionally incorporates git diff --name-only HEAD
//     to detect staged/unstaged edits.
type Fingerprinter struct {
	Runner gitexec.GitRunner
}

// Fingerprint returns a string that changes whenever the repository state
// relevant to the current compare changes. baseRef must be a symbolic ref
// (e.g., "origin/main") so that rev-parse resolves it live on each tick.
// Passing an already-resolved SHA would make the base half of the
// fingerprint static.
func (f *Fingerprinter) Fingerprint(ctx context.Context, baseRef string, workingTree bool) (string, error) {
	// rev-parse HEAD and the base ref in a single call.
	out, err := f.Runner.RunGit(ctx, "rev-parse", "HEAD", baseRef)
	if err != nil {
		return "", fmt.Errorf("fingerprint rev-parse: %w", err)
	}

	shas := strings.Split(strings.TrimSpace(out), "\n")
	if len(shas) != 2 {
		return "", fmt.Errorf("fingerprint: expected 2 SHAs, got %d", len(shas))
	}
	refSHAs := shas[0] + ":" + shas[1] // HEAD-sha:base-sha

	if !workingTree {
		return refSHAs, nil
	}

	// Working-tree mode: diff against HEAD to capture both staged
	// and unstaged edits.
	diffOut, err := f.Runner.RunGit(ctx, "diff", "--name-only", "HEAD")
	if err != nil {
		return "", fmt.Errorf("fingerprint diff: %w", err)
	}

	return refSHAs + ":" + strings.TrimRight(diffOut, "\n"), nil
}

// ShouldRefresh returns true when the fingerprint has changed and no
// refresh is already in flight. This implements the debounce/in-flight
// rule: skip refresh while one is running, reevaluate on next tick.
func ShouldRefresh(state *model.AppState, newFingerprint string) bool {
	if state.RefreshInFlight {
		return false
	}
	return newFingerprint != state.LastFingerprint
}

// --- Bubble Tea messages -------------------------------------------------

// TickMsg is sent on each watch interval to trigger a fingerprint check.
type TickMsg struct {
	At time.Time
}

// FingerprintMsg carries the result of a fingerprint check.
type FingerprintMsg struct {
	Fingerprint string
	Err         error
}

// --- Bubble Tea commands -------------------------------------------------

// TickCmd returns a tea.Cmd that sends a TickMsg after the given interval.
func TickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return TickMsg{At: t}
	})
}

// CheckCmd returns a tea.Cmd that computes the fingerprint asynchronously.
// The provided context allows cancellation on app shutdown.
func CheckCmd(ctx context.Context, f *Fingerprinter, baseRef string, workingTree bool) tea.Cmd {
	return func() tea.Msg {
		fp, err := f.Fingerprint(ctx, baseRef, workingTree)
		return FingerprintMsg{Fingerprint: fp, Err: err}
	}
}
