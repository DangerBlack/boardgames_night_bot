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
	"github.com/DangerBlack/gobgg"
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

	expectToExtractGameInfo := false
	bggMock.ExtractGameInfoFunc = func(ctx context.Context, id int64, gameName string) (*models.BggInfo, error) {
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
		expectToExtractGameInfo = true

		return &models.BggInfo{
			Name:       &bggName,
			Url:        &bggUrl,
			ImageUrl:   &bggImageUrl,
			MaxPlayers: &maxPlayers,
		}, nil
	}

	expectToSearchGame := false
	bggMock.SearchFunc = func(ctx context.Context, query string, setter []gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error) {

		if query != "Test Game" {
			t.Fatalf("Expected search query 'Test Game', got '%s'", query)
		}

		expectToSearchGame = true

		return []gobgg.SearchResult{
			{
				ID:   123456,
				Name: "BGG Test Game",
				Type: "boardgame",
			},
		}, nil
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

	if expectToExtractGameInfo {
		t.Errorf("Did not expect ExtractGameInfo to be called")
	}

	if !expectToSearchGame {
		t.Errorf("Expected Search to be called")
	}
}

func TestCreateGameWithoutBGGID(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	bggMock := service.BGG.(*mocks.MockBGGService)

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

		if bggUrl == nil {
			t.Fatalf("Expected BGG URL to be set, got nil")
		}

		if bggID == nil {
			t.Fatalf("Expected BGG ID to be set, got nil")
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

	bggID := int64(888888)
	bggMock.ExtractGameInfoFunc = func(ctx context.Context, id int64, gameName string) (*models.BggInfo, error) {
		if id != bggID {
			t.Fatalf("Expected BGG ID %d, got %d", bggID, id)
		}

		if gameName != "Test Game" {
			t.Fatalf("Expected game name 'Test Game', got '%s'", gameName)
		}

		// bggID := int64(123456)
		bggName := "BGG Test Game"
		bggUrl := "https://boardgamegeek.com/boardgame/123456"
		bggImageUrl := "https://boardgamegeek.com/image/123456.jpg"
		maxPlayers := 4

		return &models.BggInfo{
			Name:       &bggName,
			Url:        &bggUrl,
			ImageUrl:   &bggImageUrl,
			MaxPlayers: &maxPlayers,
		}, nil
	}

	maxPlayer := 4
	bggUrl := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d/azul", bggID)
	_, bg, err := service.CreateGame("mock-event-id", nil, 123456, "Test Game", &maxPlayer, &bggUrl)
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

func TestCreateGameRespectMaxPlayer(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	bggMock := service.BGG.(*mocks.MockBGGService)

	requestedMaxPlayer := 8
	isGameInserted := false
	db.InsertBoardGameFunc = func(eventID string, id *string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, string, error) {
		if maxPlayers != requestedMaxPlayer {
			t.Fatalf("Expected maxPlayers %d, got %d", requestedMaxPlayer, maxPlayers)
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

	bggID := int64(888888)
	bggMock.ExtractGameInfoFunc = func(ctx context.Context, id int64, gameName string) (*models.BggInfo, error) {
		if id != bggID {
			t.Fatalf("Expected BGG ID %d, got %d", bggID, id)
		}

		if gameName != "Test Game" {
			t.Fatalf("Expected game name 'Test Game', got '%s'", gameName)
		}

		// bggID := int64(123456)
		bggName := "BGG Test Game"
		bggUrl := "https://boardgamegeek.com/boardgame/123456"
		bggImageUrl := "https://boardgamegeek.com/image/123456.jpg"
		maxPlayers := 4

		return &models.BggInfo{
			Name:       &bggName,
			Url:        &bggUrl,
			ImageUrl:   &bggImageUrl,
			MaxPlayers: &maxPlayers,
		}, nil
	}

	bggUrl := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d/azul", bggID)
	_, bg, err := service.CreateGame("mock-event-id", nil, 123456, "Test Game", &requestedMaxPlayer, &bggUrl)
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

func TestCreateGameRespectMaxPlayerNoUrl(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	bggMock := service.BGG.(*mocks.MockBGGService)

	requestedMaxPlayer := 8
	isGameInserted := false
	db.InsertBoardGameFunc = func(eventID string, id *string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, string, error) {
		if maxPlayers != requestedMaxPlayer {
			t.Fatalf("Expected maxPlayers %d, got %d", requestedMaxPlayer, maxPlayers)
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

	expectToSearchGame := false
	bggMock.SearchFunc = func(ctx context.Context, query string, setter []gobgg.SearchOptionSetter) ([]gobgg.SearchResult, error) {

		if query != "Test Game" {
			t.Fatalf("Expected search query 'Test Game', got '%s'", query)
		}

		expectToSearchGame = true

		return []gobgg.SearchResult{
			{
				ID:   123456,
				Name: "BGG Test Game",
				Type: "boardgame",
			},
		}, nil
	}

	bggMock.GetThingsFunc = func(ctx context.Context, setters []gobgg.GetOptionSetter) ([]gobgg.ThingResult, error) {
		maxPlayers := 4
		bggName := "BGG Test Game"
		bggImageUrl := "https://boardgamegeek.com/image/123456.jpg"

		return []gobgg.ThingResult{
			{
				ID:         123456,
				Name:       bggName,
				Image:      bggImageUrl,
				MaxPlayers: maxPlayers,
			},
		}, nil
	}

	_, bg, err := service.CreateGame("mock-event-id", nil, 123456, "Test Game", &requestedMaxPlayer, nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if bg == nil {
		t.Fatalf("Expected board game to be created, got nil")
	}

	if bg.UUID != "mock-game-uuid" {
		t.Errorf("Expected game UUID 'mock-game-uuid', got '%s'", bg.UUID)
	}

	if !expectToSearchGame {
		t.Errorf("Expected Search to be called")
	}

	if !isGameInserted {
		t.Errorf("Expected game to be inserted")
	}
}

func TestUpdateGame(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)

	requestedMaxPlayer := 6
	isGameUpdated := false
	db.UpdateBoardGameBGGInfoByIDFunc = func(ID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error {
		if ID != 123456 {
			t.Fatalf("Expected game ID 123456, got %d", ID)
		}

		if maxPlayers != requestedMaxPlayer {
			t.Fatalf("Expected maxPlayers %d, got %d", requestedMaxPlayer, maxPlayers)
		}
		isGameUpdated = true
		return nil
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
				ID:         123456,
				UUID:       "mock-game-uuid",
				Name:       "Test Game",
				MaxPlayers: 4,
			},
			},
			Locked: false,
		}, nil
	}

	_, _, err := service.UpdateGame("mock-event-id", 123456, 891011, models.UpdateGameRequest{
		MaxPlayers: &requestedMaxPlayer,
	})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !isGameUpdated {
		t.Errorf("Expected game to be updated")
	}
}

func TestUpdateGameWithBGG(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	bggMock := service.BGG.(*mocks.MockBGGService)

	requestedMaxPlayer := 6
	expectMaxPlayer := 6
	bggNewUrl := "https://boardgamegeek.com/boardgame/999999/new-game"
	isGameUpdated := false
	db.UpdateBoardGameBGGInfoByIDFunc = func(ID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error {
		if ID != 123456 {
			t.Fatalf("Expected game ID 123456, got %d", ID)
		}

		if maxPlayers != expectMaxPlayer {
			t.Fatalf("Expected maxPlayers %d, got %d", expectMaxPlayer, maxPlayers)
		}
		isGameUpdated = true
		return nil
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
				ID:         123456,
				UUID:       "mock-game-uuid",
				Name:       "Test Game",
				MaxPlayers: 4,
			},
			},
			Locked: false,
		}, nil
	}

	bggID := int64(999999)
	bggMock.ExtractGameInfoFunc = func(ctx context.Context, id int64, gameName string) (*models.BggInfo, error) {
		if id != bggID {
			t.Fatalf("Expected BGG ID %d, got %d", bggID, id)
		}

		if gameName != "Test Game" {
			t.Fatalf("Expected game name 'Test Game', got '%s'", gameName)
		}

		bggName := "BGG Test Game"
		bggUrl := "https://boardgamegeek.com/boardgame/123456"
		bggImageUrl := "https://boardgamegeek.com/image/123456.jpg"
		maxPlayers := 4

		return &models.BggInfo{
			Name:       &bggName,
			Url:        &bggUrl,
			ImageUrl:   &bggImageUrl,
			MaxPlayers: &maxPlayers,
		}, nil
	}

	_, _, err := service.UpdateGame("mock-event-id", 123456, 891011, models.UpdateGameRequest{
		MaxPlayers: &requestedMaxPlayer,
		BggUrl:     &bggNewUrl,
	})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !isGameUpdated {
		t.Errorf("Expected game to be updated")
	}
}

func TestDeleteGame(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	telegram := service.Bot.(*mocks.MockTelegramService)

	// Setup mock event and game
	eventMessageID := int64(11111)
	eventID := "mock-event-id"
	gameUUID := "mock-game-uuid"
	userID := int64(67890)
	username := "testuser"
	gameName := "Test Game"
	isGameDeleted := false

	db.SelectEventByEventIDFunc = func(id string) (*models.Event, error) {
		return &models.Event{
			ID:        eventID,
			ChatID:    12345,
			UserID:    userID,
			UserName:  username,
			MessageID: &eventMessageID,
			Name:      "event",
			BoardGames: []models.BoardGame{{
				ID:         123456,
				UUID:       gameUUID,
				Name:       gameName,
				MaxPlayers: 4,
			}},
			Locked: false,
		}, nil
	}
	db.DeleteBoardGameByIDFunc = func(id string) error {
		if id != gameUUID {
			t.Fatalf("Expected game UUID %s, got %s", gameUUID, id)
		}
		isGameDeleted = true
		return nil
	}

	isExpectedToSend := false
	telegram.SendFunc = func(to telebot.Recipient, what interface{}, options ...interface{}) (*telebot.Message, error) {
		isExpectedToSend = true
		return &telebot.Message{ID: 1}, nil
	}

	// Act
	event, game, err := service.DeleteGame(eventID, gameUUID, userID, username)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if event == nil {
		t.Fatalf("Expected event, got nil")
	}
	if game == nil {
		t.Fatalf("Expected game, got nil")
	}
	if !isGameDeleted {
		t.Fatalf("Expected game to be deleted")
	}
	if game.UUID != gameUUID {
		t.Fatalf("Expected game UUID %s, got %s", gameUUID, game.UUID)
	}
	if !isExpectedToSend {
		t.Fatalf("Expected Telegram message to be sent")
	}
}

func TestAddPlayer(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	telegram := service.Bot.(*mocks.MockTelegramService)

	eventID := "mock-event-id"
	gameID := int64(123456)
	userID := int64(67890)
	username := "testuser"
	participantID := "mock-participant-id"
	isParticipantInserted := false

	db.InsertParticipantFunc = func(id *string, eID string, gID, uID int64, uName string) (string, error) {
		if eID != eventID {
			t.Fatalf("Expected eventID %s, got %s", eventID, eID)
		}
		if gID != gameID {
			t.Fatalf("Expected gameID %d, got %d", gameID, gID)
		}
		if uID != userID {
			t.Fatalf("Expected userID %d, got %d", userID, uID)
		}
		if uName != username {
			t.Fatalf("Expected username %s, got %s", username, uName)
		}
		isParticipantInserted = true
		return participantID, nil
	}

	messageID := int64(11111)
	db.SelectEventByEventIDFunc = func(eventID string) (*models.Event, error) {
		return &models.Event{
			ID:        eventID,
			ChatID:    12345,
			UserID:    userID,
			UserName:  username,
			MessageID: &messageID,
			Name:      "event",
			BoardGames: []models.BoardGame{{
				ID:         gameID,
				UUID:       "mock-game-uuid",
				Name:       "Test Game",
				MaxPlayers: 4,
			}},
			Locked: false,
		}, nil
	}

	expectMessageUpdate := false
	telegram.EditFunc = func(msg telebot.Editable, what interface{}, opts ...interface{}) (*telebot.Message, error) {
		expectMessageUpdate = true
		return &telebot.Message{ID: 1}, nil
	}

	pid, _, _, err := service.AddPlayer(nil, eventID, gameID, userID, username)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if pid != participantID {
		t.Fatalf("Expected participantID %s, got %s", participantID, pid)
	}
	if !isParticipantInserted {
		t.Fatalf("Expected participant to be inserted")
	}
	if !expectMessageUpdate {
		t.Fatalf("Expected Telegram message to be updated")
	}
}

func TestDeletePlayer(t *testing.T) {
	service := BeforeEach()
	db := service.DB.(*mocks.MockDatabase)
	telegram := service.Bot.(*mocks.MockTelegramService)

	eventID := "mock-event-id"
	gameID := int64(123456)
	userID := int64(67890)
	username := "testuser"
	participantID := "mock-participant-id"
	isParticipantDeleted := false

	db.RemoveParticipantFunc = func(eID string, uID int64) (string, int64, error) {
		if eID != eventID {
			t.Fatalf("Expected eventID %s, got %s", eventID, eID)
		}

		if uID != userID {
			t.Fatalf("Expected userID %d, got %d", userID, uID)
		}
		isParticipantDeleted = true
		return participantID, gameID, nil
	}

	messageID := int64(11111)
	db.SelectEventByEventIDFunc = func(eventID string) (*models.Event, error) {
		return &models.Event{
			ID:        eventID,
			ChatID:    12345,
			UserID:    userID,
			UserName:  username,
			MessageID: &messageID,
			Name:      "event",
			BoardGames: []models.BoardGame{{
				ID:         gameID,
				UUID:       "mock-game-uuid",
				Name:       "Test Game",
				MaxPlayers: 4,
			}},
			Locked: false,
		}, nil
	}

	expectMessageUpdate := false
	telegram.EditFunc = func(msg telebot.Editable, what interface{}, opts ...interface{}) (*telebot.Message, error) {
		expectMessageUpdate = true
		return &telebot.Message{ID: 1}, nil
	}

	_, _, _, err := service.DeletePlayer(eventID, userID)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !isParticipantDeleted {
		t.Fatalf("Expected participant to be inserted")
	}
	if !expectMessageUpdate {
		t.Fatalf("Expected Telegram message to be updated")
	}
}
