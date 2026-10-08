package ui

import (
	"bytes"
	"cleanpause/assets"
	"fmt"
	"image"
	"image/png"
	"unsafe"
)

type artwork struct {
	Width, Height int
	Pixels        []byte
}
type bitmapInfo struct {
	Size                        uint32
	Width, Height               int32
	Planes, BitCount            uint16
	Compression, ImageSize      uint32
	X, Y                        int32
	ColorsUsed, ColorsImportant uint32
}

func loadArtwork() (*artwork, error) {
	return decodeArtwork(assets.CountdownArtwork)
}
func decodeArtwork(data []byte) (*artwork, error) {
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("иллюстрация отсчёта: %w", err)
	}
	bounds := im.Bounds()
	a := &artwork{Width: bounds.Dx(), Height: bounds.Dy()}
	a.Pixels = make([]byte, a.Width*a.Height*4)
	for y := 0; y < a.Height; y++ {
		for x := 0; x < a.Width; x++ {
			r, g, b, _ := im.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			i := (y*a.Width + x) * 4
			a.Pixels[i], a.Pixels[i+1], a.Pixels[i+2], a.Pixels[i+3] = byte(b>>8), byte(g>>8), byte(r>>8), 255
		}
	}
	return a, nil
}
func containArtwork(width, height, imageWidth, imageHeight int) image.Rectangle {
	w, h := width, width*imageHeight/imageWidth
	if h > height {
		h = height
		w = height * imageWidth / imageHeight
	}
	return image.Rect((width-w)/2, (height-h)/2, (width-w)/2+w, (height-h)/2+h)
}

// SS_BITMAP uses Windows' own painting rather than parent owner-draw callbacks.
func (w *window) artworkBitmap(width, height int) (uintptr, error) {
	return nativeArtworkBitmap(w.art, width, height)
}
func nativeArtworkBitmap(art *artwork, width, height int, background ...uint32) (uintptr, error) {
	info := bitmapInfo{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bitmap, _, err := gdi.NewProc("CreateDIBSection").Call(0, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return 0, fmt.Errorf("CreateDIBSection: %v", err)
	}
	if bits == nil {
		gdi.NewProc("DeleteObject").Call(bitmap)
		return 0, fmt.Errorf("CreateDIBSection returned no pixel buffer")
	}
	destination := unsafe.Slice((*byte)(bits), width*height*4)
	bg := uint32(0xffffff)
	if len(background) > 0 {
		bg = background[0]
	}
	for i := 0; i < len(destination); i += 4 {
		destination[i], destination[i+1], destination[i+2], destination[i+3] = byte(bg>>16), byte(bg>>8), byte(bg), 255
	}
	r := containArtwork(width, height, art.Width, art.Height)
	// Fill the bitmap's pixel memory directly. This avoids StretchDIBits failures
	// on display/compatible DCs while retaining Windows' standard bitmap painting.
	for y := 0; y < r.Dy(); y++ {
		sy := y * art.Height / r.Dy()
		for x := 0; x < r.Dx(); x++ {
			sx := x * art.Width / r.Dx()
			source := (sy*art.Width + sx) * 4
			target := ((y+r.Min.Y)*width + x + r.Min.X) * 4
			copy(destination[target:target+4], art.Pixels[source:source+4])
		}
	}
	return bitmap, nil
}
