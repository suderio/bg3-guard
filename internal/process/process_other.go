//go:build !windows

package process

import (
	"os/exec"
	"strings"
)

// IsGameRunning checks if Baldur's Gate 3 is currently active on non-Windows platforms.
func IsGameRunning() (bool, string, error) {
	out, err := exec.Command("pgrep", "-l", "-f", "bg3").Output()
	if err != nil {
		// pgrep returns non-zero when not found
		return false, "", nil
	}
	output := string(out)
	for _, name := range BG3ProcessNames {
		if strings.Contains(strings.ToLower(output), strings.ToLower(name)) {
			return true, name, nil
		}
	}
	return false, "", nil
}
