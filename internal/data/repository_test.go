package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rjansen/deathcounter/internal/data/model"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	t.Cleanup(func() { repo.db.Close() })
	return repo
}

func TestNewRepository(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	defer repo.db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Verify tables exist
	var name string
	err = repo.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&name)
	if err != nil {
		t.Fatal("sessions table not created")
	}
	err = repo.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='death_events'").Scan(&name)
	if err != nil {
		t.Fatal("death_events table not created")
	}
}

func TestNewRepository_InvalidPath(t *testing.T) {
	_, err := NewRepository("/nonexistent/path/test.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestRecordDeath_CreatesSession(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.RecordDeath(1); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}

	deaths, err := repo.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 1 {
		t.Errorf("got %d deaths, want 1", deaths)
	}
}

func TestRecordDeath_ReusesOpenSession(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.RecordDeath(1); err != nil {
		t.Fatalf("first RecordDeath: %v", err)
	}
	if err := repo.RecordDeath(2); err != nil {
		t.Fatalf("second RecordDeath: %v", err)
	}

	sessions, err := repo.GetSessionHistory(10)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Deaths != 2 {
		t.Errorf("got %d deaths, want 2", sessions[0].Deaths)
	}
}

func TestRecordDeath_InsertsDeathEvents(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.RecordDeath(1); err != nil {
		t.Fatalf("first RecordDeath: %v", err)
	}
	if err := repo.RecordDeath(2); err != nil {
		t.Fatalf("second RecordDeath: %v", err)
	}

	var count int
	err := repo.db.QueryRow("SELECT COUNT(*) FROM death_events").Scan(&count)
	if err != nil {
		t.Fatalf("query death_events: %v", err)
	}
	if count != 2 {
		t.Errorf("got %d death events, want 2", count)
	}
}

func TestEndCurrentSession(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.RecordDeath(3); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}
	if err := repo.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}

	deaths, err := repo.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 0 {
		t.Errorf("got %d current session deaths after ending, want 0", deaths)
	}
}

func TestEndCurrentSession_NoOpenSession(t *testing.T) {
	repo := newTestRepository(t)

	// Should not error when there's nothing to end
	if err := repo.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}
}

func TestGetTotalDeaths_Empty(t *testing.T) {
	repo := newTestRepository(t)

	total, err := repo.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	if total != 0 {
		t.Errorf("got %d total deaths, want 0", total)
	}
}

func TestGetTotalDeaths_AcrossSessions(t *testing.T) {
	repo := newTestRepository(t)

	// Session 1: 5 deaths
	if err := repo.RecordDeath(5); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}
	if err := repo.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}

	// Session 2: 3 deaths
	if err := repo.RecordDeath(3); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}

	total, err := repo.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	if total != 8 {
		t.Errorf("got %d total deaths, want 8", total)
	}
}

func TestGetCurrentSessionDeaths_NoSession(t *testing.T) {
	repo := newTestRepository(t)

	deaths, err := repo.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 0 {
		t.Errorf("got %d deaths, want 0", deaths)
	}
}

func TestGetSessionHistory(t *testing.T) {
	repo := newTestRepository(t)

	// Create 3 sessions
	for i := uint32(1); i <= 3; i++ {
		if err := repo.RecordDeath(i * 10); err != nil {
			t.Fatalf("RecordDeath: %v", err)
		}
		if err := repo.EndCurrentSession(); err != nil {
			t.Fatalf("EndCurrentSession: %v", err)
		}
	}

	sessions, err := repo.GetSessionHistory(10)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("got %d sessions, want 3", len(sessions))
	}

	// Sessions are ordered by start_time DESC
	if sessions[0].Deaths != 30 {
		t.Errorf("most recent session: got %d deaths, want 30", sessions[0].Deaths)
	}
	if sessions[2].Deaths != 10 {
		t.Errorf("oldest session: got %d deaths, want 10", sessions[2].Deaths)
	}

	// Ended sessions should have EndTime set
	for i, s := range sessions {
		if s.EndTime == nil {
			t.Errorf("session %d: EndTime is nil, want non-nil", i)
		}
	}
}

