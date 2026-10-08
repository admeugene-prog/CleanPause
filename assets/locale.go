package assets

import (
	"embed"
	"encoding/json"
)

//go:embed locales/*.json
var Files embed.FS
var catalogs = func() map[string]map[string]string {
	all := map[string]map[string]string{}
	for _, language := range []string{"ru", "en"} {
		b, err := Files.ReadFile("locales/" + language + ".json")
		if err != nil {
			panic(err)
		}
		var m map[string]string
		if err = json.Unmarshal(b, &m); err != nil {
			panic(err)
		}
		all[language] = m
	}
	return all
}()
var language = "ru"

func SetLanguage(code string) {
	if _, ok := catalogs[code]; ok {
		language = code
	}
}
func Language() string { return language }

func Text(key string) string {
	if s, ok := catalogs[language][key]; ok {
		return s
	}
	if s, ok := catalogs["ru"][key]; ok {
		return s
	}
	return key
}

func Translate(text string) string {
	if language == "ru" {
		return text
	}
	if s, ok := catalogs[language][text]; ok {
		return s
	}
	for key, value := range catalogs["ru"] {
		if value == text {
			return Text(key)
		}
	}
	return text
}
