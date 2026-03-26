package route

import (
	"errors"
	"testing"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
)

// mockGameReader implements GameReader for testing.
type mockGameReader struct {
	flags         map[uint32]bool
	flagErr       error
	memValues     map[string]uint32
	memErr        error
	invQuantities map[uint32]uint32 // itemID → quantity
	invErr        error
	igt           int64
	igtErr        error
	deathCount    uint32
	deathCountErr error
}

func newMockGameReader() *mockGameReader {
	return &mockGameReader{
		flags:         make(map[uint32]bool),
		memValues:     make(map[string]uint32),
		invQuantities: make(map[uint32]uint32),
	}
}

func (m *mockGameReader) ReadEventFlag(flagID uint32) (bool, error) {
	if m.flagErr != nil {
		return false, m.flagErr
	}
	return m.flags[flagID], nil
}

func (m *mockGameReader) ReadMemoryValue(pathName string, extraOffset int64, size int) (uint32, error) {
	if m.memErr != nil {
		return 0, m.memErr
	}
	val, ok := m.memValues[pathName]
	if !ok {
		return 0, nil
	}
	return val, nil
}

func (m *mockGameReader) ReadInventoryItemQuantity(itemID uint32) (uint32, error) {
	if m.invErr != nil {
		return 0, m.invErr
	}
	return m.invQuantities[itemID], nil
}

func (m *mockGameReader) ReadIGT() (int64, error) {
	if m.igtErr != nil {
		return 0, m.igtErr
	}
	return m.igt, nil
}

func (m *mockGameReader) ReadDeathCount() (uint32, error) {
	if m.deathCountErr != nil {
		return 0, m.deathCountErr
	}
	return m.deathCount, nil
}

func newTestRepo(t *testing.T) *data.Repository {
	t.Helper()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	return repo
}

// testRoute creates a simple route with flag-based checkpoints.
func testRunnerRoute() *Route {
	// Separate from testRoute in state_test.go to avoid redeclaration.
	return &Route{
		ID:       "test-route",
		Name:     "Test Route",
		Game:     "ds3",
		Category: "Any%",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 200}},
			{ID: "boss3", Name: "Boss 3", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 300}},
		},
	}
}

func TestNewRunner(t *testing.T) {
	r := testRunnerRoute()
	repo := newTestRepo(t)
	runner := NewRunner(r, repo, nil)

	if runner.route != r {
		t.Error("expected route to be set")
	}
	if runner.state == nil {
		t.Fatal("expected state to be initialized")
	}
	if runner.runID != 0 {
		t.Error("expected runID to be zero")
	}
	if runner.IsActive() {
		t.Error("expected runner not to be active before Start")
	}
}

func TestRunner_Start(t *testing.T) {
	repo := newTestRepo(t)
	runner := NewRunner(testRunnerRoute(), repo, nil)

	if err := runner.Start(42, 0); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !runner.IsActive() {
		t.Error("expected runner to be active after Start")
	}
	if runner.runID == 0 {
		t.Error("expected runID to be non-zero")
	}
	if runner.state.LastDeathCount != 42 {
		t.Errorf("expected LastDeathCount=42, got %d", runner.state.LastDeathCount)
	}
}

func TestRunner_Abandon(t *testing.T) {
	repo := newTestRepo(t)
	runner := NewRunner(testRunnerRoute(), repo, nil)
	_ = runner.Start(0, 0)

	if err := runner.Abandon(); err != nil {
		t.Fatalf("Abandon failed: %v", err)
	}
	if runner.IsActive() {
		t.Error("expected runner not to be active after Abandon")
	}
}

func TestRunner_Accessors(t *testing.T) {
	repo := newTestRepo(t)
	r := testRunnerRoute()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	if runner.GetRoute() != r {
		t.Error("GetRoute returned wrong route")
	}
	if runner.TotalCount() != 3 {
		t.Errorf("expected TotalCount=3, got %d", runner.TotalCount())
	}
	if runner.CompletedCount() != 0 {
		t.Errorf("expected CompletedCount=0, got %d", runner.CompletedCount())
	}
	if runner.CompletionPercent() != 0 {
		t.Errorf("expected CompletionPercent=0, got %f", runner.CompletionPercent())
	}

	cp := runner.CurrentCheckpoint()
	if cp == nil || cp.ID != "boss1" {
		t.Errorf("expected CurrentCheckpoint=boss1, got %v", cp)
	}
	if runner.SegmentDeaths() != 0 {
		t.Errorf("expected SegmentDeaths=0, got %d", runner.SegmentDeaths())
	}
}

