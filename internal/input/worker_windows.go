package input

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var block = syscall.NewLazyDLL("user32.dll").NewProc("BlockInput")
var tick = kernel.NewProc("GetTickCount64")

type Event struct {
	Type      string `json:"type"`
	Message   string `json:"message,omitempty"`
	Remaining int    `json:"remaining,omitempty"`
}

func millis() uint64 { r, _, _ := tick.Call(); return uint64(r) }

// Worker owns both BlockInput calls on one OS thread. No UI or pipe write can delay its deadline.
func Worker(args []string) int {
	if len(args) != 3 {
		return 2
	}
	seconds, e := strconv.Atoi(args[0])
	parent, e2 := strconv.Atoi(args[1])
	simulation := args[2] == "simulate"
	if e != nil || e2 != nil || seconds < 60 || seconds > 900 || parent <= 0 || (!simulation && args[2] != "real") {
		return 2
	}
	ready := make(chan bool, 1)
	go func() { s := bufio.NewScanner(os.Stdin); ready <- s.Scan() && s.Text() == "START" }()
	select {
	case ok := <-ready:
		if !ok {
			return 2
		}
	case <-time.After(5 * time.Second):
		return 2
	}
	events := make(chan Event, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		enc := json.NewEncoder(os.Stdout)
		for ev := range events {
			if enc.Encode(ev) != nil {
				return
			}
		}
	}()
	emit := func(ev Event) {
		select {
		case events <- ev:
		default:
		}
	}
	handle, _, _ := kernel.NewProc("OpenProcess").Call(0x100000, 0, uintptr(parent))
	if handle == 0 {
		emit(Event{Type: "error", Message: "Не удалось проверить управляющий процесс"})
		close(events)
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		return 1
	}
	defer syscall.CloseHandle(syscall.Handle(handle))
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var desktopMonitor *desktopWatch
	if !simulation {
		u := syscall.NewLazyDLL("user32.dll")
		desktop, _, desktopErr := u.NewProc("OpenInputDesktop").Call(0, 0, 0x101)
		if desktop == 0 {
			emit(Event{Type: "error", Message: fmt.Sprintf("Интерактивный рабочий стол недоступен: %v", desktopErr)})
			close(events)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return 1
		}
		var desktopName [128]uint16
		var required uint32
		ok, _, _ := u.NewProc("GetUserObjectInformationW").Call(desktop, 2, uintptr(unsafe.Pointer(&desktopName[0])), uintptr(unsafe.Sizeof(desktopName)), uintptr(unsafe.Pointer(&required)))
		u.NewProc("CloseDesktop").Call(desktop)
		if ok == 0 || syscall.UTF16ToString(desktopName[:]) != "Default" {
			emit(Event{Type: "error", Message: "Очистка доступна только на обычном интерактивном рабочем столе Windows"})
			close(events)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return 1
		}
		var watchErr error
		desktopMonitor, watchErr = newDesktopWatch()
		if watchErr != nil {
			emit(Event{Type: "error", Message: watchErr.Error()})
			close(events)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return 1
		}
		defer desktopMonitor.Close()
		r, _, err := block.Call(1)
		if r == 0 {
			message := fmt.Sprintf("Windows отказала в блокировке ввода: %v", err)
			if err == syscall.ERROR_ACCESS_DENIED {
				message = "Windows запретила блокировку ввода (ошибка 5: доступ запрещён).\nОчистка не началась, ввод не был заблокирован.\n\nЗавершите CleanPause через трей и запустите с флагом --enable-input, подтвердив запрос UAC. Если приложение уже запущено с правами администратора, причиной могут быть ограничения сеанса или политики Windows."
			}
			emit(Event{Type: "error", Message: message})
			close(events)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return 1
		}
		defer block.Call(0)
	}
	deadline := millis() + uint64(seconds)*1000
	emit(Event{Type: "locked", Remaining: seconds})
	interrupted := waitForDeadline(deadline, func() bool { status, _, _ := kernel.NewProc("WaitForSingleObject").Call(handle, 0); return status == 0 }, func() bool { return desktopMonitor != nil && desktopMonitor.Interrupted() }, emit)
	if !simulation {
		r, _, err := block.Call(0)
		if r == 0 && !interrupted {
			emit(Event{Type: "error", Message: fmt.Sprintf("Блокировка уже снята системой либо API вернула ошибку: %v", err)})
			close(events)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return 1
		}
	}
	if interrupted {
		emit(Event{Type: "interrupted", Message: "Очистка остановлена после переключения рабочего стола или системного восстановления ввода."})
	} else {
		emit(Event{Type: "unlocked"})
	}
	close(events)
	select {
	case <-done:
	case <-time.After(time.Second):
	}
	return 0
}

