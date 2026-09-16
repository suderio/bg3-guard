package restore

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"bg3-guard/internal/backup"
	"bg3-guard/internal/config"
	"bg3-guard/internal/process"
)

var timestampRegex = regexp.MustCompile(`-(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})$`)

// FormatTimestamp converts a 12-digit YYYYMMDDHHMM suffix to 'YYYY-MM-DD at HH:MM'
func FormatTimestamp(folderName string) string {
	matches := timestampRegex.FindStringSubmatch(folderName)
	if len(matches) == 6 {
		return fmt.Sprintf("%s-%s-%s at %s:%s", matches[1], matches[2], matches[3], matches[4], matches[5])
	}
	return folderName
}

// RestoreOptions holds the configuration and chosen backup for a restore operation.
type RestoreOptions struct {
	BackupEntry backup.BackupEntry
	Force       bool // if true, proceed even if game is running
}

// RestoreResult provides detailed feedback about the restore operation.
type RestoreResult struct {
	Success          bool
	TargetSource     string
	SafetyBackupPath string
	Warning          string
	Message          string
}

// CheckRestoreSafety checks if the game is running before restoring.
func CheckRestoreSafety() (bool, string) {
	running, name, err := process.IsGameRunning()
	if err == nil && running {
		return false, fmt.Sprintf("Baldur's Gate 3 is currently running (%s)! Restoring while the game is active can cause save corruption or game crashes. It is strongly recommended to exit the game first.", name)
	}
	return true, ""
}

// PerformSafeRestore executes transactional restore with safety copy and rollback on error.
func PerformSafeRestore(cfg *config.Config, entry backup.BackupEntry, force bool, logFn func(string)) (*RestoreResult, error) {
	result := &RestoreResult{
		TargetSource: filepath.Join(cfg.SourcePath, entry.GroupName),
	}

	// 1. Process safety warning
	safe, warningMsg := CheckRestoreSafety()
	if !safe {
		result.Warning = warningMsg
		if !force {
			return result, fmt.Errorf("safety check failed: %s", warningMsg)
		}
	}

	if _, err := os.Stat(entry.FullName); os.IsNotExist(err) {
		return nil, fmt.Errorf("selected backup does not exist: %s", entry.FullName)
	}

	if err := os.MkdirAll(cfg.SourcePath, 0755); err != nil {
		return nil, fmt.Errorf("source folder inaccessible: %w", err)
	}

	// 2. User Requirement: Always make a backup copy of the current save in destination
	targetDir := filepath.Join(cfg.SourcePath, entry.GroupName)
	if _, err := os.Stat(targetDir); err == nil {
		timestamp := time.Now().Format("200601021504")
		safetyBackupName := fmt.Sprintf("%s-PreRestore-%s", entry.GroupName, timestamp)
		safetyBackupPath := filepath.Join(cfg.DestinationPath, safetyBackupName)

		if logFn != nil {
			logFn(fmt.Sprintf("Creating safety backup of current save: %s", safetyBackupName))
		}

		if err := backup.CopyDirectory(targetDir, safetyBackupPath); err != nil {
			return nil, fmt.Errorf("failed to create pre-restore safety copy: %w", err)
		}
		result.SafetyBackupPath = safetyBackupPath
	}

	// 3. Transactional pattern: Rename current save before copying
	tempSafeDir := targetDir + "_safe_restoration"
	hasOriginal := false

	if _, err := os.Stat(targetDir); err == nil {
		hasOriginal = true
		// Remove stale temporary folder if it was left behind
		_ = os.RemoveAll(tempSafeDir)

		if err := os.Rename(targetDir, tempSafeDir); err != nil {
			return nil, fmt.Errorf("failed to rename current save for safety: %w", err)
		}
		if logFn != nil {
			logFn("Original save temporarily renamed for transactional safety.")
		}
	}

	// 4. Copy selected backup to targetSource
	if logFn != nil {
		logFn(fmt.Sprintf("Copying backup '%s' to '%s'", entry.Name, targetDir))
	}

	if err := backup.CopyDirectory(entry.FullName, targetDir); err != nil {
		// Rollback
		if logFn != nil {
			logFn(fmt.Sprintf("Restore error: %v. Rolling back original save...", err))
		}
		_ = os.RemoveAll(targetDir) // remove partial copy
		if hasOriginal {
			if rbErr := os.Rename(tempSafeDir, targetDir); rbErr != nil {
				return nil, fmt.Errorf("restore failed (%v) and critical rollback failure: %w. Check %s manually", err, rbErr, tempSafeDir)
			}
			if logFn != nil {
				logFn("Original save successfully rolled back. No data was lost.")
			}
		}
		return nil, fmt.Errorf("restoration failed: %w", err)
	}

	// 5. Clean up temporary transactional folder on success
	if hasOriginal {
		_ = os.RemoveAll(tempSafeDir)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Successfully restored %s (%s)", entry.GroupName, FormatTimestamp(entry.Name))
	if logFn != nil {
		logFn(result.Message)
	}

	return result, nil
}

// GetAvailableBackupsForGroup returns all backups for a group sorted descending.
func GetAvailableBackupsForGroup(destDir, groupName string) ([]backup.BackupEntry, error) {
	groups, err := backup.GetBackupsByGroup(destDir)
	if err != nil {
		return nil, err
	}
	items := groups[groupName]
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name > items[j].Name
	})
	return items, nil
}
