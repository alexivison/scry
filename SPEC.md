# Scry Specification (v0.1)

## Project Overview
`Scry` is a minimal, read-only terminal UI for reviewing Git branch diffs with pull-request semantics.

The product goal is narrow and intentional: provide the fastest keyboard-only workflow to inspect what changed between two refs. It works in any terminal, with particular attention to tmux compatibility.

## Goals
- Deliver a focused diff review tool, not a general Git client.
- Default to three-dot comparison semantics for PR parity.
- Render large diffs responsively via metadata-first and lazy patch loading.
- Provide deterministic, scriptable behavior with clear CLI flags and exit codes.

## Explicit Non-Goals (v0.1)
- No staging, committing, rebasing, cherry-picking, or conflict resolution. *(v0.2 introduces opt-in AI-assisted commit via `--commit`; all other write operations remain non-goals.)*
- No inline PR comments or code review thread management.
- No plugin system.
- No syntax-aware/AST diff mode by default.
- No desktop app target in v0.1.

## Architecture

### Layered design
1. Source Resolution
- Resolve repository context, refs, and compare mode (`three-dot` default, `two-dot` optional).
- Detect linked worktrees and resolve `GitDir` / `GitCommonDir` separately from `WorktreeRoot`.

2. Git Command Boundary
- Execute all `git` calls through a single runner abstraction.

3. Diff Metadata Pipeline
- Build ordered file list with statuses and line counts.

4. Patch Loading and Parsing
- Load patch data per selected file, parse into domain model.

5. Review State Store
- Maintain typed UI state, caches, search context, and selection.

6. TUI Renderer
- Bubble Tea panes for file list, patch view, and status/help.

### Repository structure
```text
scry/
├── cmd/
│   └── scry/
│       └── main.go
├── internal/
│   ├── app/
│   ├── config/
│   ├── model/
│   ├── source/
│   ├── gitexec/
│   ├── diff/
│   ├── search/
│   ├── review/
│   ├── ui/
│   │   └── panes/
│   └── terminal/
├── testdata/
│   ├── repos/
│   └── golden/
├── scripts/
├── go.mod
└── README.md
```

### Module boundaries
- `model`, `source`, `diff`, `search`, `review` are UI-agnostic core logic.
- `ui` contains rendering and key handling only.
- `gitexec` is the only subprocess boundary.
- `app` wires interfaces; `cmd/scry` is process entrypoint only.

## Technical Specification

### Core types
```go
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
    Repo      RepoContext
    BaseRef   string
    HeadRef   string
    MergeBase string // SHA of merge-base in three-dot mode; empty string in two-dot mode.
    DiffRange string // Range string passed to git diff: "base...head" or "base..head".
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
    LineNoNewline LineKind = "no-newline" // "No newline at end of file"
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
```

### Core interfaces
```go
type CompareResolver interface {
    Resolve(ctx context.Context, req CompareRequest) (ResolvedCompare, error)
}

// GitRunner is the sole subprocess boundary. WorkDir is fixed at
// construction time (see bootstrap sequence below).
type GitRunner interface {
    RunGit(ctx context.Context, args ...string) ([]byte, error)
}

// GitRunnerConfig controls runner behavior.
type GitRunnerConfig struct {
    WorkDir string        // Working directory for all git commands.
    Timeout time.Duration // Per-command timeout. Default: 30s.
}

type MetadataService interface {
    ListFiles(ctx context.Context, cmp ResolvedCompare) ([]FileSummary, error)
}

type PatchService interface {
    // LoadPatch returns the parsed patch for a single file.
    //
    // Edge-case return contract:
    //   - Oversized patch (>50k lines or >8 MiB raw): return (FilePatch{Summary: valid, Hunks: nil}, ErrOversized).
    //   - Binary file: return (FilePatch{Summary: valid, Hunks: nil}, ErrBinaryFile).
    //   - Submodule: return (FilePatch{Summary: valid, Hunks: nil}, ErrSubmodule).
    //   - Parse failure: return (FilePatch{}, wrapped error).
    //
    // Callers check error type for fallback UI; Summary is always populated on sentinel errors.
    // ignoreWhitespace controls whether -w is appended to the git diff command.
    LoadPatch(ctx context.Context, cmp ResolvedCompare, filePath string, ignoreWhitespace bool) (FilePatch, error)
}

// Sentinel errors for PatchService edge cases.
var (
    ErrOversized  = errors.New("patch exceeds size threshold")
    ErrBinaryFile = errors.New("binary file")
    ErrSubmodule  = errors.New("submodule change")
)

type SearchDirection int

const (
    SearchNext SearchDirection = iota
    SearchPrev
)

type Index interface {
    // Build constructs the search index from parsed lines. Input is
    // already validated by the patch parser; this operation is infallible.
    Build(patch FilePatch)
    Find(query string, fromLine int, dir SearchDirection) (line int, ok bool)
}
```

