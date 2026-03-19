// Package diff implements diff metadata parsing and merge logic.
package diff

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

// MetadataService lists files changed between two commits.
type MetadataService struct {
	Runner gitexec.GitRunner
}

// ListFiles returns file summaries in the order emitted by git diff --name-status.
// It runs two git commands (--name-status -z and --numstat -z), parses the
// NUL-delimited output, and merges counts into the authoritative ordering.
func (s *MetadataService) ListFiles(ctx context.Context, cmp model.ResolvedCompare) ([]model.FileSummary, error) {
	nsOut, err := s.Runner.RunGit(ctx, "diff", "--name-status", "-z", "-M", cmp.DiffRange)
	if err != nil {
		return nil, fmt.Errorf("name-status: %w", err)
	}

	numOut, err := s.Runner.RunGit(ctx, "diff", "--numstat", "-z", "-M", cmp.DiffRange)
	if err != nil {
		return nil, fmt.Errorf("numstat: %w", err)
	}

	files, err := parseNameStatus(nsOut)
	if err != nil {
		return nil, fmt.Errorf("parse name-status: %w", err)
	}
	if len(files) == 0 {
		return nil, nil
	}

	stats := parseNumstat(numOut)
	mergeStats(files, stats)

	if cmp.WorkingTree {
		untracked, err := s.listUntracked(ctx, cmp.Repo.WorktreeRoot)
		if err != nil {
			return nil, fmt.Errorf("untracked: %w", err)
		}
		files = append(files, untracked...)
	}

	return files, nil
}

// listUntracked returns FileSummary entries for untracked files.
func (s *MetadataService) listUntracked(ctx context.Context, root string) ([]model.FileSummary, error) {
	out, err := s.Runner.RunGit(ctx, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	paths := strings.Split(out, "\x00")
	if len(paths) > 0 && paths[len(paths)-1] == "" {
		paths = paths[:len(paths)-1]
	}

	files := make([]model.FileSummary, 0, len(paths))
	for _, p := range paths {
		fs := model.FileSummary{
			Path:   p,
			Status: model.StatusUntracked,
		}
		fs.Additions, fs.IsBinary = countFileLines(filepath.Join(root, p))
		files = append(files, fs)
	}
	return files, nil
}

// countFileLines returns the line count and binary flag for a file.
// Binary detection uses a simple NUL-byte heuristic on the first 8 KiB.
func countFileLines(path string) (lines int, binary bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	// Check first 8 KiB for NUL bytes (binary heuristic).
	probe := data
	if len(probe) > 8192 {
		probe = probe[:8192]
	}
	if bytes.ContainsRune(probe, 0) {
		return 0, true
	}
	if len(data) == 0 {
		return 0, false
	}
	n := bytes.Count(data, []byte{'\n'})
	// Count final line even without trailing newline.
	if data[len(data)-1] != '\n' {
		n++
	}
	return n, false
}

// parseNameStatus parses NUL-delimited --name-status -z output.
// Format: status\0path\0  or  status-with-score\0old\0new\0
func parseNameStatus(raw string) ([]model.FileSummary, error) {
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

// parseNumstat parses NUL-delimited --numstat -z output into a map keyed by path.
// Format: add\tdel\tpath\0  or  add\tdel\t\0old\0new\0  (rename/copy)
func parseNumstat(raw string) map[string]numstatEntry {
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
			if i+1 < len(fields) {
				// Rename/copy: empty path field, next two fields are old\0new.
				stats[fields[i]+"\x00"+fields[i+1]] = entry
				i += 2
			}
			// Orphaned empty path with insufficient fields — skip silently.
			continue
		}
		stats[pathPart] = entry
	}

	return stats
}

// mergeStats enriches file summaries with numstat data.
// Missing entries default to 0/0 (zero-value FileSummary fields).
func mergeStats(files []model.FileSummary, stats map[string]numstatEntry) {
	for i := range files {
		key := files[i].Path
		if files[i].OldPath != "" {
			key = files[i].OldPath + "\x00" + files[i].Path
		}

		entry, ok := stats[key]
		if !ok {
			continue
		}
		files[i].Additions = entry.additions
		files[i].Deletions = entry.deletions
		files[i].IsBinary = entry.isBinary
	}
}
