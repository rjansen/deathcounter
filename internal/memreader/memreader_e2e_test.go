//go:build e2e && windows

package memreader

import (
	"testing"
	"time"
)

// skipIfNoGameRunning creates a real GameReader and skips the test if no game is running.
func skipIfNoGameRunning(t *testing.T) *GameReader {
	t.Helper()
	reader, err := NewGameReader()
	if err != nil || !reader.IsAttached() {
		t.Skipf("No supported game running: %v", err)
	}
	return reader
}

func TestE2E_AttachToRunningGame(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()

	if !reader.IsAttached() {
		t.Error("expected reader to be attached")
	}
	game := reader.GetCurrentGame()
	if game == "" {
		t.Error("expected non-empty game name")
	}
	t.Logf("Attached to: %s", game)
}

func TestE2E_ReadDeathCount(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()

	count, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}

	t.Logf("[%s] Death count: %d", reader.GetCurrentGame(), count)
}

func TestE2E_ReadDeathCountStable(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()

	first, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("initial ReadDeathCount failed: %v", err)
	}

	// Read 10 times over 5 seconds and verify count is stable
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		count, err := reader.ReadDeathCount()
		if err != nil {
			t.Fatalf("ReadDeathCount iteration %d failed: %v", i, err)
		}
		if count != first {
			t.Logf("count changed from %d to %d at iteration %d (player may have died)", first, count, i)
			first = count // Accept the new count and keep checking stability
		}
	}

	t.Logf("[%s] Stable death count: %d", reader.GetCurrentGame(), first)
}

func TestE2E_DetachAndReattach(t *testing.T) {
	reader := skipIfNoGameRunning(t)

	game := reader.GetCurrentGame()
	count, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("initial ReadDeathCount failed: %v", err)
	}

	reader.Detach()
	if reader.IsAttached() {
		t.Error("should not be attached after detach")
	}

	// Reattach
	err = reader.Attach()
	if err != nil {
		t.Fatalf("reattach failed: %v", err)
	}

	if reader.GetCurrentGame() != game {
		t.Errorf("expected same game %q, got %q", game, reader.GetCurrentGame())
	}

	newCount, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount after reattach failed: %v", err)
	}
	if newCount != count {
		t.Logf("count changed from %d to %d between detach/reattach (player may have died)", count, newCount)
	}

	reader.Detach()
}

func TestE2E_MonitorDeathCountChange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping interactive test in short mode")
	}

	reader := skipIfNoGameRunning(t)
	defer reader.Detach()

	initial, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("initial ReadDeathCount failed: %v", err)
	}

	t.Logf("[%s] Current death count: %d", reader.GetCurrentGame(), initial)
	t.Log("Please die in-game within 60 seconds...")

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Skip("timed out waiting for death count to change (no death occurred)")
		case <-ticker.C:
			count, err := reader.ReadDeathCount()
			if err != nil {
				t.Fatalf("ReadDeathCount failed during monitoring: %v", err)
			}
			if count != initial {
				diff := count - initial
				t.Logf("Death count changed: %d → %d (diff: %d)", initial, count, diff)
				if diff != 1 {
					t.Errorf("expected count to increase by 1, increased by %d", diff)
				}
				return
			}
		}
	}
}
