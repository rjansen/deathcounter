//go:build integration

package data

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/rjansen/deathcounter/internal/data/model"
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
	save, err := repo.FindOrCreateSave("Dark Souls III", 0, "IntegrationKnight")
	if err != nil {
		t.Fatalf("FindOrCreateSave: %v", err)
	}

	// Session 1: record deaths 1..5
	for i := uint32(1); i <= 5; i++ {
		if err := repo.RecordDeathForSave(i, save.ID); err != nil {
			t.Fatalf("RecordDeathForSave(%d): %v", i, err)
		}
	}
	if err := repo.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}

	// Session 2: record deaths 1..3
	for i := uint32(1); i <= 3; i++ {
		if err := repo.RecordDeathForSave(i, save.ID); err != nil {
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

	save, err := repo.FindOrCreateSave("Dark Souls III", 0, "RouteRunner")
	if err != nil {
		t.Fatalf("FindOrCreateSave: %v", err)
	}

	run, err := repo.StartRouteRun("ds3-all-bosses-integ", "Dark Souls III", save.ID)
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
		if err := repo.RecordCheckpoint(run.ID, cp.id, cp.name, cp.igtMs, cp.durationMs, cp.deaths); err != nil {
			t.Fatalf("RecordCheckpoint(%s): %v", cp.id, err)
		}
		if err := repo.UpdatePersonalBest("ds3-all-bosses-integ", cp.id, cp.igtMs, cp.durationMs); err != nil {
			t.Fatalf("UpdatePersonalBest(%s): %v", cp.id, err)
		}
	}

	// Save state vars for cumulative inventory
	if err := repo.SaveStateVar(run.ID, "embers", 0x400001F4, 4, 6); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}
	if err := repo.SaveStateVar(run.ID, "firebombs", 0x40000124, 10, 10); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}

	// End run
	totalDeaths := uint32(2 + 1 + 3 + 5)
	if err := repo.EndRouteRun(run.ID, "completed", totalDeaths, 680000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	// Verify completed checkpoints
	completed, err := repo.LoadCompletedCheckpoints(run.ID)
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
	vars, err := repo.LoadStateVars(run.ID)
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
	err = repo.DB().QueryRow("SELECT save_id FROM route_runs WHERE id = ?", run.ID).Scan(&runSaveID)
	if err != nil {
		t.Fatalf("query route_runs save_id: %v", err)
	}
	if !runSaveID.Valid || runSaveID.Int64 != save.ID {
		t.Errorf("route_runs.save_id: got %v, want %d", runSaveID, save.ID)
	}

	// Verify run status
	var status string
	err = repo.DB().QueryRow("SELECT status FROM route_runs WHERE id = ?", run.ID).Scan(&status)
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
	save, _ := repo.FindOrCreateSave("Dark Souls III", 0, "PBTracker")

	// Run 1
	run1, _ := repo.StartRouteRun(routeID, "Dark Souls III", save.ID)
	if err := repo.RecordCheckpoint(run1.ID, "boss1", "Iudex Gundyr", 100000, 100000, 3); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}
	if err := repo.RecordCheckpoint(run1.ID, "boss2", "Vordt", 250000, 150000, 2); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}
	if err := repo.UpdatePersonalBest(routeID, "boss1", 100000, 100000); err != nil {
		t.Fatalf("UpdatePersonalBest: %v", err)
	}
	if err := repo.UpdatePersonalBest(routeID, "boss2", 250000, 150000); err != nil {
		t.Fatalf("UpdatePersonalBest: %v", err)
	}
	if err := repo.EndRouteRun(run1.ID, "completed", 5, 250000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	// Run 2: boss1 faster, boss2 slower
	run2, err := repo.StartRouteRun(routeID, "Dark Souls III", save.ID)
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}
	if err := repo.RecordCheckpoint(run2.ID, "boss1", "Iudex Gundyr", 85000, 85000, 1); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}
	if err := repo.RecordCheckpoint(run2.ID, "boss2", "Vordt", 260000, 175000, 4); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}
	if err := repo.UpdatePersonalBest(routeID, "boss1", 85000, 85000); err != nil {
		t.Fatalf("UpdatePersonalBest: %v", err)
	}
	if err := repo.UpdatePersonalBest(routeID, "boss2", 260000, 175000); err != nil {
		t.Fatalf("UpdatePersonalBest: %v", err)
	}
	if err := repo.EndRouteRun(run2.ID, "completed", 5, 260000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	// Verify PBs kept the best of each
	pbs, err := repo.GetPersonalBest(routeID)
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 2 {
		t.Fatalf("PBs: got %d, want 2", len(pbs))
	}

	pbMap := map[string]model.RoutePB{}
	for _, pb := range pbs {
		pbMap[pb.CheckpointID] = pb
	}

	// boss1: run2 was faster (85000 < 100000)
	if pb := pbMap["boss1"]; pb.BestIGTMs != 85000 {
		t.Errorf("boss1 PB IGT: got %d, want 85000", pb.BestIGTMs)
	}
	if pb := pbMap["boss1"]; pb.BestSplitMs != 85000 {
		t.Errorf("boss1 PB split: got %d, want 85000", pb.BestSplitMs)
	}

	// boss2: run1 was faster (250000 < 260000, 150000 < 175000)
	if pb := pbMap["boss2"]; pb.BestIGTMs != 250000 {
		t.Errorf("boss2 PB IGT: got %d, want 250000", pb.BestIGTMs)
	}
	if pb := pbMap["boss2"]; pb.BestSplitMs != 150000 {
		t.Errorf("boss2 PB split: got %d, want 150000", pb.BestSplitMs)
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
	runA, err := repo.StartRouteRun(routeID, "Dark Souls III", saveA.ID)
	if err != nil {
		t.Fatalf("StartRouteRun(A): %v", err)
	}
	if err := repo.RecordCheckpoint(runA.ID, "boss1", "Iudex Gundyr", 95000, 95000, 2); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}

	// Character changes — abandon run A
	if err := repo.EndRouteRun(runA.ID, "abandoned", 2, 95000); err != nil {
		t.Fatalf("EndRouteRun(abandon): %v", err)
	}

	// Start new run for save B
	runB, err := repo.StartRouteRun(routeID, "Dark Souls III", saveB.ID)
	if err != nil {
		t.Fatalf("StartRouteRun(B): %v", err)
	}

	// FindLatestRun for save A should return the abandoned run
	gotA, err := repo.FindLatestRun(routeID, saveA.ID)
	if err != nil {
		t.Fatalf("FindLatestRun(A): %v", err)
	}
	if gotA.ID != runA.ID {
		t.Errorf("FindLatestRun(A) ID: got %d, want %d", gotA.ID, runA.ID)
	}
	if gotA.Status != "abandoned" {
		t.Errorf("FindLatestRun(A) status: got %q, want abandoned", gotA.Status)
	}

	// FindLatestRun for save B should return the in_progress run
	gotB, err := repo.FindLatestRun(routeID, saveB.ID)
	if err != nil {
		t.Fatalf("FindLatestRun(B): %v", err)
	}
	if gotB.ID != runB.ID {
		t.Errorf("FindLatestRun(B) ID: got %d, want %d", gotB.ID, runB.ID)
	}
	if gotB.Status != "in_progress" {
		t.Errorf("FindLatestRun(B) status: got %q, want in_progress", gotB.Status)
	}

	// Abandoned run's checkpoints should still be intact
	completed, err := repo.LoadCompletedCheckpoints(runA.ID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints(A): %v", err)
	}
	if len(completed) != 1 || completed[0] != "boss1" {
		t.Errorf("abandoned run checkpoints: got %v, want [boss1]", completed)
	}

	if err := repo.EndRouteRun(runB.ID, "completed", 0, 0); err != nil { // cleanup
		t.Fatalf("EndRouteRun: %v", err)
	}
}

