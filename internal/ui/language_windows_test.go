package ui

import (
	"cleanpause/assets"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLanguageDetection(t *testing.T) {
	for id, want := range map[uint16]string{0x0419: "ru", 0x0819: "ru", 0x0409: "en", 0x0809: "en", 0x0407: "en"} {
		if got := languageForWindows(id); got != want {
			t.Fatalf("%x: %s", id, got)
		}
	}
}
func TestLanguageRegistryRoundTrip(t *testing.T) {
	path := fmt.Sprintf(`Software\CleanPause\Tests\Language-%d-%d`, os.Getpid(), time.Now().UnixNano())
	defer syscall.NewLazyDLL("advapi32.dll").NewProc("RegDeleteKeyW").Call(uintptr(syscall.HKEY_CURRENT_USER), ptr(path))
	for _, code := range []string{"ru", "en", ""} {
		if err := writeLanguageOverride(path, code); err != nil {
			t.Fatal(err)
		}
		got, err := readLanguageOverride(path)
		if err != nil || got != code {
			t.Fatalf("%q: %q %v", code, got, err)
		}
	}
	if err := writeLanguageOverride(path, "invalid"); err == nil {
		t.Fatal("invalid language accepted")
	}
}
func TestEnglishInstructionDurationAndCoverage(t *testing.T) {
	cards := parseInstructions(assets.EnglishInstructions)
	if len(cards) != len(parseInstructions(assets.Instructions)) {
		t.Fatalf("lost cards: %d", len(cards))
	}
	for _, card := range cards {
		if strings.Contains(card.Text(1), "5 minutes") || strings.Contains(card.Body, "**") {
			t.Fatal("incorrect English instruction", card)
		}
	}
}
