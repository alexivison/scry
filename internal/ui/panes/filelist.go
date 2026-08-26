package panes

import (
	"fmt"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/ui/theme"
)

var (
	fileSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.BrightText).Background(theme.SelectedBg)
	fileDimStyle      = lipgloss.NewStyle().Faint(true)

	// Status icon colors.
	statusAddedStyle    = lipgloss.NewStyle().Foreground(theme.Added)
	statusDeletedStyle  = lipgloss.NewStyle().Foreground(theme.Deleted)
	statusModifiedStyle = lipgloss.NewStyle().Foreground(theme.Dirty)
	statusRenamedStyle  = lipgloss.NewStyle().Foreground(theme.Accent)
	statusDefaultStyle  = lipgloss.NewStyle().Foreground(theme.Muted)

	// Directory header style.
	dirHeaderStyle           = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true)
	treeConnectorStyle       = lipgloss.NewStyle().Foreground(theme.Muted)
	treeConnectorSelectStyle = treeConnectorStyle.Background(theme.SelectedBg)
)

// FileListOpts holds optional parameters for file list rendering.
type FileListOpts struct {
	GroupByDirectory bool // when true, group files by directory with dim headers
	Filter           model.FileFilter
	Collapsed        map[string]bool
	Cursor           int
	UseCursor        bool
	HideCursor       bool
	NoteCount        int
}

// FileTreeRowKind identifies whether a visible file tree row is a directory or file.
type FileTreeRowKind int

const (
	FileTreeRowDir FileTreeRowKind = iota
	FileTreeRowFile
	FileTreeRowNotes
)

// FileTreeRow is a visible row in the projected file tree.
type FileTreeRow struct {
	Kind          FileTreeRowKind
	Path          string
	Label         string
	Depth         int
	FileIndex     int
	Collapsed     bool
	Last          bool
	Continuations []bool
}

// FileTreeProjection is the visible tree plus cursor-derived selection data.
type FileTreeProjection struct {
	Rows          []FileTreeRow
	Cursor        int
	SelectedFile  int
	FilteredFiles int
	TotalFiles    int
}

type fileTreeNode struct {
	path  string
	dirs  map[string]*fileTreeNode
	files []int
}

// RenderFileList renders a scrollable file list constrained to the given dimensions.
// It adjusts scrollOffset to keep selectedIdx visible and returns the rendered
// string along with the new scroll offset.
func RenderFileList(files []model.FileSummary, selectedIdx, scrollOffset, width, height int, active bool, opts ...FileListOpts) (string, int) {
	var o FileListOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	if len(files) == 0 && o.NoteCount == 0 {
		return "No files changed.", 0
	}

	cursor := o.Cursor
	if !o.UseCursor {
		cursor, _ = FileTreeCursorForFile(files, o.Filter, o.Collapsed, selectedIdx)
	}

	proj := ProjectFileTree(files, o.Filter, o.Collapsed, cursor, o.NoteCount)
	if len(proj.Rows) == 0 {
		return EmptyFileListMessage(o.Filter), 0
	}

	scrollOffset = EnsureVisible(proj.Cursor, scrollOffset, height, len(proj.Rows))
	end := scrollOffset + height
	if end > len(proj.Rows) {
		end = len(proj.Rows)
	}

	lines := make([]string, 0, end-scrollOffset)
	for i := scrollOffset; i < end; i++ {
		row := proj.Rows[i]
		line := renderFileTreeRow(files, row, !o.HideCursor && i == proj.Cursor, width)
		if !active {
			line = fileDimStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), scrollOffset
}

// ProjectFileTree builds visible directory and file rows from the flat file list.
func ProjectFileTree(files []model.FileSummary, filter model.FileFilter, collapsed map[string]bool, cursor int, noteCount ...int) FileTreeProjection {
	root := &fileTreeNode{dirs: make(map[string]*fileTreeNode)}
	filtered := 0
	for i, file := range files {
		if !includeFile(file.Path, filter) {
			continue
		}
		filtered++
		root.addFile(file.Path, i)
	}

	rows := make([]FileTreeRow, 0, filtered+1)
	root.appendRows(&rows, files, collapsed, nil)
	if len(noteCount) > 0 && noteCount[0] > 0 {
		rows = append(rows, FileTreeRow{
			Kind:      FileTreeRowNotes,
			Label:     fmt.Sprintf("Notes (%d)", noteCount[0]),
			FileIndex: -1,
		})
	}

	if len(rows) == 0 {
		return FileTreeProjection{
			Rows:          nil,
			Cursor:        0,
			SelectedFile:  -1,
			FilteredFiles: filtered,
			TotalFiles:    len(files),
		}
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(rows) {
		cursor = len(rows) - 1
	}

	selectedFile := -1
	if rows[cursor].Kind == FileTreeRowFile {
		selectedFile = rows[cursor].FileIndex
	}

	return FileTreeProjection{
		Rows:          rows,
		Cursor:        cursor,
		SelectedFile:  selectedFile,
		FilteredFiles: filtered,
		TotalFiles:    len(files),
	}
}

// FileTreeCursorForFile returns the visible row cursor for a file index.
func FileTreeCursorForFile(files []model.FileSummary, filter model.FileFilter, collapsed map[string]bool, fileIndex int) (int, bool) {
	if fileIndex < 0 || fileIndex >= len(files) {
		return 0, false
	}
	proj := ProjectFileTree(files, filter, collapsed, 0)
	for i, row := range proj.Rows {
		if row.Kind == FileTreeRowFile && row.FileIndex == fileIndex {
			return i, true
		}
	}
	return proj.Cursor, false
}

// EmptyFileListMessage returns the empty-state text for the current filter.
func EmptyFileListMessage(filter model.FileFilter) string {
	switch filter {
	case model.FileFilterTests:
		return "No test files changed."
	case model.FileFilterNonTests:
		return "No non-test files changed."
	default:
		return "No files changed."
	}
}

// IsTestFile reports whether path matches common test file conventions.
func IsTestFile(path string) bool {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "__tests__", "tests":
			return true
		}
	}

	base := pathpkg.Base(path)
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return true
	case strings.Contains(base, ".test."), strings.Contains(base, ".spec."):
		return true
	case strings.HasSuffix(base, ".py") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")):
		return true
	case strings.HasSuffix(base, "Test.java"), strings.HasSuffix(base, "Tests.java"):
		return true
	case strings.HasSuffix(base, "Test.kt"), strings.HasSuffix(base, "Tests.kt"):
		return true
	default:
		return false
	}
}