// --- Test 5: Run Resume After Restart ---

func TestIntegration_RunResumeAfterRestart(t *testing.T) {
	routeID := "ds3-resume-integ"

	// Phase 1: start run and record some data
	repo1, err := NewRepository(testDBPath)
	if err != nil {
		t.Fatalf("NewRepository (phase 1): %v", err)
	}

	save, _ := repo1.FindOrCreateSave("Dark Souls III", 0, "ResumeKnight")
	run, _ := repo1.StartRouteRun(routeID, "Dark Souls III", save.ID)
	if err := repo1.RecordCheckpoint(run.ID, "boss1", "Iudex Gundyr", 95000, 95000, 2); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}
	if err := repo1.SaveStateVar(run.ID, "embers", 0x400001F4, 3, 5); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}

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
	save2, _ := repo2.FindOrCreateSave("Dark Souls III", 0, "ResumeKnight")
	if save2.ID != save.ID {
		t.Errorf("save ID changed across restart: %d -> %d", save.ID, save2.ID)
	}

	// Find the in-progress run
	found, err := repo2.FindLatestRun(routeID, save2.ID)
	if err != nil {
		t.Fatalf("FindLatestRun: %v", err)
	}
	if found.ID != run.ID {
		t.Errorf("run ID: got %d, want %d", found.ID, run.ID)
	}
	if found.Status != "in_progress" {
		t.Errorf("status: got %q, want in_progress", found.Status)
	}

	// Load completed checkpoints from previous session
	completed, err := repo2.LoadCompletedCheckpoints(found.ID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints: %v", err)
	}
	if len(completed) != 1 || completed[0] != "boss1" {
		t.Errorf("completed checkpoints: got %v, want [boss1]", completed)
	}

	// Load state vars from previous session
	vars, err := repo2.LoadStateVars(found.ID)
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
	if err = repo2.RecordCheckpoint(found.ID, "boss2", "Vordt", 225000, 130000, 1); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}
	if err = repo2.SaveStateVar(found.ID, "embers", 0x400001F4, 5, 8); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}
	if err = repo2.EndRouteRun(found.ID, "completed", 3, 225000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	// Verify final state
	completed, _ = repo2.LoadCompletedCheckpoints(found.ID)
	if len(completed) != 2 {
		t.Errorf("final completed checkpoints: got %d, want 2", len(completed))
	}
	vars, _ = repo2.LoadStateVars(found.ID)
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
	if err := repo.RecordDeathForSave(5, save1.ID); err != nil {
		t.Fatalf("RecordDeathForSave: %v", err)
	}
	if err := repo.RecordDeathForSave(10, save2.ID); err != nil {
		t.Fatalf("RecordDeathForSave: %v", err)
	}

	// Verify sessions are isolated — each save has its own session
	var count1, count2 int
	if err := repo.DB().QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE save_id = ? AND end_time IS NULL", save1.ID,
	).Scan(&count1); err != nil {
		t.Fatalf("QueryRow count1: %v", err)
	}
	if err := repo.DB().QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE save_id = ? AND end_time IS NULL", save2.ID,
	).Scan(&count2); err != nil {
		t.Fatalf("QueryRow count2: %v", err)
	}

	if count1 != 1 {
		t.Errorf("save1 open sessions: got %d, want 1", count1)
	}
	if count2 != 1 {
		t.Errorf("save2 open sessions: got %d, want 1", count2)
	}

	// Verify each session has the correct death count
	var deaths1, deaths2 uint32
	if err := repo.DB().QueryRow(
		"SELECT deaths FROM sessions WHERE save_id = ? AND end_time IS NULL", save1.ID,
	).Scan(&deaths1); err != nil {
		t.Fatalf("QueryRow deaths1: %v", err)
	}
	if err := repo.DB().QueryRow(
		"SELECT deaths FROM sessions WHERE save_id = ? AND end_time IS NULL", save2.ID,
	).Scan(&deaths2); err != nil {
		t.Fatalf("QueryRow deaths2: %v", err)
	}

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

	save, _ := repo.FindOrCreateSave("Dark Souls III", 0, "StateVarTracker")
	run, _ := repo.StartRouteRun("ds3-statevar-integ", "Dark Souls III", save.ID)

	// Initial tick: picked up 3 embers and 5 firebombs
	if err := repo.SaveStateVar(run.ID, "embers", 0x400001F4, 3, 3); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}
	if err := repo.SaveStateVar(run.ID, "firebombs", 0x40000124, 5, 5); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}

	// Tick 2: used 1 ember (qty drops to 2), picked up 2 more firebombs
	if err := repo.SaveStateVar(run.ID, "embers", 0x400001F4, 2, 3); err != nil { // accumulated stays at 3
		t.Fatalf("SaveStateVar: %v", err)
	}
	if err := repo.SaveStateVar(run.ID, "firebombs", 0x40000124, 7, 7); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}

	// Tick 3: picked up 2 embers (qty now 4, net +2 since last positive)
	if err := repo.SaveStateVar(run.ID, "embers", 0x400001F4, 4, 5); err != nil { // accumulated 3+2=5
		t.Fatalf("SaveStateVar: %v", err)
	}
	if err := repo.SaveStateVar(run.ID, "firebombs", 0x40000124, 7, 7); err != nil { // unchanged
		t.Fatalf("SaveStateVar: %v", err)
	}

	// Verify only latest values persisted (no duplicates)
	vars, err := repo.LoadStateVars(run.ID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(vars) != 2 {
		t.Fatalf("expected 2 state vars, got %d", len(vars))
	}

	varMap := map[string]model.RouteStateVar{}
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
		"SELECT COUNT(*) FROM route_state_vars WHERE run_id = ?", run.ID,
	).Scan(&rowCount)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if rowCount != 2 {
		t.Errorf("route_state_vars rows: got %d, want 2", rowCount)
	}

	if err := repo.EndRouteRun(run.ID, "completed", 0, 0); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}
}

