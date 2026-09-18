//go:build windows

package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleProcessList = modkernel32.NewProc("GetConsoleProcessList")
)

// getConsoleProcessCount returns the number of processes attached to the current console.
// When run inside an existing interactive terminal (PowerShell, CMD, Git Bash), count >= 2.
// When double-clicked from Windows Explorer, Windows allocates a console host with count == 1.
// If no console is attached or the Win32 call fails, count is 0.
func getConsoleProcessCount() int {
	var pids [2]uint32
	r, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(len(pids)),
	)
	return int(r)
}

// isDoubleClicked checks if the executable was launched by double-clicking in Explorer.
func isDoubleClicked() bool {
	return getConsoleProcessCount() == 1
}

// EnsureInteractiveTerminal checks if the application was executed via double-click
// without an interactive shell, and if so, restarts the application inside
// Windows Terminal (wt.exe) with fallback to cmd.exe.
func EnsureInteractiveTerminal() {
	// Guard against infinite recursive auto-spawning
	if os.Getenv(EnvTerminalSpawned) == "1" {
		return
	}

	// Only auto-spawn if double-clicked from Windows Explorer
	if !isDoubleClicked() {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return
	}

	// Priority 1: Windows Terminal (wt.exe)
	if wtPath, err := exec.LookPath("wt.exe"); err == nil {
		args := append([]string{"-w", "new", exe}, os.Args[1:]...)
		cmd := exec.Command(wtPath, args...)
		cmd.Env = append(os.Environ(), EnvTerminalSpawned+"=1")
		if err := cmd.Start(); err == nil {
			os.Exit(0)
		}
	}

	// Fallback: Command Prompt (cmd.exe)
	if cmdPath, err := exec.LookPath("cmd.exe"); err == nil {
		args := append([]string{"/c", "start", "", exe}, os.Args[1:]...)
		cmd := exec.Command(cmdPath, args...)
		cmd.Env = append(os.Environ(), EnvTerminalSpawned+"=1")
		if err := cmd.Start(); err == nil {
			os.Exit(0)
		}
	}
}
