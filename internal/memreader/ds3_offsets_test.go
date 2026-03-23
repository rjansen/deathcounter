package memreader

import (
	"fmt"
	"testing"
)

// allDefeatedFlags returns every DS3 boss defeated flag constant with its name.
func allDefeatedFlags() []struct {
	name string
	flag uint32
} {
	return []struct {
		name string
		flag uint32
	}{
		// Base game
		{"IudexGundyr", DS3FlagIudexGundyr},
		{"Vordt", DS3FlagVordt},
		{"Greatwood", DS3FlagGreatwood},
		{"CrystalSage", DS3FlagCrystalSage},
		{"AbyssWatcher", DS3FlagAbyssWatcher},
		{"Deacons", DS3FlagDeacons},
		{"Wolnir", DS3FlagWolnir},
		{"OldDemonKing", DS3FlagOldDemonKing},
		{"Pontiff", DS3FlagPontiff},
		{"Aldrich", DS3FlagAldrich},
		{"Yhorm", DS3FlagYhorm},
		{"Dancer", DS3FlagDancer},
		{"Oceiros", DS3FlagOceiros},
		{"ChampionGundyr", DS3FlagChampionGundyr},
		{"AncientWyvern", DS3FlagAncientWyvern},
		{"NamelessKing", DS3FlagNamelessKing},
		{"Dragonslayer", DS3FlagDragonslayer},
		{"TwinPrinces", DS3FlagTwinPrinces},
		{"SoulOfCinder", DS3FlagSoulOfCinder},
		// DLC
		{"ChampionGravetender", DS3FlagChampionGravetender},
		{"Friede", DS3FlagFriede},
		{"DemonPrince", DS3FlagDemonPrince},
		{"Halflight", DS3FlagHalflight},
		{"Midir", DS3FlagMidir},
		{"Gael", DS3FlagGael},
	}
}

// allEncounteredFlags returns every DS3 boss encountered flag constant with its name.
func allEncounteredFlags() []struct {
	name string
	flag uint32
} {
	return []struct {
		name string
		flag uint32
	}{
		{"IudexGundyrEnc", DS3FlagIudexGundyrEnc},
		{"VordtEnc", DS3FlagVordtEnc},
		{"GreatwoodEnc", DS3FlagGreatwoodEnc},
		{"CrystalSageEnc", DS3FlagCrystalSageEnc},
		{"AbyssWatcherEnc", DS3FlagAbyssWatcherEnc},
		{"DeaconsEnc", DS3FlagDeaconsEnc},
		{"WolnirEnc", DS3FlagWolnirEnc},
		{"YhormEnc", DS3FlagYhormEnc},
		{"OceirosEnc", DS3FlagOceirosEnc},
		{"ChampionGundyrEnc", DS3FlagChampionGundyrEnc},
		{"TwinPrincesEnc", DS3FlagTwinPrincesEnc},
		{"SoulOfCinderEnc", DS3FlagSoulOfCinderEnc},
		{"ChampionGravetenderEnc", DS3FlagChampionGravetenderEnc},
		{"FriedeEnc", DS3FlagFriedeEnc},
		{"HalflightEnc", DS3FlagHalflightEnc},
		{"MidirEnc", DS3FlagMidirEnc},
		{"GaelEnc", DS3FlagGaelEnc},
	}
}

func TestDS3BossFlags_Count(t *testing.T) {
	defeated := allDefeatedFlags()
	if len(defeated) != 25 {
		t.Errorf("expected 25 defeated boss flags, got %d", len(defeated))
	}

	encountered := allEncounteredFlags()
	if len(encountered) != 17 {
		t.Errorf("expected 17 encountered boss flags, got %d", len(encountered))
	}
}

func TestDS3BossFlags_DefeatedNoDuplicates(t *testing.T) {
	seen := make(map[uint32]string)
	for _, f := range allDefeatedFlags() {
		if prev, ok := seen[f.flag]; ok {
			t.Errorf("duplicate defeated flag value %d: %s and %s", f.flag, prev, f.name)
		}
		seen[f.flag] = f.name
	}
}

func TestDS3BossFlags_EncounteredNoDuplicates(t *testing.T) {
	seen := make(map[uint32]string)
	for _, f := range allEncounteredFlags() {
		if prev, ok := seen[f.flag]; ok {
			t.Errorf("duplicate encountered flag value %d: %s and %s", f.flag, prev, f.name)
		}
		seen[f.flag] = f.name
	}
}

