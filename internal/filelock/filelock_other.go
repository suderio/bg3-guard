//go:build !windows

package filelock

import (
	"os"
	"syscall"
)

// IsFileLocked tests whether a file is locked via POSIX flock.
func IsFileLocked(filePath string) (bool, error) {
	file, err := os.OpenFile(filePath, os.O_RDWR, 0)
	if err != nil {
		if os.IsPermission(err) {
			return true, nil
		}
		if os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	defer file.Close()

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return true, nil
		}
	} else {
		// Unlock if lock succeeded
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	}

	return false, nil
}
