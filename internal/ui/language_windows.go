package ui

import (
	"cleanpause/assets"
	"cleanpause/internal/app"
	"fmt"
	"log/slog"
	"syscall"
	"unsafe"
)

const languageRegistryKey = `Software\CleanPause`

func languageForWindows(id uint16) string {
	if id&0x3ff == 0x19 {
		return "ru"
	}
	return "en"
}
func windowsLanguage() string {
	id, _, _ := kernel.NewProc("GetUserDefaultUILanguage").Call()
	return languageForWindows(uint16(id))
}
func readLanguageOverride(path string) (string, error) {
	var key syscall.Handle
	err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, syscall.StringToUTF16Ptr(path), 0, syscall.KEY_QUERY_VALUE, &key)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer syscall.RegCloseKey(key)
	var value [16]uint16
	kind, size := uint32(0), uint32(unsafe.Sizeof(value))
	err = syscall.RegQueryValueEx(key, syscall.StringToUTF16Ptr("Language"), nil, &kind, (*byte)(unsafe.Pointer(&value[0])), &size)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	code := syscall.UTF16ToString(value[:])
	if kind == syscall.REG_SZ && (code == "ru" || code == "en") {
		return code, nil
	}
	return "", fmt.Errorf("invalid language override")
}
func writeLanguageOverride(path, code string) error {
	if code != "" && code != "ru" && code != "en" {
		return fmt.Errorf("unsupported language: %s", code)
	}
	advapi := syscall.NewLazyDLL("advapi32.dll")
	var key syscall.Handle
	result, _, _ := advapi.NewProc("RegCreateKeyExW").Call(uintptr(syscall.HKEY_CURRENT_USER), ptr(path), 0, 0, 0, syscall.KEY_SET_VALUE, 0, uintptr(unsafe.Pointer(&key)), 0)
	if result != 0 {
		return syscall.Errno(result)
	}
	defer syscall.RegCloseKey(key)
	if code == "" {
		result, _, _ = advapi.NewProc("RegDeleteValueW").Call(uintptr(key), ptr("Language"))
		if result == 2 {
			return nil
		}
	} else {
		value, _ := syscall.UTF16FromString(code)
		result, _, _ = advapi.NewProc("RegSetValueExW").Call(uintptr(key), ptr("Language"), 0, syscall.REG_SZ, uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)*2))
	}
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
}
func (w *window) initializeLanguage() {
	code, err := readLanguageOverride(languageRegistryKey)
	if err != nil {
		slog.Warn("language registry read", "error", err)
	}
	w.languageOverride = code
	if code == "" {
		code = windowsLanguage()
	}
	w.cfg.Language = code
	assets.SetLanguage(code)
}
func (w *window) selectLanguage(code string) {
	if err := writeLanguageOverride(languageRegistryKey, code); err != nil {
		ShowError(assets.Translate("Не удалось сохранить язык в реестре") + ": " + err.Error())
		return
	}
	w.languageOverride = code
	if code == "" {
		code = windowsLanguage()
	}
	w.cfg.Language = code
	assets.SetLanguage(code)
	w.cards = nil
	w.cardIndex = 0
	w.saveQuiet()
	w.tray(1, "")
	if w.machine.State() == app.Preparing {
		w.cancelPreparation()
		w.hide()
	} else if w.visible {
		w.show(w.screen)
	}
}
