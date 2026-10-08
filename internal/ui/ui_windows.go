package ui

import (
	"cleanpause/assets"
	"cleanpause/internal/app"
	"cleanpause/internal/config"
	"cleanpause/internal/input"
	"cleanpause/internal/platform"
	"cleanpause/internal/scheduler"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var shell = syscall.NewLazyDLL("shell32.dll")
var gdi = syscall.NewLazyDLL("gdi32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")

func call(name string, args ...uintptr) uintptr {
	r, _, _ := user.NewProc(name).Call(args...)
	return r
}

var utf16Cache = map[string]*uint16{}

func ptr(s string) uintptr {
	p := utf16Cache[s]
	if p == nil {
		p, _ = syscall.UTF16PtrFromString(s)
		utf16Cache[s] = p
	}
	return uintptr(unsafe.Pointer(p))
}

type point struct{ X, Y int32 }
type message struct {
	H       uintptr
	ID      uint32
	W, L    uintptr
	Time    uint32
	P       point
	Private uint32
}
type wc struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Class                        uintptr
	SmallIcon                          uintptr
}
type guid struct {
	A    uint32
	B, C uint16
	D    [8]byte
}
type notify struct {
	Size                uint32
	H                   uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, Mask         uint32
	Info                [256]uint16
	Version             uint32
	Title               [64]uint16
	InfoFlags           uint32
	GUID                guid
	BalloonIcon         uintptr
}

func wide(dst []uint16, s string) { v, _ := syscall.UTF16FromString(s); copy(dst, v) }

var current *window

type window struct {
	h, instance, font, titleFont, timerFont, icon, activeIcon, brush uintptr
	scale                                                            float64
	cfg                                                              config.Config
	path, screen                                                     string
	controls                                                         map[int]uintptr
	children                                                         []uintptr
	machine                                                          app.Machine
	session                                                          *input.Session
	cancelledSession                                                 bool
	simulate, visible, quitting                                      bool
	countdown                                                        int
	preparingUntil                                                   time.Time
	remaining                                                        int
	start, end                                                       time.Time
	taskbar                                                          uint32
	log                                                              *os.File
	art                                                              *artwork
	artBitmap                                                        uintptr
	banner                                                           *artwork
	cards                                                            []instructionCard
	cardIndex                                                        int
	cardChanged                                                      time.Time
	cardFont                                                         uintptr
	languageOverride                                                 string
}