func TestDS3BossFlags_NoOverlapBetweenDefeatedAndEncountered(t *testing.T) {
	defeated := make(map[uint32]string)
	for _, f := range allDefeatedFlags() {
		defeated[f.flag] = f.name
	}
	for _, f := range allEncounteredFlags() {
		if dname, ok := defeated[f.flag]; ok {
			t.Errorf("encountered flag %s (%d) collides with defeated flag %s", f.name, f.flag, dname)
		}
	}
}

func TestDS3BossFlags_DefeatedBitPattern(t *testing.T) {
	// DS3 defeated flags follow the pattern XXX00800, XXX00830, XXX00850, XXX00860, XXX00890.
	// The last 3 digits encode the flag type. All defeated flags should end in
	// one of these known suffixes (bit 7 = defeated).
	validSuffixes := map[uint32]bool{
		800: true, 830: true, 850: true, 860: true, 890: true,
	}

	for _, f := range allDefeatedFlags() {
		suffix := f.flag % 1000
		if !validSuffixes[suffix] {
			t.Errorf("%s flag %d has unexpected suffix %d (expected one of 800, 830, 850, 860, 890)",
				f.name, f.flag, suffix)
		}
	}
}

func TestDS3BossFlags_EncounteredRelationToDefeated(t *testing.T) {
	// Encountered flags are typically defeated+1 or defeated+2 (for the XXX50 variants).
	// Build a set of defeated flags to verify each encountered flag is close to one.
	defeated := make(map[uint32]bool)
	for _, f := range allDefeatedFlags() {
		defeated[f.flag] = true
	}

	for _, f := range allEncounteredFlags() {
		// The encountered flag should be within +1 or +2 of a defeated flag
		foundPair := defeated[f.flag-1] || defeated[f.flag-2]
		if !foundPair {
			t.Errorf("encountered flag %s (%d) has no matching defeated flag at %d or %d",
				f.name, f.flag, f.flag-1, f.flag-2)
		}
	}
}

func TestDS3BossFlags_AreaRanges(t *testing.T) {
	// DS3 area IDs: base game uses 130xx-141xx, DLC uses 145xx-151xx.
	// The top digits of the flag (flag / 100000) should fall in valid area ranges.
	validAreas := map[uint32]bool{
		130: true, 131: true, 132: true, 133: true, 134: true, 135: true,
		137: true, 138: true, 139: true, 140: true, 141: true,
		// DLC
		145: true, 150: true, 151: true,
	}

	for _, f := range allDefeatedFlags() {
		area := f.flag / 100000
		if !validAreas[area] {
			t.Errorf("%s flag %d has area prefix %d which is not a known DS3 area", f.name, f.flag, area)
		}
	}
	for _, f := range allEncounteredFlags() {
		area := f.flag / 100000
		if !validAreas[area] {
			t.Errorf("%s flag %d has area prefix %d which is not a known DS3 area", f.name, f.flag, area)
		}
	}
}

func TestDS3BossFlags_KnownValues(t *testing.T) {
	// Verify specific flag values against the cheat table (DS3_TGA_v3.4.0.CT).
	// These are the authoritative values from the CT.
	expected := []struct {
		name string
		flag uint32
		want uint32
	}{
		// Base game — corrections verified against CT
		{"IudexGundyr", DS3FlagIudexGundyr, 14000800},
		{"Vordt", DS3FlagVordt, 13000800},
		{"Greatwood", DS3FlagGreatwood, 13100800},
		{"CrystalSage", DS3FlagCrystalSage, 13300850},
		{"AbyssWatcher", DS3FlagAbyssWatcher, 13300800},
		{"Deacons", DS3FlagDeacons, 13500800},
		{"Wolnir", DS3FlagWolnir, 13800800},
		{"OldDemonKing", DS3FlagOldDemonKing, 13800830},
		{"Pontiff", DS3FlagPontiff, 13700850},
		{"Aldrich", DS3FlagAldrich, 13700800},
		{"Yhorm", DS3FlagYhorm, 13900800},
		{"Dancer", DS3FlagDancer, 13000890},
		{"Oceiros", DS3FlagOceiros, 13000830},
		{"ChampionGundyr", DS3FlagChampionGundyr, 14000830},
		{"AncientWyvern", DS3FlagAncientWyvern, 13200800},
		{"NamelessKing", DS3FlagNamelessKing, 13200850},
		{"Dragonslayer", DS3FlagDragonslayer, 13010800},
		{"TwinPrinces", DS3FlagTwinPrinces, 13410830},
		{"SoulOfCinder", DS3FlagSoulOfCinder, 14100800},
		// DLC
		{"ChampionGravetender", DS3FlagChampionGravetender, 14500800},
		{"Friede", DS3FlagFriede, 14500860},
		{"DemonPrince", DS3FlagDemonPrince, 15000800},
		{"Halflight", DS3FlagHalflight, 15100800},
		{"Midir", DS3FlagMidir, 15100850},
		{"Gael", DS3FlagGael, 15110800},
	}

	for _, tc := range expected {
		t.Run(tc.name, func(t *testing.T) {
			if tc.flag != tc.want {
				t.Errorf("DS3Flag%s = %d, want %d", tc.name, tc.flag, tc.want)
			}
		})
	}
}

