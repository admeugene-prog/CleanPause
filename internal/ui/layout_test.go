package ui

import (
	"cleanpause/internal/app"
	"cleanpause/internal/config"
	"cleanpause/internal/input"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

// These native ABI structures must match the x64 Windows headers.
func TestWindowsABIStructures(t *testing.T) {
	if unsafe.Sizeof(wc{}) != 80 {
		t.Fatal("WNDCLASSEXW", unsafe.Sizeof(wc{}))
	}
	if unsafe.Sizeof(message{}) != 48 {
		t.Fatal("MSG", unsafe.Sizeof(message{}))
	}
	if unsafe.Sizeof(notify{}) != 976 {
		t.Fatal("NOTIFYICONDATAW", unsafe.Sizeof(notify{}))
	}
}

func TestFocusNotificationsDoNotStartOrCancelCleaning(t *testing.T) {
	w := &window{simulate: true, controls: map[int]uintptr{}}
	w.machine.Move(app.Preparing)
	previous := current
	current = w
	defer func() { current = previous }()
	for _, id := range []uintptr{3, 4} {
		wndProc(0, 0x111, id|(6<<16), nil) // BN_SETFOCUS
		if w.countdown != 0 || w.machine.State() != app.Preparing {
			t.Fatal("focus treated as a button click")
		}
	}
}
func TestEmergencyRecoveryRestoresControls(t *testing.T) {
	w := window{remaining: 240, countdown: 3, controls: map[int]uintptr{}}
	w.machine.Move(app.Preparing)
	w.machine.Move(app.Cleaning)
	w.finishCleaning(true)
	if w.machine.State() != app.Idle || w.remaining != 0 || w.countdown != 0 {
		t.Fatal("cleaning UI did not stop")
	}
	if !allowedDuringCleaning(4) || !allowedDuringCleaning(10) || allowedDuringCleaning(1) {
		t.Fatal("cannot stop session after input recovery")
	}
}
func TestLateWorkerEventsAfterStopAreIgnored(t *testing.T) {
	w := window{cancelledSession: true, session: &input.Session{Events: make(chan input.Event, 3)}, controls: map[int]uintptr{}}
	w.session.Events <- input.Event{Type: "heartbeat", Remaining: 240}
	w.session.Events <- input.Event{Type: "locked", Remaining: 300}
	w.session.Events <- input.Event{Type: "error", Message: "cancelled"}
	close(w.session.Events)
	w.onTick()
	if w.remaining != 0 || w.machine.State() != app.Idle || w.session != nil {
		t.Fatal("stopped session resumed from stale events")
	}
}

func TestCancelCountdownIgnoresQueuedWorkerLock(t *testing.T) {
	w := window{countdown: 2, preparingUntil: time.Now().Add(time.Second), controls: map[int]uintptr{}, session: &input.Session{Events: make(chan input.Event, 1)}}
	w.machine.Move(app.Preparing)
	w.session.Events <- input.Event{Type: "locked", Remaining: 300}
	close(w.session.Events)
	w.close()
	w.onTick()
	if w.machine.State() != app.Idle || w.countdown != 0 || !w.preparingUntil.IsZero() || w.session != nil {
		t.Fatal("cancelled countdown retained an active session")
	}
}

func TestExitCancelsCountdown(t *testing.T) {
	w := window{countdown: 3, controls: map[int]uintptr{}}
	w.machine.Move(app.Preparing)
	w.command(10)
	if !w.quitting || w.machine.State() != app.Idle || w.countdown != 0 {
		t.Fatal("exit during preparation was blocked")
	}
}

func TestClosedWorkerRestoresIdleDuringPreparation(t *testing.T) {
	w := window{controls: map[int]uintptr{}, session: &input.Session{Events: make(chan input.Event)}}
	w.machine.Move(app.Preparing)
	close(w.session.Events)
	w.onTick()
	if w.machine.State() != app.Idle || w.session != nil {
		t.Fatal("closed worker left tray in preparation state")
	}
}

func TestScheduledReminderCreatesVisibleWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w := window{scale: 1, simulate: true, cfg: config.Default(), controls: map[int]uintptr{}, path: filepath.Join(t.TempDir(), "config.json")}
	w.instance, _, _ = kernel.NewProc("GetModuleHandleW").Call(0)
	w.h = call("CreateWindowExW", 0, ptr("STATIC"), ptr("CleanPause reminder test"), 0x00C80000, 0, 0, 100, 100, 0, 0, w.instance, 0)
	if w.h == 0 {
		t.Fatal("cannot create test window")
	}
	w.font = w.newFont(16, 400)
	w.titleFont = w.newFont(26, 700)
	defer func() {
		call("DestroyWindow", w.h)
		for _, object := range []uintptr{w.font, w.titleFont, w.brush, w.artBitmap} {
			if object != 0 {
				gdi.NewProc("DeleteObject").Call(object)
			}
		}
	}()
	w.cfg.Reminders.SnoozedUntil = nil
	due := time.Now().Add(-time.Second)
	w.cfg.Reminders.NextDue = &due
	w.onTick()
	if w.machine.State() != app.Reminder || w.screen != "reminder" || call("IsWindowVisible", w.h) == 0 || w.controls[102] == 0 {
		t.Fatal("scheduled event did not show reminder window", w.machine.State(), w.screen)
	}
	w.command(1)
	if w.machine.State() != app.Preparing || w.screen != "countdown" || w.countdown != 3 || w.preparingUntil.IsZero() {
		t.Fatal("reminder start required a second confirmation", w.machine.State(), w.screen, w.countdown)
	}
}
