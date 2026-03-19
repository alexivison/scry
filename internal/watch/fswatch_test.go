package watch

import (
	"testing"
	"time"
)

func TestDebouncer_CollapsesRapidEvents(t *testing.T) {
	t.Parallel()

	out := make(chan struct{}, 10)
	d := NewDebouncer(150*time.Millisecond, func() { out <- struct{}{} })
	defer d.Stop()

	// Fire 5 rapid events — should collapse into 1.
	for i := 0; i < 5; i++ {
		d.Trigger()
	}

	select {
	case <-out:
		// good — got the collapsed notification
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected debounced callback within 500ms")
	}

	// Should not fire again.
	select {
	case <-out:
		t.Error("debouncer fired more than once for rapid events")
	case <-time.After(300 * time.Millisecond):
		// good — no extra fire
	}
}

func TestDebouncer_SeparateEventsFireSeparately(t *testing.T) {
	t.Parallel()

	count := 0
	out := make(chan struct{}, 10)
	d := NewDebouncer(50*time.Millisecond, func() {
		count++
		out <- struct{}{}
	})
	defer d.Stop()

	// First event.
	d.Trigger()
	select {
	case <-out:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("first callback not received")
	}

	// Wait for debounce window to pass, then second event.
	time.Sleep(100 * time.Millisecond)
	d.Trigger()
	select {
	case <-out:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("second callback not received")
	}

	if count != 2 {
		t.Errorf("callback count = %d, want 2", count)
	}
}

func TestDebouncer_StopPreventsCallback(t *testing.T) {
	t.Parallel()

	out := make(chan struct{}, 10)
	d := NewDebouncer(100*time.Millisecond, func() { out <- struct{}{} })

	d.Trigger()
	d.Stop()

	select {
	case <-out:
		t.Error("callback should not fire after Stop")
	case <-time.After(250 * time.Millisecond):
		// good
	}
}

func TestNewFSWatcher_FallbackOnError(t *testing.T) {
	t.Parallel()

	// Watching a nonexistent path should return nil (fallback to polling).
	w := NewFSWatcher("/nonexistent/path/that/does/not/exist", "", func() {})
	if w != nil {
		w.Close()
		t.Error("expected nil watcher for nonexistent path")
	}
}
