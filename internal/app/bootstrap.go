// Package app wires scry's bootstrap pipeline:
// Config → phase1 runner → RepoContext → phase2 runner → resolve compare → list files → launch TUI.
package app

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/commit"
	"github.com/alexivison/scry/internal/config"
	"github.com/alexivison/scry/internal/diff"
	"github.com/alexivison/scry/internal/gitexec"
	"github.com/alexivison/scry/internal/model"
	"github.com/alexivison/scry/internal/source"
	"github.com/alexivison/scry/internal/terminal"
	"github.com/alexivison/scry/internal/ui"
	"github.com/alexivison/scry/internal/watch"
)

// Run executes the full scry pipeline and returns an exit code.
func Run(cfg config.Config) int {
	if !terminal.IsTTY(os.Stdin) || !terminal.IsTTY(os.Stdout) {
		fmt.Fprintln(os.Stderr, "scry: not a terminal; scry requires an interactive TTY")
		return 128
	}

	ctx := context.Background()

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

	if cfg.Worktrees {
		return runDashboard(ctx, cfg, boot)
	}
	return runDiff(ctx, cfg, boot)
}

// runDiff is the normal diff-view pipeline.
func runDiff(ctx context.Context, cfg config.Config, boot source.BootstrapResult) int {
	resolver := &source.CompareResolver{Runner: boot.Runner}
	req := model.CompareRequest{
		Repo:             boot.Repo,
		BaseRef:          cfg.BaseRef,
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

	focusPane := model.PaneFiles
	if cfg.Watch && len(files) == 0 {
		focusPane = model.PaneIdle
	}

	state := model.AppState{
		Compare:          cmp,
		Files:            files,
		IgnoreWhitespace: cfg.IgnoreWhitespace,
		FocusPane:        focusPane,
		Patches:          make(map[string]model.PatchLoadState),
		WatchEnabled:     cfg.Watch,
		WatchInterval:    cfg.WatchInterval,
		CommitEnabled:    cfg.Commit,
		CommitAuto:       cfg.CommitAuto,
	}

	patchSvc := &diff.PatchService{Runner: boot.Runner}
	opts := []ui.ModelOption{
		ui.WithPatchLoader(patchSvc),
		ui.WithMetadataLoader(metaSvc),
		ui.WithCompareResolver(resolver, req),
	}
	if cfg.Watch {
		baseRef := cmp.WatchBaseRef // symbolic fallback from resolver (e.g. "origin/main")
		if baseRef == "" {
			// No fallback was needed — use explicit base or @{upstream}.
			baseRef = cfg.BaseRef
			if baseRef == "" {
				baseRef = "@{upstream}"
			}
		}
		opts = append(opts, ui.WithWatch(&watch.Fingerprinter{Runner: boot.Runner}, baseRef))
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
func runDashboard(ctx context.Context, cfg config.Config, boot source.BootstrapResult) int {
	loader := &worktreeLoaderImpl{runner: boot.Runner}
	worktrees, err := loader.LoadWorktrees(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 128
	}

	interval := cfg.WatchInterval
	if interval == 0 {
		interval = 2 * time.Second
	}

	state := model.AppState{
		FocusPane:    model.PaneDashboard,
		WorktreeMode: true,
		WatchEnabled: true,
		WatchInterval: interval,
		DashboardState: model.DashboardState{
			Worktrees: worktrees,
		},
		Patches: make(map[string]model.PatchLoadState),
	}

	drillDown := &drillDownProviderImpl{}
	m := ui.NewModel(state, ui.WithWorktreeLoader(loader), ui.WithDrillDownProvider(drillDown))
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

	infos := make([]model.WorktreeInfo, 0, len(entries))
	for _, e := range entries {
		info := model.WorktreeInfo{
			Path:   e.Path,
			Branch: gitexec.ShortBranch(e.Branch),
			Bare:   e.Bare,
		}

		if e.Bare {
			infos = append(infos, info)
			continue
		}

		// Get dirty state.
		clean, err := gitexec.StatusClean(ctx, w.runner, e.Path)
		if err == nil {
			info.Dirty = !clean
		}

		// Get commit info.
		hash, subject, err := gitexec.CommitSubject(ctx, w.runner, e.Path)
		if err == nil {
			info.CommitHash = hash
			info.Subject = subject
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// drillDownProviderImpl creates a diff context for a specific worktree.
type drillDownProviderImpl struct{}

func (d *drillDownProviderImpl) LoadDrillDown(ctx context.Context, worktreePath string) (ui.DrillDownResult, error) {
	runner := gitexec.NewGitRunner(gitexec.GitRunnerConfig{WorkDir: worktreePath})

	resolver := &source.CompareResolver{Runner: runner}
	req := model.CompareRequest{
		BaseRef: "", // resolves to @{upstream}
		HeadRef: "", // working tree mode
		Mode:    model.CompareThreeDot,
	}
	cmp, err := resolver.Resolve(ctx, req)
	if err != nil {
		return ui.DrillDownResult{}, fmt.Errorf("resolve compare for %s: %w", worktreePath, err)
	}

	metaSvc := &diff.MetadataService{Runner: runner}
	files, err := metaSvc.ListFiles(ctx, cmp)
	if err != nil {
		return ui.DrillDownResult{}, fmt.Errorf("list files for %s: %w", worktreePath, err)
	}

	patchSvc := &diff.PatchService{Runner: runner}
	return ui.DrillDownResult{
		Compare:     cmp,
		Files:       files,
		PatchLoader: patchSvc,
	}, nil
}
