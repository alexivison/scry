package diff

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	godiff "github.com/sourcegraph/go-diff/diff"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

const (
	maxPatchBytes = 8 << 20 // 8 MiB
	maxPatchLines = 50_000
	tabWidth      = 8 // terminal tab stop width; see expandTabs.
)

// OversizedError wraps ErrOversized with the measured byte size and line count
// so the UI can display them in the fallback message.
type OversizedError struct {
	Bytes int
	Lines int
}

func (e *OversizedError) Error() string {
	return fmt.Sprintf("patch exceeds size threshold (%d lines, %d bytes)", e.Lines, e.Bytes)
}

func (e *OversizedError) Unwrap() error { return model.ErrOversized }

// PatchService loads and parses unified diffs.
type PatchService struct {
	Runner gitexec.GitRunner
}

// LoadPatch runs git diff --patch for a single file and parses the result.
// For untracked files (status == StatusUntracked), it uses --no-index against /dev/null.
func (s *PatchService) LoadPatch(ctx context.Context, cmp model.ResolvedCompare, filePath string, status model.FileStatus, ignoreWhitespace bool) (model.FilePatch, error) {
	raw, err := s.runDiff(ctx, cmp, filePath, status, ignoreWhitespace)
	if err != nil {
		return model.FilePatch{}, fmt.Errorf("git diff --patch: %w", err)
	}

	if raw == "" {
		return model.FilePatch{}, nil
	}

	summary := model.FileSummary{Path: filePath}

	// Byte-size gate before parsing.
	if len(raw) > maxPatchBytes {
		return model.FilePatch{Summary: summary}, &OversizedError{Bytes: len(raw), Lines: strings.Count(raw, "\n")}
	}

	// Binary file detection: the marker "Binary files ... differ" always
	// appears at the start of a line outside any hunk. Anchoring to line-start
	// avoids false-positives from hunk content containing those words.
	if strings.Contains("\n"+raw, "\nBinary files ") && strings.Contains(raw, " differ") {
		summary.IsBinary = true
		return model.FilePatch{Summary: summary}, model.ErrBinaryFile
	}

	// Submodule detection: gitlink mode 160000 in known diff header lines.
	if hasGitlinkMode(raw) {
		summary.IsSubmodule = true
		return model.FilePatch{Summary: summary}, model.ErrSubmodule
	}

	fd, err := godiff.ParseFileDiff([]byte(raw))
	if err != nil {
		return model.FilePatch{}, fmt.Errorf("parse patch for %s: %w", filePath, err)
	}

	// go-diff strips "\ No newline at end of file" markers from Hunk.Body.
	// Extract raw hunk bodies from the original text to preserve them.
	rawBodies := splitRawBodies(raw)

	hunks, totalLines := mapHunks(fd.Hunks, rawBodies)

	if totalLines > maxPatchLines {
		return model.FilePatch{Summary: summary}, &OversizedError{Bytes: len(raw), Lines: totalLines}
	}

	return model.FilePatch{Summary: summary, Hunks: hunks}, nil
}

