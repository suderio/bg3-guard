# Walkthrough - BG3: Guard (bg3-guard)

The Go application **BG3: Guard** (`bg3-guard`) has been created according to the specifications in [README.md](../README.md) and guided by [honourmodebackup.ps1](./honourmodebackup.ps1).

The application compiles into a single, self-contained executable (`bg3-guard.exe`) that embeds the complete **LSLib Divine toolchain** without requiring external PowerShell scripts or runtime .NET dependencies.

---

## What Was Implemented

### 1. Configuration & CLI System

- [config.go](../internal/config/config.go): Cross-platform configuration management via Viper. Automatically detects paths on Windows (`%LOCALAPPDATA%`, `%APPDATA%`, `%USERPROFILE%`) and Linux. Expands environment variables and `~`.
- [root.go](../cmd/root.go): Cobra CLI exposing `-r/--retention`, `-i/--interval`, `-d/--destination`, `-s/--source`, and `--filter`. Any flag provided on the command line immediately persists to `%APPDATA%\bg3-guard\config.yaml`.
- Provides subcommands:
  - `bg3-guard` (launches interactive Charm TUI)
  - `bg3-guard backup` (headless background monitoring daemon)
  - `bg3-guard restore` (interactive command-line restore)
  - `bg3-guard recover` (interactive command-line Honour Mode rescue)

### 2. Process & File Corruption Protection

- [process_windows.go](../internal/process/process_windows.go): High-performance process detection using `windows.CreateToolhelp32Snapshot` to check if `bg3.exe` or `bg3_dx11.exe` is running.
- [filelock_windows.go](../internal/filelock/filelock_windows.go): File lock and stability detection:
  - Detects Windows sharing violations (`ERROR_SHARING_VIOLATION` / `ERROR_LOCK_VIOLATION`).
  - Verifies file size stability and minimum file age before copying. If the game is actively writing to a save, backup is delayed smoothly to prevent capturing corrupt or partial saves.

### 3. Backup & Retention Engine

- [backup.go](../internal/backup/backup.go): Centralized regex `^(.+)-(\d{12})$` for timestamped backups (`FolderName-YYYYMMDDHHMM`).
- Recursive file comparison (`HasFolderChanged`) checking file counts, paths, sizes, and timestamps to skip redundant backups.
- Retention cleanup: Automatically prunes excess snapshots beyond the retention limit per campaign group.
- Graceful shutdown on `os.Interrupt` / `SIGINT` (Ctrl+C).

### 4. Safe Restore Engine

- [restore.go](../internal/restore/restore.go):
  - Game process warning: Alerts the user if BG3 is running and asks for confirmation.
  - **Mandatory Pre-Restore Backup**: Automatically creates a safety backup of the current save (`<Campaign>-PreRestore-YYYYMMDDHHMM`) in the destination folder before modifying anything.
  - **Transactional Rollback**: Renames the active save to `..._safe_restoration` before copying. Automatically rolls back the original save if copying fails or is interrupted.

### 5. Dual-Layer Honour Mode Recovery Engine

- [divine.go](../internal/recover/divine.go): Wrapper for `divine.exe` supporting package extraction, recompilation, and LZ4HC packaging (ensuring temp directories avoid dot-prefixed paths which LSLib treats as hidden).
- [xml_patch.go](../internal/recover/xml_patch.go):
  - Patches `meta.lsx` inside the `.lsv` package: resets `DishonorDifficultySelection` to zeroes, sets `DisabledSingleSave` to `False`, sets `GameDifficulty` to `3` (Honour), and restores RuleSetId `b1935c10-0931-4148-8df0-7d722ec781aa`.
  - Patches `profile8.lsx`: removes the campaign session GUID from `<node id="DisabledSingleSaveSessions">`.
- [recover.go](../internal/recover/recover.go):
  - Checks if the game is running.
  - Prominently displays the critical **Steam Cloud & Larian Cross-Save Warning**.
  - Takes an automatic safeguard snapshot `Custom-<Campaign>-YYYYMMDDHHMM` before making changes.

### 6. Terminal User Interface (TUI)

- [tui.go](../internal/tui/tui.go) & [styles.go](../internal/tui/styles.go):
  - AltScreen fullscreen mode.
  - Real-time countdown timer to the next backup check.
  - Live activity log feed with timestamps.
  - Tabbed navigation: `[1/b] Backup Monitor`, `[2/r] Restore Backups`, `[3/h] Honour Mode Recover`.
  - Search/filter for snapshots with `/`.
  - Interactive warning modals for restore and recovery.

---

## Verification & Testing

### Automated Test Suite

Ran `make test` across all packages:

```text
=== RUN   TestHasFolderChanged
--- PASS: TestHasFolderChanged (0.02s)
=== RUN   TestRetentionCleanup
--- PASS: TestRetentionCleanup (0.01s)
=== RUN   TestRunBackupCycle
--- PASS: TestRunBackupCycle (0.22s)
PASS ok   bg3-guard/internal/backup

=== RUN   TestLoadAndSaveConfig
--- PASS: TestLoadAndSaveConfig (0.01s)
=== RUN   TestGetPublicProfileDir
--- PASS: TestGetPublicProfileDir (0.00s)
PASS ok   bg3-guard/internal/config

=== RUN   TestFileLockUnlocked
--- PASS: TestFileLockUnlocked (0.02s)
=== RUN   TestFileLockRecentModification
--- PASS: TestFileLockRecentModification (0.00s)
PASS ok   bg3-guard/internal/filelock

=== RUN   TestIsGameRunning
--- PASS: TestIsGameRunning (0.01s)
PASS ok   bg3-guard/internal/process

=== RUN   TestPatchMetaLSX
--- PASS: TestPatchMetaLSX (0.01s)
=== RUN   TestProfileLSXPatching
--- PASS: TestProfileLSXPatching (0.01s)
PASS ok   bg3-guard/internal/recover

=== RUN   TestFormatTimestamp
--- PASS: TestFormatTimestamp (0.00s)
=== RUN   TestPerformSafeRestore
--- PASS: TestPerformSafeRestore (0.03s)
PASS ok   bg3-guard/internal/restore
```

### Build Verification

Ran `make build`:

```bash
make build
# Output:
# Divine toolchain already present in assets/divine
# Building bg3-guard.exe...
# go build -ldflags="-s -w" -o bg3-guard.exe .
# Build complete: bg3-guard.exe (21.3 MB standalone binary)
```

### CLI Flag Persistence & Live Backup Test

- Passed flags `-r 7 -i 12` to `bg3-guard.exe`.
- Verified that `config.yaml` was automatically created/updated with `retention_limit: 7` and `interval_minutes: 12`.
- Verified live snapshot creation of the actual Honour Mode save (`f6711bc0-a85a-95d7-952c-d3f18dc6f974__HonourMode-202609161405`) into `Saved Games\BG3_Honour_Backups` containing intact `.lsv` and `.WebP` files.

---

## How to Run the Application

```powershell
# 1. Interactive TUI (Recommended)
.\bg3-guard.exe

# 2. Change configuration (persists across sessions)
.\bg3-guard.exe -r 15 -i 20 -d "D:\MyBackups\BG3"

# 3. Headless background backup daemon
.\bg3-guard.exe backup

# 4. Interactive CLI restore
.\bg3-guard.exe restore

# 5. Interactive CLI Honour Mode recovery
.\bg3-guard.exe recover
```
