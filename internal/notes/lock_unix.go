//go:build darwin || linux

package notes

import (
	"errors"
	"os"
	"syscall"
)

func acquireLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, noteError("storage", err.Error())
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, noteError("storage", err.Error())
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, noteError("busy", "note ledger is busy")
		}
		return nil, noteError("storage", err.Error())
	}
	return file, nil
}
