package ui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type instructionCard struct{ Title, Body string }

var numberedInstruction = regexp.MustCompile(`^\d+\.\s+`)

func plainInstruction(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "**", ""), "*", ""))
}
func instructionHeading(s string) string {
	s = strings.TrimLeftFunc(strings.TrimSpace(s), func(r rune) bool { return !unicode.IsLetter(r) })
	return numberedInstruction.ReplaceAllString(s, "")
}
func parseInstructions(markdown string) []instructionCard {
	cards := []instructionCard{}
	heading := "Как почистить устройства"
	table := []string{}
	flush := func() {
		if len(table) > 0 {
			cards = append(cards, instructionCard{heading, strings.Join(table, "\n")})
			table = nil
		}
	}
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "|") {
			cols := strings.Split(strings.Trim(line, "|"), "|")
			if len(cols) >= 2 && !strings.Contains(cols[0], "---") && !strings.Contains(cols[0], "Условия использования") && strings.TrimSpace(cols[0]) != "Usage" {
				table = append(table, strings.TrimSpace(cols[0])+" — "+strings.TrimSpace(cols[1]))
			}
			continue
		}
		flush()
		if strings.HasPrefix(line, "#") {
			heading = instructionHeading(strings.TrimLeft(line, "# "))
			continue
		}
		line = numberedInstruction.ReplaceAllString(line, "")
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "> ")
		if start := strings.Index(line, "**"); start >= 0 {
			if end := strings.Index(line[start+2:], "**"); end >= 0 {
				title := strings.TrimRight(plainInstruction(line[start+2:start+2+end]), ".,:")
				body := strings.TrimLeft(plainInstruction(line[start+end+4:]), " ,:—–-")
				if body != "" {
					cards = append(cards, instructionCard{title, body})
					continue
				}
			}
		}
		cards = append(cards, instructionCard{heading, plainInstruction(line)})
	}
	flush()
	return cards
}
func (card instructionCard) Text(minutes int) string {
	text := strings.ReplaceAll(card.Body, "Через 5 минут", fmt.Sprintf("Через %d мин", minutes))
	text = strings.ReplaceAll(text, "Всего 5 минут", fmt.Sprintf("Всего %d мин", minutes))
	text = strings.ReplaceAll(text, "after 5 minutes", fmt.Sprintf("after %d min", minutes))
	return strings.ReplaceAll(text, "Just 5 minutes", fmt.Sprintf("Just %d min", minutes))
}
func wrappedCardIndex(index, delta, count int) int {
	if count == 0 {
		return 0
	}
	return ((index+delta)%count + count) % count
}
