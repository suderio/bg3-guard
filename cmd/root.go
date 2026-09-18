package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/suderio/bg3-guard/internal/backup"
	"github.com/suderio/bg3-guard/internal/config"
	"github.com/suderio/bg3-guard/internal/recover"
	"github.com/suderio/bg3-guard/internal/restore"
	"github.com/suderio/bg3-guard/internal/terminal"
	"github.com/suderio/bg3-guard/internal/tui"
)

var (
	flagConfig      string
	flagRetention   int
	flagInterval    int
	flagDestination string
	flagSource      string
	flagFilter      string

	// Build metadata populated via GoReleaser ldflags
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "bg3-guard",
	Short: "Baldur's Gate 3 Honour Mode automated backup, safe restore, and save recovery",
	Long: `BG3: Guard (bg3-guard) is a tool for Baldur's Gate 3.
It provides automatic save backups with file-corruption protection, safe transactional restore,
and Honour Mode rescue after a Total Party Kill (TPK).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, configPath, err := loadAndPersistConfig(cmd)
		if err != nil {
			return err
		}

		// Launch Bubble Tea TUI
		p := tea.NewProgram(tui.NewModel(cfg, configPath), tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func loadAndPersistConfig(cmd *cobra.Command) (*config.Config, string, error) {
	cfg, configPath, err := config.LoadConfig(flagConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load configuration: %w", err)
	}

	persisted := false
	if cmd.Flags().Changed("retention") {
		cfg.RetentionLimit = flagRetention
		persisted = true
	}
	if cmd.Flags().Changed("interval") {
		cfg.IntervalMinutes = flagInterval
		persisted = true
	}
	if cmd.Flags().Changed("destination") {
		cfg.DestinationPath = config.ExpandPath(flagDestination)
		persisted = true
	}
	if cmd.Flags().Changed("source") {
		cfg.SourcePath = config.ExpandPath(flagSource)
		persisted = true
	}
	if cmd.Flags().Changed("filter") {
		cfg.SaveFilter = flagFilter
		persisted = true
	}

	if persisted {
		if err := config.SaveConfig(cfg, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to persist updated config flags: %v\n", err)
		} else {
			fmt.Printf("Configuration successfully updated in %s\n", configPath)
		}
	}

	return cfg, configPath, nil
}

// backupCmd runs a headless backup monitor loop without TUI
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Start the automatic backup monitoring loop in headless mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, configPath, err := loadAndPersistConfig(cmd)
		if err != nil {
			return err
		}

		fmt.Println("=== Baldur's Gate 3 Honour Mode Auto-Backup ===")
		fmt.Printf("Config File:   %s\n", configPath)
		fmt.Printf("Source Path:   %s\n", cfg.SourcePath)
		fmt.Printf("Destination:   %s\n", cfg.DestinationPath)
		fmt.Printf("Interval:      %d minutes\n", cfg.IntervalMinutes)
		fmt.Printf("Retention:     %d backups per campaign\n", cfg.RetentionLimit)
		fmt.Printf("Save Filter:   %s\n", cfg.SaveFilter)
		fmt.Println("Monitoring started. Press Ctrl+C to stop.")
		fmt.Println()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		ticker := time.NewTicker(time.Duration(cfg.IntervalMinutes) * time.Minute)
		defer ticker.Stop()

		logFn := func(msg string) {
			fmt.Printf("[%s] %s\n", time.Now().Format("15:04:05"), msg)
		}

		// Initial cycle
		_, _ = backup.RunBackupCycle(cfg, logFn)

		for {
			select {
			case <-sigChan:
				fmt.Println("\nShutdown signal received. Executing final retention cleanup...")
				_, _ = backup.CleanupRetention(cfg.DestinationPath, cfg.RetentionLimit, logFn)
				fmt.Println("Exiting safely.")
				return nil
			case <-ticker.C:
				_, _ = backup.RunBackupCycle(cfg, logFn)
			}
		}
	},
}

// restoreCmd provides an interactive CLI restoration prompt
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Interactively restore a previous save backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadAndPersistConfig(cmd)
		if err != nil {
			return err
		}

		backups, err := backup.GetBackups(cfg.DestinationPath)
		if err != nil || len(backups) == 0 {
			fmt.Printf("No backups found in '%s'.\n", cfg.DestinationPath)
			return nil
		}

		groups, _ := backup.GetBackupsByGroup(cfg.DestinationPath)
		var groupNames []string
		for name := range groups {
			groupNames = append(groupNames, name)
		}

		reader := bufio.NewReader(os.Stdin)

		selectedGroup := groupNames[0]
		if len(groupNames) > 1 {
			fmt.Println("\n=== Available Save Campaigns ===")
			for i, name := range groupNames {
				fmt.Printf("[%d] %s (%d backups)\n", i+1, name, len(groups[name]))
			}
			fmt.Printf("Choose group (1-%d): ", len(groupNames))
			line, _ := reader.ReadString('\n')
			idx, _ := strconv.Atoi(strings.TrimSpace(line))
			if idx < 1 || idx > len(groupNames) {
				return fmt.Errorf("invalid group selection")
			}
			selectedGroup = groupNames[idx-1]
		}

		groupBackups := groups[selectedGroup]
		fmt.Printf("\n=== Available Backups for '%s' ===\n", selectedGroup)
		for i, b := range groupBackups {
			fmt.Printf("[%d] %s (%s)\n", i+1, restore.FormatTimestamp(b.Name), b.Name)
		}
		fmt.Printf("Choose backup (1-%d): ", len(groupBackups))
		line, _ := reader.ReadString('\n')
		idx, _ := strconv.Atoi(strings.TrimSpace(line))
		if idx < 1 || idx > len(groupBackups) {
			return fmt.Errorf("invalid backup selection")
		}
		chosenBackup := groupBackups[idx-1]

		// Safety check
		safe, warnMsg := restore.CheckRestoreSafety()
		if !safe {
			fmt.Println("\n" + warnMsg)
			fmt.Print("Proceed anyway? (y/N): ")
			ans, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(ans)) != "y" {
				fmt.Println("Restoration cancelled.")
				return nil
			}
		}

		res, err := restore.PerformSafeRestore(cfg, chosenBackup, true, func(s string) {
			fmt.Println("  -> " + s)
		})
		if err != nil {
			return err
		}
		fmt.Printf("\nSUCCESS: %s\n", res.Message)
		return nil
	},
}

// recoverCmd provides an interactive CLI Honour Mode reactivation
var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover and reactivate Honour Mode for a save converted to Custom",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, configPath, err := loadAndPersistConfig(cmd)
		if err != nil {
			return err
		}

		entries, err := os.ReadDir(cfg.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source directory %s: %w", cfg.SourcePath, err)
		}

		var allSaves []string
		for _, e := range entries {
			if e.IsDir() {
				allSaves = append(allSaves, e.Name())
			}
		}

		if len(allSaves) == 0 {
			fmt.Println("No save folders found in " + cfg.SourcePath)
			return nil
		}

		reader := bufio.NewReader(os.Stdin)
		var selectedSave string

		for {
			filtered := backup.FilterSaveFolders(allSaves, cfg.SaveFilter)

			fmt.Println("\n=== Honour Mode Save Recovery ===")
			fmt.Println(recover.CloudSyncWarningMessage)
			fmt.Printf("\nSave Filter: %s\n", cfg.SaveFilter)

			if len(filtered) == 0 {
				fmt.Printf("No save folders matching filter '%s' found (total folders: %d).\n", cfg.SaveFilter, len(allSaves))
				fmt.Print("Enter new filter pattern (or '*' to show all, 'q' to abort): ")
				fline, _ := reader.ReadString('\n')
				fline = strings.TrimSpace(fline)
				if strings.ToLower(fline) == "q" {
					fmt.Println("Recovery aborted.")
					return nil
				}
				if fline != "" {
					cfg.SaveFilter = fline
					_ = config.SaveConfig(cfg, configPath)
					fmt.Printf("Save filter updated to '%s' (persisted to config).\n", cfg.SaveFilter)
				}
				continue
			}

			fmt.Printf("Available save folders (matching '%s'):\n", cfg.SaveFilter)
			for i, name := range filtered {
				fmt.Printf("[%d] %s\n", i+1, name)
			}
			fmt.Printf("[f] Change filter (currently '%s')\n", cfg.SaveFilter)

			fmt.Printf("Choose save to recover (1-%d, or 'f' to change filter): ", len(filtered))
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if strings.ToLower(line) == "f" {
				fmt.Print("Enter new filter pattern (e.g. '*' or '*Custom*'): ")
				fline, _ := reader.ReadString('\n')
				fline = strings.TrimSpace(fline)
				if fline != "" {
					cfg.SaveFilter = fline
					_ = config.SaveConfig(cfg, configPath)
					fmt.Printf("Save filter updated to '%s' (persisted to config).\n", cfg.SaveFilter)
				}
				continue
			}

			idx, err := strconv.Atoi(line)
			if err != nil || idx < 1 || idx > len(filtered) {
				fmt.Println("Invalid selection, please try again.")
				continue
			}

			selectedSave = filtered[idx-1]
			break
		}


		running, proc, cloudWarn := recover.CheckRecoverySafety()
		if running {
			fmt.Printf("\nWARNING: Baldur's Gate 3 is currently running (%s)!\n", proc)
			fmt.Print("Proceeding while the game is running will cause corruption. Force continue? (y/N): ")
			ans, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(ans)) != "y" {
				fmt.Println("Recovery aborted.")
				return nil
			}
		}

		fmt.Println("\n" + cloudWarn)
		fmt.Printf("\nConfirm Honour Mode recovery for '%s'? (y/N): ", selectedSave)
		ans, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(ans)) != "y" {
			fmt.Println("Recovery aborted.")
			return nil
		}

		res, err := recover.RecoverSave(cfg, selectedSave, true, func(s string) {
			fmt.Println("  -> " + s)
		})
		if err != nil {
			return err
		}

		fmt.Printf("\nSUCCESS: %s\n", res.Message)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print application version and build details",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("bg3-guard %s\nCommit:   %s\nBuilt at: %s\nBuilt by: %s\n", Version, Commit, Date, BuiltBy)
	},
}

func init() {
	// Disable Cobra's default Windows mousetrap splash screen so the application
	// can launch its interactive Bubble Tea TUI seamlessly.
	cobra.MousetrapHelpText = ""

	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(fmt.Sprintf("bg3-guard {{.Version}} (commit: %s, date: %s, built by: %s)\n", Commit, Date, BuiltBy))


	rootCmd.PersistentFlags().StringVarP(&flagConfig, "config", "c", "", "Path to custom config.yaml")
	rootCmd.PersistentFlags().IntVarP(&flagRetention, "retention", "r", config.DefaultRetentionLimit, "Retention limit for backups")
	rootCmd.PersistentFlags().IntVarP(&flagInterval, "interval", "i", config.DefaultIntervalMinutes, "Auto-backup check interval in minutes")
	rootCmd.PersistentFlags().StringVarP(&flagDestination, "destination", "d", "", "Destination directory for backups")
	rootCmd.PersistentFlags().StringVarP(&flagSource, "source", "s", "", "Source directory containing BG3 Story save games")
	rootCmd.PersistentFlags().StringVar(&flagFilter, "filter", config.DefaultSaveFilter, "Glob pattern to filter save folders")

	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(recoverCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root CLI command.
func Execute() error {
	terminal.EnsureInteractiveTerminal()
	return rootCmd.Execute()
}
