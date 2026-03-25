package monitor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
	"github.com/rjansen/deathcounter/internal/stats"
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

func newTestTracker(t *testing.T) *stats.Tracker {
	t.Helper()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}
	t.Cleanup(func() { tracker.Close() })
	return tracker
}

// tick simulates one Start() loop iteration for a GameMonitor:
// Attach, OnAttach (PhaseAttached→PhaseLoaded), and Tick (PhaseLoaded).
func tick(t *testing.T, mon *GameMonitor) error {
	t.Helper()
	reader, err := mon.Attach()
	if errors.Is(err, ErrGameDetached) {
		mon.tracker.OnDetach()
		mon.publishDetached()
		return err
	}
	if err != nil {
		return err
	}
	// Always inject AOB addresses (idempotent)
	reader.SetTestAOBAddresses(int64(testGameDataManGlobalAddr), int64(testGameManGlobalAddr))

	// PhaseAttached → PhaseLoaded (via OnAttach)
	if mon.phase == PhaseAttached {
		if err := mon.tracker.OnAttach(mon.attachedGameID); err != nil {
			return err
		}
		mon.phase = PhaseLoaded
		return nil // no Tick this cycle, matches Start() behavior
	}

	// PhaseLoaded: Tick
	update, err := mon.tracker.Tick(reader)
	if err != nil {
		if errors.Is(err, memreader.ErrGameRead) {
			mon.Detach()
			mon.tracker.OnDetach()
			mon.publishDetached()
		}
		return err
	}
	mon.publish(update)
	return nil
}

// attachMonitor attaches the monitor and transitions to PhaseLoaded,
// bypassing OnAttach. Used by tests that inject tracker state directly.
func attachMonitor(t *testing.T, mon *GameMonitor) {
	t.Helper()
	reader, err := mon.Attach()
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	reader.SetTestAOBAddresses(int64(testGameDataManGlobalAddr), int64(testGameManGlobalAddr))
	mon.phase = PhaseLoaded
}

// newDeathMonitor creates a GameMonitor with a DeathTracker for testing.
func newDeathMonitor(gameID string, ops memreader.ProcessOps, tracker *stats.Tracker) *GameMonitor {
	return NewGameMonitor(gameID, ops, NewDeathTracker(gameID, tracker))
}

// newRouteMonitor creates a GameMonitor with a RouteTracker for testing.
func newRouteMonitor(gameID, routeID, routesDir string, ops memreader.ProcessOps, tracker *stats.Tracker) *GameMonitor {
	return NewGameMonitor(gameID, ops, NewRouteTracker(gameID, routeID, routesDir, tracker))
}

// routeTracker extracts the RouteTracker from a GameMonitor for test assertions.
func routeTracker(mon *GameMonitor) *RouteTracker {
	return mon.tracker.(*RouteTracker)
}

// deathTracker extracts the DeathTracker from a GameMonitor for test assertions.
func deathTracker(mon *GameMonitor) *DeathTracker {
	return mon.tracker.(*DeathTracker)
}

func TestDeathTracker_NotAttached(t *testing.T) {
	mock := newMockProcessOps()
	tracker := newTestTracker(t)

	mon := newDeathMonitor("ds3", mock, tracker)
	err := tick(t, mon)

	// ErrNoGame is returned when no game process is running
	if !errors.Is(err, ErrNoGame) {
		t.Errorf("expected ErrNoGame, got %v", err)
	}

	// No display update should be available
	select {
	case update := <-mon.DisplayUpdates():
		t.Errorf("expected no display update, got %+v", update)
	default:
		// expected
	}
}

