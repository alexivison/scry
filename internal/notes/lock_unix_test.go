//go:build darwin || linux

package notes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLockCreatesOwnerOnlyPersistentFile(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t)
	if err := os.MkdirAll(filepath.Dir(store.lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireLock(store.lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("lock permissions = %o, want 600", info.Mode().Perm())
	}
}
