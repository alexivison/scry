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
	baseRef, err := cr.resolveBase(ctx, req.BaseRef)
	if err != nil {
		return model.ResolvedCompare{}, err
	}

	headRef := req.HeadRef
	if headRef == "" {
		headRef = "HEAD"
	}

	headSHA, err := cr.resolveRef(ctx, headRef)
	if err != nil {
		return model.ResolvedCompare{}, fmt.Errorf("failed to resolve head ref %q: %w", headRef, err)
	}

	baseSHA, err := cr.resolveRef(ctx, baseRef)
	if err != nil {
		return model.ResolvedCompare{}, fmt.Errorf("failed to resolve base ref %q: %w", baseRef, err)
	}

	res := model.ResolvedCompare{
		Repo:    req.Repo,
		BaseRef: baseSHA,
		HeadRef: headSHA,
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

// resolveBase resolves the base ref. If empty, it resolves @{upstream}.
func (cr *CompareResolver) resolveBase(ctx context.Context, baseRef string) (string, error) {
	if baseRef != "" {
		return baseRef, nil
	}

	out, err := cr.Runner.RunGit(ctx, "rev-parse", "--symbolic-full-name", "--verify", "@{upstream}")
	if err != nil {
		return "", fmt.Errorf("no upstream configured; use --base to specify a base ref: %w", err)
	}
	return strings.TrimSpace(out), nil
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
		return BootstrapResult{}, err
	}

	repoRunner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: repo.WorktreeRoot})
	return BootstrapResult{Repo: repo, Runner: repoRunner}, nil
}
