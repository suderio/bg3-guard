//go:build !windows

package terminal

// EnsureInteractiveTerminal is a no-op on non-Windows platforms (Linux, macOS, etc.)
// where graphical desktop launchers or terminal emulators already handle execution.
func EnsureInteractiveTerminal() {
}
