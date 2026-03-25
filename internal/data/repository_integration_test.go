//go:build integration

package data

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const testDBName = "deathcounter_test.db"

var testDBPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "data_integration")
	if err != nil {
		panic(err)
	}
	testDBPath = filepath.Join(dir, testDBName)

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}

func openTestRepo(t *testing.T) *Repository {
	t.Helper()
	repo, err := NewRepository(testDBPath)
	if err != nil {
		t.Fatalf("NewRepository(%s): %v", testDBPath, err)
	}
	return repo
}

// --- Test 1: Full Death Tracking Lifecycle ---

func TestIntegration_DeathTrackingLifecycle(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	// Create save identity
	saveID, err := repo.FindOrCreateSave("Dark Souls III", 0, "IntegrationKnight")
	if err != nil {
		t.Fatalf("FindOrCreateSave: %v", err)
	}

	// Session 1: record deaths 1..5
	for i := uint32(1); i <= 5; i++ {
		if err := repo.RecordDeathForSave(i, saveID); err != nil {
			t.Fatalf("RecordDeathForSave(%d): %v", i, err)
		}
	}
	if err := repo.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}

	// Session 2: record deaths 1..3
	for i := uint32(1); i <= 3; i++ {
		if err := repo.RecordDeathForSave(i, saveID); err != nil {
			t.Fatalf("RecordDeathForSave(%d): %v", i, err)
		}
	}
	if err := repo.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}

	// Total deaths = session1(5) + session2(3) = 8
	total, err := repo.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	if total != 8 {
		t.Errorf("total deaths: got %d, want 8", total)
	}

	// Session history should have at least 2 sessions, most recent first
	sessions, err := repo.GetSessionHistory(10)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("expected at least 2 sessions, got %d", len(sessions))
	}
	// Most recent ended session should have 3 deaths
	if sessions[0].Deaths != 3 {
		t.Errorf("most recent session deaths: got %d, want 3", sessions[0].Deaths)
	}
	// All sessions should be ended
	for i, s := range sessions {
		if s.EndTime == nil {
			t.Errorf("session %d (id=%d): expected EndTime to be set", i, s.ID)
		}
	}

	// Verify death_events FK integrity
	var orphans int
	err = repo.DB().QueryRow(`
		SELECT COUNT(*) FROM death_events de
		LEFT JOIN sessions s ON de.session_id = s.id
		WHERE s.id IS NULL`).Scan(&orphans)
	if err != nil {
		t.Fatalf("FK integrity query: %v", err)
	}
	if orphans != 0 {
		t.Errorf("found %d orphaned death_events", orphans)
	}

	// Verify sessions.save_id is set
	var nullSaveCount int
	err = repo.DB().QueryRow(`
		SELECT COUNT(*) FROM sessions
		WHERE save_id IS NULL AND id IN (
			SELECT id FROM sessions ORDER BY start_time DESC LIMIT 2
		)`).Scan(&nullSaveCount)
	if err != nil {
		t.Fatalf("save_id query: %v", err)
	}
	if nullSaveCount != 0 {
		t.Errorf("expected all recent sessions to have save_id set, got %d without", nullSaveCount)
	}
}

// --- Test 2: Full Route Run Lifecycle ---

