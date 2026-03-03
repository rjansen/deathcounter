package memreader

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = kernel32.NewProc("Process32FirstW")
	procProcess32Next            = kernel32.NewProc("Process32NextW")
	procModule32First            = kernel32.NewProc("Module32FirstW")
	procModule32Next             = kernel32.NewProc("Module32NextW")
	procIsWow64Process           = kernel32.NewProc("IsWow64Process")
)

const (
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
	TH32CS_SNAPPROCESS        = 0x00000002
	TH32CS_SNAPMODULE         = 0x00000008
	TH32CS_SNAPMODULE32       = 0x00000010
)

// PROCESSENTRY32 represents a process in the snapshot
type PROCESSENTRY32 struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [260]uint16
}

// MODULEENTRY32 represents a module in the snapshot
type MODULEENTRY32 struct {
	Size         uint32
	ModuleID     uint32
	ProcessID    uint32
	GlblcntUsage uint32
	ProccntUsage uint32
	ModBaseAddr  uintptr
	ModBaseSize  uint32
	HModule      syscall.Handle
	SzModule     [256]uint16
	SzExePath    [260]uint16
}

// GameConfig holds the configuration for a specific FromSoftware game
type GameConfig struct {
	Name       string
	ProcessName string
	Offsets32  []int64 // Offsets for 32-bit version (if exists)
	Offsets64  []int64 // Offsets for 64-bit version
}

var supportedGames = []GameConfig{
	{
		Name:        "Dark Souls: Prepare To Die Edition",
		ProcessName: "DARKSOULS",
		Offsets32:   []int64{0xF78700, 0x5C},
		Offsets64:   nil,
	},
	{
		Name:        "Dark Souls II",
		ProcessName: "DarkSoulsII",
		Offsets32:   []int64{0x1150414, 0x74, 0xB8, 0x34, 0x4, 0x28C, 0x100},
		Offsets64:   []int64{0x16148F0, 0xD0, 0x490, 0x104},
	},
	{
		Name:        "Dark Souls III",
		ProcessName: "DarkSoulsIII",
		Offsets32:   nil,
		Offsets64:   []int64{0x47572B8, 0x98},
	},
	{
		Name:        "Dark Souls Remastered",
		ProcessName: "DarkSoulsRemastered",
		Offsets32:   nil,
		Offsets64:   []int64{0x1C8A530, 0x98},
	},
	{
		Name:        "Sekiro: Shadows Die Twice",
		ProcessName: "sekiro",
		Offsets32:   nil,
		Offsets64:   []int64{0x3D5AAC0, 0x90},
	},
	{
		Name:        "Elden Ring",
		ProcessName: "eldenring",
		Offsets32:   nil,
		Offsets64:   []int64{0x3D5DF38, 0x94},
	},
}

// GameReader handles reading memory from FromSoftware games
type GameReader struct {
	processHandle syscall.Handle
	baseAddress   uintptr
	game          *GameConfig
	is64Bit       bool
	attached      bool
	currentGame   string
}

// NewGameReader creates a new memory reader for FromSoftware games
func NewGameReader() (*GameReader, error) {
	reader := &GameReader{}

	if err := reader.Attach(); err != nil {
		return reader, err // Return reader anyway, will retry connection
	}

	return reader, nil
}

