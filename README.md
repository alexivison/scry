# Scry

**See what changed. Naught else.**

A minimal, keyboard-driven TUI for reviewing Git branch diffs with pull-request semantics.

Scry does one thing well: show you what changed between two refs, with the same three-dot comparison semantics GitHub uses for pull requests. No staging, no committing, no distractions.

## Why Scry?

| Tool | What it is | How Scry differs |
|------|-----------|-----------------|
| **lazygit / gitui / tig** | Full Git clients with staging, committing, rebasing | Scry is read-only by design. No risk of accidental operations. Purpose-built for review. |
| **delta / diff-so-fancy** | Diff renderers that enhance `git diff` output | Scry provides navigation, search, file-level workflow, and lazy loading. Not just a pager. |
| **GitHub web UI** | Browser-based PR review | Scry works offline, in your terminal, with no context switch. |

## Features (v0.1)

- Three-dot branch comparison (PR-style semantics by default)
- File list with status indicators and line counts
- Unified patch viewer with hunk navigation (`n`/`p`)
- Bidirectional search within patches (`/`, `Enter`, `N`)
- Whitespace-ignore toggle (`W`)
- Manual refresh (`r`)
- Lazy patch loading for responsive large diffs
- Graceful handling of binary files, submodules, and oversized patches

## Features (v0.2)

- **Watch mode** (`--watch`): auto-refresh when the repo state changes, with configurable polling interval (`--watch-interval`)
- **Idle screen**: shown when watch mode is active but no files have diverged yet; auto-transitions to file list on change
- **AI commit messages** (`--commit`): generate conventional commit messages via Claude; confirm, edit, or regenerate before committing
- **Auto-commit** (`--commit-auto`): skip confirmation and commit immediately after message generation (requires `--commit`)
- **Worktree dashboard** (`--worktrees`): list all git worktrees with dirty state, branch, and latest commit; drill down into any worktree's diff

## Install

### From source (requires Go 1.24.2+)

```bash
go install github.com/alexivison/scry/cmd/scry@latest
```

### Prebuilt binaries

Download from [GitHub Releases](https://github.com/alexivison/scry/releases).

## Quick start

```bash
# Review current branch against origin/main (default)
scry --base origin/main

# Compare two specific refs
scry --base v1.0.0 --head feature-branch

# Use two-dot comparison instead of three-dot
scry --base main --head HEAD --mode two-dot

# Watch mode: auto-refresh on repo changes
scry --base origin/main --watch

# Watch with custom polling interval
scry --base origin/main --watch --watch-interval 5s

# AI commit message generation (requires ANTHROPIC_API_KEY)
scry --base origin/main --commit

# Auto-commit without confirmation prompt
scry --base origin/main --commit --commit-auto

# Worktree dashboard
scry --worktrees
```

## Keymap

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate file list / worktree list |
| `l` / `Enter` | Select file / drill into worktree |
| `h` / `Esc` | Back to file list / dashboard |
| `n` / `p` | Next / previous hunk |
| `/` | Search in current patch |
| `Enter` | Next search match |
| `N` | Previous search match |
| `W` | Toggle whitespace-ignore mode |
| `Tab` | Toggle split/modal layout |
| `c` | Generate commit message (when `--commit`) |
| `e` | Edit generated commit message |
| `r` | Refresh / regenerate commit message |
| `?` | Show help |
| `q` | Quit |

## Requirements

- Go 1.24.2+ (build from source)
- Git (any reasonably modern version)
- A terminal with color support
- `ANTHROPIC_API_KEY` environment variable (only when using `--commit`)

## Non-goals

These are intentional omissions, not missing features:

- No staging or rebasing — commit is opt-in via `--commit`
- No inline PR comments or review thread management
- No plugin system
- No syntax-aware / AST diff mode

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Architecture

See [docs/SPEC.md](docs/SPEC.md) for the full technical specification, architecture, and implementation plan.

## License

[MIT](LICENSE)
