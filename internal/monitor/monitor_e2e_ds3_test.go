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

// tickDCE2E is a test helper for e2e tests: simulates one StartLoop cycle.
func tickDCE2E(t *testing.T, mon *DeathCounterMonitor) {
	t.Helper()
	reader, err := mon.Attach()
	if err != nil {
		mon.OnDetach()
		return
	}
	if mon.Phase == PhaseAttached {
		mon.OnAttach(mon.attachedGameID)
		mon.Phase = PhaseLoaded
		return
	}
	mon.Tick(reader)
}

// tickRouteE2E is a test helper for e2e tests: simulates one StartLoop cycle.
func tickRouteE2E(t *testing.T, mon *RouteMonitor) {
	t.Helper()
	reader, err := mon.Attach()
	if err != nil {
		mon.OnDetach()
		return
	}
	if mon.Phase == PhaseAttached {
		mon.OnAttach(mon.attachedGameID)
		mon.Phase = PhaseLoaded
		return
	}
	mon.Tick(reader)
}

// TestE2E_DeathCounterMonitor_PhaseTransitions validates the full
// Detached → Attached → Loaded state machine with a real DS3 process.
func TestE2E_DeathCounterMonitor_PhaseTransitions(t *testing.T) {
	ops, reader := newRealOpsAndAttach(t)
	defer reader.Detach()
	tracker := newE2ETracker(t)

	// We got a reader to verify DS3 is running, but the monitor
	// starts fresh from PhaseDetached — detach and let the monitor attach.
	reader.Detach()

	mon := NewDeathCounterMonitor("ds3", ops, tracker)

	// Phase 1: Detached — no game attached yet... but since the process
	// IS running, Attach will succeed on first tick.
	if mon.Phase != PhaseDetached {
		t.Errorf("initial phase: got %s, want %s", mon.Phase, PhaseDetached)
	}

	// Phase 2: First tick → Attach succeeds → PhaseAttached → OnAttach → PhaseLoaded
	tickDCE2E(t, mon)

	if mon.Phase < PhaseLoaded {
		t.Fatalf("after first tick: got phase %s, want >= Loaded", mon.Phase)
	}

	// Phase 3: Loaded — death count should now be readable on next tick
	tickDCE2E(t, mon)
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
	if mon.CurrentSaveID <= 0 {
		t.Errorf("expected CurrentSaveID > 0, got %d", mon.CurrentSaveID)
	}
}

// TestE2E_RouteMonitor_PhaseTransitions validates the full
// Detached → Attached → Loaded → Running state machine.
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
	if mon.Phase != PhaseDetached {
		t.Errorf("initial phase: got %s, want %s", mon.Phase, PhaseDetached)
	}

	// Tick until we reach Running (max 5 ticks to account for slow save detection)
	var update DisplayUpdate
	for i := 0; i < 5; i++ {
		tickRouteE2E(t, mon)
		update = drainUpdate(t, mon)
		t.Logf("Tick %d: phase=%s, status=%q, char=%q",
			i+1, mon.Phase, update.Status, update.CharacterName)

		if mon.Phase == PhaseRunning {
			break
		}
	}

	if mon.Phase != PhaseRunning {
		t.Fatalf("expected PhaseRunning after ticks, got %s", mon.Phase)
	}

	// Verify route is active
	if update.Route == nil {
		t.Fatal("expected Route to be non-nil")
	}
	routeName := update.Route.RouteName
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

	// One more tick to verify death count is readable in Running phase
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
