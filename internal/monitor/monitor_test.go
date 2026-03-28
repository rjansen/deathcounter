package monitor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
)

// mockProcessOps implements memreader.ProcessOps for testing.
type mockProcessOps struct {
	processes     map[string]uint32
	modules       map[string]uintptr
	architectures map[uint32]bool
	memory        map[uintptr][]byte
	nextHandle    memreader.ProcessHandle
	handleToPid   map[memreader.ProcessHandle]uint32
}

func newMockProcessOps() *mockProcessOps {
	return &mockProcessOps{
		processes:     make(map[string]uint32),
		modules:       make(map[string]uintptr),
		architectures: make(map[uint32]bool),
		memory:        make(map[uintptr][]byte),
		handleToPid:   make(map[memreader.ProcessHandle]uint32),
	}
}

func (m *mockProcessOps) FindProcessByName(name string) (uint32, error) {
	pid, ok := m.processes[name]
	if !ok {
		return 0, fmt.Errorf("not found: %s", name)
	}
	return pid, nil
}

func (m *mockProcessOps) OpenProcess(access uint32, inheritHandle bool, pid uint32) (memreader.ProcessHandle, error) {
	m.nextHandle++
	m.handleToPid[m.nextHandle] = pid
	return m.nextHandle, nil
}

func (m *mockProcessOps) GetModuleBaseAddress(pid uint32, moduleName string) (uintptr, error) {
	key := fmt.Sprintf("%d:%s", pid, moduleName)
	addr, ok := m.modules[key]
	if !ok {
		return 0, fmt.Errorf("module not found")
	}
	return addr, nil
}

func (m *mockProcessOps) IsProcess64Bit(handle memreader.ProcessHandle) (bool, error) {
	pid := m.handleToPid[handle]
	is64, ok := m.architectures[pid]
	if !ok {
		return false, fmt.Errorf("unknown pid")
	}
	return is64, nil
}

func (m *mockProcessOps) ReadProcessMemory(handle memreader.ProcessHandle, address uintptr, buffer []byte) error {
	data, ok := m.memory[address]
	if !ok {
		return fmt.Errorf("no memory at 0x%X", address)
	}
	copy(buffer, data)
	return nil
}

func (m *mockProcessOps) CloseHandle(handle memreader.ProcessHandle) error {
	return nil
}

// AOB addresses used by setupDS3Mock.
const (
	testGameDataManGlobalAddr = uintptr(0x500000000)
	testGameManGlobalAddr     = uintptr(0x600000000)
)

