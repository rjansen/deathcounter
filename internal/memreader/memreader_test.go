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

// setMemoryByte writes a single byte at the given address (padded to 8 bytes).
func (m *mockProcessOps) setMemoryByte(address uintptr, value byte) {
	b := make([]byte, 8)
	b[0] = value
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

// setupGlobalFlag sets up mock memory for a global event flag (area >= 90 or area+block == 0).
// Global flags have category=0 and skip the FieldArea lookup.
func setupGlobalFlag(mock *mockProcessOps) {
	// EventFlagOffsets64: {0x4768E78, 0x0} → SprjEventFlagMan
	// base(0x140000000) + 0x4768E78 = 0x144768E78
	mock.setMemory64(0x144768E78, 0x30000000) // SprjEventFlagMan

	// SprjEventFlagMan + DS3OffsetFlagArray → array of flag region pointers
	mock.setMemory64(uintptr(0x30000000+DS3OffsetFlagArray), 0x50000000) // array base

	// array[1] (div10000000=1 for flag 13000800) → 0x50000000 + 1*DS3FlagArrayEntryStride
	mock.setMemory64(uintptr(0x50000000+DS3FlagArrayEntryStride), 0x60000000) // flag region base

	// Flag data pointer: base + (div1000<<4) + category*DS3FlagCategoryStride
	// For flag 13000800: div1000=0, category=0 → 0x60000000
	mock.setMemory64(0x60000000, 0x70000000) // flag data
}

func TestReadEventFlag_GlobalFlag_Set(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	setupGlobalFlag(mock)

	// Flag 13000800: area=30, block=0. area+block != 0 and area < 90 → NOT global.
	// Use flag 19000800 instead (area=90 → global, category=0)
	// div10000000=1, area=90, div10000=0, div1000=0, remainder=800

	// For area >= 90: category=0
	// dwordIndex = (800 >> 5) * 4 = 25 * 4 = 100
	// bitIndex = 0x1f - (800 & 0x1f) = 31 - 0 = 31
	// mask = 1 << 31 = 0x80000000
	mock.setMemory32(0x70000000+100, 0x80000000) // bit 31 set

	set, err := reader.ReadEventFlag(19000800)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if !set {
		t.Error("expected global flag to be set")
	}
}

func TestReadEventFlag_GlobalFlag_NotSet(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	setupGlobalFlag(mock)

	mock.setMemory32(0x70000000+100, 0x00000000) // no bits set

	set, err := reader.ReadEventFlag(19000800)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if set {
		t.Error("expected global flag to not be set")
	}
}

// setupAreaFlag sets up mock memory for an area-specific event flag that requires FieldArea lookup.
func setupAreaFlag(mock *mockProcessOps, flagID uint32, category int32) {
	area := int(flagID/100000) % 100
	block := int(flagID/10000) % 10

	// SprjEventFlagMan chain
	mock.setMemory64(0x144768E78, 0x30000000)
	mock.setMemory64(uintptr(0x30000000+DS3OffsetFlagArray), 0x50000000)
	div10000000 := int(flagID/10000000) % 10
	mock.setMemory64(uintptr(int64(0x50000000)+int64(div10000000)*DS3FlagArrayEntryStride), 0x60000000)

	// FieldAreaOffsets64: {0x4768028, 0x0}
	// base + 0x4768028 = 0x144768028 → deref → 0xA0000000 (fieldArea, last offset 0x0 not derefed)
	mock.setMemory64(0x144768028, 0xA0000000) // fieldArea base

	// lookupFieldAreaCategory: readPtr(fieldArea) → ptr1
	mock.setMemory64(0xA0000000, 0xA0500000) // ptr1

	// readPtr(ptr1 + DS3OffsetFieldAreaPtr) → worldInfoOwner
	mock.setMemory64(uintptr(0xA0500000+DS3OffsetFieldAreaPtr), 0xA1000000) // worldInfoOwner

	// worldInfoOwner + DS3OffsetWorldInfoSize = size, + DS3OffsetWorldInfoVector = pointer to vector
	mock.setMemory32(uintptr(0xA1000000+DS3OffsetWorldInfoSize), 1)          // 1 world info entry
	mock.setMemory64(uintptr(0xA1000000+DS3OffsetWorldInfoVector), 0xA2000000) // vector pointer

	// Entry 0 at vectorBase(0xA2000000): area byte at +DS3OffsetWorldInfoArea
	vectorBase := uintptr(0xA2000000)
	mock.setMemoryByte(vectorBase+uintptr(DS3OffsetWorldInfoArea), byte(area))

	// Block count at entry +DS3OffsetBlockCount
	mock.setMemoryByte(vectorBase+uintptr(DS3OffsetBlockCount), 1)

	// Block vector ptr at entry +DS3OffsetBlockVector
	mock.setMemory64(vectorBase+uintptr(DS3OffsetBlockVector), 0xA3000000)

	// Block entry 0: packed flag at +DS3OffsetBlockFlag = (area << 24) | (block << 16)
	packedFlag := int32(area<<24) | int32(block<<16)
	mock.setMemory32(uintptr(int64(0xA3000000)+DS3OffsetBlockFlag), uint32(packedFlag))

	// Category at +DS3OffsetBlockCategory
	mock.setMemory32(uintptr(int64(0xA3000000)+DS3OffsetBlockCategory), uint32(category))

	// Flag data: base + (div1000<<4) + (category+1)*DS3FlagCategoryStride
	div1000 := int(flagID/1000) % 10
	dataAddr := uintptr(int64(0x60000000) + int64(div1000*0x10) + int64(int(category)+1)*DS3FlagCategoryStride)
	mock.setMemory64(dataAddr, 0x70000000) // flag data pointer
}

func TestReadEventFlag_AreaFlag_Set(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	// Flag 13000800: Iudex Gundyr
	// area=30, block=0, div1000=0, remainder=800
	setupAreaFlag(mock, 13000800, 5) // arbitrary category=5

	// dwordIndex = (800 >> 5) * 4 = 100
	// bitIndex = 31
	mock.setMemory32(0x70000000+100, 0x80000000)

	set, err := reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if !set {
		t.Error("expected area flag to be set")
	}
}

func TestReadEventFlag_AreaFlag_NotSet(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	setupAreaFlag(mock, 13000800, 5)

	mock.setMemory32(0x70000000+100, 0x00000000)

	set, err := reader.ReadEventFlag(13000800)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if set {
		t.Error("expected area flag to not be set")
	}
}

func TestReadEventFlag_BitPosition(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	setupGlobalFlag(mock)

	// Flag 19000803: remainder=803
	// dwordIndex = (803 >> 5) * 4 = 25 * 4 = 100
	// bitIndex = 0x1f - (803 & 0x1f) = 31 - 3 = 28
	// mask = 1 << 28 = 0x10000000
	mock.setMemory32(0x70000000+100, 0x10000000)

	set, err := reader.ReadEventFlag(19000803)
	if err != nil {
		t.Fatalf("ReadEventFlag: %v", err)
	}
	if !set {
		t.Error("expected flag at bit 28 to be set")
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

	// SprjEventFlagMan returns null
	mock.setMemory64(0x144768E78, 0)

	_, err := reader.ReadEventFlag(19000800)
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

// --- ReadMemoryValue tests ---

// setupGameDataManChain sets up a GameDataMan → PlayerGameData chain
// via AOB-resolved addresses. Returns the PlayerGameData base address
// (stats are inline on PlayerGameData).
func setupGameDataManChain(mock *mockProcessOps, reader *GameReader) uintptr {
	// GameDataMan global pointer at AOB-resolved address
	gameDataManGlobal := uintptr(0x800000000)
	mock.setMemory64(gameDataManGlobal, 0x30000000) // GameDataMan object
	mock.setMemory64(0x30000010, 0x40000000)         // PlayerGameData
	reader.SetTestAOBAddresses(int64(gameDataManGlobal), 0)
	return 0x40000000
}

func TestReadMemoryValue_4Byte(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	statsBase := setupGameDataManChain(mock, reader)

	// SoulLevel at offset 0x68 = 120
	mock.setMemory32(statsBase+0x68, 120)

	val, err := reader.ReadMemoryValue("player_stats", 0x68, 4)
	if err != nil {
		t.Fatalf("ReadMemoryValue: %v", err)
	}
	if val != 120 {
		t.Errorf("got %d, want 120", val)
	}
}

func TestReadMemoryValue_2Byte(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	statsBase := setupGameDataManChain(mock, reader)

	// 2-byte value at offset 0x100 = 500
	b := make([]byte, 8)
	b[0] = 0xF4 // 500 = 0x01F4
	b[1] = 0x01
	mock.memory[statsBase+0x100] = b

	val, err := reader.ReadMemoryValue("player_stats", 0x100, 2)
	if err != nil {
		t.Fatalf("ReadMemoryValue: %v", err)
	}
	if val != 500 {
		t.Errorf("got %d, want 500", val)
	}
}

func TestReadMemoryValue_1Byte(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	statsBase := setupGameDataManChain(mock, reader)

	b := make([]byte, 8)
	b[0] = 7 // weapon upgrade level
	mock.memory[statsBase+0x200] = b

	val, err := reader.ReadMemoryValue("player_stats", 0x200, 1)
	if err != nil {
		t.Fatalf("ReadMemoryValue: %v", err)
	}
	if val != 7 {
		t.Errorf("got %d, want 7", val)
	}
}

func TestReadMemoryValue_DefaultSize(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)
	statsBase := setupGameDataManChain(mock, reader)
	mock.setMemory32(statsBase+0x68, 42)

	// size=0 should default to 4
	val, err := reader.ReadMemoryValue("player_stats", 0x68, 0)
	if err != nil {
		t.Fatalf("ReadMemoryValue: %v", err)
	}
	if val != 42 {
		t.Errorf("got %d, want 42", val)
	}
}

func TestReadMemoryValue_UnknownPath(t *testing.T) {
	_, reader := attachDS3WithEventFlags(t)

	_, err := reader.ReadMemoryValue("nonexistent", 0, 4)
	if err == nil {
		t.Fatal("expected error for unknown path")
	}
}

func TestReadMemoryValue_NotAttached(t *testing.T) {
	mock := newMockProcessOps()
	reader := NewGameReaderWithOps(mock)

	_, err := reader.ReadMemoryValue("player_stats", 0x68, 4)
	if err == nil {
		t.Fatal("expected error when not attached")
	}
}

func TestReadMemoryValue_NullPointer(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	// GameDataMan global pointer resolves to null (game loading)
	gameDataManGlobal := uintptr(0x800000000)
	mock.setMemory64(gameDataManGlobal, 0) // null
	reader.SetTestAOBAddresses(int64(gameDataManGlobal), 0)

	_, err := reader.ReadMemoryValue("player_stats", 0x68, 4)
	if err == nil {
		t.Fatal("expected error for null pointer")
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

// --- readUTF16 tests ---

func setupDS3WithGameDataMan(t *testing.T) (*mockProcessOps, *GameReader) {
	t.Helper()
	mock, reader := attachDS3WithEventFlags(t)

	// GameDataMan via AOB-resolved global pointer
	gameDataManGlobal := uintptr(0x800000000)
	mock.setMemory64(gameDataManGlobal, 0x300000000) // GameDataMan object

	// PlayerGameData: GameDataMan + 0x10 -> 0x400000000
	mock.setMemory64(0x300000010, 0x400000000)

	reader.SetTestAOBAddresses(int64(gameDataManGlobal), 0)
	return mock, reader
}

func setCharacterName(mock *mockProcessOps, name string) {
	runes := []rune(name)
	size := (len(runes) + 1) * 2
	if size < 32 {
		size = 32
	}
	buf := make([]byte, size)
	for i, r := range runes {
		buf[i*2] = byte(r)
		buf[i*2+1] = byte(r >> 8)
	}
	// PlayerGameData base (0x400000000) + DS3OffsetCharName
	mock.memory[uintptr(0x400000000+DS3OffsetCharName)] = buf
}

func TestReadUTF16_BasicASCII(t *testing.T) {
	mock, reader := setupDS3WithGameDataMan(t)
	setCharacterName(mock, "Knight")

	name, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("ReadCharacterName: %v", err)
	}
	if name != "Knight" {
		t.Errorf("expected 'Knight', got %q", name)
	}
}

func TestReadUTF16_Empty(t *testing.T) {
	mock, reader := setupDS3WithGameDataMan(t)
	// Empty name (null first char)
	buf := make([]byte, 32)
	mock.memory[0x400000088] = buf

	name, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("ReadCharacterName: %v", err)
	}
	if name != "" {
		t.Errorf("expected empty string, got %q", name)
	}
}

func TestReadUTF16_MaxLength(t *testing.T) {
	mock, reader := setupDS3WithGameDataMan(t)
	setCharacterName(mock, "AbcdefghijklmnO") // 15 chars

	name, err := reader.ReadCharacterName()
	if err != nil {
		t.Fatalf("ReadCharacterName: %v", err)
	}
	if name != "AbcdefghijklmnO" {
		t.Errorf("expected 'AbcdefghijklmnO', got %q", name)
	}
}

func TestReadCharacterName_NullPointer(t *testing.T) {
	mock, reader := attachDS3WithEventFlags(t)

	// GameDataMan returns null
	mock.setMemory64(0x144768E78, 0)

	_, err := reader.ReadCharacterName()
	if err == nil {
		t.Fatal("expected error for null pointer")
	}
}

func TestReadCharacterName_Unsupported(t *testing.T) {
	// Elden Ring has no CharNamePathKey set
	mock := newMockProcessOps()
	mock.processes["eldenring.exe"] = 9999
	mock.modules["9999:eldenring.exe"] = 0x140000000
	mock.architectures[9999] = true

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	_, err := reader.ReadCharacterName()
	if err == nil {
		t.Fatal("expected error for unsupported game")
	}
	if err != ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}

func TestReadSaveSlotIndex_DS3_GameManAOB(t *testing.T) {
	_, reader := setupDS3WithGameDataMan(t)

	// Simulate AOB-resolved GameMan address
	gameManGlobalPtr := int64(0xB0000000)
	gameManObj := int64(0xC0000000)
	reader.gameManAOBAddr = gameManGlobalPtr

	// Write GameMan pointer at the AOB-resolved address
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(gameManObj))
	reader.ops.(*mockProcessOps).memory[uintptr(gameManGlobalPtr)] = b

	// Write save slot byte (3) at GameMan + DS3OffsetSaveSlot
	slotBuf := make([]byte, 8)
	slotBuf[0] = 3
	reader.ops.(*mockProcessOps).memory[uintptr(gameManObj+DS3OffsetSaveSlot)] = slotBuf

	slot, err := reader.ReadSaveSlotIndex()
	if err != nil {
		t.Fatalf("ReadSaveSlotIndex: %v", err)
	}
	if slot != 3 {
		t.Errorf("expected slot 3, got %d", slot)
	}
}

func TestReadSaveSlotIndex_NullPointer(t *testing.T) {
	_, reader := setupDS3WithGameDataMan(t)

	// Simulate AOB-resolved GameMan address that contains a null pointer
	gameManGlobalPtr := int64(0xB0000000)
	reader.gameManAOBAddr = gameManGlobalPtr

	// Write null pointer at the AOB-resolved address
	b := make([]byte, 8)
	reader.ops.(*mockProcessOps).memory[uintptr(gameManGlobalPtr)] = b

	_, err := reader.ReadSaveSlotIndex()
	if err == nil {
		t.Fatal("expected error for null pointer")
	}
	if err != ErrNullPointer {
		t.Errorf("expected ErrNullPointer, got %v", err)
	}
}

func TestReadSaveSlotIndex_NonDS3_Unsupported(t *testing.T) {
	// DSR has no SaveSlotPathKey set
	mock := newMockProcessOps()
	mock.processes["DarkSoulsRemastered.exe"] = 7777
	mock.modules["7777:DarkSoulsRemastered.exe"] = 0x140000000
	mock.architectures[7777] = true

	reader := NewGameReaderWithOps(mock)
	if err := reader.Attach(); err != nil {
		t.Fatalf("attach failed: %v", err)
	}

	_, err := reader.ReadSaveSlotIndex()
	if err == nil {
		t.Fatal("expected error for unsupported game")
	}
	if err != ErrNotSupported {
		t.Errorf("expected ErrNotSupported, got %v", err)
	}
}