func TestIntegration_RouteRunLifecycle(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	saveID, err := repo.FindOrCreateSave("Dark Souls III", 0, "RouteRunner")
	if err != nil {
		t.Fatalf("FindOrCreateSave: %v", err)
	}

	runID, err := repo.StartRouteRun("ds3-all-bosses-integ", "Dark Souls III", saveID)
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}

	// Record checkpoints with realistic data
	checkpoints := []struct {
		id, name                string
		igtMs, durationMs      int64
		deaths                 uint32
	}{
		{"iudex-gundyr", "Iudex Gundyr", 95000, 95000, 2},
		{"vordt", "Vordt of the Boreal Valley", 225000, 130000, 1},
		{"sage", "Crystal Sage", 410000, 185000, 3},
		{"abyss-watchers", "Abyss Watchers", 680000, 270000, 5},
	}

	for _, cp := range checkpoints {
		if err := repo.RecordCheckpoint(runID, cp.id, cp.name, cp.igtMs, cp.durationMs, cp.deaths); err != nil {
			t.Fatalf("RecordCheckpoint(%s): %v", cp.id, err)
		}
		if err := repo.UpdatePersonalBest("ds3-all-bosses-integ", cp.id, cp.igtMs, cp.durationMs); err != nil {
			t.Fatalf("UpdatePersonalBest(%s): %v", cp.id, err)
		}
	}

	// Save state vars for cumulative inventory
	if err := repo.SaveStateVar(runID, "embers", 0x400001F4, 4, 6); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}
	if err := repo.SaveStateVar(runID, "firebombs", 0x40000124, 10, 10); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}

	// End run
	totalDeaths := uint32(2 + 1 + 3 + 5)
	if err := repo.EndRouteRun(runID, "completed", totalDeaths, 680000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	// Verify completed checkpoints
	completed, err := repo.LoadCompletedCheckpoints(runID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints: %v", err)
	}
	if len(completed) != 4 {
		t.Errorf("completed checkpoints: got %d, want 4", len(completed))
	}

	// Verify personal bests
	pbs, err := repo.GetPersonalBest("ds3-all-bosses-integ")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 4 {
		t.Errorf("PBs: got %d, want 4", len(pbs))
	}
	// PBs ordered by best_igt_ms ascending
	if len(pbs) >= 1 && pbs[0].CheckpointID != "iudex-gundyr" {
		t.Errorf("first PB: got %q, want iudex-gundyr", pbs[0].CheckpointID)
	}

	// Verify state vars
	vars, err := repo.LoadStateVars(runID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(vars) != 2 {
		t.Errorf("state vars: got %d, want 2", len(vars))
	}

	// Verify FK: route_checkpoints → route_runs
	var orphanCheckpoints int
	err = repo.DB().QueryRow(`
		SELECT COUNT(*) FROM route_checkpoints rc
		LEFT JOIN route_runs rr ON rc.run_id = rr.id
		WHERE rr.id IS NULL`).Scan(&orphanCheckpoints)
	if err != nil {
		t.Fatalf("FK integrity query: %v", err)
	}
	if orphanCheckpoints != 0 {
		t.Errorf("found %d orphaned route_checkpoints", orphanCheckpoints)
	}

	// Verify FK: route_runs → saves
	var runSaveID sql.NullInt64
	err = repo.DB().QueryRow("SELECT save_id FROM route_runs WHERE id = ?", runID).Scan(&runSaveID)
	if err != nil {
		t.Fatalf("query route_runs save_id: %v", err)
	}
	if !runSaveID.Valid || runSaveID.Int64 != saveID {
		t.Errorf("route_runs.save_id: got %v, want %d", runSaveID, saveID)
	}

	// Verify run status
	var status string
	err = repo.DB().QueryRow("SELECT status FROM route_runs WHERE id = ?", runID).Scan(&status)
	if err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != "completed" {
		t.Errorf("run status: got %q, want completed", status)
	}
}

// --- Test 3: Multiple Runs with PB Tracking ---

func TestIntegration_MultipleRunsPBTracking(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	routeID := "ds3-pb-tracking-integ"
	saveID, _ := repo.FindOrCreateSave("Dark Souls III", 0, "PBTracker")

	// Run 1
	run1, _ := repo.StartRouteRun(routeID, "Dark Souls III", saveID)
	repo.RecordCheckpoint(run1, "boss1", "Iudex Gundyr", 100000, 100000, 3)
	repo.RecordCheckpoint(run1, "boss2", "Vordt", 250000, 150000, 2)
	repo.UpdatePersonalBest(routeID, "boss1", 100000, 100000)
	repo.UpdatePersonalBest(routeID, "boss2", 250000, 150000)
	repo.EndRouteRun(run1, "completed", 5, 250000)

	// Run 2: boss1 faster, boss2 slower
	run2, _ := repo.StartRouteRun(routeID, "Dark Souls III", saveID)
	repo.RecordCheckpoint(run2, "boss1", "Iudex Gundyr", 85000, 85000, 1)
	repo.RecordCheckpoint(run2, "boss2", "Vordt", 260000, 175000, 4)
	repo.UpdatePersonalBest(routeID, "boss1", 85000, 85000)
	repo.UpdatePersonalBest(routeID, "boss2", 260000, 175000)
	repo.EndRouteRun(run2, "completed", 5, 260000)

	// Verify PBs kept the best of each
	pbs, err := repo.GetPersonalBest(routeID)
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 2 {
		t.Fatalf("PBs: got %d, want 2", len(pbs))
	}

	pbMap := map[string]RouteCheckpoint{}
	for _, pb := range pbs {
		pbMap[pb.CheckpointID] = pb
	}

	// boss1: run2 was faster (85000 < 100000)
	if pb := pbMap["boss1"]; pb.IGTMs != 85000 {
		t.Errorf("boss1 PB IGT: got %d, want 85000", pb.IGTMs)
	}
	if pb := pbMap["boss1"]; pb.CheckpointDurationMs != 85000 {
		t.Errorf("boss1 PB split: got %d, want 85000", pb.CheckpointDurationMs)
	}

	// boss2: run1 was faster (250000 < 260000, 150000 < 175000)
	if pb := pbMap["boss2"]; pb.IGTMs != 250000 {
		t.Errorf("boss2 PB IGT: got %d, want 250000", pb.IGTMs)
	}
	if pb := pbMap["boss2"]; pb.CheckpointDurationMs != 150000 {
		t.Errorf("boss2 PB split: got %d, want 150000", pb.CheckpointDurationMs)
	}
}

// --- Test 4: Save Change Mid-Run (Abandon + Restart) ---

func TestIntegration_SaveChangeMidRun(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	routeID := "ds3-save-change-integ"
	saveA, _ := repo.FindOrCreateSave("Dark Souls III", 0, "KnightA")
	saveB, _ := repo.FindOrCreateSave("Dark Souls III", 1, "KnightB")

	// Start run for save A, record a checkpoint
	runA, err := repo.StartRouteRun(routeID, "Dark Souls III", saveA)
	if err != nil {
		t.Fatalf("StartRouteRun(A): %v", err)
	}
	repo.RecordCheckpoint(runA, "boss1", "Iudex Gundyr", 95000, 95000, 2)

	// Character changes — abandon run A
	if err := repo.EndRouteRun(runA, "abandoned", 2, 95000); err != nil {
		t.Fatalf("EndRouteRun(abandon): %v", err)
	}

	// Start new run for save B
	runB, err := repo.StartRouteRun(routeID, "Dark Souls III", saveB)
	if err != nil {
		t.Fatalf("StartRouteRun(B): %v", err)
	}

	// FindLatestRun for save A should return the abandoned run
	gotID, gotStatus, err := repo.FindLatestRun(routeID, saveA)
	if err != nil {
		t.Fatalf("FindLatestRun(A): %v", err)
	}
	if gotID != runA {
		t.Errorf("FindLatestRun(A) ID: got %d, want %d", gotID, runA)
	}
	if gotStatus != "abandoned" {
		t.Errorf("FindLatestRun(A) status: got %q, want abandoned", gotStatus)
	}

	// FindLatestRun for save B should return the in_progress run
	gotID, gotStatus, err = repo.FindLatestRun(routeID, saveB)
	if err != nil {
		t.Fatalf("FindLatestRun(B): %v", err)
	}
	if gotID != runB {
		t.Errorf("FindLatestRun(B) ID: got %d, want %d", gotID, runB)
	}
	if gotStatus != "in_progress" {
		t.Errorf("FindLatestRun(B) status: got %q, want in_progress", gotStatus)
	}

	// Abandoned run's checkpoints should still be intact
	completed, err := repo.LoadCompletedCheckpoints(runA)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints(A): %v", err)
	}
	if len(completed) != 1 || completed[0] != "boss1" {
		t.Errorf("abandoned run checkpoints: got %v, want [boss1]", completed)
	}

	repo.EndRouteRun(runB, "completed", 0, 0) // cleanup
}

