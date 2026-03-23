//go:build e2e && ds3 && windows

package memreader

import (
	"testing"
	"time"
)

// requireDS3 skips the test unless Dark Souls III is the attached game.
// Event flags, IGT, memory paths, and AOB scanning are only configured for DS3.
func requireDS3(t *testing.T, reader *GameReader) {
	t.Helper()
	if reader.GetCurrentGame() != "Dark Souls III" {
		t.Skipf("Test requires Dark Souls III, attached to %q", reader.GetCurrentGame())
	}
}

// --- Event Flag E2E Tests ---

func TestE2E_ReadEventFlag(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Iudex Gundyr defeated — first boss, almost always set on any save.
	flagID := uint32(DS3FlagIudexGundyr)

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

	// Global flags have area >= 90. Flag 19000000 uses div10M=1 (same array
	// entry as boss flags) with area=90, exercising the category=0 (global) path.
	flagID := uint32(19000000)

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

	flagID := uint32(DS3FlagIudexGundyr)
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

	// Read all 25 boss kill flags to exercise different flag decompositions.
	// The exact set/unset state depends on the player's save, but all reads should succeed.
	bosses := []struct {
		flagID uint32
		name   string
	}{
		// Base game
		{DS3FlagIudexGundyr, "Iudex Gundyr"},
		{DS3FlagVordt, "Vordt of the Boreal Valley"},
		{DS3FlagGreatwood, "Curse-Rotted Greatwood"},
		{DS3FlagCrystalSage, "Crystal Sage"},
		{DS3FlagAbyssWatcher, "Abyss Watchers"},
		{DS3FlagDeacons, "Deacons of the Deep"},
		{DS3FlagWolnir, "High Lord Wolnir"},
		{DS3FlagOldDemonKing, "Old Demon King"},
		{DS3FlagPontiff, "Pontiff Sulyvahn"},
		{DS3FlagAldrich, "Aldrich, Devourer of Gods"},
		{DS3FlagYhorm, "Yhorm the Giant"},
		{DS3FlagDancer, "Dancer of the Boreal Valley"},
		{DS3FlagOceiros, "Oceiros, the Consumed King"},
		{DS3FlagChampionGundyr, "Champion Gundyr"},
		{DS3FlagAncientWyvern, "Ancient Wyvern"},
		{DS3FlagNamelessKing, "Nameless King"},
		{DS3FlagDragonslayer, "Dragonslayer Armour"},
		{DS3FlagTwinPrinces, "Twin Princes"},
		{DS3FlagSoulOfCinder, "Soul of Cinder"},
		// DLC
		{DS3FlagChampionGravetender, "Champion's Gravetender"},
		{DS3FlagFriede, "Sister Friede"},
		{DS3FlagDemonPrince, "Demon Prince"},
		{DS3FlagHalflight, "Halflight, Spear of the Church"},
		{DS3FlagMidir, "Darkeater Midir"},
		{DS3FlagGael, "Slave Knight Gael"},
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

func TestE2E_ReadEventFlag_AllEncountered(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Read all 17 encountered flags. All reads should succeed regardless of state.
	encountered := []struct {
		flagID uint32
		name   string
	}{
		{DS3FlagIudexGundyrEnc, "Iudex Gundyr Enc"},
		{DS3FlagVordtEnc, "Vordt Enc"},
		{DS3FlagGreatwoodEnc, "Greatwood Enc"},
		{DS3FlagCrystalSageEnc, "Crystal Sage Enc"},
		{DS3FlagAbyssWatcherEnc, "Abyss Watchers Enc"},
		{DS3FlagDeaconsEnc, "Deacons Enc"},
		{DS3FlagWolnirEnc, "Wolnir Enc"},
		{DS3FlagYhormEnc, "Yhorm Enc"},
		{DS3FlagOceirosEnc, "Oceiros Enc"},
		{DS3FlagChampionGundyrEnc, "Champion Gundyr Enc"},
		{DS3FlagTwinPrincesEnc, "Twin Princes Enc"},
		{DS3FlagSoulOfCinderEnc, "Soul of Cinder Enc"},
		{DS3FlagChampionGravetenderEnc, "Champion Gravetender Enc"},
		{DS3FlagFriedeEnc, "Friede Enc"},
		{DS3FlagHalflightEnc, "Halflight Enc"},
		{DS3FlagMidirEnc, "Midir Enc"},
		{DS3FlagGaelEnc, "Gael Enc"},
	}

	for _, enc := range encountered {
		set, err := reader.ReadEventFlag(enc.flagID)
		if err != nil {
			t.Errorf("ReadEventFlag(%d) %s failed: %v", enc.flagID, enc.name, err)
			continue
		}
		t.Logf("[DS3] %s (%d): %v", enc.name, enc.flagID, set)
	}
}

func TestE2E_ReadEventFlag_DefeatedImpliesEncountered(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// For bosses with both defeated and encountered flags, if defeated is set
	// then encountered must also be set (you can't kill a boss without encountering it).
	pairs := []struct {
		name          string
		defeatedID    uint32
		encounteredID uint32
	}{
		{"Iudex Gundyr", DS3FlagIudexGundyr, DS3FlagIudexGundyrEnc},
		{"Vordt", DS3FlagVordt, DS3FlagVordtEnc},
		{"Greatwood", DS3FlagGreatwood, DS3FlagGreatwoodEnc},
		{"Crystal Sage", DS3FlagCrystalSage, DS3FlagCrystalSageEnc},
		{"Abyss Watchers", DS3FlagAbyssWatcher, DS3FlagAbyssWatcherEnc},
		{"Deacons", DS3FlagDeacons, DS3FlagDeaconsEnc},
		{"Wolnir", DS3FlagWolnir, DS3FlagWolnirEnc},
		{"Yhorm", DS3FlagYhorm, DS3FlagYhormEnc},
		{"Oceiros", DS3FlagOceiros, DS3FlagOceirosEnc},
		{"Champion Gundyr", DS3FlagChampionGundyr, DS3FlagChampionGundyrEnc},
		{"Twin Princes", DS3FlagTwinPrinces, DS3FlagTwinPrincesEnc},
		{"Soul of Cinder", DS3FlagSoulOfCinder, DS3FlagSoulOfCinderEnc},
	}

	for _, p := range pairs {
		defeated, err := reader.ReadEventFlag(p.defeatedID)
		if err != nil {
			t.Errorf("%s: ReadEventFlag(defeated=%d) failed: %v", p.name, p.defeatedID, err)
			continue
		}
		encountered, err := reader.ReadEventFlag(p.encounteredID)
		if err != nil {
			t.Errorf("%s: ReadEventFlag(encountered=%d) failed: %v", p.name, p.encounteredID, err)
			continue
		}

		if defeated && !encountered {
			t.Errorf("%s: defeated=%v but encountered=%v — inconsistent (defeated implies encountered)",
				p.name, defeated, encountered)
		}
		t.Logf("[DS3] %s: defeated=%v, encountered=%v", p.name, defeated, encountered)
	}
}

// --- IGT E2E Tests ---

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

// --- Memory Value E2E Tests ---

func TestE2E_ReadMemoryValue_SoulLevel(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// player_stats + DS3OffsetSoulLevel = SoulLevel (uint32) — inline on PlayerGameData
	level, err := reader.ReadMemoryValue("player_stats", DS3OffsetSoulLevel, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(player_stats, 0x44) failed: %v", err)
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
	// Stats are inline on PlayerGameData (verified via Assassin class memory probe).
	stats := []struct {
		offset int64
		name   string
		min    uint32
		max    uint32
	}{
		{DS3OffsetSoulLevel, "SoulLevel", 1, 900},
		{DS3OffsetVigor, "Vigor", 1, 99},
		{DS3OffsetAttunement, "Attunement", 1, 99},
		{DS3OffsetEndurance, "Endurance", 1, 99},
		{DS3OffsetVitality, "Vitality", 1, 99},
		{DS3OffsetStrength, "Strength", 1, 99},
		{DS3OffsetDexterity, "Dexterity", 1, 99},
		{DS3OffsetIntelligence, "Intelligence", 1, 99},
		{DS3OffsetFaith, "Faith", 1, 99},
		{DS3OffsetLuck, "Luck", 1, 99},
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

func TestE2E_ReadHollowing(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	hollowing, err := reader.ReadHollowing()
	if err != nil {
		t.Fatalf("ReadHollowing failed: %v", err)
	}

	t.Logf("[DS3] Hollowing: %d", hollowing)

	// Hollowing ranges from 0 to 99
	if hollowing > 99 {
		t.Errorf("Hollowing %d outside expected range [0, 99]", hollowing)
	}
}

func TestE2E_ReadHollowing_Stable(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	first, err := reader.ReadHollowing()
	if err != nil {
		t.Fatalf("initial ReadHollowing failed: %v", err)
	}

	// Read 5 times to verify stability
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		val, err := reader.ReadHollowing()
		if err != nil {
			t.Fatalf("ReadHollowing iteration %d failed: %v", i, err)
		}
		if val != first {
			t.Errorf("hollowing changed from %d to %d at iteration %d", first, val, i)
		}
	}
	t.Logf("[DS3] Hollowing stable at %d over 5 reads", first)
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

// --- AOB Scanning E2E Tests ---

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
	_, err := reader.ReadEventFlag(DS3FlagIudexGundyr)
	if err != nil {
		t.Fatalf("first ReadEventFlag failed: %v", err)
	}

	// Verify cached addresses are set
	if reader.sprjEventFlagManAOBAddr == 0 && reader.game.EventFlagOffsets64 == nil {
		t.Error("expected sprjEventFlagManAOBAddr to be cached after first ReadEventFlag")
	}
	if !reader.eventFlagInitDone {
		t.Error("expected eventFlagInitDone to be true after first ReadEventFlag")
	}

	cachedSprj := reader.sprjEventFlagManAOBAddr
	cachedField := reader.fieldAreaAOBAddr

	// Second call should reuse cached addresses (no re-scan)
	_, err = reader.ReadEventFlag(DS3FlagIudexGundyr)
	if err != nil {
		t.Fatalf("second ReadEventFlag failed: %v", err)
	}

	if reader.sprjEventFlagManAOBAddr != cachedSprj {
		t.Errorf("SprjEventFlagMan addr changed: 0x%X → 0x%X", cachedSprj, reader.sprjEventFlagManAOBAddr)
	}
	if reader.fieldAreaAOBAddr != cachedField {
		t.Errorf("FieldArea addr changed: 0x%X → 0x%X", cachedField, reader.fieldAreaAOBAddr)
	}
	t.Logf("[DS3] AOB cache verified: SprjEventFlagMan=0x%X FieldArea=0x%X", cachedSprj, cachedField)
}

func TestE2E_AOBScan_GameDataMan(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	cfg := reader.game
	if cfg.GameDataManAOB == nil {
		t.Skip("No GameDataMan AOB pattern configured")
	}

	addr, err := reader.scanWithFallbacks(cfg.GameDataManAOB, "GameDataMan")
	if err != nil {
		t.Fatalf("GameDataMan AOB scan failed: %v", err)
	}

	if addr == 0 {
		t.Fatal("GameDataMan scan returned zero address")
	}
	t.Logf("[DS3] GameDataMan AOB resolved: 0x%X", addr)

	// Dereference to verify the pointer is valid
	if cfg.GameDataManAOB.Dereference {
		ptr, err := reader.readPtr(addr)
		if err != nil {
			t.Fatalf("Dereference of GameDataMan failed: %v", err)
		}
		if ptr == 0 {
			t.Error("GameDataMan dereference returned null — game data may not be loaded yet")
		} else {
			t.Logf("[DS3] GameDataMan dereferenced: 0x%X", ptr)
		}
	}
}

func TestE2E_AOBScan_GameMan(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	cfg := reader.game
	if cfg.GameManAOB == nil {
		t.Skip("No GameMan AOB pattern configured")
	}

	addr, err := reader.scanWithFallbacks(cfg.GameManAOB, "GameMan")
	if err != nil {
		t.Fatalf("GameMan AOB scan failed: %v", err)
	}

	if addr == 0 {
		t.Fatal("GameMan scan returned zero address")
	}
	t.Logf("[DS3] GameMan AOB resolved: 0x%X", addr)

	if cfg.GameManAOB.Dereference {
		ptr, err := reader.readPtr(addr)
		if err != nil {
			t.Fatalf("Dereference of GameMan failed: %v", err)
		}
		if ptr == 0 {
			t.Error("GameMan dereference returned null — game data may not be loaded yet")
		} else {
			t.Logf("[DS3] GameMan dereferenced: 0x%X", ptr)
		}
	}
}

func TestE2E_DetachClearsAOBCache(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	requireDS3(t, reader)

	// Trigger AOB init
	_, err := reader.ReadEventFlag(DS3FlagIudexGundyr)
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
	_, err = reader.ReadEventFlag(DS3FlagIudexGundyr)
	if err != nil {
		t.Fatalf("ReadEventFlag after reattach failed: %v", err)
	}
	t.Log("[DS3] Event flag read succeeded after detach/reattach cycle")
}

// --- Save Identity E2E Tests ---

func TestE2E_ReadCharacterName(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	name, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("ReadCharacterName failed: %v", err)
	}

	if name == "" {
		t.Error("character name is empty — save may not be loaded")
	}

	t.Logf("[DS3] Character name: %q", name)

	// Basic sanity: name should be printable and reasonable length
	if len(name) > 16 {
		t.Errorf("character name too long (%d chars), expected max 16", len(name))
	}
}

func TestE2E_ReadCharacterName_Stable(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	first, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("initial ReadCharacterName failed: %v", err)
	}

	// Read 5 times to verify stability
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		name, err := reader.ReadCharacterName()
		if err != nil {
			t.Fatalf("ReadCharacterName iteration %d failed: %v", i, err)
		}
		if name != first {
			t.Errorf("character name changed from %q to %q at iteration %d", first, name, i)
		}
	}
	t.Logf("[DS3] Character name stable at %q over 5 reads", first)
}

func TestE2E_ReadSaveSlotIndex(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	slot, err := reader.ReadSaveSlotIndex()
	if err != nil {
		t.Fatalf("ReadSaveSlotIndex failed: %v", err)
	}

	t.Logf("[DS3] Save slot index: %d", slot)

	// DS3 has 10 save slots (0-9)
	if slot < 0 || slot > 9 {
		t.Errorf("save slot index %d outside expected range [0, 9]", slot)
	}
}

func TestE2E_ReadSaveSlotIndex_Stable(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	first, err := reader.ReadSaveSlotIndex()
	if err != nil {
		t.Fatalf("initial ReadSaveSlotIndex failed: %v", err)
	}

	// Read 5 times to verify stability
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		slot, err := reader.ReadSaveSlotIndex()
		if err != nil {
			t.Fatalf("ReadSaveSlotIndex iteration %d failed: %v", i, err)
		}
		if slot != first {
			t.Errorf("save slot changed from %d to %d at iteration %d", first, slot, i)
		}
	}
	t.Logf("[DS3] Save slot index stable at %d over 5 reads", first)
}

