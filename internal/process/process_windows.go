//go:build windows

package process

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsGameRunning checks if Baldur's Gate 3 (bg3.exe or bg3_dx11.exe) is currently active.
func IsGameRunning() (bool, string, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, "", err
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	err = windows.Process32First(snapshot, &entry)
	for err == nil {
		procName := syscall.UTF16ToString(entry.ExeFile[:])
		for _, name := range BG3ProcessNames {
			if strings.EqualFold(procName, name) {
				return true, procName, nil
			}
		}
		err = windows.Process32Next(snapshot, &entry)
	}

	return false, "", nil
}
