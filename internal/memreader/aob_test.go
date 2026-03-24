package memreader

import (
	"encoding/binary"
	"testing"
)

// --- parseAOBPattern tests ---

func TestParseAOBPattern_SimpleBytes(t *testing.T) {
	p := parseAOBPattern("48 c7 05")
	if len(p.bytes) != 3 {
		t.Fatalf("expected 3 bytes, got %d", len(p.bytes))
	}
	if p.bytes[0] != 0x48 || p.bytes[1] != 0xc7 || p.bytes[2] != 0x05 {
		t.Errorf("unexpected bytes: %x %x %x", p.bytes[0], p.bytes[1], p.bytes[2])
	}
	for i, m := range p.mask {
		if !m {
			t.Errorf("mask[%d] should be true", i)
		}
	}
}

func TestParseAOBPattern_WithWildcards(t *testing.T) {
	p := parseAOBPattern("48 ? ? 05")
	if len(p.bytes) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(p.bytes))
	}
	if !p.mask[0] || p.mask[1] || p.mask[2] || !p.mask[3] {
		t.Errorf("unexpected mask: %v", p.mask)
	}
	if p.bytes[0] != 0x48 || p.bytes[3] != 0x05 {
		t.Errorf("unexpected bytes: %x, %x", p.bytes[0], p.bytes[3])
	}
}

func TestParseAOBPattern_AllWildcards(t *testing.T) {
	p := parseAOBPattern("? ? ?")
	for i, m := range p.mask {
		if m {
			t.Errorf("mask[%d] should be false (wildcard)", i)
		}
	}
}

func TestParseAOBPattern_Empty(t *testing.T) {
	p := parseAOBPattern("")
	if len(p.bytes) != 0 {
		t.Errorf("expected empty pattern, got %d bytes", len(p.bytes))
	}
}

func TestParseAOBPattern_DoubleQuestionMark(t *testing.T) {
	p := parseAOBPattern("48 ?? 05")
	if len(p.bytes) != 3 {
		t.Fatalf("expected 3 bytes, got %d", len(p.bytes))
	}
	if p.mask[1] {
		t.Error("?? should be treated as wildcard")
	}
}

// --- scanAOB tests ---

func TestScanAOB_ExactMatch(t *testing.T) {
	data := []byte{0x00, 0x48, 0xc7, 0x05, 0x00}
	p := parseAOBPattern("48 c7 05")
	offset := scanAOB(data, p)
	if offset != 1 {
		t.Errorf("expected offset 1, got %d", offset)
	}
}