### Typed UI state
```go
type Pane string

const (
    PaneFiles  Pane = "files"
    PanePatch  Pane = "patch"
    PaneSearch Pane = "search"
)

type LoadStatus string

const (
    LoadIdle    LoadStatus = "idle"
    LoadLoading LoadStatus = "loading"
    LoadLoaded  LoadStatus = "loaded"
    LoadFailed  LoadStatus = "failed"
)

type PatchLoadState struct {
    Status     LoadStatus
    Patch      *FilePatch
    Err        error
    Generation int
}

type AppState struct {
    Compare          ResolvedCompare
    Files            []FileSummary
    SelectedFile     int // Index into Files. -1 when Files is empty.
    Patches          map[string]PatchLoadState
    CacheGeneration  int
    IgnoreWhitespace bool
    SearchQuery      string
    FocusPane        Pane
}
```

### CLI config model
```go
// Config is parsed from CLI flags in cmd/scry and threaded into app bootstrap.
type Config struct {
    BaseRef          string      // --base; default: "" (resolved to @{upstream})
    HeadRef          string      // --head; default: "" (resolved to HEAD)
    Mode             CompareMode // --mode; default: CompareThreeDot
    IgnoreWhitespace bool        // --ignore-whitespace; default: false
}
```

### Command strategy

#### Bootstrap sequence
Startup proceeds in two phases to resolve the circular dependency between `GitRunner` (needs `WorktreeRoot`) and `RepoContext` (needs git commands):

1. **Phase 1 — Discovery runner:** Construct a temporary `GitRunner` bound to the process working directory (`os.Getwd()`). Use it to execute the `rev-parse` commands below.
2. **Phase 2 — Repo-scoped runner:** Construct the permanent `GitRunner` bound to `RepoContext.WorktreeRoot`. This runner is threaded through all subsequent operations.

#### Repository context resolution
Resolved once at startup (phase 1), before compare resolution:
- `WorktreeRoot` = `git rev-parse --show-toplevel`
- `GitDir` = `git rev-parse --absolute-git-dir`
- `GitCommonDir` = canonicalized `git rev-parse --git-common-dir`
- `IsLinkedWorktree` = `GitDir != GitCommonDir`

All subsequent `git` commands (phase 2 onward) execute with `WorktreeRoot` as working directory. No code path may construct paths via `WorktreeRoot + ".git" + "..."` — in a linked worktree, `.git` is a file, not a directory. Use `GitDir` or `GitCommonDir` for any state storage paths.

#### Default ref resolution
- `--head` omitted: defaults to `HEAD`.
- `--base` omitted: resolve upstream via `git rev-parse --symbolic-full-name --verify @{upstream}`.
- If upstream is absent or unresolvable: fatal error with actionable message (*"No upstream branch found. Set upstream with `git branch --set-upstream-to` or pass `--base` explicitly."*).
- Rename detection: always-on (`-M` in all diff commands). No user-facing toggle in v0.1.

#### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Clean exit (includes empty diff, normal quit, runtime errors surfaced in UI) |
| `1` | Unrecoverable runtime failure forcing abnormal termination after TUI started |
| `2` | Usage/config error (invalid flags, bad mode value) |
| `128` | Fatal startup failure before TUI (not a repo, `git` missing, refs unresolvable) |

#### Compare resolution
- Three-dot mode (default):
  - `git merge-base <base> <head>`
  - `git diff <base>...<head>`
- Two-dot mode (explicit):
  - `git diff <base>..<head>`

#### Metadata merge strategy
Use NUL-delimited output for path safety.

1. Authoritative stream for ordering and status:
- `git diff --name-status -z -M <range>`

2. Enrichment stream for line counts and binary markers:
- `git diff --numstat -z -M <range>`

