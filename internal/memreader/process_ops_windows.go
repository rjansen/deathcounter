//go:build windows

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
	thSnapProcess  = 0x00000002
	thSnapModule   = 0x00000008
	thSnapModule32 = 0x00000010
)

// processEntry32 represents a process in the snapshot.
type processEntry32 struct {
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

// moduleEntry32 represents a module in the snapshot.
type moduleEntry32 struct {
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

// windowsProcessOps implements ProcessOps using Windows API calls.
type windowsProcessOps struct{}

func (w *windowsProcessOps) FindProcessByName(name string) (uint32, error) {
	snapshot, _, err := procCreateToolhelp32Snapshot.Call(thSnapProcess, 0)
	if snapshot == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer procCloseHandle.Call(snapshot)

	var pe processEntry32
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

func (w *windowsProcessOps) GetModuleBaseAddress(pid uint32, moduleName string) (uintptr, error) {
	snapshot, _, err := procCreateToolhelp32Snapshot.Call(
		thSnapModule|thSnapModule32,
		uintptr(pid),
	)
	if snapshot == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", err)
	}
	defer procCloseHandle.Call(snapshot)

	var me moduleEntry32
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

func (w *windowsProcessOps) IsProcess64Bit(handle ProcessHandle) (bool, error) {
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

func (w *windowsProcessOps) OpenProcess(access uint32, inheritHandle bool, pid uint32) (ProcessHandle, error) {
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

	return ProcessHandle(handle), nil
}

func (w *windowsProcessOps) ReadProcessMemory(handle ProcessHandle, address uintptr, buffer []byte) error {
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

func (w *windowsProcessOps) CloseHandle(handle ProcessHandle) error {
	return syscall.CloseHandle(syscall.Handle(handle))
}
