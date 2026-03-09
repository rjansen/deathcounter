package memreader

import (
	"encoding/binary"
	"fmt"
	"log"
	"strings"
)

const (
	// aobChunkSize is the size of each memory chunk read during AOB scanning.
	aobChunkSize = 64 * 1024 // 64 KB
)

// aobPattern represents a byte pattern with wildcards for scanning.
type aobPattern struct {
	bytes []byte
	mask  []bool // true = must match, false = wildcard
}

// parseAOBPattern parses a hex string pattern like "48 c7 05 ? ? ? ? 00".
// Each token is either a two-character hex byte or "?" for wildcard.
func parseAOBPattern(pattern string) aobPattern {
	tokens := strings.Fields(pattern)
	p := aobPattern{
		bytes: make([]byte, len(tokens)),
		mask:  make([]bool, len(tokens)),
	}
	for i, tok := range tokens {
		if tok == "?" || tok == "??" {
			p.bytes[i] = 0
			p.mask[i] = false
		} else {
			var b byte
			fmt.Sscanf(tok, "%x", &b)
			p.bytes[i] = b
			p.mask[i] = true
		}
	}
	return p
}

// scanAOB scans a byte slice for the given pattern, returns the offset of
// the first match or -1 if not found.
func scanAOB(data []byte, pattern aobPattern) int {
	patLen := len(pattern.bytes)
	if patLen == 0 || len(data) < patLen {
		return -1
	}

	limit := len(data) - patLen
	for i := 0; i <= limit; i++ {
		found := true
		for j := 0; j < patLen; j++ {
			if pattern.mask[j] && data[i+j] != pattern.bytes[j] {
				found = false
				break
			}
		}
		if found {
			return i
		}
	}
	return -1
}

// resolveRelativePtr resolves a RIP-relative address.
// matchAddr is the absolute address of the pattern match in the target process.
// data is the local buffer containing the matched bytes.
// matchOffset is the offset within data where the match starts.
// relativeOffsetPos is the position within the pattern where the int32 relative offset lives.
// instrLen is the total instruction length (used as base for RIP-relative calculation).
//
// Formula: matchAddr + instrLen + int32_at_position = target address
// (relativeOffsetPos is only used to locate the displacement bytes in the buffer)
func resolveRelativePtr(matchAddr int64, data []byte, matchOffset int, relativeOffsetPos int, instrLen int) int64 {
	// Read the int32 relative offset from the matched data
	relBytes := data[matchOffset+relativeOffsetPos : matchOffset+relativeOffsetPos+4]
	relOffset := int32(binary.LittleEndian.Uint32(relBytes))

	return matchAddr + int64(instrLen) + int64(relOffset)
}

// textSectionInfo holds the virtual address and size of the .text section.
type textSectionInfo struct {
	virtualAddress int64
	virtualSize    int64
}

// findTextSection reads the PE header from the process memory to locate the .text section.
func (r *GameReader) findTextSection() (*textSectionInfo, error) {
	base := int64(r.baseAddress)

	// Read DOS header: e_lfanew at offset 0x3C (uint32)
	eLfanew, err := r.readUint32(base + 0x3C)
	if err != nil {
		return nil, fmt.Errorf("failed to read e_lfanew: %w", err)
	}

	peHeader := base + int64(eLfanew)

	// Read NumberOfSections at PE header + 0x6 (uint16)
	buf := make([]byte, 2)
	err = r.ops.ReadProcessMemory(r.processHandle, uintptr(peHeader+0x6), buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read NumberOfSections: %w", err)
	}
	numSections := binary.LittleEndian.Uint16(buf)

	// Read SizeOfOptionalHeader at PE header + 0x14 (uint16)
	err = r.ops.ReadProcessMemory(r.processHandle, uintptr(peHeader+0x14), buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read SizeOfOptionalHeader: %w", err)
	}
	optHeaderSize := binary.LittleEndian.Uint16(buf)

	// Section headers start at PE header + 0x18 + SizeOfOptionalHeader
	sectionStart := peHeader + 0x18 + int64(optHeaderSize)

	// Each section header is 40 bytes
	sectionBuf := make([]byte, 40)
	for i := uint16(0); i < numSections; i++ {
		sectionAddr := sectionStart + int64(i)*40
		err = r.ops.ReadProcessMemory(r.processHandle, uintptr(sectionAddr), sectionBuf)
		if err != nil {
			continue
		}

		// Name is first 8 bytes, null-terminated
		name := string(sectionBuf[0:8])
		// Trim null bytes
		name = strings.TrimRight(name, "\x00")

		if name == ".text" {
			virtualSize := binary.LittleEndian.Uint32(sectionBuf[8:12])
			virtualAddress := binary.LittleEndian.Uint32(sectionBuf[12:16])
			return &textSectionInfo{
				virtualAddress: int64(virtualAddress),
				virtualSize:    int64(virtualSize),
			}, nil
		}
	}

	return nil, fmt.Errorf(".text section not found")
}

// ScanForPointer scans game memory for an AOB pattern and resolves the pointer.
// It reads the .text section in chunks and searches for the pattern.
// pattern is a hex string like "48 c7 05 ? ? ? ? 00 00 00 00".
// relativeOffsetPos is the position within the pattern where the int32 relative offset lives.
// instrLen is the total instruction length for the RIP-relative calculation.
func (r *GameReader) ScanForPointer(pattern string, relativeOffsetPos int, instrLen int) (int64, error) {
	if !r.attached {
		return 0, fmt.Errorf("not attached to process")
	}

	aob := parseAOBPattern(pattern)
	patLen := len(aob.bytes)

	// Find the .text section
	textSection, err := r.findTextSection()
	if err != nil {
		return 0, fmt.Errorf("failed to find .text section: %w", err)
	}

	base := int64(r.baseAddress)
	scanStart := base + textSection.virtualAddress
	scanEnd := scanStart + textSection.virtualSize

	log.Printf("[AOB] Scanning .text section: 0x%X - 0x%X (%d bytes)",
		scanStart, scanEnd, textSection.virtualSize)

	// Read in chunks with overlap to handle patterns spanning chunk boundaries
	overlap := patLen - 1
	buf := make([]byte, aobChunkSize+overlap)

	for addr := scanStart; addr < scanEnd; {
		// Calculate how much to read
		remaining := scanEnd - addr
		readSize := int64(aobChunkSize + overlap)
		if readSize > remaining {
			readSize = remaining
		}
		if readSize < int64(patLen) {
			break
		}

		chunk := buf[:readSize]
		err := r.ops.ReadProcessMemory(r.processHandle, uintptr(addr), chunk)
		if err != nil {
			// Skip unreadable regions
			addr += int64(aobChunkSize)
			continue
		}

		offset := scanAOB(chunk, aob)
		if offset >= 0 {
			matchAddr := addr + int64(offset)
			log.Printf("[AOB] Pattern found at 0x%X", matchAddr)
			resolved := resolveRelativePtr(matchAddr, chunk, offset, relativeOffsetPos, instrLen)
			log.Printf("[AOB] Resolved pointer: 0x%X", resolved)
			return resolved, nil
		}

		// Advance by chunk size (not chunk+overlap) so the next read overlaps
		addr += int64(aobChunkSize)
	}

	return 0, fmt.Errorf("AOB pattern not found in .text section")
}
