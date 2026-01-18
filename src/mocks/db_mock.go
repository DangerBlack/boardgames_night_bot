package mocks

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"time"
)

type MockDatabase struct {
	InsertEventFunc                func(id *string, chatID, userID int64, userName, name string, messageID *int64, location *string, startsAt *time.Time) (string, error)
	InsertBoardGameFunc            func(eventID string, id *string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, string, error)
	UpdateBoardGameBGGInfoByIDFunc func(ID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error
	UpdateEventMessageIDFunc       func(eventID string, messageID int64) error
	DeleteBoardGameByIDFunc        func(ID string) error
	SelectEventByEventIDFunc       func(eventID string) (*models.Event, error)
	DeleteEventFunc                func(id string) error
	InsertParticipantFunc          func(id *string, eventID string, boardgameID, userID int64, userName string, isTelegramUsername bool) (string, error)
	RemoveParticipantFunc          func(eventID string, userID int64) (string, int64, error)
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{}
}

func (m *MockDatabase) CreateTables() {}

func (m *MockDatabase) Close() {}

func (m *MockDatabase) InsertEvent(id *string, chatID, userID int64, userName, name string, messageID *int64, location *string, startsAt *time.Time) (string, error) {
	if m.InsertEventFunc != nil {
		return m.InsertEventFunc(id, chatID, userID, userName, name, messageID, location, startsAt)
	}
	return "mock-event-id", nil
}

func (m *MockDatabase) SelectEvent(chatID int64) (*models.Event, error) {
	return &models.Event{ID: "mock-event-id", Name: "Mock Event", ChatID: chatID}, nil
}

func (m *MockDatabase) SelectEventByEventID(eventID string) (*models.Event, error) {
	if m.SelectEventByEventIDFunc != nil {
		return m.SelectEventByEventIDFunc(eventID)
	}
	return &models.Event{ID: eventID, Name: "Mock Event", ChatID: 12345}, nil
}

func (m *MockDatabase) DeleteEvent(id string) error {
	if m.DeleteEventFunc != nil {
		return m.DeleteEventFunc(id)
	}
	return nil
}

func (m *MockDatabase) InsertBoardGame(eventID string, id *string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, string, error) {
	if m.InsertBoardGameFunc != nil {
		return m.InsertBoardGameFunc(eventID, id, name, maxPlayers, bggID, bggName, bggUrl, bggImageUrl)
	}
	return 1, "mock-game-uuid", nil
}

func (m *MockDatabase) UpdateEventMessageID(eventID string, messageID int64) error {
	if m.UpdateEventMessageIDFunc != nil {
		return m.UpdateEventMessageIDFunc(eventID, messageID)
	}
	return nil
}
func (m *MockDatabase) UpdateBoardGameBGGInfoByID(ID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error {
	if m.UpdateBoardGameBGGInfoByIDFunc != nil {
		return m.UpdateBoardGameBGGInfoByIDFunc(ID, maxPlayers, bggID, bggName, bggUrl, bggImageUrl)
	}
	return nil
}

func (m *MockDatabase) InsertParticipant(id *string, eventID string, boardgameID, userID int64, userName string, isTelegramUsername bool) (string, error) {
	if m.InsertParticipantFunc != nil {
		return m.InsertParticipantFunc(id, eventID, boardgameID, userID, userName, isTelegramUsername)
	}
	return "mock-participant-uuid", nil
}

func (m *MockDatabase) RemoveParticipant(eventID string, userID int64) (string, int64, error) {
	if m.RemoveParticipantFunc != nil {
		return m.RemoveParticipantFunc(eventID, userID)
	}
	return "mock-participant-uuid", 0, nil
}

func (m *MockDatabase) HasBoardGameWithMessageID(messageID int64) bool {
	return true
}

func (m *MockDatabase) SelectGameIDByGameUUID(gameUUID string) (int64, error) {
	return 1, nil
}

func (m *MockDatabase) SelectGameUUIDByGameID(gameID int64) (string, error) {
	return "mock-game-uuid", nil
}

func (m *MockDatabase) DeleteBoardGameByID(ID string) error {
	if m.DeleteBoardGameByIDFunc != nil {
		return m.DeleteBoardGameByIDFunc(ID)
	}
	return nil
}

func (m *MockDatabase) InsertChat(chatID int64, language *string, location *string, timezone *string) error {
	return nil
}

func (m *MockDatabase) GetPreferredLanguage(chatID int64) string {
	return "en"
}

func (m *MockDatabase) GetDefaultTimezoneLocation(chatID int64) *time.Location {
	loc, _ := time.LoadLocation("UTC")
	return loc
}

func (m *MockDatabase) InsertWebhook(chatID int64, threadID *int64, url, secret string) (*int64, *string, error) {
	id := int64(1)
	uuid := "mock-webhook-uuid"
	return &id, &uuid, nil
}

func (m *MockDatabase) RemoveWebhook(webhookID int64) error {
	return nil
}

func (m *MockDatabase) GetWebhooksByChatID(chatID int64) ([]models.Webhook, error) {
	return []models.Webhook{}, nil
}

func (m *MockDatabase) GetWebhookByWebhookID(webhookID string) (*models.Webhook, error) {
	return &models.Webhook{ID: 1, UUID: "mock-webhook-uuid", ChatID: 123, Url: "mock-url", Secret: "mock-secret"}, nil
}

var _ database.DatabaseService = &MockDatabase{}
