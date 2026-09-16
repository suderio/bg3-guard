package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"bg3-guard/internal/config"
	"bg3-guard/internal/filelock"
)

// BackupRegex matches backup folders: FolderName-YYYYMMDDHHMM (12 numeric digits at the end)
var BackupRegex = regexp.MustCompile(`^(.+)-(\d{12})$`)

// BackupEntry represents an individual backup folder snapshot.
type BackupEntry struct {
	Name      string
	FullName  string
	GroupName string
	Timestamp string
	ModTime   time.Time
}

// GetBackups returns all valid backups found in the destination folder.
func GetBackups(destDir string) ([]BackupEntry, error) {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []BackupEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		matches := BackupRegex.FindStringSubmatch(entry.Name())
		if len(matches) == 3 {
			info, err := entry.Info()
			var modTime time.Time
			if err == nil {
				modTime = info.ModTime()
			}
			backups = append(backups, BackupEntry{
				Name:      entry.Name(),
				FullName:  filepath.Join(destDir, entry.Name()),
				GroupName: matches[1],
				Timestamp: matches[2],
				ModTime:   modTime,
			})
		}
	}
	return backups, nil
}

// GetBackupsByGroup returns backups grouped by their original folder name and sorted newest first.
func GetBackupsByGroup(destDir string) (map[string][]BackupEntry, error) {
	all, err := GetBackups(destDir)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]BackupEntry)
	for _, b := range all {
		groups[b.GroupName] = append(groups[b.GroupName], b)
	}

	for group := range groups {
		sort.Slice(groups[group], func(i, j int) bool {
			return groups[group][i].Name > groups[group][j].Name
		})
	}

	return groups, nil
}

// HasFolderChanged compares the source folder files with the previous backup folder.
// Returns true if any file was changed, added or removed.
func HasFolderChanged(sourceFolder, backupFolder string) (bool, error) {
	if _, err := os.Stat(sourceFolder); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := os.Stat(backupFolder); os.IsNotExist(err) {
		return true, nil
	}

	sourceFiles := make(map[string]os.FileInfo)
	err := filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(sourceFolder, path)
		sourceFiles[filepath.ToSlash(rel)] = info
		return nil
	})
	if err != nil {
		return true, err
	}

	backupFiles := make(map[string]os.FileInfo)
	err = filepath.Walk(backupFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(backupFolder, path)
		backupFiles[filepath.ToSlash(rel)] = info
		return nil
	})
	if err != nil {
		return true, err
	}

	if len(sourceFiles) != len(backupFiles) {
		return true, nil
	}

	for relPath, srcInfo := range sourceFiles {
		bakInfo, exists := backupFiles[relPath]
		if !exists {
			return true, nil
		}
		if srcInfo.Size() != bakInfo.Size() {
			return true, nil
		}
		if !srcInfo.ModTime().Equal(bakInfo.ModTime()) {
			return true, nil
		}
	}

	return false, nil
}

// CopyDirectory recursively copies a directory from src to dst.
func CopyDirectory(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile copies a single file from src to dst preserving mtime and permissions.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	srcInfo, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	_ = os.Chtimes(dst, time.Now(), srcInfo.ModTime())
	return nil
}

// CleanupRetention removes backups exceeding the retention limit per group.
func CleanupRetention(destDir string, limit int, logFn func(string)) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	groups, err := GetBackupsByGroup(destDir)
	if err != nil {
		return 0, err
	}

	deletedCount := 0
	for groupName, items := range groups {
		if len(items) > limit {
			excess := items[limit:]
			for _, item := range excess {
				if logFn != nil {
					logFn(fmt.Sprintf("Retention policy: removing old backup %s (group %s)", item.Name, groupName))
				}
				if err := os.RemoveAll(item.FullName); err != nil {
					if logFn != nil {
						logFn(fmt.Sprintf("Warning: failed to remove %s: %v", item.FullName, err))
					}
				} else {
					deletedCount++
				}
			}
		}
	}
	return deletedCount, nil
}

