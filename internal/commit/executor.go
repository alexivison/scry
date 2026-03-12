package commit

import (
	"context"
	"strings"

	"github.com/alexivison/scry/internal/gitexec"
)

// Executor runs git commit with a given message.
type Executor struct {
	Git gitexec.GitRunner
}

// Execute runs git commit -m and returns the resulting short SHA.
func (e *Executor) Execute(ctx context.Context, message string) (string, error) {
	if _, err := e.Git.RunGit(ctx, "commit", "-m", message); err != nil {
		return "", err
	}

	sha, err := e.Git.RunGit(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(sha), nil
}
