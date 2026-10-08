package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Schedule struct {
	Type string `json:"type"`
	Days []int  `json:"days"`
	Time string `json:"time"`
}
type Cleaning struct {
	Duration    int `json:"duration_seconds"`
	Preparation int `json:"preparation_seconds"`
}
type Reminders struct {
	Enabled        bool       `json:"enabled"`
	Schedule       Schedule   `json:"schedule"`
	SnoozeMinutes  int        `json:"snooze_minutes"`
	SnoozedUntil   *time.Time `json:"snoozed_until"`
	NextDue        *time.Time `json:"next_due"`
	LastOccurrence string     `json:"last_occurrence"`
}
type Config struct {
	Version   int       `json:"version"`
	Autostart bool      `json:"autostart"`
	Language  string    `json:"language"`
	Theme     string    `json:"theme"`
	Notify    bool      `json:"notify_completion"`
	Cleaning  Cleaning  `json:"cleaning"`
	Reminders Reminders `json:"reminders"`
}

func Default() Config {
	return Config{Version: 1, Autostart: true, Language: "ru", Theme: "system", Notify: true, Cleaning: Cleaning{300, 3}, Reminders: Reminders{Enabled: true, Schedule: Schedule{"workdays", []int{1, 2, 3, 4, 5}, "10:00"}, SnoozeMinutes: 15}}
}
func Path() string { return filepath.Join(os.Getenv("LOCALAPPDATA"), "CleanPause", "config.json") }
func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("неподдерживаемая версия настроек: %d", c.Version)
	}
	if c.Cleaning.Duration < 60 || c.Cleaning.Duration > 900 || c.Cleaning.Preparation != 3 {
		return fmt.Errorf("длительность должна быть 1–15 минут, подготовка — 3 секунды")
	}
	if (c.Language != "ru" && c.Language != "en") || (c.Theme != "system" && c.Theme != "light" && c.Theme != "dark") {
		return fmt.Errorf("неподдерживаемый язык или тема")
	}
	if parsed, err := time.Parse("15:04", c.Reminders.Schedule.Time); err != nil || parsed.Format("15:04") != c.Reminders.Schedule.Time {
		return fmt.Errorf("время должно иметь формат ЧЧ:ММ")
	}
	s := c.Reminders.Schedule
	if s.Type != "daily" && s.Type != "workdays" && s.Type != "custom" {
		return fmt.Errorf("неизвестный тип расписания")
	}
	if s.Type == "custom" && len(s.Days) == 0 {
		return fmt.Errorf("выберите хотя бы один день")
	}
	for _, d := range s.Days {
		if d < 0 || d > 6 {
			return fmt.Errorf("неверный день недели")
		}
	}
	if c.Reminders.SnoozeMinutes != 15 && c.Reminders.SnoozeMinutes != 60 && c.Reminders.SnoozeMinutes != 120 && c.Reminders.SnoozeMinutes != 180 {
		return fmt.Errorf("неверный интервал переноса")
	}
	return nil
}
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err == nil {
		err = c.Validate()
	}
	if err != nil {
		return Default(), err
	}
	return c, nil
}
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return replace(name, path)
}
