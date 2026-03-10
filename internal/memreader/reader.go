package memreader

import (
	"errors"
	"fmt"
	"log"
)

// ErrNullPointer is returned when a null pointer is encountered in the chain.
// This typically means the game is still loading and player data isn't available yet.
var ErrNullPointer = errors.New("null pointer in chain")

// GameReader handles reading memory from FromSoftware games.
type GameReader struct {
	processHandle ProcessHandle
	baseAddress   uintptr
	game          *GameConfig
	is64Bit       bool
	attached      bool
	currentGame   string
	ops           ProcessOps

	// AOB-resolved pointer cache
	sprjEventFlagManAddr int64
	fieldAreaAddr        int64
	eventFlagInitDone    bool
}

// NewGameReaderWithOps creates a new GameReader with the given ProcessOps.
// This is primarily used for testing with mock implementations.
func NewGameReaderWithOps(ops ProcessOps) *GameReader {
	return &GameReader{ops: ops}
}

// Attach finds and attaches to any supported FromSoftware game process.
func (r *GameReader) Attach() error {
	for _, game := range supportedGames {
		pid, err := r.ops.FindProcessByName(game.ProcessName + ".exe")
		if err != nil {
			continue // Try next game
		}

		handle, err := r.ops.OpenProcess(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION, false, pid)
		if err != nil {
			continue
		}

		// Get base address
		baseAddr, err := r.ops.GetModuleBaseAddress(pid, game.ProcessName+".exe")
		if err != nil {
			r.ops.CloseHandle(handle)
			continue
		}

		// Check if process is 64-bit
		is64Bit, err := r.ops.IsProcess64Bit(handle)
		if err != nil {
			r.ops.CloseHandle(handle)
			continue
		}

		// Verify we have offsets for this architecture
		if is64Bit && game.Offsets64 == nil {
			r.ops.CloseHandle(handle)
			continue
		}
		if !is64Bit && game.Offsets32 == nil {
			r.ops.CloseHandle(handle)
			continue
		}

		r.processHandle = handle
		r.baseAddress = baseAddr
		r.game = &game
		r.is64Bit = is64Bit
		r.attached = true
		r.currentGame = game.Name

		return nil
	}

	return fmt.Errorf("no supported game process found")
}

// Detach closes the process handle.
func (r *GameReader) Detach() {
	if r.processHandle != 0 {
		r.ops.CloseHandle(r.processHandle)
		r.processHandle = 0
		r.attached = false
		r.currentGame = ""
		r.sprjEventFlagManAddr = 0
		r.fieldAreaAddr = 0
		r.eventFlagInitDone = false
	}
}

// IsAttached returns whether the reader is attached to a process.
func (r *GameReader) IsAttached() bool {
	return r.attached
}

// GetCurrentGame returns the name of the currently attached game.
func (r *GameReader) GetCurrentGame() string {
	return r.currentGame
}

// ReadDeathCount reads the death count from memory using pointer traversal.
func (r *GameReader) ReadDeathCount() (uint32, error) {
	if !r.attached {
		return 0, fmt.Errorf("not attached to process")
	}

	// Get the appropriate offsets for the architecture
	var offsets []int64
	if r.is64Bit {
		offsets = r.game.Offsets64
	} else {
		offsets = r.game.Offsets32
	}

	if len(offsets) == 0 {
		return 0, fmt.Errorf("no offsets available for this game architecture")
	}

	// Start at base address
	address := int64(r.baseAddress)
	buffer := make([]byte, 8)

	// Follow the pointer chain
	for _, offset := range offsets {
		if address == 0 {
			return 0, ErrNullPointer
		}

		address += offset

		// Read memory at current address
		err := r.ops.ReadProcessMemory(r.processHandle, uintptr(address), buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read memory at 0x%X: %w", address, err)
		}

		// Parse as pointer (32-bit or 64-bit)
		if r.is64Bit {
			address = int64(uint64(buffer[0]) |
				uint64(buffer[1])<<8 |
				uint64(buffer[2])<<16 |
				uint64(buffer[3])<<24 |
				uint64(buffer[4])<<32 |
				uint64(buffer[5])<<40 |
				uint64(buffer[6])<<48 |
				uint64(buffer[7])<<56)
		} else {
			address = int64(uint32(buffer[0]) |
				uint32(buffer[1])<<8 |
				uint32(buffer[2])<<16 |
				uint32(buffer[3])<<24)
		}
	}

	// Final address value IS the death count (not a pointer)
	return uint32(address), nil
}

