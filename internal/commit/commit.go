package commit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

// CommitMessageProvider generates commit messages from diffs.
type CommitMessageProvider interface {
	Generate(ctx context.Context, diff string, files []model.FileSummary) (string, error)
}

var (
	ErrMissingAPIKey     = errors.New("missing API key")
	ErrProviderRequest   = errors.New("provider request failed")
	ErrMalformedResponse = errors.New("malformed provider response")
	ErrUnstagedChanges   = errors.New("unstaged changes present alongside staged changes")
)

// BuildPrompt constructs a deterministic prompt from a staged-only diff and file summaries.
func BuildPrompt(diff string, files []model.FileSummary) string {
	var b strings.Builder

	b.WriteString("Generate a conventional commit message for the following staged changes.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Subject line: type(optional scope): description, max 72 characters.\n")
	b.WriteString("- Types: feat, fix, refactor, docs, test, chore, style, perf, ci, build.\n")
	b.WriteString("- Optional body separated by a blank line; wrap at 72 characters.\n")
	b.WriteString("- Do not include any explanation outside the commit message itself.\n\n")

	if len(files) > 0 {
		b.WriteString("File summary:\n")
		for _, f := range files {
			b.WriteString(fmt.Sprintf("  %s %s (+%d -%d)\n", f.Status, f.Path, f.Additions, f.Deletions))
		}
		b.WriteString("\n")
	}

	b.WriteString("Diff:\n")
	b.WriteString(diff)

	return b.String()
}

// CollectStagedSnapshot returns the staged-only diff text and parsed file summaries.
func CollectStagedSnapshot(ctx context.Context, git gitexec.GitRunner) (string, []model.FileSummary, error) {
	diff, err := git.RunGit(ctx, "diff", "--cached", "--no-color", "--no-ext-diff", "-M")
	if err != nil {
		return "", nil, fmt.Errorf("staged diff: %w", err)
	}

	nsOut, err := git.RunGit(ctx, "diff", "--cached", "--name-status", "-z", "-M")
	if err != nil {
		return "", nil, fmt.Errorf("staged name-status: %w", err)
	}

	files, err := parseStagedNameStatus(nsOut)
	if err != nil {
		return "", nil, fmt.Errorf("parse staged name-status: %w", err)
	}
	if len(files) == 0 {
		return diff, nil, nil
	}

	numOut, err := git.RunGit(ctx, "diff", "--cached", "--numstat", "-z", "-M")
	if err != nil {
		return "", nil, fmt.Errorf("staged numstat: %w", err)
	}

	stats := parseStagedNumstat(numOut)
	for i := range files {
		if s, ok := stats[files[i].Path]; ok {
			files[i].Additions = s.additions
			files[i].Deletions = s.deletions
			files[i].IsBinary = s.isBinary
		}
	}

	return diff, files, nil
}

// CheckStagingGuard returns ErrUnstagedChanges when unstaged or untracked
// changes are present alongside staged changes.
func CheckStagingGuard(ctx context.Context, git gitexec.GitRunner) error {
	hasStaged, err := hasChanges(ctx, git, true)
	if err != nil {
		return fmt.Errorf("check staged: %w", err)
	}
	if !hasStaged {
		return nil
	}

	hasUnstaged, err := hasChanges(ctx, git, false)
	if err != nil {
		return fmt.Errorf("check unstaged: %w", err)
	}
	if hasUnstaged {
		return ErrUnstagedChanges
	}

	untracked, err := hasUntrackedFiles(ctx, git)
	if err != nil {
		return fmt.Errorf("check untracked: %w", err)
	}
	if untracked {
		return ErrUnstagedChanges
	}

	return nil
}

// hasChanges returns true when git diff --quiet exits with code 1 (changes present).
// Other git errors are propagated.
func hasChanges(ctx context.Context, git gitexec.GitRunner, cached bool) (bool, error) {
	args := []string{"diff", "--quiet"}
	if cached {
		args = []string{"diff", "--cached", "--quiet"}
	}

	_, err := git.RunGit(ctx, args...)
	if err == nil {
		return false, nil
	}

	var gitErr *gitexec.GitError
	if errors.As(err, &gitErr) && gitErr.ExitCode == 1 {
		return true, nil
	}
	return false, err
}

// hasUntrackedFiles returns true when untracked files exist in the working tree.
func hasUntrackedFiles(ctx context.Context, git gitexec.GitRunner) (bool, error) {
	out, err := git.RunGit(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// --- staged diff parsing ---

func parseStagedNameStatus(raw string) ([]model.FileSummary, error) {
	if raw == "" {
		return nil, nil
	}

	fields := strings.Split(raw, "\x00")
	if len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}

	var files []model.FileSummary
	for i := 0; i < len(fields); {
		statusField := fields[i]
		if statusField == "" {
			i++
			continue
		}

		letter := model.FileStatus(statusField[:1])
		isRename := letter == model.StatusRenamed || letter == model.StatusCopied
		i++

		if i >= len(fields) {
			return nil, fmt.Errorf("unexpected end after status %q", statusField)
		}

		var fs model.FileSummary
		fs.Status = letter

		if isRename {
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("unexpected end in rename for status %q", statusField)
			}
			fs.OldPath = fields[i]
			fs.Path = fields[i+1]
			i += 2
		} else {
			fs.Path = fields[i]
			i++
		}

		files = append(files, fs)
	}

	return files, nil
}

type numstatEntry struct {
	additions int
	deletions int
	isBinary  bool
}

func parseStagedNumstat(raw string) map[string]numstatEntry {
	if raw == "" {
		return nil
	}

	fields := strings.Split(raw, "\x00")
	if len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}

	stats := make(map[string]numstatEntry)
	for i := 0; i < len(fields); {
		line := fields[i]
		i++

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		entry := numstatEntry{}
		if parts[0] == "-" && parts[1] == "-" {
			entry.isBinary = true
		} else {
			entry.additions, _ = strconv.Atoi(parts[0])
			entry.deletions, _ = strconv.Atoi(parts[1])
		}

		pathPart := parts[2]
		if pathPart == "" {
			// Rename/copy: next two fields are old\0new.
			if i+1 >= len(fields) {
				continue
			}
			stats[fields[i+1]] = entry
			i += 2
			continue
		}
		stats[pathPart] = entry
	}

	return stats
}
