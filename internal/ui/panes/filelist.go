package panes

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/review"
	"github.com/alexivison/scry/internal/ui/theme"
)

var (
	fileSelectedStyle = lipgloss.NewStyle().Reverse(true)
	fileDimStyle      = lipgloss.NewStyle().Faint(true)

	// Status icon colors.
	statusAddedStyle    = lipgloss.NewStyle().Foreground(theme.Added)
	statusDeletedStyle  = lipgloss.NewStyle().Foreground(theme.Deleted)
	statusModifiedStyle = lipgloss.NewStyle().Foreground(theme.Dirty)
	statusRenamedStyle  = lipgloss.NewStyle().Foreground(theme.HunkHeader)
	statusDefaultStyle  = lipgloss.NewStyle().Foreground(theme.Muted)

	// Freshness markers.
	freshnessHotStyle  = lipgloss.NewStyle().Foreground(theme.Added).Bold(true)
	freshnessWarmStyle = lipgloss.NewStyle().Foreground(theme.Muted)

	// Flag marker.
	flagStyle = lipgloss.NewStyle().Foreground(theme.Dirty).Bold(true)

	// Directory header style.
	dirHeaderStyle = lipgloss.NewStyle().Foreground(theme.Muted).Faint(true)
)

// FileListOpts holds optional parameters for file list rendering.
type FileListOpts struct {
	ChangeGen        map[string]int  // per-file last-change generation (nil to disable)
	CurrentGen       int             // current CacheGeneration for freshness calculation
	FlaggedFiles     map[string]bool // session-scoped bookmarks
	GroupByDirectory bool            // when true, group files by directory with dim headers
}

// RenderFileList renders a scrollable file list constrained to the given dimensions.
// It adjusts scrollOffset to keep selectedIdx visible and returns the rendered
// string along with the new scroll offset.
func RenderFileList(files []model.FileSummary, selectedIdx, scrollOffset, width, height int, active bool, opts ...FileListOpts) (string, int) {
	if len(files) == 0 {
		return "No files changed.", 0
	}

	var o FileListOpts
	if len(opts) > 0 {
		o = opts[0]
	}

	// When grouping, sort files by directory for proper grouping.
	if o.GroupByDirectory {
		files = sortByDirectory(files)
	}

	// Ensure selected item is visible.
	// When grouping, reduce effective height to account for directory headers.
	effectiveHeight := height
	if o.GroupByDirectory {
		headerCount := countHeadersInRange(files, scrollOffset, scrollOffset+height)
		if headerCount > 0 {
			effectiveHeight = height - headerCount
			if effectiveHeight < 1 {
				effectiveHeight = 1
			}
		}
	}
	scrollOffset = EnsureVisible(selectedIdx, scrollOffset, effectiveHeight, len(files))

	// Determine visible window.
	end := scrollOffset + height
	if end > len(files) {
		end = len(files)
	}

	// Initialize lastDir from the file before scrollOffset for consistent header logic.
	lastDir := ""
	if o.GroupByDirectory && scrollOffset > 0 {
		lastDir = fileDir(files[scrollOffset-1].Path)
	}

	lines := make([]string, 0, end-scrollOffset)
	for i := scrollOffset; i < end; i++ {
		// Insert directory header when grouping is enabled and directory changes.
		if o.GroupByDirectory {
			dir := fileDir(files[i].Path)
			if dir != lastDir {
				lastDir = dir
				if dir != "" && len(lines) < height {
					lines = append(lines, dirHeaderStyle.Render("  "+dir))
				}
			}
		}
		if len(lines) >= height {
			break
		}
		tier := review.FreshnessCold
		if o.ChangeGen != nil {
			if gen, ok := o.ChangeGen[files[i].Path]; ok {
				tier = review.ComputeFreshness(gen, o.CurrentGen)
			}
		}
		flagged := o.FlaggedFiles[files[i].Path]
		line := renderFileEntry(files[i], i, selectedIdx, width, tier, flagged)
		if !active {
			line = fileDimStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), scrollOffset
}

// EnsureVisible adjusts scrollOffset so selectedIdx is within the visible window.
func EnsureVisible(selectedIdx, scrollOffset, height, total int) int {
	if selectedIdx < 0 || total == 0 {
		return 0
	}
	if selectedIdx < scrollOffset {
		return selectedIdx
	}
	if selectedIdx >= scrollOffset+height {
		return selectedIdx - height + 1
	}
	return scrollOffset
}

func renderFileEntry(f model.FileSummary, idx, selectedIdx, width int, tier review.FreshnessTier, flagged bool) string {
	selected := idx == selectedIdx
	path := f.Path
	if f.OldPath != "" {
		path = fmt.Sprintf("%s → %s", f.OldPath, f.Path)
	}

	marker := prefixMarker(tier, flagged, selected)

	prefix := "  "
	if selected {
		prefix = "> "
	}

	// Reserve space: prefix(2) + marker(1) + gap(1) + status(1) + gap(2) + counts + gap(1).
	countsWidth := lipgloss.Width(FormatCounts(f))
	pathWidth := width - 2 - 1 - 1 - 1 - 2 - countsWidth - 1
	if pathWidth < 5 {
		pathWidth = 5
	}

	if lipgloss.Width(path) > pathWidth {
		path = truncatePath(path, pathWidth)
	}

	icon := RenderIcon(f.Status, selected)
	counts := RenderCounts(f, selected)
	paddedPath := fmt.Sprintf("%-*s", pathWidth, path)

	if selected {
		rev := fileSelectedStyle
		return rev.Render(prefix) + marker + rev.Render(" ") + icon + rev.Render("  "+paddedPath+" ") + counts
	}
	return prefix + marker + " " + icon + "  " + paddedPath + " " + counts
}

// prefixMarker returns a styled single-character prefix: flag takes priority over freshness.
func prefixMarker(tier review.FreshnessTier, flagged, selected bool) string {
	if flagged {
		s := flagStyle
		if selected {
			s = s.Reverse(true)
		}
		return s.Render("⚑")
	}
	switch tier {
	case review.FreshnessHot:
		s := freshnessHotStyle
		if selected {
			s = s.Reverse(true)
		}
		return s.Render("●")
	case review.FreshnessWarm:
		s := freshnessWarmStyle
		if selected {
			s = s.Reverse(true)
		}
		return s.Render("○")
	default:
		if selected {
			return fileSelectedStyle.Render(" ")
		}
		return " "
	}
}

// truncatePath trims a path to fit within maxWidth, adding "…" as ellipsis.
func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 1 {
		return "…"
	}
	// Walk runes until we'd exceed maxWidth-1 (leaving room for …).
	w := 0
	for i, r := range path {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth-1 {
			return path[:i] + "…"
		}
		w += rw
	}
	return path
}

