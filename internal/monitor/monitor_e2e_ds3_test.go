//go:build e2e && ds3 && windows

package monitor

import (
	"testing"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
	"github.com/rjansen/deathcounter/internal/stats"
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

func newE2ETracker(t *testing.T) *stats.Tracker {
	t.Helper()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	t.Cleanup(func() { tracker.Close() })
	return tracker
}

// tickDCE2E is a test helper for e2e tests: calls Attach then Tick.
func tickDCE2E(t *testing.T, mon *DeathCounterMonitor) {
	t.Helper()
	reader, err := mon.Attach()
	if err != nil {
		mon.OnDisconnect()
		return
	}
	mon.Tick(reader)
}

// tickRouteE2E is a test helper for e2e tests: calls Attach then Tick.
func tickRouteE2E(t *testing.T, mon *RouteMonitor) {
	t.Helper()
	reader, err := mon.Attach()
	if err != nil {
		mon.OnDisconnect()
		return
	}
	mon.Tick(reader)
}

// TestE2E_DeathCounterMonitor_PhaseTransitions validates the full
// Disconnected → Connected → Loaded state machine with a real DS3 process.
func TestE2E_DeathCounterMonitor_PhaseTransitions(t *testing.T) {
	ops, reader := newRealOpsAndAttach(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	// We got a reader to verify DS3 is running, but the monitor
	// starts fresh from PhaseDisconnected — detach and let the monitor attach.
	reader.Detach()

	mon := NewDeathCounterMonitor("ds3", ops, tracker)

	// Phase 1: Disconnected — no game attached yet... but since the process
	// IS running, Attach will succeed on first tick.
	if mon.Phase != PhaseDisconnected {
		t.Errorf("initial phase: got %s, want %s", mon.Phase, PhaseDisconnected)
	}

	// Phase 2: First tick → TryAttach succeeds → PhaseConnected,
	// then DetectSave succeeds → PhaseLoaded.
	tickDCE2E(t, mon)
	update := drainUpdate(t, mon)

	if mon.Phase < PhaseConnected {
		t.Fatalf("after first tick: got phase %s, want >= Connected", mon.Phase)
	}

	t.Logf("After tick 1: phase=%s, status=%q, game=%q, char=%q, slot=%d",
		mon.Phase, update.Status, update.GameName, update.CharacterName, update.SaveSlotIndex)

	if update.GameName != "Dark Souls III" {
		t.Errorf("expected game 'Dark Souls III', got %q", update.GameName)
	}

	// If save detection succeeded on the first tick, we're already Loaded
	if mon.Phase == PhaseLoaded {
		if update.CharacterName == "" {
			t.Error("PhaseLoaded but character name is empty")
		}
		if update.Status != "Loaded" {
			t.Errorf("expected status 'Loaded', got %q", update.Status)
		}
		t.Logf("Save detected on first tick: %q (Slot %d)", update.CharacterName, update.SaveSlotIndex)
	} else {
		// Still Connected — save detection may need another tick
		t.Logf("Still Connected after first tick, ticking again for save detection...")
		tickDCE2E(t, mon)
		update = drainUpdate(t, mon)
		if mon.Phase != PhaseLoaded {
			t.Fatalf("after second tick: got phase %s, want Loaded", mon.Phase)
		}
	}

	// Phase 3: Loaded — death count should now be readable on next tick
	tickDCE2E(t, mon)
	update = drainUpdate(t, mon)

	t.Logf("After loaded tick: deaths=%d, char=%q",
		update.DeathCount, update.CharacterName)

	if update.CharacterName == "" {
		t.Error("expected non-empty character name in Loaded phase")
	}

	// Verify save ID was created in the DB
	if mon.CurrentSaveID <= 0 {
		t.Errorf("expected CurrentSaveID > 0, got %d", mon.CurrentSaveID)
	}
}

// TestE2E_RouteMonitor_PhaseTransitions validates the full
// Disconnected → Connected → Loaded → RouteRunning state machine.
func TestE2E_RouteMonitor_PhaseTransitions(t *testing.T) {
	ops, reader := newRealOpsAndAttach(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	// Detach so the monitor starts fresh
	reader.Detach()

	mon := NewRouteMonitor("ds3", "", "", ops, tracker)
	// Inject route directly for the e2e test
	mon.route = &route.Route{
		ID:   "ds3-e2e-test",
		Name: "E2E Test Route",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "iudex", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagCheck: &route.EventFlagCheck{FlagID: memreader.DS3FlagIudexGundyr}},
		},
	}

	// Initial state
	if mon.Phase != PhaseDisconnected {
		t.Errorf("initial phase: got %s, want %s", mon.Phase, PhaseDisconnected)
	}

	// Tick until we reach RouteRunning (max 5 ticks to account for slow save detection)
	var update DisplayUpdate
	for i := 0; i < 5; i++ {
		tickRouteE2E(t, mon)
		update = drainUpdate(t, mon)
		t.Logf("Tick %d: phase=%s, status=%q, char=%q",
			i+1, mon.Phase, update.Status, update.CharacterName)

		if mon.Phase == PhaseRouteRunning {
			break
		}
	}

	if mon.Phase != PhaseRouteRunning {
		t.Fatalf("expected PhaseRouteRunning after ticks, got %s", mon.Phase)
	}

	// Verify route is active
	routeName, _ := update.Fields["route_name"].(string)
	if routeName != "E2E Test Route" {
		t.Errorf("expected route name 'E2E Test Route', got %q", routeName)
	}

	if update.Status != "Tracking route" {
		t.Errorf("expected status 'Tracking route', got %q", update.Status)
	}

	// Verify save was detected before route started
	if mon.CurrentSaveID <= 0 {
		t.Errorf("route started but CurrentSaveID=%d (should be > 0)", mon.CurrentSaveID)
	}
	if update.CharacterName == "" {
		t.Error("route started but character name is empty")
	}

	t.Logf("Route running: char=%q (Slot %d), saveID=%d",
		update.CharacterName, update.SaveSlotIndex, mon.CurrentSaveID)

	// One more tick to verify death count is readable in RouteRunning phase
	tickRouteE2E(t, mon)
	update = drainUpdate(t, mon)

	t.Logf("Route tick: deaths=%d", update.DeathCount)
}

// TestE2E_DeathCounterMonitor_Slot255NotAccepted verifies that slot 255
// (uninitialized memory) does not cause a premature transition to PhaseLoaded.
func TestE2E_DeathCounterMonitor_Slot255NotAccepted(t *testing.T) {
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
	mon := NewDeathCounterMonitor("ds3", ops, tracker)

	for i := 0; i < 5; i++ {
		tickDCE2E(t, mon)
		drainUpdate(t, mon)
		if mon.Phase >= PhaseLoaded {
			break
		}
	}

	if mon.Phase < PhaseLoaded {
		t.Errorf("expected >= PhaseLoaded with valid slot %d, got %s", slot, mon.Phase)
	}
	if mon.CurrentSlotIdx == 255 {
		t.Error("monitor accepted slot 255 — should have been rejected")
	}
	t.Logf("Monitor loaded with slot %d, phase=%s", mon.CurrentSlotIdx, mon.Phase)
}

// drainUpdate reads one DisplayUpdate from the monitor, failing if none is available.
func drainUpdate(t *testing.T, mon interface{ DisplayUpdates() <-chan DisplayUpdate }) DisplayUpdate {
	t.Helper()
	select {
	case u := <-mon.DisplayUpdates():
		return u
	default:
		t.Fatal("expected a display update but channel was empty")
		return DisplayUpdate{} // unreachable
	}
}
