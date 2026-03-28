//go:build e2e && ds3 && windows

package monitor

import (
	"testing"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
)

// newRealOpsAndAttach creates real ProcessOps, finds DS3, and returns ops + reader.
// Skips the test if no DS3 process is running.
func newRealOpsAndAttach(t *testing.T) (memreader.ProcessOps, *memreader.GameReader) {
	t.Helper()
	ops := memreader.NewProcessOps()
	cfg, proc, err := memreader.FindGame(ops, "ds3")
	if err != nil {
		t.Skipf("No DS3 process running: %v", err)
	}
	reader := memreader.NewGameReader(ops, cfg, proc)
	return ops, reader
}

func newE2ERepo(t *testing.T) *data.Repository {
	t.Helper()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo
}

// initE2EMonitor initializes the display channel for manual tick-based testing
// (bypasses Start() which creates a goroutine with a ticker).
func initE2EMonitor(mon *GameMonitor) {
	mon.displayCh = make(chan DisplayUpdate, 1)
}

// tickE2E is a test helper for e2e tests: simulates one Start() loop cycle.
func tickE2E(t *testing.T, mon *GameMonitor) {
	t.Helper()
	if err := mon.state.Tick(mon); err != nil {
		t.Logf("tickE2E: %v", err)
	}
}