func TestRunner_CatchUp_AllNew(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	_ = runner.Start(0, 0)

	// No flags set
	if err := runner.CatchUp(reader); err != nil {
		t.Errorf("expected CatchUp to succeed when all flags unset, got %v", err)
	}
	if runner.CompletedCount() != 0 {
		t.Error("expected no checkpoints completed")
	}
}

func TestRunner_CatchUp_PreExisting(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}, BackupFlagCheck: &EventFlagCheck{FlagID: 101}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 200}},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.flags[100] = true // boss1 already killed

	if err := runner.CatchUp(reader); err != nil {
		t.Errorf("expected CatchUp to succeed, got %v", err)
	}
	if !runner.state.CompletedFlags["boss1"] {
		t.Error("expected boss1 to be marked completed")
	}
	if !runner.state.BackupDone["boss1"] {
		t.Error("expected boss1 backup to be marked done")
	}
	if runner.state.CompletedFlags["boss2"] {
		t.Error("expected boss2 to NOT be completed")
	}
}

func TestRunner_CatchUp_ReadError(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	_ = runner.Start(0, 0)

	reader.flagErr = errors.New("not ready")

	if err := runner.CatchUp(reader); err == nil {
		t.Error("expected CatchUp to return error on read failure")
	}
}

func TestRunner_CatchUp_NotActive(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	// Not started

	if err := runner.CatchUp(reader); err != nil {
		t.Errorf("expected CatchUp to succeed when not active, got %v", err)
	}
}

func TestRunner_Tick_NotActive(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	// Not started

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Error("expected nil events when not active")
	}
}

func TestRunner_Tick_Checkpoint(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	_ = runner.Start(0, 0)

	reader.flags[100] = true // boss1 killed
	reader.igt = 60000
	reader.deathCount = 3

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Checkpoint.ID != "boss1" {
		t.Errorf("expected boss1 event, got %s", events[0].Checkpoint.ID)
	}
	if events[0].Deaths != 3 {
		t.Errorf("expected 3 deaths, got %d", events[0].Deaths)
	}
	if events[0].IGT != 60000 {
		t.Errorf("expected IGT=60000, got %d", events[0].IGT)
	}
	if runner.CompletedCount() != 1 {
		t.Errorf("expected 1 completed, got %d", runner.CompletedCount())
	}
}

func TestRunner_Tick_NullPointer(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	_ = runner.Start(0, 0)

	reader.flagErr = memreader.ErrNullPointer

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("expected nil error for ErrNullPointer, got %v", err)
	}
	if events != nil {
		t.Error("expected nil events for ErrNullPointer")
	}
}

func TestRunner_Tick_FatalError(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	_ = runner.Start(0, 0)

	reader.flagErr = errors.New("process gone")

	_, err := runner.Tick(reader)
	if err == nil {
		t.Error("expected error for fatal read failure")
	}
}

func TestRunner_Tick_MemCheck(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-memcheck",
		Name: "MemCheck Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "level10", Name: "Level 10", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x10, Comparison: "gte", Value: 10, Size: 4},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.memValues["player_stats"] = 10
	reader.igt = 30000

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Checkpoint.ID != "level10" {
		t.Errorf("expected level10 event, got %s", events[0].Checkpoint.ID)
	}
}

func TestRunner_Tick_RunCompletion(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-complete",
		Name: "Completion Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.flags[100] = true
	reader.igt = 120000

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if runner.IsActive() {
		t.Error("expected runner to be inactive after all checkpoints completed")
	}
}

func TestRunner_Tick_BackupOnKillNoEncounterFlag(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-backup",
		Name: "Backup Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}},
			// No BackupFlagCheck — backup triggers on kill
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil) // nil backup manager
	_ = runner.Start(0, 0)

	reader.flags[100] = true
	reader.igt = 10000

	// Should not panic with nil backup manager
	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestRunner_findGameConfig(t *testing.T) {
	repo := newTestRepo(t)
	runner := NewRunner(testRunnerRoute(), repo, nil) // Game: "ds3"

	cfg := runner.findGameConfig()
	if cfg == nil {
		t.Fatal("expected to find DS3 config")
	}
	if cfg.ID != "ds3" {
		t.Errorf("expected 'ds3', got %q", cfg.ID)
	}

	// Unknown game
	runner2 := NewRunner(&Route{Game: "Unknown Game"}, repo, nil)
	if runner2.findGameConfig() != nil {
		t.Error("expected nil for unknown game")
	}
}

