# Agentic UX Improvements Plan

Scry's differentiator is being a **live-updating diff viewer**. These improvements lean into that — making the real-time experience better when the thing generating diffs is an agent (fast, many files, potentially across worktrees).

---

## 1. Freshness Indicators & "What Just Changed"

**Problem**: When watch mode detects a change and refreshes, the user sees the new file list but has no way to tell *which* files just changed. An agent might touch 3 files out of 30 — the user has to manually scan the list.

**Changes**:

### 1a. Track per-file "last changed" generation

Add `LastChangedGen int` to `FileSummary` (or a parallel map in `AppState`). On each watch refresh, compare the old file list to the new one — files whose diff content changed (new additions/deletions counts, or newly appeared/disappeared) get their `LastChangedGen` set to the current `CacheGeneration`.

```go
// in AppState
FileChangeGen map[string]int // path -> generation when file last changed
```

### 1b. Visual freshness marker in file list

Files that changed in the *most recent* refresh get a visible marker (e.g., a dot or `*` next to the filename, styled with a highlight color). Files that changed 2-3 refreshes ago get a dimmer marker. Older files show no marker.

This is a 3-tier recency indicator:
- **Hot** (changed this refresh): bright marker
- **Warm** (changed 1-2 refreshes ago): dim marker
- **Cold** (unchanged): no marker

### 1c. "Jump to next recently changed file" keybinding

`g` (or `G`) — jump to the next file in the list that has a "hot" or "warm" marker. This lets you quickly cycle through just what the agent touched without scrolling through unchanged files.

**Files to modify**: `internal/model/state.go`, `internal/ui/panes/filelist.go`, `internal/ui/model.go` (key handler + refresh reconciliation)

---

## 2. Smoother Real-Time Rendering Under Rapid Changes

**Problem**: An agent can modify 10+ files in under 2 seconds. The current 2s polling interval means changes batch naturally, but the refresh path clears the entire patch cache and reloads metadata. If the user is reading a patch when a refresh hits, the viewport jumps.

**Changes**:

### 2a. Preserve scroll position across refreshes

On watch refresh, if the currently-selected file still exists in the new file list *and* its patch content hasn't changed, preserve the viewport scroll offset and current hunk position. Only reset scroll if the patch content actually differs.

Currently, refresh clears the patch cache entirely (`review.ClearPatches()`), which forces a reload even for unchanged files. Instead:
- After metadata reload, compare old and new file summaries
- Only evict cache entries for files whose additions/deletions changed
- Preserve cached patches for unchanged files

### 2b. Diff-aware cache invalidation

Instead of clearing the whole patch cache on refresh, selectively invalidate:
- **Changed files**: evict from cache, reload if selected
- **Unchanged files**: keep cached patch, no reload needed
- **New files**: no cache entry exists, load on selection
- **Removed files**: evict from cache

This reduces unnecessary git diff calls from N (total files) to M (actually changed files) per refresh cycle.

**Files to modify**: `internal/review/cache.go`, `internal/review/refresh.go`, `internal/ui/model.go` (refresh handler)

### 2c. Event-driven refresh via fsnotify (with polling fallback)

**Problem**: The current 2s polling interval means up to 2 seconds of latency before the UI reflects an agent's write. For a "live" viewer, that's noticeable.

