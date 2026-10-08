package app

import "sync"

type State string

const (
	Idle      State = "IDLE"
	Reminder  State = "REMINDER"
	Snoozed   State = "SNOOZED"
	Preparing State = "PREPARING"
	Cleaning  State = "CLEANING"
	Finishing State = "FINISHING"
	Error     State = "ERROR"
)

type Machine struct {
	mu    sync.Mutex
	state State
}

func (m *Machine) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == "" {
		return Idle
	}
	return m.state
}
func (m *Machine) Move(to State) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	from := m.state
	if from == "" {
		from = Idle
	}
	ok := false
	switch from {
	case Idle, Snoozed:
		ok = to == Reminder || to == Preparing || to == Idle
	case Reminder:
		ok = to == Preparing || to == Snoozed || to == Idle
	case Preparing:
		ok = to == Cleaning || to == Error || to == Idle
	case Cleaning:
		ok = to == Finishing || to == Error
	case Finishing, Error:
		ok = to == Idle
	}
	if ok {
		m.state = to
	}
	return ok
}
