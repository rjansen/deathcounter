package route

import (
	"testing"
)

func testRoute() *Route {
	return &Route{
		ID:   "test",
		Name: "Test Route",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagID: 1000},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagID: 2000},
			{ID: "boss3", Name: "Boss 3", EventType: "boss_kill", EventFlagID: 3000},
		},
	}
}

func TestNewRunState(t *testing.T) {
	rs := NewRunState(testRoute())
	if rs.Status != RunNotStarted {
		t.Errorf("got status %q, want %q", rs.Status, RunNotStarted)
	}
	if rs.IsComplete() {
		t.Error("new run should not be complete")
	}
	if rs.CompletionPercent() != 0 {
		t.Errorf("got completion %.1f%%, want 0%%", rs.CompletionPercent())
	}
}

func TestRunState_Start(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()
	if rs.Status != RunInProgress {
		t.Errorf("got status %q, want %q", rs.Status, RunInProgress)
	}
	if rs.StartTime.IsZero() {
		t.Error("start time should be set")
	}
}

func TestRunState_Abandon(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()
	rs.Abandon()
	if rs.Status != RunAbandoned {
		t.Errorf("got status %q, want %q", rs.Status, RunAbandoned)
	}
}

func TestProcessTick_NotStarted(t *testing.T) {
	rs := NewRunState(testRoute())
	flags := map[uint32]bool{1000: true}
	events := rs.ProcessTick(flags, 10000, 5)
	if len(events) != 0 {
		t.Errorf("got %d events from not-started run, want 0", len(events))
	}
}

func TestProcessTick_SingleCheckpoint(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	flags := map[uint32]bool{1000: true}
	events := rs.ProcessTick(flags, 95000, 3)

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Checkpoint.ID != "boss1" {
		t.Errorf("got checkpoint %q, want %q", events[0].Checkpoint.ID, "boss1")
	}
	if events[0].IGT != 95000 {
		t.Errorf("got IGT %d, want 95000", events[0].IGT)
	}
	if events[0].SplitDuration != 95000 {
		t.Errorf("got split duration %d, want 95000", events[0].SplitDuration)
	}
	if events[0].Deaths != 3 {
		t.Errorf("got deaths %d, want 3", events[0].Deaths)
	}
}

func TestProcessTick_FullProgression(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	// Tick 1: Boss 1 killed
	events := rs.ProcessTick(map[uint32]bool{1000: true}, 95000, 3)
	if len(events) != 1 {
		t.Fatalf("tick 1: got %d events, want 1", len(events))
	}
	expected := 100.0 / 3.0
	got := rs.CompletionPercent()
	if got < expected-0.1 || got > expected+0.1 {
		t.Errorf("tick 1: got completion %.1f%%, want ~%.1f%%", got, expected)
	}

	// Tick 2: Boss 2 killed (boss 1 flag still true)
	events = rs.ProcessTick(map[uint32]bool{1000: true, 2000: true}, 225000, 7)
	if len(events) != 1 {
		t.Fatalf("tick 2: got %d events, want 1", len(events))
	}
	if events[0].Checkpoint.ID != "boss2" {
		t.Errorf("tick 2: got checkpoint %q, want %q", events[0].Checkpoint.ID, "boss2")
	}
	if events[0].SplitDuration != 130000 { // 225000 - 95000
		t.Errorf("tick 2: got split duration %d, want 130000", events[0].SplitDuration)
	}
	if events[0].Deaths != 4 { // 7 - 3
		t.Errorf("tick 2: got deaths %d, want 4", events[0].Deaths)
	}

	// Tick 3: Boss 3 killed - run completes
	events = rs.ProcessTick(map[uint32]bool{1000: true, 2000: true, 3000: true}, 400000, 10)
	if len(events) != 1 {
		t.Fatalf("tick 3: got %d events, want 1", len(events))
	}
	if rs.Status != RunCompleted {
		t.Errorf("expected run to be completed, got %q", rs.Status)
	}
	if rs.CompletionPercent() != 100.0 {
		t.Errorf("got completion %.1f%%, want 100%%", rs.CompletionPercent())
	}
}

func TestProcessTick_NoNewFlags(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	events := rs.ProcessTick(map[uint32]bool{}, 10000, 0)
	if len(events) != 0 {
		t.Errorf("got %d events for no flags, want 0", len(events))
	}
}

func TestProcessTick_AfterAbandon(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()
	rs.Abandon()

	events := rs.ProcessTick(map[uint32]bool{1000: true}, 10000, 0)
	if len(events) != 0 {
		t.Errorf("got %d events after abandon, want 0", len(events))
	}
}

func TestCurrentCheckpoint(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	cp := rs.CurrentCheckpoint()
	if cp == nil {
		t.Fatal("expected non-nil checkpoint")
	}
	if cp.ID != "boss1" {
		t.Errorf("got checkpoint %q, want %q", cp.ID, "boss1")
	}

	// Complete boss1
	rs.ProcessTick(map[uint32]bool{1000: true}, 95000, 0)
	cp = rs.CurrentCheckpoint()
	if cp == nil {
		t.Fatal("expected non-nil checkpoint")
	}
	if cp.ID != "boss2" {
		t.Errorf("got checkpoint %q, want %q", cp.ID, "boss2")
	}
}

func TestCurrentCheckpoint_AllDone(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	rs.ProcessTick(map[uint32]bool{1000: true, 2000: true, 3000: true}, 400000, 0)
	cp := rs.CurrentCheckpoint()
	if cp != nil {
		t.Errorf("expected nil checkpoint when all done, got %q", cp.ID)
	}
}

func TestOptionalCheckpoints(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagID: 1000},
			{ID: "optional1", Name: "Optional", EventType: "boss_kill", EventFlagID: 5000, Optional: true},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagID: 2000},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Complete required bosses only (skip optional)
	rs.ProcessTick(map[uint32]bool{1000: true}, 95000, 0)
	if rs.CompletionPercent() != 50.0 {
		t.Errorf("got completion %.1f%%, want 50%%", rs.CompletionPercent())
	}

	rs.ProcessTick(map[uint32]bool{1000: true, 2000: true}, 225000, 0)
	if !rs.IsComplete() {
		t.Error("run should be complete (optional skipped)")
	}
	if rs.Status != RunCompleted {
		t.Errorf("expected completed status, got %q", rs.Status)
	}

	// CurrentCheckpoint should skip optional
	rs2 := NewRunState(route)
	rs2.Start()
	cp := rs2.CurrentCheckpoint()
	if cp.ID != "boss1" {
		t.Errorf("got %q, want boss1", cp.ID)
	}
	rs2.CompletedFlags["boss1"] = true
	cp = rs2.CurrentCheckpoint()
	if cp.ID != "boss2" {
		t.Errorf("got %q, want boss2 (skip optional)", cp.ID)
	}
}

func TestMultipleCheckpointsSameTick(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	// Both boss1 and boss2 flagged in same tick
	events := rs.ProcessTick(map[uint32]bool{1000: true, 2000: true}, 200000, 5)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Checkpoint.ID != "boss1" {
		t.Errorf("first event: got %q, want boss1", events[0].Checkpoint.ID)
	}
	if events[1].Checkpoint.ID != "boss2" {
		t.Errorf("second event: got %q, want boss2", events[1].Checkpoint.ID)
	}
}
