package memreader

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

// mockProcessOps implements ProcessOps with configurable behavior for testing.
type mockProcessOps struct {
	// process name -> pid
	processes map[string]uint32
	// "pid:moduleName" -> base address
	modules map[string]uintptr
	// pid -> is64bit
	architectures map[uint32]bool
	// address -> bytes (always 8 bytes)
	memory map[uintptr][]byte

	// Error injection (overrides normal behavior when set)
	findProcessErr    error
	openProcessErr    error
	getModuleErr      error
	isProcess64BitErr error
	readMemoryErr     error
	closeHandleErr    error

	// Handle tracking
	nextHandle    ProcessHandle
	handleToPid   map[ProcessHandle]uint32
	openedHandles map[ProcessHandle]bool
	closedHandles map[ProcessHandle]bool
}

func newMockProcessOps() *mockProcessOps {
	return &mockProcessOps{
		processes:     make(map[string]uint32),
		modules:       make(map[string]uintptr),
		architectures: make(map[uint32]bool),
		memory:        make(map[uintptr][]byte),
		handleToPid:   make(map[ProcessHandle]uint32),
		openedHandles: make(map[ProcessHandle]bool),
		closedHandles: make(map[ProcessHandle]bool),
	}
}

func (m *mockProcessOps) FindProcessByName(name string) (uint32, error) {
	if m.findProcessErr != nil {
		return 0, m.findProcessErr
	}
	pid, ok := m.processes[name]
	if !ok {
		return 0, fmt.Errorf("process not found: %s", name)
	}
	return pid, nil
}

func (m *mockProcessOps) OpenProcess(access uint32, inheritHandle bool, pid uint32) (ProcessHandle, error) {
	if m.openProcessErr != nil {
		return 0, m.openProcessErr
	}
	m.nextHandle++
	handle := m.nextHandle
	m.handleToPid[handle] = pid
	m.openedHandles[handle] = true
	return handle, nil
}

func (m *mockProcessOps) GetModuleBaseAddress(pid uint32, moduleName string) (uintptr, error) {
	if m.getModuleErr != nil {
		return 0, m.getModuleErr
	}
	key := fmt.Sprintf("%d:%s", pid, moduleName)
	addr, ok := m.modules[key]
	if !ok {
		return 0, fmt.Errorf("module not found: %s", moduleName)
	}
	return addr, nil
}

func (m *mockProcessOps) IsProcess64Bit(handle ProcessHandle) (bool, error) {
	if m.isProcess64BitErr != nil {
		return false, m.isProcess64BitErr
	}
	pid, ok := m.handleToPid[handle]
	if !ok {
		return false, fmt.Errorf("unknown handle")
	}
	is64, ok := m.architectures[pid]
	if !ok {
		return false, fmt.Errorf("unknown architecture for pid %d", pid)
	}
	return is64, nil
}

func (m *mockProcessOps) ReadProcessMemory(handle ProcessHandle, address uintptr, buffer []byte) error {
	if m.readMemoryErr != nil {
		return m.readMemoryErr
	}
	data, ok := m.memory[address]
	if !ok {
		return fmt.Errorf("no data at address 0x%X", address)
	}
	copy(buffer, data)
	return nil
}

func (m *mockProcessOps) CloseHandle(handle ProcessHandle) error {
	if m.closeHandleErr != nil {
		return m.closeHandleErr
	}
	m.closedHandles[handle] = true
	return nil
}

// setMemory64 writes a little-endian uint64 value at the given address.
func (m *mockProcessOps) setMemory64(address uintptr, value uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, value)
	m.memory[address] = b
}

// setMemory32 writes a little-endian uint32 value at the given address (padded to 8 bytes).
func (m *mockProcessOps) setMemory32(address uintptr, value uint32) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b, value)
	m.memory[address] = b
}

// --- State management tests ---

func TestGetSupportedGames(t *testing.T) {
	games := GetSupportedGames()
	if len(games) != 6 {
		t.Fatalf("expected 6 games, got %d", len(games))
	}

	expected := []string{
		"Dark Souls: Prepare To Die Edition",
		"Dark Souls II",
		"Dark Souls III",
		"Dark Souls Remastered",
		"Sekiro: Shadows Die Twice",
		"Elden Ring",
	}
	for i, name := range expected {
		if games[i] != name {
			t.Errorf("game %d: expected %q, got %q", i, name, games[i])
		}
	}
}

