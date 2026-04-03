package route

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// routesDir returns the absolute path to the project's routes/ directory.
func routesDir(t *testing.T) string {
	t.Helper()
	// This file is at internal/route/route_integration_test.go.
	// The routes dir is at ../../routes relative to this file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "routes")
}

func TestDS3Route_LoadsSuccessfully(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "ds3", "01-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	if route.Game != "ds3" {
		t.Errorf("game = %q, want %q", route.Game, "ds3")
	}
	if route.Version != "11" {
		t.Errorf("version = %q, want %q", route.Version, "11")
	}
}

func TestDS3Route_CheckpointFlagsMatchConstants(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "ds3", "01-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	// Map of checkpoint ID to expected defeated flag from ds3_offsets.go constants.
	expectedFlags := map[string]uint32{
		"iudex-gundyr":        memreader.DS3FlagIudexGundyr,
		"vordt":               memreader.DS3FlagVordt,
		"abyss-watchers":      memreader.DS3FlagAbyssWatcher,
		"wolnir":              memreader.DS3FlagWolnir,
		"crystal-sage":        memreader.DS3FlagCrystalSage,
		"deacons":             memreader.DS3FlagDeacons,
		"yhorm":               memreader.DS3FlagYhorm,
		"pontiff":             memreader.DS3FlagPontiff,
		"aldrich":             memreader.DS3FlagAldrich,
		"dancer":              memreader.DS3FlagDancer,
		"dragonslayer-armour": memreader.DS3FlagDragonslayer,
		"twin-princes":        memreader.DS3FlagTwinPrinces,
		"soul-of-cinder":      memreader.DS3FlagSoulOfCinder,
	}

	for _, cp := range route.Checkpoints {
		expected, ok := expectedFlags[cp.ID]
		if !ok {
			continue // mem_check checkpoint, no flag to validate
		}
		t.Run(cp.ID, func(t *testing.T) {
			if cp.EventFlagCheck == nil || cp.EventFlagCheck.FlagID != expected {
				got := uint32(0)
				if cp.EventFlagCheck != nil {
					got = cp.EventFlagCheck.FlagID
				}
				t.Errorf("event_flag_id = %d, want %d (DS3Flag constant)", got, expected)
			}
		})
	}
}

func TestDS3Route_BackupFlagsMatchEncounteredConstants(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "ds3", "01-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	// Map of checkpoint ID to expected encountered flag.
	// Bosses with no known encounter flag should have BackupFlagCheck == nil (omitted from JSON).
	expectedBackup := map[string]uint32{
		"iudex-gundyr":   memreader.DS3FlagIudexGundyrEnc,
		"vordt":          memreader.DS3FlagVordtEnc,
		"abyss-watchers": memreader.DS3FlagAbyssWatcherEnc,
		"wolnir":         memreader.DS3FlagWolnirEnc,
		"crystal-sage":   memreader.DS3FlagCrystalSageEnc,
		"deacons":        memreader.DS3FlagDeaconsEnc,
		"yhorm":          memreader.DS3FlagYhormEnc,
		"twin-princes":   memreader.DS3FlagTwinPrincesEnc,
		"soul-of-cinder": memreader.DS3FlagSoulOfCinderEnc,
		// These bosses have no known encounter flag — backup should be 0 (absent).
		"pontiff":             0,
		"aldrich":             0,
		"dancer":              0,
		"dragonslayer-armour": 0,
	}

	for _, cp := range route.Checkpoints {
		expected, ok := expectedBackup[cp.ID]
		if !ok {
			continue // mem_check checkpoint
		}
		t.Run(cp.ID, func(t *testing.T) {
			if expected == 0 {
				if cp.BackupFlagCheck != nil {
					t.Errorf("backup_flag_id = %d, want nil (no encounter flag)", cp.BackupFlagCheck.FlagID)
				}
			} else {
				if cp.BackupFlagCheck == nil || cp.BackupFlagCheck.FlagID != expected {
					got := uint32(0)
					if cp.BackupFlagCheck != nil {
						got = cp.BackupFlagCheck.FlagID
					}
					t.Errorf("backup_flag_id = %d, want %d", got, expected)
				}
			}
		})
	}
}

func TestDS3Route_NoDuplicateEventFlags(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "ds3", "01-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	seen := make(map[uint32]string)
	for _, cp := range route.Checkpoints {
		if cp.EventFlagCheck == nil {
			continue
		}
		if prev, ok := seen[cp.EventFlagCheck.FlagID]; ok {
			t.Errorf("duplicate event_flag_id %d: %q and %q", cp.EventFlagCheck.FlagID, prev, cp.ID)
		}
		seen[cp.EventFlagCheck.FlagID] = cp.ID
	}
}

func TestDS3Route_NoDuplicateBackupFlags(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "ds3", "01-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	seen := make(map[uint32]string)
	for _, cp := range route.Checkpoints {
		if cp.BackupFlagCheck == nil {
			continue
		}
		if prev, ok := seen[cp.BackupFlagCheck.FlagID]; ok {
			t.Errorf("duplicate backup_flag_id %d: %q and %q", cp.BackupFlagCheck.FlagID, prev, cp.ID)
		}
		seen[cp.BackupFlagCheck.FlagID] = cp.ID
	}
}

func TestDS3Route_AllBossCheckpointsHaveEventFlags(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "ds3", "01-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	for _, cp := range route.Checkpoints {
		if cp.EventType == "boss_kill" && cp.EventFlagCheck == nil {
			t.Errorf("boss_kill checkpoint %q has no event_flag_id", cp.ID)
		}
	}
}

func TestAllRoutes_LoadSuccessfully(t *testing.T) {
	dir := routesDir(t)
	routeMap, err := LoadRoutesDir(dir)
	if err != nil {
		t.Fatalf("LoadRoutesDir: %v", err)
	}

	if len(routeMap) == 0 {
		t.Fatal("no game directories found in routes/")
	}

	for gameID, routes := range routeMap {
		for _, r := range routes {
			t.Logf("Loaded route: %s (%s) game=%s — %d checkpoints", r.Name, r.ID, gameID, len(r.Checkpoints))
		}
	}
}
