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
	result := rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true}, IGT: 10000, DeathCount: 5})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events from not-started run, want 0", len(result.Checkpoints))
	}
}

func TestProcessTick_SingleCheckpoint(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	result := rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true}, IGT: 95000, DeathCount: 3})

	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss1" {
		t.Errorf("got checkpoint %q, want %q", result.Checkpoints[0].Checkpoint.ID, "boss1")
	}
	if result.Checkpoints[0].IGT != 95000 {
		t.Errorf("got IGT %d, want 95000", result.Checkpoints[0].IGT)
	}
	if result.Checkpoints[0].CheckpointDuration != 95000 {
		t.Errorf("got split duration %d, want 95000", result.Checkpoints[0].CheckpointDuration)
	}
	if result.Checkpoints[0].Deaths != 3 {
		t.Errorf("got deaths %d, want 3", result.Checkpoints[0].Deaths)
	}
}

func TestProcessTick_FullProgression(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	// Tick 1: Boss 1 killed
	result := rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true}, IGT: 95000, DeathCount: 3})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("tick 1: got %d events, want 1", len(result.Checkpoints))
	}
	expected := 100.0 / 3.0
	got := rs.CompletionPercent()
	if got < expected-0.1 || got > expected+0.1 {
		t.Errorf("tick 1: got completion %.1f%%, want ~%.1f%%", got, expected)
	}

	// Tick 2: Boss 2 killed (boss 1 flag still true)
	result = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true}, IGT: 225000, DeathCount: 7})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("tick 2: got %d events, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss2" {
		t.Errorf("tick 2: got checkpoint %q, want %q", result.Checkpoints[0].Checkpoint.ID, "boss2")
	}
	if result.Checkpoints[0].CheckpointDuration != 130000 { // 225000 - 95000
		t.Errorf("tick 2: got split duration %d, want 130000", result.Checkpoints[0].CheckpointDuration)
	}
	if result.Checkpoints[0].Deaths != 4 { // 7 - 3
		t.Errorf("tick 2: got deaths %d, want 4", result.Checkpoints[0].Deaths)
	}

	// Tick 3: Boss 3 killed - run completes
	result = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true, 3000: true}, IGT: 400000, DeathCount: 10})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("tick 3: got %d events, want 1", len(result.Checkpoints))
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

	result := rs.ProcessTick(TickInput{Flags: map[uint32]bool{}, IGT: 10000, DeathCount: 0})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events for no flags, want 0", len(result.Checkpoints))
	}
}

func TestProcessTick_AfterAbandon(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()
	rs.Abandon()

	result := rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true}, IGT: 10000, DeathCount: 0})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events after abandon, want 0", len(result.Checkpoints))
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
	_ = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true}, IGT: 95000, DeathCount: 0})
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

	_ = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true, 3000: true}, IGT: 400000, DeathCount: 0})
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
	_ = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true}, IGT: 95000, DeathCount: 0})
	if rs.CompletionPercent() != 50.0 {
		t.Errorf("got completion %.1f%%, want 50%%", rs.CompletionPercent())
	}

	_ = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true}, IGT: 225000, DeathCount: 0})
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
	result := rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true}, IGT: 200000, DeathCount: 5})
	if len(result.Checkpoints) != 2 {
		t.Fatalf("got %d events, want 2", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss1" {
		t.Errorf("first event: got %q, want boss1", result.Checkpoints[0].Checkpoint.ID)
	}
	if result.Checkpoints[1].Checkpoint.ID != "boss2" {
		t.Errorf("second event: got %q, want boss2", result.Checkpoints[1].Checkpoint.ID)
	}
}

func TestMemCheck_LevelUp(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "level-20", Name: "Reach Level 20", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x68, Comparison: "gte", Value: 20},
			},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Level 15: not triggered
	result := rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"level-20": 15},
		IGT: 60000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Fatalf("got %d events at level 15, want 0", len(result.Checkpoints))
	}

	// Level 20: triggered
	result = rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"level-20": 20},
		IGT: 120000, DeathCount: 1,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at level 20, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "level-20" {
		t.Errorf("got checkpoint %q, want level-20", result.Checkpoints[0].Checkpoint.ID)
	}
}