func TestNewGameReaderWithOps_InitialState(t *testing.T) {
	mock := newMockProcessOps()
	reader := NewGameReaderWithOps(mock)

	if reader.IsAttached() {
		t.Error("new reader should not be attached")
	}
	if reader.GetCurrentGame() != "" {
		t.Errorf("new reader should have empty game name, got %q", reader.GetCurrentGame())
	}
}

func TestDetach(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	reader.Detach()

	if reader.IsAttached() {
		t.Error("should not be attached after detach")
	}
	if reader.GetCurrentGame() != "" {
		t.Errorf("game should be empty after detach, got %q", reader.GetCurrentGame())
	}
	if len(mock.closedHandles) != 1 {
		t.Errorf("expected 1 handle closed, got %d", len(mock.closedHandles))
	}
}

func TestDetach_WhenNotAttached(t *testing.T) {
	mock := newMockProcessOps()
	reader := NewGameReaderWithOps(mock)

	// Should not panic
	reader.Detach()

	if len(mock.closedHandles) != 0 {
		t.Errorf("should not close any handles, got %d", len(mock.closedHandles))
	}
}

// --- Attach flow tests ---

func TestAttach_64BitGame(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	reader := NewGameReaderWithOps(mock)
	err := reader.Attach()
	if err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	if !reader.IsAttached() {
		t.Error("should be attached")
	}
	if reader.GetCurrentGame() != "Dark Souls III" {
		t.Errorf("expected Dark Souls III, got %q", reader.GetCurrentGame())
	}
}

func TestAttach_32BitGame(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DARKSOULS.exe"] = 5678
	mock.modules["5678:DARKSOULS.exe"] = 0x400000
	mock.architectures[5678] = false // 32-bit

	reader := NewGameReaderWithOps(mock)
	err := reader.Attach()
	if err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	if !reader.IsAttached() {
		t.Error("should be attached")
	}
	if reader.GetCurrentGame() != "Dark Souls: Prepare To Die Edition" {
		t.Errorf("expected Dark Souls: Prepare To Die Edition, got %q", reader.GetCurrentGame())
	}
}

func TestAttach_ScansMultipleGames(t *testing.T) {
	mock := newMockProcessOps()
	// Only Elden Ring running (last in supportedGames)
	mock.processes["eldenring.exe"] = 9999
	mock.modules["9999:eldenring.exe"] = 0x140000000
	mock.architectures[9999] = true

	reader := NewGameReaderWithOps(mock)
	err := reader.Attach()
	if err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	if reader.GetCurrentGame() != "Elden Ring" {
		t.Errorf("expected Elden Ring, got %q", reader.GetCurrentGame())
	}
}

func TestAttach_NoGameRunning(t *testing.T) {
	mock := newMockProcessOps()

	reader := NewGameReaderWithOps(mock)
	err := reader.Attach()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if reader.IsAttached() {
		t.Error("should not be attached")
	}
}

func TestAttach_SkipsGameWithoutMatchingOffsets(t *testing.T) {
	mock := newMockProcessOps()
	// DS1 PTDE running as 64-bit, but it only has Offsets32
	mock.processes["DARKSOULS.exe"] = 1234
	mock.modules["1234:DARKSOULS.exe"] = 0x400000
	mock.architectures[1234] = true // 64-bit

	reader := NewGameReaderWithOps(mock)
	err := reader.Attach()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if reader.IsAttached() {
		t.Error("should not be attached")
	}
	// Handle should have been opened and then closed during cleanup
	if len(mock.openedHandles) != 1 {
		t.Errorf("expected 1 handle opened, got %d", len(mock.openedHandles))
	}
	if len(mock.closedHandles) != 1 {
		t.Errorf("expected 1 handle closed, got %d", len(mock.closedHandles))
	}
}

func TestAttach_CleansUpHandleOnModuleFailure(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	// Don't set up module → GetModuleBaseAddress will fail
	mock.architectures[1234] = true

	reader := NewGameReaderWithOps(mock)
	err := reader.Attach()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if len(mock.closedHandles) != 1 {
		t.Errorf("expected 1 handle closed, got %d", len(mock.closedHandles))
	}
}

func TestAttach_CleansUpHandleOnArchDetectFailure(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	// Don't set up architecture → IsProcess64Bit will fail

	reader := NewGameReaderWithOps(mock)
	err := reader.Attach()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if len(mock.closedHandles) != 1 {
		t.Errorf("expected 1 handle closed, got %d", len(mock.closedHandles))
	}
}

