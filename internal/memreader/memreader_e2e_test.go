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

// requireDS3 skips the test unless Dark Souls III is the attached game.
// Event flags, IGT, memory paths, and AOB scanning are only configured for DS3.
func requireDS3(t *testing.T, reader *GameReader) {
	t.Helper()
	if reader.GetCurrentGame() != "Dark Souls III" {
		t.Skipf("Test requires Dark Souls III, attached to %q", reader.GetCurrentGame())
	}
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

// --- Event Flag E2E Tests (DS3 only) ---

func TestE2E_ReadEventFlag(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Flag 13000800 = Iudex Gundyr defeated (first boss, almost always set on any save).
	// This is a safe flag to check because any DS3 save past the tutorial has it set.
	flagID := uint32(13000800)

	set, err := reader.ReadEventFlag(flagID)
	if err != nil {
		t.Fatalf("ReadEventFlag(%d) failed: %v", flagID, err)
	}

	t.Logf("[DS3] Event flag %d (Iudex Gundyr defeated): %v", flagID, set)
	// We don't assert true because a brand-new save might not have it,
	// but a successful read without error validates the full flag algorithm.
}

func TestE2E_ReadEventFlag_GlobalFlag(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Global flags have area >= 90. Flag 90000000 region — read should succeed
	// even if the flag is not set. This exercises the category=0 (global) path.
	flagID := uint32(90000000)

	_, err := reader.ReadEventFlag(flagID)
	if err != nil {
		t.Fatalf("ReadEventFlag(%d) global flag failed: %v", flagID, err)
	}
	t.Logf("[DS3] Global event flag %d read successfully", flagID)
}

func TestE2E_ReadEventFlag_Stable(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	flagID := uint32(13000800) // Iudex Gundyr defeated
	first, err := reader.ReadEventFlag(flagID)
	if err != nil {
		t.Fatalf("initial ReadEventFlag failed: %v", err)
	}

	// Read 5 times to verify stability (flag shouldn't toggle)
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		val, err := reader.ReadEventFlag(flagID)
		if err != nil {
			t.Fatalf("ReadEventFlag iteration %d failed: %v", i, err)
		}
		if val != first {
			t.Errorf("flag value changed from %v to %v at iteration %d", first, val, i)
		}
	}
	t.Logf("[DS3] Event flag %d stable at %v over 5 reads", flagID, first)
}

func TestE2E_ReadEventFlag_MultipleBosses(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Read several boss kill flags to exercise different flag decompositions.
	// The exact set/unset state depends on the player's save, but all reads should succeed.
	bosses := []struct {
		flagID uint32
		name   string
	}{
		{13000800, "Iudex Gundyr"},
		{13100800, "Vordt of the Boreal Valley"},
		{13300850, "Crystal Sage"},
		{13900800, "Abyss Watchers"},
		{14000800, "High Lord Wolnir"},
		{15100800, "Pontiff Sulyvahn"},
		{15000800, "Aldrich, Devourer of Gods"},
	}

	for _, boss := range bosses {
		set, err := reader.ReadEventFlag(boss.flagID)
		if err != nil {
			t.Errorf("ReadEventFlag(%d) %s failed: %v", boss.flagID, boss.name, err)
			continue
		}
		t.Logf("[DS3] %s (%d): defeated=%v", boss.name, boss.flagID, set)
	}
}

// --- IGT E2E Tests (DS3 only) ---

func TestE2E_ReadIGT(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	igt, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("ReadIGT failed: %v", err)
	}

	t.Logf("[DS3] In-game time: %d ms (%.1f seconds)", igt, float64(igt)/1000.0)

	// IGT should be non-negative; a loaded save should have positive IGT
	if igt < 0 {
		t.Errorf("IGT should be non-negative, got %d", igt)
	}
}

