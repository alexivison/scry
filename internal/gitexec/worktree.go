package gitexec

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// WorktreeEntry represents a single worktree from `git worktree list --porcelain`.
type WorktreeEntry struct {
	Path     string // absolute path to worktree root
	HEAD     string // commit SHA
	Branch   string // e.g. "refs/heads/main"; empty for detached HEAD
	Bare     bool
	Prunable bool
}

// WorktreeList parses `git worktree list --porcelain` and returns all entries.
func WorktreeList(ctx context.Context, r GitRunner) ([]WorktreeEntry, error) {
	out, err := r.RunGit(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("worktree list: %w", err)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}

	// Porcelain output uses blank lines to separate entries.
	blocks := strings.Split(out, "\n\n")
	entries := make([]WorktreeEntry, 0, len(blocks))

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		var e WorktreeEntry
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				e.Path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "HEAD "):
				e.HEAD = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				e.Branch = strings.TrimPrefix(line, "branch ")
			case line == "bare":
				e.Bare = true
			case strings.HasPrefix(line, "prunable"):
				e.Prunable = true
			}
		}

		if e.Path != "" {
			entries = append(entries, e)
		}
	}

	return entries, nil
}

// WorktreeRemove removes a worktree using `git worktree remove`.
// When force is true, uses --force to remove dirty worktrees.
func WorktreeRemove(ctx context.Context, r GitRunner, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := r.RunGit(ctx, args...)
	if err != nil {
		return fmt.Errorf("worktree remove: %w", err)
	}
	return nil
}

// CommitMetaResult holds parsed commit metadata from a single git log call.
type CommitMetaResult struct {
	Hash       string
	Subject    string
	CommitDate time.Time
}

// CommitMeta returns the short hash, committer date, and subject of HEAD in a worktree.
// Uses committer date (%cI) rather than author date (%aI) so rebased/cherry-picked
// branches reflect when they were last updated, not when the code was originally authored.
func CommitMeta(ctx context.Context, r GitRunner, worktreePath string) (CommitMetaResult, error) {
	out, err := r.RunGit(ctx, "-C", worktreePath, "log", "-1", "--format=%h%x00%cI%x00%s")
	if err != nil {
		return CommitMetaResult{}, fmt.Errorf("commit meta for %s: %w", worktreePath, err)
	}
	line := strings.TrimSpace(out)
	parts := strings.SplitN(line, "\x00", 3)
	if len(parts) < 3 {
		return CommitMetaResult{}, fmt.Errorf("commit meta for %s: unexpected format %q", worktreePath, line)
	}
	commitDate, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return CommitMetaResult{}, fmt.Errorf("commit meta for %s: parse date %q: %w", worktreePath, parts[1], err)
	}
	return CommitMetaResult{
		Hash:       parts[0],
		Subject:    parts[2],
		CommitDate: commitDate,
	}, nil
}

// StatusCount returns the number of changed files in a worktree via `git status --porcelain`.
// Note: untracked directories are reported as a single entry unless status.showUntrackedFiles=all.
func StatusCount(ctx context.Context, r GitRunner, worktreePath string) (int, error) {
	out, err := r.RunGit(ctx, "-C", worktreePath, "status", "--porcelain")
	if err != nil {
		return 0, fmt.Errorf("status count for %s: %w", worktreePath, err)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, nil
	}
	return len(strings.Split(out, "\n")), nil
}

// ShortBranch strips the "refs/heads/" prefix from a branch ref.
func ShortBranch(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}
