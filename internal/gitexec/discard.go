package gitexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// DiscardFile restores a file to its committed state.
// For tracked files (untracked=false), runs `git checkout HEAD -- path`.
// For untracked files (untracked=true), removes the file from disk under workdir.
// The path is interpreted relative to workdir for the untracked case and is passed
// through to git verbatim for the tracked case (git resolves it relative to the runner's working directory).
func DiscardFile(ctx context.Context, r GitRunner, workdir, path string, untracked bool) error {
	if untracked {
		if err := os.Remove(filepath.Join(workdir, path)); err != nil {
			return fmt.Errorf("discard untracked %s: %w", path, err)
		}
		return nil
	}
	if _, err := r.RunGit(ctx, "checkout", "HEAD", "--", path); err != nil {
		return fmt.Errorf("discard tracked %s: %w", path, err)
	}
	return nil
}
