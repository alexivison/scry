package gitexec

import (
	"context"
	"fmt"
	"strings"
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

// StatusClean returns true if the worktree at the given path has no uncommitted changes.
func StatusClean(ctx context.Context, r GitRunner, worktreePath string) (bool, error) {
	out, err := r.RunGit(ctx, "-C", worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("status check for %s: %w", worktreePath, err)
	}
	return strings.TrimSpace(out) == "", nil
}

// CommitSubject returns the short hash and subject of a commit.
func CommitSubject(ctx context.Context, r GitRunner, worktreePath string) (hash, subject string, err error) {
	out, err := r.RunGit(ctx, "-C", worktreePath, "log", "-1", "--format=%h %s")
	if err != nil {
		return "", "", fmt.Errorf("commit subject for %s: %w", worktreePath, err)
	}
	line := strings.TrimSpace(out)
	if idx := strings.IndexByte(line, ' '); idx >= 0 {
		return line[:idx], line[idx+1:], nil
	}
	return line, "", nil
}

// ShortBranch strips the "refs/heads/" prefix from a branch ref.
func ShortBranch(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}