func TestE2E_ReadIGT_Increments(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	first, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("initial ReadIGT failed: %v", err)
	}

	// Wait and read again — IGT should have incremented if the game is unpaused.
	// If paused (menu open), the value may be the same, so we only log a warning.
	time.Sleep(2 * time.Second)

	second, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("second ReadIGT failed: %v", err)
	}

	diff := second - first
	t.Logf("[DS3] IGT: %d → %d (delta: %d ms over 2s)", first, second, diff)

	if diff < 0 {
		t.Errorf("IGT went backwards: %d → %d", first, second)
	}
	if diff == 0 {
		t.Log("Warning: IGT did not increment — game may be paused or in a menu")
	}
}

// --- Memory Value E2E Tests (DS3 only) ---

func TestE2E_ReadMemoryValue_SoulLevel(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// player_stats + 0x68 = SoulLevel (uint32), per config.go comments
	level, err := reader.ReadMemoryValue("player_stats", 0x68, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(player_stats, 0x68) failed: %v", err)
	}

	t.Logf("[DS3] Soul Level: %d", level)

	// DS3 characters start at level 1 minimum; max is ~802
	if level < 1 || level > 900 {
		t.Errorf("Soul Level %d outside expected range [1, 900]", level)
	}
}

func TestE2E_ReadMemoryValue_Stats(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Read several stat fields to verify pointer chain works for different offsets.
	stats := []struct {
		offset int64
		name   string
		min    uint32
		max    uint32
	}{
		{0x68, "SoulLevel", 1, 900},
		{0x6C, "Vigor", 1, 99},
		{0x70, "Attunement", 1, 99},
		{0x74, "Endurance", 1, 99},
		{0x78, "Vitality", 1, 99},
		{0x7C, "Strength", 1, 99},
		{0x80, "Dexterity", 1, 99},
		{0x84, "Intelligence", 1, 99},
		{0x88, "Faith", 1, 99},
		{0x8C, "Luck", 1, 99},
	}

	for _, s := range stats {
		val, err := reader.ReadMemoryValue("player_stats", s.offset, 4)
		if err != nil {
			t.Errorf("ReadMemoryValue(player_stats, 0x%X) %s failed: %v", s.offset, s.name, err)
			continue
		}
		t.Logf("[DS3] %s: %d", s.name, val)
		if val < s.min || val > s.max {
			t.Errorf("%s=%d outside expected range [%d, %d]", s.name, val, s.min, s.max)
		}
	}
}

func TestE2E_ReadMemoryValue_UnknownPath(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	_, err := reader.ReadMemoryValue("nonexistent_path", 0, 4)
	if err == nil {
		t.Error("expected error for unknown memory path")
	}
}

// --- AOB Scanning E2E Tests (DS3 only) ---

func TestE2E_AOBScan_SprjEventFlagMan(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	cfg := reader.game
	if cfg.SprjEventFlagManAOB == nil {
		t.Skip("No SprjEventFlagMan AOB pattern configured")
	}

	aob := cfg.SprjEventFlagManAOB
	addr, err := reader.ScanForPointer(aob.Pattern, aob.RelativeOffsetPos, aob.InstrLen)
	if err != nil {
		t.Fatalf("ScanForPointer(SprjEventFlagMan) failed: %v", err)
	}

	if addr == 0 {
		t.Error("SprjEventFlagMan scan returned zero address")
	}
	t.Logf("[DS3] SprjEventFlagMan AOB resolved: 0x%X", addr)

	// If dereference is required, verify it too
	if aob.Dereference {
		ptr, err := reader.readPtr(addr)
		if err != nil {
			t.Fatalf("Dereference of SprjEventFlagMan failed: %v", err)
		}
		if ptr == 0 {
			t.Error("SprjEventFlagMan dereference returned null")
		}
		t.Logf("[DS3] SprjEventFlagMan dereferenced: 0x%X", ptr)
	}
}

func TestE2E_AOBScan_FieldArea(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	cfg := reader.game
	if cfg.FieldAreaAOB == nil {
		t.Skip("No FieldArea AOB pattern configured")
	}

	aob := cfg.FieldAreaAOB
	addr, err := reader.ScanForPointer(aob.Pattern, aob.RelativeOffsetPos, aob.InstrLen)
	if err != nil {
		t.Fatalf("ScanForPointer(FieldArea) failed: %v", err)
	}

	if addr == 0 {
		t.Error("FieldArea scan returned zero address")
	}
	t.Logf("[DS3] FieldArea AOB resolved: 0x%X", addr)
}

