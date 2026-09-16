//go:build windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// IsFileLocked tests whether a file on Windows is locked or being written to.
func IsFileLocked(filePath string) (bool, error) {
	pathPtr, err := windows.UTF16PtrFromString(filePath)
	if err != nil {
		return false, err
	}

	// First: Try opening with read access and read sharing.
	// If this fails with ERROR_SHARING_VIOLATION, the file is open with exclusive access (e.g. active write lock).
	h, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		var errno windows.Errno
		if errors.As(err, &errno) {
			if errno == windows.ERROR_SHARING_VIOLATION ||
				errno == windows.ERROR_LOCK_VIOLATION {
				return true, nil
			}
			if errno == windows.ERROR_ACCESS_DENIED {
				// Access denied may also occur if file is exclusively locked
				return true, nil
			}
		}
		if errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		return true, nil
	}
	windows.CloseHandle(h)

	// Second: Try opening with write access and read sharing.
	// If BG3 has the file open for writing, it does not share write access, which will trigger ERROR_SHARING_VIOLATION.
	hWrite, errWrite := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if errWrite != nil {
		var errno windows.Errno
		if errors.As(errWrite, &errno) {
			if errno == windows.ERROR_SHARING_VIOLATION ||
				errno == windows.ERROR_LOCK_VIOLATION {
				return true, nil
			}
		}
		// If read-only filesystem or other permission error, don't necessarily treat as locked if read succeeded
	} else {
		windows.CloseHandle(hWrite)
	}

	return false, nil
}
