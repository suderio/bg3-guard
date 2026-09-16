package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg3guard_config_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "test_config.yaml")

	// 1. Initial load should create default config file
	cfg, path, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if path != configPath {
		t.Fatalf("Expected path %s, got %s", configPath, path)
	}
	if cfg.IntervalMinutes != DefaultIntervalMinutes {
		t.Errorf("Expected IntervalMinutes %d, got %d", DefaultIntervalMinutes, cfg.IntervalMinutes)
	}
	if cfg.RetentionLimit != DefaultRetentionLimit {
		t.Errorf("Expected RetentionLimit %d, got %d", DefaultRetentionLimit, cfg.RetentionLimit)
	}
	if cfg.SaveFilter != DefaultSaveFilter {
		t.Errorf("Expected SaveFilter %s, got %s", DefaultSaveFilter, cfg.SaveFilter)
	}

	// 2. Modify and save
	cfg.IntervalMinutes = 15
	cfg.RetentionLimit = 5
	cfg.SaveFilter = "*Test*"
	if err := SaveConfig(cfg, configPath); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// 3. Reload and verify changes
	reloadedCfg, _, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if reloadedCfg.IntervalMinutes != 15 {
		t.Errorf("Expected IntervalMinutes 15, got %d", reloadedCfg.IntervalMinutes)
	}
	if reloadedCfg.RetentionLimit != 5 {
		t.Errorf("Expected RetentionLimit 5, got %d", reloadedCfg.RetentionLimit)
	}
	if reloadedCfg.SaveFilter != "*Test*" {
		t.Errorf("Expected SaveFilter '*Test*', got %s", reloadedCfg.SaveFilter)
	}
}

func TestGetPublicProfileDir(t *testing.T) {
	cfg := &Config{
		SourcePath: filepath.Join("C:", "Users", "User", "AppData", "Local", "Larian Studios", "Baldur's Gate 3", "PlayerProfiles", "Public", "Savegames", "Story"),
	}
	expectedPublic := filepath.Join("C:", "Users", "User", "AppData", "Local", "Larian Studios", "Baldur's Gate 3", "PlayerProfiles", "Public")
	gotPublic := cfg.GetPublicProfileDir()
	if gotPublic != expectedPublic {
		t.Errorf("Expected %s, got %s", expectedPublic, gotPublic)
	}

	expectedProfile8 := filepath.Join(expectedPublic, "profile8.lsf")
	gotProfile8 := cfg.GetProfile8Path()
	if gotProfile8 != expectedProfile8 {
		t.Errorf("Expected %s, got %s", expectedProfile8, gotProfile8)
	}
}
