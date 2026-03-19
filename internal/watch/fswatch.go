package watch

import (
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Debouncer collapses rapid events into a single callback after a quiet period.
type Debouncer struct {
	interval time.Duration
	callback func()
	timer    *time.Timer
	mu       sync.Mutex
	stopped  bool
}

// NewDebouncer creates a debouncer that fires callback after interval of quiet.
func NewDebouncer(interval time.Duration, callback func()) *Debouncer {
	return &Debouncer{
		interval: interval,
		callback: callback,
	}
}

// Trigger resets the debounce timer. The callback fires after interval of quiet.
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.interval, d.callback)
}

// Stop cancels any pending callback.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = true
	if d.timer != nil {
		d.timer.Stop()
	}
}

// FSWatcher wraps fsnotify to watch a repo root for file changes.
// It debounces events and calls onRefresh when changes settle.
type FSWatcher struct {
	watcher   *fsnotify.Watcher
	debouncer *Debouncer
	done      chan struct{}
}

// NewFSWatcher creates an fsnotify watcher on the repo root and git directory.
// gitDir should be the resolved per-worktree git dir (from RepoContext.GitDir).
// Returns nil if the watcher cannot be initialized (caller falls back to polling).
func NewFSWatcher(repoRoot, gitDir string, onRefresh func()) *FSWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil
	}

	// Watch repo root (best-effort acceleration for top-level changes).
	if err := watcher.Add(repoRoot); err != nil {
		watcher.Close()
		return nil
	}

	// Watch the resolved git directory for ref changes (commits, checkouts, fetches).
	// For linked worktrees this differs from repoRoot/.git.
	if gitDir != "" {
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			watcher.Add(gitDir) // best-effort, ignore error
		}
	}

	debouncer := NewDebouncer(150*time.Millisecond, onRefresh)
	fw := &FSWatcher{
		watcher:   watcher,
		debouncer: debouncer,
		done:      make(chan struct{}),
	}

	go fw.loop()
	return fw
}

func (fw *FSWatcher) loop() {
	defer close(fw.done)
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			// Trigger on any write/create/remove/rename event.
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) ||
				event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				fw.debouncer.Trigger()
			}
		case _, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// Swallow errors — polling fallback ensures correctness.
		}
	}
}

// Close shuts down the watcher and debouncer. Safe to call multiple times.
func (fw *FSWatcher) Close() {
	fw.debouncer.Stop()
	fw.watcher.Close()
	<-fw.done
}
