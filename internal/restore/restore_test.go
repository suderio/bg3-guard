package restore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suderio/bg3-guard/internal/backup"
	"github.com/suderio/bg3-guard/internal/config"
)

func TestFormatTimestamp(t *testing.T) {
	name := "Camp_HonourMode-202609142324"
	formatted := FormatTimestamp(name)
	expected := "2026-09-14 at 23:24"
	if formatted != expected {
		t.Errorf("Expected %q, got %q", expected, formatted)
	}
}

func TestPerformSafeRestore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_restore_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "saves")
	dstDir := filepath.Join(tempDir, "backups")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.MkdirAll(dstDir, 0755)

	groupName := "HonourParty"
	currentSave := filepath.Join(srcDir, groupName)
	_ = os.MkdirAll(currentSave, 0755)
	_ = os.WriteFile(filepath.Join(currentSave, "HonourMode.lsv"), []byte("active save version 2"), 0644)

	backupFolder := filepath.Join(dstDir, groupName+"-202609011200")
	_ = os.MkdirAll(backupFolder, 0755)
	_ = os.WriteFile(filepath.Join(backupFolder, "HonourMode.lsv"), []byte("backup save version 1"), 0644)

	cfg := &config.Config{
		SourcePath:      srcDir,
		DestinationPath: dstDir,
	}

	entry := backup.BackupEntry{
		Name:      groupName + "-202609011200",
		FullName:  backupFolder,
		GroupName: groupName,
		Timestamp: "202609011200",
	}

	res, err := PerformSafeRestore(cfg, entry, true, nil)
	if err != nil {
		t.Fatalf("PerformSafeRestore failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("Expected success=true")
	}

	// 1. Target save should now contain the backup's content ("backup save version 1")
	restoredContent, err := os.ReadFile(filepath.Join(currentSave, "HonourMode.lsv"))
	if err != nil {
		t.Fatalf("Failed to read restored save: %v", err)
	}
	if string(restoredContent) != "backup save version 1" {
		t.Errorf("Expected 'backup save version 1', got %q", string(restoredContent))
	}

	// 2. A PreRestore safety backup should exist in dstDir
	if res.SafetyBackupPath == "" {
		t.Errorf("Expected SafetyBackupPath to be set")
	} else {
		content, err := os.ReadFile(filepath.Join(res.SafetyBackupPath, "HonourMode.lsv"))
		if err != nil {
			t.Fatalf("Failed to read pre-restore safety file: %v", err)
		}
		if string(content) != "active save version 2" {
			t.Errorf("Expected pre-restore safety copy to have 'active save version 2', got %q", string(content))
		}
	}
}