func waitForDeadline(deadline uint64, parentExited, desktopInterrupted func() bool, emit func(Event)) bool {
	timer := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
	last := uint64(0)
	for range timer.C {
		// Observe interruption before the deadline so a simultaneous Ctrl+Alt+Del
		// is handled as an interrupted session, without an erroneous unlock error.
		if desktopInterrupted() {
			return true
		}
		now := millis()
		if now >= deadline || parentExited() {
			return false
		}
		if now-last >= 1000 {
			emit(Event{Type: "heartbeat", Remaining: int((deadline - now + 999) / 1000)})
			last = now
		}
	}
	return false
}

type Session struct {
	ID     string
	Events chan Event
	cmd    *exec.Cmd
	once   sync.Once
}

func (s *Session) Stop() {
	s.once.Do(func() {
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	})
}

// Anonymous inherited pipes are local IPC; the process handle is an independent watchdog signal.
func Start(seconds int, simulation bool) (*Session, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	mode := "real"
	if simulation {
		mode = "simulate"
	}
	cmd := exec.Command(exe, "--input-worker", strconv.Itoa(seconds), strconv.Itoa(os.Getpid()), mode)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		stdout.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	var sessionID [16]byte
	_, _ = rand.Read(sessionID[:])
	s := &Session{ID: hex.EncodeToString(sessionID[:]), Events: make(chan Event, 32), cmd: cmd}
	if _, err = fmt.Fprintln(stdin, "START"); err != nil {
		s.Stop()
	}
	stdin.Close()
	go func() {
		incoming := make(chan Event, 32)
		readerDone := make(chan struct{})
		go func() {
			defer close(readerDone)
			scan := bufio.NewScanner(stdout)
			for scan.Scan() {
				var ev Event
				if json.Unmarshal(scan.Bytes(), &ev) == nil {
					select {
					case incoming <- ev:
					default:
					}
				}
			}
		}()
		watchdog := time.NewTicker(time.Second)
		defer watchdog.Stop()
		last := time.Now()
		hardEnd := last.Add(time.Duration(seconds+10) * time.Second)
		finished := false
		deliver := func(ev Event) {
			// Reserve capacity for terminal events when the GUI is stalled.
			if ev.Type == "heartbeat" && len(s.Events) >= cap(s.Events)-4 {
				return
			}
			select {
			case s.Events <- ev:
			default:
			}
		}
		alive := true
		for alive {
			select {
			case ev := <-incoming:
				last = time.Now()
				if terminalEvent(ev.Type) {
					finished = true
				}
				deliver(ev)
			case <-readerDone:
				for {
					select {
					case ev := <-incoming:
						if terminalEvent(ev.Type) {
							finished = true
						}
						deliver(ev)
					default:
						alive = false
					}
					if !alive {
						break
					}
				}
			case <-watchdog.C:
				if time.Since(last) > 5*time.Second || time.Now().After(hardEnd) {
					s.Stop()
					deliver(Event{Type: "error", Message: "Watchdog завершил worker: превышено время ожидания"})
					finished = true
					alive = false
				}
			}
		}
		waited := make(chan error, 1)
		go func() { waited <- cmd.Wait() }()
		var err error
		select {
		case err = <-waited:
		case <-time.After(2 * time.Second):
			s.Stop()
			err = <-waited
			if !finished {
				deliver(Event{Type: "error", Message: "Worker закрыл IPC и не завершился; процесс остановлен"})
				finished = true
			}
		}
		if !finished {
			deliver(Event{Type: "error", Message: fmt.Sprintf("Worker неожиданно завершился: %v", err)})
		}
		close(s.Events)
	}()
	return s, nil
}
func terminalEvent(kind string) bool {
	return kind == "unlocked" || kind == "interrupted" || kind == "error"
}
