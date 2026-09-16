package filelock

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CheckFolderReadyForBackup checks whether a save directory is safe to copy.
// It verifies:
// 1. No temporary write files (like *.tmp) are present.
// 2. No files are currently locked by BG3 or other processes.
// 3. File modification times are older than minAge (default: 3s).
// 4. File sizes are stable over a short sample duration.
func CheckFolderReadyForBackup(dirPath string, minAge time.Duration, sampleDelay time.Duration) (bool, string, error) {
	var files []string
	var totalSize int64
	now := time.Now()

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// 1. Check for temporary write files
		lowerName := strings.ToLower(d.Name())
		if strings.HasSuffix(lowerName, ".tmp") || strings.HasSuffix(lowerName, ".crdownload") {
			return fmt.Errorf("temporary save file found: %s", d.Name())
		}

		// 2. Check if file is modified very recently
		age := now.Sub(info.ModTime())
		if age < minAge {
			return fmt.Errorf("file %s was modified %v ago (less than required %v stability window)", d.Name(), age.Round(time.Millisecond), minAge)
		}

		// 3. Check file locks
		locked, err := IsFileLocked(path)
		if err != nil {
			return fmt.Errorf("error checking lock on %s: %w", d.Name(), err)
		}
		if locked {
			return fmt.Errorf("file %s is currently locked/in use by another process", d.Name())
		}

		files = append(files, path)
		totalSize += info.Size()
		return nil
	})

	if err != nil {
		return false, err.Error(), nil
	}

	if len(files) == 0 {
		return true, "empty directory", nil
	}

	// 4. Stability sample check if sampleDelay > 0
	if sampleDelay > 0 {
		time.Sleep(sampleDelay)
		var recheckSize int64
		for _, f := range files {
			info, err := os.Stat(f)
			if err != nil {
				return false, fmt.Sprintf("file %s stat failed during stability check: %v", filepath.Base(f), err), nil
			}
			recheckSize += info.Size()
		}
		if recheckSize != totalSize {
			return false, fmt.Sprintf("save directory size changed from %d to %d bytes during sampling", totalSize, recheckSize), nil
		}
	}

	return true, "ready", nil
}
