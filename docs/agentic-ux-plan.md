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

1. **CLI simplification** (5a, 5b, 5c) — zero-config defaults make the tool approachable, should ship before adding more features
2. **Freshness indicators** (1a, 1b, 1c) — highest impact, directly addresses "what did the agent just change"
3. **Scroll preservation** (2a) — quality of life, prevents jarring viewport jumps
4. **File flags** (4a, 4b) — lightweight interaction model
5. **Dashboard enhancements** (3a, 3b) — improves multi-agent visibility
6. **Config file** (5d) — move power-user knobs out of flags
7. **Delete worktree** (3d) — natural cleanup action for finished agent worktrees
8. **Diff-aware cache** (2b) — performance optimization for large diffs
9. **Event-driven refresh** (2c) — near-instant UI updates via fsnotify, polling fallback
10. **Dashboard preview** (3c) — nice to have
11. **Export flagged** (4c) — the bridge to external tools

## What This Doesn't Do (Intentionally)

- No JSON output, no `--format` flag, no scriptable CLI additions (gh/jq already do this)
- No inline comments or review threads (GitHub already does this)
- No agent communication protocol (out of scope for a viewer)
- No plugin system

Every improvement here makes the **live viewing experience** better. That's the thing only Scry does.
