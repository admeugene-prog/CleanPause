package config

import (
	"syscall"
	"unsafe"
)

func replace(from, to string) error {
	a, _ := syscall.UTF16PtrFromString(from)
	b, _ := syscall.UTF16PtrFromString(to)
	r, _, e := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 9)
	if r == 0 {
		return e
	}
	return nil
}