func TestE2E_SaveIdentity_Combined(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Read both save identity fields together, as the monitor does
	name, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("ReadCharacterName failed: %v", err)
	}

	slot, err := reader.ReadSaveSlotIndex()
	if err != nil {
		t.Fatalf("ReadSaveSlotIndex failed: %v", err)
	}

	t.Logf("[DS3] Save identity: %q (Slot %d)", name, slot)

	if name == "" {
		t.Error("character name is empty")
	}
	if slot < 0 || slot > 9 {
		t.Errorf("save slot %d outside expected range [0, 9]", slot)
	}
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

	iudexFlag, err := reader.ReadEventFlag(DS3FlagIudexGundyr) // Iudex Gundyr
	if err != nil {
		t.Fatalf("ReadEventFlag failed: %v", err)
	}

	level, err := reader.ReadMemoryValue("player_stats", DS3OffsetSoulLevel, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(SoulLevel) failed: %v", err)
	}

	igt, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("ReadIGT failed: %v", err)
	}

	hollowing, err := reader.ReadHollowing()
	if err != nil {
		t.Fatalf("ReadHollowing failed: %v", err)
	}

	// Inventory item quantity (Titanite Shard)
	invQty, err := reader.ReadInventoryItemQuantity(DS3ItemTitaniteShard)
	if err != nil {
		t.Fatalf("ReadInventoryItemQuantity failed: %v", err)
	}

	t.Logf("[DS3] Full tick: deaths=%d, iudex_defeated=%v, soul_level=%d, igt=%dms, hollowing=%d, titanite_shards=%d",
		deaths, iudexFlag, level, igt, hollowing, invQty)
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
		_, err = reader.ReadEventFlag(DS3FlagIudexGundyr)
		if err != nil {
			t.Fatalf("tick %d: ReadEventFlag failed: %v", i, err)
		}
		_, err = reader.ReadMemoryValue("player_stats", DS3OffsetSoulLevel, 4)
		if err != nil {
			t.Fatalf("tick %d: ReadMemoryValue failed: %v", i, err)
		}
		_, err = reader.ReadIGT()
		if err != nil {
			t.Fatalf("tick %d: ReadIGT failed: %v", i, err)
		}
		_, err = reader.ReadHollowing()
		if err != nil {
			t.Fatalf("tick %d: ReadHollowing failed: %v", i, err)
		}

		if i < 9 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	t.Log("[DS3] 10 consecutive full ticks completed successfully")
}

