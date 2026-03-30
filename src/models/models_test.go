package models

import (
	"fmt"
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

	// Verify queued players are marked correctly
	expectedQueued1 := "@Player5 (queued 1)"
	expectedQueued2 := "@Player6 (queued 2)"

	if !contains(msg, expectedQueued1) {
		t.Errorf("Expected message to contain '%s', got:\n%s", expectedQueued1, msg)
	}

	if !contains(msg, expectedQueued2) {
		t.Errorf("Expected message to contain '%s', got:\n%s", expectedQueued2, msg)
	}

	// Verify regular players don't have queued marker
	if contains(msg, "@Player1 (queued") {
		t.Errorf("Expected @Player1 to NOT have queued marker, got:\n%s", msg)
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
	if contains(msg, "(queued") {
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
	if contains(msg, "(queued") {
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

	// Verify queued player with non-telegram username
	expectedQueued := "Player4 (queued 1)"
	if !contains(msg, expectedQueued) {
		t.Errorf("Expected message to contain '%s', got:\n%s", expectedQueued, msg)
	}

	// Verify telegram username with queue
	expectedQueuedTG := "@Player3"
	if !contains(msg, expectedQueuedTG) {
		t.Errorf("Expected message to contain '%s', got:\n%s", expectedQueuedTG, msg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
