package ui

import (
	"image"
	"testing"
	"unsafe"
)

func TestArtworkEmbeddedAndDecodable(t *testing.T) {
	a, err := loadArtwork()
	if err != nil {
		t.Fatal(err)
	}
	if a.Width < 480 || a.Height < 270 || len(a.Pixels) != a.Width*a.Height*4 {
		t.Fatal("missing or truncated countdown illustration")
	}
	if unsafe.Sizeof(bitmapInfo{}) != 40 {
		t.Fatal("invalid BITMAPINFOHEADER ABI")
	}
	t.Logf("embedded artwork: %dx%d", a.Width, a.Height)
}
func TestArtworkWindowsBitmapContainsImage(t *testing.T) {
	a, err := loadArtwork()
	if err != nil {
		t.Fatal(err)
	}
	w := window{art: a}
	bitmap, err := w.artworkBitmap(480, 270)
	if err != nil {
		t.Fatal(err)
	}
	defer gdi.NewProc("DeleteObject").Call(bitmap)
	dc, _, _ := gdi.NewProc("CreateCompatibleDC").Call(0)
	defer gdi.NewProc("DeleteDC").Call(dc)
	pixels := make([]byte, 480*270*4)
	info := bitmapInfo{Size: 40, Width: 480, Height: -270, Planes: 1, BitCount: 32}
	lines, _, err := gdi.NewProc("GetDIBits").Call(dc, bitmap, 0, 270, uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), 0)
	if lines != 270 {
		t.Fatal("Windows bitmap not readable", lines, err)
	}
	colors := map[uint32]bool{}
	for i := 0; i < len(pixels); i += 4 * 97 {
		colors[uint32(pixels[i])|uint32(pixels[i+1])<<8|uint32(pixels[i+2])<<16] = true
	}
	if len(colors) < 20 {
		t.Fatal("Windows bitmap is empty or a solid fill", len(colors))
	}
}
func TestArtworkFitsWithoutStretching(t *testing.T) {
	for _, c := range []struct {
		width, height, imWidth, imHeight int
		want                             image.Rectangle
	}{{500, 400, 16, 9, image.Rect(0, 59, 500, 340)}, {400, 500, 9, 16, image.Rect(59, 0, 340, 500)}, {480, 270, 16, 9, image.Rect(0, 0, 480, 270)}} {
		if got := containArtwork(c.width, c.height, c.imWidth, c.imHeight); got != c.want {
			t.Fatalf("image distorted or clipped: %v, want %v", got, c.want)
		}
	}
}