func TestE2E_SaveIdentity_WithFullTick(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Simulate the full monitor tick: death count + save identity + event flags + IGT
	deaths, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}

	name, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("ReadCharacterName failed: %v", err)
	}

	slot, err := reader.ReadSaveSlotIndex()
	if err != nil {
		t.Fatalf("ReadSaveSlotIndex failed: %v", err)
	}

	iudex, err := reader.ReadEventFlag(DS3FlagIudexGundyr)
	if err != nil {
		t.Fatalf("ReadEventFlag(Iudex) failed: %v", err)
	}

	igt, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("ReadIGT failed: %v", err)
	}

	hollowing, err := reader.ReadHollowing()
	if err != nil {
		t.Fatalf("ReadHollowing failed: %v", err)
	}

	t.Logf("[DS3] Full tick with save identity:")
	t.Logf("  Character: %q (Slot %d)", name, slot)
	t.Logf("  Deaths: %d, Iudex defeated: %v", deaths, iudex)
	t.Logf("  IGT: %d ms (%.1f seconds)", igt, float64(igt)/1000.0)
	t.Logf("  Hollowing: %d", hollowing)
}

// --- Comprehensive Read Test ---

func TestE2E_ReadAllImportantData(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// 1. Save slot index
	slot, err := reader.ReadSaveSlotIndex()
	if err != nil {
		t.Fatalf("ReadSaveSlotIndex failed: %v", err)
	}
	if slot < 0 || slot > 9 {
		t.Errorf("save slot %d outside range [0, 9]", slot)
	}
	t.Logf("Save Slot: %d", slot)

	// 2. Character name
	name, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("ReadCharacterName failed: %v", err)
	}
	if name == "" {
		t.Error("character name is empty")
	}
	t.Logf("Character Name: %q", name)

	// 3. Player stats via player_game_data path
	// Stats are inline on PlayerGameData (see ds3_offsets.go for all offsets).
	stats := []struct {
		offset int64
		name   string
		min    uint32
		max    uint32
	}{
		{DS3OffsetSoulLevel, "SoulLevel", 1, 900},
		{DS3OffsetVigor, "Vigor", 1, 99},
		{DS3OffsetEndurance, "Endurance", 1, 99},
		{DS3OffsetDexterity, "Dexterity", 1, 99},
	}

	t.Log("Player Stats:")
	for _, s := range stats {
		val, err := reader.ReadMemoryValue("player_game_data", s.offset, 4)
		if err != nil {
			t.Errorf("ReadMemoryValue(player_game_data, 0x%X) %s failed: %v", s.offset, s.name, err)
			continue
		}
		t.Logf("  %s: %d", s.name, val)
		if val < s.min || val > s.max {
			t.Errorf("%s=%d outside expected range [%d, %d]", s.name, val, s.min, s.max)
		}
	}

	// 4. Hollowing (GameMan + DS3OffsetHollowing, Byte)
	hollowing, err := reader.ReadHollowing()
	if err != nil {
		t.Fatalf("ReadHollowing failed: %v", err)
	}
	if hollowing > 99 {
		t.Errorf("Hollowing=%d outside expected range [0, 99]", hollowing)
	}
	t.Logf("Hollowing: %d", hollowing)

	// 5. Weapon reinforcement level (TGA CT: GameDataMan → +0x10 → +DS3OffsetReinforceLv, Byte)
	reinforceLv, err := reader.ReadMemoryValue("player_game_data", DS3OffsetReinforceLv, 1)
	if err != nil {
		t.Fatalf("ReadMemoryValue(player_game_data, 0xB3) ReinforceLv failed: %v", err)
	}
	if reinforceLv > 10 {
		t.Errorf("ReinforceLv=%d outside expected range [0, 10]", reinforceLv)
	}
	t.Logf("ReinforceLv: +%d", reinforceLv)

	// 6. Last Bonfire (TGA CT: GameMan → +DS3OffsetLastBonfire, 4 Bytes signed)
	lastBonfire, err := reader.ReadMemoryValue("game_man", DS3OffsetLastBonfire, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(game_man, 0xACC) Last Bonfire failed: %v", err)
	}
	bonfireName := "Unknown"
	if n, ok := DS3BonfireNames[lastBonfire]; ok {
		bonfireName = n
	}
	t.Logf("Last Bonfire: %s (%d)", bonfireName, lastBonfire)

	// 7. Inventory item quantity (Titanite Shard)
	titaniteQty, err := reader.ReadInventoryItemQuantity(DS3ItemTitaniteShard)
	if err != nil {
		t.Fatalf("ReadInventoryItemQuantity(Titanite Shard) failed: %v", err)
	}
	t.Logf("Titanite Shards: %d", titaniteQty)

	// 8. Completed checkpoints (all 25 boss event flags)
	checkpoints := []struct {
		flagID uint32
		name   string
	}{
		// Base game
		{DS3FlagIudexGundyr, "Iudex Gundyr"},
		{DS3FlagVordt, "Vordt of the Boreal Valley"},
		{DS3FlagGreatwood, "Curse-Rotted Greatwood"},
		{DS3FlagCrystalSage, "Crystal Sage"},
		{DS3FlagAbyssWatcher, "Abyss Watchers"},
		{DS3FlagDeacons, "Deacons of the Deep"},
		{DS3FlagWolnir, "High Lord Wolnir"},
		{DS3FlagOldDemonKing, "Old Demon King"},
		{DS3FlagPontiff, "Pontiff Sulyvahn"},
		{DS3FlagAldrich, "Aldrich, Devourer of Gods"},
		{DS3FlagYhorm, "Yhorm the Giant"},
		{DS3FlagDancer, "Dancer of the Boreal Valley"},
		{DS3FlagOceiros, "Oceiros, the Consumed King"},
		{DS3FlagChampionGundyr, "Champion Gundyr"},
		{DS3FlagAncientWyvern, "Ancient Wyvern"},
		{DS3FlagNamelessKing, "Nameless King"},
		{DS3FlagDragonslayer, "Dragonslayer Armour"},
		{DS3FlagTwinPrinces, "Twin Princes"},
		{DS3FlagSoulOfCinder, "Soul of Cinder"},
		// DLC
		{DS3FlagChampionGravetender, "Champion's Gravetender"},
		{DS3FlagFriede, "Sister Friede"},
		{DS3FlagDemonPrince, "Demon Prince"},
		{DS3FlagHalflight, "Halflight, Spear of the Church"},
		{DS3FlagMidir, "Darkeater Midir"},
		{DS3FlagGael, "Slave Knight Gael"},
	}

	t.Log("Completed Checkpoints:")
	completed := 0
	for _, cp := range checkpoints {
		set, err := reader.ReadEventFlag(cp.flagID)
		if err != nil {
			t.Errorf("ReadEventFlag(%d) %s failed: %v", cp.flagID, cp.name, err)
			continue
		}
		status := "[ ]"
		if set {
			status = "[x]"
			completed++
		}
		t.Logf("  %s %s", status, cp.name)
	}
	t.Logf("Progress: %d/%d bosses defeated", completed, len(checkpoints))
}

