// Package model defines the core domain types for scry.
package model

import "errors"

// RepoContext is resolved once at startup via git rev-parse.
// In a linked worktree, .git is a file (not a directory), so code
// must NEVER construct paths via WorktreeRoot + ".git" + "...".
type RepoContext struct {
	WorktreeRoot     string // git rev-parse --show-toplevel
	GitDir           string // git rev-parse --absolute-git-dir (per-worktree)
	GitCommonDir     string // git rev-parse --git-common-dir (shared across worktrees)
	IsLinkedWorktree bool   // GitDir != GitCommonDir after path canonicalization
}

type CompareMode string

const (
	CompareThreeDot CompareMode = "three-dot"
	CompareTwoDot   CompareMode = "two-dot"
)

type CompareRequest struct {
	Repo             RepoContext
	BaseRef          string
	HeadRef          string
	Mode             CompareMode
	IgnoreWhitespace bool
}

type ResolvedCompare struct {
	Repo        RepoContext
	BaseRef     string
	HeadRef     string
	WorkingTree bool   // true when diffing against the working tree (no head ref).
	MergeBase   string // SHA of merge-base in three-dot mode; empty string in two-dot mode.
	DiffRange   string // Range string passed to git diff: "base...head", "base..head", or just "base" in working tree mode.
}

type FileStatus string

const (
	StatusAdded    FileStatus = "A"
	StatusModified FileStatus = "M"
	StatusDeleted  FileStatus = "D"
	StatusRenamed  FileStatus = "R"
	StatusCopied   FileStatus = "C"
	StatusTypeChg  FileStatus = "T"
	StatusUnmerged FileStatus = "U"
)

type FileSummary struct {
	Path        string
	OldPath     string
	Status      FileStatus
	Additions   int
	Deletions   int
	IsBinary    bool
	IsSubmodule bool
}

type LineKind string

const (
	LineContext    LineKind = "context"
	LineAdded     LineKind = "added"
	LineDeleted   LineKind = "deleted"
	LineNoNewline LineKind = "no-newline"
)

type DiffLine struct {
	Kind  LineKind
	OldNo *int
	NewNo *int
	Text  string
}

type Hunk struct {
	Header           string
	OldStart, OldLen int
	NewStart, NewLen int
	Lines            []DiffLine
}

type FilePatch struct {
	Summary FileSummary
	Hunks   []Hunk
}

// Sentinel errors for PatchService edge cases.
var (
	ErrOversized  = errors.New("patch exceeds size threshold")
	ErrBinaryFile = errors.New("binary file")
	ErrSubmodule  = errors.New("submodule change")
)
