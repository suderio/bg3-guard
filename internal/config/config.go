package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	SourcePath      string `mapstructure:"source_path" yaml:"source_path"`
	DestinationPath string `mapstructure:"destination_path" yaml:"destination_path"`
	IntervalMinutes int    `mapstructure:"interval_minutes" yaml:"interval_minutes"`
	RetentionLimit  int    `mapstructure:"retention_limit" yaml:"retention_limit"`
	SaveFilter      string `mapstructure:"save_filter" yaml:"save_filter"`
}

// Default constants.
const (
	DefaultIntervalMinutes = 30
	DefaultRetentionLimit  = 10
	DefaultSaveFilter      = "*HonourMode*"
	AppName                = "bg3-guard"
	ConfigFileName         = "config.yaml"
)

// GetDefaultConfigDir returns the OS-specific directory for configuration.
func GetDefaultConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, AppName), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", AppName), nil
}

// GetDefaultSourcePath returns the default BG3 Story saves path.
func GetDefaultSourcePath() string {
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "Larian Studios", "Baldur's Gate 3", "PlayerProfiles", "Public", "Savegames", "Story")
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".local", "share", "Larian Studios", "Baldur's Gate 3", "PlayerProfiles", "Public", "Savegames", "Story")
	}
	return ""
}

// GetDefaultDestinationPath returns the default backups destination path.
func GetDefaultDestinationPath() string {
	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			return filepath.Join(userProfile, "Saved Games", "BG3_Honour_Backups")
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, "Saved Games", "BG3_Honour_Backups")
	}
	return filepath.Join(".", "BG3_Honour_Backups")
}

// ExpandPath expands ~ or %ENV% in file paths.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return os.ExpandEnv(path)
}

// LoadConfig loads the configuration from disk, creating default config if needed.
func LoadConfig(overridePath string) (*Config, string, error) {
	v := viper.New()

	var configDir string
	var configFilePath string

	if overridePath != "" {
		configFilePath = ExpandPath(overridePath)
		configDir = filepath.Dir(configFilePath)
		v.SetConfigFile(configFilePath)
	} else {
		var err error
		configDir, err = GetDefaultConfigDir()
		if err != nil {
			return nil, "", fmt.Errorf("failed to get config directory: %w", err)
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, "", fmt.Errorf("failed to create config directory %s: %w", configDir, err)
		}
		configFilePath = filepath.Join(configDir, ConfigFileName)
		v.SetConfigFile(configFilePath)
	}

	v.SetConfigType("yaml")
	v.SetDefault("source_path", GetDefaultSourcePath())
	v.SetDefault("destination_path", GetDefaultDestinationPath())
	v.SetDefault("interval_minutes", DefaultIntervalMinutes)
	v.SetDefault("retention_limit", DefaultRetentionLimit)
	v.SetDefault("save_filter", DefaultSaveFilter)

	cfg := &Config{}

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		// File does not exist, populate with defaults and write
		cfg.SourcePath = v.GetString("source_path")
		cfg.DestinationPath = v.GetString("destination_path")
		cfg.IntervalMinutes = v.GetInt("interval_minutes")
		cfg.RetentionLimit = v.GetInt("retention_limit")
		cfg.SaveFilter = v.GetString("save_filter")

		if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err == nil {
			_ = SaveConfig(cfg, configFilePath)
		}
		return cfg, configFilePath, nil
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, configFilePath, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, configFilePath, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand paths
	cfg.SourcePath = ExpandPath(cfg.SourcePath)
	cfg.DestinationPath = ExpandPath(cfg.DestinationPath)

	if cfg.IntervalMinutes <= 0 {
		cfg.IntervalMinutes = DefaultIntervalMinutes
	}
	if cfg.RetentionLimit <= 0 {
		cfg.RetentionLimit = DefaultRetentionLimit
	}
	if cfg.SaveFilter == "" {
		cfg.SaveFilter = DefaultSaveFilter
	}

	return cfg, configFilePath, nil
}

// SaveConfig persists the given config to the specified file path.
func SaveConfig(cfg *Config, configFilePath string) error {
	v := viper.New()
	v.SetConfigFile(configFilePath)
	v.SetConfigType("yaml")

	v.Set("source_path", cfg.SourcePath)
	v.Set("destination_path", cfg.DestinationPath)
	v.Set("interval_minutes", cfg.IntervalMinutes)
	v.Set("retention_limit", cfg.RetentionLimit)
	v.Set("save_filter", cfg.SaveFilter)

	if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
		return err
	}

	return v.WriteConfigAs(configFilePath)
}

// GetPublicProfileDir returns the path to the Public profile directory (containing profile8.lsf).
func (c *Config) GetPublicProfileDir() string {
	// If source is .../PlayerProfiles/Public/Savegames/Story, public dir is .../PlayerProfiles/Public
	clean := filepath.Clean(c.SourcePath)
	parts := strings.Split(clean, string(filepath.Separator))
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.EqualFold(parts[i], "Public") {
			return strings.Join(parts[:i+1], string(filepath.Separator))
		}
	}
	// Fallback to two levels up if source ends with Savegames/Story
	parent := filepath.Dir(c.SourcePath)
	grandParent := filepath.Dir(parent)
	return grandParent
}

// GetProfile8Path returns the full path to profile8.lsf.
func (c *Config) GetProfile8Path() string {
	return filepath.Join(c.GetPublicProfileDir(), "profile8.lsf")
}
