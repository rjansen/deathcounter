package tray

import (
	"bytes"
	"image"
	"image/png"
)

// loadIconImage extracts the PNG from the embedded ICO data and returns
// a stdlib image.Image. The platform implementation converts this to
// its native icon format.
func loadIconImage() (image.Image, error) {
	pngBytes := iconData[iconPNGOffset:]
	return png.Decode(bytes.NewReader(pngBytes))
}
