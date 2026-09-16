package filelock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLockUnlocked(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_lock_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "save.lsv")
	if err := os.WriteFile(testFile, []byte("save data content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	locked, err := IsFileLocked(testFile)
	if err != nil {
		t.Fatalf("IsFileLocked error: %v", err)
	}
	if locked {
		t.Errorf("Expected file to be unlocked, got locked")
	}

	// Stability check
	ready, reason, err := CheckFolderReadyForBackup(tempDir, 0, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("CheckFolderReadyForBackup error: %v", err)
	}
	if !ready {
		t.Errorf("Expected folder ready, got false: %s", reason)
	}
}

func TestFileLockRecentModification(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_lock_recent_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "save.lsv")
	if err := os.WriteFile(testFile, []byte("new data"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Require minAge of 5 seconds -> should fail as not ready
	ready, reason, err := CheckFolderReadyForBackup(tempDir, 5*time.Second, 0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if ready {
		t.Errorf("Expected not ready due to recent modification, got ready (%s)", reason)
	}
}
