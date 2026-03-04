//go:build windows

package memreader

// NewGameReader creates a new memory reader for FromSoftware games.
func NewGameReader() (*GameReader, error) {
	reader := &GameReader{
		ops: &windowsProcessOps{},
	}

	if err := reader.Attach(); err != nil {
		return reader, err // Return reader anyway, will retry connection
	}

	return reader, nil
}
