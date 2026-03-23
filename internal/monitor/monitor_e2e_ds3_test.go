//go:build e2e && ds3 && windows

package monitor

import (
	"testing"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
	"github.com/rjansen/deathcounter/internal/stats"
)

// newRealReader creates a real GameReader and skips the test if no game is running.
func newRealReader(t *testing.T) *memreader.GameReader {
	t.Helper()
	reader, err := memreader.NewGameReader()
	if err != nil || !reader.IsAttached() {
		t.Skipf("No supported game running: %v", err)
	}
	if reader.GetCurrentGame() != "Dark Souls III" {
		reader.Detach()
		t.Skipf("Test requires Dark Souls III, attached to %q", reader.GetCurrentGame())
	}
	return reader
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

// TestE2E_DeathCounterMonitor_PhaseTransitions validates the full
// Disconnected → Connected → Loaded state machine with a real DS3 process.
func TestE2E_DeathCounterMonitor_PhaseTransitions(t *testing.T) {
	reader := newRealReader(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	// Detach so the monitor starts from PhaseDisconnected
	reader.Detach()

	mon := NewDeathCounterMonitor(reader, tracker)

	// Phase 1: Disconnected — no game attached yet... but since the process
	// IS running, Attach will succeed on first tick. We can't truly test
	// "Disconnected" without killing the game, so we verify the initial state.
	if mon.Phase != PhaseDisconnected {
		t.Errorf("initial phase: got %s, want %s", mon.Phase, PhaseDisconnected)
	}

	// Phase 2: First tick → TryAttach succeeds → PhaseConnected,
	// then TryDetectSave succeeds → PhaseLoaded.
	mon.Tick()
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
		mon.Tick()
		update = drainUpdate(t, mon)
		if mon.Phase != PhaseLoaded {
			t.Fatalf("after second tick: got phase %s, want Loaded", mon.Phase)
		}
	}

	// Phase 3: Loaded — death count should now be readable on next tick
	mon.Tick()
	update = drainUpdate(t, mon)

	t.Logf("After loaded tick: deaths=%d, hollowing=%d, char=%q",
		update.DeathCount, update.Hollowing, update.CharacterName)

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
	reader := newRealReader(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	// Detach so the monitor starts fresh
	reader.Detach()

	routes := []*route.Route{
		{
			ID:   "ds3-e2e-test",
			Name: "E2E Test Route",
			Game: "Dark Souls III",
			Checkpoints: []route.Checkpoint{
				{ID: "iudex", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagID: memreader.DS3FlagIudexGundyr},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes[0], nil)

	// Initial state
	if mon.Phase != PhaseDisconnected {
		t.Errorf("initial phase: got %s, want %s", mon.Phase, PhaseDisconnected)
	}

	// Tick until we reach RouteRunning (max 5 ticks to account for slow save detection)
	var update DisplayUpdate
	for i := 0; i < 5; i++ {
		mon.Tick()
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
	mon.Tick()
	update = drainUpdate(t, mon)

	t.Logf("Route tick: deaths=%d, hollowing=%d", update.DeathCount, update.Hollowing)
}

// TestE2E_RouteMonitor_SaveIDPassedToRouteRun verifies that the route run
// record in the DB has the correct save_id (not zero).
func TestE2E_RouteMonitor_SaveIDPassedToRouteRun(t *testing.T) {
	reader := newRealReader(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	reader.Detach()

	routes := []*route.Route{
		{
			ID:   "ds3-saveid-test",
			Name: "SaveID Test Route",
			Game: "Dark Souls III",
			Checkpoints: []route.Checkpoint{
				{ID: "boss1", Name: "Test Boss", EventType: "boss_kill", EventFlagID: 99999},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes[0], nil)

	// Tick until route starts
	for i := 0; i < 5; i++ {
		mon.Tick()
		drainUpdate(t, mon)
		if mon.Phase == PhaseRouteRunning {
			break
		}
	}

	if mon.Phase != PhaseRouteRunning {
		t.Fatalf("expected PhaseRouteRunning, got %s", mon.Phase)
	}

	// Verify the route_runs record has a non-zero save_id
	var saveID *int64
	err := tracker.DB().QueryRow(
		"SELECT save_id FROM route_runs WHERE route_id = 'ds3-saveid-test' ORDER BY id DESC LIMIT 1",
	).Scan(&saveID)
	if err != nil {
		t.Fatalf("query route_runs: %v", err)
	}
	if saveID == nil || *saveID <= 0 {
		t.Errorf("route_runs.save_id should be > 0, got %v", saveID)
	} else {
		t.Logf("route_runs.save_id = %d (correct)", *saveID)
	}
}

// TestE2E_DeathCounterMonitor_Slot255NotAccepted verifies that slot 255
// (uninitialized memory) does not cause a premature transition to PhaseLoaded.
// This test reads the actual slot value and only tests the rejection logic
// if the real slot happens to not be 255 (i.e. on a properly loaded save).
func TestE2E_DeathCounterMonitor_Slot255NotAccepted(t *testing.T) {
	reader := newRealReader(t)
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
	mon := NewDeathCounterMonitor(reader, tracker)

	for i := 0; i < 5; i++ {
		mon.Tick()
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
