// Package app wires scry's bootstrap pipeline:
// Config → phase1 runner → RepoContext → phase2 runner → resolve compare → list files → launch TUI.
package app

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexivison/scry/internal/config"
	"github.com/alexivison/scry/internal/diff"
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
	cmp, err := resolver.Resolve(ctx, model.CompareRequest{
		Repo:             boot.Repo,
		BaseRef:          cfg.BaseRef,
		HeadRef:          cfg.HeadRef,
		Mode:             cfg.Mode,
		IgnoreWhitespace: cfg.IgnoreWhitespace,
	})
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
	}

	patchSvc := &diff.PatchService{Runner: boot.Runner}
	m := ui.NewModel(state, ui.WithPatchLoader(patchSvc), ui.WithMetadataLoader(metaSvc))
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "scry: %v\n", err)
		return 1
	}

	return 0
}