func TestGetSessionHistory_Limit(t *testing.T) {
	repo := newTestRepository(t)

	for i := uint32(1); i <= 5; i++ {
		if err := repo.RecordDeath(i); err != nil {
			t.Fatalf("RecordDeath: %v", err)
		}
		if err := repo.EndCurrentSession(); err != nil {
			t.Fatalf("EndCurrentSession: %v", err)
		}
	}

	sessions, err := repo.GetSessionHistory(2)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(sessions))
	}
}

func TestGetSessionHistory_Empty(t *testing.T) {
	repo := newTestRepository(t)

	sessions, err := repo.GetSessionHistory(10)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessions))
	}
}

func TestGetSessionHistory_OpenSession(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.RecordDeath(7); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}

	sessions, err := repo.GetSessionHistory(10)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].EndTime != nil {
		t.Error("open session should have nil EndTime")
	}
	if sessions[0].Deaths != 7 {
		t.Errorf("got %d deaths, want 7", sessions[0].Deaths)
	}
}

// --- Route run tests ---

func TestStartRouteRun(t *testing.T) {
	repo := newTestRepository(t)

	run, err := repo.StartRouteRun("ds3-any-percent", "Dark Souls III", 0)
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}
	if run.ID <= 0 {
		t.Errorf("got run ID %d, want > 0", run.ID)
	}
	if run.Status != "in_progress" {
		t.Errorf("got status %q, want in_progress", run.Status)
	}
}

func TestRecordCheckpoint(t *testing.T) {
	repo := newTestRepository(t)

	run, err := repo.StartRouteRun("ds3-any-percent", "Dark Souls III", 0)
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}

	if err := repo.RecordCheckpoint(run.ID, "boss1", "Iudex Gundyr", 95000, 95000, 3); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}

	var count int
	err = repo.db.QueryRow("SELECT COUNT(*) FROM route_checkpoints WHERE run_id = ?", run.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d checkpoints, want 1", count)
	}
}

func TestEndRouteRun(t *testing.T) {
	repo := newTestRepository(t)

	run, _ := repo.StartRouteRun("ds3-any-percent", "Dark Souls III", 0)
	if err := repo.RecordCheckpoint(run.ID, "boss1", "Boss 1", 95000, 95000, 2); err != nil {
		t.Fatal(err)
	}

	if err := repo.EndRouteRun(run.ID, "completed", 10, 400000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	var status string
	var totalDeaths int
	var finalIGT int64
	err := repo.db.QueryRow("SELECT status, total_deaths, final_igt_ms FROM route_runs WHERE id = ?", run.ID).
		Scan(&status, &totalDeaths, &finalIGT)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "completed" {
		t.Errorf("got status %q, want %q", status, "completed")
	}
	if totalDeaths != 10 {
		t.Errorf("got total deaths %d, want 10", totalDeaths)
	}
	if finalIGT != 400000 {
		t.Errorf("got final IGT %d, want 400000", finalIGT)
	}
}

func TestUpdatePersonalBest_NewPB(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.UpdatePersonalBest("ds3", "boss1", 95000, 95000); err != nil {
		t.Fatalf("UpdatePersonalBest: %v", err)
	}

	pbs, err := repo.GetPersonalBest("ds3")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 1 {
		t.Fatalf("got %d PBs, want 1", len(pbs))
	}
	if pbs[0].BestIGTMs != 95000 {
		t.Errorf("got IGT %d, want 95000", pbs[0].BestIGTMs)
	}
}

func TestUpdatePersonalBest_BetterTime(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.UpdatePersonalBest("ds3", "boss1", 95000, 95000); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdatePersonalBest("ds3", "boss1", 90000, 88000); err != nil { // better
		t.Fatal(err)
	}

	pbs, err := repo.GetPersonalBest("ds3")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if pbs[0].BestIGTMs != 90000 {
		t.Errorf("got IGT %d, want 90000", pbs[0].BestIGTMs)
	}
	if pbs[0].BestSplitMs != 88000 {
		t.Errorf("got split %d, want 88000", pbs[0].BestSplitMs)
	}
}

func TestUpdatePersonalBest_WorseTime(t *testing.T) {
	repo := newTestRepository(t)

	if err := repo.UpdatePersonalBest("ds3", "boss1", 90000, 90000); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdatePersonalBest("ds3", "boss1", 95000, 95000); err != nil { // worse
		t.Fatal(err)
	}

	pbs, err := repo.GetPersonalBest("ds3")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if pbs[0].BestIGTMs != 90000 {
		t.Errorf("got IGT %d, want 90000 (should keep better time)", pbs[0].BestIGTMs)
	}
}

func TestGetPersonalBest_Empty(t *testing.T) {
	repo := newTestRepository(t)

	pbs, err := repo.GetPersonalBest("nonexistent")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 0 {
		t.Errorf("got %d PBs, want 0", len(pbs))
	}
}

func TestRouteRunLifecycle(t *testing.T) {
	repo := newTestRepository(t)

	// Start run
	run, err := repo.StartRouteRun("ds3-any", "Dark Souls III", 0)
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}

	// Record checkpoints
	if err := repo.RecordCheckpoint(run.ID, "boss1", "Iudex Gundyr", 95000, 95000, 3); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordCheckpoint(run.ID, "boss2", "Vordt", 225000, 130000, 2); err != nil {
		t.Fatal(err)
	}

	// Update PBs
	if err := repo.UpdatePersonalBest("ds3-any", "boss1", 95000, 95000); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdatePersonalBest("ds3-any", "boss2", 225000, 130000); err != nil {
		t.Fatal(err)
	}

	// End run
	if err := repo.EndRouteRun(run.ID, "completed", 5, 225000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	// Verify PBs
	pbs, err := repo.GetPersonalBest("ds3-any")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 2 {
		t.Errorf("got %d PBs, want 2", len(pbs))
	}
}

func TestClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}

	if err := repo.RecordDeath(5); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and verify session was ended and data persisted
	repo2, err := NewRepository(dbPath)
	if err != nil {
		t.Fatalf("NewRepository (reopen): %v", err)
	}
	defer repo2.db.Close()

	// No open session should exist
	deaths, err := repo2.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 0 {
		t.Errorf("got %d current deaths after reopen, want 0", deaths)
	}

	// Total should be preserved
	total, err := repo2.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	if total != 5 {
		t.Errorf("got %d total deaths after reopen, want 5", total)
	}
}

