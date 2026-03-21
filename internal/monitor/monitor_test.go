package monitor

import (
	"encoding/binary"
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

// setupDS3Mock sets up a mock with Dark Souls III process and a death count value.
// It also sets up the save slot and character name memory chains so that
// TryDetectSave works correctly.
func setupDS3Mock(deathCount uint32) (*mockProcessOps, *memreader.GameReader) {
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
	gameDataManGlobalAddr := uintptr(0x500000000) // address of the global pointer variable
	gameDataManPtr := uint64(0x300000000)          // the GameDataMan object itself
	gameDataManBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(gameDataManBytes, gameDataManPtr)
	mock.memory[gameDataManGlobalAddr] = gameDataManBytes

	// GameMan global pointer at a simulated AOB-resolved address
	gameManGlobalAddr := uintptr(0x600000000)
	gameManPtr := uint64(0x700000000)
	gameManBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(gameManBytes, gameManPtr)
	mock.memory[gameManGlobalAddr] = gameManBytes

	// Save slot index at GameMan + 0xA60
	slotBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(slotBytes, 0) // slot 0
	mock.memory[uintptr(gameManPtr+0xA60)] = slotBytes

	// Hollowing at GameMan + 0x204E
	hollowBytes := make([]byte, 8)
	hollowBytes[0] = 15 // hollowing level 15
	mock.memory[uintptr(gameManPtr+0x204E)] = hollowBytes

	// PlayerGameData: GameDataMan + 0x10 -> PlayerGameData object
	playerGameDataPtr := uint64(0x400000000)
	pgdBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(pgdBytes, playerGameDataPtr)
	mock.memory[uintptr(gameDataManPtr+0x10)] = pgdBytes

	// Character name at PlayerGameData + 0x88 (UTF-16LE "Knight")
	charName := encodeUTF16LE("Knight")
	mock.memory[uintptr(playerGameDataPtr+0x88)] = charName

	reader := memreader.NewGameReaderWithOps(mock)
	// Inject AOB-resolved addresses (bypasses real AOB scanning)
	reader.SetTestAOBAddresses(int64(gameDataManGlobalAddr), int64(gameManGlobalAddr))
	return mock, reader
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

func TestDeathCounterMonitor_NotAttached(t *testing.T) {
	mock := newMockProcessOps()
	reader := memreader.NewGameReaderWithOps(mock)
	tracker := newTestTracker(t)

	mon := NewDeathCounterMonitor(reader, tracker)
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.Status != "Waiting for game..." {
			t.Errorf("expected 'Waiting for game...' status, got %q", update.Status)
		}
		if update.GameName != "" {
			t.Errorf("expected empty game name, got %q", update.GameName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestDeathCounterMonitor_AttachAndRead(t *testing.T) {
	_, reader := setupDS3Mock(42)
	tracker := newTestTracker(t)

	mon := NewDeathCounterMonitor(reader, tracker)

	// First tick: attach → PhaseConnected, detect save → PhaseLoaded
	// But death count is not read until PhaseLoaded, and PhaseLoaded
	// only happens after save detection succeeds.
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.GameName != "Dark Souls III" {
			t.Errorf("expected 'Dark Souls III', got %q", update.GameName)
		}
		// First tick with save detection should transition to Loaded
		if update.Status != "Loaded" {
			t.Errorf("expected 'Loaded', got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}

	// Second tick: PhaseLoaded, reads death count
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.DeathCount != 42 {
			t.Errorf("expected death count 42, got %d", update.DeathCount)
		}
		if update.Hollowing != 15 {
			t.Errorf("expected hollowing 15, got %d", update.Hollowing)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestDeathCounterMonitor_DeathCountChange(t *testing.T) {
	mock, reader := setupDS3Mock(10)
	tracker := newTestTracker(t)

	mon := NewDeathCounterMonitor(reader, tracker)

	// First tick: attach + save detection → PhaseLoaded
	mon.Tick()
	<-mon.DisplayUpdates()

	// Second tick: read initial count
	mon.Tick()
	<-mon.DisplayUpdates()

	// Update death count in memory
	valueAddr := uintptr(0x200000000 + 0x98)
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(valueBytes, 15)
	mock.memory[valueAddr] = valueBytes

	// Third tick: should detect change
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.DeathCount != 15 {
			t.Errorf("expected death count 15, got %d", update.DeathCount)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestDeathCounterMonitor_Detach(t *testing.T) {
	mock, reader := setupDS3Mock(5)
	tracker := newTestTracker(t)

	mon := NewDeathCounterMonitor(reader, tracker)

	// First tick: attach + save detect
	mon.Tick()
	<-mon.DisplayUpdates()

	// Remove the process to simulate game exit
	delete(mock.processes, "DarkSoulsIII.exe")

	// Clear memory to cause read error (simulating process gone)
	for k := range mock.memory {
		delete(mock.memory, k)
	}

	// Next tick: read fails, detach
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.GameName != "" {
			t.Errorf("expected empty game name after detach, got %q", update.GameName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_NoRoute(t *testing.T) {
	_, reader := setupDS3Mock(0)
	tracker := newTestTracker(t)

	mon := NewRouteMonitor(reader, tracker, nil, nil)
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.Fields != nil {
			routeName, _ := update.Fields["route_name"].(string)
			if routeName != "" {
				t.Errorf("expected no route name, got %q", routeName)
			}
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_MatchingRoute(t *testing.T) {
	_, reader := setupDS3Mock(0)
	tracker := newTestTracker(t)

	routes := []*route.Route{
		{
			ID:   "ds3-any",
			Name: "DS3 Any%",
			Game: "Dark Souls III",
			Checkpoints: []route.Checkpoint{
				{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagID: 100},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes, nil)

	// First tick: attach → Connected, detect save → Loaded → start route
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		routeName, _ := update.Fields["route_name"].(string)
		if routeName != "DS3 Any%" {
			t.Errorf("expected route name 'DS3 Any%%', got %q", routeName)
		}
		if update.Status != "Tracking route" {
			t.Errorf("expected 'Tracking route' status, got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_NonMatchingRoute(t *testing.T) {
	_, reader := setupDS3Mock(0)
	tracker := newTestTracker(t)

	routes := []*route.Route{
		{
			ID:   "sekiro-any",
			Name: "Sekiro Any%",
			Game: "Sekiro",
			Checkpoints: []route.Checkpoint{
				{ID: "boss1", Name: "Genichiro", EventType: "boss_kill", EventFlagID: 100},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes, nil)
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.Fields != nil {
			routeName, _ := update.Fields["route_name"].(string)
			if routeName != "" {
				t.Errorf("expected no route name, got %q", routeName)
			}
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_countBackups(t *testing.T) {
	_, reader := setupDS3Mock(0)
	tracker := newTestTracker(t)
	mon := NewRouteMonitor(reader, tracker, nil, nil)

	events := []route.CheckpointEvent{
		{Checkpoint: route.Checkpoint{ID: "boss1", BackupFlagID: 101}}, // has encounter flag → NOT counted
		{Checkpoint: route.Checkpoint{ID: "boss2", BackupFlagID: 0}},   // no encounter flag → counted (kill-based)
		{Checkpoint: route.Checkpoint{ID: "boss3", BackupFlagID: 0}},   // no encounter flag → counted
	}

	count := mon.countBackups(events)
	if count != 2 {
		t.Errorf("expected 2 kill-based backups, got %d", count)
	}
}

func TestGameMonitor_PublishState_NonBlocking(t *testing.T) {
	mock := newMockProcessOps()
	reader := memreader.NewGameReaderWithOps(mock)
	tracker := newTestTracker(t)

	gm := InitGameMonitor[DeathCounterState](reader, tracker)

	// Publish multiple states without consuming — should not block
	for i := 0; i < 5; i++ {
		gm.PublishState(DeathCounterState{
			DeathCount: uint32(i),
		})
	}

	// Should get the latest state
	select {
	case update := <-gm.DisplayUpdates():
		if update.DeathCount != 4 {
			t.Errorf("expected latest death count 4, got %d", update.DeathCount)
		}
	default:
		t.Fatal("expected a display update")
	}
}

// --- Save detection tests ---

func TestDeathCounterMonitor_SaveDetection(t *testing.T) {
	_, reader := setupDS3Mock(5)
	tracker := newTestTracker(t)

	mon := NewDeathCounterMonitor(reader, tracker)

	// First tick: attach + save detection
	mon.Tick()

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

	// Second tick: reads death count + hollowing
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.Hollowing != 15 {
			t.Errorf("expected hollowing 15, got %d", update.Hollowing)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_DetectsSave(t *testing.T) {
	_, reader := setupDS3Mock(0)
	tracker := newTestTracker(t)

	routes := []*route.Route{
		{
			ID:   "ds3-any",
			Name: "DS3 Any%",
			Game: "Dark Souls III",
			Checkpoints: []route.Checkpoint{
				{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagID: 100},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes, nil)
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.CharacterName != "Knight" {
			t.Errorf("expected 'Knight', got %q", update.CharacterName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_SaveDetectionGatesRoute(t *testing.T) {
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

	reader := memreader.NewGameReaderWithOps(mock)
	reader.SetTestAOBAddresses(int64(gameDataManGlobalAddr), 0)
	tracker := newTestTracker(t)

	routes := []*route.Route{
		{
			ID:   "ds3-any",
			Name: "DS3 Any%",
			Game: "Dark Souls III",
			Checkpoints: []route.Checkpoint{
				{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagID: 100},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes, nil)
	mon.Tick()

	// Save detection fails → route should NOT start, phase stays Connected
	select {
	case update := <-mon.DisplayUpdates():
		if update.Status != "Connected" {
			t.Errorf("expected 'Connected' status while save pending, got %q", update.Status)
		}
		// No route should be active
		routeName, _ := update.Fields["route_name"].(string)
		if routeName != "" {
			t.Errorf("expected no route name before save detection, got %q", routeName)
		}
		// Death count should NOT be read yet (still in Connected phase)
		if update.DeathCount != 0 {
			t.Errorf("expected death count 0 in Connected phase, got %d", update.DeathCount)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_Slot255Rejected(t *testing.T) {
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

	reader := memreader.NewGameReaderWithOps(mock)
	reader.SetTestAOBAddresses(int64(gameDataManGlobalAddr), int64(gameManGlobalAddr))
	tracker := newTestTracker(t)

	routes := []*route.Route{
		{
			ID:   "ds3-any",
			Name: "DS3 Any%",
			Game: "Dark Souls III",
			Checkpoints: []route.Checkpoint{
				{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagID: 100},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes, nil)
	mon.Tick()

	// Slot 255 should be rejected → stays Connected, no route started
	select {
	case update := <-mon.DisplayUpdates():
		if update.Status != "Connected" {
			t.Errorf("expected 'Connected' with slot 255, got %q", update.Status)
		}
		routeName, _ := update.Fields["route_name"].(string)
		if routeName != "" {
			t.Errorf("expected no route with slot 255, got %q", routeName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_SaveChange_AbandonsRun(t *testing.T) {
	mock, reader := setupDS3Mock(0)
	tracker := newTestTracker(t)

	routes := []*route.Route{
		{
			ID:   "ds3-any",
			Name: "DS3 Any%",
			Game: "Dark Souls III",
			Checkpoints: []route.Checkpoint{
				{ID: "boss1", Name: "Iudex Gundyr", EventType: "boss_kill", EventFlagID: 100},
			},
		},
	}

	mon := NewRouteMonitor(reader, tracker, routes, nil)

	// First tick: attach, detect save, start route
	mon.Tick()
	<-mon.DisplayUpdates()

	// Change character name to simulate save switch
	setCharacterName(mock, "Pyromancer")

	// Second tick: save changed → abandon + restart
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.CharacterName != "Pyromancer" {
			t.Errorf("expected 'Pyromancer', got %q", update.CharacterName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

func TestRouteMonitor_NonDS3_SkipsSaveDetection(t *testing.T) {
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

	reader := memreader.NewGameReaderWithOps(mock)
	tracker := newTestTracker(t)

	mon := NewDeathCounterMonitor(reader, tracker)

	// First tick: attach → Connected, save unsupported → immediately Loaded
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.GameName != "Elden Ring" {
			t.Errorf("expected 'Elden Ring', got %q", update.GameName)
		}
		if update.Status != "Loaded" {
			t.Errorf("expected 'Loaded' for unsupported save detection, got %q", update.Status)
		}
	default:
		t.Fatal("expected a display update")
	}

	// Second tick: PhaseLoaded, reads death count
	mon.Tick()

	select {
	case update := <-mon.DisplayUpdates():
		if update.DeathCount != 7 {
			t.Errorf("expected death count 7, got %d", update.DeathCount)
		}
		// No character name for unsupported game
		if update.CharacterName != "" {
			t.Errorf("expected empty character name, got %q", update.CharacterName)
		}
	default:
		t.Fatal("expected a display update")
	}
}

// setCharacterName updates the character name in mock memory (PlayerGameData + 0x88).
func setCharacterName(mock *mockProcessOps, name string) {
	mock.memory[uintptr(0x400000088)] = encodeUTF16LE(name)
}