func TestDeathTracker_AttachAndRead(t *testing.T) {
	mock := setupDS3Mock(42)
	tracker := newTestTracker(t)

	mon := newDeathMonitor("ds3", mock, tracker)

	// First tick: attach → PhaseAttached → OnAttach → PhaseLoaded (no Tick)
	tick(t, mon)

	// Second tick: Tick reads death count
	tick(t, mon)

	select {
	case update := <-mon.DisplayUpdates():
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
	tracker := newTestTracker(t)

	mon := newDeathMonitor("ds3", mock, tracker)

	// First tick: attach + load
	tick(t, mon)

	// Second tick: read initial count
	tick(t, mon)
	<-mon.DisplayUpdates()

	// Update death count in memory
	valueAddr := uintptr(0x200000000 + 0x98)
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(valueBytes, 15)
	mock.memory[valueAddr] = valueBytes

	// Third tick: should detect change
	tick(t, mon)

	select {
	case update := <-mon.DisplayUpdates():
		if update.DeathCount != 15 {
			t.Errorf("expected death count 15, got %d", update.DeathCount)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestDeathTracker_Detach(t *testing.T) {
	mock := setupDS3Mock(5)
	tracker := newTestTracker(t)

	mon := newDeathMonitor("ds3", mock, tracker)

	// First tick: attach + load
	tick(t, mon)

	// Second tick: read death count
	tick(t, mon)
	<-mon.DisplayUpdates()

	// Remove the process to simulate game exit
	delete(mock.processes, "DarkSoulsIII.exe")

	// Clear memory to cause read error (simulating process gone)
	for k := range mock.memory {
		delete(mock.memory, k)
	}

	// Next tick: Attach returns existing reader (still non-nil), Tick fails on read → detach
	tick(t, mon)

	select {
	case update := <-mon.DisplayUpdates():
		// After detach, status reverts to "Waiting for game..."
		if update.Status != "Waiting for game..." {
			t.Errorf("expected 'Waiting for game...' after detach, got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteTracker_NoRoute(t *testing.T) {
	mock := setupDS3Mock(0)
	tracker := newTestTracker(t)

	// Use a non-existent route ID so OnAttach returns error
	mon := newRouteMonitor("ds3", "nonexistent-route", "testdata", mock, tracker)

	// First tick: attach → OnAttach fails (route not found) → error returned
	err := tick(t, mon)
	if err == nil {
		t.Fatal("expected error from OnAttach when route doesn't exist")
	}
	rt := routeTracker(mon)
	if rt.route != nil {
		t.Errorf("expected nil route, got %+v", rt.route)
	}
}

func TestRouteTracker_MatchingRoute(t *testing.T) {
	mock := setupDS3Mock(0)
	tracker := newTestTracker(t)

	mon := newRouteMonitor("ds3", "", "", mock, tracker)
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

	// First tick: Tick → detectSave → startRouteRun → CatchUp fails → runner nil'd
	tick(t, mon)

	select {
	case update := <-mon.DisplayUpdates():
		if update.Status != "Loaded" {
			t.Errorf("expected 'Loaded' status (CatchUp pending), got %q", update.Status)
		}
		// Runner is nil after CatchUp failure, so no route info in update
		if update.Route != nil {
			t.Errorf("expected nil Route after CatchUp failure, got %+v", update.Route)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteTracker_OnAttach_GameMismatch(t *testing.T) {
	mock := setupDS3Mock(0)
	tracker := newTestTracker(t)

	rt := NewRouteTracker("ds3", "", "", tracker)

	err := rt.OnAttach("sekiro")
	if !errors.Is(err, ErrAttachedGameMismatch) {
		t.Errorf("expected ErrAttachedGameMismatch, got %v", err)
	}
	if rt.route != nil {
		t.Errorf("expected nil route after mismatch, got %+v", rt.route)
	}
	_ = mock // mock unused in this test but kept for consistency
}

func TestRouteTracker_countBackups(t *testing.T) {
	tracker := newTestTracker(t)
	rt := NewRouteTracker("ds3", "", "", tracker)

	events := []route.CheckpointEvent{
		{Checkpoint: route.Checkpoint{ID: "boss1", BackupFlagCheck: &route.EventFlagCheck{FlagID: 101}}}, // has encounter flag → NOT counted
		{Checkpoint: route.Checkpoint{ID: "boss2"}},                                                      // no encounter flag → counted (kill-based)
		{Checkpoint: route.Checkpoint{ID: "boss3"}},                                                      // no encounter flag → counted
	}

	count := rt.countBackups(events)
	if count != 2 {
		t.Errorf("expected 2 kill-based backups, got %d", count)
	}
}

func TestGameMonitor_Publish_NonBlocking(t *testing.T) {
	mock := newMockProcessOps()
	tracker := newTestTracker(t)

	mon := newDeathMonitor("ds3", mock, tracker)

	// Publish multiple states without consuming — should not block
	for i := 0; i < 5; i++ {
		mon.publish(DisplayUpdate{
			DeathCount: uint32(i),
		})
	}

	// Should get the latest state
	select {
	case update := <-mon.DisplayUpdates():
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
	tracker := newTestTracker(t)

	mon := newDeathMonitor("ds3", mock, tracker)

	// First tick: attach + load (no Tick)
	tick(t, mon)

	// Second tick: Tick → detectSave detects "Knight" slot 0
	tick(t, mon)

	select {
	case update := <-mon.DisplayUpdates():
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
	tracker := newTestTracker(t)

	mon := newRouteMonitor("ds3", "", "", mock, tracker)
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
	tick(t, mon)

	select {
	case update := <-mon.DisplayUpdates():
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

	tracker := newTestTracker(t)

	mon := newRouteMonitor("ds3", "", "", mock, tracker)
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
	reader, err := mon.Attach()
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	reader.SetTestAOBAddresses(int64(gameDataManGlobalAddr), 0)

	// Manually transition to PhaseLoaded (OnAttach would do this in Start)
	mon.phase = PhaseLoaded

	// Tick: save detection fails (null GameDataMan) → route CatchUp also fails
	update, tickErr := mon.tracker.Tick(reader)
	if tickErr == nil {
		mon.publish(update)
	}

	select {
	case got := <-mon.DisplayUpdates():
		if got.Status != "Loaded" {
			t.Errorf("expected 'Loaded' status while save pending, got %q", got.Status)
		}
		// No route should be active (CatchUp fails with null GameDataMan)
		routeName := ""
		if got.Route != nil {
			routeName = got.Route.RouteName
		}
		if routeName != "" {
			t.Errorf("expected no route name before save detection, got %q", routeName)
		}
	default:
		t.Fatal("expected a display update")
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

	tracker := newTestTracker(t)

	mon := newRouteMonitor("ds3", "", "", mock, tracker)
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
	reader, err := mon.Attach()
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	reader.SetTestAOBAddresses(int64(gameDataManGlobalAddr), int64(gameManGlobalAddr))

	// Manually transition to PhaseLoaded (OnAttach would do this in Start)
	mon.phase = PhaseLoaded

	update, tickErr := mon.tracker.Tick(reader)
	if tickErr == nil {
		mon.publish(update)
	}

	// Slot 255 rejected → save pending, route CatchUp fails → no route active
	select {
	case got := <-mon.DisplayUpdates():
		if got.Status != "Loaded" {
			t.Errorf("expected 'Loaded' with slot 255, got %q", got.Status)
		}
		routeName := ""
		if got.Route != nil {
			routeName = got.Route.RouteName
		}
		if routeName != "" {
			t.Errorf("expected no route with slot 255, got %q", routeName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteTracker_SaveChange_AbandonsRun(t *testing.T) {
	mock := setupDS3Mock(0)
	tracker := newTestTracker(t)

	mon := newRouteMonitor("ds3", "", "", mock, tracker)
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
	tick(t, mon)
	<-mon.DisplayUpdates()

	// Change character name to simulate save switch
	setCharacterName(mock, "Pyromancer")

	// Third tick: save changed → abandon + restart
	tick(t, mon)

	select {
	case update := <-mon.DisplayUpdates():
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

	tracker := newTestTracker(t)

	mon := newDeathMonitor("er", mock, tracker)

	// Attach → PhaseAttached, manually transition to PhaseLoaded (OnAttach no-op)
	reader, err := mon.Attach()
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	mon.phase = PhaseLoaded

	// First Tick: detectSave (unsupported) + ReadDeathCount
	update, tickErr := mon.tracker.Tick(reader)
	if tickErr != nil {
		t.Fatalf("Tick failed: %v", tickErr)
	}
	mon.publish(update)

	select {
	case got := <-mon.DisplayUpdates():
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

// setCharacterName updates the character name in mock memory (PlayerGameData + 0x88).
func setCharacterName(mock *mockProcessOps, name string) {
	mock.memory[uintptr(0x400000088)] = encodeUTF16LE(name)
}