// --- Save slot tests ---

func TestFindOrCreateSave_New(t *testing.T) {
	repo := newTestRepository(t)

	save, err := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")
	if err != nil {
		t.Fatalf("FindOrCreateSave: %v", err)
	}
	if save.ID <= 0 {
		t.Errorf("expected id > 0, got %d", save.ID)
	}
	if save.Game != "Dark Souls III" {
		t.Errorf("got game %q, want Dark Souls III", save.Game)
	}
	if save.CharacterName != "Knight" {
		t.Errorf("got name %q, want Knight", save.CharacterName)
	}
}

func TestFindOrCreateSave_Existing(t *testing.T) {
	repo := newTestRepository(t)

	save1, err := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	save2, err := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if save1.ID != save2.ID {
		t.Errorf("expected same ID, got %d and %d", save1.ID, save2.ID)
	}
}

func TestFindOrCreateSave_DifferentSlot(t *testing.T) {
	repo := newTestRepository(t)

	save1, _ := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")
	save2, _ := repo.FindOrCreateSave("Dark Souls III", 1, "Knight")

	if save1.ID == save2.ID {
		t.Error("different slots should produce different IDs")
	}
}

func TestFindOrCreateSave_DifferentName(t *testing.T) {
	repo := newTestRepository(t)

	save1, _ := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")
	save2, _ := repo.FindOrCreateSave("Dark Souls III", 0, "Pyromancer")

	if save1.ID == save2.ID {
		t.Error("different names on same slot should produce different IDs")
	}
}

func TestStartRouteRun_WithSaveID(t *testing.T) {
	repo := newTestRepository(t)

	save, _ := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")
	run, err := repo.StartRouteRun("ds3-any", "Dark Souls III", save.ID)
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}

	var storedSaveID int64
	err = repo.db.QueryRow("SELECT save_id FROM route_runs WHERE id = ?", run.ID).Scan(&storedSaveID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if storedSaveID != save.ID {
		t.Errorf("expected save_id %d, got %d", save.ID, storedSaveID)
	}
}