// initEventFlagPointers runs AOB scanning to find SprjEventFlagMan and FieldArea
// pointers. Results are cached so scanning only happens once per attach.
// Falls back to static offsets if AOB scanning fails.
func (r *GameReader) initEventFlagPointers() {
	if r.eventFlagInitDone {
		return
	}
	r.eventFlagInitDone = true

	// Try AOB scan for SprjEventFlagMan
	if r.game.SprjEventFlagManAOB != nil {
		aob := r.game.SprjEventFlagManAOB
		addr, err := r.ScanForPointer(aob.Pattern, aob.RelativeOffsetPos, aob.InstrLen)
		if err != nil {
			log.Printf("[AOB] SprjEventFlagMan scan failed, using static offsets: %v", err)
		} else {
			if aob.Dereference {
				ptr, err := r.readPtr(addr)
				if err != nil {
					log.Printf("[AOB] SprjEventFlagMan dereference failed: %v", err)
				} else {
					r.sprjEventFlagManAddr = ptr
					log.Printf("[AOB] SprjEventFlagMan resolved: 0x%X", r.sprjEventFlagManAddr)
				}
			} else {
				r.sprjEventFlagManAddr = addr
				log.Printf("[AOB] SprjEventFlagMan resolved: 0x%X", r.sprjEventFlagManAddr)
			}
		}
	}

	// Try AOB scan for FieldArea
	if r.game.FieldAreaAOB != nil {
		aob := r.game.FieldAreaAOB
		addr, err := r.ScanForPointer(aob.Pattern, aob.RelativeOffsetPos, aob.InstrLen)
		if err != nil {
			log.Printf("[AOB] FieldArea scan failed, using static offsets: %v", err)
		} else {
			if aob.Dereference {
				ptr, err := r.readPtr(addr)
				if err != nil {
					log.Printf("[AOB] FieldArea dereference failed: %v", err)
				} else {
					r.fieldAreaAddr = ptr
					log.Printf("[AOB] FieldArea resolved: 0x%X", r.fieldAreaAddr)
				}
			} else {
				r.fieldAreaAddr = addr
				log.Printf("[AOB] FieldArea resolved: 0x%X", r.fieldAreaAddr)
			}
		}
	}
}

// ReadEventFlag checks if a game event flag is set (boss killed, bonfire lit, etc.).
// Implements the DS3 hierarchical flag lookup ported from SoulSplitter.
func (r *GameReader) ReadEventFlag(flagID uint32) (bool, error) {
	if !r.attached {
		return false, fmt.Errorf("not attached to process")
	}

	if !r.is64Bit || r.game.EventFlagOffsets64 == nil {
		return false, fmt.Errorf("event flag reading not supported for this game")
	}

	// Lazily initialize AOB-scanned pointers
	r.initEventFlagPointers()

	// Decompose flag ID into components
	div10000000 := int(flagID/10000000) % 10
	area := int(flagID/100000) % 100
	div10000 := int(flagID/10000) % 10
	div1000 := int(flagID/1000) % 10
	remainder := int(flagID % 1000)

	// Step 1: Determine category
	category := -1
	if area >= 90 || area+div10000 == 0 {
		category = 0
	} else {
		cat, err := r.lookupFieldAreaCategory(area, div10000)
		if err != nil {
			return false, err
		}
		if cat >= 0 {
			category = cat + 1
		}
	}

	if category < 0 {
		return false, fmt.Errorf("could not resolve event flag category for flag %d", flagID)
	}

	// Step 2: Follow SprjEventFlagMan pointer chain (use AOB-cached address if available)
	var sprjBase int64
	if r.sprjEventFlagManAddr != 0 {
		sprjBase = r.sprjEventFlagManAddr
	} else {
		var err error
		sprjBase, err = r.followPointerChain(r.game.EventFlagOffsets64)
		if err != nil {
			return false, err
		}
	}

	// Navigate: sprjBase + 0x218 → [div10000000 * 0x18] → [0x0]
	ptr, err := r.readPtr(sprjBase + 0x218)
	if err != nil {
		return false, fmt.Errorf("failed to read SprjEventFlagMan array: %w", err)
	}
	if ptr == 0 {
		return false, ErrNullPointer
	}

	ptr, err = r.readPtr(ptr + int64(div10000000*0x18))
	if err != nil {
		return false, fmt.Errorf("failed to read flag array entry: %w", err)
	}
	if ptr == 0 {
		return false, ErrNullPointer
	}

	// Step 3: Compute final data address
	dataAddr := ptr + int64(div1000<<4) + int64(category)*0xa8

	// Dereference to get the flag data pointer
	flagDataPtr, err := r.readPtr(dataAddr)
	if err != nil {
		return false, fmt.Errorf("failed to read flag data pointer: %w", err)
	}
	if flagDataPtr == 0 {
		return false, ErrNullPointer
	}

	// Step 4: Read the bit from a uint32
	dwordIndex := (remainder >> 5) * 4
	bitIndex := 0x1f - (remainder & 0x1f)

	val, err := r.readUint32(flagDataPtr + int64(dwordIndex))
	if err != nil {
		return false, fmt.Errorf("failed to read event flag data: %w", err)
	}

	mask := uint32(1) << uint(bitIndex)
	return (val & mask) != 0, nil
}

