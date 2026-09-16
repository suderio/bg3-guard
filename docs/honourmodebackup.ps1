# Backup name pattern: FolderName-YYYYMMDDHHMM (12 numeric digits at the end)
$BackupRegex = '^(.+)-(\d{12})$'

# Setting paths dynamically and robustly (without specific user paths)
$source = Join-Path $env:LOCALAPPDATA "Larian Studios\Baldur's Gate 3\PlayerProfiles\Public\Savegames\Story"
$destination = Join-Path $env:USERPROFILE "Saved Games"

# Settings
$intervalMinutes = 5
$backupLimit = 30

# Ensures the destination folder exists
if (-not (Test-Path -Path $destination)) {
    try {
        New-Item -ItemType Directory -Path $destination -Force | Out-Null
    }
    catch {
        Write-Error "Could not create destination folder '$destination': $_"
        exit 1
    }
}

# Returns all valid backups found in the destination using the centralized regex pattern
function Get-Backups {
    if (Test-Path -Path $destination) {
        Get-ChildItem -Path $destination -Directory | Where-Object { $_.Name -match $BackupRegex }
    }
    else {
        @()
    }
}

# Compares the source folder files with the previous backup folder
# Returns $true if any file was changed, added or removed; otherwise, returns $false
function Test-FolderChanged {
    param (
        [Parameter(Mandatory = $true)]
        [string]$SourceFolder,

        [Parameter(Mandatory = $true)]
        [string]$BackupFolder
    )

    if (-not (Test-Path -Path $SourceFolder)) {
        return $false
    }
    if (-not (Test-Path -Path $BackupFolder)) {
        # If the previous backup folder does not exist, we consider there are changes (initial backup needed)
        return $true
    }

    $sourceFiles = @(Get-ChildItem -Path $SourceFolder -Recurse -File)
    $backupFiles = @(Get-ChildItem -Path $BackupFolder -Recurse -File)

    # If the number of files is different (files added or removed)
    if ($sourceFiles.Count -ne $backupFiles.Count) {
        return $true
    }

    # Maps backup files by relative path for quick search
    $backupMap = @{}
    foreach ($file in $backupFiles) {
        $relativePath = $file.FullName.Substring($BackupFolder.Length).TrimStart('\', '/')
        $backupMap[$relativePath] = $file
    }

    foreach ($sourceFile in $sourceFiles) {
        $relativePath = $sourceFile.FullName.Substring($SourceFolder.Length).TrimStart('\', '/')

        # New / added file in source that doesn't exist in backup
        if (-not $backupMap.ContainsKey($relativePath)) {
            return $true
        }

        $backupFile = $backupMap[$relativePath]

        # File with changed size
        if ($sourceFile.Length -ne $backupFile.Length) {
            return $true
        }

        # File with changed modification date
        if ($sourceFile.LastWriteTimeUtc -ne $backupFile.LastWriteTimeUtc) {
            return $true
        }
    }

    # No files were changed, added, or removed
    return $false
}

# Cleans up excess old backups keeping only the configured limit per group
function Invoke-Cleanup {
    Write-Host "`nStarting cleanup routine in destination..." -ForegroundColor Yellow

    $backupFolders = Get-Backups

    if (-not $backupFolders) {
        Write-Host "No date-formatted backup found for cleanup." -ForegroundColor Gray
        return
    }

    # Groups by original name (removing the timestamp suffix)
    $groups = $backupFolders | Group-Object { $_.Name -replace '-\d{12}$', '' }

    foreach ($group in $groups) {
        Write-Host "Checking group: $($group.Name) (Total: $($group.Count))" -ForegroundColor Cyan

        # Since the timestamp 'yyyyMMddHHmm' is alphanumerically sortable, sorting by Name ensures chronological order
        $sortedBackups = $group.Group | Sort-Object -Property Name -Descending

        # Skips the N most recent and selects the excess ones for deletion
        $excessBackups = $sortedBackups | Select-Object -Skip $backupLimit

        if ($excessBackups) {
            foreach ($item in $excessBackups) {
                Write-Host "  -> Removing old version: $($item.Name)" -ForegroundColor DarkGray
                try {
                    Remove-Item -Path $item.FullName -Recurse -Force -ErrorAction Stop
                }
                catch {
                    Write-Warning "Failed to remove old backup $($item.FullName): $_"
                }
            }
        }
        else {
            Write-Host "  -> No excess versions (limit of $backupLimit not reached)." -ForegroundColor Gray
        }
    }
}

function Restore {
    # Validates source path before proceeding
    if (-not (Test-Path -Path $source)) {
        Write-Error "The original game savegames folder was not found at '$source'."
        return
    }

    $backups = Get-Backups

    if (-not $backups) {
        Write-Warning "No backup found in '$destination'."
        return
    }

    $groups = $backups | Group-Object { $_.Name -replace '-\d{12}$', '' }
    $selectedGroup = $null

    # Savegame group selection
    if ($groups.Count -eq 1) {
        $selectedGroup = $groups[0]
        Write-Host "`nSelected group: " -NoNewline
        Write-Host "$($selectedGroup.Name)" -ForegroundColor Cyan
    }
    else {
        Write-Host "`n=== Available Groups ===" -ForegroundColor Yellow
        for ($i = 0; $i -lt $groups.Count; $i++) {
            Write-Host "[$($i + 1)] $($groups[$i].Name) ($($groups[$i].Count) backups)" -ForegroundColor Cyan
        }

        $groupChoice = 0
        while ($groupChoice -lt 1 -or $groupChoice -gt $groups.Count) {
            $inputGroup = Read-Host "`nChoose the group (1-$($groups.Count))"
            if ($inputGroup -match '^\d+$') {
                $groupChoice = [int]$inputGroup
            }
            if ($groupChoice -lt 1 -or $groupChoice -gt $groups.Count) {
                Write-Host "Invalid option. Type a valid number corresponding to the group." -ForegroundColor Red
            }
        }
        $selectedGroup = $groups[$groupChoice - 1]
    }

    # Selection of specific backup from the selected group
    $sortedBackups = $selectedGroup.Group | Sort-Object -Property Name -Descending

    Write-Host "`n=== Backups for '$($selectedGroup.Name)' ===" -ForegroundColor Yellow
    for ($j = 0; $j -lt $sortedBackups.Count; $j++) {
        $folderName = $sortedBackups[$j].Name
        if ($folderName -match '-(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})$') {
            # Format: YYYY-MM-DD at HH:MM
            $displayDate = "$($Matches[1])-$($Matches[2])-$($Matches[3]) at $($Matches[4]):$($Matches[5])"
        }
        else {
            $displayDate = $folderName
        }
        Write-Host "[$($j + 1)] " -NoNewline -ForegroundColor Green
        Write-Host "$displayDate " -NoNewline
        Write-Host "($folderName)" -ForegroundColor DarkGray
    }

    $backupChoice = 0
    while ($backupChoice -lt 1 -or $backupChoice -gt $sortedBackups.Count) {
        $inputBackup = Read-Host "`nChoose the backup number (1-$($sortedBackups.Count))"
        if ($inputBackup -match '^\d+$') {
            $backupChoice = [int]$inputBackup
        }
        if ($backupChoice -lt 1 -or $backupChoice -gt $sortedBackups.Count) {
            Write-Host "Invalid option. Type a valid number corresponding to the backup." -ForegroundColor Red
        }
    }

    $chosenBackup = $sortedBackups[$backupChoice - 1]
    $targetSourceFolder = Join-Path -Path $source -ChildPath $selectedGroup.Name

    Write-Host "`nStarting safe restoration:" -ForegroundColor Yellow
    Write-Host "  Source:       $($chosenBackup.FullName)" -ForegroundColor DarkGray
    Write-Host "  Destination:  $targetSourceFolder" -ForegroundColor DarkGray

    # Safe transactional pattern: Rename current save before copying backup
    $temporaryBackup = $null
    if (Test-Path -Path $targetSourceFolder) {
        $temporaryBackup = "${targetSourceFolder}_safe_restoration"
        if (Test-Path -Path $temporaryBackup) {
            # If a residual temporary folder exists for some reason, remove it first
            try {
                Remove-Item -Path $temporaryBackup -Recurse -Force -ErrorAction Stop
            }
            catch {
                Write-Error "Could not clean old temporary folder '$temporaryBackup': $_"
                return
            }
        }

        try {
            # Rename the original save for safety
            Rename-Item -Path $targetSourceFolder -NewName (Split-Path $temporaryBackup -Leaf) -ErrorAction Stop
            Write-Host "  -> Original save temporarily renamed for safety." -ForegroundColor DarkGray
        }
        catch {
            Write-Error "Failed to rename original save for safety: $_. Restoration aborted."
            return
        }
    }

    # Copy the selected backup to the original destination
    try {
        Copy-Item -Path $chosenBackup.FullName -Destination $targetSourceFolder -Recurse -Force -ErrorAction Stop
        Write-Host "  -> Backup successfully copied to the destination." -ForegroundColor DarkGray
        
        # If copy was successful, remove old original save (temporary)
        if ($temporaryBackup -and (Test-Path -Path $temporaryBackup)) {
            Remove-Item -Path $temporaryBackup -Recurse -Force -ErrorAction Stop
            Write-Host "  -> Old temporary safety save removed." -ForegroundColor DarkGray
        }
        Write-Host "`nRestoration successfully completed!" -ForegroundColor Green
    }
    catch {
        Write-Error "Error copying backup files: $_"
        
        # Try to rollback if copying backup failed
        if ($temporaryBackup -and (Test-Path -Path $temporaryBackup)) {
            Write-Warning "Attempting to restore original save due to copy failure..."
            if (Test-Path -Path $targetSourceFolder) {
                # Try to remove corrupted/partial failed copy
                Remove-Item -Path $targetSourceFolder -Recurse -Force -ErrorAction SilentlyContinue
            }
            try {
                Rename-Item -Path $temporaryBackup -NewName (Split-Path $targetSourceFolder -Leaf) -ErrorAction Stop
                Write-Host "Original save restored successfully. No data was lost." -ForegroundColor Green
            }
            catch {
                Write-Error "Critical failure when rolling back original save from '$temporaryBackup' to '$targetSourceFolder'. Please check the files manually."
            }
        }
    }
}

function Start-BackupLoop {
    Write-Host "Monitoring started. Press Ctrl+C to stop.`n" -ForegroundColor Yellow

    # Initial source validation
    if (-not (Test-Path -Path $source)) {
        Write-Warning "The saves source folder ('$source') was not found. Make sure the game is installed and you already have saved games."
    }

    try {
        while ($true) {
            $timestamp = Get-Date -Format "yyyyMMddHHmm"
            $executionTime = Get-Date -Format "HH:mm:ss"

            if (Test-Path -Path $source) {
                # Search for folders containing "HonourMode" in their name
                $foundFolders = Get-ChildItem -Path $source -Directory | Where-Object { $_.Name -like "*HonourMode*" }

                if ($foundFolders) {
                    $anyBackupMade = $false
                    foreach ($folder in $foundFolders) {
                        # Search for the most recent existing backup for this folder
                        $latestBackup = @(Get-Backups | Where-Object {
                                $_.Name -match $BackupRegex -and $Matches[1] -eq $folder.Name
                            } | Sort-Object -Property Name -Descending) | Select-Object -First 1

                        $shouldBackup = $true
                        if ($latestBackup) {
                            $changed = Test-FolderChanged -SourceFolder $folder.FullName -BackupFolder $latestBackup.FullName
                            if (-not $changed) {
                                $shouldBackup = $false
                                Write-Host "[$executionTime] Files have not changed: $($folder.Name)" -ForegroundColor DarkGray
                            }
                        }

                        if ($shouldBackup) {
                            $nameWithTimestamp = "$($folder.Name)-$timestamp"
                            $destinationPath = Join-Path -Path $destination -ChildPath $nameWithTimestamp

                            Write-Host "[$executionTime] Copying: $($folder.Name) -> $nameWithTimestamp" -ForegroundColor Cyan
                            try {
                                Copy-Item -Path $folder.FullName -Destination $destinationPath -Recurse -Force -ErrorAction Stop
                                $anyBackupMade = $true
                            }
                            catch {
                                Write-Error "[$executionTime] Failed to copy savegame '$($folder.Name)': $_"
                            }
                        }
                    }
                    if ($anyBackupMade) {
                        Write-Host "[$executionTime] Backup successfully completed." -ForegroundColor Green
                        
                        # Runs cleanup periodically after each successful backup, reducing risk of excessive accumulation
                        Invoke-Cleanup
                    }
                }
                else {
                    Write-Host "[$executionTime] No matching folder ('*HonourMode*') found at '$source'." -ForegroundColor DarkGray
                }
            }
            else {
                Write-Warning "[$executionTime] Source folder '$source' inaccessible or missing."
            }

            Write-Host "Next backup in $intervalMinutes minutes...`n" -ForegroundColor Gray
            Start-Sleep -Seconds ($intervalMinutes * 60)
        }
    }
    finally {
        # Execute final cleanup when script is terminated by Ctrl+C
        Invoke-Cleanup
    }
}

# Entry Point
Write-Host "`n=== Baldur's Gate 3 Honour Mode Tool ===" -ForegroundColor Yellow
Write-Host "1 - Back Up (Automatic monitoring)" -ForegroundColor Cyan
Write-Host "2 - Restore (Restore a backup)" -ForegroundColor Cyan

$option = 0
while ($option -ne 1 -and $option -ne 2) {
    $inputOption = Read-Host "`nChoose an option (1 or 2)"
    if ($inputOption -match '^[12]$') {
        $option = [int]$inputOption
    }
    else {
        Write-Host "Invalid option. Type 1 or 2." -ForegroundColor Red
    }
}

if ($option -eq 1) {
    Start-BackupLoop
}
else {
    Restore
}