func TestScanAOB_MatchAtStart(t *testing.T) {
	data := []byte{0x48, 0xc7, 0x05, 0x00}
	p := parseAOBPattern("48 c7 05")
	offset := scanAOB(data, p)
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestScanAOB_MatchAtEnd(t *testing.T) {
	data := []byte{0x00, 0x00, 0x48, 0xc7, 0x05}
	p := parseAOBPattern("48 c7 05")
	offset := scanAOB(data, p)
	if offset != 2 {
		t.Errorf("expected offset 2, got %d", offset)
	}
}

func TestScanAOB_WithWildcards(t *testing.T) {
	data := []byte{0x00, 0x48, 0xAA, 0xBB, 0x05, 0x00}
	p := parseAOBPattern("48 ? ? 05")
	offset := scanAOB(data, p)
	if offset != 1 {
		t.Errorf("expected offset 1, got %d", offset)
	}
}

func TestScanAOB_NoMatch(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	p := parseAOBPattern("48 c7 05")
	offset := scanAOB(data, p)
	if offset != -1 {
		t.Errorf("expected -1, got %d", offset)
	}
}

func TestScanAOB_PatternLongerThanData(t *testing.T) {
	data := []byte{0x48, 0xc7}
	p := parseAOBPattern("48 c7 05")
	offset := scanAOB(data, p)
	if offset != -1 {
		t.Errorf("expected -1, got %d", offset)
	}
}

func TestScanAOB_EmptyPattern(t *testing.T) {
	data := []byte{0x48, 0xc7}
	p := parseAOBPattern("")
	offset := scanAOB(data, p)
	if offset != -1 {
		t.Errorf("expected -1 for empty pattern, got %d", offset)
	}
}

func TestScanAOB_EmptyData(t *testing.T) {
	p := parseAOBPattern("48 c7 05")
	offset := scanAOB(nil, p)
	if offset != -1 {
		t.Errorf("expected -1, got %d", offset)
	}
}

func TestScanAOB_ExactSizeMatch(t *testing.T) {
	data := []byte{0x48, 0xc7, 0x05}
	p := parseAOBPattern("48 c7 05")
	offset := scanAOB(data, p)
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestScanAOB_FirstMatchReturned(t *testing.T) {
	data := []byte{0x48, 0xc7, 0x00, 0x48, 0xc7, 0x00}
	p := parseAOBPattern("48 c7")
	offset := scanAOB(data, p)
	if offset != 0 {
		t.Errorf("expected first match at 0, got %d", offset)
	}
}

func TestScanAOB_DS3SprjPattern(t *testing.T) {
	// Test with the actual DS3 SprjEventFlagMan pattern structure
	pattern := "48 c7 05 ? ? ? ? 00 00 00 00 48 8b 7c 24 38 c7 46 54 ff ff ff ff 48 83 c4 20 5e c3"
	p := parseAOBPattern(pattern)

	// Build a byte slice that should match
	match := []byte{
		0x48, 0xc7, 0x05,
		0x12, 0x34, 0x56, 0x78, // wildcards (relative offset)
		0x00, 0x00, 0x00, 0x00,
		0x48, 0x8b, 0x7c, 0x24, 0x38,
		0xc7, 0x46, 0x54, 0xff, 0xff, 0xff, 0xff,
		0x48, 0x83, 0xc4, 0x20, 0x5e, 0xc3,
	}
	// Pad with some bytes before and after
	data := make([]byte, 100)
	copy(data[10:], match)

	offset := scanAOB(data, p)
	if offset != 10 {
		t.Errorf("expected offset 10, got %d", offset)
	}
}

// --- resolveRelativePtr tests ---

func TestResolveRelativePtr_PositiveOffset(t *testing.T) {
	// Simulate: instruction at address 0x1000, relative offset at position 3,
	// instruction length 7, relative offset = 0x100
	data := make([]byte, 20)
	binary.LittleEndian.PutUint32(data[3:7], 0x100) // relative offset = 256

	// Formula: matchAddr + instrLen + relOffset
	// = 0x1000 + 7 + 256 = 0x1107
	result := resolveRelativePtr(0x1000, data, 0, 3, 7)
	expected := int64(0x1000 + 7 + 256)
	if result != expected {
		t.Errorf("expected 0x%X, got 0x%X", expected, result)
	}
}

func TestResolveRelativePtr_NegativeOffset(t *testing.T) {
	// Negative relative offset (backward reference)
	data := make([]byte, 20)
	binary.LittleEndian.PutUint32(data[3:7], uint32(0xFFFFFF00)) // relative offset = -256

	// = 0x1000 + 7 + (-256) = 0x1000 + 7 - 256 = 0x0F07
	result := resolveRelativePtr(0x1000, data, 0, 3, 7)
	expected := int64(0x1000 + 7 - 256)
	if result != expected {
		t.Errorf("expected 0x%X, got 0x%X", expected, result)
	}
}

func TestResolveRelativePtr_WithMatchOffset(t *testing.T) {
	// Match starts at offset 10 within the buffer
	data := make([]byte, 30)
	binary.LittleEndian.PutUint32(data[13:17], 0x200) // matchOffset(10) + relPos(3) = 13

	result := resolveRelativePtr(0x2000, data, 10, 3, 11)
	expected := int64(0x2000 + 11 + 0x200)
	if result != expected {
		t.Errorf("expected 0x%X, got 0x%X", expected, result)
	}
}

func TestResolveRelativePtr_DS3SprjEventFlagMan(t *testing.T) {
	// DS3 SprjEventFlagMan: relativeOffsetPos=3, instrLen=11
	// Simulate a match at process address 0x140100000
	data := make([]byte, 40)
	// Put a known relative offset at position 3
	relOffset := int32(0x00367E68)
	binary.LittleEndian.PutUint32(data[3:7], uint32(relOffset))

	matchAddr := int64(0x140100000)
	result := resolveRelativePtr(matchAddr, data, 0, 3, 11)
	// 0x140100000 + 11 + 0x367E68 = 0x140467E73
	expected := matchAddr + 11 + int64(relOffset)
	if result != expected {
		t.Errorf("expected 0x%X, got 0x%X", expected, result)
	}
}

func TestResolveRelativePtr_DS3FieldArea(t *testing.T) {
	// DS3 FieldArea: relativeOffsetPos=3, instrLen=7
	data := make([]byte, 40)
	relOffset := int32(0x00366FE1)
	binary.LittleEndian.PutUint32(data[3:7], uint32(relOffset))

	matchAddr := int64(0x140101000)
	result := resolveRelativePtr(matchAddr, data, 0, 3, 7)
	expected := matchAddr + 7 + int64(relOffset)
	if result != expected {
		t.Errorf("expected 0x%X, got 0x%X", expected, result)
	}
}

// --- ScanForPointer integration test with mock ---

func TestScanForPointer_FindsPattern(t *testing.T) {
	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	base := uintptr(0x140000000)

	// Set up a minimal PE header
	// DOS header: e_lfanew at offset 0x3C
	dosHeader := make([]byte, 8)
	binary.LittleEndian.PutUint32(dosHeader, 0x80) // e_lfanew = 0x80
	mock.memory[base+0x3C] = dosHeader

	peBase := base + 0x80
	// NumberOfSections at PE + 0x6
	numSections := make([]byte, 2)
	binary.LittleEndian.PutUint16(numSections, 1) // 1 section
	mock.memory[peBase+0x6] = numSections

	// SizeOfOptionalHeader at PE + 0x14
	optSize := make([]byte, 2)
	binary.LittleEndian.PutUint16(optSize, 0xF0) // typical value
	mock.memory[peBase+0x14] = optSize

	// Section header at PE + 0x18 + 0xF0 = PE + 0x108
	sectionAddr := peBase + 0x108
	sectionHeader := make([]byte, 40)
	copy(sectionHeader[0:8], []byte(".text\x00\x00\x00"))
	binary.LittleEndian.PutUint32(sectionHeader[8:12], 0x1000)  // VirtualSize = 4096
	binary.LittleEndian.PutUint32(sectionHeader[12:16], 0x1000) // VirtualAddress = 0x1000
	mock.memory[sectionAddr] = sectionHeader

	// Now set up the .text section content at base + 0x1000
	// We need mock to support arbitrary-length reads. The current mock only works
	// with exact address lookups. Let's place the pattern at the start of .text.
	textAddr := base + 0x1000
	patternBytes := []byte{
		0x4c, 0x8b, 0x3d,
		0x00, 0x00, 0x00, 0x00, // relative offset (will fill in)
		0x8b, 0x45, 0x87,
	}
	// Set relative offset to 0x100
	binary.LittleEndian.PutUint32(patternBytes[3:7], 0x100)

	// The mock's ReadProcessMemory needs to handle chunk reads.
	// We need to extend the mock to handle range-based reads.
	// For this test, let's set up overlapping 8-byte entries to cover the chunk.
	// Actually, the mock reads exact addresses. We need a contiguous memory mock.

	// Let's use a custom mock that supports contiguous memory for this test.
	cmock := &contiguousMemoryMock{
		mockProcessOps: mock,
		contiguous:     make(map[uintptr][]byte),
	}

	// Store the text section as contiguous memory
	textContent := make([]byte, 0x1000) // 4096 bytes
	copy(textContent[0:], patternBytes)
	cmock.contiguous[textAddr] = textContent

	// Re-create reader with contiguous mock
	reader2 := attachGame(t, cmock, "ds3")

	result, err := reader2.ScanForPointer(
		"4c 8b 3d ? ? ? ? 8b 45 87",
		3, 7,
	)
	if err != nil {
		t.Fatalf("ScanForPointer: %v", err)
	}

	// matchAddr = 0x140001000 (base + 0x1000 + 0 offset in text)
	// resolved = 0x140001000 + 7 + 0x100 = 0x140001107
	expected := int64(0x140001000 + 7 + 0x100)
	if result != expected {
		t.Errorf("expected 0x%X, got 0x%X", expected, result)
	}
}

func TestScanForPointer_PatternNotFound(t *testing.T) {
	cmock := setupScanMock(t, make([]byte, 0x1000)) // empty text section

	reader := attachGame(t, cmock, "ds3")

	_, err := reader.ScanForPointer("48 c7 05 ? ? ? ? 00 00 00 00", 3, 11)
	if err == nil {
		t.Fatal("expected error when pattern not found")
	}
}

// contiguousMemoryMock wraps mockProcessOps but supports reading from contiguous
// memory regions, which is needed for AOB scanning (chunk reads).
type contiguousMemoryMock struct {
	*mockProcessOps
	contiguous map[uintptr][]byte // base address -> contiguous byte slice
}

func (c *contiguousMemoryMock) ReadProcessMemory(handle ProcessHandle, address uintptr, buffer []byte) error {
	// First try contiguous memory regions
	for base, data := range c.contiguous {
		if address >= base && int(address-base)+len(buffer) <= len(data) {
			offset := int(address - base)
			copy(buffer, data[offset:offset+len(buffer)])
			return nil
		}
	}
	// Fall back to exact-match mock
	return c.mockProcessOps.ReadProcessMemory(handle, address, buffer)
}

// setupScanMock creates a contiguousMemoryMock with a PE header and .text section
// containing the given content.
func setupScanMock(t *testing.T, textContent []byte) *contiguousMemoryMock {
	t.Helper()

	mock := newMockProcessOps()
	mock.processes["DarkSoulsIII.exe"] = 1234
	mock.modules["1234:DarkSoulsIII.exe"] = 0x140000000
	mock.architectures[1234] = true

	base := uintptr(0x140000000)

	// DOS header
	dosHeader := make([]byte, 8)
	binary.LittleEndian.PutUint32(dosHeader, 0x80)
	mock.memory[base+0x3C] = dosHeader

	peBase := base + 0x80
	numSections := make([]byte, 2)
	binary.LittleEndian.PutUint16(numSections, 1)
	mock.memory[peBase+0x6] = numSections

	optSize := make([]byte, 2)
	binary.LittleEndian.PutUint16(optSize, 0xF0)
	mock.memory[peBase+0x14] = optSize

	sectionAddr := peBase + 0x108
	sectionHeader := make([]byte, 40)
	copy(sectionHeader[0:8], []byte(".text\x00\x00\x00"))
	binary.LittleEndian.PutUint32(sectionHeader[8:12], uint32(len(textContent)))
	binary.LittleEndian.PutUint32(sectionHeader[12:16], 0x1000)
	mock.memory[sectionAddr] = sectionHeader

	cmock := &contiguousMemoryMock{
		mockProcessOps: mock,
		contiguous:     make(map[uintptr][]byte),
	}
	cmock.contiguous[base+0x1000] = textContent

	return cmock
}