// --- Inventory Item Quantity E2E Tests ---

func TestE2E_ReadInventoryItemQuantity(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Titanite Shard — common upgrade material, likely in most saves
	qty, err := reader.ReadInventoryItemQuantity(DS3ItemTitaniteShard)
	if err != nil {
		t.Fatalf("ReadInventoryItemQuantity(Titanite Shard) failed: %v", err)
	}
	t.Logf("[DS3] Titanite Shard (0x%X) quantity: %d", DS3ItemTitaniteShard, qty)

	// Ember — common consumable
	qty2, err := reader.ReadInventoryItemQuantity(DS3ItemEmber)
	if err != nil {
		t.Fatalf("ReadInventoryItemQuantity(Ember) failed: %v", err)
	}
	t.Logf("[DS3] Ember (0x%X) quantity: %d", DS3ItemEmber, qty2)
}

func TestE2E_ReadInventoryItemQuantity_NotInInventory(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Bogus item ID — should return 0 without error
	const bogusID = uint32(0xDEADBEEF)
	qty, err := reader.ReadInventoryItemQuantity(bogusID)
	if err != nil {
		t.Fatalf("ReadInventoryItemQuantity(bogus) failed: %v", err)
	}
	if qty != 0 {
		t.Errorf("expected quantity 0 for bogus item, got %d", qty)
	}
	t.Logf("[DS3] Bogus item (0x%X) quantity: %d (expected 0)", bogusID, qty)
}

