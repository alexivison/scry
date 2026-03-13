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

**Files to modify**: `internal/model/worktree.go`, `internal/gitexec/worktree.go`, `internal/ui/panes/dashboard.go`, `internal/ui/dashboard.go`

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

## Implementation Priority

1. **Freshness indicators** (1a, 1b, 1c) — highest impact, directly addresses "what did the agent just change"
2. **Scroll preservation** (2a) — quality of life, prevents jarring viewport jumps
3. **File flags** (4a, 4b) — lightweight interaction model
4. **Dashboard enhancements** (3a, 3b) — improves multi-agent visibility
5. **Diff-aware cache** (2b) — performance optimization for large diffs
6. **Dashboard preview** (3c) — nice to have
7. **Export flagged** (4c) — the bridge to external tools

## What This Doesn't Do (Intentionally)

- No JSON output, no `--format` flag, no scriptable CLI additions (gh/jq already do this)
- No inline comments or review threads (GitHub already does this)
- No agent communication protocol (out of scope for a viewer)
- No plugin system

Every improvement here makes the **live viewing experience** better. That's the thing only Scry does.
