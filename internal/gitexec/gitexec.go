// Package gitexec is the sole subprocess boundary for git commands.
// No other package may execute git directly.
package gitexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout is applied when GitRunnerConfig.Timeout is zero.
const DefaultTimeout = 30 * time.Second

// RemoveTimeout is used for destructive operations like worktree removal
// which can be significantly slower than read-only git commands.
const RemoveTimeout = 2 * time.Minute

// GitRunner executes git commands in a fixed working directory.
type GitRunner interface {
	RunGit(ctx context.Context, args ...string) (string, error)
}

// GitRunnerConfig configures a GitRunner instance.
type GitRunnerConfig struct {
	WorkDir string
	Timeout time.Duration // Per-command timeout; zero uses DefaultTimeout.
}

// GitError wraps a non-zero git exit with structured context.
// Stdout is populated so callers can recover output from commands
// that exit non-zero by design (e.g. git diff --no-index).
type GitError struct {
	Args     []string
	ExitCode int
	Stderr   string
	Stdout   string
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git %s: exit %d: %s",
		strings.Join(e.Args, " "), e.ExitCode, e.Stderr)
}

type runner struct {
	workDir string
	timeout time.Duration
}

// NewGitRunner returns a GitRunner that executes in cfg.WorkDir.
func NewGitRunner(cfg GitRunnerConfig) GitRunner {
	t := cfg.Timeout
	if t == 0 {
		t = DefaultTimeout
	}
	return &runner{workDir: cfg.WorkDir, timeout: t}
}

func (r *runner) RunGit(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Surface context errors directly so callers can match.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", &GitError{
				Args:     args,
				ExitCode: exitErr.ExitCode(),
				Stderr:   strings.TrimSpace(stderr.String()),
				Stdout:   stdout.String(),
			}
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return stdout.String(), nil
}