func TestRunner_triggerBackup_NilManager(t *testing.T) {
	repo := newTestRepo(t)
	runner := NewRunner(testRunnerRoute(), repo, nil)

	// Should not panic
	runner.triggerBackup("boss1")
}

func TestRunner_Tick_MemCheckNullPointerSkipsWithoutBlockingFlags(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-mixed",
		Name: "Mixed Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}, BackupFlagCheck: &EventFlagCheck{FlagID: 101}},
			{
				ID: "level10", Name: "Level 10", EventType: "level_up", Optional: true,
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x10, Comparison: "gte", Value: 10, Size: 4},
			},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 200}, BackupFlagCheck: &EventFlagCheck{FlagID: 201}},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.flags[100] = true                 // boss1 killed
	reader.flags[101] = true                 // boss1 encountered
	reader.memErr = memreader.ErrNullPointer // player_stats not readable yet
	reader.igt = 60000
	reader.deathCount = 2

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (boss1), got %d", len(events))
	}
	if events[0].Checkpoint.ID != "boss1" {
		t.Errorf("expected boss1 event, got %s", events[0].Checkpoint.ID)
	}
	// level10 should NOT be completed (mem read failed)
	if runner.state.CompletedFlags["level10"] {
		t.Error("level10 should not be completed when mem read fails")
	}
}

func TestRunner_Tick_IGTNullPointerUsesLastKnown(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-igt",
		Name: "IGT Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)
	runner.state.LastIGT = 50000 // simulate prior tick with valid IGT

	reader.flags[100] = true
	reader.igtErr = memreader.ErrNullPointer

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].IGT != 50000 {
		t.Errorf("expected IGT=50000 (last known), got %d", events[0].IGT)
	}
}

func TestRunner_Tick_IGTError(t *testing.T) {
	repo := newTestRepo(t)
	reader := newMockGameReader()
	runner := NewRunner(testRunnerRoute(), repo, nil)
	_ = runner.Start(0, 0)

	// No flags set, so event flag reads succeed but return false
	reader.igtErr = memreader.ErrNullPointer

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("expected nil error for IGT ErrNullPointer, got %v", err)
	}
	if events != nil {
		t.Error("expected nil events for IGT ErrNullPointer")
	}
}

func TestRunner_Tick_InventoryCheck(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-inv",
		Name: "Inventory Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.invQuantities[0x400003E8] = 3 // only 3 shards
	reader.igt = 30000

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events at qty 3, got %d", len(events))
	}

	// Pick up more shards
	reader.invQuantities[0x400003E8] = 5
	reader.igt = 60000

	events, err = runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event at qty 5, got %d", len(events))
	}
	if events[0].Checkpoint.ID != "shards-5" {
		t.Errorf("expected shards-5 event, got %s", events[0].Checkpoint.ID)
	}
}

func TestRunner_Tick_InventoryCheckNullPointer(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-inv-null",
		Name: "Inventory Null Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}},
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
				Optional:       true,
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.flags[100] = true
	reader.invErr = memreader.ErrNullPointer
	reader.igt = 60000

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (boss1), got %d", len(events))
	}
	if events[0].Checkpoint.ID != "boss1" {
		t.Errorf("expected boss1 event, got %s", events[0].Checkpoint.ID)
	}
	if runner.state.CompletedFlags["shards-5"] {
		t.Error("shards-5 should not be completed when inv read fails")
	}
}

