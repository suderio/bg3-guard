package recover

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suderio/bg3-guard/internal/backup"
	"github.com/suderio/bg3-guard/internal/config"
	"github.com/suderio/bg3-guard/internal/process"
)

// CloudSyncWarningMessage is the critical warning regarding Steam and Larian Cloud Saves.
const CloudSyncWarningMessage = `[CRITICAL WARNING - CLOUD SYNC]
Steam Cloud and Larian Cross-Save synchronization MUST be disabled before launching the game!
If cloud sync is active, the game or launcher may detect timestamp/checksum differences and
overwrite your recovered save with the dead/custom state from the cloud.
Steps to safely play:
1. Disable Steam Cloud for Baldur's Gate 3 (Steam -> BG3 -> Properties -> General -> Steam Cloud OFF).
2. Disable Cross-Save in the BG3 in-game options.
3. Launch BG3 and verify your save is back in Honour Mode.
4. Make a new save in-game to establish a fresh timestamp.
5. Re-enable Cloud Sync afterwards if desired.`

// RecoveryResult stores the outcome of an Honour Mode recovery attempt.
type RecoveryResult struct {
	Success             bool
	SafeguardBackupPath string
	SessionID           string
	GameWarning         string
	CloudSyncWarning    string
	Message             string
	ProfileUpdated      bool
	PackageUpdated      bool
}

// CheckRecoverySafety checks for game processes and provides cloud sync warnings.
func CheckRecoverySafety() (gameRunning bool, gameProc string, cloudWarning string) {
	running, proc, _ := process.IsGameRunning()
	return running, proc, CloudSyncWarningMessage
}

// RecoverSave executes the full Honour Mode reactivation workflow.
func RecoverSave(cfg *config.Config, saveFolderName string, force bool, logFn func(string)) (*RecoveryResult, error) {
	result := &RecoveryResult{
		CloudSyncWarning: CloudSyncWarningMessage,
	}

	// 1. Process safety check
	running, proc, _ := CheckRecoverySafety()
	if running {
		result.GameWarning = fmt.Sprintf("Baldur's Gate 3 is currently running (%s)! Modifying save files while the game is running will cause corruption.", proc)
		if !force {
			return result, fmt.Errorf("%s", result.GameWarning)
		}
	}

	saveDir := filepath.Join(cfg.SourcePath, saveFolderName)
	if _, err := os.Stat(saveDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("save folder does not exist: %s", saveDir)
	}

	// Find the .lsv file in saveDir
	entries, err := os.ReadDir(saveDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read save directory: %w", err)
	}

	var lsvFile string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".lsv") {
			lsvFile = filepath.Join(saveDir, e.Name())
			break
		}
	}
	if lsvFile == "" {
		return nil, fmt.Errorf("no .lsv save file found inside %s", saveDir)
	}

	// 2. User Requirement: Automatic safeguard backup (Custom-YYYYMMDDHHMM) before modifying
	timestamp := time.Now().Format("200601021504")
	safeguardName := fmt.Sprintf("Custom-%s-%s", saveFolderName, timestamp)
	safeguardPath := filepath.Join(cfg.DestinationPath, safeguardName)

	if logFn != nil {
		logFn(fmt.Sprintf("Creating pre-recovery safeguard snapshot: %s", safeguardName))
	}
	if err := backup.CopyDirectory(saveDir, safeguardPath); err != nil {
		return nil, fmt.Errorf("safeguard backup failed: %w", err)
	}
	result.SafeguardBackupPath = safeguardPath

	// 3. Create a clean temporary working directory (free from dot-prefixed paths)
	// We use %LOCALAPPDATA%\bg3-guard\temp or system temp
	tempBase := filepath.Join(os.TempDir(), "bg3guard_work")
	_ = os.MkdirAll(tempBase, 0755)
	tempWorkDir, err := os.MkdirTemp(tempBase, "recover_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp working dir: %w", err)
	}
	defer os.RemoveAll(tempWorkDir)

	// 4. Extract .lsv package
	if logFn != nil {
		logFn("Unpacking .lsv container using divine...")
	}
	extractedDir := filepath.Join(tempWorkDir, "extracted")
	if err := ExtractPackage(lsvFile, extractedDir); err != nil {
		return nil, fmt.Errorf("divine extraction failed: %w", err)
	}

	// 5. Decompile meta.lsf to meta.lsx
	metaLsfPath := filepath.Join(extractedDir, "meta.lsf")
	if _, err := os.Stat(metaLsfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("meta.lsf not found inside extracted package")
	}

	metaLsxPath := filepath.Join(tempWorkDir, "meta.lsx")
	if logFn != nil {
		logFn("Decompiling meta.lsf to XML...")
	}
	if err := ConvertResource(metaLsfPath, metaLsxPath); err != nil {
		return nil, fmt.Errorf("failed to decompile meta.lsf: %w", err)
	}

	// 6. Patch meta.lsx
	if logFn != nil {
		logFn("Patching metadata: restoring Honour RuleSet GUID, Difficulty, and single-save flags...")
	}
	sessionID, err := PatchMetaLSX(metaLsxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to patch meta.lsx: %w", err)
	}
	result.SessionID = sessionID

	// 7. Recompile meta.lsx to meta.lsf
	if logFn != nil {
		logFn("Recompiling patched XML back to binary meta.lsf...")
	}
	if err := ConvertResource(metaLsxPath, metaLsfPath); err != nil {
		return nil, fmt.Errorf("failed to recompile meta.lsx: %w", err)
	}

	// 8. Repack .lsv
	if logFn != nil {
		logFn("Repacking .lsv package with LZ4HC compression...")
	}
	repackedLsv := filepath.Join(tempWorkDir, "repacked.lsv")
	if err := CreatePackage(extractedDir, repackedLsv); err != nil {
		return nil, fmt.Errorf("failed to repack .lsv: %w", err)
	}

	// Replace original .lsv with repacked .lsv
	if err := backup.CopyFile(repackedLsv, lsvFile); err != nil {
		return nil, fmt.Errorf("failed to write updated .lsv file: %w", err)
	}
	result.PackageUpdated = true

	// 9. Patch profile8.lsf
	profile8Path := cfg.GetProfile8Path()
	if _, err := os.Stat(profile8Path); err == nil {
		if logFn != nil {
			logFn(fmt.Sprintf("Cleaning deactivated session from profile8.lsf (%s)...", profile8Path))
		}
		// Create .backup copy of profile8.lsf
		_ = backup.CopyFile(profile8Path, profile8Path+".backup")

		profileLsxPath := filepath.Join(tempWorkDir, "profile8.lsx")
		if err := ConvertResource(profile8Path, profileLsxPath); err == nil {
			modified, _ := RemoveSessionFromProfileLSX(profileLsxPath, sessionID)
			if modified {
				if err := ConvertResource(profileLsxPath, profile8Path); err == nil {
					result.ProfileUpdated = true
					if logFn != nil {
						logFn("profile8.lsf successfully updated: session removed from DisabledSingleSaveSessions.")
					}
				}
			}
		}
	}

	result.Success = true
	result.Message = fmt.Sprintf("Honour Mode successfully restored for '%s'!", saveFolderName)
	if logFn != nil {
		logFn(result.Message)
		logFn(CloudSyncWarningMessage)
	}

	return result, nil
}
