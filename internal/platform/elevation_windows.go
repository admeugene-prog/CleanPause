package platform

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func Elevated() (bool, error) {
	var token syscall.Handle
	process, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentProcess").Call()
	adv := syscall.NewLazyDLL("advapi32.dll")
	r, _, err := adv.NewProc("OpenProcessToken").Call(process, 8, uintptr(unsafe.Pointer(&token)))
	if r == 0 {
		return false, err
	}
	defer syscall.CloseHandle(token)
	var elevated, needed uint32
	r, _, err = adv.NewProc("GetTokenInformation").Call(uintptr(token), 20, uintptr(unsafe.Pointer(&elevated)), 4, uintptr(unsafe.Pointer(&needed)))
	if r == 0 {
		return false, err
	}
	return elevated != 0, nil
}

type shellExecuteInfo struct {
	Size, Mask                        uint32
	Window                            uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance, IDList                  uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	Icon, Process                     uintptr
}

func parameters(args []string) string {
	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = syscall.EscapeArg(a)
	}
	return strings.Join(escaped, " ")
}

// Re-exec the entire UI before acquiring its single-instance mutex. Its worker
// inherits the elevated token, so pipe inheritance and watchdog access still work.
// This function does not start a cleaning session and never bypasses UAC.
func RelaunchElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString(parameters(args))
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL("ole32.dll")
	hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 6)
	if int32(hr) < 0 {
		return fmt.Errorf("не удалось подготовить запрос прав: HRESULT 0x%x", hr)
	}
	defer ole.NewProc("CoUninitialize").Call()
	info := shellExecuteInfo{Size: uint32(unsafe.Sizeof(shellExecuteInfo{})), Mask: 0x140, Verb: verb, File: file, Parameters: params, Show: 1}
	r, _, err := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	runtime.KeepAlive(params)
	if r == 0 {
		if err == syscall.Errno(1223) {
			return fmt.Errorf("Запрос прав администратора отменён. Очистка не запускалась.")
		}
		return fmt.Errorf("Не удалось запустить CleanPause с правами администратора: %v", err)
	}
	if info.Process != 0 {
		syscall.CloseHandle(syscall.Handle(info.Process))
	}
	return nil
}
