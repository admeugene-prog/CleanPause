package app

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSingleRequest(t *testing.T) {
	var m Machine
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.Move(Preparing) {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal(wins.Load())
	}
	if !m.Move(Cleaning) || m.Move(Preparing) || m.Move(Idle) {
		t.Fatal("invalid cleaning transition")
	}
	if !m.Move(Finishing) || !m.Move(Idle) {
		t.Fatal("finish failed")
	}
}
func TestErrorAndSnooze(t *testing.T) {
	var m Machine
	for _, s := range []State{Reminder, Snoozed, Reminder, Preparing, Error, Idle} {
		if !m.Move(s) {
			t.Fatal(s)
		}
	}
}