// --- Test 5: Run Resume After Restart ---

func TestIntegration_RunResumeAfterRestart(t *testing.T) {
	routeID := "ds3-resume-integ"

	// Phase 1: start run and record some data
	repo1, err := NewRepository(testDBPath)
	if err != nil {
		t.Fatalf("NewRepository (phase 1): %v", err)
	}

	saveID, _ := repo1.FindOrCreateSave("Dark Souls III", 0, "ResumeKnight")
	runID, _ := repo1.StartRouteRun(routeID, "Dark Souls III", saveID)
	repo1.RecordCheckpoint(runID, "boss1", "Iudex Gundyr", 95000, 95000, 2)
	repo1.SaveStateVar(runID, "embers", 0x400001F4, 3, 5)

	// Close (simulates app shutdown)
	if err := repo1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Phase 2: reopen (simulates app restart)
	repo2, err := NewRepository(testDBPath)
	if err != nil {
		t.Fatalf("NewRepository (phase 2): %v", err)
	}
	defer repo2.Close()

	// Re-resolve save
	saveID2, _ := repo2.FindOrCreateSave("Dark Souls III", 0, "ResumeKnight")
	if saveID2 != saveID {
		t.Errorf("save ID changed across restart: %d -> %d", saveID, saveID2)
	}

	// Find the in-progress run
	foundID, status, err := repo2.FindLatestRun(routeID, saveID2)
	if err != nil {
		t.Fatalf("FindLatestRun: %v", err)
	}
	if foundID != runID {
		t.Errorf("run ID: got %d, want %d", foundID, runID)
	}
	if status != "in_progress" {
		t.Errorf("status: got %q, want in_progress", status)
	}

	// Load completed checkpoints from previous session
	completed, err := repo2.LoadCompletedCheckpoints(foundID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints: %v", err)
	}
	if len(completed) != 1 || completed[0] != "boss1" {
		t.Errorf("completed checkpoints: got %v, want [boss1]", completed)
	}

	// Load state vars from previous session
	vars, err := repo2.LoadStateVars(foundID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(vars) != 1 {
		t.Fatalf("state vars: got %d, want 1", len(vars))
	}
	if vars[0].VarName != "embers" || vars[0].Accumulated != 5 {
		t.Errorf("state var: got %+v, want embers/accumulated=5", vars[0])
	}

	// Continue the run
	repo2.RecordCheckpoint(foundID, "boss2", "Vordt", 225000, 130000, 1)
	repo2.SaveStateVar(foundID, "embers", 0x400001F4, 5, 8)
	repo2.EndRouteRun(foundID, "completed", 3, 225000)

	// Verify final state
	completed, _ = repo2.LoadCompletedCheckpoints(foundID)
	if len(completed) != 2 {
		t.Errorf("final completed checkpoints: got %d, want 2", len(completed))
	}
	vars, _ = repo2.LoadStateVars(foundID)
	if len(vars) != 1 || vars[0].Accumulated != 8 {
		t.Errorf("final state var: got %+v, want accumulated=8", vars)
	}
}

// --- Test 6: Concurrent Save Sessions ---

func TestIntegration_ConcurrentSaveSessions(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	save1, _ := repo.FindOrCreateSave("Dark Souls III", 0, "ConcurrentA")
	save2, _ := repo.FindOrCreateSave("Dark Souls III", 1, "ConcurrentB")

	// Record deaths for each save independently
	repo.RecordDeathForSave(5, save1)
	repo.RecordDeathForSave(10, save2)

	// Verify sessions are isolated — each save has its own session
	var count1, count2 int
	repo.DB().QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE save_id = ? AND end_time IS NULL", save1,
	).Scan(&count1)
	repo.DB().QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE save_id = ? AND end_time IS NULL", save2,
	).Scan(&count2)

	if count1 != 1 {
		t.Errorf("save1 open sessions: got %d, want 1", count1)
	}
	if count2 != 1 {
		t.Errorf("save2 open sessions: got %d, want 1", count2)
	}

	// Verify each session has the correct death count
	var deaths1, deaths2 uint32
	repo.DB().QueryRow(
		"SELECT deaths FROM sessions WHERE save_id = ? AND end_time IS NULL", save1,
	).Scan(&deaths1)
	repo.DB().QueryRow(
		"SELECT deaths FROM sessions WHERE save_id = ? AND end_time IS NULL", save2,
	).Scan(&deaths2)

	if deaths1 != 5 {
		t.Errorf("save1 deaths: got %d, want 5", deaths1)
	}
	if deaths2 != 10 {
		t.Errorf("save2 deaths: got %d, want 10", deaths2)
	}

	// GetTotalDeaths sums across all sessions
	total, err := repo.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	// Total includes deaths from all tests sharing this DB, so check it's at least 15
	if total < 15 {
		t.Errorf("total deaths: got %d, want >= 15", total)
	}
}