func TestRecordDeathForSave(t *testing.T) {
	repo := newTestRepository(t)

	save, _ := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")
	if err := repo.RecordDeathForSave(5, save.ID); err != nil {
		t.Fatalf("RecordDeathForSave: %v", err)
	}

	// Verify session was created with save_id
	var sessionSaveID int64
	err := repo.db.QueryRow(
		"SELECT save_id FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1",
	).Scan(&sessionSaveID)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if sessionSaveID != save.ID {
		t.Errorf("expected session save_id %d, got %d", save.ID, sessionSaveID)
	}
}

func TestGetOrCreateSessionForSave(t *testing.T) {
	repo := newTestRepository(t)

	save, _ := repo.FindOrCreateSave("Dark Souls III", 0, "Knight")

	// First call creates a session
	s1, err := repo.GetOrCreateSessionForSave(save.ID)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if s1.ID <= 0 {
		t.Errorf("expected session id > 0, got %d", s1.ID)
	}

	// Second call reuses the same session
	s2, err := repo.GetOrCreateSessionForSave(save.ID)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if s1.ID != s2.ID {
		t.Errorf("expected same session ID, got %d and %d", s1.ID, s2.ID)
	}
}

func TestSaveAndLoadStateVars(t *testing.T) {
	repo := newTestRepository(t)

	run, _ := repo.StartRouteRun("ds3-any", "Dark Souls III", 0)

	if err := repo.SaveStateVar(run.ID, "embers", 0x400001F4, 3, 5, 1); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}
	if err := repo.SaveStateVar(run.ID, "firebombs", 0x40000124, 2, 2, 0); err != nil {
		t.Fatalf("SaveStateVar: %v", err)
	}

	rows, err := repo.LoadStateVars(run.ID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 state vars, got %d", len(rows))
	}

	// Find the embers row
	var embers *model.RouteStateVar
	for i := range rows {
		if rows[i].VarName == "embers" {
			embers = &rows[i]
		}
	}
	if embers == nil {
		t.Fatal("embers state var not found")
	}
	if embers.ItemID != 0x400001F4 {
		t.Errorf("expected item_id %d, got %d", 0x400001F4, embers.ItemID)
	}
	if embers.LastQuantity != 3 {
		t.Errorf("expected last_quantity 3, got %d", embers.LastQuantity)
	}
	if embers.Acquired != 5 {
		t.Errorf("expected acquired 5, got %d", embers.Acquired)
	}
	if embers.Consumed != 1 {
		t.Errorf("expected consumed 1, got %d", embers.Consumed)
	}
}

