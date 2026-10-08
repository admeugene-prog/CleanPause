package ui

import (
	"cleanpause/assets"
	"cleanpause/internal/app"
	"strings"
	"testing"
	"time"
)

func TestProvidedInstructionsBecomeReadableCards(t *testing.T) {
	cards := parseInstructions(assets.Instructions)
	if len(cards) < 10 {
		t.Fatal("instructions dropped", len(cards))
	}
	joined := ""
	for _, card := range cards {
		if card.Title == "" || card.Body == "" || strings.Contains(card.Body, "**") {
			t.Fatal("unreadable markdown", card)
		}
		joined += card.Title + card.Body
	}
	for _, phrase := range []string{"Используйте спиртовые салфетки", "Дождитесь полного высыхания", "не означает автоматически высокий риск заболевания", "Раз в 1–2 недели", "Соблюдайте рекомендации производителя"} {
		if !strings.Contains(joined, phrase) {
			t.Fatal("missing source content", phrase)
		}
	}
	for _, card := range cards {
		if strings.Contains(card.Text(1), "5 минут") {
			t.Fatal("incorrect cleaning duration", card)
		}
	}
	t.Logf("%d instruction cards", len(cards))
}
func TestCarouselCyclesWhileInputIsBlocked(t *testing.T) {
	w := window{cards: parseInstructions(assets.Instructions), visible: true, screen: "cleaning", cardChanged: time.Now().Add(-carouselInterval - time.Second), remaining: 123, controls: map[int]uintptr{}}
	w.machine.Move(app.Preparing)
	w.machine.Move(app.Cleaning)
	w.tickCarousel()
	if w.cardIndex != 1 || w.remaining != 123 || w.machine.State() != app.Cleaning {
		t.Fatal("carousel affected blocking state or did not advance")
	}
	w.visible = false
	w.cardChanged = time.Now().Add(-carouselInterval - time.Second)
	w.tickCarousel()
	if w.cardIndex != 1 {
		t.Fatal("hidden carousel still running")
	}
	w.advanceCard(-2)
	if w.cardIndex != len(w.cards)-1 {
		t.Fatal("previous card did not wrap")
	}
}
func TestReminderBannerDecodesIntoWindowsBitmap(t *testing.T) {
	a, err := decodeArtwork(assets.ReminderArtwork)
	if err != nil {
		t.Fatal(err)
	}
	bitmap, err := nativeArtworkBitmap(a, 480, 270)
	if err != nil {
		t.Fatal(err)
	}
	defer gdi.NewProc("DeleteObject").Call(bitmap)
	if bitmap == 0 {
		t.Fatal("reminder banner missing")
	}
}