// lookupFieldAreaCategory resolves the world block info category for a given area and block.
// Ported from SoulSplitter: _fieldArea.Append(0x0, 0x10).CreatePointerFromAddress()
func (r *GameReader) lookupFieldAreaCategory(area, block int) (int, error) {
	// Use AOB-cached address if available, otherwise fall back to static offsets
	var fieldArea int64
	if r.fieldAreaAddr != 0 {
		fieldArea = r.fieldAreaAddr
	} else {
		if r.game.FieldAreaOffsets64 == nil {
			return -1, fmt.Errorf("FieldArea offsets not configured")
		}
		var err error
		fieldArea, err = r.followPointerChain(r.game.FieldAreaOffsets64)
		if err != nil {
			log.Printf("[EventFlag] FieldArea chain error: %v", err)
			return -1, err
		}
	}
	if fieldArea == 0 {
		return -1, ErrNullPointer
	}

	// FieldArea → deref [0x0] → deref [0x10] = worldInfoOwner base
	ptr1, err := r.readPtr(fieldArea)
	if err != nil {
		return -1, fmt.Errorf("failed to read FieldArea: %w", err)
	}
	if ptr1 == 0 {
		return -1, ErrNullPointer
	}

	worldInfoOwner, err := r.readPtr(ptr1 + 0x10)
	if err != nil {
		return -1, fmt.Errorf("failed to read WorldInfoOwner: %w", err)
	}
	if worldInfoOwner == 0 {
		return -1, ErrNullPointer
	}

	// Read size at worldInfoOwner + 0x8
	size, err := r.readInt32(worldInfoOwner + 0x8)
	if err != nil {
		return -1, fmt.Errorf("failed to read world info size: %w", err)
	}

	// Vector base is a pointer at worldInfoOwner + 0x10
	vectorBase, err := r.readPtr(worldInfoOwner + 0x10)
	if err != nil {
		return -1, fmt.Errorf("failed to read world info vector base: %w", err)
	}
	if vectorBase == 0 {
		return -1, ErrNullPointer
	}

	for i := range size {
		entryBase := vectorBase + int64(i)*0x38

		// Read area byte at offset 0xb
		entryArea, err := r.readByte(entryBase + 0xb)
		if err != nil {
			continue
		}

		if int(entryArea) != area {
			continue
		}

		// Found matching area, now search block sub-vector
		count, err := r.readByte(entryBase + 0x20)
		if err != nil {
			continue
		}

		if count < 1 {
			continue
		}

		blockVectorPtr, err := r.readPtr(entryBase + 0x28)
		if err != nil {
			continue
		}

		for j := range count {
			blockEntry := blockVectorPtr + int64(j)*0x70

			flag, err := r.readInt32(blockEntry + 0x8)
			if err != nil {
				continue
			}

			flagArea := int((flag >> 0x18) & 0xff)
			flagBlock := int((flag >> 0x10) & 0xff)

			if flagArea == area && flagBlock == block {
				cat, err := r.readInt32(blockEntry + 0x20)
				if err != nil {
					return -1, fmt.Errorf("failed to read category: %w", err)
				}
				return int(cat), nil
			}
		}
	}

	return -1, nil
}

// followPointerChain follows a chain of offsets, dereferencing all except the last.
func (r *GameReader) followPointerChain(offsets []int64) (int64, error) {
	address := int64(r.baseAddress)
	for i, offset := range offsets {
		if address == 0 {
			return 0, ErrNullPointer
		}
		address += offset
		if i < len(offsets)-1 {
			ptr, err := r.readPtr(address)
			if err != nil {
				return 0, fmt.Errorf("failed to read pointer at 0x%X: %w", address, err)
			}
			address = ptr
		}
	}
	return address, nil
}

// readPtr reads an 8-byte pointer at the given address.
func (r *GameReader) readPtr(address int64) (int64, error) {
	buf := make([]byte, 8)
	err := r.ops.ReadProcessMemory(r.processHandle, uintptr(address), buf)
	if err != nil {
		return 0, err
	}
	return int64(uint64(buf[0]) |
		uint64(buf[1])<<8 |
		uint64(buf[2])<<16 |
		uint64(buf[3])<<24 |
		uint64(buf[4])<<32 |
		uint64(buf[5])<<40 |
		uint64(buf[6])<<48 |
		uint64(buf[7])<<56), nil
}

