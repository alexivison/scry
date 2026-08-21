// Package app wires scry's bootstrap pipeline:
// Config → phase1 runner → RepoContext → phase2 runner → resolve compare → list files → launch TUI.
package app

import (
	"context"
	"fmt"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/commit"
	"github.com/alexivison/scry/internal/config"
	"github.com/alexivison/scry/internal/diff"
	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/source"
	"github.com/alexivison/scry/internal/terminal"
	"github.com/alexivison/scry/internal/ui"
)

// Run executes the full scry pipeline and returns an exit code.
func Run(cfg config.Config) int {
	if !terminal.IsTTY(os.Stdin) || !terminal.IsTTY(os.Stdout) {
		fmt.Fprintln(os.Stderr, "scry: not a terminal; scry requires an interactive TTY")
		return 128
	}

	ctx := context.Background()
	colorProfile := terminal.DetectColorProfile(terminal.OSEnv())

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 128
	}

	boot, err := source.Bootstrap(ctx, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 128
	}

	// Smart-default mode dispatch: linked-worktree cwd shows that worktree's
	// diff; primary cwd with linked worktrees shows the dashboard; primary cwd
	// alone shows its own diff. --no-dashboard forces diff mode. The worktree
	// list is only enumerated when the count actually matters so detection
	// failures fall through to the diff default rather than crashing.
	isLinked := boot.Repo.IsLinkedWorktree
	worktreeCount := 1
	if !cfg.NoDashboard && !isLinked {
		entries, err := gitexec.WorktreeList(ctx, boot.Runner)
		if err == nil {
			worktreeCount = len(entries)
		}
	}

	if cfg.ShouldUseDashboard(isLinked, worktreeCount) {
		return runDashboard(ctx, cfg, boot, colorProfile)
	}
	return runDiff(ctx, cfg, boot, colorProfile)
}

// runDiff is the normal diff-view pipeline.
func runDiff(ctx context.Context, cfg config.Config, boot source.BootstrapResult, colorProfile terminal.ColorProfile) int {
	resolver := &source.CompareResolver{Runner: boot.Runner}
	basis := model.CompareBasisUpstream
	req := model.CompareRequest{
		Repo:             boot.Repo,
		BaseRef:          cfg.BaseRef,
		Basis:            basis,
		HeadRef:          cfg.HeadRef,
		Mode:             cfg.Mode,
		IgnoreWhitespace: cfg.IgnoreWhitespace,
	}
	cmp, err := resolver.Resolve(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 128
	}

	metaSvc := &diff.MetadataService{Runner: boot.Runner}
	files, err := metaSvc.ListFiles(ctx, cmp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 128
	}

	state := initialDiffState(cfg, cmp, basis, files)

	patchSvc := &diff.PatchService{Runner: boot.Runner}
	opts := []ui.ModelOption{
		ui.WithColorProfile(colorProfile),
		ui.WithPatchLoader(patchSvc),
		ui.WithMetadataLoader(metaSvc),
		ui.WithCompareResolver(resolver, req),
		ui.WithFileDiscarder(&fileDiscarderImpl{runner: boot.Runner, workdir: boot.Repo.WorktreeRoot}),
	}
	if cfg.Commit {
		provider, err := commit.NewClaudeProvider(
			"", // reads ANTHROPIC_API_KEY from env
			cfg.CommitModel,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scry: %v\n", err)
			return 128
		}

		cp := &commitProviderAdapter{provider: provider, git: boot.Runner}
		executor := &commit.Executor{Git: boot.Runner}
		opts = append(opts, ui.WithCommitProvider(cp), ui.WithCommitExecutor(executor))
	}
	m := ui.NewModel(state, opts...)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 1
	}

	return 0
}

