package models

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestFormatBGZeroCapacity(t *testing.T) {
	localizer := setupLocalizer()
	url := WebUrl{
		BaseUrl:       "http://example.com",
		BotMiniAppURL: "https://t.me/boardgame_night_bot",
	}

	// MaxPlayers=0 means zero capacity: room is always full and every participant is queued
	bg := BoardGame{
		ID:         1,
		UUID:       "test-uuid",
		Name:       "Test Game",
		MaxPlayers: 0,
		Participants: []Participant{
			{ID: 1, UserName: "Player1", IsTelegramUsername: true},
			{ID: 2, UserName: "Player2", IsTelegramUsername: true},
		},
	}

	event := Event{ID: "test-event"}
	msg, _, err := event.FormatBG(localizer, url, bg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Room is full
	if !strings.Contains(msg, "🚫") {
		t.Errorf("Expected room-full symbol 🚫, got:\n%s", msg)
	}

	// Every participant is queued
	if !strings.Contains(msg, "Player1 (queued") {
		t.Errorf("Expected Player1 to be queued, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Player2 (queued") {
		t.Errorf("Expected Player2 to be queued, got:\n%s", msg)
	}
}

func TestFormatBGUnlimitedPlayers(t *testing.T) {
	localizer := setupLocalizer()
	url := WebUrl{
		BaseUrl:       "http://example.com",
		BotMiniAppURL: "https://t.me/boardgame_night_bot",
	}

	// MaxPlayers=UnlimitedPlayers (-1): no cap, no queue, no full marker
	bg := BoardGame{
		ID:         1,
		UUID:       "test-uuid",
		Name:       "Test Game",
		MaxPlayers: UnlimitedPlayers,
		Participants: []Participant{
			{ID: 1, UserName: "Player1", IsTelegramUsername: true},
			{ID: 2, UserName: "Player2", IsTelegramUsername: true},
		},
	}

	event := Event{ID: "test-event"}
	msg, _, err := event.FormatBG(localizer, url, bg)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Room is not full
	if strings.Contains(msg, "🚫") {
		t.Errorf("Expected no room-full symbol, got:\n%s", msg)
	}

	// No participants are queued
	if strings.Contains(msg, "(queued") {
		t.Errorf("Expected no queued markers, got:\n%s", msg)
	}
}

func TestFormatMsgFullEvent(t *testing.T) {
	localizer := setupLocalizer()
	webUrl := WebUrl{
		BaseUrl:       "http://example.com",
		BotMiniAppURL: "https://t.me/boardgame_night_bot",
	}

	bggUrl := "https://boardgamegeek.com/boardgame/174430/gloomhaven"
	bggName := "Gloomhaven"
	location := "Via Roma 12"
	startsAt := mustParseTime("2026-06-01 20:00")

	event := Event{
		ID:       "test-event",
		ChatID:   12345,
		UserID:   1,
		UserName: "organizer",
		Location: &location,
		StartsAt: &startsAt,
		BoardGames: []BoardGame{
			{
				ID:         1,
				Name:       "Gloomhaven",
				MaxPlayers: 4,
				BggUrl:     &bggUrl,
				BggName:    &bggName,
				Participants: []Participant{
					{ID: 1, UserName: "alice", IsTelegramUsername: true},
					{ID: 2, UserName: "bob", IsTelegramUsername: true},
				},
			},
			{
				ID:         2,
				Name:       PLAYER_COUNTER,
				MaxPlayers: UnlimitedPlayers,
				Participants: []Participant{
					{ID: 3, UserName: "carol", IsTelegramUsername: false},
				},
			},
		},
	}

	msg, markup := event.FormatMsg(localizer, webUrl)

	// Event header
	if !strings.Contains(msg, "organizer") {
		t.Errorf("Expected organizer name in message, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Via Roma 12") {
		t.Errorf("Expected location in message, got:\n%s", msg)
	}
	if !strings.Contains(msg, "2026-06-01 20:00") {
		t.Errorf("Expected start time in message, got:\n%s", msg)
	}

	// BGG-linked game shows participant count
	if !strings.Contains(msg, "2/4") {
		t.Errorf("Expected participant count '2/4', got:\n%s", msg)
	}

	// Telegram usernames prefixed with @
	if !strings.Contains(msg, "@alice") {
		t.Errorf("Expected '@alice' in message, got:\n%s", msg)
	}

	// Non-telegram username has no @
	if strings.Contains(msg, "@carol") {
		t.Errorf("Expected 'carol' without @ prefix, got:\n%s", msg)
	}

	// PLAYER_COUNTER game shows localised "Join" label not raw constant
	if strings.Contains(msg, PLAYER_COUNTER) {
		t.Errorf("Expected PLAYER_COUNTER to be replaced by localised label, got:\n%s", msg)
	}

	// Buttons: one join per game + "not coming" + "add game"
	totalButtons := 0
	for _, row := range markup.InlineKeyboard {
		totalButtons += len(row)
	}
	if totalButtons != 4 { // join Gloomhaven + join PLAYER_COUNTER + not coming + add game
		t.Errorf("Expected 4 inline buttons, got %d", totalButtons)
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestFormatMsgEscapesUserText(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile("../../localization/active.en.toml")
	localizer := i18n.NewLocalizer(bundle, "en")

	loc := `<i>home</i>`
	e := Event{
		ID:       "e1",
		Name:     `<a href="https://evil.example">Night</a>`,
		UserName: "<b>x",
		Location: &loc,
		BoardGames: []BoardGame{{
			ID: 1, Name: `Catan <a href="https://evil.example">`, MaxPlayers: 4,
			Participants: []Participant{{UserID: -1, UserName: "<script>"}},
		}},
	}
	msg, _ := e.FormatMsg(localizer, WebUrl{})
	if strings.Contains(msg, "evil.example\">") || strings.Contains(msg, "<script>") || strings.Contains(msg, "<i>home") || strings.Contains(msg, "<b>x") {
		t.Fatalf("user text not escaped:\n%s", msg)
	}
	if !strings.Contains(msg, "&lt;script&gt; "+ExternalUserMarker) {
		t.Fatalf("expected escaped external participant, got:\n%s", msg)
	}
}

func TestFormatMsgShareLinkInText(t *testing.T) {
	localizer := setupLocalizer()
	web := WebUrl{BaseUrl: "https://bgn.example/", BotMiniAppURL: "https://t.me/bot/app"}
	e := Event{ID: "e1", Name: "Night", AllowExternal: true}

	msg, markup := e.FormatMsg(localizer, web)
	if !strings.Contains(msg, `🔗 <a href="https://bgn.example/events/e1">Event link</a>`) {
		t.Fatalf("expected share link in text, got:\n%s", msg)
	}
	for _, row := range markup.InlineKeyboard {
		for _, b := range row {
			if strings.Contains(b.URL, "/events/e1") || strings.Contains(b.URL, "t.me/share") {
				t.Fatalf("share link should not be a button: %+v", b)
			}
		}
	}

	e.AllowExternal = false
	if msg, _ := e.FormatMsg(localizer, web); strings.Contains(msg, "/events/e1") {
		t.Fatalf("closed event must not show the share link:\n%s", msg)
	}
}