// RunBackupCycle executes a single backup check across all matching saves in cfg.SourcePath.
// Returns the number of backups successfully created.
func RunBackupCycle(cfg *config.Config, logFn func(string)) (int, error) {
	if _, err := os.Stat(cfg.SourcePath); os.IsNotExist(err) {
		return 0, fmt.Errorf("source folder not found: %s", cfg.SourcePath)
	}

	if err := os.MkdirAll(cfg.DestinationPath, 0755); err != nil {
		return 0, fmt.Errorf("could not create destination folder %s: %w", cfg.DestinationPath, err)
	}

	entries, err := os.ReadDir(cfg.SourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read source directory: %w", err)
	}

	// Filter directories
	var dirNames []string
	for _, e := range entries {
		if e.IsDir() {
			dirNames = append(dirNames, e.Name())
		}
	}
	targetFolders := FilterSaveFolders(dirNames, cfg.SaveFilter)

	if len(targetFolders) == 0 {
		if logFn != nil {
			logFn(fmt.Sprintf("No matching folders found for filter '%s' in %s", cfg.SaveFilter, cfg.SourcePath))
		}
		return 0, nil
	}

	backupsMade := 0
	timestamp := time.Now().Format("200601021504")

	for _, folderName := range targetFolders {
		folderPath := filepath.Join(cfg.SourcePath, folderName)

		// 1. Find latest backup for this folder
		allBackups, err := GetBackups(cfg.DestinationPath)
		if err != nil {
			return backupsMade, err
		}

		var latestBackup *BackupEntry
		var groupBackups []BackupEntry
		for _, b := range allBackups {
			if b.GroupName == folderName {
				groupBackups = append(groupBackups, b)
			}
		}
		if len(groupBackups) > 0 {
			sort.Slice(groupBackups, func(i, j int) bool {
				return groupBackups[i].Name > groupBackups[j].Name
			})
			latestBackup = &groupBackups[0]
		}

		// 2. Check if changed
		shouldBackup := true
		if latestBackup != nil {
			changed, err := HasFolderChanged(folderPath, latestBackup.FullName)
			if err != nil {
				if logFn != nil {
					logFn(fmt.Sprintf("Warning checking changes on %s: %v", folderName, err))
				}
			} else if !changed {
				shouldBackup = false
				if logFn != nil {
					logFn(fmt.Sprintf("Save files have not changed: %s", folderName))
				}
			}
		}

		if !shouldBackup {
			continue
		}

		// 3. File corruption protection: Verify file stability and lack of locks
		ready, reason, err := filelock.CheckFolderReadyForBackup(folderPath, 2*time.Second, 200*time.Millisecond)
		if err != nil || !ready {
			if logFn != nil {
				logFn(fmt.Sprintf("[DELAY] Save '%s' is in use or being written (%s). Delaying backup to prevent corruption.", folderName, reason))
			}
			continue
		}

		// 4. Perform backup
		destName := fmt.Sprintf("%s-%s", folderName, timestamp)
		destPath := filepath.Join(cfg.DestinationPath, destName)

		if logFn != nil {
			logFn(fmt.Sprintf("Creating backup: %s -> %s", folderName, destName))
		}

		if err := CopyDirectory(folderPath, destPath); err != nil {
			if logFn != nil {
				logFn(fmt.Sprintf("Error copying %s: %v", folderName, err))
			}
			// Clean up incomplete destination
			_ = os.RemoveAll(destPath)
			continue
		}

		backupsMade++
		if logFn != nil {
			logFn(fmt.Sprintf("Successfully backed up: %s", destName))
		}
	}

	if backupsMade > 0 {
		_, _ = CleanupRetention(cfg.DestinationPath, cfg.RetentionLimit, logFn)
	}

	return backupsMade, nil
}

// MatchesSaveFilter checks whether a save folder name matches a glob or substring pattern.
func MatchesSaveFilter(name, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	matched, _ := filepath.Match(pattern, name)
	if matched {
		return true
	}
	clean := strings.Trim(pattern, "*")
	if clean != "" && strings.Contains(strings.ToLower(name), strings.ToLower(clean)) {
		return true
	}
	return false
}

// FilterSaveFolders filters a list of folder names according to pattern.
func FilterSaveFolders(folders []string, pattern string) []string {
	if pattern == "" || pattern == "*" {
		return folders
	}
	var res []string
	for _, f := range folders {
		if MatchesSaveFilter(f, pattern) {
			res = append(res, f)
		}
	}
	return res
}
