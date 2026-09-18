//go:build windows

package terminal

import (
	"testing"
)

func TestGetConsoleProcessCountWindows(t *testing.T) {
	count := getConsoleProcessCount()
	t.Logf("Console process count in test runner: %d", count)
	// During 'go test', there is a test binary running inside a console/shell session,
	// so count should typically be >= 1.
	if count < 0 {
		t.Errorf("expected count >= 0, got %d", count)
	}
}

func TestIsDoubleClickedInShell(t *testing.T) {
	// Running within go test in a terminal should not report as double-clicked (count should be != 1).
	count := getConsoleProcessCount()
	doubleClicked := isDoubleClicked()
	if count > 1 && doubleClicked {
		t.Errorf("isDoubleClicked reported true but count was %d", count)
	}
}