func ShowError(s string) { call("MessageBoxW", 0, ptr(s), ptr("CleanPause"), 0x10) }
func Run(simulate bool, initial, configPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name, _ := syscall.UTF16PtrFromString("Local\\CleanPause.UI.v1")
	mutex, _, err := kernel.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if mutex == 0 {
		return fmt.Errorf("mutex: %v", err)
	}
	defer syscall.CloseHandle(syscall.Handle(mutex))
	if err == syscall.Errno(183) {
		return nil
	}
	// Per-monitor awareness is available on modern Windows; system awareness is a fallback.
	if p := user.NewProc("SetProcessDpiAwarenessContext"); p.Find() == nil {
		p.Call(^uintptr(3))
	} else {
		call("SetProcessDPIAware")
	}
	w := &window{simulate: simulate, path: config.Path(), scale: 1, controls: map[int]uintptr{}}
	if configPath != "" {
		w.path = configPath
	}
	current = w
	os.MkdirAll(filepath.Dir(w.path), 0700)
	w.log, _ = os.OpenFile(filepath.Join(filepath.Dir(w.path), "cleanpause.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if w.log != nil {
		defer w.log.Close()
		slog.SetDefault(slog.New(slog.NewJSONHandler(w.log, nil)))
	}
	c, loadErr := config.Load(w.path)
	slog.Info("startup", "initial", initial, "config", w.path)
	w.cfg = c
	w.initializeLanguage()
	if loadErr != nil {
		slog.Error("config recovery", "error", loadErr)
		if c.Version == 1 {
			_ = os.Rename(w.path, w.path+".invalid-"+time.Now().Format("20060102-150405"))
		}
	}
	w.instance, _, _ = kernel.NewProc("GetModuleHandleW").Call(0)
	w.icon = makeIcon(false)
	w.activeIcon = makeIcon(true)
	klass := wc{Size: uint32(unsafe.Sizeof(wc{})), Proc: syscall.NewCallback(wndProc), Instance: w.instance, Icon: w.icon, SmallIcon: w.icon, Cursor: call("LoadCursorW", 0, 32512), Background: 6, Class: ptr("CleanPauseWindow")}
	if call("RegisterClassExW", uintptr(unsafe.Pointer(&klass))) == 0 {
		return fmt.Errorf("не удалось зарегистрировать окно")
	}
	w.h = call("CreateWindowExW", 0, klass.Class, ptr("CleanPause"), 0x02C80000, 0x80000000, 0x80000000, 600, 540, 0, 0, w.instance, 0)
	slog.Info("window created", "hwnd", w.h)
	if w.h == 0 {
		return fmt.Errorf("не удалось создать окно")
	}
	if p := user.NewProc("GetDpiForWindow"); p.Find() == nil {
		dpi, _, _ := p.Call(w.h)
		if dpi > 0 {
			w.scale = float64(dpi) / 96
		}
	}
	w.font = w.newFont(16, 400)
	w.titleFont = w.newFont(26, 700)
	w.cardFont = w.newFont(18, 400)
	w.timerFont = w.newFont(72, 700)
	defer func() {
		w.tray(2, "")
		call("DestroyWindow", w.h)
		if w.artBitmap != 0 {
			gdi.NewProc("DeleteObject").Call(w.artBitmap)
		}
		for _, f := range []uintptr{w.font, w.titleFont, w.timerFont, w.cardFont, w.brush} {
			if f != 0 {
				gdi.NewProc("DeleteObject").Call(f)
			}
		}
		call("DestroyIcon", w.icon)
		call("DestroyIcon", w.activeIcon)
	}()
	w.taskbar = uint32(call("RegisterWindowMessageW", ptr("TaskbarCreated")))
	w.tray(0, "")
	call("SetTimer", w.h, 1, 1000, 0)
	// Refresh the executable path after installation or moving a portable copy.
	if configPath == "" {
		if e := w.setAutostart(w.cfg.Autostart); e != nil {
			slog.Error("autostart refresh", "error", e)
		}
	}
	due, changed := scheduler.Poll(&w.cfg.Reminders, scheduler.Now())
	if changed {
		w.saveQuiet()
	}
	if due {
		w.remind()
	}
	if _, e := os.Stat(w.path); os.IsNotExist(e) {
		if e = w.setAutostart(w.cfg.Autostart); e != nil {
			slog.Error("autostart", "error", e)
			w.cfg.Autostart = false
		}
		w.saveQuiet()
	}
	if loadErr != nil {
		w.balloon("Настройки восстановлены", "Повреждённая конфигурация сохранена рядом с config.json.")
	}
	var msg message
	if initial != "" {
		w.show(initial)
	}
	for {
		r := call("GetMessageW", uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			return fmt.Errorf("ошибка цикла сообщений")
		}
		if r == 0 {
			break
		}
		if !w.visible || call("IsDialogMessageW", w.h, uintptr(unsafe.Pointer(&msg))) == 0 {
			call("TranslateMessage", uintptr(unsafe.Pointer(&msg)))
			call("DispatchMessageW", uintptr(unsafe.Pointer(&msg)))
		}
	}
	return nil
}
func (w *window) newFont(size, weight int) uintptr {
	r, _, _ := gdi.NewProc("CreateFontW").Call(uintptr(int32(-w.px(size))), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, ptr("Segoe UI"))
	return r
}
func (w *window) px(v int) int { return int(float64(v) * w.scale) }
func makeIcon(active bool) uintptr {
	and := make([]byte, 128)
	xor := make([]byte, 32*32*4)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			i := (y*32 + x) * 4
			b, g, r := byte(145), byte(175), byte(9)
			if active {
				b, g, r = 60, 145, 245
			}
			if x >= 6 && x <= 25 && y >= 10 && y <= 22 {
				b, g, r = 245, 255, 245
				if x > 7 && x < 24 && y > 11 && y < 21 {
					b, g, r = 100, 140, 0
					if active {
						b, g, r = 45, 90, 160
					}
					if (y == 14 || y == 17) && x%4 == 0 {
						b, g, r = 245, 255, 245
					}
				}
			}
			xor[i], xor[i+1], xor[i+2], xor[i+3] = b, g, r, 255
		}
	}
	return call("CreateIcon", 0, 32, 32, 1, 32, uintptr(unsafe.Pointer(&and[0])), uintptr(unsafe.Pointer(&xor[0])))
}
func (w *window) tray(action uint32, info string) {
	n := notify{Size: uint32(unsafe.Sizeof(notify{})), H: w.h, ID: 1, Flags: 7, Callback: 0x8001, Icon: w.icon}
	if w.machine.State() == app.Cleaning {
		n.Icon = w.activeIcon
	}
	wide(n.Tip[:], assets.Translate("CleanPause — защита клавиатуры и мыши"))
	shell.NewProc("Shell_NotifyIconW").Call(uintptr(action), uintptr(unsafe.Pointer(&n)))
}
func (w *window) balloon(title, body string) {
	n := notify{Size: uint32(unsafe.Sizeof(notify{})), H: w.h, ID: 1, Flags: 0x10, InfoFlags: 1}
	wide(n.Title[:], assets.Translate(title))
	wide(n.Info[:], assets.Translate(body))
	shell.NewProc("Shell_NotifyIconW").Call(1, uintptr(unsafe.Pointer(&n)))
}
func wndProc(h uintptr, msg uint32, wp uintptr, data unsafe.Pointer) uintptr {
	lp := uintptr(data)
	w := current
	if w == nil {
		return call("DefWindowProcW", h, uintptr(msg), wp, lp)
	}
	if msg == w.taskbar && w.taskbar != 0 {
		w.tray(0, "")
		return 0
	}
	switch msg {
	case 0x8001:
		switch uint32(lp) {
		case 0x203:
			w.prepare()
		case 0x205, 0x7b:
			w.menu()
		case 0x405:
			if w.machine.State() == app.Reminder {
				w.show("reminder")
			}
		}
		return 0
	case 0x111:
		// Menu commands and BN_CLICKED have notification code zero. Focus and
		// selection notifications must not start or cancel a cleaning session.
		if wp>>16 == 0 {
			w.command(int(wp & 0xffff))
		}
		return 0
	case 0x2b:
		if w.drawInstruction(data) {
			return 1
		}
		if w.drawButton(data) {
			return 1
		}
	case 0x113:
		w.onTick()
		return 0
	case 0x10:
		w.close()
		return 0
	case 0x2e0:
		w.scale = float64(wp&0xffff) / 96
		for _, f := range []uintptr{w.font, w.titleFont, w.timerFont, w.cardFont} {
			gdi.NewProc("DeleteObject").Call(f)
		}
		w.font = w.newFont(16, 400)
		w.titleFont = w.newFont(26, 700)
		w.cardFont = w.newFont(18, 400)
		w.timerFont = w.newFont(72, 700)
		if w.visible {
			w.show(w.screen)
		}
		return 0
	case 0x312:
		if w.machine.State() != app.Cleaning {
			w.prepare()
		}
		return 0
	case 0x138, 0x133, 0x135:
		dark := w.dark()
		text, bg := uintptr(0x605C55), uintptr(0xFFFFFF)
		if dark {
			text, bg = 0xF5F2EB, 0x261C17
		}
		if lp == w.controls[101] || lp == w.controls[106] || lp == w.controls[107] {
			if dark {
				text = 0xB6AA9F
			} else {
				text = 0x7B756F
			}
		}
		gdi.NewProc("SetTextColor").Call(wp, text)
		gdi.NewProc("SetBkColor").Call(wp, bg)
		return w.brush
	case 0x14:
		type rect struct{ L, T, R, B int32 }
		var r rect
		call("GetClientRect", h, uintptr(unsafe.Pointer(&r)))
		call("FillRect", wp, uintptr(unsafe.Pointer(&r)), w.brush)
		return 1
	case 0x16:
		if w.session != nil {
			w.session.Stop()
		}
		return 1
	case 0x11:
		if w.session != nil {
			w.session.Stop()
		}
		return 1
	}
	return call("DefWindowProcW", h, uintptr(msg), wp, lp)
}
func (w *window) dark() bool {
	if w.cfg.Theme == "dark" {
		return true
	}
	if w.cfg.Theme == "light" {
		return false
	}
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, syscall.StringToUTF16Ptr("Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize"), 0, syscall.KEY_READ, &key) != nil {
		return false
	}
	defer syscall.RegCloseKey(key)
	var val, kind, size uint32
	size = 4
	if syscall.RegQueryValueEx(key, syscall.StringToUTF16Ptr("AppsUseLightTheme"), nil, &kind, (*byte)(unsafe.Pointer(&val)), &size) != nil {
		return false
	}
	return val == 0
}
func (w *window) add(class, text string, id, x, y, width, height int, style uintptr) uintptr {
	text = assets.Translate(text)
	// Content coordinates omit the former 60 px brand header.
	h := call("CreateWindowExW", 0, ptr(class), ptr(text), 0x50000000|style, uintptr(w.px(x)), uintptr(w.px(y-60)), uintptr(w.px(width)), uintptr(w.px(height)), w.h, uintptr(id), w.instance, 0)
	call("SendMessageW", h, 0x30, w.font, 1)
	w.children = append(w.children, h)
	w.controls[id] = h
	return h
}
func (w *window) label(text string, id, x, y, width, height int, font uintptr) {
	h := w.add("STATIC", text, id, x, y, width, height, 0)
	if font != 0 {
		call("SendMessageW", h, 0x30, font, 1)
	}
}
func (w *window) button(text string, id, x, y, width int) {
	style := uintptr(0x10000)
	if id == 1 || id == 3 || id == 7 || id == 8 || id == 60 || id == 61 {
		style |= 0xb
	}
	w.add("BUTTON", text, id, x, y, width, 38, style)
}
func (w *window) drawButton(data unsafe.Pointer) bool {
	type rect struct{ L, T, R, B int32 }
	type drawItem struct {
		Kind, ID, ItemID, Action, State uint32
		H, DC                           uintptr
		R                               rect
		Data                            uintptr
	}
	d := (*drawItem)(data)
	if d.Kind != 4 {
		return false
	}
	color := uintptr(0x8AAA08)
	if d.ID == 60 {
		color = 0x75685C
	}
	if d.State&1 != 0 {
		color = 0x699006
	}
	brush, _, _ := gdi.NewProc("CreateSolidBrush").Call(color)
	call("FillRect", d.DC, uintptr(unsafe.Pointer(&d.R)), brush)
	gdi.NewProc("DeleteObject").Call(brush)
	gdi.NewProc("SetBkMode").Call(d.DC, 1)
	gdi.NewProc("SetTextColor").Call(d.DC, 0xffffff)
	old, _, _ := gdi.NewProc("SelectObject").Call(d.DC, w.font)
	call("DrawTextW", d.DC, ptr(w.text(int(d.ID))), ^uintptr(0), uintptr(unsafe.Pointer(&d.R)), 0x25)
	gdi.NewProc("SelectObject").Call(d.DC, old)
	if d.State&0x10 != 0 {
		r := d.R
		r.L += 4
		r.T += 4
		r.R -= 4
		r.B -= 4
		call("DrawFocusRect", d.DC, uintptr(unsafe.Pointer(&r)))
	}
	return true
}
func (w *window) check(text string, id, x, y, width int, on bool) {
	h := w.add("BUTTON", text, id, x, y, width, 30, 0x10003)
	if on {
		call("SendMessageW", h, 0xf1, 1, 0)
	}
}
func (w *window) combo(id, x, y, width int, items []string, selected int) {
	h := w.add("COMBOBOX", "", id, x, y, width, 240, 0x10000|0x200000|3)
	for _, s := range items {
		call("SendMessageW", h, 0x143, 0, ptr(assets.Translate(s)))
	}
	call("SendMessageW", h, 0x14e, uintptr(selected), 0)
}
func (w *window) selected(id int) int { return int(call("SendMessageW", w.controls[id], 0x147, 0, 0)) }
func (w *window) checked(id int) bool { return call("SendMessageW", w.controls[id], 0xf0, 0, 0) == 1 }
func (w *window) text(id int) string {
	b := make([]uint16, 256)
	call("GetWindowTextW", w.controls[id], uintptr(unsafe.Pointer(&b[0])), 256)
	return syscall.UTF16ToString(b)
}
func (w *window) show(screen string) {
	slog.Info("window show", "screen", screen, "hwnd", w.h)
	w.screen = screen
	for _, h := range w.children {
		call("DestroyWindow", h)
	}
	w.children = nil
	if w.artBitmap != 0 {
		gdi.NewProc("DeleteObject").Call(w.artBitmap)
		w.artBitmap = 0
	}
	w.controls = map[int]uintptr{}
	if w.brush != 0 {
		gdi.NewProc("DeleteObject").Call(w.brush)
	}
	bg := uintptr(0xFFFFFF)
	if w.dark() {
		bg = 0x261C17
	}
	w.brush, _, _ = gdi.NewProc("CreateSolidBrush").Call(bg)
	dark := int32(0)
	if w.dark() {
		dark = 1
	}
	syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute").Call(w.h, 20, uintptr(unsafe.Pointer(&dark)), 4)
	width, height := 580, 540
	if screen == "settings" {
		height = 580
	}
	if screen == "reminder" {
		width = 900
	}
	if screen == "prepare" || screen == "countdown" || screen == "cleaning" {
		width, height = 900, 610
	}
	w.centerWindow(width, height-60)
	switch screen {
	case "reminder":
		w.label(assets.Text("reminder_title"), 102, 30, 82, 840, 36, w.titleFont)
		w.illustration()
		w.label("Протрите клавиатуру и мышь без случайных нажатий и кликов.", 103, 550, 132, 280, 80, 0)
		w.label(fmt.Sprintf(assets.Translate("Продолжительность: %d мин"), w.cfg.Cleaning.Duration/60), 104, 550, 234, 280, 28, 0)
		w.button(assets.Text("start"), 1, 550, 278, 280)
		w.label(assets.Text("later"), 105, 550, 346, 280, 24, 0)
		w.combo(20, 550, 386, 280, []string{"Через 15 минут", "Через 1 час", "Через 2 часа", "Через 3 часа", "На следующий допустимый день"}, w.snoozeIndex())
		w.button(assets.Text("snooze"), 2, 550, 436, 280)
	case "prepare":
		w.label(assets.Text("prepare_title"), 102, 30, 82, 840, 36, w.titleFont)
		w.illustration()
		w.label("На время очистки клавиатура и мышь перестанут реагировать.", 103, 550, 132, 280, 64, 0)
		w.label("Сохраните работу перед началом.", 106, 550, 204, 280, 44, 0)
		w.label("Подготовка", 105, 550, 252, 150, 24, 0)
		w.label("3 сек", 109, 735, 252, 95, 24, 0)
		w.label("Длительность", 111, 550, 282, 150, 24, 0)
		w.label(fmt.Sprintf(assets.Translate("%d мин"), w.cfg.Cleaning.Duration/60), 112, 735, 282, 95, 24, 0)
		if w.simulate {
			call("SetWindowTextW", w.controls[106], ptr(assets.Translate("Режим проверки: ввод остаётся доступен.")))
		}
		w.button("Начать очистку", 3, 550, 316, 176)
		w.button(assets.Text("cancel"), 4, 740, 316, 90)
		w.emergencyFooter()
	case "countdown":
		w.label("Приготовьтесь к очистке", 102, 30, 82, 840, 36, w.titleFont)
		w.illustration()
		w.label("До начала очистки", 103, 550, 132, 280, 30, 0)
		w.label(strconv.Itoa(w.countdown), 110, 605, 164, 160, 110, w.timerFont)
		w.label("После отсчёта ввод будет временно заблокирован.", 106, 550, 278, 280, 44, 0)
		w.button(assets.Text("cancel"), 4, 550, 330, 280)
		w.emergencyFooter()
	case "cleaning":
		w.label(assets.Text("ready_title"), 102, 30, 82, 840, 36, w.titleFont)
		w.illustration()
		w.label("До восстановления ввода", 105, 550, 132, 280, 30, 0)
		w.label(w.timerText(), 110, 550, 164, 280, 110, w.timerFont)
		w.label("Управление вернётся, когда таймер закончится.", 103, 550, 278, 280, 44, 0)
		if w.simulate {
			call("SetWindowTextW", w.controls[103], ptr(assets.Translate("Режим проверки: ввод не заблокирован.")))
		}
		w.button("Завершить сеанс", 4, 550, 330, 280)
		w.emergencyFooter()
	case "settings":
		w.button("Расписание", 5, 30, 108, 245)
		w.button("Настройки", 6, 295, 108, 245)
		if platform.IsPackaged {
			text := "Автозапуск: управление в настройках Windows"
			if assets.Language() == "en" {
				text = "Startup: managed in Windows Settings"
			}
			w.label(text, 30, 30, 175, 510, 30, 0)
		} else {
			w.check(assets.Text("autostart"), 30, 30, 175, 510, w.cfg.Autostart)
		}
		w.check(assets.Text("notify"), 31, 30, 215, 510, w.cfg.Notify)
		w.label(assets.Text("duration"), 102, 30, 267, 300, 26, 0)
		items := []string{}
		for i := 1; i <= 15; i++ {
			items = append(items, fmt.Sprintf(assets.Translate("%d мин"), i))
		}
		w.combo(32, 350, 260, 190, items, w.cfg.Cleaning.Duration/60-1)
		w.label(assets.Text("theme"), 103, 30, 320, 300, 26, 0)
		themes := map[string]int{"system": 0, "light": 1, "dark": 2}
		w.combo(33, 350, 313, 190, []string{"Системная", "Светлая", "Тёмная"}, themes[w.cfg.Theme])
		w.label("Язык", 105, 30, 365, 300, 26, 0)
		languageIndex := map[string]int{"": 0, "ru": 1, "en": 2}[w.languageOverride]
		w.combo(34, 350, 358, 190, []string{"Как в Windows", "Русский", "English"}, languageIndex)
		w.label("Подготовительный отсчёт: 3 секунды", 104, 30, 420, 510, 30, 0)
		w.button(assets.Text("save"), 7, 350, 460, 190)
	case "schedule":
		w.button("Расписание", 5, 30, 108, 245)
		w.button("Настройки", 6, 295, 108, 245)
		w.check(assets.Text("reminders"), 40, 30, 167, 510, w.cfg.Reminders.Enabled)
		w.label("Дни напоминаний", 104, 30, 212, 510, 28, 0)
		names := []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
		for i, n := range names {
			day := (i + 1) % 7
			w.check(n, 50+i, 30+i*73, 269, 72, scheduler.Allowed(w.cfg.Reminders.Schedule, time.Weekday(day)))
		}
		w.label("Время напоминания (ЧЧ:ММ)", 102, 30, 326, 330, 28, 0)
		w.add("EDIT", w.cfg.Reminders.Schedule.Time, 42, 370, 320, 170, 34, 0x10000|0x800000)
		next := "Следующее: расписание отключено"
		if w.cfg.Reminders.Enabled {
			n := scheduler.Next(w.cfg.Reminders.Schedule, scheduler.Now())
			if w.cfg.Reminders.NextDue != nil {
				n = *w.cfg.Reminders.NextDue
			}
			if w.cfg.Reminders.SnoozedUntil != nil {
				n = *w.cfg.Reminders.SnoozedUntil
			}
			next = assets.Translate("Следующее: ") + n.Local().Format("02.01.2006 15:04")
		}
		w.label(next, 103, 30, 377, 510, 28, 0)
		w.button(assets.Text("save"), 8, 350, 420, 190)
	case "about":
		w.label("CleanPause 1.0.0 · прототип MVP", 102, 30, 122, 510, 44, w.titleFont)
		w.label("Локальная утилита на Go для Windows 10/11 x64.\nЛицензия MIT. Разработчики CleanPause.\nБез сети и телеметрии.\n\nBlockInput требует испытаний аварийного восстановления\nна целевых системах перед распространением.", 103, 30, 185, 510, 190, 0)
		w.button("Закрыть", 4, 350, 420, 190)
	}
	if screen == "prepare" || screen == "countdown" || screen == "cleaning" {
		w.instructionCarousel()
	}
	w.visible = true
	call("ShowWindow", w.h, 5)
	// A launcher may provide SW_HIDE in STARTUPINFO; subsequent calls use our requested state.
	call("ShowWindow", w.h, 5)
	call("SetWindowPos", w.h, ^uintptr(0), 0, 0, 0, 0, 3)
	call("SetForegroundWindow", w.h)
	call("InvalidateRect", w.h, 0, 1)
	slog.Info("window visibility", "visible", call("IsWindowVisible", w.h))
	for _, id := range []int{1, 3, 40, 30, 4} {
		if h := w.controls[id]; h != 0 {
			call("SetFocus", h)
			break
		}
	}
}
func (w *window) illustration() {
	var selected *artwork
	if w.screen == "reminder" {
		if w.banner == nil {
			var err error
			w.banner, err = decodeArtwork(assets.ReminderArtwork)
			if err != nil {
				slog.Error("reminder banner load", "error", err)
			}
		}
		selected = w.banner
	} else {
		if w.art == nil {
			var err error
			w.art, err = loadArtwork()
			if err != nil {
				slog.Error("artwork load", "error", err)
			}
		}
		selected = w.art
	}
	if selected == nil {
		return
	}
	var err error
	height := 220
	if w.screen == "reminder" {
		height = 270
	}
	background := uint32(0xffffff)
	if w.dark() {
		background = 0x261c17
	}
	w.artBitmap, err = nativeArtworkBitmap(selected, w.px(480), w.px(height), background)
	if err != nil {
		slog.Error("artwork render", "error", err)
		return
	}
	h := w.add("STATIC", "", 120, 30, 132, 480, height, 0xe)
	call("SendMessageW", h, 0x172, 0, w.artBitmap)
	call("InvalidateRect", h, 0, 1)
}
func (w *window) menu() {
	m := call("CreatePopupMenu")
	defer call("DestroyMenu", m)
	for _, item := range []struct {
		id    int
		label string
	}{{1, assets.Text("start")}, {5, assets.Text("schedule")}, {6, assets.Text("settings")}, {9, assets.Text("about")}, {10, assets.Text("exit")}} {
		flags := uintptr(0)
		if w.machine.State() == app.Cleaning && !allowedDuringCleaning(item.id) {
			flags = 3
		}
		call("AppendMenuW", m, flags, uintptr(item.id), ptr(item.label))
	}
	var p point
	call("GetCursorPos", uintptr(unsafe.Pointer(&p)))
	call("SetForegroundWindow", w.h)
	id := call("TrackPopupMenu", m, 0x102, uintptr(p.X), uintptr(p.Y), 0, w.h, 0)
	if id != 0 {
		w.command(int(id))
	}
	call("PostMessageW", w.h, 0, 0, 0)
}
func (w *window) prepare() {
	if w.session != nil {
		return
	}
	if !w.machine.Move(app.Preparing) {
		if w.visible {
			call("SetForegroundWindow", w.h)
		}
		return
	}
	w.countdown = 0
	w.cardIndex = 0
	w.cardChanged = time.Now()
	w.show("prepare")
}
func (w *window) snoozeIndex() int {
	switch w.cfg.Reminders.SnoozeMinutes {
	case 60:
		return 1
	case 120:
		return 2
	case 180:
		return 3
	}
	return 0
}
func (w *window) snooze(index int) {
	now := scheduler.Now()
	var n time.Time
	if index == 4 {
		n = scheduler.NextDay(w.cfg.Reminders.Schedule, now)
	} else {
		minutes := []int{15, 60, 120, 180}
		if index < 0 || index > 3 {
			index = 0
		}
		n = now.Add(time.Duration(minutes[index]) * time.Minute)
		w.cfg.Reminders.SnoozeMinutes = minutes[index]
	}
	w.cfg.Reminders.SnoozedUntil = &n
	w.machine.Move(app.Snoozed)
	w.saveQuiet()
	w.hide()
}
func (w *window) hide() { w.visible = false; call("ShowWindow", w.h, 0) }
func (w *window) close() {
	switch w.machine.State() {
	case app.Cleaning:
		w.cancelCleaning()
		return
	case app.Reminder:
		w.snooze(w.selected(20))
		return
	case app.Preparing:
		w.cancelPreparation()
	}
	w.hide()
}
func (w *window) command(id int) {
	if w.machine.State() == app.Cleaning && !allowedDuringCleaning(id) {
		return
	}
	switch id {
	case 60:
		w.advanceCard(-1)
	case 61:
		w.advanceCard(1)
	case 1:
		fromReminder := w.machine.State() == app.Reminder
		w.prepare()
		if fromReminder {
			w.command(3)
		}
	case 2:
		w.snooze(w.selected(20))
	case 3:
		if w.machine.State() != app.Preparing || w.countdown > 0 || w.session != nil {
			return
		}
		w.countdown = 3
		w.preparingUntil = time.Now().Add(3 * time.Second)
		w.show("countdown")
	case 4:
		w.close()
	case 5, 6, 9:
		if w.machine.State() == app.Preparing {
			w.cancelPreparation()
		}
		if w.machine.State() == app.Reminder {
			w.snooze(w.snoozeIndex())
		}
		screen := map[int]string{5: "schedule", 6: "settings", 9: "about"}[id]
		w.show(screen)
	case 7:
		languageChoice := []string{"", "ru", "en"}[w.selected(34)]
		c := w.cfg
		c.Autostart = w.checked(30)
		c.Notify = w.checked(31)
		c.Cleaning.Duration = (w.selected(32) + 1) * 60
		c.Theme = []string{"system", "light", "dark"}[w.selected(33)]
		if w.commit(c) && languageChoice != w.languageOverride {
			w.selectLanguage(languageChoice)
		}
	case 8:
		c := w.cfg
		c.Reminders.Enabled = w.checked(40)
		c.Reminders.Schedule.Type = "custom"
		c.Reminders.Schedule.Time = w.text(42)
		c.Reminders.Schedule.Days = nil
		for i := 0; i < 7; i++ {
			if w.checked(50 + i) {
				c.Reminders.Schedule.Days = append(c.Reminders.Schedule.Days, (i+1)%7)
			}
		}
		scheduler.Reset(&c.Reminders, scheduler.Now())
		w.commit(c)
	case 10:
		if w.machine.State() == app.Cleaning {
			w.cancelCleaning()
		}
		if w.machine.State() == app.Preparing {
			w.cancelPreparation()
		}
		w.quitting = true
		call("PostQuitMessage", 0)
	}
}
func (w *window) commit(c config.Config) bool {
	if err := c.Validate(); err != nil {
		ShowError(err.Error())
		return false
	}
	if err := w.setAutostart(c.Autostart); err != nil {
		ShowError(err.Error())
		return false
	}
	if err := config.Save(w.path, c); err != nil {
		_ = w.setAutostart(w.cfg.Autostart)
		ShowError(err.Error())
		return false
	}
	w.cfg = c
	slog.Info("settings saved", "screen", w.screen, "reminder_time", c.Reminders.Schedule.Time, "next_due", c.Reminders.NextDue, "snoozed_until", c.Reminders.SnoozedUntil)
	w.show(w.screen)
	w.balloon("Настройки сохранены", "CleanPause продолжает работать в трее.")
	return true
}
func (w *window) saveQuiet() {
	if err := config.Save(w.path, w.cfg); err != nil {
		slog.Error("config save", "error", err)
		w.balloon("Не удалось сохранить настройки", err.Error())
	}
}
func (w *window) setAutostart(on bool) error {
	if platform.IsPackaged {
		return nil // MSIX StartupTask is controlled by Windows, not HKCU Run.
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	key := "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"
	var cmd *exec.Cmd
	if on {
		value := "\"" + exe + "\""
		if w.simulate {
			value += " --simulate-input"
		}
		cmd = exec.Command("reg.exe", "add", key, "/v", "CleanPause", "/t", "REG_SZ", "/d", value, "/f")
	} else {
		var k syscall.Handle
		if e := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, syscall.StringToUTF16Ptr("Software\\Microsoft\\Windows\\CurrentVersion\\Run"), 0, syscall.KEY_SET_VALUE, &k); e != nil {
			if e == syscall.ERROR_FILE_NOT_FOUND {
				return nil
			}
			return e
		}
		defer syscall.RegCloseKey(k)
		r, _, err := syscall.NewLazyDLL("advapi32.dll").NewProc("RegDeleteValueW").Call(uintptr(k), ptr("CleanPause"))
		if r == 0 || r == 2 {
			return nil
		}
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if b, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("автозапуск: %v (%s)", e, b)
	}
	return nil
}
func (w *window) remind() {
	if !w.machine.Move(app.Reminder) {
		return
	}
	w.show("reminder")
}
func (w *window) timerText() string { return fmt.Sprintf("%02d:%02d", w.remaining/60, w.remaining%60) }
func (w *window) onTick() {
	w.tickCarousel()
	if w.countdown > 0 {
		left := time.Until(w.preparingUntil)
		w.countdown = int((left + time.Second - 1) / time.Second)
		if w.countdown < 0 {
			w.countdown = 0
		}
		if w.countdown > 0 {
			call("SetWindowTextW", w.controls[110], ptr(strconv.Itoa(w.countdown)))
		} else {
			call("SetWindowTextW", w.controls[110], ptr("…"))
			s, err := input.Start(w.cfg.Cleaning.Duration, w.simulate)
			if err != nil {
				w.failure(err.Error())
			} else {
				w.session = s
				w.cancelledSession = false
				slog.Info("worker started", "session", s.ID, "pid_owner", os.Getpid(), "simulation", w.simulate)
			}
		}
	}
	if w.session != nil {
		for {
			select {
			case ev, ok := <-w.session.Events:
				if !ok {
					w.session = nil
					w.cancelledSession = false
					if w.machine.State() == app.Preparing || w.machine.State() == app.Cleaning {
						w.finishCleaning(true)
					}
					goto drained
				}
				if w.cancelledSession {
					continue
				}
				slog.Debug("worker event", "type", ev.Type)
				switch ev.Type {
				case "locked":
					if w.machine.Move(app.Cleaning) {
						w.start = time.Now()
						w.end = w.start.Add(time.Duration(w.cfg.Cleaning.Duration) * time.Second)
						w.remaining = ev.Remaining
						w.cfg.Reminders.SnoozedUntil = nil
						w.saveQuiet()
						w.show("cleaning")
						w.tray(1, "")
					} else {
						w.session.Stop()
					}
				case "heartbeat":
					w.remaining = ev.Remaining
					call("SetWindowTextW", w.controls[110], ptr(w.timerText()))
				case "unlocked":
					w.finishCleaning(false)
				case "interrupted":
					w.finishCleaning(true)
				case "error":
					w.failure(ev.Message)
				}
			default:
				goto drained
			}
		}
	}
drained:
	due, changed := scheduler.Poll(&w.cfg.Reminders, scheduler.Now())
	if changed {
		w.saveQuiet()
	}
	if due {
		slog.Info("reminder due", "state", w.machine.State(), "time", w.cfg.Reminders.Schedule.Time)
		switch w.machine.State() {
		case app.Idle, app.Snoozed:
			w.remind()
		default:
			slog.Info("reminder coalesced", "state", w.machine.State())
		}
	}
}
func allowedDuringCleaning(id int) bool { return id == 4 || id == 10 || id == 60 || id == 61 }
func (w *window) cancelPreparation() {
	w.countdown = 0
	w.preparingUntil = time.Time{}
	w.remaining = 0
	if w.session != nil {
		w.cancelledSession = true
		w.session.Stop()
	}
	w.machine.Move(app.Idle)
	w.tray(1, "")
}
func (w *window) cancelCleaning() {
	if w.session != nil {
		w.cancelledSession = true
		w.session.Stop()
	}
	w.finishCleaning(true)
}
func (w *window) finishCleaning(interrupted bool) {
	w.machine.Move(app.Finishing)
	w.machine.Move(app.Idle)
	w.countdown = 0
	w.remaining = 0
	w.hide()
	w.tray(1, "")
	if interrupted {
		slog.Info("cleaning interrupted")
		if w.cfg.Notify {
			w.balloon("Очистка остановлена", "Сеанс очистки завершён досрочно. Управление доступно.")
		}
	} else {
		slog.Info("cleaning completed")
		if w.cfg.Notify {
			w.balloon("Очистка завершена", "Управление клавиатурой и мышью восстановлено.")
		}
	}
}
func (w *window) failure(s string) {
	wasCleaning := w.machine.State() == app.Cleaning
	w.countdown = 0
	w.machine.Move(app.Error)
	w.machine.Move(app.Idle)
	w.hide()
	w.tray(1, "")
	slog.Error("cleaning failure", "error", s)
	if wasCleaning {
		s += "\n\nЕсли управление не восстановилось, нажмите Ctrl+Alt+Del."
	}
	ShowError(s)
}

