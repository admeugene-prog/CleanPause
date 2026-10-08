package input

import (
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const desktopSwitchEvent = 0x20

var user32 = syscall.NewLazyDLL("user32.dll")

type desktopWatch struct {
	hook     uintptr
	switched atomic.Bool
}
type desktopMessage struct {
	Window  uintptr
	ID      uint32
	W, L    uintptr
	Time    uint32
	X, Y    int32
	Private uint32
}

// Called on the worker's locked OS thread. The callback latches even a rapid
// switch away and back; polling the desktop name alone could miss that sequence.
func newDesktopWatch() (*desktopWatch, error) {
	w := &desktopWatch{}
	callback := syscall.NewCallback(func(h uintptr, event uint32, window uintptr, object, child int32, thread, stamp uint32) uintptr {
		w.recordEvent(event)
		return 0
	})
	w.hook, _, _ = user32.NewProc("SetWinEventHook").Call(desktopSwitchEvent, desktopSwitchEvent, 0, callback, 0, 0, 0)
	if w.hook == 0 {
		return nil, fmt.Errorf("Не удалось включить наблюдение за аварийным выходом; очистка не началась")
	}
	return w, nil
}
func (w *desktopWatch) recordEvent(event uint32) {
	if event == desktopSwitchEvent {
		w.switched.Store(true)
	}
}
func (w *desktopWatch) Close() {
	if w.hook != 0 {
		user32.NewProc("UnhookWinEvent").Call(w.hook)
		w.hook = 0
	}
}
func (w *desktopWatch) Interrupted() bool {
	// Out-of-context WinEvents require a message pump on their registering thread.
	var msg desktopMessage
	for i := 0; i < 64; i++ {
		r, _, _ := user32.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
		if r == 0 {
			break
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
	if w.switched.Load() {
		return true
	}
	// Also stop safely if the active input desktop became inaccessible.
	desktop, _, _ := user32.NewProc("OpenInputDesktop").Call(0, 0, 1)
	if desktop == 0 {
		return true
	}
	defer user32.NewProc("CloseDesktop").Call(desktop)
	var name [128]uint16
	var size uint32
	r, _, _ := user32.NewProc("GetUserObjectInformationW").Call(desktop, 2, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Sizeof(name)), uintptr(unsafe.Pointer(&size)))
	return r == 0 || syscall.UTF16ToString(name[:]) != "Default"
}
