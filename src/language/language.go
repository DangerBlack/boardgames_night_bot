package language

import (
	"fmt"
	"os"
	"path"
	"strings"
)

var ErrorLanguageNotAvailable = fmt.Errorf("Language not available")

type LanguagePack struct {
	Languages []string
}

func BuildLanguagePack(dir string) (*LanguagePack, error) {
	l := &LanguagePack{}
	err := l.LoadLanguages(dir)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (l *LanguagePack) LoadLanguages(dir string) error {
	l.Languages = []string{}
	println("Loading available languages from:", path.Join(dir, "localization"))
	entries, err := os.ReadDir(path.Join(dir, "localization"))
	if err != nil {
		return ErrorLanguageNotAvailable

	}

	for _, e := range entries {
		parts := strings.Split(e.Name(), ".")
		if len(parts) > 1 {
			l.Languages = append(l.Languages, parts[1])
		}
	}

	return nil
}

func (l LanguagePack) HasLanguage(language string) bool {
	for _, lang := range l.Languages {
		if lang == language {
			return true
		}
	}

	return false
}
