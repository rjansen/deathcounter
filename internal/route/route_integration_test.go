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
	route, err := LoadRoute(filepath.Join(dir, "01-ds3-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	if route.Game != "Dark Souls III" {
		t.Errorf("game = %q, want %q", route.Game, "Dark Souls III")
	}
	if route.Version != "8" {
		t.Errorf("version = %q, want %q", route.Version, "8")
	}
}

func TestDS3Route_CheckpointFlagsMatchConstants(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "01-ds3-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	// Map of checkpoint ID to expected defeated flag from ds3_offsets.go constants.
	expectedFlags := map[string]uint32{
		"iudex-gundyr":       memreader.DS3FlagIudexGundyr,
		"vordt":              memreader.DS3FlagVordt,
		"abyss-watchers":     memreader.DS3FlagAbyssWatcher,
		"wolnir":             memreader.DS3FlagWolnir,
		"crystal-sage":       memreader.DS3FlagCrystalSage,
		"deacons":            memreader.DS3FlagDeacons,
		"yhorm":              memreader.DS3FlagYhorm,
		"pontiff":            memreader.DS3FlagPontiff,
		"aldrich":            memreader.DS3FlagAldrich,
		"dancer":             memreader.DS3FlagDancer,
		"dragonslayer-armour": memreader.DS3FlagDragonslayer,
		"twin-princes":       memreader.DS3FlagTwinPrinces,
		"soul-of-cinder":     memreader.DS3FlagSoulOfCinder,
	}

	for _, cp := range route.Checkpoints {
		expected, ok := expectedFlags[cp.ID]
		if !ok {
			continue // mem_check checkpoint, no flag to validate
		}
		t.Run(cp.ID, func(t *testing.T) {
			if cp.EventFlagID != expected {
				t.Errorf("event_flag_id = %d, want %d (DS3Flag constant)", cp.EventFlagID, expected)
			}
		})
	}
}

func TestDS3Route_BackupFlagsMatchEncounteredConstants(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "01-ds3-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	// Map of checkpoint ID to expected encountered flag.
	// Bosses with no known encounter flag should have BackupFlagID == 0 (omitted from JSON).
	expectedBackup := map[string]uint32{
		"iudex-gundyr":       memreader.DS3FlagIudexGundyrEnc,
		"vordt":              memreader.DS3FlagVordtEnc,
		"abyss-watchers":     memreader.DS3FlagAbyssWatcherEnc,
		"wolnir":             memreader.DS3FlagWolnirEnc,
		"crystal-sage":       memreader.DS3FlagCrystalSageEnc,
		"deacons":            memreader.DS3FlagDeaconsEnc,
		"yhorm":              memreader.DS3FlagYhormEnc,
		"twin-princes":       memreader.DS3FlagTwinPrincesEnc,
		"soul-of-cinder":     memreader.DS3FlagSoulOfCinderEnc,
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
			if cp.BackupFlagID != expected {
				t.Errorf("backup_flag_id = %d, want %d", cp.BackupFlagID, expected)
			}
		})
	}
}

func TestDS3Route_NoDuplicateEventFlags(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "01-ds3-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	seen := make(map[uint32]string)
	for _, cp := range route.Checkpoints {
		if cp.EventFlagID == 0 {
			continue
		}
		if prev, ok := seen[cp.EventFlagID]; ok {
			t.Errorf("duplicate event_flag_id %d: %q and %q", cp.EventFlagID, prev, cp.ID)
		}
		seen[cp.EventFlagID] = cp.ID
	}
}

func TestDS3Route_NoDuplicateBackupFlags(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "01-ds3-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	seen := make(map[uint32]string)
	for _, cp := range route.Checkpoints {
		if cp.BackupFlagID == 0 {
			continue
		}
		if prev, ok := seen[cp.BackupFlagID]; ok {
			t.Errorf("duplicate backup_flag_id %d: %q and %q", cp.BackupFlagID, prev, cp.ID)
		}
		seen[cp.BackupFlagID] = cp.ID
	}
}

func TestDS3Route_ReferenceTimesMonotonicallyIncrease(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "01-ds3-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	if len(route.ReferenceTimes) == 0 {
		t.Skip("no reference times defined")
	}

	for i := 1; i < len(route.ReferenceTimes); i++ {
		if route.ReferenceTimes[i] <= route.ReferenceTimes[i-1] {
			t.Errorf("reference_times[%d] (%d) <= reference_times[%d] (%d): not monotonically increasing",
				i, route.ReferenceTimes[i], i-1, route.ReferenceTimes[i-1])
		}
	}
}

func TestDS3Route_AllBossCheckpointsHaveEventFlags(t *testing.T) {
	dir := routesDir(t)
	route, err := LoadRoute(filepath.Join(dir, "01-ds3-glitchless-any-percent-e2e.json"))
	if err != nil {
		t.Fatalf("failed to load DS3 route: %v", err)
	}

	for _, cp := range route.Checkpoints {
		if cp.EventType == "boss_kill" && cp.EventFlagID == 0 {
			t.Errorf("boss_kill checkpoint %q has no event_flag_id", cp.ID)
		}
	}
}

func TestAllRoutes_LoadSuccessfully(t *testing.T) {
	dir := routesDir(t)
	routes, err := LoadRoutesDir(dir)
	if err != nil {
		t.Fatalf("LoadRoutesDir: %v", err)
	}

	if len(routes) == 0 {
		t.Fatal("no routes found in routes/ directory")
	}

	for _, r := range routes {
		t.Logf("Loaded route: %s (%s) — %d checkpoints", r.Name, r.Game, len(r.Checkpoints))
	}
}