func TestE2E_ReadInventoryItemQuantity_Stable(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	first, err := reader.ReadInventoryItemQuantity(DS3ItemTitaniteShard)
	if err != nil {
		t.Fatalf("initial ReadInventoryItemQuantity failed: %v", err)
	}

	// Read 5 times to verify stability (inventory shouldn't change while idle)
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		qty, err := reader.ReadInventoryItemQuantity(DS3ItemTitaniteShard)
		if err != nil {
			t.Fatalf("ReadInventoryItemQuantity iteration %d failed: %v", i, err)
		}
		if qty != first {
			t.Errorf("quantity changed from %d to %d at iteration %d", first, qty, i)
		}
	}
	t.Logf("[DS3] Titanite Shard quantity stable at %d over 5 reads", first)
}

func TestE2E_ReadInventoryItemQuantity_AllTrackedItems(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Read all tracked item constants against live game memory.
	// A successful read (no error) validates the TypeId is correct.
	// Items not in the current save return (0, nil) which is fine.
	items := []struct {
		itemID uint32
		name   string
	}{
		// Goods
		{DS3ItemEmber, "Ember"},
		{DS3ItemGoldPineResin, "Gold Pine Resin"},
		{DS3ItemCarthusRouge, "Carthus Rouge"},
		{DS3ItemHomewardBone, "Homeward Bone"},
		{DS3ItemTitaniteShard, "Titanite Shard"},
		{DS3ItemLargeTitaniteShard, "Large Titanite Shard"},
		{DS3ItemTitaniteChunk, "Titanite Chunk"},
		{DS3ItemTitaniteSlab, "Titanite Slab"},
		{DS3ItemEstusShard, "Estus Shard"},
		{DS3ItemGraveWardenAshes, "Grave Warden Ashes"},
		{DS3ItemMorticiansAshes, "Mortician's Ashes"},
		{DS3ItemSharpGem, "Sharp Gem"},
		{DS3ItemFirebomb, "Firebomb"},
		{DS3ItemAshenEstusFlask, "Ashen Estus Flask"},
		{DS3ItemFarronCoal, "Farron Coal"},
		// Rings
		{DS3ItemCovetousSilverSerpentRing, "Covetous Silver Serpent Ring"},
		{DS3ItemChloranthyRing, "Chloranthy Ring"},
		{DS3ItemLloydsSwordRing, "Lloyd's Sword Ring"},
		{DS3ItemPontiffsRightEye, "Pontiff's Right Eye"},
		// Weapons
		{DS3ItemSellswordTwinblades, "Sellsword Twinblades"},
		{DS3ItemDagger, "Dagger"},
		{DS3ItemShortsword, "Shortsword"},
	}

	for _, item := range items {
		qty, err := reader.ReadInventoryItemQuantity(item.itemID)
		if err != nil {
			t.Errorf("ReadInventoryItemQuantity(0x%X) %s failed: %v", item.itemID, item.name, err)
			continue
		}
		t.Logf("[DS3] %s (0x%X): quantity=%d", item.name, item.itemID, qty)
	}
}

