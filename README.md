# Scry

**See what changed. Naught else.**

A minimal, keyboard-driven TUI for reviewing Git branch diffs with pull-request semantics.

Scry does one thing well: show you what changed between two refs, with the same three-dot comparison semantics GitHub uses for pull requests. No staging, no committing, no distractions.

## Why Scry?

| Tool | What it is | How Scry differs |
|------|-----------|-----------------|
| **lazygit / gitui / tig** | Full Git clients with staging, committing, rebasing | Scry stays out of write-ops by default. The only destructive actions — opt-in `--commit` and per-file `X` discard — are gated behind explicit confirmation. Purpose-built for review. |
| **delta / diff-so-fancy** | Diff renderers that enhance `git diff` output | Scry provides navigation, search, file-level workflow, and lazy loading. Not just a pager. |
| **GitHub web UI** | Browser-based PR review | Scry works offline, in your terminal, with no context switch. |

## Features (v0.1)

- Three-dot branch comparison (PR-style semantics by default)
- File list with status indicators and line counts
- Unified and side-by-side patch viewer with hunk navigation (`n`/`p`)
- Bidirectional search within patches (`/`, `Enter`, `N`)
- Whitespace-ignore toggle (`W`)
- Compare basis cycling (`b`): upstream, local trunk, or HEAD/dirty worktree
- Manual refresh (`r`)
- Lazy patch loading for responsive large diffs
- Graceful handling of binary files, submodules, and oversized patches

## Features (v0.2)

- **Watch mode** (`--watch`): auto-refresh when the repo state changes, with configurable polling interval (`--watch-interval`)
- **Idle screen**: shown when watch mode is active but no files have diverged yet; auto-transitions to file list on change
- **AI commit messages** (`--commit`): generate conventional commit messages via Claude; confirm, edit, or regenerate before committing
- **Auto-commit** (`--commit-auto`): skip confirmation and commit immediately after message generation (requires `--commit`)
- **Worktree dashboard**: list all git worktrees with dirty state, branch, and latest commit; drill down into any worktree's diff. Shown automatically when run from the primary repo with linked worktrees; use `--no-dashboard` to opt out

## Install

### From source (requires Go 1.24.2+)

```bash
go install github.com/alexivison/scry/cmd/scry@latest
```

### Prebuilt binaries

Download from [GitHub Releases](https://github.com/alexivison/scry/releases).

## Quick start

```bash
# Smart default: linked-worktree cwd → that worktree's diff;
# primary repo with linked worktrees → dashboard; primary alone → diff
scry

# Compare against an explicit base ref
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

# Force diff mode even when the smart default would pick the dashboard
scry --no-dashboard
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
| `s` | Toggle unified / side-by-side diff |
| `W` | Toggle whitespace-ignore mode |
| `b` | Cycle compare basis |
| `Tab` | Toggle split/modal layout |
| `o` | Open selected file in nvim |
| `c` | Generate commit message (when `--commit`) |
| `e` | Edit generated commit message |
| `r` | Refresh / regenerate commit message |
| `X` | Discard selected file's changes (modal y/N confirmation) |
| `?` | Show help |
| `q` | Quit |

## Requirements

- Go 1.24.2+ (build from source)
- Git (any reasonably modern version)
- A terminal with color support
- `ANTHROPIC_API_KEY` environment variable (only when using `--commit`)

## Non-goals

These are intentional omissions, not missing features:

- No staging, rebasing, cherry-picking, or conflict resolution. Commit is opt-in via `--commit`; per-file `X` discard is the only other write op and is gated behind a modal y/N confirmation.
- No hunk-level or bulk discard
- No inline PR comments or review thread management
- No plugin system
- No syntax-aware / AST diff mode

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Architecture

See [docs/SPEC.md](docs/SPEC.md) for the full technical specification, architecture, and implementation plan.

## License

[MIT](LICENSE)
