// Package app wires scry's bootstrap pipeline:
// Config → phase1 runner → RepoContext → phase2 runner → resolve compare → list files → launch TUI.
package app

import (
	"context"
	"fmt"
	"os"

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

	state := model.AppState{
		Compare:          cmp,
		Files:            files,
		IgnoreWhitespace: cfg.IgnoreWhitespace,
		FocusPane:        model.PaneFiles,
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
		baseRef := cfg.BaseRef
		if baseRef == "" {
			baseRef = "@{upstream}"
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