// setupDS3Mock sets up a mock with Dark Souls III process and a death count value.
// It also sets up the save slot and character name memory chains so that
// detectSave works correctly.
func setupDS3Mock(deathCount uint32) *mockProcessOps {
	mock := newMockProcessOps()

	// DS3 process
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	// DS3 death count pointer chain: base + 0x47572B8 -> ptr + 0x98 -> value
	ptrAddr := uintptr(0x140000000 + 0x47572B8)
	targetPtr := uint64(0x200000000)
	ptrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptrBytes, targetPtr)
	mock.memory[ptrAddr] = ptrBytes

	// Death count value at ptr + 0x98
	valueAddr := uintptr(targetPtr + 0x98)
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(valueBytes, deathCount)
	mock.memory[valueAddr] = valueBytes

	// GameDataMan global pointer at a simulated AOB-resolved address
	gameDataManPtr := uint64(0x300000000) // the GameDataMan object itself
	gameDataManBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(gameDataManBytes, gameDataManPtr)
	mock.memory[testGameDataManGlobalAddr] = gameDataManBytes

	// GameMan global pointer at a simulated AOB-resolved address
	gameManPtr := uint64(0x700000000)
	gameManBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(gameManBytes, gameManPtr)
	mock.memory[testGameManGlobalAddr] = gameManBytes

	// Save slot index at GameMan + 0xA60
	slotBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(slotBytes, 0) // slot 0
	mock.memory[uintptr(gameManPtr+0xA60)] = slotBytes

	// PlayerGameData: GameDataMan + 0x10 -> PlayerGameData object
	playerGameDataPtr := uint64(0x400000000)
	pgdBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(pgdBytes, playerGameDataPtr)
	mock.memory[uintptr(gameDataManPtr+0x10)] = pgdBytes

	// Character name at PlayerGameData + 0x88 (UTF-16LE "Knight")
	charName := encodeUTF16LE("Knight")
	mock.memory[uintptr(playerGameDataPtr+0x88)] = charName

	// SprjEventFlagMan static chain: [0x4768E78, 0x0]
	// followPointerChain dereferences all offsets except the last:
	//   base + 0x4768E78 → read ptr → sprjBase; sprjBase + 0x0 → return sprjBase
	sprjBase := uint64(0x800000000)
	sprjBaseBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(sprjBaseBytes, sprjBase)
	mock.memory[uintptr(0x140000000+0x4768E78)] = sprjBaseBytes

	// Flag array: sprjBase + 0x218 → flagArrayBase
	flagArrayBase := uint64(0x810000000)
	flagArrayBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(flagArrayBytes, flagArrayBase)
	mock.memory[uintptr(sprjBase+0x218)] = flagArrayBytes

	// Flag region: flagArrayBase[0] → flagRegion (div10000000=0 for FlagID=100)
	flagRegion := uint64(0x820000000)
	flagRegionBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(flagRegionBytes, flagRegion)
	mock.memory[uintptr(flagArrayBase)] = flagRegionBytes

	// Flag data pointer: flagRegion + 0 (div1000=0, category=0) → flagDataPtr
	flagDataPtr := uint64(0x830000000)
	flagDataPtrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(flagDataPtrBytes, flagDataPtr)
	mock.memory[uintptr(flagRegion)] = flagDataPtrBytes

	// Flag value: flagDataPtr + 12 (dwordIndex for remainder=100) → 0 (flag not set)
	flagValueBytes := make([]byte, 4)
	mock.memory[uintptr(flagDataPtr+12)] = flagValueBytes

	// IGT value: sprjBase + 0xA4 (static chain [0x4768E78, 0xA4] reads here)
	igtBytes := make([]byte, 8)
	mock.memory[uintptr(sprjBase+0xA4)] = igtBytes

	return mock
}

