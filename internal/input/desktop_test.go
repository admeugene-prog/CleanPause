package input

import (
	"runtime"
	"testing"
	"time"
)

func TestDesktopSwitchNotificationIsLatched(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	watcher, err := newDesktopWatch()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	// System desktop-switch events cannot be synthesized with NotifyWinEvent.
	// Exercise the callback's latch directly without switching the real desktop.
	watcher.recordEvent(0x800c)
	if watcher.switched.Load() {
		t.Fatal("unrelated accessibility event cancelled cleaning")
	}
	watcher.recordEvent(desktopSwitchEvent)
	if !watcher.switched.Load() {
		t.Fatal("desktop switch event not delivered on worker thread")
	}
	if !watcher.Interrupted() {
		t.Fatal("rapid switch back erased interruption")
	}
}
func TestDesktopInterruptionEndsCountdown(t *testing.T) {
	start := time.Now()
	heartbeats := 0
	interrupted := waitForDeadline(millis()+60000, func() bool { return false }, func() bool { return true }, func(Event) { heartbeats++ })
	if !interrupted || heartbeats != 0 || time.Since(start) > 2*time.Second {
		t.Fatal("countdown continued after emergency recovery")
	}
}
func TestDeadlineStillFinishesNormally(t *testing.T) {
	if waitForDeadline(millis()+10, func() bool { return false }, func() bool { return false }, func(Event) {}) {
		t.Fatal("normal completion classified as interruption")
	}
}