func TestSaveStateVar_Upsert(t *testing.T) {
	repo := newTestRepository(t)

	run, _ := repo.StartRouteRun("ds3-any", "Dark Souls III", 0)

	if err := repo.SaveStateVar(run.ID, "embers", 0x400001F4, 2, 2, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveStateVar(run.ID, "embers", 0x400001F4, 5, 7, 1); err != nil { // update
		t.Fatal(err)
	}

	rows, err := repo.LoadStateVars(run.ID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 state var after upsert, got %d", len(rows))
	}
	if rows[0].LastQuantity != 5 {
		t.Errorf("expected last_quantity 5, got %d", rows[0].LastQuantity)
	}
	if rows[0].Acquired != 7 {
		t.Errorf("expected acquired 7, got %d", rows[0].Acquired)
	}
	if rows[0].Consumed != 1 {
		t.Errorf("expected consumed 1, got %d", rows[0].Consumed)
	}
}

func TestLoadStateVars_Empty(t *testing.T) {
	repo := newTestRepository(t)

	run, _ := repo.StartRouteRun("ds3-any", "Dark Souls III", 0)

	rows, err := repo.LoadStateVars(run.ID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 state vars, got %d", len(rows))
	}
}

func TestLoadCompletedCheckpoints(t *testing.T) {
	repo := newTestRepository(t)

	run, _ := repo.StartRouteRun("ds3-any", "Dark Souls III", 0)

	// No checkpoints yet
	ids, err := repo.LoadCompletedCheckpoints(run.ID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 checkpoints, got %d", len(ids))
	}

	// Record some checkpoints
	if err = repo.RecordCheckpoint(run.ID, "boss1", "Iudex Gundyr", 95000, 95000, 3); err != nil {
		t.Fatalf("RecordCheckpoint boss1: %v", err)
	}
	if err = repo.RecordCheckpoint(run.ID, "boss2", "Vordt", 225000, 130000, 2); err != nil {
		t.Fatalf("RecordCheckpoint boss2: %v", err)
	}

	ids, err = repo.LoadCompletedCheckpoints(run.ID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 checkpoints, got %d", len(ids))
	}

	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found["boss1"] || !found["boss2"] {
		t.Errorf("expected boss1 and boss2, got %v", ids)
	}
}

func TestLoadCompletedCheckpoints_CaughtUp(t *testing.T) {
	repo := newTestRepository(t)

	run, _ := repo.StartRouteRun("ds3-any", "Dark Souls III", 0)

	// Caught-up checkpoints have IGT=0, duration=0, deaths=0
	if err := repo.RecordCheckpoint(run.ID, "boss1", "Iudex Gundyr", 0, 0, 0); err != nil {
		t.Fatalf("RecordCheckpoint boss1: %v", err)
	}

	ids, err := repo.LoadCompletedCheckpoints(run.ID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints: %v", err)
	}
	if len(ids) != 1 || ids[0] != "boss1" {
		t.Errorf("expected [boss1], got %v", ids)
	}
}

func TestFindLatestRun_NoRun(t *testing.T) {
	repo := newTestRepository(t)
	save, _ := repo.FindOrCreateSave("ds3", 0, "Knight")

	_, err := repo.FindLatestRun("ds3-any", save.ID)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFindLatestRun_InProgress(t *testing.T) {
	repo := newTestRepository(t)
	save, _ := repo.FindOrCreateSave("ds3", 0, "Knight")

	run, _ := repo.StartRouteRun("ds3-any", "ds3", save.ID)

	got, err := repo.FindLatestRun("ds3-any", save.ID)
	if err != nil {
		t.Fatalf("FindLatestRun: %v", err)
	}
	if got.ID != run.ID {
		t.Errorf("expected run ID %d, got %d", run.ID, got.ID)
	}
	if got.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", got.Status)
	}
}

func TestFindLatestRun_Completed(t *testing.T) {
	repo := newTestRepository(t)
	save, _ := repo.FindOrCreateSave("ds3", 0, "Knight")

	run, _ := repo.StartRouteRun("ds3-any", "ds3", save.ID)
	if err := repo.EndRouteRun(run.ID, "completed", 10, 400000); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindLatestRun("ds3-any", save.ID)
	if err != nil {
		t.Fatalf("FindLatestRun: %v", err)
	}
	if got.ID != run.ID {
		t.Errorf("expected run ID %d, got %d", run.ID, got.ID)
	}
	if got.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", got.Status)
	}
}

func TestFindLatestRun_DifferentSave(t *testing.T) {
	repo := newTestRepository(t)
	save1, _ := repo.FindOrCreateSave("ds3", 0, "Knight")
	save2, _ := repo.FindOrCreateSave("ds3", 1, "Pyromancer")

	if _, err := repo.StartRouteRun("ds3-any", "ds3", save1.ID); err != nil {
		t.Fatal(err)
	}

	_, err := repo.FindLatestRun("ds3-any", save2.ID)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMigration(t *testing.T) {
	// Verify that creating a repo runs migration without error
	repo := newTestRepository(t)

	// Check sessions has save_id column
	// Verify by trying to insert with save_id
	_, err := repo.db.Exec(
		"INSERT INTO sessions (start_time, deaths, save_id) VALUES (?, 0, NULL)", time.Now())
	if err != nil {
		t.Errorf("sessions should accept save_id column: %v", err)
	}
	_, err = repo.db.Exec(
		"INSERT INTO route_runs (route_id, game, status, start_time, save_id) VALUES ('test', 'test', 'in_progress', ?, NULL)", time.Now())
	if err != nil {
		t.Errorf("route_runs should accept save_id column: %v", err)
	}
}
