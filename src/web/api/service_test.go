package api

import (
	"boardgame-night-bot/src/mocks"
	"boardgame-night-bot/src/models"
	"context"
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

func BeforeEach() Service {
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

	db := mocks.NewMockDatabase()
	telegram := mocks.NewMockTelegramService()
	bggMock := mocks.NewMockBGGService()
	service := &Service{
		DB:             db,
		Bot:            telegram,
		LanguageBundle: bundle,
		BGG:            bggMock,
		Url: models.WebUrl{
			BaseUrl:       "http://example.com",
			BotMiniAppURL: "https://t.me/boardgame_night_bot",
		},
	}

	return *service
}

func TestCreateEvent(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	telegram := service.Bot.(*mocks.MockTelegramService)
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
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)

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

func TestDeleteEvent(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	telegram := service.Bot.(*mocks.MockTelegramService)

	isEventDeleted := false

	db.DeleteEventFunc = func(id string) error {
		if id != "mock-event-id" {
			t.Fatalf("Expected event ID 'mock-event-id', got '%s'", id)
		}
		isEventDeleted = true
		return nil
	}

	eventMessageID := int64(11111)
	db.SelectEventByEventIDFunc = func(eventID string) (*models.Event, error) {
		return &models.Event{
			ID:         eventID,
			ChatID:     12345,
			UserID:     67890,
			UserName:   "test",
			MessageID:  &eventMessageID,
			Name:       "event",
			BoardGames: []models.BoardGame{},
			Locked:     false,
		}, nil
	}

	isMessageSent := false
	telegram.SendFunc = func(to telebot.Recipient, what interface{}, opts ...interface{}) (*telebot.Message, error) {
		isMessageSent = true
		return &telebot.Message{}, nil
	}

	isMessageDeleted := false
	telegram.DeleteFunc = func(msg telebot.Editable) error {
		isMessageDeleted = true
		if msg.(*telebot.Message).ID != int(eventMessageID) {
			t.Fatalf("Expected message ID %d, got %d", eventMessageID, msg.(*telebot.Message).ID)
		}
		return nil
	}

	userID := int64(67890)
	err := service.DeleteEvent("mock-event-id", &userID, "testuser")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !isEventDeleted {
		t.Fatalf("Expected event to be deleted")
	}

	if !isMessageSent {
		t.Fatalf("Expected Telegram notification to be sent")
	}

	if !isMessageDeleted {
		t.Fatalf("Expected Telegram event message to be deleted")
	}
}

func TestCreateGame(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	bggMock := service.BGG.(*mocks.MockBGGService)
	// telegram := service.Bot.(*mocks.MockTelegramService)

	isGameInserted := false
	db.InsertBoardGameFunc = func(eventID string, id *string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, string, error) {
		if eventID != "mock-event-id" {
			t.Fatalf("Expected eventID 'mock-event-id', got '%s'", eventID)
		}

		if name != "Test Game" {
			t.Fatalf("Expected game name 'Test Game', got '%s'", name)
		}

		if maxPlayers != 4 {
			t.Fatalf("Expected maxPlayers 4, got %d", maxPlayers)
		}

		isGameInserted = true
		return 1, "mock-game-uuid", nil
	}

	eventMessageID := int64(11111)
	db.SelectEventByEventIDFunc = func(eventID string) (*models.Event, error) {
		return &models.Event{
			ID:        eventID,
			ChatID:    12345,
			UserID:    67890,
			UserName:  "test",
			MessageID: &eventMessageID,
			Name:      "event",
			BoardGames: []models.BoardGame{{
				ID:         1,
				UUID:       "mock-game-uuid",
				Name:       "Test Game",
				MaxPlayers: 4,
			},
			},
			Locked: false,
		}, nil
	}

	bggMock.ExtractGameInfoFunc = func(ctx context.Context, id int64, gameName string) (*int, *string, *string, *string, error) {
		if id != 0 {
			t.Fatalf("Expected BGG ID 0, got %d", id)
		}

		if gameName != "Test Game" {
			t.Fatalf("Expected game name 'Test Game', got '%s'", gameName)
		}

		// bggID := int64(123456)
		bggName := "BGG Test Game"
		bggUrl := "https://boardgamegeek.com/boardgame/123456"
		bggImageUrl := "https://boardgamegeek.com/image/123456.jpg"
		maxPlayers := 4

		return &maxPlayers, &bggName, &bggUrl, &bggImageUrl, nil
	}

	maxPlayer := 4
	_, bg, err := service.CreateGame("mock-event-id", nil, 123456, "Test Game", &maxPlayer, nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if bg == nil {
		t.Fatalf("Expected board game to be created, got nil")
	}

	if bg.UUID != "mock-game-uuid" {
		t.Errorf("Expected game UUID 'mock-game-uuid', got '%s'", bg.UUID)
	}

	if !isGameInserted {
		t.Errorf("Expected game to be inserted")
	}
}
