package platform

import (
	"reflect"
	"syscall"
	"testing"
	"unsafe"
)

func TestShellExecuteABI(t *testing.T) {
	if unsafe.Sizeof(shellExecuteInfo{}) != 112 || unsafe.Offsetof(shellExecuteInfo{}.Process) != 104 {
		t.Fatal("invalid SHELLEXECUTEINFOW ABI")
	}
}
func TestElevatedTokenQuery(t *testing.T) {
	if _, err := Elevated(); err != nil {
		t.Fatal(err)
	}
}
func TestArgumentsRoundTrip(t *testing.T) {
	args := []string{"--show-settings", "--config", `C:\My Folder\настройки.json`, `a"b`, "", `C:\folder with space\`}
	p, e := syscall.UTF16PtrFromString(parameters(args))
	if e != nil {
		t.Fatal(e)
	}
	var count int32
	argv, err := syscall.CommandLineToArgv(p, &count)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(argv))))
	got := []string{}
	for _, v := range argv[:count] {
		got = append(got, syscall.UTF16ToString(v[:]))
	}
	if !reflect.DeepEqual(args, got) {
		t.Fatal(got)
	}
}