// encodeUTF16LE encodes a string as UTF-16LE bytes, padded to at least 32 bytes.
func encodeUTF16LE(s string) []byte {
	runes := []rune(s)
	size := (len(runes) + 1) * 2 // +1 for null terminator
	if size < 32 {
		size = 32
	}
	buf := make([]byte, size)
	for i, r := range runes {
		buf[i*2] = byte(r)
		buf[i*2+1] = byte(r >> 8)
	}
	// null terminator already zero from make
	return buf
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

// tick simulates one Start() loop iteration for a GameMonitor by delegating
// to state.Tick(), which handles Attach, OnAttach, tracker.Tick, and publishing.
func tick(t *testing.T, mon *GameMonitor) error {
	t.Helper()
	// Inject AOB addresses after attach so the reader can resolve pointers.
	// We hook into this by checking if the reader was just created.
	hadReader := mon.reader != nil
	err := mon.state.Tick(mon)
	if !hadReader && mon.reader != nil {
		mon.reader.SetTestAOBAddresses(int64(testGameDataManGlobalAddr), int64(testGameManGlobalAddr))
	}
	return err
}

// attachMonitor attaches the monitor and transitions to PhaseLoaded,
// bypassing OnAttach. Used by tests that inject tracker state directly.
func attachMonitor(t *testing.T, mon *GameMonitor) {
	t.Helper()
	// Use detachedState.Attach to find the game process and create the reader.
	reader, err := mon.state.Attach(mon)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	reader.SetTestAOBAddresses(int64(testGameDataManGlobalAddr), int64(testGameManGlobalAddr))
	// Skip OnAttach (attachedState.Attach) and go directly to loaded.
	mon.setState(&loadedState{})
}

// newDeathMonitor creates a GameMonitor with a DeathTracker for testing.
func newDeathMonitor(gameID string, ops memreader.ProcessOps) *GameMonitor {
	return NewGameMonitor(gameID, ops, NewDeathTracker(gameID))
}

// newRouteMonitor creates a GameMonitor with a RouteTracker for testing.
func newRouteMonitor(gameID, routeID, routesDir string, ops memreader.ProcessOps, repo *data.Repository) *GameMonitor {
	return NewGameMonitor(gameID, ops, NewRouteTracker(gameID, routeID, routesDir, repo))
}

// initDisplayCh creates the display channel for tests that call tick() directly
// instead of Start(). Returns the receive-only channel for assertions.
func initDisplayCh(mon *GameMonitor) <-chan DisplayUpdate {
	mon.displayCh = make(chan DisplayUpdate, 1)
	return mon.displayCh
}

// routeTracker extracts the RouteTracker from a GameMonitor for test assertions.
func routeTracker(mon *GameMonitor) *RouteTracker {
	return mon.tracker.(*RouteTracker)
}

func TestDeathTracker_NotAttached(t *testing.T) {
	mock := newMockProcessOps()

	mon := newDeathMonitor("ds3", mock)
	displayCh := initDisplayCh(mon)
	err := tick(t, mon)

	// detachedState.Tick wraps ErrNoGame with state context
	if !errors.Is(err, ErrNoGame) {
		t.Errorf("expected ErrNoGame in chain, got %v", err)
	}
	if !strings.Contains(err.Error(), "detached_state.attach_error:") {
		t.Errorf("expected detached_state.attach_error prefix, got %q", err.Error())
	}

	// No display update should be available
	select {
	case update := <-displayCh:
		t.Errorf("expected no display update, got %+v", update)
	default:
		// expected
	}
}

func TestDeathTracker_AttachAndRead(t *testing.T) {
	mock := setupDS3Mock(42)

	mon := newDeathMonitor("ds3", mock)
	displayCh := initDisplayCh(mon)

	// Tick 1: detached → attach (find game) → attached
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 1: %v", err)
	}

	// Tick 2: attached → OnAttach → loaded
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 2: %v", err)
	}

	// Tick 3: loaded → tracker.Tick reads death count
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 3: %v", err)
	}

	select {
	case update := <-displayCh:
		if update.GameName != "Dark Souls III" {
			t.Errorf("expected 'Dark Souls III', got %q", update.GameName)
		}
		if update.DeathCount != 42 {
			t.Errorf("expected death count 42, got %d", update.DeathCount)
		}
		if update.Status != "Loaded" {
			t.Errorf("expected 'Loaded', got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestDeathTracker_DeathCountChange(t *testing.T) {
	mock := setupDS3Mock(10)

	mon := newDeathMonitor("ds3", mock)
	displayCh := initDisplayCh(mon)

	// Tick 1: detached → attached
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 1: %v", err)
	}

	// Tick 2: attached → loaded
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 2: %v", err)
	}

	// Tick 3: loaded → read initial count
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 3: %v", err)
	}
	<-displayCh

	// Update death count in memory
	valueAddr := uintptr(0x200000000 + 0x98)
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(valueBytes, 15)
	mock.memory[valueAddr] = valueBytes

	// Third tick: should detect change
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 3: %v", err)
	}

	select {
	case update := <-displayCh:
		if update.DeathCount != 15 {
			t.Errorf("expected death count 15, got %d", update.DeathCount)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestDeathTracker_Detach(t *testing.T) {
	mock := setupDS3Mock(5)

	mon := newDeathMonitor("ds3", mock)
	displayCh := initDisplayCh(mon)

	// Tick 1: detached → attached
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 1: %v", err)
	}

	// Tick 2: attached → loaded
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 2: %v", err)
	}

	// Tick 3: loaded → read death count
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 3: %v", err)
	}
	<-displayCh

	// Remove the process to simulate game exit
	delete(mock.processes, "DarkSoulsIII.exe")

	// Clear memory to cause read error (simulating process gone)
	for k := range mock.memory {
		delete(mock.memory, k)
	}

	// Next tick: Attach returns existing reader (still non-nil), Tick fails on read → detach
	err := tick(t, mon)
	if !errors.Is(err, memreader.ErrGameRead) {
		t.Fatalf("expected ErrGameRead in chain, got %v", err)
	}
	if !strings.Contains(err.Error(), "loaded_state.tick_error:") {
		t.Errorf("expected loaded_state.tick_error prefix, got %q", err.Error())
	}

	select {
	case update := <-displayCh:
		// After detach, status reverts to "Waiting for game..."
		if update.Status != "Waiting for game..." {
			t.Errorf("expected 'Waiting for game...' after detach, got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestLoadedState_NilReader_AttachError(t *testing.T) {
	mock := setupDS3Mock(0)

	mon := newDeathMonitor("ds3", mock)
	displayCh := initDisplayCh(mon)

	// Tick 1: detached → attached
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 1: %v", err)
	}

	// Tick 2: attached → loaded
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 2: %v", err)
	}

	// Simulate reader disappearing while in loaded state
	mon.reader = nil

	// Tick 3: loadedState.Tick → Attach fails (nil reader) → ErrGameDetached
	err := tick(t, mon)
	if !errors.Is(err, ErrGameDetached) {
		t.Fatalf("expected ErrGameDetached in chain, got %v", err)
	}
	if !strings.Contains(err.Error(), "loaded_state.attach_error:") {
		t.Errorf("expected loaded_state.attach_error prefix, got %q", err.Error())
	}

	// Should publish detached status
	select {
	case update := <-displayCh:
		if update.Status != "Waiting for game..." {
			t.Errorf("expected 'Waiting for game...' after nil reader, got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}

	// Monitor should be back in detached state
	if mon.state.Phase() != PhaseDetached {
		t.Errorf("expected PhaseDetached, got %s", mon.state.Phase())
	}
}

func TestRouteTracker_NoRoute(t *testing.T) {
	mock := setupDS3Mock(0)
	repo := newTestRepo(t)

	// Use a non-existent route ID so OnAttach returns error
	mon := newRouteMonitor("ds3", "nonexistent-route", "testdata", mock, repo)

	// Tick 1: detached → attached (find game)
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 1: %v", err)
	}

	// Tick 2: attached → OnAttach fails (route not found) → error returned
	err := tick(t, mon)
	if err == nil {
		t.Fatal("expected error from OnAttach when route doesn't exist")
	}
	if !strings.Contains(err.Error(), "attached_state.attach_error:") {
		t.Errorf("expected attached_state.attach_error prefix, got %q", err.Error())
	}
	rt := routeTracker(mon)
	if rt.route != nil {
		t.Errorf("expected nil route, got %+v", rt.route)
	}
}

func TestRouteTracker_MatchingRoute(t *testing.T) {
	mock := setupDS3Mock(0)
	repo := newTestRepo(t)

	mon := newRouteMonitor("ds3", "", "", mock, repo)
	displayCh := initDisplayCh(mon)
	rt := routeTracker(mon)
	// Inject route directly and skip OnAttach (which would fail loading from disk)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagCheck: &route.EventFlagCheck{FlagID: 100}},
		},
	}

	// Attach and transition to PhaseLoaded (bypassing OnAttach since route is injected)
	attachMonitor(t, mon)

	// First tick: Tick → detectSave → startRouteRun → CatchUp succeeds → route running
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick: %v", err)
	}

	select {
	case update := <-displayCh:
		if update.Status != "Tracking route" {
			t.Errorf("expected 'Tracking route' status, got %q", update.Status)
		}
		if update.Route == nil {
			t.Fatal("expected Route to be non-nil after successful CatchUp")
		}
		if update.Route.RouteName != "DS3 Any%" {
			t.Errorf("expected route name 'DS3 Any%%', got %q", update.Route.RouteName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteTracker_OnAttach_GameMismatch(t *testing.T) {
	mock := setupDS3Mock(0)
	repo := newTestRepo(t)

	rt := NewRouteTracker("ds3", "", "", repo)

	err := rt.OnAttach("sekiro")
	if !errors.Is(err, ErrAttachedGameMismatch) {
		t.Errorf("expected ErrAttachedGameMismatch, got %v", err)
	}
	if rt.route != nil {
		t.Errorf("expected nil route after mismatch, got %+v", rt.route)
	}
	_ = mock // mock unused in this test but kept for consistency
}

func TestGameMonitor_Publish_NonBlocking(t *testing.T) {
	mock := newMockProcessOps()

	mon := newDeathMonitor("ds3", mock)
	displayCh := initDisplayCh(mon)

	// Publish multiple states without consuming — should not block
	for i := 0; i < 5; i++ {
		if err := mon.publish(DisplayUpdate{
			DeathCount: uint32(i),
		}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	// Should get the latest state
	select {
	case update := <-displayCh:
		if update.DeathCount != 4 {
			t.Errorf("expected latest death count 4, got %d", update.DeathCount)
		}
	default:
		t.Fatal("expected a display update")
	}
}

// --- Save detection tests ---

func TestDeathTracker_SaveDetection(t *testing.T) {
	mock := setupDS3Mock(5)

	mon := newDeathMonitor("ds3", mock)
	displayCh := initDisplayCh(mon)

	// Tick 1: detached → attached
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 1: %v", err)
	}

	// Tick 2: attached → loaded
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 2: %v", err)
	}

	// Tick 3: loaded → detectSave detects "Knight" slot 0
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick 3: %v", err)
	}

	select {
	case update := <-displayCh:
		if update.CharacterName != "Knight" {
			t.Errorf("expected 'Knight', got %q", update.CharacterName)
		}
		if update.SaveSlotIndex != 0 {
			t.Errorf("expected slot 0, got %d", update.SaveSlotIndex)
		}
		if update.Status != "Loaded" {
			t.Errorf("expected 'Loaded', got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteTracker_DetectsSave(t *testing.T) {
	mock := setupDS3Mock(0)
	repo := newTestRepo(t)

	mon := newRouteMonitor("ds3", "", "", mock, repo)
	displayCh := initDisplayCh(mon)
	rt := routeTracker(mon)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagCheck: &route.EventFlagCheck{FlagID: 100}},
		},
	}

	// Attach and transition to PhaseLoaded (bypassing OnAttach)
	attachMonitor(t, mon)

	// Tick → detectSave
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick: %v", err)
	}

	select {
	case update := <-displayCh:
		if update.CharacterName != "Knight" {
			t.Errorf("expected 'Knight', got %q", update.CharacterName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteTracker_SaveDetectionGatesRoute(t *testing.T) {
	mock := newMockProcessOps()

	// DS3 process
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	// Death count chain set up
	ptrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptrBytes, 0x200000000)
	mock.memory[uintptr(0x140000000+0x47572B8)] = ptrBytes
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(valueBytes, 3)
	mock.memory[uintptr(0x200000098)] = valueBytes

	// GameDataMan AOB resolved but global pointer is null (game loading)
	gameDataManGlobalAddr := uintptr(0x500000000)
	gameDataManBytes := make([]byte, 8) // null pointer
	mock.memory[gameDataManGlobalAddr] = gameDataManBytes

	repo := newTestRepo(t)

	mon := newRouteMonitor("ds3", "", "", mock, repo)
	initDisplayCh(mon)
	rt := routeTracker(mon)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagCheck: &route.EventFlagCheck{FlagID: 100}},
		},
	}

	// Attach and inject AOB with null GameDataMan (simulating game loading)
	reader, err := mon.state.Attach(mon)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	reader.SetTestAOBAddresses(int64(gameDataManGlobalAddr), 0)

	// Manually transition to PhaseLoaded (OnAttach would do this in Start)
	mon.setState(&loadedState{})

	// Tick: save detection fails (null GameDataMan) → startRouteRun CatchUp also fails
	update, tickErr := mon.tracker.Tick(reader)
	if tickErr == nil {
		t.Fatal("expected error from CatchUp with null GameDataMan")
	}

	// Update is still returned with the error — verify no route is active
	if update.Status != "Loaded" {
		t.Errorf("expected 'Loaded' status while save pending, got %q", update.Status)
	}
	if update.Route != nil {
		t.Errorf("expected nil Route after CatchUp failure, got %+v", update.Route)
	}
}

func TestRouteTracker_Slot255Rejected(t *testing.T) {
	mock := newMockProcessOps()

	// DS3 process
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	// Death count chain
	ptrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptrBytes, 0x200000000)
	mock.memory[uintptr(0x140000000+0x47572B8)] = ptrBytes
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(valueBytes, 0)
	mock.memory[uintptr(0x200000098)] = valueBytes

	// GameDataMan + PlayerGameData chain
	gameDataManGlobalAddr := uintptr(0x500000000)
	gameDataManPtr := uint64(0x300000000)
	gdmBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(gdmBytes, gameDataManPtr)
	mock.memory[gameDataManGlobalAddr] = gdmBytes

	playerGameDataPtr := uint64(0x400000000)
	pgdBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(pgdBytes, playerGameDataPtr)
	mock.memory[uintptr(gameDataManPtr+0x10)] = pgdBytes

	// Character name
	mock.memory[uintptr(playerGameDataPtr+0x88)] = encodeUTF16LE("Knight")

	// GameMan with slot 255 (uninitialized)
	gameManGlobalAddr := uintptr(0x600000000)
	gameManPtr := uint64(0x700000000)
	gmBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(gmBytes, gameManPtr)
	mock.memory[gameManGlobalAddr] = gmBytes

	slotBytes := make([]byte, 8)
	slotBytes[0] = 255 // uninitialized slot
	mock.memory[uintptr(gameManPtr+0xA60)] = slotBytes

	repo := newTestRepo(t)

	mon := newRouteMonitor("ds3", "", "", mock, repo)
	initDisplayCh(mon)
	rt := routeTracker(mon)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagCheck: &route.EventFlagCheck{FlagID: 100}},
		},
	}

	// Attach and inject AOB addresses
	reader, err := mon.state.Attach(mon)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	reader.SetTestAOBAddresses(int64(gameDataManGlobalAddr), int64(gameManGlobalAddr))

	// Manually transition to PhaseLoaded (OnAttach would do this in Start)
	mon.setState(&loadedState{})

	// Slot 255 rejected → save pending, startRouteRun CatchUp fails → no route active
	update, tickErr := mon.tracker.Tick(reader)
	if tickErr == nil {
		t.Fatal("expected error from CatchUp with slot 255")
	}

	// Update is still returned with the error — verify no route is active
	if update.Status != "Loaded" {
		t.Errorf("expected 'Loaded' with slot 255, got %q", update.Status)
	}
	if update.Route != nil {
		t.Errorf("expected nil Route after CatchUp failure, got %+v", update.Route)
	}
}