func TestMemCheck_WeaponUpgrade(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "weapon-plus5", Name: "Weapon +5", EventType: "weapon_upgrade",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x100, Comparison: "eq", Value: 5, Size: 4},
			},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// +3: not triggered
	result := rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"weapon-plus5": 3},
		IGT: 60000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events at +3, want 0", len(result.Checkpoints))
	}

	// +5: exact match triggered
	result = rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"weapon-plus5": 5},
		IGT: 180000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at +5, want 1", len(result.Checkpoints))
	}
}

func TestMemCheck_GreaterThan(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "souls-1000", Name: "Over 1000 Souls", EventType: "item_pickup",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x50, Comparison: "gt", Value: 1000},
			},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Exactly 1000: gt means strictly greater
	result := rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"souls-1000": 1000},
		IGT: 10000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events at 1000, want 0 (gt, not gte)", len(result.Checkpoints))
	}

	// 1001: triggered
	result = rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"souls-1000": 1001},
		IGT: 20000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at 1001, want 1", len(result.Checkpoints))
	}
}

func TestMixedFlagAndMemCheckpoints(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagID: 1000},
			{
				ID: "level-30", Name: "Reach Level 30", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x68, Comparison: "gte", Value: 30},
			},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagID: 2000},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Boss 1 killed, level still low
	result := rs.ProcessTick(TickInput{
		Flags:     map[uint32]bool{1000: true},
		MemValues: map[string]uint32{"level-30": 25},
		IGT: 95000, DeathCount: 2,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1 (boss1 only)", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss1" {
		t.Errorf("got %q, want boss1", result.Checkpoints[0].Checkpoint.ID)
	}

	// Level up to 30 + boss 2 killed
	result = rs.ProcessTick(TickInput{
		Flags:     map[uint32]bool{1000: true, 2000: true},
		MemValues: map[string]uint32{"level-30": 30},
		IGT: 300000, DeathCount: 5,
	})
	if len(result.Checkpoints) != 2 {
		t.Fatalf("got %d events, want 2 (level-30 + boss2)", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "level-30" {
		t.Errorf("first: got %q, want level-30", result.Checkpoints[0].Checkpoint.ID)
	}
	if result.Checkpoints[1].Checkpoint.ID != "boss2" {
		t.Errorf("second: got %q, want boss2", result.Checkpoints[1].Checkpoint.ID)
	}
	if rs.Status != RunCompleted {
		t.Errorf("expected completed, got %q", rs.Status)
	}
}

func TestProcessTick_BackupOnEncounter(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagID: 1000, BackupFlagID: 1001},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagID: 2000, BackupFlagID: 2001},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Encounter boss 1 (backup flag set, kill flag not)
	result := rs.ProcessTick(TickInput{
		Flags: map[uint32]bool{1001: true, 1000: false},
		IGT:   50000, DeathCount: 0,
	})
	if len(result.Backups) != 1 {
		t.Fatalf("got %d backup events, want 1", len(result.Backups))
	}
	if result.Backups[0].Checkpoint.ID != "boss1" {
		t.Errorf("backup for %q, want boss1", result.Backups[0].Checkpoint.ID)
	}
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d checkpoint events, want 0 (not killed yet)", len(result.Checkpoints))
	}

	// Same tick again: backup should NOT fire again
	result = rs.ProcessTick(TickInput{
		Flags: map[uint32]bool{1001: true, 1000: false},
		IGT:   51000, DeathCount: 0,
	})
	if len(result.Backups) != 0 {
		t.Errorf("got %d backup events on repeat, want 0", len(result.Backups))
	}

	// Kill boss 1: checkpoint completes, no duplicate backup
	result = rs.ProcessTick(TickInput{
		Flags: map[uint32]bool{1001: true, 1000: true},
		IGT:   95000, DeathCount: 2,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d checkpoint events, want 1", len(result.Checkpoints))
	}
	if len(result.Backups) != 0 {
		t.Errorf("got %d backup events on kill, want 0 (already triggered)", len(result.Backups))
	}
}
