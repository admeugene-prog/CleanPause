package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c := Default()
	n := time.Now().Add(time.Hour).Truncate(time.Second)
	c.Reminders.SnoozedUntil = &n
	if e := Save(p, c); e != nil {
		t.Fatal(e)
	}
	c.Cleaning.Duration = 60
	if e := Save(p, c); e != nil {
		t.Fatal(e)
	}
	got, e := Load(p)
	if e != nil || got.Cleaning.Duration != 60 || !got.Reminders.SnoozedUntil.Equal(n) {
		t.Fatal(got, e)
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*.tmp"))
	if len(files) != 0 {
		t.Fatal(files)
	}
}
func TestCorruptAndInvalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	for _, s := range []string{"{broken", `{"version":7}`, `{"cleaning":{"duration_seconds":901}}`, `{"reminders":{"schedule":{"type":"custom","days":[],"time":"10:00"}}}`} {
		os.WriteFile(p, []byte(s), 0600)
		c, e := Load(p)
		if e == nil || c.Cleaning.Duration != 300 {
			t.Fatal(c, e)
		}
	}
}
func TestInvalidDoesNotReplace(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c := Default()
	Save(p, c)
	c.Cleaning.Duration = 0
	if Save(p, c) == nil {
		t.Fatal("accepted invalid config")
	}
	c, e := Load(p)
	if e != nil || c.Cleaning.Duration != 300 {
		t.Fatal(c, e)
	}
}