func TestRouteTracker_SaveChange_PausesRun(t *testing.T) {
	mock := setupDS3Mock(0)
	repo := newTestRepo(t)

	mon := newRouteMonitor("ds3", "", "", mock, repo)
	displayCh := initDisplayCh(mon)
	rt := routeTracker(mon)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagCheck: &route.EventFlagCheck{FlagID: 100}},
		},
	}

	// Attach and transition to PhaseLoaded (bypassing OnAttach)
	attachMonitor(t, mon)

	// First tick: Tick → detectSave → start route
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick: %v", err)
	}
	<-displayCh

	// Change character name to simulate save switch
	setCharacterName(mock, "Pyromancer")

	// Third tick: save changed → pause + restart
	if err := tick(t, mon); err != nil {
		t.Fatalf("tick: %v", err)
	}

	select {
	case update := <-displayCh:
		if update.CharacterName != "Pyromancer" {
			t.Errorf("expected 'Pyromancer', got %q", update.CharacterName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestDeathTracker_NonDS3_SkipsSaveDetection(t *testing.T) {
	mock := newMockProcessOps()

	// Elden Ring process (no save slot support)
	mock.processes["eldenring.exe"] = 9999
	mock.modules["9999:eldenring.exe"] = 0x140000000
	mock.architectures[9999] = true

	// Set up death count chain for Elden Ring: {0x3D5DF38, 0x94}
	ptrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptrBytes, 0x200000000)
	mock.memory[uintptr(0x140000000+0x3D5DF38)] = ptrBytes
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(valueBytes, 7)
	mock.memory[uintptr(0x200000094)] = valueBytes

	mon := newDeathMonitor("er", mock)
	displayCh := initDisplayCh(mon)

	// Attach → PhaseAttached, manually transition to PhaseLoaded (OnAttach no-op)
	reader, err := mon.state.Attach(mon)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	mon.setState(&loadedState{})

	// First Tick: detectSave (unsupported) + ReadDeathCount
	update, tickErr := mon.tracker.Tick(reader)
	if tickErr != nil {
		t.Fatalf("Tick failed: %v", tickErr)
	}
	if err := mon.publish(update); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-displayCh:
		if got.GameName != "Elden Ring" {
			t.Errorf("expected 'Elden Ring', got %q", got.GameName)
		}
		if got.Status != "Loaded" {
			t.Errorf("expected 'Loaded' for unsupported save detection, got %q", got.Status)
		}
		if got.DeathCount != 7 {
			t.Errorf("expected death count 7, got %d", got.DeathCount)
		}
		// No character name for unsupported game
		if got.CharacterName != "" {
			t.Errorf("expected empty character name, got %q", got.CharacterName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

// --- buildUpdate event propagation tests ---

func TestRouteTracker_BuildUpdate_WithEvents(t *testing.T) {
	repo := newTestRepo(t)

	rt := NewRouteTracker("ds3", "", "", repo)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill"},
			{ID: "boss2", Name: "Vordt", EventType: "boss_kill"},
		},
	}
	rt.runner = route.NewRunner(rt.route, repo, nil)
	rt.setTrackerState(&routeRunningState{})

	events := []route.CheckpointEvent{
		{
			Checkpoint:         route.Checkpoint{ID: "boss1", Name: "Iudex Gundyr"},
			IGT:                180000,
			CheckpointDuration: 180000,
		},
	}

	update := rt.buildUpdate(events)

	if update.Route == nil {
		t.Fatal("expected Route to be non-nil")
	}
	if len(update.Route.CompletedEvents) != 1 {
		t.Fatalf("expected 1 CompletedEvent, got %d", len(update.Route.CompletedEvents))
	}

	evt := update.Route.CompletedEvents[0]
	if evt.Name != "Iudex Gundyr" {
		t.Errorf("Name = %q, want %q", evt.Name, "Iudex Gundyr")
	}
	if evt.IGT != 180000 {
		t.Errorf("IGT = %d, want %d", evt.IGT, 180000)
	}
	if evt.Duration != 180000 {
		t.Errorf("Duration = %d, want %d", evt.Duration, 180000)
	}
}

func TestRouteTracker_BuildUpdate_NilEvents(t *testing.T) {
	repo := newTestRepo(t)

	rt := NewRouteTracker("ds3", "", "", repo)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill"},
		},
	}
	rt.runner = route.NewRunner(rt.route, repo, nil)
	rt.setTrackerState(&routeRunningState{})

	update := rt.buildUpdate(nil)

	if update.Route == nil {
		t.Fatal("expected Route to be non-nil")
	}
	if len(update.Route.CompletedEvents) != 0 {
		t.Errorf("expected 0 CompletedEvents, got %d", len(update.Route.CompletedEvents))
	}
}

func TestRouteTracker_BuildUpdate_MultipleEvents(t *testing.T) {
	repo := newTestRepo(t)

	rt := NewRouteTracker("ds3", "", "", repo)
	rt.route = &route.Route{
		ID:   "ds3-any",
		Name: "DS3 Any%",
		Game: "ds3",
		Checkpoints: []route.Checkpoint{
			{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill"},
			{ID: "boss2", Name: "Vordt", EventType: "boss_kill"},
		},
	}
	rt.runner = route.NewRunner(rt.route, repo, nil)
	rt.setTrackerState(&routeRunningState{})

	events := []route.CheckpointEvent{
		{
			Checkpoint:         route.Checkpoint{ID: "boss1", Name: "Iudex Gundyr"},
			IGT:                180000,
			CheckpointDuration: 180000,
		},
		{
			Checkpoint:         route.Checkpoint{ID: "boss2", Name: "Vordt"},
			IGT:                360000,
			CheckpointDuration: 180000,
		},
	}

	update := rt.buildUpdate(events)

	if len(update.Route.CompletedEvents) != 2 {
		t.Fatalf("expected 2 CompletedEvents, got %d", len(update.Route.CompletedEvents))
	}
	if update.Route.CompletedEvents[0].Name != "Iudex Gundyr" {
		t.Errorf("event[0].Name = %q, want %q", update.Route.CompletedEvents[0].Name, "Iudex Gundyr")
	}
	if update.Route.CompletedEvents[1].Name != "Vordt" {
		t.Errorf("event[1].Name = %q, want %q", update.Route.CompletedEvents[1].Name, "Vordt")
	}
}

func TestRouteTracker_BuildUpdate_NotRunning_IgnoresEvents(t *testing.T) {
	repo := newTestRepo(t)

	rt := NewRouteTracker("ds3", "", "", repo)
	// State is routeStoppedState (default), runner is nil

	events := []route.CheckpointEvent{
		{
			Checkpoint: route.Checkpoint{ID: "boss1", Name: "Iudex Gundyr"},
			IGT:        180000,
		},
	}

	update := rt.buildUpdate(events)

	// Route should be nil when not running
	if update.Route != nil {
		t.Errorf("expected nil Route when not running, got %+v", update.Route)
	}
}

// setCharacterName updates the character name in mock memory (PlayerGameData + 0x88).
func setCharacterName(mock *mockProcessOps, name string) {
	mock.memory[uintptr(0x400000088)] = encodeUTF16LE(name)
}