// --- Pointer chain traversal (ReadDeathCount) tests ---

func TestReadDeathCount_NotAttached(t *testing.T) {
	mock := newMockProcessOps()
	reader := NewGameReaderWithOps(mock)

	_, err := reader.ReadDeathCount()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReadDeathCount_64Bit_TwoOffsetChain(t *testing.T) {
	// DS3 chain: {0x47572B8, 0x98}
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	// base(0x140000000) + 0x47572B8 = 0x1447572B8 → read → pointer 0x20000000
	mock.setMemory64(0x1447572B8, 0x20000000)
	// 0x20000000 + 0x98 = 0x20000098 → read → death count 42
	mock.setMemory64(0x20000098, 42)

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	count, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}
	if count != 42 {
		t.Errorf("expected 42, got %d", count)
	}
}

func TestReadDeathCount_32Bit_TwoOffsetChain(t *testing.T) {
	// DS1 PTDE chain: {0xF78700, 0x5C}
	mock := newMockProcessOps()
	mock.processes["DARKSOULS.exe"] = 5678
	mock.modules["5678:DARKSOULS.exe"] = 0x400000
	mock.architectures[5678] = false // 32-bit

	// base(0x400000) + 0xF78700 = 0x1378700 → read → pointer 0x1000000
	mock.setMemory32(0x1378700, 0x1000000)
	// 0x1000000 + 0x5C = 0x100005C → read → death count 7
	mock.setMemory32(0x100005C, 7)

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	count, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}
	if count != 7 {
		t.Errorf("expected 7, got %d", count)
	}
}

func TestReadDeathCount_32Bit_LongChain(t *testing.T) {
	// DS2 32-bit chain: {0x1150414, 0x74, 0xB8, 0x34, 0x4, 0x28C, 0x100}
	mock := newMockProcessOps()
	mock.processes["DarkSoulsII.exe"] = 4321
	mock.modules["4321:DarkSoulsII.exe"] = 0x400000
	mock.architectures[4321] = false // 32-bit

	// Step 1: 0x400000 + 0x1150414 = 0x1550414 → 0x2000000
	mock.setMemory32(0x1550414, 0x2000000)
	// Step 2: 0x2000000 + 0x74 = 0x2000074 → 0x3000000
	mock.setMemory32(0x2000074, 0x3000000)
	// Step 3: 0x3000000 + 0xB8 = 0x30000B8 → 0x4000000
	mock.setMemory32(0x30000B8, 0x4000000)
	// Step 4: 0x4000000 + 0x34 = 0x4000034 → 0x5000000
	mock.setMemory32(0x4000034, 0x5000000)
	// Step 5: 0x5000000 + 0x4 = 0x5000004 → 0x6000000
	mock.setMemory32(0x5000004, 0x6000000)
	// Step 6: 0x6000000 + 0x28C = 0x600028C → 0x7000000
	mock.setMemory32(0x600028C, 0x7000000)
	// Step 7: 0x7000000 + 0x100 = 0x7000100 → death count 99
	mock.setMemory32(0x7000100, 99)

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	count, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}
	if count != 99 {
		t.Errorf("expected 99, got %d", count)
	}
}

func TestReadDeathCount_Zero(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	mock.setMemory64(0x1447572B8, 0x20000000)
	mock.setMemory64(0x20000098, 0) // zero deaths (new character)

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	count, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestReadDeathCount_MaxUint32(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	mock.setMemory64(0x1447572B8, 0x20000000)
	mock.setMemory64(0x20000098, uint64(math.MaxUint32))

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	count, err := reader.ReadDeathCount()
	if err != nil {
		t.Fatalf("ReadDeathCount failed: %v", err)
	}
	if count != math.MaxUint32 {
		t.Errorf("expected %d, got %d", uint32(math.MaxUint32), count)
	}
}

func TestReadDeathCount_NullPointerInChain(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	// First read returns null pointer (0)
	mock.setMemory64(0x1447572B8, 0)

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	_, err := reader.ReadDeathCount()
	if err == nil {
		t.Fatal("expected error for null pointer, got nil")
	}
}

func TestReadDeathCount_MemoryReadFailure(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	// Set up first hop but not the second → second read fails
	mock.setMemory64(0x1447572B8, 0x20000000)
	// Don't set memory at 0x20000098 → read will fail

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	_, err := reader.ReadDeathCount()
	if err == nil {
		t.Fatal("expected error for memory read failure, got nil")
	}
}

// --- ReadEventFlag tests ---

func attachDS3WithEventFlags(t *testing.T) (*mockProcessOps, *GameReader) {
	t.Helper()
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}
	return mock, reader
}

