package memreader

import (
	"errors"
	"fmt"
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

	if offsets == nil || len(offsets) == 0 {
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

// ReadEventFlag checks if a game event flag is set (boss killed, bonfire lit, etc.).
// The flag ID is game-specific; for DS3, flags come from the event flag manager.
func (r *GameReader) ReadEventFlag(flagID uint32) (bool, error) {
	if !r.attached {
		return false, fmt.Errorf("not attached to process")
	}

	if !r.is64Bit || r.game.EventFlagOffsets64 == nil {
		return false, fmt.Errorf("event flag reading not supported for this game")
	}

	offsets := r.game.EventFlagOffsets64

	// Follow pointer chain to event flag manager base
	address := int64(r.baseAddress)
	buffer := make([]byte, 8)

	for _, offset := range offsets {
		if address == 0 {
			return false, ErrNullPointer
		}

		address += offset

		err := r.ops.ReadProcessMemory(r.processHandle, uintptr(address), buffer)
		if err != nil {
			return false, fmt.Errorf("failed to read memory at 0x%X: %w", address, err)
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
		return false, ErrNullPointer
	}

	// Compute byte offset and bit position from flag ID
	// DS3 event flag layout: flagID maps to byte offset and bit
	byteOffset := (flagID / 8)
	bitPos := flagID % 8

	flagAddr := uintptr(address) + uintptr(byteOffset)

	singleByte := make([]byte, 8)
	err := r.ops.ReadProcessMemory(r.processHandle, flagAddr, singleByte)
	if err != nil {
		return false, fmt.Errorf("failed to read event flag at 0x%X: %w", flagAddr, err)
	}

	return (singleByte[0] & (1 << bitPos)) != 0, nil
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
