package api

import (
	"boardgame-night-bot/src/mocks"
	"fmt"
	"log"
	"testing"
	"time"

	langpack "boardgame-night-bot/src/language"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/telebot.v3"
)

func BeforeEach() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	lp, err := langpack.BuildLanguagePack("../../../")
	if err != nil {
		log.Fatal(err)
	}

	for _, lang := range lp.Languages {
		log.Default().Printf("Loading language file: %s", lang)
		bundle.MustLoadMessageFile(fmt.Sprintf("../../../localization/active.%s.toml", lang))
	}

	return bundle
}

func TestCreateEvent(t *testing.T) {
	bundle := BeforeEach()
	db := mocks.NewMockDatabase()
	telegram := mocks.NewMockTelegramService()
	service := &Service{
		DB:             db,
		Bot:            telegram,
		LanguageBundle: bundle,
	}

	db.InsertEventFunc = func(id *string, chatID, userID int64, userName, name string, messageID *int64, location *string, startsAt *time.Time) (string, error) {
		if chatID != 12345 {
			t.Fatalf("Expected chatID 12345, got %d", chatID)
		}

		if userID != 67890 {
			t.Fatalf("Expected userID 67890, got %d", userID)
		}

		if userName != "testuser" {
			t.Fatalf("Expected userName 'testuser', got '%s'", userName)
		}

		if name != "Test Event" {
			t.Fatalf("Expected event name 'Test Event', got '%s'", name)
		}

		if location != nil {
			t.Fatalf("Expected location to be nil, got '%v'", location)
		}

		if startsAt != nil {
			t.Fatalf("Expected startsAt to be nil, got '%v'", startsAt)
		}

		return "mock-event-id", nil
	}

	responseTelegramID := int64(123456789)
	alignedTelegramMessageID := false
	db.UpdateEventMessageIDFunc = func(eventID string, messageID int64) error {
		if eventID != "mock-event-id" {
			t.Fatalf("Expected eventID 'mock-event-id', got '%s'", eventID)
		}
		if messageID != responseTelegramID {
			t.Fatalf("Expected messageID %d, got %d", responseTelegramID, messageID)
		}

		alignedTelegramMessageID = true
		return nil
	}

	isBoardGameInserted := false
	db.InsertBoardGameFunc = func(eventID string, id *string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, string, error) {
		isBoardGameInserted = true
		return 1, "mock-game-uuid", nil
	}

	isMessageSent := false
	telegram.SendFunc = func(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error) {
		isMessageSent = true
		return &telebot.Message{ID: int(responseTelegramID)}, nil
	}

	event, err := service.CreateEvent(12345, nil, nil, 67890, "testuser", "Test Event", nil, nil, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if event.ID != "mock-event-id" {
		t.Errorf("Expected event ID 'mock-event-id', got %s", event.ID)
	}

	if !isBoardGameInserted {
		t.Errorf("Expected board game to be inserted")
	}

	if !isMessageSent {
		t.Errorf("Expected message to be sent")
	}

	if !alignedTelegramMessageID {
		t.Errorf("Expected Telegram message ID to be aligned in database")
	}
}

func TestCreateEventWithLocationAndStartTime(t *testing.T) {
	bundle := BeforeEach()
	db := mocks.NewMockDatabase()
	service := &Service{
		DB:             db,
		Bot:            mocks.NewMockTelegramService(),
		LanguageBundle: bundle,
	}

	db.InsertEventFunc = func(id *string, chatID, userID int64, userName, name string, messageID *int64, location *string, startsAt *time.Time) (string, error) {
		if location == nil || *location != "Test Location" {
			t.Fatalf("Expected location 'Test Location', got '%v'", location)
		}

		if startsAt == nil || !startsAt.Equal(time.Time{}.Add(24*time.Hour)) {
			t.Fatalf("Expected startsAt '%v', got '%v'", time.Time{}.Add(24*time.Hour), startsAt)
		}

		return "mock-event-id", nil
	}

	isBoardGameInserted := false
	db.InsertBoardGameFunc = func(eventID string, id *string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, string, error) {
		isBoardGameInserted = true
		return 1, "mock-game-uuid", nil
	}

	location := "Test Location"
	startsAt := time.Time{}.Add(24 * time.Hour)
	_, err := service.CreateEvent(12345, nil, nil, 67890, "testuser", "Test Event with Location and Time", &location, &startsAt, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if isBoardGameInserted {
		t.Errorf("Did not expect board game to be inserted")
	}
}