// LoadFolderPatch loads the changed files under folder in one git diff command.
func (s *PatchService) LoadFolderPatch(ctx context.Context, cmp model.ResolvedCompare, folder string, files []model.FileSummary, ignoreWhitespace bool) (model.FilePatch, error) {
	summary := model.FileSummary{Path: folder}
	for _, file := range files {
		summary.Additions += file.Additions
		summary.Deletions += file.Deletions
	}

	raw, err := s.runDiff(ctx, cmp, folder, "", ignoreWhitespace)
	if err != nil {
		return model.FilePatch{}, fmt.Errorf("git diff --patch: %w", err)
	}
	if len(raw) > maxPatchBytes {
		return model.FilePatch{Summary: summary}, &OversizedError{Bytes: len(raw), Lines: strings.Count(raw, "\n")}
	}

	patch := model.FilePatch{Summary: summary}
	totalLines := 0
	if raw != "" {
		fileDiffs, err := godiff.ParseMultiFileDiff([]byte(raw))
		if err != nil {
			return model.FilePatch{}, fmt.Errorf("parse patch for %s: %w", folder, err)
		}
		rawBodies := splitRawBodies(raw)
		bodyOffset := 0
		for _, fileDiff := range fileDiffs {
			hunks, lines := mapHunks(fileDiff.Hunks, rawBodies[bodyOffset:])
			bodyOffset += len(fileDiff.Hunks)
			for i := range hunks {
				hunks[i].FilePath = fileDiffPath(fileDiff)
			}
			patch.Hunks = append(patch.Hunks, hunks...)
			totalLines += lines
		}
	}

	var firstErr error
	// ponytail: untracked files need one --no-index call each; use a temporary-tree diff if large untracked folders become common.
	for _, file := range files {
		if file.Status != model.StatusUntracked {
			continue
		}
		filePatch, err := s.LoadPatch(ctx, cmp, file.Path, file.Status, ignoreWhitespace)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for i := range filePatch.Hunks {
			filePatch.Hunks[i].FilePath = file.Path
		}
		patch.Hunks = append(patch.Hunks, filePatch.Hunks...)
		for _, hunk := range filePatch.Hunks {
			totalLines += len(hunk.Lines)
		}
	}

	if totalLines > maxPatchLines {
		return model.FilePatch{Summary: summary}, &OversizedError{Bytes: len(raw), Lines: totalLines}
	}
	if len(patch.Hunks) == 0 && firstErr != nil {
		return patch, firstErr
	}
	return patch, nil
}

func fileDiffPath(fileDiff *godiff.FileDiff) string {
	path := fileDiff.NewName
	if path == "/dev/null" {
		path = fileDiff.OrigName
	}
	path = strings.TrimPrefix(path, "a/")
	return strings.TrimPrefix(path, "b/")
}

// runDiff executes the appropriate git diff command for the file.
func (s *PatchService) runDiff(ctx context.Context, cmp model.ResolvedCompare, filePath string, status model.FileStatus, ignoreWS bool) (string, error) {
	if status == model.StatusUntracked {
		return s.runNoIndexDiff(ctx, filePath, ignoreWS)
	}

	args := []string{"diff", "--patch", "--no-color", "--no-ext-diff", "-M"}
	if ignoreWS {
		args = append(args, "-w")
	}
	args = append(args, cmp.DiffRange, "--", filePath)
	return s.Runner.RunGit(ctx, args...)
}

// runNoIndexDiff diffs /dev/null against an untracked file.
// git diff --no-index exits 1 when files differ, so we recover stdout from GitError.
func (s *PatchService) runNoIndexDiff(ctx context.Context, filePath string, ignoreWS bool) (string, error) {
	args := []string{"diff", "--no-index", "--patch", "--no-color", "--no-ext-diff"}
	if ignoreWS {
		args = append(args, "-w")
	}
	args = append(args, "--", "/dev/null", filePath)

	raw, err := s.Runner.RunGit(ctx, args...)
	if err != nil {
		var ge *gitexec.GitError
		if errors.As(err, &ge) && ge.ExitCode == 1 && ge.Stdout != "" {
			return ge.Stdout, nil
		}
		return "", err
	}
	return raw, nil
}

// splitRawBodies extracts raw hunk body text between @@ headers,
// preserving "\ No newline at end of file" markers that go-diff strips.
func splitRawBodies(raw string) []string {
	lines := strings.Split(raw, "\n")
	var bodies []string
	var bodyLines []string
	inHunk := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if inHunk {
				bodies = append(bodies, strings.Join(bodyLines, "\n"))
			}
			bodyLines = nil
			inHunk = false
			continue
		}
		if strings.HasPrefix(line, "@@") {
			if inHunk {
				bodies = append(bodies, strings.Join(bodyLines, "\n"))
			}
			bodyLines = nil
			inHunk = true
			continue
		}
		if inHunk {
			bodyLines = append(bodyLines, line)
		}
	}
	if inHunk {
		bodies = append(bodies, strings.Join(bodyLines, "\n"))
	}

	return bodies
}

