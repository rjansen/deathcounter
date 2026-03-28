package route

import (
	"testing"
)

func testRoute() *Route {
	return &Route{
		ID:   "test",
		Name: "Test Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 2000}},
			{ID: "boss3", Name: "Boss 3", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 3000}},
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

	// Active window processes one required checkpoint per tick
	_ = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true, 3000: true}, IGT: 200000, DeathCount: 0})
	_ = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true, 3000: true}, IGT: 300000, DeathCount: 0})
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
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
			{ID: "optional1", Name: "Optional", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 5000}, Optional: true},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 2000}},
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

	// Both flags set, but active window only includes boss1 (first required)
	result := rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true}, IGT: 200000, DeathCount: 5})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss1" {
		t.Errorf("got %q, want boss1", result.Checkpoints[0].Checkpoint.ID)
	}

	// Second tick: boss2 now active and completes
	result = rs.ProcessTick(TickInput{Flags: map[uint32]bool{1000: true, 2000: true}, IGT: 200000, DeathCount: 5})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss2" {
		t.Errorf("got %q, want boss2", result.Checkpoints[0].Checkpoint.ID)
	}
}

func TestMemCheck_LevelUp(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
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
		IGT:       60000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Fatalf("got %d events at level 15, want 0", len(result.Checkpoints))
	}

	// Level 20: triggered
	result = rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"level-20": 20},
		IGT:       120000, DeathCount: 1,
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
		Game: "ds3",
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
		IGT:       60000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events at +3, want 0", len(result.Checkpoints))
	}

	// +5: exact match triggered
	result = rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"weapon-plus5": 5},
		IGT:       180000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at +5, want 1", len(result.Checkpoints))
	}
}

func TestMemCheck_GreaterThan(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
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
		IGT:       10000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events at 1000, want 0 (gt, not gte)", len(result.Checkpoints))
	}

	// 1001: triggered
	result = rs.ProcessTick(TickInput{
		MemValues: map[string]uint32{"souls-1000": 1001},
		IGT:       20000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at 1001, want 1", len(result.Checkpoints))
	}
}

