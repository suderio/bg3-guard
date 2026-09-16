package process

import "testing"

func TestIsGameRunning(t *testing.T) {
	running, name, err := IsGameRunning()
	if err != nil {
		t.Fatalf("IsGameRunning returned error: %v", err)
	}
	t.Logf("BG3 running: %v (detected executable: %q)", running, name)
}
