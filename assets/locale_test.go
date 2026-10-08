package assets

import "testing"

func TestEnglishCatalogAndSwitch(t *testing.T) {
	previous := Language()
	defer SetLanguage(previous)
	for key := range catalogs["ru"] {
		if catalogs["en"][key] == "" {
			t.Fatal("missing English key", key)
		}
	}
	SetLanguage("en")
	if Text("exit") != "Exit" || Translate("Расписание") != "Schedule" || Translate("Начать очистку") != "Start cleaning" {
		t.Fatal("translation failed")
	}
	SetLanguage("ru")
	if Text("exit") != "Выход" || Translate("Расписание") != "Расписание" {
		t.Fatal("Russian fallback failed")
	}
}
