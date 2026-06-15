// Package source resolves repository context and compare specifications.
package source

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
)

// ResolveRepoContext queries git rev-parse to build a RepoContext.
func ResolveRepoContext(ctx context.Context, r gitexec.GitRunner) (model.RepoContext, error) {
	toplevel, err := r.RunGit(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return model.RepoContext{}, fmt.Errorf("failed to resolve worktree root: %w", err)
	}

	gitDir, err := r.RunGit(ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return model.RepoContext{}, fmt.Errorf("failed to resolve git dir: %w", err)
	}

	commonDir, err := r.RunGit(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return model.RepoContext{}, fmt.Errorf("failed to resolve git common dir: %w", err)
	}

	gitDir = strings.TrimSpace(gitDir)
	commonDir = strings.TrimSpace(commonDir)

	// git rev-parse --git-common-dir returns a relative path in the main
	// worktree (e.g. ".git") but an absolute path in linked worktrees.
	// Canonicalize: if relative, it equals gitDir (main worktree).
	if !filepath.IsAbs(commonDir) {
		commonDir = gitDir
	}

	return model.RepoContext{
		WorktreeRoot:     strings.TrimSpace(toplevel),
		GitDir:           gitDir,
		GitCommonDir:     commonDir,
		IsLinkedWorktree: gitDir != commonDir,
	}, nil
}

// CompareResolver resolves a CompareRequest into a ResolvedCompare.
type CompareResolver struct {
	Runner gitexec.GitRunner
}

// Resolve turns a CompareRequest into a fully-resolved ResolvedCompare.
func (cr *CompareResolver) Resolve(ctx context.Context, req model.CompareRequest) (model.ResolvedCompare, error) {
	br, err := cr.resolveBase(ctx, req.BaseRef, req.HeadRef, req.Basis)
	if err != nil {
		return model.ResolvedCompare{}, err
	}

	baseSHA, err := cr.resolveRef(ctx, br.ref)
	if err != nil {
		return model.ResolvedCompare{}, fmt.Errorf("failed to resolve base ref %q: %w", br.ref, err)
	}

	// Working tree mode: when head is omitted, diff base against working tree.
	if req.HeadRef == "" {
		diffRange := baseSHA
		if br.diffRange != "" {
			diffRange = br.diffRange
		}
		return model.ResolvedCompare{
			Repo:         req.Repo,
			BaseRef:      baseSHA,
			Basis:        normalizeBasis(req.Basis),
			WorkingTree:  true,
			DiffRange:    diffRange,
			WatchBaseRef: br.watchRef,
		}, nil
	}

	headSHA, err := cr.resolveRef(ctx, req.HeadRef)
	if err != nil {
		return model.ResolvedCompare{}, fmt.Errorf("failed to resolve head ref %q: %w", req.HeadRef, err)
	}

	res := model.ResolvedCompare{
		Repo:         req.Repo,
		BaseRef:      baseSHA,
		Basis:        normalizeBasis(req.Basis),
		HeadRef:      headSHA,
		WatchBaseRef: br.watchRef,
	}

	switch req.Mode {
	case model.CompareThreeDot:
		mb, err := cr.mergeBase(ctx, baseSHA, headSHA)
		if err != nil {
			return model.ResolvedCompare{}, fmt.Errorf("failed to compute merge-base: %w", err)
		}
		res.MergeBase = mb
		res.DiffRange = baseSHA + "..." + headSHA
	case model.CompareTwoDot:
		res.DiffRange = baseSHA + ".." + headSHA
	default:
		return model.ResolvedCompare{}, fmt.Errorf("unsupported compare mode: %q", req.Mode)
	}

	return res, nil
}

// baseResult holds the resolved base ref and an optional symbolic ref for
// watch-mode fingerprinting. watchRef is non-empty only when a fallback was
// used (upstream returns a symbolic ref that git can re-resolve each tick).
type baseResult struct {
	ref       string // resolved ref or SHA to use as base
	diffRange string // optional range string to pass to git diff in working-tree mode
	watchRef  string // symbolic fallback name for watch fingerprinting (e.g. "origin/main")
}

// resolveBase resolves the base ref. If empty, it tries @{upstream} first,
// then falls back to merge-base of the effective head and the default branch.
// headRef is the explicit --head value; when empty, HEAD is used for merge-base.
func (cr *CompareResolver) resolveBase(ctx context.Context, baseRef, headRef string, basis model.CompareBasis) (baseResult, error) {
	normalized := normalizeBasis(basis)
	if normalized == model.CompareBasisHeadDirty && headRef == "" {
		return baseResult{ref: "HEAD", diffRange: "HEAD"}, nil
	}
	if baseRef != "" {
		return baseResult{ref: baseRef}, nil
	}

	switch normalized {
	case model.CompareBasisLocalTrunk:
		return cr.resolveLocalTrunkBase(ctx, headRef)
	case model.CompareBasisHeadDirty:
		return baseResult{ref: "HEAD"}, nil
	default:
		return cr.resolveUpstreamBase(ctx, headRef)
	}
}

func normalizeBasis(basis model.CompareBasis) model.CompareBasis {
	switch basis {
	case model.CompareBasisLocalTrunk:
		return model.CompareBasisLocalTrunk
	case model.CompareBasisHeadDirty:
		return model.CompareBasisHeadDirty
	default:
		return model.CompareBasisUpstream
	}
}