3. Merge rules:
- Non-rename/copy key: `Path`
- Rename/copy key: `OldPath + "\x00" + Path`
- Build `[]FileSummary` from `--name-status -z` in emitted order.
- Build `map[key]numstat` from `--numstat -z`.
- Enrich summaries by key lookup.
- If counts are missing, set `0/0`, keep row, and emit non-fatal debug warning.

#### Patch loading strategy
- Per selected file:
  - `git diff --patch --no-color --no-ext-diff -M <range> -- <file>`
  - Append `-w` when whitespace-ignore mode is enabled.

#### Cache invalidation strategy
- Cache patch entries by file path within a cache generation.
- On whitespace toggle:
  - Increment `CacheGeneration`.
  - Clear the entire patch cache.
  - Reload selected file.
- Async responses include generation id and are discarded if stale.

#### Oversized patch threshold
- Dual gate: if `lineCount > 50,000` OR `rawPatchBytes > 8 MiB`, do not parse or render full patch.
- Show file metadata (additions/deletions/status) with fallback message: *"Patch too large to display (N lines, M bytes). Use `git diff -- <path>` to view."*

#### Search semantics
- Smart-case matching per-query: if the query contains any uppercase rune, the entire match is case-sensitive; otherwise case-insensitive.
- Wrap-around: forward search past the last line wraps to the first; backward search past the first wraps to the last.
- Start point: next line after cursor for forward (`Enter`), previous line before cursor for backward (`N`).
- Plain substring match only in v0.1; no regex support.

### Terminal capability strategy
- Require TTY for interactive mode; fail fast with actionable message otherwise.
- Minimum terminal dimensions: 80 columns, 24 rows. Reject smaller sizes with guidance (*"Terminal too small (NxM). Scry requires at least 80x24."*).
- Detect color capability (`NO_COLOR`, `COLORTERM`, terminfo fallback) and degrade styles gracefully.
- Detect tmux and handle resize events without layout corruption.

### Error model
- Fatal startup errors: not a Git repo, unresolved refs, missing `git` binary.
- Non-fatal runtime errors: single-file patch parse failure, unmatched stats row, oversized patch handling.
- Runtime failures must surface in status UI and must not crash the application.

## MVP Feature List and Acceptance Criteria

### F1. Compare target resolution
- Scope: `--base`, `--head`, `--mode=three-dot|two-dot`, repo context.
- Acceptance criteria:
  - Default mode is `three-dot`.
  - `--head` defaults to `HEAD`; `--base` defaults to `@{upstream}`.
  - Missing upstream without explicit `--base` produces fatal error with actionable message.
  - Invalid refs return clear errors.
  - Active compare range is visible in status UI.
  - Works identically in main checkout and linked worktrees.
  - `RepoContext` correctly distinguishes `GitDir` from `GitCommonDir` in linked worktrees.

### F2. Deterministic file list with status and counts
- Scope: merged metadata pipeline.
- Acceptance criteria:
  - File order follows `--name-status -z` emission order.
  - Status and rename path pairs are correct.
  - Add/delete counts align with `--numstat -z`.

### F3. Patch viewer and hunk navigation
- Scope: unified patch rendering, `n/p` hunk navigation.
- Navigation rules:
  - `n` moves viewport to the next hunk header. At the last hunk, `n` is a no-op (no wrap).
  - `p` moves viewport to the previous hunk header. At the first hunk, `p` is a no-op (no wrap).
  - On file selection, viewport starts at the first hunk.
- Acceptance criteria:
  - Hunk headers and line types render correctly.
  - `n`/`p` at boundaries are no-ops (no wrap, no crash).
  - Navigation behavior is deterministic and documented.

### F4. Lazy loading and responsive rendering
- Scope: metadata-first paint, patch-on-selection, viewport virtualization.
- Acceptance criteria:
  - File list appears before full patch population.
  - Revisiting file in same generation uses cache.
  - UI remains responsive on large fixture repository.

### F5. Bidirectional patch search
- Scope: `/` enter search, `Enter` next, `N` previous.
- Acceptance criteria:
  - Smart-case matching: all-lowercase query is case-insensitive; any uppercase triggers case-sensitive.
  - Forward search (`Enter`) starts from the line after the cursor; backward (`N`) from the line before.
  - Search wraps around in both directions.
  - Empty query is a no-op.
  - No-match state is explicit and stable.

### F6. Whitespace-ignore toggle with global cache reset
- Scope: toggle key `W` for `-w` mode.
- Acceptance criteria:
  - Toggle clears full patch cache and reloads selected file.
  - Stale async results are dropped by generation guard.

