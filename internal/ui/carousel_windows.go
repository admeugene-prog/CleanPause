package ui

import (
	"cleanpause/assets"
	"fmt"
	"time"
	"unsafe"
)

const carouselInterval = 6 * time.Second

func (w *window) instructionCarousel() {
	if len(w.cards) == 0 {
		instructions := assets.Instructions
		if assets.Language() == "en" {
			instructions = assets.EnglishInstructions
		}
		w.cards = parseInstructions(instructions)
	}
	if len(w.cards) == 0 {
		return
	}
	w.add("STATIC", "Советы по очистке", 130, 30, 370, 840, 150, 0xd|0x04000000)
	w.updateCard()
	if w.cardChanged.IsZero() {
		w.cardChanged = time.Now()
	}
}
func (w *window) updateCard() {
	if len(w.cards) == 0 {
		return
	}
	w.cardIndex = wrappedCardIndex(w.cardIndex, 0, len(w.cards))
	call("InvalidateRect", w.controls[130], 0, 1)
}

// The whole instruction is painted together so its typography and spacing
// remain consistent across cards, including multiline recommendations.
func (w *window) drawInstruction(data unsafe.Pointer) bool {
	type rect struct{ L, T, R, B int32 }
	type item struct {
		Kind, ID, ItemID, Action, State uint32
		H, DC                           uintptr
		R                               rect
		Data                            uintptr
	}
	d := (*item)(data)
	if d.Kind != 5 || d.ID != 130 || len(w.cards) == 0 {
		return false
	}
	bg, ink, muted, accent := uintptr(0xEFF8F3), uintptr(0x605C55), uintptr(0x7B756F), uintptr(0x8AAA08)
	if w.dark() {
		bg, ink, muted = 0x393029, 0xF5F2EB, 0xC6BCAD
	}
	fill := func(r rect, color uintptr, rounded bool) {
		brush, _, _ := gdi.NewProc("CreateSolidBrush").Call(color)
		if rounded {
			old, _, _ := gdi.NewProc("SelectObject").Call(d.DC, brush)
			pen, _, _ := gdi.NewProc("GetStockObject").Call(8) // NULL_PEN
			oldPen, _, _ := gdi.NewProc("SelectObject").Call(d.DC, pen)
			gdi.NewProc("RoundRect").Call(d.DC, uintptr(r.L), uintptr(r.T), uintptr(r.R), uintptr(r.B), uintptr(w.px(18)), uintptr(w.px(18)))
			gdi.NewProc("SelectObject").Call(d.DC, oldPen)
			gdi.NewProc("SelectObject").Call(d.DC, old)
		} else {
			call("FillRect", d.DC, uintptr(unsafe.Pointer(&r)), brush)
		}
		gdi.NewProc("DeleteObject").Call(brush)
	}
	r := func(x, y, width, height int) rect {
		return rect{int32(w.px(x)), int32(w.px(y)), int32(w.px(x + width)), int32(w.px(y + height))}
	}
	fill(d.R, bg, true)
	fill(r(0, 16, 5, 118), accent, false)
	gdi.NewProc("SetBkMode").Call(d.DC, 1)
	text := func(s string, box rect, font, color uintptr, flags uintptr) {
		old, _, _ := gdi.NewProc("SelectObject").Call(d.DC, font)
		gdi.NewProc("SetTextColor").Call(d.DC, color)
		call("DrawTextW", d.DC, ptr(s), ^uintptr(0), uintptr(unsafe.Pointer(&box)), flags|0x800)
		gdi.NewProc("SelectObject").Call(d.DC, old)
	}
	text(fmt.Sprintf("%02d / %02d", w.cardIndex+1, len(w.cards)), r(724, 18, 90, 24), w.font, muted, 2)
	card := w.cards[w.cardIndex]
	text(card.Title, r(26, 14, 690, 34), w.titleFont, ink, 0x20)
	text(card.Text(w.cfg.Cleaning.Duration/60), r(26, 54, 788, 84), w.cardFont, ink, 0x10)
	return true
}
func (w *window) advanceCard(delta int) {
	if len(w.cards) == 0 {
		return
	}
	w.cardIndex = wrappedCardIndex(w.cardIndex, delta, len(w.cards))
	w.cardChanged = time.Now()
	w.updateCard()
}
func (w *window) tickCarousel() {
	if w.visible && (w.screen == "prepare" || w.screen == "countdown" || w.screen == "cleaning") && len(w.cards) > 0 && time.Since(w.cardChanged) >= carouselInterval {
		w.advanceCard(1)
	}
}
