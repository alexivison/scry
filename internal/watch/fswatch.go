package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	ready     chan struct{} // closed when the async directory walk completes
}

// NewFSWatcher creates an fsnotify watcher on the repo root and git directory.
// gitDir should be the resolved per-worktree git dir (from RepoContext.GitDir).
// Returns nil if the watcher cannot be initialized (caller falls back to polling).
// The recursive directory walk runs asynchronously to avoid blocking startup.
func NewFSWatcher(repoRoot, gitDir string, onRefresh func()) *FSWatcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil
	}

	// Watch root immediately so top-level changes are caught right away.
	if err := watcher.Add(repoRoot); err != nil {
		watcher.Close()
		return nil
	}

	// Watch the resolved git directory for ref changes (commits, checkouts, fetches).
	if gitDir != "" {
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			watcher.Add(gitDir) // best-effort
		}
	}

	debouncer := NewDebouncer(150*time.Millisecond, onRefresh)
	fw := &FSWatcher{
		watcher:   watcher,
		debouncer: debouncer,
		done:      make(chan struct{}),
		ready:     make(chan struct{}),
	}

	// Walk subdirectories in the background so the TUI starts instantly.
	// After the walk, trigger a refresh to catch any events missed during the window.
	go func() {
		addSubdirsRecursive(watcher, repoRoot)
		close(fw.ready)
		fw.debouncer.Trigger()
	}()

	go fw.loop()
	return fw
}

// skipDir returns true for directories that should not be watched.
func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".next", "__pycache__", ".gradle":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// addSubdirsRecursive walks root and adds every subdirectory to the watcher,
// skipping hidden dirs and common heavy directories. Root itself is assumed
// to be already watched.
func addSubdirsRecursive(w *fsnotify.Watcher, root string) {
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries
		}
		if !d.IsDir() || path == root {
			return nil
		}
		if skipDir(d.Name()) {
			return filepath.SkipDir
		}
		w.Add(path) // best-effort, ignore per-dir errors
		return nil
	})
}

func (fw *FSWatcher) loop() {
	defer close(fw.done)
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			// Auto-watch newly created directories so we pick up future changes.
			if event.Has(fsnotify.Create) {
				if info, err := os.Lstat(event.Name); err == nil && info.IsDir() && !skipDir(filepath.Base(event.Name)) {
					fw.watcher.Add(event.Name)
					addSubdirsRecursive(fw.watcher, event.Name)
				}
			}
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
// Waits for both the background walk and event loop goroutines to finish.
func (fw *FSWatcher) Close() {
	fw.debouncer.Stop()
	fw.watcher.Close()
	<-fw.ready
	<-fw.done
}