### F6a. Manual refresh
- Scope: keybinding `r` to reload current compare range.
- Behavior:
  - Increment cache generation, clear patch cache, reload metadata.
  - Preserve selected file by path when still present; if removed, select nearest valid row.
  - Reuse same refresh pipeline as initial load and async patch loading.
  - Ignore stale async responses using generation guard.
- Acceptance criteria:
  - Pressing `r` updates file list and patch data to current repository state.
  - No stale patch content is displayed after refresh.
  - UI remains responsive during refresh.
  - `r` appears in key-help text.

### F7. Edge-case safety
- Scope: binary files, submodule changes, oversized patches (>50k lines or >8 MiB).
- Acceptance criteria:
  - Renderer never crashes on these cases.
  - Oversized patches show metadata with actionable fallback message.
  - User sees clear fallback messaging for binary and submodule entries.

### F8. Operational polish
- Scope: key help, clean exit, CLI docs, tmux behavior.
- Acceptance criteria:
  - `q` exits cleanly and restores terminal state.
  - `--help` documents all v0.1 flags.
  - Smoke-tested in tmux and non-tmux terminals.

## Task Breakdown

| Task | Deliverable | Dependencies | Verification |
|---|---|---|---|
| T1 | Repository bootstrap, `pflag` CLI, config model | none | `go test ./cmd/scry ./internal/config` |
| T2 | `gitexec` runner with timeout and stderr-rich errors | T1 | `go test ./internal/gitexec` |
| T3 | Source resolver with three-dot default, two-dot option, and worktree-safe repo context | T2 | `go test ./internal/source ./internal/model` |
| T4 | Metadata parser and explicit `name-status`/`numstat` merge | T2, T3 | `go test ./internal/diff -run TestMetadataMerge` |
| T5 | Patch parser/loader service and domain mapping | T2, T3 | `go test ./internal/diff -run TestLoadPatch` |
| T6 | Bubble Tea shell with synchronous bootstrap data render | T1, T3, T4 | `go test ./internal/ui -run TestShellRender` |
| T7 | Patch pane and hunk navigation | T5, T6 | `go test ./internal/ui -run TestHunkNavigation` |
| T8 | Async lazy patch loading, cache, viewport virtualization | T7 | `go test ./internal/review ./internal/ui -run TestLazyLoad` |
| T9 | Directional search index and UI wiring | T7 | `go test ./internal/search ./internal/ui -run TestDirectionalSearch` |
| T9a | Manual refresh action (`r`), generation bump, full cache invalidation, metadata reload, selection reconciliation | T8 | `go test ./internal/review ./internal/ui -run TestManualRefresh` |
| T10 | Whitespace toggle (calls shared cache-reset helper from T9a) | T8, T9a | `go test ./internal/diff ./internal/ui -run TestWhitespaceGeneration` |
| T11 | Edge-case hardening (binary/submodule/oversize) | T5, T8 | `go test ./internal/diff ./internal/ui -run TestEdgeCases` |
| T12 | End-to-end fixtures (including linked worktree), tmux smoke checks, `scripts/bench.sh`, release checklist | T1-T11 | `go test ./... && go test -race ./... && ./scripts/bench.sh` |

### Dependency graph
```text
T1 -> T2 -> T3 ->+-> T4 -+-> T6 -> T7 -> T8 -> T9a -> T10 -> T12
                 |        |         |      |             |
                 +-> T5 --+         +      +-> T9 ------+
                                    |      +-> T11 -----+
                                    |
                          (T6 also depends on T1, T3)
```

## Dependencies

