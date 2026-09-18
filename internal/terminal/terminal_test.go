package terminal

import (
	"testing"
)

func TestEnvTerminalSpawnedConstant(t *testing.T) {
	if EnvTerminalSpawned != "BG3_GUARD_TERMINAL" {
		t.Errorf("expected EnvTerminalSpawned to be 'BG3_GUARD_TERMINAL', got '%s'", EnvTerminalSpawned)
	}
}

func TestEnsureInteractiveTerminalWithEnv(t *testing.T) {
	// When EnvTerminalSpawned is set, EnsureInteractiveTerminal must return immediately
	t.Setenv(EnvTerminalSpawned, "1")
	EnsureInteractiveTerminal()
}

func TestEnsureInteractiveTerminalInTestSession(t *testing.T) {
	// In an active test runner, getConsoleProcessCount is >= 2 or not double-clicked,
	// so EnsureInteractiveTerminal should safely return without exiting.
	EnsureInteractiveTerminal()
}
