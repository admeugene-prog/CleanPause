package ui

import (
	"cleanpause/assets"
	"cleanpause/internal/config"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

// Export native UI screenshots without starting workers, changing preferences,
// or interacting with a user's running application. Opt in with the directory.
func TestExportUIScreenshots(t *testing.T) {
	dir := os.Getenv("CLEANPAUSE_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set CLEANPAUSE_SCREENSHOT_DIR to export native UI captures")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previous, previousLanguage := current, assets.Language()
	defer func() { current = previous; assets.SetLanguage(previousLanguage) }()
	w := &window{scale: 1, simulate: true, cfg: config.Default(), controls: map[int]uintptr{}}
	current = w
	w.instance, _, _ = kernel.NewProc("GetModuleHandleW").Call(0)
	class := wc{Size: uint32(unsafe.Sizeof(wc{})), Proc: syscall.NewCallback(wndProc), Instance: w.instance, Cursor: call("LoadCursorW", 0, 32512), Class: ptr("CleanPauseScreenshotWindow")}
	if call("RegisterClassExW", uintptr(unsafe.Pointer(&class))) == 0 {
		t.Fatal("register screenshot window")
	}
	defer call("UnregisterClassW", class.Class, w.instance)
	w.h = call("CreateWindowExW", 0, class.Class, ptr("CleanPause"), 0x02C80000, 0, 0, 900, 550, 0, 0, w.instance, 0)
	if w.h == 0 {
		t.Fatal("create screenshot window")
	}
	w.font, w.titleFont, w.cardFont, w.timerFont = w.newFont(16, 400), w.newFont(26, 700), w.newFont(18, 400), w.newFont(72, 700)
	defer func() {
		call("DestroyWindow", w.h)
		for _, object := range []uintptr{w.font, w.titleFont, w.cardFont, w.timerFont, w.brush, w.artBitmap} {
			if object != 0 {
				gdi.NewProc("DeleteObject").Call(object)
			}
		}
	}()
	w.cfg.Theme = "light"
	w.cfg.Autostart = false
	w.cfg.Reminders.Schedule = config.Schedule{Type: "custom", Days: []int{1, 2, 3, 4, 5}, Time: "10:00"}
	w.countdown, w.remaining = 3, 60
	for _, language := range []string{"en", "ru"} {
		assets.SetLanguage(language)
		w.languageOverride, w.cfg.Language = language, language
		w.cards = nil
		for _, screen := range []string{"reminder", "prepare", "countdown", "cleaning", "settings", "schedule"} {
			w.show(screen)
			call("UpdateWindow", w.h)
			call("RedrawWindow", w.h, 0, 0, 0x185)
			if err := saveWindowCapture(w.h, filepath.Join(dir, language+"-"+screen+".png")); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func saveWindowCapture(h uintptr, path string) error {
	type rect struct{ L, T, R, B int32 }
	var r rect
	call("GetClientRect", h, uintptr(unsafe.Pointer(&r)))
	width, height := int(r.R), int(r.B)
	dc := call("GetDC", h)
	defer call("ReleaseDC", h, dc)
	mem, _, _ := gdi.NewProc("CreateCompatibleDC").Call(dc)
	defer gdi.NewProc("DeleteDC").Call(mem)
	info := bitmapInfo{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bitmap, _, _ := gdi.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return fmt.Errorf("CreateDIBSection failed")
	}
	defer gdi.NewProc("DeleteObject").Call(bitmap)
	old, _, _ := gdi.NewProc("SelectObject").Call(mem, bitmap)
	if call("PrintWindow", h, mem, 1) == 0 {
		copied, _, _ := gdi.NewProc("BitBlt").Call(mem, 0, 0, uintptr(width), uintptr(height), dc, 0, 0, 0x00CC0020)
		if copied == 0 {
			return fmt.Errorf("PrintWindow and BitBlt failed")
		}
	}
	gdi.NewProc("SelectObject").Call(mem, old)
	gdi.NewProc("GdiFlush").Call()
	pixels := append([]byte(nil), unsafe.Slice((*byte)(bits), width*height*4)...)
	colors := map[uint32]bool{}
	for i := 0; i < len(pixels); i += 4 * 97 {
		colors[uint32(pixels[i])|uint32(pixels[i+1])<<8|uint32(pixels[i+2])<<16] = true
	}
	if len(colors) < 5 {
		return fmt.Errorf("empty screenshot: %d colors", len(colors))
	}
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2], pixels[i+3] = pixels[i+2], pixels[i], 255
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, &image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)})
}