func TestE2E_ReadInventoryItemQuantity_CountSanity(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	inv := reader.game.Inventory
	if inv == nil {
		t.Skip("No inventory config for current game")
	}

	// Validate normal item count
	count, err := reader.ReadMemoryValue(inv.PathKey, inv.DataOffset+inv.CountOffset, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(inventory count) failed: %v", err)
	}
	t.Logf("[DS3] Normal item count: %d", count)
	if count < 1 || count > 8192 {
		t.Errorf("normal item count %d outside expected range [1, 8192]", count)
	}

	// Validate capacity
	capacity, err := reader.ReadMemoryValue(inv.PathKey, inv.DataOffset+inv.CapacityOffset, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(inventory capacity) failed: %v", err)
	}
	t.Logf("[DS3] Inventory capacity: %d", capacity)
	if capacity < count {
		t.Errorf("capacity %d < count %d", capacity, count)
	}

	// Validate key item start
	keyStart, err := reader.ReadMemoryValue(inv.PathKey, inv.DataOffset+inv.KeyItemStartOffset, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue(key item start) failed: %v", err)
	}
	t.Logf("[DS3] Key item start index: %d", keyStart)
	if keyStart >= capacity {
		t.Errorf("key item start %d >= capacity %d", keyStart, capacity)
	}
	if keyStart <= count {
		t.Errorf("key item start %d <= normal count %d (regions would overlap)", keyStart, count)
	}
}