// --- Test 7: State Variable Cumulative Tracking ---

func TestIntegration_StateVarCumulativeTracking(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	saveID, _ := repo.FindOrCreateSave("Dark Souls III", 0, "StateVarTracker")
	runID, _ := repo.StartRouteRun("ds3-statevar-integ", "Dark Souls III", saveID)

	// Initial tick: picked up 3 embers and 5 firebombs
	repo.SaveStateVar(runID, "embers", 0x400001F4, 3, 3)
	repo.SaveStateVar(runID, "firebombs", 0x40000124, 5, 5)

	// Tick 2: used 1 ember (qty drops to 2), picked up 2 more firebombs
	repo.SaveStateVar(runID, "embers", 0x400001F4, 2, 3) // accumulated stays at 3
	repo.SaveStateVar(runID, "firebombs", 0x40000124, 7, 7)

	// Tick 3: picked up 2 embers (qty now 4, net +2 since last positive)
	repo.SaveStateVar(runID, "embers", 0x400001F4, 4, 5) // accumulated 3+2=5
	repo.SaveStateVar(runID, "firebombs", 0x40000124, 7, 7) // unchanged

	// Verify only latest values persisted (no duplicates)
	vars, err := repo.LoadStateVars(runID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(vars) != 2 {
		t.Fatalf("expected 2 state vars, got %d", len(vars))
	}

	varMap := map[string]StateVarRow{}
	for _, v := range vars {
		varMap[v.VarName] = v
	}

	embers := varMap["embers"]
	if embers.LastQuantity != 4 {
		t.Errorf("embers last_quantity: got %d, want 4", embers.LastQuantity)
	}
	if embers.Accumulated != 5 {
		t.Errorf("embers accumulated: got %d, want 5", embers.Accumulated)
	}

	firebombs := varMap["firebombs"]
	if firebombs.LastQuantity != 7 {
		t.Errorf("firebombs last_quantity: got %d, want 7", firebombs.LastQuantity)
	}
	if firebombs.Accumulated != 7 {
		t.Errorf("firebombs accumulated: got %d, want 7", firebombs.Accumulated)
	}

	// Verify via raw SQL that no duplicates exist
	var rowCount int
	err = repo.DB().QueryRow(
		"SELECT COUNT(*) FROM route_state_vars WHERE run_id = ?", runID,
	).Scan(&rowCount)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if rowCount != 2 {
		t.Errorf("route_state_vars rows: got %d, want 2", rowCount)
	}

	repo.EndRouteRun(runID, "completed", 0, 0)
}

// --- Test 8: FindOrCreateSave Upsert Updates last_seen_at ---

func TestIntegration_SaveUpsertUpdatesLastSeen(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	game, slot, name := "Dark Souls III", 0, "UpsertKnight"

	id1, err := repo.FindOrCreateSave(game, slot, name)
	if err != nil {
		t.Fatalf("first FindOrCreateSave: %v", err)
	}

	// Read initial last_seen_at
	var lastSeen1 string
	repo.DB().QueryRow(
		"SELECT last_seen_at FROM saves WHERE id = ?", id1,
	).Scan(&lastSeen1)

	// Call again — should return same ID but update last_seen_at
	id2, err := repo.FindOrCreateSave(game, slot, name)
	if err != nil {
		t.Fatalf("second FindOrCreateSave: %v", err)
	}
	if id1 != id2 {
		t.Errorf("IDs differ: %d vs %d", id1, id2)
	}

	var lastSeen2 string
	repo.DB().QueryRow(
		"SELECT last_seen_at FROM saves WHERE id = ?", id2,
	).Scan(&lastSeen2)

	if lastSeen2 < lastSeen1 {
		t.Errorf("last_seen_at did not advance: %s -> %s", lastSeen1, lastSeen2)
	}
}

// --- Test 9: FindLatestRun Returns ErrNotFound for Unknown Route ---

func TestIntegration_FindLatestRunNotFound(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	saveID, _ := repo.FindOrCreateSave("Dark Souls III", 0, "NotFoundKnight")

	_, _, err := repo.FindLatestRun("nonexistent-route", saveID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Test 10: Schema Integrity (all tables and indexes exist) ---

func TestIntegration_SchemaIntegrity(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	tables := []string{
		"sessions", "death_events", "route_runs",
		"route_checkpoints", "route_pbs", "route_state_vars", "saves",
	}
	for _, table := range tables {
		var name string
		err := repo.DB().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	indexes := []string{"idx_sessions_start", "idx_deaths_session"}
	for _, idx := range indexes {
		var name string
		err := repo.DB().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}