func includeFile(path string, filter model.FileFilter) bool {
	switch filter {
	case model.FileFilterTests:
		return IsTestFile(path)
	case model.FileFilterNonTests:
		return !IsTestFile(path)
	default:
		return true
	}
}

func (n *fileTreeNode) addFile(filePath string, fileIndex int) {
	clean := strings.Trim(filepath.ToSlash(filePath), "/")
	if clean == "" {
		n.files = append(n.files, fileIndex)
		return
	}
	parts := strings.Split(clean, "/")
	node := n
	for i, part := range parts[:len(parts)-1] {
		dirPath := strings.Join(parts[:i+1], "/")
		if node.dirs == nil {
			node.dirs = make(map[string]*fileTreeNode)
		}
		child := node.dirs[part]
		if child == nil {
			child = &fileTreeNode{path: dirPath, dirs: make(map[string]*fileTreeNode)}
			node.dirs[part] = child
		}
		node = child
	}
	node.files = append(node.files, fileIndex)
}

func (n *fileTreeNode) appendRows(rows *[]FileTreeRow, files []model.FileSummary, collapsed map[string]bool, continuations []bool) {
	dirNames := make([]string, 0, len(n.dirs))
	for name := range n.dirs {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	sort.SliceStable(n.files, func(i, j int) bool {
		return files[n.files[i]].Path < files[n.files[j]].Path
	})

	total := len(dirNames) + len(n.files)
	for i, name := range dirNames {
		child := n.dirs[name]
		visibleChild := compactDir(child, collapsed)
		isCollapsed := collapsed != nil && collapsed[visibleChild.path]
		last := i == total-1
		*rows = append(*rows, FileTreeRow{
			Kind:          FileTreeRowDir,
			Path:          visibleChild.path,
			Label:         compactDirLabel(n.path, visibleChild.path, name),
			Depth:         len(continuations),
			FileIndex:     -1,
			Collapsed:     isCollapsed,
			Last:          last,
			Continuations: copyContinuations(continuations),
		})
		if !isCollapsed {
			nextContinuations := append(copyContinuations(continuations), !last)
			visibleChild.appendRows(rows, files, collapsed, nextContinuations)
		}
	}

	for i, fileIndex := range n.files {
		last := len(dirNames)+i == total-1
		*rows = append(*rows, FileTreeRow{
			Kind:          FileTreeRowFile,
			Path:          files[fileIndex].Path,
			Depth:         len(continuations),
			FileIndex:     fileIndex,
			Last:          last,
			Continuations: copyContinuations(continuations),
		})
	}
}

func compactDir(n *fileTreeNode, collapsed map[string]bool) *fileTreeNode {
	for n != nil && len(n.files) == 0 && len(n.dirs) == 1 {
		if collapsed != nil && collapsed[n.path] {
			return n
		}
		for _, child := range n.dirs {
			n = child
			break
		}
	}
	return n
}

func compactDirLabel(parentPath, dirPath, fallback string) string {
	if parentPath == "" {
		return dirPath
	}
	prefix := parentPath + "/"
	if label, ok := strings.CutPrefix(dirPath, prefix); ok {
		return label
	}
	return fallback
}

func copyContinuations(in []bool) []bool {
	if len(in) == 0 {
		return nil
	}
	out := make([]bool, len(in))
	copy(out, in)
	return out
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

func renderFileEntry(f model.FileSummary, idx, selectedIdx, width int) string {
	return renderFileTreeFileEntry(f, idx == selectedIdx, width, FileTreeRow{Last: true})
}

func renderFileTreeRow(files []model.FileSummary, row FileTreeRow, selected bool, width int) string {
	switch row.Kind {
	case FileTreeRowDir:
		return renderFileTreeDirEntry(row, selected, width)
	case FileTreeRowNotes:
		return renderFileTreeNotesEntry(row, selected, width)
	default:
		return renderFileTreeFileEntry(files[row.FileIndex], selected, width, row)
	}
}

func renderFileTreeNotesEntry(row FileTreeRow, selected bool, width int) string {
	const separator = "── "
	label := padOrTruncateToWidth(separator+row.Label, width)
	if selected {
		return fileSelectedStyle.Width(width).Render(label)
	}
	separatorWidth := lipgloss.Width(separator)
	if width <= separatorWidth {
		return treeConnectorStyle.Render(label)
	}
	return treeConnectorStyle.Render(separator) + statusModifiedStyle.Render(padOrTruncateToWidth(row.Label, width-separatorWidth))
}

func renderFileTreeDirEntry(row FileTreeRow, selected bool, width int) string {
	marker := "[-]"
	if row.Collapsed {
		marker = "[+]"
	}
	label := row.Label
	if label == "" {
		label = pathpkg.Base(row.Path)
	}
	label += "/"
	connector := treeConnector(row)
	connectorWidth := lipgloss.Width(connector)
	rest := marker + " " + label

	restWidth := width - connectorWidth
	if restWidth < 1 {
		restWidth = 1
	}
	if lipgloss.Width(rest) > restWidth {
		rest = truncatePath(rest, restWidth)
	}
	padded := padRightCells(rest, restWidth)
	if selected {
		return renderTreeConnector(row, true) + fileSelectedStyle.Width(restWidth).Render(padded)
	}
	return renderTreeConnector(row, false) + dirHeaderStyle.Render(padded)
}

func renderFileTreeFileEntry(f model.FileSummary, selected bool, width int, row FileTreeRow) string {
	path := f.Path
	if f.OldPath != "" {
		path = fmt.Sprintf("%s → %s", pathpkg.Base(f.OldPath), pathpkg.Base(f.Path))
	} else {
		path = pathpkg.Base(path)
	}

	connector := treeConnector(row)
	icon := RenderIcon(f.Status, selected)
	if selected {
		icon = StatusIcon(f.Status)
	}
	statusWidth := lipgloss.Width(StatusIcon(f.Status))
	connectorWidth := lipgloss.Width(connector)

	counts := RenderCounts(f, selected)
	if selected {
		counts = FormatCounts(f)
	}
	countsWidth := lipgloss.Width(FormatCounts(f))
	restWidth := width - connectorWidth
	if restWidth < 1 {
		restWidth = 1
	}

	labelWidth := restWidth - countsWidth - 1
	if labelWidth < 5 {
		labelWidth = 5
	}

	pathWidth := labelWidth - statusWidth - 1
	if pathWidth < 1 {
		pathWidth = 1
	}
	if lipgloss.Width(path) > pathWidth {
		path = truncatePath(path, pathWidth)
	}
	fileLabel := icon + " " + path
	paddedLabel := padRightCells(fileLabel, labelWidth)
	restLine := alignRight(paddedLabel, counts, restWidth)

	if selected {
		return renderTreeConnector(row, true) + fileSelectedStyle.Width(restWidth).Render(restLine)
	}
	return renderTreeConnector(row, false) + alignRight(textStyle.Render(paddedLabel), counts, restWidth)
}

func treeConnector(row FileTreeRow) string {
	var b strings.Builder
	for _, continues := range row.Continuations {
		if continues {
			b.WriteString("│ ")
		} else {
			b.WriteString("  ")
		}
	}
	if row.Last {
		b.WriteString("└─ ")
	} else {
		b.WriteString("├─ ")
	}
	return b.String()
}

func renderTreeConnector(row FileTreeRow, selected bool) string {
	connector := treeConnector(row)
	if selected {
		return treeConnectorSelectStyle.Render(connector)
	}
	return treeConnectorStyle.Render(connector)
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

func alignRight(left, right string, width int) string {
	rightWidth := lipgloss.Width(right)
	leftWidth := lipgloss.Width(left)
	if leftWidth+1+rightWidth > width {
		left = truncatePath(left, width-1-rightWidth)
		leftWidth = lipgloss.Width(left)
	}
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func padRightCells(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
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

// RenderIcon returns a colored status icon.
func RenderIcon(s model.FileStatus, _ bool) string {
	return statusStyleFor(s).Render(StatusIcon(s))
}

// RenderCounts returns colored +/- counts.
func RenderCounts(f model.FileSummary, _ bool) string {
	if f.IsBinary {
		return statusDefaultStyle.Render("binary")
	}
	add := statusAddedStyle.Render(fmt.Sprintf("+%d", f.Additions))
	del := statusDeletedStyle.Render(fmt.Sprintf("-%d", f.Deletions))
	return add + " " + del
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