func (w *window) emergencyFooter() {
	w.label("Если нужно вернуть управление", 107, 30, 542, 300, 24, 0)
	w.label("Ctrl+Alt+Del", 108, 340, 540, 180, 28, w.cardFont)
}

func (w *window) centerWindow(width, height int) {
	type rect struct{ L, T, R, B int32 }
	type monitorInfo struct {
		Size          uint32
		Monitor, Work rect
		Flags         uint32
	}
	info := monitorInfo{}
	info.Size = uint32(unsafe.Sizeof(info))
	monitor := call("MonitorFromWindow", w.h, 2) // nearest monitor
	if call("GetMonitorInfoW", monitor, uintptr(unsafe.Pointer(&info))) == 0 {
		call("SystemParametersInfoW", 0x30, 0, uintptr(unsafe.Pointer(&info.Work)), 0)
	}
	width, height = w.px(width), w.px(height)
	x := int(info.Work.L) + (int(info.Work.R-info.Work.L)-width)/2
	y := int(info.Work.T) + (int(info.Work.B-info.Work.T)-height)/2
	if x < int(info.Work.L) {
		x = int(info.Work.L)
	}
	if y < int(info.Work.T) {
		y = int(info.Work.T)
	}
	call("SetWindowPos", w.h, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x14)
}
