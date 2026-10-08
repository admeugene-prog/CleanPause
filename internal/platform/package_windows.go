package platform

import (
	"syscall"
	"unsafe"
)

// A package owns its startup registration. Never write a version-specific
// WindowsApps executable path to the ordinary Run registry entry.
var IsPackaged = func() bool {
	var length uint32
	r, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentPackageFullName").Call(
		uintptr(unsafe.Pointer(&length)), 0)
	return r == 122 // ERROR_INSUFFICIENT_BUFFER; unpackaged returns APPMODEL_ERROR_NO_PACKAGE.
}()