// Attach finds and attaches to any supported FromSoftware game process
func (r *GameReader) Attach() error {
	for _, game := range supportedGames {
		pid, err := findProcessByName(game.ProcessName + ".exe")
		if err != nil {
			continue // Try next game
		}

		handle, err := openProcess(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION, false, pid)
		if err != nil {
			continue
		}

		// Get base address
		baseAddr, err := getModuleBaseAddress(pid, game.ProcessName+".exe")
		if err != nil {
			syscall.CloseHandle(handle)
			continue
		}

		// Check if process is 64-bit
		is64Bit, err := isProcess64Bit(handle)
		if err != nil {
			syscall.CloseHandle(handle)
			continue
		}

		// Verify we have offsets for this architecture
		if is64Bit && game.Offsets64 == nil {
			syscall.CloseHandle(handle)
			continue
		}
		if !is64Bit && game.Offsets32 == nil {
			syscall.CloseHandle(handle)
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

// Detach closes the process handle
func (r *GameReader) Detach() {
	if r.processHandle != 0 {
		syscall.CloseHandle(r.processHandle)
		r.processHandle = 0
		r.attached = false
		r.currentGame = ""
	}
}

// IsAttached returns whether the reader is attached to a process
func (r *GameReader) IsAttached() bool {
	return r.attached
}

// GetCurrentGame returns the name of the currently attached game
func (r *GameReader) GetCurrentGame() string {
	return r.currentGame
}

// ReadDeathCount reads the death count from memory using pointer traversal
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
		err := readProcessMemory(r.processHandle, uintptr(address), buffer)
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

// findProcessByName finds a process ID by executable name
func findProcessByName(name string) (uint32, error) {
	snapshot, _, err := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer procCloseHandle.Call(snapshot)

	var pe PROCESSENTRY32
	pe.Size = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return 0, fmt.Errorf("Process32First failed")
	}

	for {
		exeName := syscall.UTF16ToString(pe.ExeFile[:])
		if exeName == name {
			return pe.ProcessID, nil
		}

		ret, _, _ := procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&pe)))
		if ret == 0 {
			break
		}
	}

	return 0, fmt.Errorf("process not found: %s", name)
}

// getModuleBaseAddress gets the base address of a module in a process
func getModuleBaseAddress(pid uint32, moduleName string) (uintptr, error) {
	snapshot, _, err := procCreateToolhelp32Snapshot.Call(
		TH32CS_SNAPMODULE|TH32CS_SNAPMODULE32,
		uintptr(pid),
	)
	if snapshot == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer procCloseHandle.Call(snapshot)

	var me MODULEENTRY32
	me.Size = uint32(unsafe.Sizeof(me))

	ret, _, _ := procModule32First.Call(snapshot, uintptr(unsafe.Pointer(&me)))
	if ret == 0 {
		return 0, fmt.Errorf("Module32First failed")
	}

	for {
		modName := syscall.UTF16ToString(me.SzModule[:])
		if modName == moduleName {
			return me.ModBaseAddr, nil
		}

		ret, _, _ := procModule32Next.Call(snapshot, uintptr(unsafe.Pointer(&me)))
		if ret == 0 {
			break
		}
	}

	return 0, fmt.Errorf("module not found: %s", moduleName)
}

// isProcess64Bit checks if a process is 64-bit
func isProcess64Bit(handle syscall.Handle) (bool, error) {
	var isWow64 bool
	ret, _, err := procIsWow64Process.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&isWow64)),
	)

	if ret == 0 {
		return false, fmt.Errorf("IsWow64Process failed: %v", err)
	}

	// If IsWow64Process returns true, the process is 32-bit running on 64-bit Windows
	// If it returns false, the process is native to the system (64-bit on 64-bit Windows)
	return !isWow64, nil
}

// openProcess opens a process and returns a handle
func openProcess(access uint32, inheritHandle bool, pid uint32) (syscall.Handle, error) {
	inherit := 0
	if inheritHandle {
		inherit = 1
	}

	handle, _, err := procOpenProcess.Call(
		uintptr(access),
		uintptr(inherit),
		uintptr(pid),
	)

	if handle == 0 {
		return 0, fmt.Errorf("OpenProcess failed: %v", err)
	}

	return syscall.Handle(handle), nil
}

// readProcessMemory reads memory from the process
func readProcessMemory(handle syscall.Handle, address uintptr, buffer []byte) error {
	var numRead uintptr

	ret, _, err := procReadProcessMemory.Call(
		uintptr(handle),
		address,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		uintptr(unsafe.Pointer(&numRead)),
	)

	if ret == 0 {
		return fmt.Errorf("ReadProcessMemory failed: %v", err)
	}

	return nil
}

// GetSupportedGames returns a list of all supported games
func GetSupportedGames() []string {
	games := make([]string, len(supportedGames))
	for i, game := range supportedGames {
		games[i] = game.Name
	}
	return games
}