func TestReadEventFlag_FlagSet(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	// DS3 EventFlagOffsets64: {0x4768E78, 0x0, 0x0}
	// base(0x140000000) + 0x4768E78 = 0x144768E78 → pointer
	mock.setMemory64(0x144768E78, 0x30000000)
	// 0x30000000 + 0x0 = 0x30000000 → pointer
	mock.setMemory64(0x30000000, 0x40000000)
	// 0x40000000 + 0x0 = 0x40000000 → event flag manager base
	mock.setMemory64(0x40000000, 0x50000000)

	// Flag ID 13000800: byteOffset = 13000800/8 = 1625100, bitPos = 13000800%8 = 0
	flagAddr := uintptr(0x50000000 + 1625100)
	b := make([]byte, 8)
	b[0] = 0x01 // bit 0 set
	mock.memory[flagAddr] = b

	set, err := reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if !set {
		t.Error("expected flag to be set")
	}
}

func TestReadEventFlag_FlagNotSet(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	mock.setMemory64(0x144768E78, 0x30000000)
	mock.setMemory64(0x30000000, 0x40000000)
	mock.setMemory64(0x40000000, 0x50000000)

	flagAddr := uintptr(0x50000000 + 1625100)
	b := make([]byte, 8)
	b[0] = 0x00 // bit 0 not set
	mock.memory[flagAddr] = b

	set, err := reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if set {
		t.Error("expected flag to not be set")
	}
}

func TestReadEventFlag_BitPosition(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	mock.setMemory64(0x144768E78, 0x30000000)
	mock.setMemory64(0x30000000, 0x40000000)
	mock.setMemory64(0x40000000, 0x50000000)

	// Flag ID 13000803: byteOffset = 13000803/8 = 1625100, bitPos = 13000803%8 = 3
	flagAddr := uintptr(0x50000000 + 1625100)
	b := make([]byte, 8)
	b[0] = 0x08 // bit 3 set (0b00001000)
	mock.memory[flagAddr] = b

	set, err := reader.ReadEventFlag(13000803)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if !set {
		t.Error("expected flag at bit 3 to be set")
	}
}

func TestReadEventFlag_NotAttached(t *testing.T) {
	mock := newMockProcessOps()
	reader := NewGameReaderWithOps(mock)

	_, err := reader.ReadEventFlag(13000800)
	if err == nil {
		t.Fatal("expected error when not attached")
	}
}

func TestReadEventFlag_NullPointer(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	// First read returns null
	mock.setMemory64(0x144768E78, 0)

	_, err := reader.ReadEventFlag(13000800)
	if err == nil {
		t.Fatal("expected error for null pointer")
	}
}

// --- ReadIGT tests ---

func TestReadIGT(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	// DS3 IGTOffsets64: {0x4768E78, 0xA4}
	// base + 0x4768E78 → pointer
	mock.setMemory64(0x144768E78, 0x30000000)
	// 0x30000000 + 0xA4 = 0x300000A4 → IGT value
	mock.setMemory32(0x300000A4, 1234567) // 1234567 ms

	igt, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("ReadIGT: %v", err)
	}
	if igt != 1234567 {
		t.Errorf("got IGT %d, want 1234567", igt)
	}
}

func TestReadIGT_Zero(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	mock.setMemory64(0x144768E78, 0x30000000)
	mock.setMemory32(0x300000A4, 0)

	igt, err := reader.ReadIGT()
	if err != nil {
		t.Fatalf("ReadIGT: %v", err)
	}
	if igt != 0 {
		t.Errorf("got IGT %d, want 0", igt)
	}
}

func TestReadIGT_NotAttached(t *testing.T) {
	mock := newMockProcessOps()
	reader := NewGameReaderWithOps(mock)

	_, err := reader.ReadIGT()
	if err == nil {
		t.Fatal("expected error when not attached")
	}
}

func TestReadIGT_NullPointer(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	mock.setMemory64(0x144768E78, 0)

	_, err := reader.ReadIGT()
	if err == nil {
		t.Fatal("expected error for null pointer")
	}
}