func initialDiffState(cfg config.Config, cmp model.ResolvedCompare, basis model.CompareBasis, files []model.FileSummary) model.AppState {
	return model.AppState{
		Compare:          cmp,
		CompareBasis:     basis,
		Files:            files,
		IgnoreWhitespace: cfg.IgnoreWhitespace,
		FocusPane:        model.PaneFiles,
		Layout:           model.LayoutSplit,
		PatchDiffMode:    model.PatchDiffModeSideBySide,
		Patches:          make(map[string]model.PatchLoadState),
		CommitEnabled:    cfg.Commit,
		CommitAuto:       cfg.CommitAuto,
		GroupByDirectory: cfg.GroupByDirectory,
	}
}

func initialDashboardState(cfg config.Config) model.AppState {
	return model.AppState{
		FocusPane:        model.PaneDashboard,
		Layout:           model.LayoutSplit,
		PatchDiffMode:    model.PatchDiffModeSideBySide,
		CompareBasis:     model.CompareBasisUpstream,
		WorktreeMode:     true,
		GroupByDirectory: cfg.GroupByDirectory,
		RefreshInFlight:  true, // signal that initial load is pending
		Patches:          make(map[string]model.PatchLoadState),
	}
}

// commitProviderAdapter bridges the domain CommitMessageProvider to the UI CommitProvider.
// It collects staged data and delegates to the underlying provider.
type commitProviderAdapter struct {
	provider commit.CommitMessageProvider
	git      gitexec.GitRunner
}

func (a *commitProviderAdapter) Generate(ctx context.Context) (string, error) {
	if err := commit.CheckStagingGuard(ctx, a.git); err != nil {
		return "", err
	}
	diff, files, err := commit.CollectStagedSnapshot(ctx, a.git)
	if err != nil {
		return "", err
	}
	return a.provider.Generate(ctx, diff, files)
}

// runDashboard is the worktree dashboard pipeline.
// Launches the TUI immediately with an empty list and loads worktree data async.
func runDashboard(ctx context.Context, cfg config.Config, boot source.BootstrapResult, colorProfile terminal.ColorProfile) int {
	// Use a runner rooted at the common git dir (stable across worktree deletions).
	stableRoot := boot.Repo.GitCommonDir
	if stableRoot == "" {
		stableRoot = boot.Repo.WorktreeRoot
	}
	stableRunner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: stableRoot})
	loader := &worktreeLoaderImpl{runner: stableRunner}

	// Start with an empty worktree list — data loads async after TUI launches.
	state := initialDashboardState(cfg)

	drillDown := &drillDownProviderImpl{}
	removeRunner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: stableRoot, Timeout: gitexec.RemoveTimeout})
	remover := &worktreeRemoverImpl{runner: removeRunner}
	preview := &previewLoaderImpl{}
	compareLoader := &worktreeCompareLoaderImpl{}
	m := ui.NewModel(state, ui.WithColorProfile(colorProfile), ui.WithWorktreeLoader(loader), ui.WithDrillDownProvider(drillDown), ui.WithWorktreeRemover(remover), ui.WithPreviewLoader(preview), ui.WithWorktreeCompareLoader(compareLoader))
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 1
	}
	return 0
}

// worktreeLoaderImpl loads worktree info using gitexec commands.
type worktreeLoaderImpl struct {
	runner gitexec.GitRunner
}

func (w *worktreeLoaderImpl) LoadWorktrees(ctx context.Context) ([]model.WorktreeInfo, error) {
	entries, err := gitexec.WorktreeList(ctx, w.runner)
	if err != nil {
		return nil, err
	}

	infos := make([]model.WorktreeInfo, len(entries))
	var wg sync.WaitGroup
	for i, e := range entries {
		infos[i] = model.WorktreeInfo{
			Path:   e.Path,
			Branch: gitexec.ShortBranch(e.Branch),
			Bare:   e.Bare,
		}
		if e.Bare {
			continue
		}

		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			count, err := gitexec.StatusCount(ctx, w.runner, path)
			if err == nil {
				infos[idx].ChangedFiles = count
				infos[idx].Dirty = count > 0
			}
			meta, err := gitexec.CommitMeta(ctx, w.runner, path)
			if err == nil {
				infos[idx].CommitHash = meta.Hash
				infos[idx].Subject = meta.Subject
				infos[idx].HeadCommittedAt = meta.CommitDate
			}
		}(i, e.Path)
	}
	wg.Wait()

	return infos, nil
}

