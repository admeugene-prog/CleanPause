package input

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--test-parent" {
		s, e := Start(60, true)
		if e != nil {
			os.Exit(2)
		}
		for ev := range s.Events {
			if ev.Type == "locked" {
				fmt.Fprintln(os.Stdout, s.cmd.Process.Pid)
				for {
					time.Sleep(time.Hour)
				}
			}
		}
		os.Exit(2)
	}
	if len(os.Args) > 1 && os.Args[1] == "--input-worker" {
		if os.Getenv("CLEANPAUSE_TEST_INTERRUPT") == "1" {
			encoder := json.NewEncoder(os.Stdout)
			encoder.Encode(Event{Type: "locked", Remaining: 60})
			encoder.Encode(Event{Type: "interrupted"})
			os.Exit(0)
		}
		if os.Getenv("CLEANPAUSE_TEST_HANG") == "1" {
			json.NewEncoder(os.Stdout).Encode(Event{Type: "locked", Remaining: 60})
			for {
				time.Sleep(time.Hour)
			}
		}
		os.Exit(Worker(os.Args[2:]))
	}
	os.Exit(m.Run())
}
func TestInterruptedWorkerIsTerminal(t *testing.T) {
	t.Setenv("CLEANPAUSE_TEST_INTERRUPT", "1")
	s, err := Start(60, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	until(t, s, "interrupted", 5*time.Second)
	for ev := range s.Events {
		if ev.Type == "error" || ev.Type == "heartbeat" {
			t.Fatal("interrupted worker reported more activity", ev)
		}
	}
}
func TestWorkerDetectsParentExit(t *testing.T) {
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "--test-parent")
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer cmd.Process.Kill()
	pidCh := make(chan int, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			pid, _ := strconv.Atoi(scanner.Text())
			pidCh <- pid
		}
	}()
	var pid int
	select {
	case pid = <-pidCh:
	case <-time.After(5 * time.Second):
		t.Fatal("helper did not start")
	}
	handle, _, err := kernel.NewProc("OpenProcess").Call(0x100000, 0, uintptr(pid))
	if handle == 0 {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(syscall.Handle(handle))
	cmd.Process.Kill()
	cmd.Wait()
	r, _, err := kernel.NewProc("WaitForSingleObject").Call(handle, 5000)
	if r != 0 {
		kernel.NewProc("TerminateProcess").Call(handle, 1)
		t.Fatal("orphan worker survived", r, err)
	}
}
func until(t *testing.T, s *Session, kind string, timeout time.Duration) Event {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case ev, ok := <-s.Events:
			if !ok {
				t.Fatalf("worker closed before %s", kind)
			}
			if ev.Type == kind {
				return ev
			}
		case <-timer.C:
			s.Stop()
			t.Fatalf("timeout waiting for %s", kind)
		}
	}
}
func TestWorkerProtocolAndCrash(t *testing.T) {
	s, e := Start(60, true)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Stop()
	if ev := until(t, s, "locked", 5*time.Second); ev.Remaining != 60 {
		t.Fatal(ev)
	}
	until(t, s, "heartbeat", 3*time.Second)
	s.Stop()
	until(t, s, "error", 5*time.Second)
}
func TestWatchdogHang(t *testing.T) {
	t.Setenv("CLEANPAUSE_TEST_HANG", "1")
	s, e := Start(60, true)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Stop()
	until(t, s, "locked", 5*time.Second)
	ev := until(t, s, "error", 9*time.Second)
	if !strings.Contains(ev.Message, "Watchdog") {
		t.Fatal(ev)
	}
}
func TestIndependentTimerWithStalledGUI(t *testing.T) {
	if testing.Short() {
		t.Skip("60-second independent worker timer")
	}
	s, e := Start(60, true)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Stop()
	until(t, s, "locked", 5*time.Second)
	start := time.Now()
	timer := time.NewTimer(62 * time.Second)
	defer timer.Stop()
	<-timer.C
	found := false
	for ev := range s.Events {
		if ev.Type == "unlocked" {
			found = true
		}
		if ev.Type == "error" {
			t.Fatal(ev)
		}
	}
	if !found || time.Since(start) > 67*time.Second {
		t.Fatal("worker did not complete independently")
	}
}