### Go modules
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles`
- `github.com/sourcegraph/go-diff`
- `github.com/spf13/pflag`
- `golang.org/x/sync/errgroup`

### Testing
- Standard library `testing` package only (no testify). Table-driven tests with `t.Parallel()`.

### External binaries
- Required: `git`
- Optional (future PR adapter): `gh`

### Deferred by design
- `cobra` (not required for v0.1 single-command CLI)
- `go-git`/libgit2 bindings (Git CLI remains source of truth for v0.1)

### v0.2 additions (AI commit generator)
- `github.com/anthropics/anthropic-sdk-go` (Claude API client, initial provider)
- Future: `github.com/openai/openai-go` (OpenAI provider), local model adapters (Ollama)

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Diff semantics mismatch (`..` vs `...`) | Three-dot default, mode visibility in UI, resolver tests for both modes |
| Metadata stream merge corruption | NUL-safe parsing, canonical merge key, unmatched-row warning path |
| Stale cache after whitespace toggle | Full cache clear plus generation-based stale response discard |
| UI freezes on large diffs | Metadata-first paint, lazy patch fetch, viewport virtualization, patch-size ceiling |
| Terminal compatibility issues | Capability checks, graceful style fallback, tmux smoke tests |
| Scope creep into full Git client | Strict non-goals and task-scope enforcement |
| External command failures | Structured command errors surfaced in status pane |
| Linked worktree `.git` path failure | `RepoContext` resolved at startup; `.git` paths derived from `GitDir`/`GitCommonDir`, never from string concatenation on `WorktreeRoot` |

## Success Criteria (v0.1)

### Functional
- All MVP features F1-F8 (including F6a) satisfy acceptance criteria.
- Full keyboard-only review loop is complete and reliable.

### Correctness
- Golden fixtures verify metadata and patch parity with `git diff` across simple, rename, binary, submodule, large, and linked worktree cases.
- No stale patch state appears after whitespace mode transitions.
- Linked worktree fixtures confirm identical diff output and correct `RepoContext` resolution.

### Reliability
- `go test ./...` and `go test -race ./...` pass.
- No panics during interactive and fixture-driven smoke tests.

### UX and performance
- File list first paint target: under 500ms on a medium local fixture baseline.
- App remains interactive during asynchronous patch loads.

### Release gate
When all criteria above pass and non-goals remain intact, we tag `v0.1.0`.

## Post-v0.1 Roadmap

### Watch Mode (`--watch`) — v0.2, Polling-First

#### Objective
Continuously monitor divergence from the configured base/head and auto-refresh when fingerprint changes.

#### CLI
- `--watch` (bool): enable watch loop.
- `--watch-interval` (duration, default `2s`, min `500ms`).

#### Fingerprint design
- Baseline fingerprint command:
  - `git rev-parse HEAD refs/remotes/origin/main`
- Optional extended fingerprint (config-gated):
  - Include `git status --porcelain` hash for working-tree sensitivity.
- Fingerprint change is the only trigger for auto-refresh.
- Worktree note: `HEAD` is per-worktree, but remote-tracking refs (e.g. `refs/remotes/origin/main`) are shared across all worktrees. A `git fetch` in any sibling worktree mutates the shared ref and will trigger refresh in all watchers that include it in their fingerprint. This is expected behavior but should be documented for multi-agent setups.

#### Refresh behavior
- On fingerprint change:
  - Execute same refresh path as manual `r` (F6a).
  - Increment generation, clear cache, reload metadata.
- Debounce rule:
  - If refresh already in flight, skip new trigger and evaluate on next tick.

#### State additions
- `WatchEnabled bool`
- `WatchInterval time.Duration`
- `LastFingerprint string`
- `RefreshInFlight bool`
- `LastRefreshAt time.Time`

#### Acceptance criteria
- No-refresh churn when fingerprint is stable.
- Auto-refresh occurs within one watch interval after fingerprint change.
- No stale patch state after rapid successive repository updates.

### Idle Screen and Auto-Refresh — v0.2

#### Behavior
- If launched with `--watch` and no divergence, show idle screen:
  - Compare target summary.
  - Watch interval.
  - Last fingerprint check time.
  - Key hints (`q`, `r`).
- On first detected divergence, transition automatically to normal review view.

#### Acceptance criteria
- Idle view displays without attempting unnecessary patch loads.
- Transition to review view is automatic and non-disruptive.

### tmux Session Integration — v0.2

See [docs/integrations/tmux-session.md](docs/integrations/tmux-session.md) for details on launching Scry as a long-lived process in a tmux session alongside other tools.

### PR Resolver (`--pr`) — v0.2

#### Objective
Resolve compare refs from a PR identifier while reusing existing diff pipeline.

#### CLI
- `--pr <number|url|branch>`

#### Behavior
- Resolve base/head refs via `gh` (`gh pr view`/`gh api`).
- Feed resolved refs into existing compare resolver.
- Fallback with actionable error when `gh` unavailable or unauthenticated.

#### Acceptance criteria
- `--pr` and manual `--base/--head` produce equivalent diff outputs when refs match.

### AI Commit Message Generator — v0.2

#### Objective
Generate a contextual commit message from the current diff using an LLM, then optionally execute the commit. This extends Scry from a read-only review tool into an assist-and-act workflow while keeping the core diff experience unchanged.

#### CLI
- `--commit` (bool): enter commit-generation mode after review.
- `--commit-provider` (string, default `claude`): LLM provider to use (`claude` initially; future: `openai`, `ollama`, `custom`).
- `--commit-model` (string): override default model for the selected provider (e.g. `claude-sonnet-4-20250514`).
- `--commit-auto` (bool, default `false`): skip interactive confirmation and commit immediately with the generated message. Requires `--commit`.

#### Keybinding
- `c` in file-list pane: trigger commit message generation from the current diff.

#### Behavior
1. **Diff collection**: gather the full unified diff for the current compare range (reuse existing `PatchService` pipeline).
2. **Prompt construction**: build a structured prompt containing:
   - The complete unified diff.
   - File-level summary (paths, statuses, line counts).
   - Instruction to produce a conventional-commit-style message with a concise subject line (≤72 chars) and optional body.
3. **LLM request**: send prompt to the configured provider.
4. **Interactive confirmation**:
   - Display the generated message in a confirmation pane.
   - Keys: `Enter` to accept and commit, `e` to edit in `$EDITOR` before committing, `r` to regenerate, `Esc` to cancel.
5. **Commit execution**: run `git commit -m "<message>"` via `gitexec` runner against `WorktreeRoot`.
6. **Post-commit**: show commit SHA in status bar, refresh diff view (generation bump, cache clear).

#### Provider abstraction
```go
type CommitMessageProvider interface {
    Generate(ctx context.Context, diff string, files []FileSummary) (string, error)
}
```
- Initial implementation: `ClaudeProvider` using the Anthropic Messages API.
- Provider selected at startup via `--commit-provider` flag.
- API keys read from environment variables (`ANTHROPIC_API_KEY`, future: `OPENAI_API_KEY`).

#### State additions
```go
type CommitState struct {
    GeneratedMessage string
    Provider         string
    InFlight         bool
    Err              error
}
```

#### Error handling
- Missing API key: surface actionable error in status bar, do not crash.
- LLM request failure (network, rate limit, invalid response): display error, allow retry via `r`.
- Commit failure (nothing staged, hook rejection): display git stderr in status pane.

#### Acceptance criteria
- Generated message accurately reflects the diff content and follows conventional commit style.
- `Enter` executes `git commit` and the new commit SHA is visible.
- `Esc` cancels without side effects.
- `e` opens `$EDITOR` with the generated message; on save, commit proceeds.
- `r` regenerates with a fresh LLM call.
- `--commit-auto` mode commits without interactive confirmation.
- Provider abstraction allows adding new backends without modifying core logic.
- Missing or invalid API key produces a clear, non-crashing error.

### Review Queue Mode — v0.2

#### Objective
Track review progress per file for a compare range.

#### Model
- States: `unseen`, `seen`, `needs-second-look`.
- Key by compare fingerprint + file path.
- Persist under `${GitDir}/scry/review-state.json` (per-worktree by default; isolates concurrent agent sessions).

#### UX
- Key to cycle state on selected file.
- Navigation to next unresolved (`unseen` or `needs-second-look`).

#### Acceptance criteria
- State survives app restart for same compare fingerprint.
- State resets automatically when compare fingerprint changes.

### Additional Future Features

#### Noise gate profiles
- Toggle filters for generated/vendor/lockfile-heavy paths.

#### Changed symbols jump list
- Symbol-level navigation for supported languages when parsing is available.

#### Trust overlay
- Rank risky file categories (auth, migrations, infra) higher in navigation.

#### Delta-since-last-review mode
- Compare from last reviewed commit fingerprint to current head to avoid rereading unchanged hunks.

### Optional Integrations

#### Clipboard/export slice
- Export selected file/hunk/context as structured markdown or plain text for external tools.

#### tmux session launcher
- See [docs/integrations/tmux-session.md](docs/integrations/tmux-session.md).

### Roadmap Priority Order (v0.2)
1. Watch mode (`--watch`) with polling fingerprint.
2. Idle screen + auto-transition.
3. AI commit message generator (`--commit`).
4. PR resolver (`--pr`).
5. Review queue mode.
6. Noise gate profiles.
7. Clipboard/export slice.
8. Delta-since-last-review mode.
9. Changed symbols jump list.
10. Trust overlay.
