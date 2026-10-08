package scheduler

import (
	"cleanpause/internal/config"
	"testing"
	"time"
	_ "time/tzdata"
)

func date(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04", s, time.UTC)
	return t
}

func TestScheduleEditReplacesSnooze(t *testing.T) {
	r := config.Default().Reminders
	r.Schedule = config.Schedule{Type: "custom", Days: []int{4}, Time: "10:31"}
	now := date("2026-10-08 10:30")
	snooze := date("2026-10-08 10:41")
	r.SnoozedUntil = &snooze
	r.LastOccurrence = "old occurrence"
	Reset(&r, now)
	if r.SnoozedUntil != nil || r.LastOccurrence != "" || !r.NextDue.Equal(date("2026-10-08 10:31")) {
		t.Fatal("old schedule state retained", r)
	}
	if due, _ := Poll(&r, date("2026-10-08 10:31")); !due {
		t.Fatal("new time suppressed by old snooze")
	}
	if due, _ := Poll(&r, date("2026-10-08 10:31").Add(time.Second)); due {
		t.Fatal("duplicate reminder")
	}
}

func TestScheduleEditDuringCurrentMinute(t *testing.T) {
	r := config.Default().Reminders
	r.Schedule.Time = "10:31"
	now := date("2026-10-08 10:31").Add(34 * time.Second)
	Reset(&r, now)
	if due, _ := Poll(&r, now); !due {
		t.Fatal("current minute skipped")
	}
	r.Schedule.Time = "10:30"
	Reset(&r, now)
	if due, _ := Poll(&r, now); due {
		t.Fatal("past minute should use next scheduled day")
	}
	r.Enabled = false
	Reset(&r, now)
	if r.NextDue != nil {
		t.Fatal("disabled schedule was armed")
	}
}
func TestNext(t *testing.T) {
	s := config.Default().Reminders.Schedule
	cases := []struct{ now, want string }{{"2026-10-09 09:59", "2026-10-09 10:00"}, {"2026-10-09 10:00", "2026-10-12 10:00"}, {"2026-10-10 12:00", "2026-10-12 10:00"}}
	for _, c := range cases {
		if got := Next(s, date(c.now)); !got.Equal(date(c.want)) {
			t.Errorf("%s: %s", c.now, got)
		}
	}
	if n := NextDay(s, date("2026-10-09 09:00")); !n.Equal(date("2026-10-12 10:00")) {
		t.Fatal(n)
	}
}
func TestCustomAndDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	s := config.Schedule{Type: "custom", Days: []int{0}, Time: "10:00"}
	now := time.Date(2026, 3, 7, 15, 0, 0, 0, loc)
	n := Next(s, now)
	if n.Day() != 8 || n.Hour() != 10 || n.Sub(now) != 18*time.Hour {
		t.Fatal(n, n.Sub(now))
	}
	s.Time = "01:30"
	now = time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	r := config.Reminders{Enabled: true, Schedule: s}
	Poll(&r, now)
	due, _ := Poll(&r, now.Add(150*time.Minute))
	if !due {
		t.Fatal("missing DST occurrence")
	}
	due, _ = Poll(&r, now.Add(210*time.Minute))
	if due {
		t.Fatal("duplicate repeated hour")
	}
}
func TestOverdueAndPersistedSnooze(t *testing.T) {
	r := config.Default().Reminders
	now := date("2026-10-08 09:00")
	Poll(&r, now)
	due, _ := Poll(&r, date("2026-10-12 12:00"))
	if !due {
		t.Fatal("overdue missing")
	}
	due, _ = Poll(&r, date("2026-10-12 12:01"))
	if due {
		t.Fatal("overdue duplicated")
	}
	n := date("2026-10-13 12:00")
	r.SnoozedUntil = &n
	due, _ = Poll(&r, date("2026-10-13 10:01"))
	if due {
		t.Fatal("regular event ignored snooze")
	}
	due, _ = Poll(&r, n)
	if !due || r.SnoozedUntil != nil {
		t.Fatal("snooze missing")
	}
	due, _ = Poll(&r, n)
	if due {
		t.Fatal("snooze duplicated")
	}
}
func TestClockBackward(t *testing.T) {
	r := config.Default().Reminders
	Poll(&r, date("2026-10-08 09:00"))
	Poll(&r, date("2026-10-08 10:00"))
	due, _ := Poll(&r, date("2026-10-08 08:00"))
	if due {
		t.Fatal("backwards time duplicate")
	}
	due, _ = Poll(&r, date("2026-10-08 10:00"))
	if due {
		t.Fatal("duplicate")
	}
}
func TestTimezoneChangePreservesCalendarTime(t *testing.T) {
	r := config.Default().Reminders
	old := time.FixedZone("old", 3*3600)
	fresh := time.FixedZone("new", 5*3600)
	Poll(&r, time.Date(2026, 10, 8, 9, 0, 0, 0, old))
	due, changed := Poll(&r, time.Date(2026, 10, 8, 9, 30, 0, 0, fresh))
	if due || !changed || r.NextDue.Hour() != 10 {
		t.Fatal("timezone reanchor failed", r.NextDue)
	}
	due, _ = Poll(&r, time.Date(2026, 10, 8, 10, 0, 0, 0, fresh))
	if !due {
		t.Fatal("local occurrence missing")
	}
	due, _ = Poll(&r, time.Date(2026, 10, 8, 10, 0, 0, 0, old))
	if due {
		t.Fatal("timezone duplicated occurrence")
	}
}