func TestStateVar_Accumulation(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-sv",
		Name: "StateVar Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-4", Name: "4 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 4, StateVar: "embers"},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	// Tick 1: pick up 2 embers (initialize)
	reader.invQuantities[0x400001F4] = 2
	reader.igt = 10000
	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("tick 1: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("tick 1: expected 0 events, got %d", len(events))
	}
	if runner.stateVars["embers"].Accumulated != 2 {
		t.Errorf("tick 1: expected accumulated=2, got %d", runner.stateVars["embers"].Accumulated)
	}

	// Tick 2: spend 1 ember (qty drops to 1, accumulated stays 2)
	reader.invQuantities[0x400001F4] = 1
	reader.igt = 20000
	events, err = runner.Tick(reader)
	if err != nil {
		t.Fatalf("tick 2: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("tick 2: expected 0 events, got %d", len(events))
	}
	if runner.stateVars["embers"].Accumulated != 2 {
		t.Errorf("tick 2: expected accumulated=2, got %d", runner.stateVars["embers"].Accumulated)
	}

	// Tick 3: pick up 3 more (qty goes 1→4, delta=+3, accumulated=2+3=5)
	reader.invQuantities[0x400001F4] = 4
	reader.igt = 30000
	events, err = runner.Tick(reader)
	if err != nil {
		t.Fatalf("tick 3: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("tick 3: expected 1 event (gte 4 met with accumulated=5), got %d", len(events))
	}
	if runner.stateVars["embers"].Accumulated != 5 {
		t.Errorf("tick 3: expected accumulated=5, got %d", runner.stateVars["embers"].Accumulated)
	}
}

func TestStateVar_SharedAcrossCheckpoints(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-sv-shared",
		Name: "Shared StateVar Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "embers"},
			},
			{
				ID: "embers-4", Name: "4 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 4, StateVar: "embers"},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	// Pick up 2 embers
	reader.invQuantities[0x400001F4] = 2
	reader.igt = 10000
	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("tick 1: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("tick 1: expected 1 event (embers-2), got %d", len(events))
	}
	if events[0].Checkpoint.ID != "embers-2" {
		t.Errorf("tick 1: expected embers-2, got %s", events[0].Checkpoint.ID)
	}

	// Spend 1, then pick up 3 more
	reader.invQuantities[0x400001F4] = 1
	reader.igt = 20000
	if _, err := runner.Tick(reader); err != nil { // spend 1
		t.Fatalf("tick spend: %v", err)
	}

	reader.invQuantities[0x400001F4] = 4
	reader.igt = 30000
	events, err = runner.Tick(reader)
	if err != nil {
		t.Fatalf("tick 3: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("tick 3: expected 1 event (embers-4), got %d", len(events))
	}
	if events[0].Checkpoint.ID != "embers-4" {
		t.Errorf("tick 3: expected embers-4, got %s", events[0].Checkpoint.ID)
	}
	if runner.stateVars["embers"].Accumulated != 5 {
		t.Errorf("expected accumulated=5, got %d", runner.stateVars["embers"].Accumulated)
	}
}

func TestStateVar_MixedWithRawInventory(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-sv-mixed",
		Name: "Mixed Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-3", Name: "3 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 3, StateVar: "embers"},
			},
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.invQuantities[0x400001F4] = 2 // embers (state_var)
	reader.invQuantities[0x400003E8] = 5 // shards (raw)
	reader.igt = 10000

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	// shards-5 should trigger (raw qty 5 >= 5), embers-3 should not (accumulated 2 < 3)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Checkpoint.ID != "shards-5" {
		t.Errorf("expected shards-5, got %s", events[0].Checkpoint.ID)
	}
}

func TestStateVar_CatchUp(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-sv-catchup",
		Name: "CatchUp StateVar Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "embers"},
			},
			{
				ID: "embers-10", Name: "10 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 10, StateVar: "embers"},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.invQuantities[0x400001F4] = 5 // have 5 embers at route start

	if err := runner.CatchUp(reader); err != nil {
		t.Fatalf("expected CatchUp to succeed, got %v", err)
	}
	if !runner.state.CompletedFlags["embers-2"] {
		t.Error("expected embers-2 to be completed in catchup")
	}
	if runner.state.CompletedFlags["embers-10"] {
		t.Error("expected embers-10 to NOT be completed in catchup")
	}
	if runner.stateVars["embers"].Accumulated != 5 {
		t.Errorf("expected accumulated=5, got %d", runner.stateVars["embers"].Accumulated)
	}
}

func TestStateVar_Persistence(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-sv-persist",
		Name: "Persist Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-5", Name: "5 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 5, StateVar: "embers"},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.invQuantities[0x400001F4] = 3
	reader.igt = 10000

	if _, err := runner.Tick(reader); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	// Verify state var was persisted
	rows, err := repo.LoadStateVars(runner.runID)
	if err != nil {
		t.Fatalf("LoadStateVars: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 state var, got %d", len(rows))
	}
	if rows[0].VarName != "embers" {
		t.Errorf("expected var_name 'embers', got %q", rows[0].VarName)
	}
	if rows[0].Accumulated != 3 {
		t.Errorf("expected accumulated 3, got %d", rows[0].Accumulated)
	}
	if rows[0].LastQuantity != 3 {
		t.Errorf("expected last_quantity 3, got %d", rows[0].LastQuantity)
	}
}

func TestCatchUp_PersistsToDB(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-catchup-db",
		Name: "CatchUp DB Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}, BackupFlagCheck: &EventFlagCheck{FlagID: 101}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 200}},
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.flags[100] = true             // boss1 already killed
	reader.invQuantities[0x400003E8] = 7 // already have 7 shards

	if err := runner.CatchUp(reader); err != nil {
		t.Fatalf("expected CatchUp to succeed, got %v", err)
	}

	// Verify caught-up checkpoints are persisted to DB with IGT=0
	ids, err := repo.LoadCompletedCheckpoints(runner.runID)
	if err != nil {
		t.Fatalf("LoadCompletedCheckpoints: %v", err)
	}
	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found["boss1"] {
		t.Error("expected boss1 in DB")
	}
	if !found["shards-5"] {
		t.Error("expected shards-5 in DB")
	}
	if found["boss2"] {
		t.Error("boss2 should NOT be in DB")
	}

	// Verify caught-up records have zero IGT (not real splits)
	var igtMs int64
	err = repo.DB().QueryRow(
		"SELECT igt_ms FROM route_checkpoints WHERE run_id = ? AND checkpoint_id = 'boss1'",
		runner.runID,
	).Scan(&igtMs)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if igtMs != 0 {
		t.Errorf("expected IGT=0 for caught-up checkpoint, got %d", igtMs)
	}
}

func TestRestoreFromDB(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-restore",
		Name: "Restore Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}, BackupFlagCheck: &EventFlagCheck{FlagID: 101}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 200}},
			{ID: "boss3", Name: "Boss 3", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 300}, BackupFlagCheck: &EventFlagCheck{FlagID: 301}},
		},
	}

	// Create a run and record some checkpoints
	run, _ := repo.StartRouteRun(r.ID, r.Game, 0)
	if err := repo.RecordCheckpoint(run.ID, "boss1", "Boss 1", 60000, 60000, 2); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordCheckpoint(run.ID, "boss2", "Boss 2", 120000, 60000, 1); err != nil {
		t.Fatal(err)
	}

	// Create a new runner and restore from DB
	runner := NewRunner(r, repo, nil)
	runner.state.Start()
	runner.runID = run.ID

	if err := runner.RestoreFromDB(); err != nil {
		t.Fatalf("RestoreFromDB: %v", err)
	}

	if !runner.state.CompletedFlags["boss1"] {
		t.Error("expected boss1 to be completed after restore")
	}
	if !runner.state.CompletedFlags["boss2"] {
		t.Error("expected boss2 to be completed after restore")
	}
	if runner.state.CompletedFlags["boss3"] {
		t.Error("expected boss3 to NOT be completed after restore")
	}
	if !runner.state.BackupDone["boss1"] {
		t.Error("expected boss1 backup to be marked done after restore")
	}
	if runner.state.BackupDone["boss2"] {
		t.Error("boss2 has no BackupFlagCheck, backup should not be marked done")
	}
	if runner.state.BackupDone["boss3"] {
		t.Error("boss3 is not completed, backup should not be marked done")
	}
}