// mapHunks converts go-diff hunks to domain Hunks using raw bodies for line parsing.
func mapHunks(src []*godiff.Hunk, rawBodies []string) ([]model.Hunk, int) {
	if len(src) == 0 {
		return nil, 0
	}

	hunks := make([]model.Hunk, 0, len(src))
	totalLines := 0

	for i, sh := range src {
		h := model.Hunk{
			Header:   sh.Section,
			OldStart: int(sh.OrigStartLine),
			OldLen:   int(sh.OrigLines),
			NewStart: int(sh.NewStartLine),
			NewLen:   int(sh.NewLines),
		}

		var body string
		if i < len(rawBodies) {
			body = rawBodies[i]
		}

		h.Lines = mapLines(body, h.OldStart, h.NewStart)
		totalLines += len(h.Lines)
		hunks = append(hunks, h)
	}

	return hunks, totalLines
}

// expandTabs replaces TAB characters in s with spaces, expanding each tab to
// the next tab stop (a multiple of tabWidth) measured from column 0 of s.
// A real terminal renders a tab as a jump to the next stop, but
// ansi.StringWidth/lipgloss.Width measure a tab as width 0 — so scry's
// layout math (soft-wrap segmentation, right-edge truncation, padding,
// side-by-side column widths) would misjudge how much of a tab-indented
// line actually fits. Expanding tabs once here, at parse time, keeps every
// downstream consumer of DiffLine.Text (search, intraline-change spans,
// syntax highlighting, wrap segmentation) consistent, since they all
// operate on byte offsets into Text.
func expandTabs(s string, tabWidth int) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
			continue
		}
		b.WriteRune(r)
		col += ansi.StringWidth(string(r))
	}
	return b.String()
}

// mapLines parses a raw hunk body into DiffLines with line numbers.
func mapLines(body string, oldStart, newStart int) []model.DiffLine {
	body = strings.TrimSuffix(body, "\n")
	if body == "" {
		return nil
	}

	rawLines := strings.Split(body, "\n")
	lines := make([]model.DiffLine, 0, len(rawLines))
	oldNo := oldStart
	newNo := newStart

	for _, rl := range rawLines {
		// Git sometimes omits the leading space for empty context lines.
		if len(rl) == 0 {
			o, n := oldNo, newNo
			lines = append(lines, model.DiffLine{
				Kind:  model.LineContext,
				OldNo: &o,
				NewNo: &n,
				Text:  "",
			})
			oldNo++
			newNo++
			continue
		}

		prefix := rl[0]
		text := expandTabs(rl[1:], tabWidth)

		switch prefix {
		case ' ':
			o, n := oldNo, newNo
			lines = append(lines, model.DiffLine{
				Kind:  model.LineContext,
				OldNo: &o,
				NewNo: &n,
				Text:  text,
			})
			oldNo++
			newNo++
		case '+':
			n := newNo
			lines = append(lines, model.DiffLine{
				Kind:  model.LineAdded,
				NewNo: &n,
				Text:  text,
			})
			newNo++
		case '-':
			o := oldNo
			lines = append(lines, model.DiffLine{
				Kind:  model.LineDeleted,
				OldNo: &o,
				Text:  text,
			})
			oldNo++
		case '\\':
			lines = append(lines, model.DiffLine{
				Kind: model.LineNoNewline,
			})
		}
	}

	return lines
}

// hasGitlinkMode checks whether any known diff header line declares the
// gitlink mode 160000 (submodule). Only index and mode lines are inspected,
// avoiding false-positives from file paths that happen to contain "160000".
func hasGitlinkMode(raw string) bool {
	header := raw
	if idx := strings.Index(raw, "\n@@"); idx >= 0 {
		header = raw[:idx]
	}
	for _, line := range strings.Split(header, "\n") {
		if strings.HasPrefix(line, "index ") && strings.HasSuffix(line, " 160000") {
			return true
		}
		switch line {
		case "old mode 160000", "new mode 160000",
			"new file mode 160000", "deleted file mode 160000":
			return true
		}
	}
	return false
}