// TestE2E_DeathTracker_PhaseTransitions validates the full
// Detached → Attached → Loaded state machine with a real DS3 process.
func TestE2E_DeathTracker_PhaseTransitions(t *testing.T) {
	ops, reader := newRealOpsAndAttach(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	// We got a reader to verify DS3 is running, but the monitor
	// starts fresh from PhaseDetached — detach and let the monitor attach.
	reader.Detach()

	mon := NewGameMonitor("ds3", ops, NewDeathTracker("ds3", tracker))
	initE2EMonitor(mon)

	// Phase 1: Detached — no game attached yet... but since the process
	// IS running, Attach will succeed on first tick.
	if mon.state.Phase() != PhaseDetached {
		t.Errorf("initial phase: got %s, want %s", mon.state.Phase(), PhaseDetached)
	}

	// Phase 2: Tick through Detached → Attached → Loaded (each tick advances one state)
	for i := 0; i < 5; i++ {
		tickE2E(t, mon)
		if mon.state.Phase() >= PhaseLoaded {
			break
		}
	}
	if mon.state.Phase() < PhaseLoaded {
		t.Fatalf("after ticks: got phase %s, want >= Loaded", mon.state.Phase())
	}

	// Phase 3: Loaded — death count should now be readable on next tick
	tickE2E(t, mon)
	update := drainUpdate(t, mon)

	t.Logf("After loaded tick: deaths=%d, char=%q, status=%q",
		update.DeathCount, update.CharacterName, update.Status)

	if update.GameName != "Dark Souls III" {
		t.Errorf("expected game 'Dark Souls III', got %q", update.GameName)
	}

	if update.Status != "Loaded" {
		t.Errorf("expected status 'Loaded', got %q", update.Status)
	}

	// Verify save ID was created in the DB
	dt := deathTracker(mon)
	if dt.currentSaveID <= 0 {
		t.Errorf("expected currentSaveID > 0, got %d", dt.currentSaveID)
	}
}

// TestE2E_RouteTracker_PhaseTransitions validates the full
// Detached → Attached → Loaded → Running state machine.
func TestE2E_RouteTracker_PhaseTransitions(t *testing.T) {
	ops, reader := newRealOpsAndAttach(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	// Detach so the monitor starts fresh
	reader.Detach()

	// Use the actual routes directory (relative to this test's package dir)
	rt := NewRouteTracker("ds3", "ds3-glitchless-any-percent-e2e", "../../routes", tracker)
	mon := NewGameMonitor("ds3", ops, rt)
	initE2EMonitor(mon)

	// Initial state
	if mon.state.Phase() != PhaseDetached {
		t.Errorf("initial phase: got %s, want %s", mon.state.Phase(), PhaseDetached)
	}

	// Tick until the route is running (max 10 ticks to account for state transitions + save detection)
	var update DisplayUpdate
	for i := 0; i < 10; i++ {
		tickE2E(t, mon)
		// Drain update if available (early ticks may not produce one)
		select {
		case u := <-mon.displayCh:
			update = u
		default:
		}
		t.Logf("Tick %d: phase=%s, running=%v, status=%q, char=%q",
			i+1, mon.state.Phase(), rt.state.IsRunning(), update.Status, update.CharacterName)

		if rt.state.IsRunning() {
			break
		}
	}

	if !rt.state.IsRunning() {
		t.Fatalf("expected tracker to be running after ticks")
	}

	// Get a fresh update from a running tick
	tickE2E(t, mon)
	update = drainUpdate(t, mon)

	// Verify route is active
	if update.Route == nil {
		t.Fatal("expected Route to be non-nil")
	}

	if update.Status != "Tracking route" {
		t.Errorf("expected status 'Tracking route', got %q", update.Status)
	}

	// Verify save was detected before route started
	if rt.currentSaveID <= 0 {
		t.Errorf("route started but currentSaveID=%d (should be > 0)", rt.currentSaveID)
	}
	if update.CharacterName == "" {
		t.Error("route started but character name is empty")
	}

	t.Logf("Route running: route=%q, char=%q (Slot %d), saveID=%d",
		update.Route.RouteName, update.CharacterName, update.SaveSlotIndex, rt.currentSaveID)

	// One more tick to verify death count is readable in Running phase
	tickE2E(t, mon)
	update = drainUpdate(t, mon)

	t.Logf("Route tick: deaths=%d", update.DeathCount)
}

// TestE2E_DeathTracker_Slot255NotAccepted verifies that slot 255
// (uninitialized memory) does not cause a premature save detection.
func TestE2E_DeathTracker_Slot255NotAccepted(t *testing.T) {
	ops, reader := newRealOpsAndAttach(t)
	defer reader.Detach()

	// Read the actual slot value to see if it's valid
	slot, err := reader.ReadSaveSlotIndex()
	if err != nil {
		t.Skipf("ReadSaveSlotIndex failed (AOB not ready?): %v", err)
	}
	if slot == 255 {
		t.Skip("actual slot is 255 — cannot test rejection with this save state")
	}

	t.Logf("Real slot index is %d (not 255), state machine should accept it", slot)

	// Verify the monitor properly loads with a valid slot
	reader.Detach()
	tracker := newE2ETracker(t)
	mon := NewGameMonitor("ds3", ops, NewDeathTracker("ds3", tracker))
	initE2EMonitor(mon)

	for i := 0; i < 10; i++ {
		tickE2E(t, mon)
		// Drain update if available (early ticks may not produce one)
		select {
		case <-mon.displayCh:
		default:
		}
		dt := deathTracker(mon)
		if dt.saveDetected {
			break
		}
	}

	dt := deathTracker(mon)
	if !dt.saveDetected {
		t.Errorf("expected save to be detected with valid slot %d", slot)
	}
	if dt.currentSlotIdx == 255 {
		t.Error("tracker accepted slot 255 — should have been rejected")
	}
	t.Logf("Tracker detected save with slot %d", dt.currentSlotIdx)
}

// newE2ETracker creates an in-memory repository for e2e tracker tests.
func newE2ETracker(t *testing.T) *data.Repository {
	return newE2ERepo(t)
}

// deathTracker extracts the DeathTracker from a GameMonitor.
func deathTracker(mon *GameMonitor) *DeathTracker {
	return mon.tracker.(*DeathTracker)
}

// drainUpdate reads one DisplayUpdate from the monitor, failing if none is available.
func drainUpdate(t *testing.T, mon *GameMonitor) DisplayUpdate {
	t.Helper()
	select {
	case u := <-mon.displayCh:
		return u
	default:
		t.Fatal("expected a display update but channel was empty")
		return DisplayUpdate{} // unreachable
	}
}