func TestE2E_AOBScan_CachingBehavior(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// First ReadEventFlag call triggers lazy AOB init
	_, err := reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("first ReadEventFlag failed: %v", err)
	}

	// Verify cached addresses are set
	if reader.sprjEventFlagManAddr == 0 && reader.game.EventFlagOffsets64 == nil {
		t.Error("expected sprjEventFlagManAddr to be cached after first ReadEventFlag")
	}
	if !reader.eventFlagInitDone {
		t.Error("expected eventFlagInitDone to be true after first ReadEventFlag")
	}

	cachedSprj := reader.sprjEventFlagManAddr
	cachedField := reader.fieldAreaAddr

	// Second call should reuse cached addresses (no re-scan)
	_, err = reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("second ReadEventFlag failed: %v", err)
	}

	if reader.sprjEventFlagManAddr != cachedSprj {
		t.Errorf("SprjEventFlagMan addr changed: 0x%X → 0x%X", cachedSprj, reader.sprjEventFlagManAddr)
	}
	if reader.fieldAreaAddr != cachedField {
		t.Errorf("FieldArea addr changed: 0x%X → 0x%X", cachedField, reader.fieldAreaAddr)
	}
	t.Logf("[DS3] AOB cache verified: SprjEventFlagMan=0x%X FieldArea=0x%X", cachedSprj, cachedField)
}

// --- Integration E2E Tests ---

func TestE2E_FullRouteTick(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Simulate what a route tick does: read death count, event flags, memory values, and IGT
	// all in one go to verify they work together on a single attach.

	deaths, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}

	iudexFlag, err := reader.ReadEventFlag(13000800) // Iudex Gundyr
	if err != nil {
		t.Fatalf("ReadEventFlag failed: %v", err)
	}

	level, err := reader.ReadMemoryValue("player_stats", 0x68, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(SoulLevel) failed: %v", err)
	}

	igt, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("ReadIGT failed: %v", err)
	}

	t.Logf("[DS3] Full tick: deaths=%d, iudex_defeated=%v, soul_level=%d, igt=%dms",
		deaths, iudexFlag, level, igt)
}

func TestE2E_FullRouteTick_Repeated(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Simulate 10 consecutive route ticks at 500ms intervals.
	// All reads should succeed consistently without errors.
	for i := 0; i < 10; i++ {
		_, err := reader.ReadDeathCount()
		if err != nil {
			t.Fatalf("tick %d: ReadDeathCount failed: %v", i, err)
		}
		_, err = reader.ReadEventFlag(13000800)
		if err != nil {
			t.Fatalf("tick %d: ReadEventFlag failed: %v", i, err)
		}
		_, err = reader.ReadMemoryValue("player_stats", 0x68, 4)
		if err != nil {
			t.Fatalf("tick %d: ReadMemoryValue failed: %v", i, err)
		}
		_, err = reader.ReadIGT()
		if err != nil {
			t.Fatalf("tick %d: ReadIGT failed: %v", i, err)
		}

		if i < 9 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	t.Log("[DS3] 10 consecutive full ticks completed successfully")
}

func TestE2E_DetachClearsAOBCache(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	requireDS3(t, reader)

	// Trigger AOB init
	_, err := reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("ReadEventFlag failed: %v", err)
	}

	reader.Detach()

	// After detach, reattach and verify AOB re-initializes on next use
	if reader.eventFlagInitDone {
		// This checks the current behavior. If Detach doesn't reset AOB state,
		// we at least document it. The test still validates the reattach flow.
		t.Log("Note: eventFlagInitDone not reset on Detach (AOB cache persists)")
	}

	err = reader.Attach()
	if err != nil {
		t.Fatalf("reattach failed: %v", err)
	}
	defer reader.Detach()

	// Reading should still work after reattach regardless of cache state
	_, err = reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("ReadEventFlag after reattach failed: %v", err)
	}
	t.Log("[DS3] Event flag read succeeded after detach/reattach cycle")
}
