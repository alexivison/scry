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

## Install

### From source (requires Go 1.22+)

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
```

## Keymap

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate file list |
| `Enter` | Select file / toggle |
| `n` / `p` | Next / previous hunk |
| `/` | Search in current patch |
| `Enter` | Next search match |
| `N` | Previous search match |
| `W` | Toggle whitespace-ignore mode |
| `r` | Refresh (reload from git) |
| `?` | Show help |
| `q` | Quit |

## Requirements

- Git (any reasonably modern version)
- A terminal with color support

## Non-goals (v0.1)

These are intentional omissions, not missing features:

- No staging, committing, rebasing, or conflict resolution
- No inline PR comments or review thread management
- No plugin system
- No syntax-aware / AST diff mode

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Architecture

See [SPEC.md](SPEC.md) for the full technical specification, architecture, and implementation plan.

## License

[MIT](LICENSE)
