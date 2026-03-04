package memreader

import "fmt"

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
			return 0, fmt.Errorf("null pointer in chain")
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
