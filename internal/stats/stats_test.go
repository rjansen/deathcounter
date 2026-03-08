package stats

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestTracker(t *testing.T) *Tracker {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := NewTracker(dbPath)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	t.Cleanup(func() { tracker.db.Close() })
	return tracker
}

func TestNewTracker(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := NewTracker(dbPath)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tracker.db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Verify tables exist
	var name string
	err = tracker.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&name)
	if err != nil {
		t.Fatal("sessions table not created")
	}
	err = tracker.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='death_events'").Scan(&name)
	if err != nil {
		t.Fatal("death_events table not created")
	}
}

func TestNewTracker_InvalidPath(t *testing.T) {
	_, err := NewTracker("/nonexistent/path/test.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestRecordDeath_CreatesSession(t *testing.T) {
	tracker := newTestTracker(t)

	if err := tracker.RecordDeath(1); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}

	deaths, err := tracker.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 1 {
		t.Errorf("got %d deaths, want 1", deaths)
	}
}

func TestRecordDeath_ReusesOpenSession(t *testing.T) {
	tracker := newTestTracker(t)

	if err := tracker.RecordDeath(1); err != nil {
		t.Fatalf("first RecordDeath: %v", err)
	}
	if err := tracker.RecordDeath(2); err != nil {
		t.Fatalf("second RecordDeath: %v", err)
	}

	sessions, err := tracker.GetSessionHistory(10)
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
	tracker := newTestTracker(t)

	if err := tracker.RecordDeath(1); err != nil {
		t.Fatalf("first RecordDeath: %v", err)
	}
	if err := tracker.RecordDeath(2); err != nil {
		t.Fatalf("second RecordDeath: %v", err)
	}

	var count int
	err := tracker.db.QueryRow("SELECT COUNT(*) FROM death_events").Scan(&count)
	if err != nil {
		t.Fatalf("query death_events: %v", err)
	}
	if count != 2 {
		t.Errorf("got %d death events, want 2", count)
	}
}

func TestEndCurrentSession(t *testing.T) {
	tracker := newTestTracker(t)

	if err := tracker.RecordDeath(3); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}
	if err := tracker.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}

	deaths, err := tracker.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 0 {
		t.Errorf("got %d current session deaths after ending, want 0", deaths)
	}
}

func TestEndCurrentSession_NoOpenSession(t *testing.T) {
	tracker := newTestTracker(t)

	// Should not error when there's nothing to end
	if err := tracker.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}
}

func TestGetTotalDeaths_Empty(t *testing.T) {
	tracker := newTestTracker(t)

	total, err := tracker.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	if total != 0 {
		t.Errorf("got %d total deaths, want 0", total)
	}
}

func TestGetTotalDeaths_AcrossSessions(t *testing.T) {
	tracker := newTestTracker(t)

	// Session 1: 5 deaths
	if err := tracker.RecordDeath(5); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}
	if err := tracker.EndCurrentSession(); err != nil {
		t.Fatalf("EndCurrentSession: %v", err)
	}

	// Session 2: 3 deaths
	if err := tracker.RecordDeath(3); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}

	total, err := tracker.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	if total != 8 {
		t.Errorf("got %d total deaths, want 8", total)
	}
}

func TestGetCurrentSessionDeaths_NoSession(t *testing.T) {
	tracker := newTestTracker(t)

	deaths, err := tracker.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 0 {
		t.Errorf("got %d deaths, want 0", deaths)
	}
}

func TestGetSessionHistory(t *testing.T) {
	tracker := newTestTracker(t)

	// Create 3 sessions
	for i := uint32(1); i <= 3; i++ {
		if err := tracker.RecordDeath(i * 10); err != nil {
			t.Fatalf("RecordDeath: %v", err)
		}
		if err := tracker.EndCurrentSession(); err != nil {
			t.Fatalf("EndCurrentSession: %v", err)
		}
	}

	sessions, err := tracker.GetSessionHistory(10)
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
	tracker := newTestTracker(t)

	for i := uint32(1); i <= 5; i++ {
		if err := tracker.RecordDeath(i); err != nil {
			t.Fatalf("RecordDeath: %v", err)
		}
		if err := tracker.EndCurrentSession(); err != nil {
			t.Fatalf("EndCurrentSession: %v", err)
		}
	}

	sessions, err := tracker.GetSessionHistory(2)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(sessions))
	}
}

func TestGetSessionHistory_Empty(t *testing.T) {
	tracker := newTestTracker(t)

	sessions, err := tracker.GetSessionHistory(10)
	if err != nil {
		t.Fatalf("GetSessionHistory: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessions))
	}
}

func TestGetSessionHistory_OpenSession(t *testing.T) {
	tracker := newTestTracker(t)

	if err := tracker.RecordDeath(7); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}

	sessions, err := tracker.GetSessionHistory(10)
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
	tracker := newTestTracker(t)

	runID, err := tracker.StartRouteRun("ds3-any-percent", "Dark Souls III")
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}
	if runID <= 0 {
		t.Errorf("got run ID %d, want > 0", runID)
	}
}