// readUint32 reads a 4-byte unsigned integer at the given address.
func (r *GameReader) readUint32(address int64) (uint32, error) {
	buf := make([]byte, 4)
	err := r.ops.ReadProcessMemory(r.processHandle, uintptr(address), buf)
	if err != nil {
		return 0, err
	}
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24, nil
}

// readInt32 reads a 4-byte signed integer at the given address.
func (r *GameReader) readInt32(address int64) (int32, error) {
	v, err := r.readUint32(address)
	return int32(v), err
}

// readByte reads a single byte at the given address.
func (r *GameReader) readByte(address int64) (byte, error) {
	buf := make([]byte, 1)
	err := r.ops.ReadProcessMemory(r.processHandle, uintptr(address), buf)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

// ReadMemoryValue follows a named pointer path, adds an extra offset, and reads
// an integer value of the given size (1, 2, or 4 bytes). Returns the value as uint32.
func (r *GameReader) ReadMemoryValue(pathName string, extraOffset int64, size int) (uint32, error) {
	if !r.attached {
		return 0, fmt.Errorf("not attached to process")
	}

	if !r.is64Bit || r.game.MemoryPaths == nil {
		return 0, fmt.Errorf("memory path reading not supported for this game")
	}

	offsets, ok := r.game.MemoryPaths[pathName]
	if !ok {
		return 0, fmt.Errorf("unknown memory path %q", pathName)
	}

	if size == 0 {
		size = 4
	}

	// Follow pointer chain
	address := int64(r.baseAddress)
	buffer := make([]byte, 8)

	for _, offset := range offsets {
		if address == 0 {
			return 0, ErrNullPointer
		}

		address += offset

		err := r.ops.ReadProcessMemory(r.processHandle, uintptr(address), buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read memory at 0x%X: %w", address, err)
		}

		address = int64(uint64(buffer[0]) |
			uint64(buffer[1])<<8 |
			uint64(buffer[2])<<16 |
			uint64(buffer[3])<<24 |
			uint64(buffer[4])<<32 |
			uint64(buffer[5])<<40 |
			uint64(buffer[6])<<48 |
			uint64(buffer[7])<<56)
	}

	if address == 0 {
		return 0, ErrNullPointer
	}

	// Read value at resolved address + extra offset
	valueAddr := uintptr(address + extraOffset)
	err := r.ops.ReadProcessMemory(r.processHandle, valueAddr, buffer)
	if err != nil {
		return 0, fmt.Errorf("failed to read value at 0x%X: %w", valueAddr, err)
	}

	switch size {
	case 1:
		return uint32(buffer[0]), nil
	case 2:
		return uint32(uint16(buffer[0]) | uint16(buffer[1])<<8), nil
	default:
		return uint32(buffer[0]) |
			uint32(buffer[1])<<8 |
			uint32(buffer[2])<<16 |
			uint32(buffer[3])<<24, nil
	}
}

// ReadIGT reads the in-game time in milliseconds.
func (r *GameReader) ReadIGT() (int64, error) {
	if !r.attached {
		return 0, fmt.Errorf("not attached to process")
	}

	if !r.is64Bit || r.game.IGTOffsets64 == nil {
		return 0, fmt.Errorf("IGT reading not supported for this game")
	}

	offsets := r.game.IGTOffsets64

	// Follow pointer chain to IGT value
	address := int64(r.baseAddress)
	buffer := make([]byte, 8)

	for i, offset := range offsets {
		if address == 0 {
			return 0, ErrNullPointer
		}

		address += offset

		err := r.ops.ReadProcessMemory(r.processHandle, uintptr(address), buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read memory at 0x%X: %w", address, err)
		}

		// Last value in chain is the IGT value (int32 ms), not a pointer
		if i < len(offsets)-1 {
			address = int64(uint64(buffer[0]) |
				uint64(buffer[1])<<8 |
				uint64(buffer[2])<<16 |
				uint64(buffer[3])<<24 |
				uint64(buffer[4])<<32 |
				uint64(buffer[5])<<40 |
				uint64(buffer[6])<<48 |
				uint64(buffer[7])<<56)
		}
	}

	// Parse IGT as int32 milliseconds
	igtMs := int64(int32(uint32(buffer[0]) |
		uint32(buffer[1])<<8 |
		uint32(buffer[2])<<16 |
		uint32(buffer[3])<<24))

	return igtMs, nil
}
