package models

import (
	"fmt"
	"strings"
	"testing"

	langpack "boardgame-night-bot/src/language"
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func setupLocalizer() *i18n.Localizer {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	lp, err := langpack.BuildLanguagePack("../..")
	if err != nil {
		panic(err)
	}

	for _, lang := range lp.Languages {
		bundle.MustLoadMessageFile(fmt.Sprintf("../../localization/active.%s.toml", lang))
	}

	return i18n.NewLocalizer(bundle, "en")
}

func TestFormatBGWithQueue(t *testing.T) {
	localizer := setupLocalizer()
	url := WebUrl{
		BaseUrl:       "http://example.com",
		BotMiniAppURL: "https://t.me/boardgame_night_bot",
	}

	// Test case: Game with max 4 players, 6 participants (2 queued)
	bg := BoardGame{
		ID:         1,
		UUID:       "test-uuid",
		Name:       "Test Game",
		MaxPlayers: 4,
		Participants: []Participant{
			{ID: 1, UserName: "Player1", IsTelegramUsername: true},
			{ID: 2, UserName: "Player2", IsTelegramUsername: true},
			{ID: 3, UserName: "Player3", IsTelegramUsername: true},
			{ID: 4, UserName: "Player4", IsTelegramUsername: true},
			{ID: 5, UserName: "Player5", IsTelegramUsername: true}, // queued 1
			{ID: 6, UserName: "Player6", IsTelegramUsername: true}, // queued 2
		},
	}

	event := Event{ID: "test-event"}
	msg, _, err := event.FormatBG(localizer, url, bg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify queued players are marked correctly (checking for "queued" keyword which is in all languages)
	if !strings.Contains(msg, "Player5 (queued") {
		t.Errorf("Expected message to contain 'Player5 (queued...', got:\n%s", msg)
	}

	if !strings.Contains(msg, "Player6 (queued") {
		t.Errorf("Expected message to contain 'Player6 (queued...', got:\n%s", msg)
	}

	// Verify regular players don't have queued marker
	if strings.Contains(msg, "Player1 (queued") {
		t.Errorf("Expected Player1 to NOT have queued marker, got:\n%s", msg)
	}
}

func TestFormatBGNoQueue(t *testing.T) {
	localizer := setupLocalizer()
	url := WebUrl{
		BaseUrl:       "http://example.com",
		BotMiniAppURL: "https://t.me/boardgame_night_bot",
	}

	// Test case: Game with max 4 players, 3 participants (no queue)
	bg := BoardGame{
		ID:         1,
		UUID:       "test-uuid",
		Name:       "Test Game",
		MaxPlayers: 4,
		Participants: []Participant{
			{ID: 1, UserName: "Player1", IsTelegramUsername: true},
			{ID: 2, UserName: "Player2", IsTelegramUsername: true},
			{ID: 3, UserName: "Player3", IsTelegramUsername: true},
		},
	}

	event := Event{ID: "test-event"}
	msg, _, err := event.FormatBG(localizer, url, bg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify no queued markers
	if strings.Contains(msg, "(queued") {
		t.Errorf("Expected message to NOT contain any queued markers, got:\n%s", msg)
	}
}

func TestFormatBGExactCapacity(t *testing.T) {
	localizer := setupLocalizer()
	url := WebUrl{
		BaseUrl:       "http://example.com",
		BotMiniAppURL: "https://t.me/boardgame_night_bot",
	}

	// Test case: Game with max 4 players, exactly 4 participants (no queue)
	bg := BoardGame{
		ID:         1,
		UUID:       "test-uuid",
		Name:       "Test Game",
		MaxPlayers: 4,
		Participants: []Participant{
			{ID: 1, UserName: "Player1", IsTelegramUsername: true},
			{ID: 2, UserName: "Player2", IsTelegramUsername: true},
			{ID: 3, UserName: "Player3", IsTelegramUsername: true},
			{ID: 4, UserName: "Player4", IsTelegramUsername: true},
		},
	}

	event := Event{ID: "test-event"}
	msg, _, err := event.FormatBG(localizer, url, bg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify no queued markers
	if strings.Contains(msg, "(queued") {
		t.Errorf("Expected message to NOT contain any queued markers, got:\n%s", msg)
	}
}

func TestFormatBGWithNonTelegramUsernames(t *testing.T) {
	localizer := setupLocalizer()
	url := WebUrl{
		BaseUrl:       "http://example.com",
		BotMiniAppURL: "https://t.me/boardgame_night_bot",
	}

	// Test case: Mix of telegram and non-telegram usernames with queue
	bg := BoardGame{
		ID:         1,
		UUID:       "test-uuid",
		Name:       "Test Game",
		MaxPlayers: 3,
		Participants: []Participant{
			{ID: 1, UserName: "Player1", IsTelegramUsername: true},
			{ID: 2, UserName: "Player2", IsTelegramUsername: false},
			{ID: 3, UserName: "Player3", IsTelegramUsername: true},
			{ID: 4, UserName: "Player4", IsTelegramUsername: false}, // queued 1
		},
	}

	event := Event{ID: "test-event"}
	msg, _, err := event.FormatBG(localizer, url, bg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify queued player with non-telegram username (checking for "queued" keyword)
	if !strings.Contains(msg, "Player4 (queued") {
		t.Errorf("Expected message to contain 'Player4 (queued...', got:\n%s", msg)
	}

	// Verify telegram username without queued marker
	if !strings.Contains(msg, "@Player3") {
		t.Errorf("Expected message to contain '@Player3', got:\n%s", msg)
	}
}