// --- Test 8: FindOrCreateSave Upsert Updates last_seen_at ---

func TestIntegration_SaveUpsertUpdatesLastSeen(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	game, slot, name := "Dark Souls III", 0, "UpsertKnight"

	save1, err := repo.FindOrCreateSave(game, slot, name)
	if err != nil {
		t.Fatalf("first FindOrCreateSave: %v", err)
	}

	// Read initial last_seen_at
	var lastSeen1 string
	if err := repo.DB().QueryRow(
		"SELECT last_seen_at FROM saves WHERE id = ?", save1.ID,
	).Scan(&lastSeen1); err != nil {
		t.Fatalf("QueryRow lastSeen1: %v", err)
	}

	// Call again — should return same ID but update last_seen_at
	save2, err := repo.FindOrCreateSave(game, slot, name)
	if err != nil {
		t.Fatalf("second FindOrCreateSave: %v", err)
	}
	if save1.ID != save2.ID {
		t.Errorf("IDs differ: %d vs %d", save1.ID, save2.ID)
	}

	var lastSeen2 string
	if err := repo.DB().QueryRow(
		"SELECT last_seen_at FROM saves WHERE id = ?", save2.ID,
	).Scan(&lastSeen2); err != nil {
		t.Fatalf("QueryRow lastSeen2: %v", err)
	}

	if lastSeen2 < lastSeen1 {
		t.Errorf("last_seen_at did not advance: %s -> %s", lastSeen1, lastSeen2)
	}
}

// --- Test 9: FindLatestRun Returns ErrNotFound for Unknown Route ---

func TestIntegration_FindLatestRunNotFound(t *testing.T) {
	repo := openTestRepo(t)
	defer repo.Close()

	save, _ := repo.FindOrCreateSave("Dark Souls III", 0, "NotFoundKnight")

	_, err := repo.FindLatestRun("nonexistent-route", save.ID)
	if err != ErrNotFound {
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
