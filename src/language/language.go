package language

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
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
	log.Default().Printf("Loading available languages from: %s", path.Join(dir, "localization"))
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

// ValidateKeyParity checks that every key present in the reference language file
// (active.en.toml) also exists in every other language file. It logs a fatal
// error listing all missing keys so the application fails fast at startup
// rather than panicking at runtime when a user triggers a missing translation.
func ValidateKeyParity(dir string, languages []string) {
	referenceKeys := tomlKeys(path.Join(dir, "localization", "active.en.toml"))
	if referenceKeys == nil {
		log.Fatal("localization: could not parse reference file active.en.toml")
	}

	allOK := true
	for _, lang := range languages {
		if lang == "en" {
			continue
		}
		filePath := path.Join(dir, "localization", fmt.Sprintf("active.%s.toml", lang))
		keys := tomlKeys(filePath)
		if keys == nil {
			log.Printf("localization: could not parse %s", filePath)
			allOK = false
			continue
		}
		for key := range referenceKeys {
			if _, ok := keys[key]; !ok {
				log.Printf("localization: key %q missing from active.%s.toml", key, lang)
				allOK = false
			}
		}
	}

	if !allOK {
		log.Fatal("localization: key parity check failed — fix missing keys before starting")
	}
}

// tomlKeys parses a TOML file and returns a map of its top-level keys.
func tomlKeys(filePath string) map[string]struct{} {
	var raw map[string]interface{}
	if _, err := toml.DecodeFile(filePath, &raw); err != nil {
		return nil
	}
	keys := make(map[string]struct{}, len(raw))
	for k := range raw {
		keys[k] = struct{}{}
	}
	return keys
}

func (l LanguagePack) HasLanguage(language string) bool {
	for _, lang := range l.Languages {
		if lang == language {
			return true
		}
	}

	return false
}