func TestRunner_CatchUp_SkipsDBRestoredCheckpoints(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-catchup-resume",
		Name: "CatchUp Resume Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 200}},
			{ID: "boss3", Name: "Boss 3", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 300}},
		},
	}

	// Create a run and record boss1 as completed in DB
	run, _ := repo.StartRouteRun(r.ID, r.Game, 0)
	if err := repo.RecordCheckpoint(run.ID, "boss1", "Boss 1", 60000, 60000, 2); err != nil {
		t.Fatal(err)
	}

	// Resume the run (RestoreFromDB marks boss1 as completed)
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	if err := runner.Resume(run.ID, 5); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if !runner.state.CompletedFlags["boss1"] {
		t.Fatal("expected boss1 to be completed after Resume")
	}

	// Set boss1 flag in memory (still set) and boss2 flag (newly completed)
	reader.flags[100] = true // boss1 — already restored from DB
	reader.flags[200] = true // boss2 — newly completed in memory

	if err := runner.CatchUp(reader); err != nil {
		t.Fatalf("CatchUp: %v", err)
	}

	// boss2 should be caught up
	if !runner.state.CompletedFlags["boss2"] {
		t.Error("expected boss2 to be completed after CatchUp")
	}
	// boss3 should NOT be completed
	if runner.state.CompletedFlags["boss3"] {
		t.Error("expected boss3 to NOT be completed")
	}

	// Verify boss1 was NOT re-recorded in DB (should still have exactly 1 record)
	var count int
	err := repo.DB().QueryRow(
		"SELECT COUNT(*) FROM route_checkpoints WHERE run_id = ? AND checkpoint_id = 'boss1'",
		run.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 boss1 record, got %d (CatchUp duplicated a DB-restored checkpoint)", count)
	}

	// Verify boss2 was recorded by CatchUp (with IGT=0 since it's a catch-up)
	var igtMs int64
	err = repo.DB().QueryRow(
		"SELECT igt_ms FROM route_checkpoints WHERE run_id = ? AND checkpoint_id = 'boss2'",
		run.ID,
	).Scan(&igtMs)
	if err != nil {
		t.Fatalf("query boss2: %v", err)
	}
	if igtMs != 0 {
		t.Errorf("expected IGT=0 for caught-up boss2, got %d", igtMs)
	}
}

