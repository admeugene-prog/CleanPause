package scheduler

import (
	"cleanpause/internal/config"
	"fmt"
	"time"
)

func Allowed(s config.Schedule, d time.Weekday) bool {
	switch s.Type {
	case "daily":
		return true
	case "workdays":
		return d >= time.Monday && d <= time.Friday
	default:
		for _, v := range s.Days {
			if v == int(d) {
				return true
			}
		}
	}
	return false
}

// Calendar days (not 24-hour durations) preserve local wall time through DST.
func Next(s config.Schedule, after time.Time) time.Time {
	t, err := time.Parse("15:04", s.Time)
	if err != nil {
		return time.Time{}
	}
	for i := 0; i < 9; i++ {
		d := after.AddDate(0, 0, i)
		candidate := time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), 0, 0, after.Location())
		if Allowed(s, candidate.Weekday()) && candidate.After(after) {
			return candidate
		}
	}
	return time.Time{}
}
func NextDay(s config.Schedule, now time.Time) time.Time {
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return Next(s, midnight.Add(-time.Nanosecond))
}
func Key(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d/%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
}

// Reset applies an explicit schedule edit, replacing any previous snooze.
// Selecting the current minute should produce a reminder on the next poll.
func Reset(r *config.Reminders, now time.Time) {
	r.SnoozedUntil = nil
	r.LastOccurrence = ""
	r.NextDue = nil
	if r.Enabled {
		minuteStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
		n := Next(r.Schedule, minuteStart.Add(-time.Nanosecond))
		r.NextDue = &n
	}
}

// Poll consumes each regular occurrence once, retaining one overdue event across restarts.
func Poll(r *config.Reminders, now time.Time) (due, changed bool) {
	if !r.Enabled {
		return false, false
	}
	if r.NextDue == nil {
		n := Next(r.Schedule, now)
		r.NextDue = &n
		changed = true
	}
	// Re-anchor the calendar occurrence when Windows changes its UTC offset.
	_, oldOffset := r.NextDue.Zone()
	_, newOffset := now.Zone()
	if oldOffset != newOffset {
		old := *r.NextDue
		clock, _ := time.Parse("15:04", r.Schedule.Time)
		n := time.Date(old.Year(), old.Month(), old.Day(), clock.Hour(), clock.Minute(), 0, 0, now.Location())
		r.NextDue = &n
		changed = true
	}
	if !r.NextDue.After(now) {
		k := Key(*r.NextDue)
		due = k != r.LastOccurrence
		r.LastOccurrence = k
		n := Next(r.Schedule, now)
		r.NextDue = &n
		changed = true
	}
	if r.SnoozedUntil != nil {
		if !r.SnoozedUntil.After(now) {
			due = true
			r.SnoozedUntil = nil
			changed = true
		} else {
			due = false
		}
	}
	return
}
