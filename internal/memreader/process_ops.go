package memreader

// ProcessHandle is a platform-independent handle to a process.
type ProcessHandle uintptr

const (
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
)

// ProcessOps abstracts OS-level process operations for testing.
type ProcessOps interface {
	FindProcessByName(name string) (uint32, error)
	GetModuleBaseAddress(pid uint32, moduleName string) (uintptr, error)
	IsProcess64Bit(handle ProcessHandle) (bool, error)
	OpenProcess(access uint32, inheritHandle bool, pid uint32) (ProcessHandle, error)
	ReadProcessMemory(handle ProcessHandle, address uintptr, buffer []byte) error
	CloseHandle(handle ProcessHandle) error
}