// statusStyleFor returns the lipgloss style for a file status icon.
func statusStyleFor(s model.FileStatus) lipgloss.Style {
	switch s {
	case model.StatusAdded:
		return statusAddedStyle
	case model.StatusDeleted:
		return statusDeletedStyle
	case model.StatusModified:
		return statusModifiedStyle
	case model.StatusRenamed, model.StatusCopied:
		return statusRenamedStyle
	default:
		return statusDefaultStyle
	}
}

// RenderIcon returns a colored status icon, optionally with Reverse for selected rows.
func RenderIcon(s model.FileStatus, reversed bool) string {
	style := statusStyleFor(s)
	if reversed {
		style = style.Reverse(true)
	}
	return style.Render(StatusIcon(s))
}

// RenderCounts returns colored +/- counts, optionally with Reverse for selected rows.
func RenderCounts(f model.FileSummary, reversed bool) string {
	if f.IsBinary {
		style := statusDefaultStyle
		if reversed {
			style = style.Reverse(true)
		}
		return style.Render("binary")
	}
	addStyle := statusAddedStyle
	delStyle := statusDeletedStyle
	if reversed {
		addStyle = addStyle.Reverse(true)
		delStyle = delStyle.Reverse(true)
	}
	add := addStyle.Render(fmt.Sprintf("+%d", f.Additions))
	del := delStyle.Render(fmt.Sprintf("-%d", f.Deletions))
	sep := " "
	if reversed {
		sep = fileSelectedStyle.Render(" ")
	}
	return add + sep + del
}

// StatusIcon returns a single-character icon for a file status.
func StatusIcon(s model.FileStatus) string {
	switch s {
	case model.StatusAdded:
		return "A"
	case model.StatusModified:
		return "M"
	case model.StatusDeleted:
		return "D"
	case model.StatusRenamed:
		return "R"
	case model.StatusCopied:
		return "C"
	case model.StatusTypeChg:
		return "T"
	case model.StatusUnmerged:
		return "U"
	case model.StatusUntracked:
		return "?"
	default:
		return "?"
	}
}

// sortByDirectory returns a copy of files sorted by directory, preserving
// order within each directory. Root-level files come last.
func sortByDirectory(files []model.FileSummary) []model.FileSummary {
	sorted := make([]model.FileSummary, len(files))
	copy(sorted, files)
	sort.SliceStable(sorted, func(i, j int) bool {
		di := fileDir(sorted[i].Path)
		dj := fileDir(sorted[j].Path)
		// Root files (empty dir) sort after directories.
		if di == "" && dj != "" {
			return false
		}
		if di != "" && dj == "" {
			return true
		}
		return di < dj
	})
	return sorted
}

// countHeadersInRange counts how many directory headers would be inserted
// between file indices start and end.
func countHeadersInRange(files []model.FileSummary, start, end int) int {
	if start < 0 {
		start = 0
	}
	if end > len(files) {
		end = len(files)
	}
	count := 0
	lastDir := ""
	if start > 0 {
		lastDir = fileDir(files[start-1].Path)
	}
	for i := start; i < end; i++ {
		dir := fileDir(files[i].Path)
		if dir != lastDir && dir != "" {
			count++
			lastDir = dir
		} else if dir != lastDir {
			lastDir = dir
		}
	}
	return count
}

// fileDir returns the directory portion of a file path, with trailing slash.
// Returns "" for root-level files.
func fileDir(path string) string {
	dir := filepath.Dir(path)
	if dir == "." {
		return ""
	}
	return dir + "/"
}

// FormatCounts formats addition/deletion counts for display.
func FormatCounts(f model.FileSummary) string {
	if f.IsBinary {
		return "binary"
	}
	return fmt.Sprintf("+%d -%d", f.Additions, f.Deletions)
}
