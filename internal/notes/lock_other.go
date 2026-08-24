//go:build !darwin && !linux

package notes

import "os"

func acquireLock(string) (*os.File, error) {
	return nil, noteError("unsupported_platform", "note storage locking is unsupported on this platform")
}