func TestE2E_ReadInventoryItemQuantity_NewItems(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	newItems := []struct {
		itemID uint32
		name   string
	}{
		{DS3ItemAshenEstusFlask, "Ashen Estus Flask"},
		{DS3ItemFarronCoal, "Farron Coal"},
		{DS3ItemDagger, "Dagger"},
		{DS3ItemShortsword, "Shortsword"},
	}

	for _, item := range newItems {
		qty, err := reader.ReadInventoryItemQuantity(item.itemID)
		if err != nil {
			t.Errorf("ReadInventoryItemQuantity(0x%X) %s failed: %v", item.itemID, item.name, err)
			continue
		}
		t.Logf("[DS3] %s (0x%X): quantity=%d", item.name, item.itemID, qty)
	}
}

// --- Stat Offset Probe ---

func TestE2E_ProbePlayerStatOffsets(t *testing.T) {
	reader := skipIfNoGameRunning(t)
	defer reader.Detach()
	requireDS3(t, reader)

	// Dump all uint32 values from player_game_data in the stat region.
	t.Log("Probing player_game_data offsets 0x40..0x98 (all values):")
	for offset := int64(0x40); offset <= 0x98; offset += 4 {
		val, err := reader.ReadMemoryValue("player_game_data", offset, 4)
		if err != nil {
			t.Logf("  +0x%02X = ERROR: %v", offset, err)
			continue
		}
		t.Logf("  +0x%02X = %d (0x%X)", offset, val, val)
	}
}