func TestMixedFlagAndMemCheckpoints(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
			{
				ID: "level-30", Name: "Reach Level 30", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x68, Comparison: "gte", Value: 30},
			},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 2000}},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Boss 1 killed, level still low
	result := rs.ProcessTick(TickInput{
		Flags:     map[uint32]bool{1000: true},
		MemValues: map[string]uint32{"level-30": 25},
		IGT:       95000, DeathCount: 2,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1 (boss1 only)", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss1" {
		t.Errorf("got %q, want boss1", result.Checkpoints[0].Checkpoint.ID)
	}

	// Level up to 30 (active window: level-30 only, it's the next required)
	result = rs.ProcessTick(TickInput{
		Flags:     map[uint32]bool{1000: true, 2000: true},
		MemValues: map[string]uint32{"level-30": 30},
		IGT:       300000, DeathCount: 5,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1 (level-30)", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "level-30" {
		t.Errorf("got %q, want level-30", result.Checkpoints[0].Checkpoint.ID)
	}

	// Boss 2 now active and completes
	result = rs.ProcessTick(TickInput{
		Flags: map[uint32]bool{1000: true, 2000: true},
		IGT:   300000, DeathCount: 5,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1 (boss2)", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "boss2" {
		t.Errorf("got %q, want boss2", result.Checkpoints[0].Checkpoint.ID)
	}
	if rs.Status != RunCompleted {
		t.Errorf("expected completed, got %q", rs.Status)
	}
}

func TestInventoryCheck_Gte(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// 3 shards: not triggered
	result := rs.ProcessTick(TickInput{
		InventoryValues: map[string]uint32{"shards-5": 3},
		IGT:             30000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Fatalf("got %d events at qty 3, want 0", len(result.Checkpoints))
	}

	// 5 shards: triggered
	result = rs.ProcessTick(TickInput{
		InventoryValues: map[string]uint32{"shards-5": 5},
		IGT:             60000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at qty 5, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "shards-5" {
		t.Errorf("got checkpoint %q, want shards-5", result.Checkpoints[0].Checkpoint.ID)
	}
}

func TestInventoryCheck_Gt(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "shards-gt5", Name: "Over 5 Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gt", Value: 5},
			},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Exactly 5: not triggered (gt, not gte)
	result := rs.ProcessTick(TickInput{
		InventoryValues: map[string]uint32{"shards-gt5": 5},
		IGT:             10000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events at qty 5, want 0 (gt)", len(result.Checkpoints))
	}

	// 6: triggered
	result = rs.ProcessTick(TickInput{
		InventoryValues: map[string]uint32{"shards-gt5": 6},
		IGT:             20000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at qty 6, want 1", len(result.Checkpoints))
	}
}

func TestInventoryCheck_Eq(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "shards-eq3", Name: "Exactly 3 Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "eq", Value: 3},
			},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	result := rs.ProcessTick(TickInput{
		InventoryValues: map[string]uint32{"shards-eq3": 2},
		IGT:             10000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 0 {
		t.Errorf("got %d events at qty 2, want 0", len(result.Checkpoints))
	}

	result = rs.ProcessTick(TickInput{
		InventoryValues: map[string]uint32{"shards-eq3": 3},
		IGT:             20000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events at qty 3, want 1", len(result.Checkpoints))
	}
}

func TestMixedFlagAndInventoryCheckpoints(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 2000}},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Boss 1 killed, shards not enough
	result := rs.ProcessTick(TickInput{
		Flags:           map[uint32]bool{1000: true},
		InventoryValues: map[string]uint32{"shards-5": 2},
		IGT:             95000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1 (boss1 only)", len(result.Checkpoints))
	}

	// Shards (active window: shards-5, the next required)
	result = rs.ProcessTick(TickInput{
		Flags:           map[uint32]bool{1000: true, 2000: true},
		InventoryValues: map[string]uint32{"shards-5": 5},
		IGT:             300000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1 (shards-5)", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Checkpoint.ID != "shards-5" {
		t.Errorf("got %q, want shards-5", result.Checkpoints[0].Checkpoint.ID)
	}

	// Boss 2 now active and completes
	result = rs.ProcessTick(TickInput{
		Flags: map[uint32]bool{1000: true, 2000: true},
		IGT:   300000, DeathCount: 0,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d events, want 1 (boss2)", len(result.Checkpoints))
	}
	if rs.Status != RunCompleted {
		t.Errorf("expected completed, got %q", rs.Status)
	}
}

func TestProcessTick_BackupOnEncounter(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}, BackupFlagCheck: &EventFlagCheck{FlagID: 1001}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 2000}, BackupFlagCheck: &EventFlagCheck{FlagID: 2001}},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	// Encounter boss 1 (backup flag set, kill flag not)
	result := rs.ProcessTick(TickInput{
		Flags:       map[uint32]bool{1000: false},
		BackupFlags: map[uint32]bool{1001: true},
		IGT:         50000, DeathCount: 0,
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
		Flags:       map[uint32]bool{1000: false},
		BackupFlags: map[uint32]bool{1001: true},
		IGT:         51000, DeathCount: 0,
	})
	if len(result.Backups) != 0 {
		t.Errorf("got %d backup events on repeat, want 0", len(result.Backups))
	}

	// Kill boss 1: checkpoint completes, no duplicate backup
	result = rs.ProcessTick(TickInput{
		Flags:       map[uint32]bool{1000: true},
		BackupFlags: map[uint32]bool{1001: true},
		IGT:         95000, DeathCount: 2,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d checkpoint events, want 1", len(result.Checkpoints))
	}
	if len(result.Backups) != 0 {
		t.Errorf("got %d backup events on kill, want 0 (already triggered)", len(result.Backups))
	}
}

func TestActiveCheckpoints(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	active := rs.ActiveCheckpoints()
	if len(active) != 1 {
		t.Fatalf("got %d active, want 1", len(active))
	}
	if active[0].ID != "boss1" {
		t.Errorf("got %q, want boss1", active[0].ID)
	}
}

func TestActiveCheckpoints_WithOptionals(t *testing.T) {
	route := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "opt1", Name: "Optional 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 5000}, Optional: true},
			{ID: "opt2", Name: "Optional 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 6000}, Optional: true},
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
			{ID: "opt3", Name: "Optional 3", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 7000}, Optional: true},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 2000}},
		},
	}

	rs := NewRunState(route)
	rs.Start()

	active := rs.ActiveCheckpoints()
	// Should include opt1, opt2, boss1 but NOT opt3 or boss2
	if len(active) != 3 {
		t.Fatalf("got %d active, want 3", len(active))
	}
	if active[0].ID != "opt1" {
		t.Errorf("active[0]: got %q, want opt1", active[0].ID)
	}
	if active[1].ID != "opt2" {
		t.Errorf("active[1]: got %q, want opt2", active[1].ID)
	}
	if active[2].ID != "boss1" {
		t.Errorf("active[2]: got %q, want boss1", active[2].ID)
	}
}

func TestActiveCheckpoints_AllCompleted(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()
	rs.CompletedFlags["boss1"] = true
	rs.CompletedFlags["boss2"] = true
	rs.CompletedFlags["boss3"] = true

	active := rs.ActiveCheckpoints()
	if len(active) != 0 {
		t.Errorf("got %d active, want 0", len(active))
	}
}

func TestActiveCheckpoints_AdvancesAfterCompletion(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()

	// Complete boss1
	rs.CompletedFlags["boss1"] = true

	active := rs.ActiveCheckpoints()
	if len(active) != 1 {
		t.Fatalf("got %d active, want 1", len(active))
	}
	if active[0].ID != "boss2" {
		t.Errorf("got %q, want boss2", active[0].ID)
	}
}

func TestProcessTick_DeathsPerCheckpoint(t *testing.T) {
	rs := NewRunState(testRoute())
	rs.Start()
	rs.LastCheckpointDeaths = 5 // simulate initial death count at run start

	// Boss1 dies 3 times (death count goes from 5 → 8)
	result := rs.ProcessTick(TickInput{
		Flags:      map[uint32]bool{1000: true},
		DeathCount: 8,
		IGT:        60000,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d checkpoints, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Deaths != 3 {
		t.Errorf("boss1 deaths: got %d, want 3", result.Checkpoints[0].Deaths)
	}

	// Boss2 no deaths (death count stays at 8)
	result = rs.ProcessTick(TickInput{
		Flags:      map[uint32]bool{2000: true},
		DeathCount: 8,
		IGT:        120000,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d checkpoints, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Deaths != 0 {
		t.Errorf("boss2 deaths: got %d, want 0", result.Checkpoints[0].Deaths)
	}

	// Boss3 dies 7 times (death count goes from 8 → 15)
	result = rs.ProcessTick(TickInput{
		Flags:      map[uint32]bool{3000: true},
		DeathCount: 15,
		IGT:        200000,
	})
	if len(result.Checkpoints) != 1 {
		t.Fatalf("got %d checkpoints, want 1", len(result.Checkpoints))
	}
	if result.Checkpoints[0].Deaths != 7 {
		t.Errorf("boss3 deaths: got %d, want 7", result.Checkpoints[0].Deaths)
	}
}
