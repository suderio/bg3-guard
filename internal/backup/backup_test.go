package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/suderio/bg3-guard/internal/config"
)

func TestHasFolderChanged(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_backup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	src := filepath.Join(tempDir, "src")
	bak := filepath.Join(tempDir, "bak")

	_ = os.MkdirAll(src, 0755)
	_ = os.MkdirAll(bak, 0755)

	f1 := filepath.Join(src, "save.lsv")
	_ = os.WriteFile(f1, []byte("version1"), 0644)

	// bak doesn't have it yet -> changed
	changed, err := HasFolderChanged(src, bak)
	if err != nil {
		t.Fatalf("HasFolderChanged error: %v", err)
	}
	if !changed {
		t.Errorf("Expected changed=true when files differ")
	}

	// Copy to bak
	if err := CopyDirectory(src, bak); err != nil {
		t.Fatalf("CopyDirectory failed: %v", err)
	}

	// Now identical -> changed should be false
	changed, err = HasFolderChanged(src, bak)
	if err != nil {
		t.Fatalf("HasFolderChanged error: %v", err)
	}
	if changed {
		t.Errorf("Expected changed=false when files identical")
	}

	// Modify src file
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(f1, []byte("version2 modified"), 0644)

	changed, err = HasFolderChanged(src, bak)
	if err != nil {
		t.Fatalf("HasFolderChanged error: %v", err)
	}
	if !changed {
		t.Errorf("Expected changed=true after modifying src file")
	}
}

func TestRetentionCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_retention_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	group := "Campaign1_HonourMode"
	// Create 5 backups
	for i := 1; i <= 5; i++ {
		name := filepath.Join(tempDir, group+fmt.Sprintf("-20260901000%d", i))
		_ = os.MkdirAll(name, 0755)
		_ = os.WriteFile(filepath.Join(name, "save.lsv"), []byte("data"), 0644)
	}

	backups, err := GetBackups(tempDir)
	if err != nil {
		t.Fatalf("GetBackups failed: %v", err)
	}
	if len(backups) != 5 {
		t.Fatalf("Expected 5 backups, got %d", len(backups))
	}

	// Limit to 2
	deleted, err := CleanupRetention(tempDir, 2, nil)
	if err != nil {
		t.Fatalf("CleanupRetention failed: %v", err)
	}
	if deleted != 3 {
		t.Errorf("Expected 3 deleted, got %d", deleted)
	}

	remaining, _ := GetBackups(tempDir)
	if len(remaining) != 2 {
		t.Errorf("Expected 2 remaining, got %d", len(remaining))
	}

	// Verify the 2 remaining are the newest ones (0004 and 0005)
	for _, b := range remaining {
		if b.Name != group+"-202609010004" && b.Name != group+"-202609010005" {
			t.Errorf("Unexpected surviving backup: %s", b.Name)
		}
	}
}

func TestRunBackupCycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_cycle_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "saves")
	dstDir := filepath.Join(tempDir, "backups")
	_ = os.MkdirAll(srcDir, 0755)

	campaign := filepath.Join(srcDir, "Hero_HonourMode")
	_ = os.MkdirAll(campaign, 0755)
	_ = os.WriteFile(filepath.Join(campaign, "HonourMode.lsv"), []byte("sample save data"), 0644)

	// Allow modification time to age past the 2s threshold for the test
	pastTime := time.Now().Add(-5 * time.Second)
	_ = os.Chtimes(filepath.Join(campaign, "HonourMode.lsv"), pastTime, pastTime)

	cfg := &config.Config{
		SourcePath:      srcDir,
		DestinationPath: dstDir,
		RetentionLimit:  5,
		SaveFilter:      "*HonourMode*",
	}

	made, err := RunBackupCycle(cfg, nil)
	if err != nil {
		t.Fatalf("RunBackupCycle failed: %v", err)
	}
	if made != 1 {
		t.Errorf("Expected 1 backup made, got %d", made)
	}

	// Running immediately again without changes should make 0 backups
	madeSecond, err := RunBackupCycle(cfg, nil)
	if err != nil {
		t.Fatalf("RunBackupCycle second run failed: %v", err)
	}
	if madeSecond != 0 {
		t.Errorf("Expected 0 backups made on second run without changes, got %d", madeSecond)
	}
}

func TestFilterSaveFolders(t *testing.T) {
	folders := []string{
		"Tav_HonourMode-123",
		"Durge_Custom-456",
		"Gale_HonourMode-789",
		"Tactician_Campaign",
	}

	// 1. Default HonourMode filter
	res := FilterSaveFolders(folders, "*HonourMode*")
	if len(res) != 2 {
		t.Fatalf("Expected 2 matches for *HonourMode*, got %d: %v", len(res), res)
	}

	// 2. Wildcard matches all
	all := FilterSaveFolders(folders, "*")
	if len(all) != 4 {
		t.Fatalf("Expected 4 matches for '*', got %d", len(all))
	}

	// 3. Substring matching without asterisks
	custom := FilterSaveFolders(folders, "custom")
	if len(custom) != 1 || custom[0] != "Durge_Custom-456" {
		t.Fatalf("Expected 1 match for 'custom', got %v", custom)
	}

	// 4. Non-matching pattern
	none := FilterSaveFolders(folders, "*NonExistent*")
	if len(none) != 0 {
		t.Fatalf("Expected 0 matches for non-existent, got %d", len(none))
	}
}