**Approach**: Use [fsnotify](https://github.com/fsnotify/fsnotify) to get OS-level file change notifications (inotify on Linux, kqueue on macOS). When a write event fires in the worktree, trigger a refresh immediately instead of waiting for the next poll tick.

**Why fsnotify (not alternatives)**:
- [rfsnotify](https://github.com/farmergreg/rfsnotify) and [gorph](https://github.com/sean9999/go-fsnotify-recursively) add recursive watching, but we don't need deep recursion — we watch worktree roots and let git figure out what changed
- [go-fswatch](https://github.com/andreaskoch/go-fswatch) is just polling with a wrapper, no benefit over what we have
- fsnotify is the standard (13k+ importers), well-maintained, cross-platform

**Design**:
- Watch the worktree root directory (and `.git` for committed changes)
- On any write/create/rename event, debounce for 100-200ms (agents often write multiple files in rapid succession), then trigger a refresh
- Polling remains as automatic fallback for environments where fsnotify doesn't work (NFS mounts, /proc, WSL edge cases, or when the inotify watch limit is hit)
- Detection: attempt `watcher.Add()` at startup — if it returns an error, log a warning and fall back to polling silently
- The `watch.interval` config still applies to the polling fallback

**Debounce strategy**: Collect events into a sliding window. Reset the timer on each new event. Only fire the refresh when the window closes (no new events for 150ms). This collapses an agent writing 10 files in 500ms into a single refresh.

```go
// Simplified event loop
for {
    select {
    case event := <-watcher.Events:
        debounceTimer.Reset(150 * time.Millisecond)
    case <-debounceTimer.C:
        triggerRefresh()
    case err := <-watcher.Errors:
        log.Warn("fsnotify error, falling back to polling", err)
        fallbackToPolling()
    }
}
```

**Files to modify**: `internal/review/refresh.go` (new watcher setup + debounce loop), `internal/app/bootstrap.go` (initialize watcher or polling), `go.mod` (add `github.com/fsnotify/fsnotify`)

---

## 3. Worktree Dashboard Enhancements

**Problem**: The dashboard shows worktrees with branch/dirty state, but doesn't surface *what's happening* in each worktree. When running multiple agents across worktrees, you want a glanceable summary of activity.

**Changes**:

### 3a. File change count in dashboard

Show the number of changed files per worktree next to the dirty indicator. Instead of just a yellow dot for "dirty", show e.g., `● 12 files` — this tells you the scope of work in each worktree at a glance.

```go
type WorktreeInfo struct {
    // ... existing fields
    ChangedFiles int // count of files with changes
}
```

Discovery: `git -C <path> diff --name-only | wc -l` (or parse `status --porcelain` output count).

### 3b. Last-activity timestamp per worktree

Track when each worktree last had a state change (dirty→dirtier, new commit, etc.). Display as relative time ("3s ago", "2m ago"). This surfaces which worktrees have active agents vs idle ones.

```go
type WorktreeInfo struct {
    // ... existing fields
    LastActivityAt time.Time // when this worktree last changed state
}
```

### 3c. Per-worktree diff summary on selection

When a worktree is selected (highlighted but not drilled into), show a preview in a side panel: top 5 changed files with their +/- counts. This gives you a quick peek without committing to a full drill-down.

### 3d. Delete worktree from dashboard

When an agent finishes work in a worktree, clean it up without leaving Scry.

- `d` on a selected worktree opens a confirmation prompt ("Delete worktree <branch>? y/n")
- If the worktree is dirty, show a stronger warning ("Worktree has uncommitted changes. Force delete? y/n") — maps to `git worktree remove --force`
- Cannot delete the main worktree (the non-linked one) — `d` is a no-op with a status bar message
- After deletion, refresh the worktree list and reconcile selection (move to nearest neighbor)
- Uses `git worktree remove <path>` under the hood

**Implementation**: Add `WorktreeRemove(ctx, runner, path, force)` to `internal/gitexec/worktree.go`. Add `ConfirmDelete` state + selected worktree path to `DashboardState`. Wire `d` → confirm → execute → refresh in `internal/ui/dashboard.go`.

**Files to modify**: `internal/gitexec/worktree.go`, `internal/model/worktree.go`, `internal/ui/dashboard.go`, `internal/ui/panes/dashboard.go`

---

## 4. Mention / Interaction — "Flag This Diff"

**Problem**: When reviewing agent-generated diffs in real-time, you want to mark things for attention without leaving the viewer. The current model is fully read-only with no way to annotate.

**Changes**:

### 4a. File-level flag/bookmark

`m` on a file toggles a flag (bookmark). Flagged files get a visible marker in the file list (e.g., `⚑` or `!`). Flags persist for the session (not across restarts — this isn't review queue state, it's a lightweight "I need to come back to this").

### 4b. "Jump to next flagged" keybinding

`M` — jump to the next flagged file. Combined with freshness markers, workflow becomes: scan hot files, flag anything concerning, continue reviewing, then `M` to revisit flagged items.

### 4c. Export flagged files list

`ctrl+e` — copy the list of flagged file paths to clipboard (or write to stdout on exit). This is the "mention" action — you flag concerning diffs, then hand that list to the agent or paste it into a conversation.

Format: one path per line, simple and pipeable.

```
src/auth/handler.go
internal/db/migrate.go
```

**Files to modify**: `internal/model/state.go` (flagged set), `internal/ui/model.go` (key handlers), `internal/ui/panes/filelist.go` (rendering), clipboard integration

---

## 5. CLI Simplification — Zero-Config Default

**Problem**: Scry currently requires up to 11 flags to configure. For the primary use case — watching an agent work across worktrees — you shouldn't need *any* flags. The tool should detect context and do the right thing.

**Goal**: `scry` with zero arguments does the right thing 90% of the time. Power-user knobs move to a config file rather than flags.

**Changes**:

### 5a. Auto-detect worktree dashboard

If the repo has linked worktrees (`git worktree list` returns >1 entry), launch the dashboard automatically. No `--worktrees` flag needed. The single-branch diff view becomes the fallback when there's only one worktree.

To force single-branch mode from a multi-worktree repo: `scry --no-dashboard` (or just drill into a worktree from the dashboard).

### 5b. Auto-enable watch mode

Watch mode becomes the default — it's the whole point when an agent is running. Add `--no-watch` as the escape hatch for one-shot usage (e.g., reviewing a finished branch).

### 5c. Auto-resolve base ref

When `--base` is not specified, resolve it per-worktree from the upstream tracking branch (`@{upstream}`). This already works today as the default, but make it more explicit: if a branch tracks `origin/main`, that's the base. No flag needed.

### 5d. Config file for persistent preferences

Introduce `~/.config/scry/config.toml` (and `.scry.toml` per-repo) for settings that are currently flags but rarely change between invocations:

```toml
# ~/.config/scry/config.toml

[watch]
interval = "2s"         # --watch-interval default
enabled = true          # --watch default (can be overridden with --no-watch)

[diff]
ignore_whitespace = false   # --ignore-whitespace default
mode = "three-dot"          # --mode default

[commit]
provider = "claude"         # --commit-provider
model = ""                  # --commit-model (empty = provider default)
auto = false                # --commit-auto
```

Precedence: **CLI flag > repo `.scry.toml` > user `~/.config/scry/config.toml` > built-in defaults**

### 5e. Resulting CLI surface

After simplification, the common invocations become:

```sh
scry                        # dashboard + watch (auto-detected)
scry --base main            # single-branch diff, explicit base, watch on
scry --commit               # enable AI commit message generation
scry --no-watch             # one-shot mode for finished branches
scry --no-dashboard         # force single-branch view in multi-worktree repo
```

Flags that remain as CLI-only (not config):
- `--base`, `--head` — per-invocation overrides
- `--commit`, `--commit-auto` — intentional write actions, should be explicit
- `--no-watch`, `--no-dashboard` — per-invocation opt-outs

Flags that move to config-only (removed from CLI):
- `--mode` → `diff.mode` in config (rarely changed per-invocation)
- `--commit-provider`, `--commit-model` → `commit.provider`, `commit.model` in config
- `--watch-interval` → `watch.interval` in config

Flags removed entirely:
- `--worktrees` → replaced by auto-detection + `--no-dashboard`
- `--watch` → on by default, replaced by `--no-watch`
- `--ignore-whitespace` → `diff.ignore_whitespace` in config

**Files to modify**: `internal/config/config.go` (config file loading, flag pruning), `cmd/scry/main.go` (simplified flag set), new `internal/config/file.go` (TOML parsing + precedence)

---

## Implementation Priority

Grouped into phases. Each phase is independently shippable.

### Phase 1: Foundation (theme + responsiveness)
1. **Color palette & theme system** (6a) — every other visual change builds on this, so it goes first
2. **Responsive breakpoints** (6h) — wire up the adaptive layout before adding chrome
3. **CLI simplification** (5a, 5b, 5c) — zero-config defaults, ship alongside the visual refresh

### Phase 2: Core visual upgrade
4. **Diff rendering upgrade** (6c) — background tinting, styled gutter, hunk separators — the biggest visual impact
5. **File list visual upgrade** (6d) — colored status icons, colored counts, better selection highlight
6. **Structured pane borders** (6b) — wrap panes in lipgloss borders with titles/footers
7. **Status bar redesign** (6e) — segmented info strip with mode badges

### Phase 3: Agentic UX features
8. **Freshness indicators** (1a, 1b, 1c) — "what did the agent just change" (builds on theme colors)
9. **Scroll preservation** (2a) — prevent viewport jumps on refresh
10. **File flags** (4a, 4b) — lightweight annotation
11. **Dashboard enhancements** (3a, 3b) — file counts, activity timestamps

### Phase 4: Polish & delight
12. **Page navigation & gg/G** (6i) — vim-power-user navigation
13. **Help overlay** (6f) — styled modal with grouped sections
14. **Loading spinners** (6g) — animated feedback for async ops
15. **Idle screen upgrade** (6j) — bordered, pulsing watch indicator
16. **Commit screen upgrade** (6k) — styled message area + key badges
17. **Config file** (5d) — persistent preferences

### Phase 5: Performance & advanced
18. **Diff-aware cache** (2b) — selective invalidation for large diffs
19. **Event-driven refresh** (2c) — fsnotify with polling fallback
20. **Delete worktree** (3d) — cleanup action from dashboard
21. **Dashboard preview** (3c) — side-panel file summary on selection
22. **Export flagged** (4c) — bridge to external tools
23. **Directory grouping** (6d, optional) — group files by directory in file list

---

## 6. UI Visual Polish & Responsiveness

**Problem**: The current UI is functional but visually flat. Selection is bold+reverse, colors are basic 16-color ANSI, there are no borders or panel structure, no scroll indicators, and the help/idle/commit screens are unstyled plain text. For a tool you stare at while an agent works, the visual experience matters — it should feel polished, like lazygit or k9s, not like raw terminal output.

**Principles**:
- **Adaptive color**: Use the terminal's detected color profile (already in `terminal.go`) to pick the right palette — 16-color basics for dumb terminals, rich 256/truecolor for modern ones
- **Information density over decoration**: Every pixel of chrome should communicate something
- **Responsive to the bone**: No hardcoded widths, every pane adapts to terminal size, graceful degradation at every breakpoint

### 6a. Cohesive color palette & theme system

**Current state**: Scattered `lipgloss.Color("2")`, `Color("8")`, etc. across 4 files. No central palette. Colors are raw ANSI numbers.

**Change**: Define a central `Theme` struct in a new `internal/ui/theme/theme.go`:

```go
type Theme struct {
    // Chrome
    StatusBarBg     lipgloss.AdaptiveColor
    StatusBarFg     lipgloss.AdaptiveColor
    DividerFg       lipgloss.AdaptiveColor
    BorderFg        lipgloss.AdaptiveColor

    // Diff
    AddedFg         lipgloss.AdaptiveColor
    AddedBg         lipgloss.AdaptiveColor  // subtle background tint
    DeletedFg       lipgloss.AdaptiveColor
    DeletedBg       lipgloss.AdaptiveColor  // subtle background tint
    HunkHeaderFg    lipgloss.AdaptiveColor
    HunkHeaderBg    lipgloss.AdaptiveColor
    GutterFg        lipgloss.AdaptiveColor
    ContextFg       lipgloss.AdaptiveColor

    // File list
    SelectedBg      lipgloss.AdaptiveColor
    SelectedFg      lipgloss.AdaptiveColor
    StatusAddedFg   lipgloss.AdaptiveColor  // green for "A"
    StatusDeletedFg lipgloss.AdaptiveColor  // red for "D"
    StatusModifiedFg lipgloss.AdaptiveColor // yellow for "M"
    CountAddFg      lipgloss.AdaptiveColor  // green for +N
    CountDelFg      lipgloss.AdaptiveColor  // red for -N

    // Freshness (from section 1)
    FreshHotFg      lipgloss.AdaptiveColor
    FreshWarmFg     lipgloss.AdaptiveColor

    // Dashboard
    CleanFg         lipgloss.AdaptiveColor
    DirtyFg         lipgloss.AdaptiveColor
    HashFg          lipgloss.AdaptiveColor

    // Search
    MatchBg         lipgloss.AdaptiveColor
    MatchFg         lipgloss.AdaptiveColor
    NotFoundBg      lipgloss.AdaptiveColor
    NotFoundFg      lipgloss.AdaptiveColor
}
```

Use `lipgloss.AdaptiveColor{Light: "...", Dark: "..."}` so the palette works on both light and dark terminal backgrounds. The default theme targets dark backgrounds (the 90% case for developer terminals), with a light theme as a config option.

**Files**: new `internal/ui/theme/theme.go`, update all style vars in `panes/*.go` and `model.go`

### 6b. Structured pane layout with borders

**Current state**: Panes are raw text blocks stitched together with `\n`. Split mode joins left+divider+right per line. No visual boundaries.

**Change**: Use lipgloss's `Border()` to wrap panes. This gives clear visual separation and looks professional.

```
┌─ Files ──────────────┐│┌─ src/auth/handler.go ───────────────────┐
│ > M  src/auth/handler │││ @@ -42,8 +42,12 @@ func HandleLogin()   │
│   A  src/auth/oauth.g │││  func HandleLogin(w http.ResponseWriter │
│   M  internal/db/conn │││ -    token := generateToken()           │
│   D  old/deprecated.g │││ +    token, err := generateToken()     │
│                       │││ +    if err != nil {                    │
│                       │││ +        http.Error(w, "auth failed",   │
│                       │││ +        return                         │
│                       │││ +    }                                  │
│                       │││                                         │
│  4 files  ● 2 hot     │││  hunk 1/3          42%                 │
└───────────────────────┘│└─────────────────────────────────────────┘
 origin/main (working tree)  [watch 2s 15:23:47]        12 files
```

Key details:
- **Pane titles**: File list shows "Files", patch pane shows the current filename — you always know what you're looking at
- **Pane footers**: File list shows file count + freshness summary; patch shows hunk position + scroll percentage
- **Border style**: `lipgloss.RoundedBorder()` for modern terminals, `lipgloss.NormalBorder()` as fallback
- **Active pane highlight**: The focused pane's border uses the accent color; inactive pane borders are dimmed
- The single `│` divider between panes is absorbed into the border — no separate divider column needed

**Files**: `internal/ui/model.go` (viewSplit, viewPatch, viewFileList), `internal/ui/panes/*.go`

### 6c. Diff rendering upgrade

**Current state**: Diffs show colored foreground text on the default background. The gutter is plain `NNNN MMMM`. Hunk headers are cyan+bold text. There's no visual separation between hunks.

**Changes**:

1. **Background tinting for added/deleted lines**: Subtle green background for `+` lines, subtle red for `-` lines (like GitHub's diff view). This is the single biggest visual improvement — it makes diffs scannable at a glance. Use `lipgloss.AdaptiveColor` to pick appropriate tint levels for the terminal's color depth.

2. **Styled gutter**: Dim the line numbers (gray foreground), add a thin separator between gutter and content. The gutter should feel like a margin, not data.

3. **Hunk separators**: Between hunks, render a horizontal rule (`───`) with the hunk header text. This visually breaks the diff into scannable sections instead of a continuous stream.

```
 ─── @@ -42,8 +42,12 @@ func HandleLogin() ───────────────
  42   42   func HandleLogin(w http.ResponseWriter, r *http.Request) {
  43      -     token := generateToken()
       43 +     token, err := generateToken()
       44 +     if err != nil {
       45 +         http.Error(w, "auth failed", 500)
       46 +         return
       47 +     }
  44   48       session.Save(token)
 ─── @@ -67,3 +71,5 @@ func HandleLogout() ──────────────
```

4. **Scroll position indicator**: A thin scroll indicator on the right edge of the patch pane, like a scrollbar. Show it as a highlighted segment of the border — position maps to `scrollOffset / totalLines`.

**Files**: `internal/ui/panes/patch.go` (all rendering), `internal/ui/theme/theme.go` (diff colors)

### 6d. File list visual upgrade

**Current state**: Files show `> M  path  +N -N` with bold+reverse selection. Status icons are plain letters. No color differentiation.

**Changes**:

1. **Colored status icons**: `A` in green, `D` in red, `M` in yellow, `R` in cyan. These are already universally recognized git colors.

2. **Colored counts**: `+N` in green, `-N` in red (like git diff --stat). Currently both are the same default color.

3. **Selection highlight**: Replace bold+reverse with a subtle background highlight (the accent color at ~20% intensity). This is less harsh than reverse video and looks more modern.

4. **Directory grouping** (optional, configurable): Group files by directory with a dim header:
```
  src/auth/
  > M  handler.go            +12 -3
    A  oauth.go               +8 -0
  internal/db/
    M  connection.go          +4 -1
```

This reduces visual noise when an agent modifies many files in the same package.

**Files**: `internal/ui/panes/filelist.go`, `internal/ui/theme/theme.go`

### 6e. Status bar redesign

**Current state**: A single dark-gray bar with left-aligned ref info and right-aligned file count. Functional but bland.

**Change**: Make the status bar a structured information strip:

```
 origin/main (working tree)  │  W  C  │  watch ● 2s  15:23:47  │  12 files
```

- **Segments separated by dim `│`** for visual rhythm
- **Mode indicators** (`W` for whitespace ignore, `C` for commit enabled) as styled badges — highlighted when active, dim when off
- **Watch indicator**: A colored dot (green = watching, yellow = refreshing, red = error) with interval and last check time
- **Breadcrumb in drill-down**: `Dashboard > feature-branch > src/auth/handler.go` — you always know where you are

**Files**: `internal/ui/model.go` (viewStatusBar)

### 6f. Help screen as styled overlay

**Current state**: Plain text list of keybindings, rendered directly into the main view area.

**Change**: Render help as a centered modal overlay with a border, title, and grouped sections:

```
╭─ Keyboard Shortcuts ─────────────────────────╮
│                                               │
│  Navigation                                   │
│    j/k       navigate file list / scroll diff │
│    l/Enter   select file / focus patch        │
│    h/Esc     back to file list                │
│    n/p       next / previous hunk             │
│                                               │
│  Search                                       │
│    /          search in patch                 │
│    Enter/N   next / previous match            │
│                                               │
│  Actions                                      │
│    m          flag file for review            │
│    M          jump to next flagged file       │
│    r          refresh                         │
│    W          toggle whitespace ignore        │
│    Tab        toggle split / modal layout     │
│    c          generate commit message         │
│                                               │
│  Press ? or Esc to close                      │
╰───────────────────────────────────────────────╯
```

The overlay renders on top of the existing view (dim the background), not replacing it. This lets the user peek at the help while still seeing context.

**Files**: `internal/ui/model.go` (viewHelp), `internal/ui/theme/theme.go`

### 6g. Loading states with spinners

**Current state**: "Loading...", "Generating commit message...", "Committing..." as plain text strings.

**Change**: Use [charmbracelet/bubbles spinner](https://github.com/charmbracelet/bubbles) for animated loading indicators:
- Patch loading: spinner in the patch pane area
- Commit generation: spinner with "Generating commit message..."
- Watch refresh: subtle spinner in the status bar watch segment
- Initial load: spinner centered in the terminal

The spinner integrates with Bubble Tea's update loop — no goroutine hacks needed.

**Files**: `internal/ui/model.go` (spinner component), `go.mod` (already has bubbles? check)

### 6h. Responsive breakpoints

**Current state**: Two modes — split (≥80 cols) and modal (fallback). Hardcoded file list width formula: `max(25, min(width*0.3, 50))`.

**Change**: Define three breakpoints with graceful transitions:

| Terminal width | Layout | Behavior |
|---|---|---|
| ≥120 cols | **Wide split** | File list gets more room (up to 60 cols), patch pane is comfortable |
| 80-119 cols | **Compact split** | Current behavior, tighter file list (25-40 cols) |
| 60-79 cols | **Modal only** | Full-width panes, Tab switches between them |
| 40-59 cols | **Minimal** | Truncated paths, no gutter line numbers, abbreviated status bar |
| <40 cols | **Too small** | Error message (current behavior) |

The "minimal" tier is new — it lets scry work in narrow tmux panes or split terminals without the "too small" error. At 50 cols, you can still see file names and diffs, just without the gutter.

Also handle height responsively:
- ≥30 rows: Show pane footer info (hunk position, scroll %)
- 24-29 rows: Standard layout, no footer
- 15-23 rows: Compact mode — reduce padding, shorter help overlay
- <15 rows: Too small

**Files**: `internal/ui/model.go` (WindowSizeMsg handler, view functions), `internal/terminal/terminal.go` (new breakpoint constants)

### 6i. Smooth scrolling & page navigation

**Current state**: `j`/`k` scroll one line at a time. No page up/down, no jump-to-top/bottom.

**Change**: Add standard vim-like navigation:

| Key | Action |
|---|---|
| `j`/`k` | Scroll 1 line (existing) |
| `ctrl+d`/`ctrl+u` | Half-page down/up |
| `ctrl+f`/`ctrl+b` | Full page down/up |
| `g` `g` | Jump to top (first file / first line) |
| `G` | Jump to bottom (last file / last line) |
| `{`/`}` | Jump to prev/next hunk (alias for `p`/`n`) |

For the file list: `ctrl+d`/`ctrl+u` move half the visible file count, `G` goes to last file, `gg` to first.

**Note on `gg`**: This is a two-key chord. Implement with a pending-key buffer: first `g` sets a "pending g" flag, second `g` within 500ms executes. Any other key (or timeout) cancels the pending state.

**Files**: `internal/ui/model.go` (Update key handlers)

### 6j. Idle screen upgrade

**Current state**: Plain text showing "Watching for changes..." with basic info.

**Change**: Make the idle screen visually distinctive — it's the "home screen" when waiting for agent activity:

```
╭─────────────────────────────────────────────╮
│                                             │
│           ◉  Watching for changes           │
│                                             │
│   Base       origin/main (working tree)     │
│   Interval   every 2s                       │
│   Last check 15:23:47                       │
│   Status     No divergence detected         │
│                                             │
│   q quit   ? help   r refresh               │
│                                             │
╰─────────────────────────────────────────────╯
```

The `◉` pulses (alternates between `◉` and `○` on each watch tick) to show the tool is alive and watching. The bordered box centers in the terminal.

**Files**: `internal/ui/idle.go`

### 6k. Commit screen upgrade

**Current state**: Plain text with the generated message and keybinding hints.

**Change**: Structured layout with the commit message in a bordered text area and styled action hints:

```
╭─ Commit Message ────────────────────────────╮
│                                             │
│  Fix: Update configuration handling for     │
│  new schema                                 │
│                                             │
│  - Migrate TOML parser to v2 API            │
│  - Add validation for nested keys           │
│  - Remove deprecated fallback paths         │
│                                             │
╰─────────────────────────────────────────────╯

  Enter  commit    e  edit in $EDITOR
  r  regenerate    Esc  cancel
```

Action hints use styled key badges (key in a contrasting background, description in normal text).

**Files**: `internal/ui/model.go` (viewCommit)

**Files to modify (summary for all of section 6)**:
- New: `internal/ui/theme/theme.go` (central palette + theme struct)
- Modified: `internal/ui/panes/patch.go` (diff rendering, hunk separators, scroll indicator)
- Modified: `internal/ui/panes/filelist.go` (colored icons/counts, selection style, directory grouping)
- Modified: `internal/ui/panes/dashboard.go` (themed styles)
- Modified: `internal/ui/model.go` (viewSplit borders, viewHelp overlay, viewStatusBar segments, viewCommit styled, spinner, breakpoints, page navigation)
- Modified: `internal/ui/idle.go` (bordered idle screen, pulsing indicator)
- Modified: `internal/terminal/terminal.go` (breakpoint constants)
- Modified: `go.mod` (potentially bubbles spinner if not already present)

---

## What This Doesn't Do (Intentionally)

- No JSON output, no `--format` flag, no scriptable CLI additions (gh/jq already do this)
- No inline comments or review threads (GitHub already does this)
- No agent communication protocol (out of scope for a viewer)
- No plugin system

Every improvement here makes the **live viewing experience** better. That's the thing only Scry does.