func TestRunner_CatchUp_InventoryCheck(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-inv-catchup",
		Name: "Inventory CatchUp Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}},
		},
	}
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	_ = runner.Start(0, 0)

	reader.invQuantities[0x400003E8] = 7 // already have 7 shards

	if err := runner.CatchUp(reader); err != nil {
		t.Errorf("expected CatchUp to succeed, got %v", err)
	}
	if !runner.state.CompletedFlags["shards-5"] {
		t.Error("expected shards-5 to be marked completed in CatchUp")
	}
	if runner.state.CompletedFlags["boss1"] {
		t.Error("boss1 should not be completed")
	}
}

func TestRunner_Resume(t *testing.T) {
	repo := newTestRepo(t)
	r := &Route{
		ID:   "test-resume",
		Name: "Resume Route",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 100}, BackupFlagCheck: &EventFlagCheck{FlagID: 101}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 200}},
			{ID: "boss3", Name: "Boss 3", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 300}},
		},
	}

	// Create a run and record some checkpoints
	run, err := repo.StartRouteRun(r.ID, r.Game, 0)
	if err != nil {
		t.Fatalf("StartRouteRun: %v", err)
	}
	if err := repo.RecordCheckpoint(run.ID, "boss1", "Boss 1", 60000, 60000, 2); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}

	// Resume the run with a new runner
	reader := newMockGameReader()
	runner := NewRunner(r, repo, nil)
	if err := runner.Resume(run.ID, 5); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if !runner.IsActive() {
		t.Error("expected runner to be active after Resume")
	}
	if runner.runID != run.ID {
		t.Errorf("expected runID %d, got %d", run.ID, runner.runID)
	}
	if !runner.state.CompletedFlags["boss1"] {
		t.Error("expected boss1 to be completed after Resume")
	}
	if !runner.state.BackupDone["boss1"] {
		t.Error("expected boss1 backup to be done after Resume")
	}
	if runner.state.CompletedFlags["boss2"] {
		t.Error("boss2 should not be completed after Resume")
	}
	if runner.state.LastDeathCount != 5 {
		t.Errorf("expected LastDeathCount=5, got %d", runner.state.LastDeathCount)
	}

	// Verify we can continue ticking from the resumed state
	reader.flags[200] = true
	reader.igt = 120000
	reader.deathCount = 8

	events, err := runner.Tick(reader)
	if err != nil {
		t.Fatalf("Tick after Resume: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (boss2), got %d", len(events))
	}
	if events[0].Checkpoint.ID != "boss2" {
		t.Errorf("expected boss2, got %s", events[0].Checkpoint.ID)
	}
}
