//go:build windows

package tray

import (
	"bytes"
	"image/png"

	"github.com/lxn/walk"
)

// loadIcon extracts the PNG from the embedded ICO data and creates a walk.Icon.
func loadIcon() (*walk.Icon, error) {
	pngBytes := iconData[iconPNGOffset:]
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}
	return walk.NewIconFromImageForDPI(img, 96)
}