// worktreeRemoverImpl removes worktrees using git commands.
type worktreeRemoverImpl struct {
	runner gitexec.GitRunner
}

func (w *worktreeRemoverImpl) Remove(ctx context.Context, path string, force bool) error {
	return gitexec.WorktreeRemove(ctx, w.runner, path, force)
}

// drillDownProviderImpl creates a diff context for a specific worktree.
type drillDownProviderImpl struct{}

func (d *drillDownProviderImpl) LoadDrillDown(ctx context.Context, worktreePath string, basis model.CompareBasis) (ui.DrillDownResult, error) {
	resolved, err := resolveWorktreeCompare(ctx, worktreePath, basis)
	if err != nil {
		return ui.DrillDownResult{}, err
	}

	metaSvc := &diff.MetadataService{Runner: resolved.runner}
	files, err := metaSvc.ListFiles(ctx, resolved.compare)
	if err != nil {
		return ui.DrillDownResult{}, fmt.Errorf("list files for %s: %w", worktreePath, err)
	}

	patchSvc := &diff.PatchService{Runner: resolved.runner}
	return ui.DrillDownResult{
		Compare:       resolved.compare,
		Files:         files,
		PatchLoader:   patchSvc,
		FileDiscarder: &fileDiscarderImpl{runner: resolved.runner, workdir: resolved.repo.WorktreeRoot},
	}, nil
}

// fileDiscarderImpl implements ui.FileDiscarder via gitexec.DiscardFile.
type fileDiscarderImpl struct {
	runner  gitexec.GitRunner
	workdir string
}

func (f *fileDiscarderImpl) Discard(ctx context.Context, path string, untracked bool) error {
	return gitexec.DiscardFile(ctx, f.runner, f.workdir, path, untracked)
}

type previewLoaderImpl struct{}

func (p *previewLoaderImpl) LoadPreview(ctx context.Context, worktreePath string, basis model.CompareBasis) (ui.PreviewResult, error) {
	resolved, err := resolveWorktreeCompare(ctx, worktreePath, basis)
	if err != nil {
		return ui.PreviewResult{}, err
	}
	metaSvc := &diff.MetadataService{Runner: resolved.runner}
	files, err := metaSvc.ListFiles(ctx, resolved.compare)
	if err != nil {
		return ui.PreviewResult{}, err
	}
	return ui.PreviewResult{Compare: resolved.compare, Files: files}, nil
}

type worktreeCompareLoaderImpl struct{}

func (w *worktreeCompareLoaderImpl) LoadCompare(ctx context.Context, worktreePath string, basis model.CompareBasis) (model.ResolvedCompare, error) {
	resolved, err := resolveWorktreeCompare(ctx, worktreePath, basis)
	if err != nil {
		return model.ResolvedCompare{}, err
	}
	return resolved.compare, nil
}

type resolvedWorktreeCompare struct {
	runner  gitexec.GitRunner
	repo    model.RepoContext
	compare model.ResolvedCompare
}

func resolveWorktreeCompare(ctx context.Context, worktreePath string, basis model.CompareBasis) (resolvedWorktreeCompare, error) {
	runner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: worktreePath})
	repo, err := source.ResolveRepoContext(ctx, runner)
	if err != nil {
		return resolvedWorktreeCompare{}, fmt.Errorf("resolve repo for %s: %w", worktreePath, err)
	}

	resolver := &source.CompareResolver{Runner: runner}
	cmp, err := resolver.Resolve(ctx, model.CompareRequest{
		Repo:  repo,
		Basis: basis,
		Mode:  model.CompareThreeDot,
	})
	if err != nil {
		return resolvedWorktreeCompare{}, fmt.Errorf("resolve compare for %s: %w", worktreePath, err)
	}

	return resolvedWorktreeCompare{runner: runner, repo: repo, compare: cmp}, nil
}
