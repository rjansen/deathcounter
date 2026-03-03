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