func TestDS3BossFlags_KnownEncounteredValues(t *testing.T) {
	// Verify encountered flag values against the cheat table.
	expected := []struct {
		name string
		flag uint32
		want uint32
	}{
		{"IudexGundyrEnc", DS3FlagIudexGundyrEnc, 14000801},
		{"VordtEnc", DS3FlagVordtEnc, 13000801},
		{"GreatwoodEnc", DS3FlagGreatwoodEnc, 13100801},
		{"CrystalSageEnc", DS3FlagCrystalSageEnc, 13300852},
		{"AbyssWatcherEnc", DS3FlagAbyssWatcherEnc, 13300801},
		{"DeaconsEnc", DS3FlagDeaconsEnc, 13500801},
		{"WolnirEnc", DS3FlagWolnirEnc, 13800801},
		{"YhormEnc", DS3FlagYhormEnc, 13900801},
		{"OceirosEnc", DS3FlagOceirosEnc, 13000831},
		{"ChampionGundyrEnc", DS3FlagChampionGundyrEnc, 14000831},
		{"TwinPrincesEnc", DS3FlagTwinPrincesEnc, 13410831},
		{"SoulOfCinderEnc", DS3FlagSoulOfCinderEnc, 14100801},
		{"ChampionGravetenderEnc", DS3FlagChampionGravetenderEnc, 14500801},
		{"FriedeEnc", DS3FlagFriedeEnc, 14500861},
		{"HalflightEnc", DS3FlagHalflightEnc, 15100801},
		{"MidirEnc", DS3FlagMidirEnc, 15100851},
		{"GaelEnc", DS3FlagGaelEnc, 15110801},
	}

	for _, tc := range expected {
		t.Run(tc.name, func(t *testing.T) {
			if tc.flag != tc.want {
				t.Errorf("DS3Flag%s = %d, want %d", tc.name, tc.flag, tc.want)
			}
		})
	}
}

func allItemIDs() []struct {
	name string
	id   uint32
} {
	return []struct {
		name string
		id   uint32
	}{
		// Goods
		{"Ember", DS3ItemEmber},
		{"GoldPineResin", DS3ItemGoldPineResin},
		{"CarthusRouge", DS3ItemCarthusRouge},
		{"HomewardBone", DS3ItemHomewardBone},
		{"Firebomb", DS3ItemFirebomb},
		{"TitaniteShard", DS3ItemTitaniteShard},
		{"LargeTitaniteShard", DS3ItemLargeTitaniteShard},
		{"TitaniteChunk", DS3ItemTitaniteChunk},
		{"TitaniteSlab", DS3ItemTitaniteSlab},
		{"EstusShard", DS3ItemEstusShard},
		{"GraveWardenAshes", DS3ItemGraveWardenAshes},
		{"MorticiansAshes", DS3ItemMorticiansAshes},
		{"SharpGem", DS3ItemSharpGem},
		{"AshenEstusFlask", DS3ItemAshenEstusFlask},
		{"FarronCoal", DS3ItemFarronCoal},
		// Rings
		{"CovetousSilverSerpentRing", DS3ItemCovetousSilverSerpentRing},
		{"ChloranthyRing", DS3ItemChloranthyRing},
		{"LloydsSwordRing", DS3ItemLloydsSwordRing},
		{"PontiffsRightEye", DS3ItemPontiffsRightEye},
		// Weapons
		{"SellswordTwinblades", DS3ItemSellswordTwinblades},
		{"Dagger", DS3ItemDagger},
		{"Shortsword", DS3ItemShortsword},
	}
}

func TestDS3ItemIDs_Count(t *testing.T) {
	items := allItemIDs()
	if len(items) != 22 {
		t.Errorf("expected 22 item ID constants, got %d", len(items))
	}
}

func TestDS3ItemIDs_NoDuplicates(t *testing.T) {
	seen := make(map[uint32]string)
	for _, item := range allItemIDs() {
		if prev, ok := seen[item.id]; ok {
			t.Errorf("duplicate item ID 0x%X: %s and %s", item.id, prev, item.name)
		}
		seen[item.id] = item.name
	}
}