func TestRecordSplit(t *testing.T) {
	tracker := newTestTracker(t)

	runID, err := tracker.StartRouteRun("ds3-any-percent", "Dark Souls III")
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}

	if err := tracker.RecordSplit(runID, "boss1", "Iudex Gundyr", 95000, 95000, 3); err != nil {
		t.Fatalf("RecordSplit: %v", err)
	}

	var count int
	err = tracker.db.QueryRow("SELECT COUNT(*) FROM route_splits WHERE run_id = ?", runID).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d splits, want 1", count)
	}
}

func TestEndRouteRun(t *testing.T) {
	tracker := newTestTracker(t)

	runID, _ := tracker.StartRouteRun("ds3-any-percent", "Dark Souls III")
	tracker.RecordSplit(runID, "boss1", "Boss 1", 95000, 95000, 2)

	if err := tracker.EndRouteRun(runID, "completed", 10, 400000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	var status string
	var totalDeaths int
	var finalIGT int64
	err := tracker.db.QueryRow("SELECT status, total_deaths, final_igt_ms FROM route_runs WHERE id = ?", runID).
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
	tracker := newTestTracker(t)

	if err := tracker.UpdatePersonalBest("ds3", "boss1", 95000, 95000); err != nil {
		t.Fatalf("UpdatePersonalBest: %v", err)
	}

	pbs, err := tracker.GetPersonalBest("ds3")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 1 {
		t.Fatalf("got %d PBs, want 1", len(pbs))
	}
	if pbs[0].IGTMs != 95000 {
		t.Errorf("got IGT %d, want 95000", pbs[0].IGTMs)
	}
}

func TestUpdatePersonalBest_BetterTime(t *testing.T) {
	tracker := newTestTracker(t)

	tracker.UpdatePersonalBest("ds3", "boss1", 95000, 95000)
	tracker.UpdatePersonalBest("ds3", "boss1", 90000, 88000) // better

	pbs, err := tracker.GetPersonalBest("ds3")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if pbs[0].IGTMs != 90000 {
		t.Errorf("got IGT %d, want 90000", pbs[0].IGTMs)
	}
	if pbs[0].SplitDurationMs != 88000 {
		t.Errorf("got split %d, want 88000", pbs[0].SplitDurationMs)
	}
}

func TestUpdatePersonalBest_WorseTime(t *testing.T) {
	tracker := newTestTracker(t)

	tracker.UpdatePersonalBest("ds3", "boss1", 90000, 90000)
	tracker.UpdatePersonalBest("ds3", "boss1", 95000, 95000) // worse

	pbs, err := tracker.GetPersonalBest("ds3")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if pbs[0].IGTMs != 90000 {
		t.Errorf("got IGT %d, want 90000 (should keep better time)", pbs[0].IGTMs)
	}
}

func TestGetPersonalBest_Empty(t *testing.T) {
	tracker := newTestTracker(t)

	pbs, err := tracker.GetPersonalBest("nonexistent")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 0 {
		t.Errorf("got %d PBs, want 0", len(pbs))
	}
}

func TestRouteRunLifecycle(t *testing.T) {
	tracker := newTestTracker(t)

	// Start run
	runID, err := tracker.StartRouteRun("ds3-any", "Dark Souls III")
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}

	// Record splits
	tracker.RecordSplit(runID, "boss1", "Iudex Gundyr", 95000, 95000, 3)
	tracker.RecordSplit(runID, "boss2", "Vordt", 225000, 130000, 2)

	// Update PBs
	tracker.UpdatePersonalBest("ds3-any", "boss1", 95000, 95000)
	tracker.UpdatePersonalBest("ds3-any", "boss2", 225000, 130000)

	// End run
	if err := tracker.EndRouteRun(runID, "completed", 5, 225000); err != nil {
		t.Fatalf("EndRouteRun: %v", err)
	}

	// Verify PBs
	pbs, err := tracker.GetPersonalBest("ds3-any")
	if err != nil {
		t.Fatalf("GetPersonalBest: %v", err)
	}
	if len(pbs) != 2 {
		t.Errorf("got %d PBs, want 2", len(pbs))
	}
}

func TestClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := NewTracker(dbPath)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	if err := tracker.RecordDeath(5); err != nil {
		t.Fatalf("RecordDeath: %v", err)
	}
	if err := tracker.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and verify session was ended and data persisted
	tracker2, err := NewTracker(dbPath)
	if err != nil {
		t.Fatalf("NewTracker (reopen): %v", err)
	}
	defer tracker2.db.Close()

	// No open session should exist
	deaths, err := tracker2.GetCurrentSessionDeaths()
	if err != nil {
		t.Fatalf("GetCurrentSessionDeaths: %v", err)
	}
	if deaths != 0 {
		t.Errorf("got %d current deaths after reopen, want 0", deaths)
	}

	// Total should be preserved
	total, err := tracker2.GetTotalDeaths()
	if err != nil {
		t.Fatalf("GetTotalDeaths: %v", err)
	}
	if total != 5 {
		t.Errorf("got %d total deaths after reopen, want 5", total)
	}
}