func (cr *CompareResolver) resolveUpstreamBase(ctx context.Context, headRef string) (baseResult, error) {
	out, err := cr.Runner.RunGit(ctx, "rev-parse", "--symbolic-full-name", "--verify", "@{upstream}")
	if err == nil {
		upstream := strings.TrimSpace(out)
		// Use merge-base so the diff shows only the branch's own changes,
		// not the full 2-way divergence with upstream.
		mbHead := "HEAD"
		if headRef != "" {
			mbHead = headRef
		}
		mb, mbErr := cr.Runner.RunGit(ctx, "merge-base", mbHead, upstream)
		if mbErr == nil {
			return baseResult{ref: strings.TrimSpace(mb), watchRef: upstream}, nil
		}
		return baseResult{ref: upstream, watchRef: upstream}, nil
	}

	// No upstream — compute merge-base against the effective head.
	mbHead := "HEAD"
	if headRef != "" {
		mbHead = headRef
	}

	for _, fallback := range []string{"origin/HEAD", "origin/main", "origin/master"} {
		mb, err := cr.Runner.RunGit(ctx, "merge-base", mbHead, fallback)
		if err == nil {
			return baseResult{ref: strings.TrimSpace(mb), watchRef: fallback}, nil
		}
	}

	return baseResult{}, fmt.Errorf("no upstream configured and no fallback found; use --base to specify a base ref")
}

func (cr *CompareResolver) resolveLocalTrunkBase(ctx context.Context, headRef string) (baseResult, error) {
	branch, err := LocalDefaultBranch(ctx, cr.Runner)
	if err != nil {
		return baseResult{}, err
	}

	mbHead := "HEAD"
	if headRef != "" {
		mbHead = headRef
	}

	trunkRef := "refs/heads/" + branch
	mb, err := cr.Runner.RunGit(ctx, "merge-base", mbHead, trunkRef)
	if err != nil {
		return baseResult{}, fmt.Errorf("failed to compute merge-base against local trunk %q: %w", trunkRef, err)
	}
	return baseResult{ref: strings.TrimSpace(mb), watchRef: trunkRef}, nil
}

// resolveRef resolves a ref to its SHA via rev-parse --verify.
func (cr *CompareResolver) resolveRef(ctx context.Context, ref string) (string, error) {
	out, err := cr.Runner.RunGit(ctx, "rev-parse", "--verify", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// mergeBase computes the merge-base between two SHAs.
func (cr *CompareResolver) mergeBase(ctx context.Context, base, head string) (string, error) {
	out, err := cr.Runner.RunGit(ctx, "merge-base", base, head)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// LocalDefaultBranch returns the name of the local default branch, preferring
// "main" over "master". Returns an error if neither local branch exists.
func LocalDefaultBranch(ctx context.Context, r gitexec.GitRunner) (string, error) {
	for _, name := range []string{"main", "master"} {
		if _, err := r.RunGit(ctx, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no local default branch found (tried main, master)")
}

// WorktreeBaseRef returns the BaseRef to use for dashboard drill-down diffs:
// the merge-base of HEAD and the local default branch, so a worktree's diff
// shows its own changes relative to local trunk rather than @{upstream}.
// Returns "" when no local default branch exists or merge-base fails, so the
// caller falls back to the default @{upstream} resolution.
//
// Uses the fully qualified refs/heads/<branch> form for merge-base so a tag
// of the same name cannot be picked up instead of the local branch.
func WorktreeBaseRef(ctx context.Context, r gitexec.GitRunner) string {
	branch, err := LocalDefaultBranch(ctx, r)
	if err != nil {
		return ""
	}
	mb, err := r.RunGit(ctx, "merge-base", "HEAD", "refs/heads/"+branch)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(mb)
}

// BootstrapResult holds the resolved repo context and a repo-scoped runner.
type BootstrapResult struct {
	Repo   model.RepoContext
	Runner gitexec.GitRunner
}

// Bootstrap performs two-phase discovery: creates a runner at cwd to resolve
// RepoContext, then creates a permanent runner scoped to WorktreeRoot.
func Bootstrap(ctx context.Context, cwd string) (BootstrapResult, error) {
	discovery := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: cwd})

	repo, err := ResolveRepoContext(ctx, discovery)
	if err != nil {
		if result, fallbackErr := bootstrapFromWorktreeList(ctx, discovery); fallbackErr == nil {
			return result, nil
		}
		return BootstrapResult{}, err
	}

	repoRunner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: repo.WorktreeRoot})
	return BootstrapResult{Repo: repo, Runner: repoRunner}, nil
}

func bootstrapFromWorktreeList(ctx context.Context, discovery gitexec.GitRunner) (BootstrapResult, error) {
	entries, err := gitexec.WorktreeList(ctx, discovery)
	if err != nil {
		return BootstrapResult{}, err
	}

	branch := currentBranch(ctx, discovery)
	worktreeRoot := selectBootstrapWorktree(entries, branch)
	if worktreeRoot == "" {
		return BootstrapResult{}, fmt.Errorf("no non-bare worktree found")
	}

	runner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: worktreeRoot})
	repo, err := ResolveRepoContext(ctx, runner)
	if err != nil {
		return BootstrapResult{}, err
	}
	// A git-dir launch point is the dashboard host, not the linked worktree
	// selected as a command runner.
	repo.IsLinkedWorktree = false
	return BootstrapResult{Repo: repo, Runner: runner}, nil
}

func currentBranch(ctx context.Context, r gitexec.GitRunner) string {
	out, err := r.RunGit(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func selectBootstrapWorktree(entries []gitexec.WorktreeEntry, branch string) string {
	if branch != "" {
		ref := "refs/heads/" + branch
		for _, entry := range entries {
			if !entry.Bare && !entry.Prunable && entry.Branch == ref {
				return entry.Path
			}
		}
	}
	for _, entry := range entries {
		if !entry.Bare && !entry.Prunable {
			return entry.Path
		}
	}
	return ""
}