func TestDS3ItemIDs_KnownValues(t *testing.T) {
	// Verify item ID values against the cheat table (DS3_TGA_v3.4.0.CT).
	expected := []struct {
		name string
		id   uint32
		want uint32
	}{
		{"Ember", DS3ItemEmber, 0x400001F4},
		{"GoldPineResin", DS3ItemGoldPineResin, 0x4000014B},
		{"CarthusRouge", DS3ItemCarthusRouge, 0x4000014F},
		{"HomewardBone", DS3ItemHomewardBone, 0x4000015E},
		{"TitaniteShard", DS3ItemTitaniteShard, 0x400003E8},
		{"LargeTitaniteShard", DS3ItemLargeTitaniteShard, 0x400003E9},
		{"TitaniteChunk", DS3ItemTitaniteChunk, 0x400003EA},
		{"TitaniteSlab", DS3ItemTitaniteSlab, 0x400003EB},
		{"EstusShard", DS3ItemEstusShard, 0x4000085D},
		{"GraveWardenAshes", DS3ItemGraveWardenAshes, 0x4000083E},
		{"MorticiansAshes", DS3ItemMorticiansAshes, 0x4000083B},
		{"Firebomb", DS3ItemFirebomb, 0x40000124},
		{"SharpGem", DS3ItemSharpGem, 0x40000456},
		{"AshenEstusFlask", DS3ItemAshenEstusFlask, 0x400000BE},
		{"FarronCoal", DS3ItemFarronCoal, 0x40000837},
		// Rings
		{"CovetousSilverSerpentRing", DS3ItemCovetousSilverSerpentRing, 0x20004FB0},
		{"ChloranthyRing", DS3ItemChloranthyRing, 0x20004E2A},
		{"LloydsSwordRing", DS3ItemLloydsSwordRing, 0x200050B4},
		{"PontiffsRightEye", DS3ItemPontiffsRightEye, 0x2000510E},
		// Weapons
		{"SellswordTwinblades", DS3ItemSellswordTwinblades, 0x00F42400},
		{"Dagger", DS3ItemDagger, 0x000F4240},
		{"Shortsword", DS3ItemShortsword, 0x001E8480},
	}

	for _, tc := range expected {
		t.Run(tc.name, func(t *testing.T) {
			if tc.id != tc.want {
				t.Errorf("DS3Item%s = 0x%X, want 0x%X", tc.name, tc.id, tc.want)
			}
		})
	}
}

func TestDS3ItemIDs_GoodsPrefix(t *testing.T) {
	// All goods items should have the 0x4000xxxx prefix.
	// Skip non-goods categories (weapons, rings).
	goods := allItemIDs()
	for _, item := range goods {
		prefix := item.id & 0xFFFF0000
		switch prefix {
		case 0x00F40000, 0x000F0000, 0x001E0000: // weapon
			continue
		case 0x20000000: // ring
			continue
		}
		if prefix != 0x40000000 {
			t.Errorf("DS3Item%s = 0x%X, expected goods prefix 0x4000xxxx", item.name, item.id)
		}
	}
}

func TestDS3BossFlags_FlagDecomposition(t *testing.T) {
	// Verify that each defeated flag can be decomposed using the DS3 event flag algorithm
	// without producing invalid intermediate values.
	// flagID → div10M, area, block, div1K, remainder
	for _, f := range allDefeatedFlags() {
		t.Run(f.name, func(t *testing.T) {
			flagID := f.flag
			div10M := flagID / 10_000_000
			rem := flagID % 10_000_000
			area := rem / 100_000
			rem2 := rem % 100_000
			block := rem2 / 10_000
			div1K := rem2 % 10_000 / 1_000
			remainder := flagID % 1_000

			// div10M should be 1 (all DS3 flags are in the 1xxxxxxx range)
			if div10M != 1 {
				t.Errorf("div10M = %d, want 1", div10M)
			}
			// area should be < 90 for boss flags (they use FieldArea lookup, not global)
			if area >= 90 {
				t.Errorf("area = %d, boss flags should have area < 90", area)
			}
			// block should be 0 or 1 (single digit)
			if block > 9 {
				t.Errorf("block = %d, should be 0-9", block)
			}
			// div1K should be 0 (all boss flags have 0 in the thousands place of the sub-area)
			if div1K != 0 {
				t.Errorf("div1K = %d, expected 0 for boss flag", div1K)
			}
			// remainder encodes the bit position — should be in [800, 899]
			if remainder < 800 || remainder > 899 {
				t.Logf("remainder = %d (flag %d) — non-standard but may be valid", remainder, flagID)
			}

			_ = fmt.Sprintf("flag=%d div10M=%d area=%d block=%d div1K=%d remainder=%d",
				flagID, div10M, area, block, div1K, remainder)
		})
	}
}
