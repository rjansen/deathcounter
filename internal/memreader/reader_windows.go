//go:build windows

package memreader

// NewProcessOps creates platform-specific process operations for Windows.
func NewProcessOps() ProcessOps {
	return &windowsProcessOps{}
}
