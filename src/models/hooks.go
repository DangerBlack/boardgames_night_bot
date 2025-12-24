package models

import "time"

type HookWebhookType string

const (
	HookWebhookTypeNewEvent          HookWebhookType = "new_event"
	HookWebhookTypeDeleteEvent       HookWebhookType = "delete_event"
	HookWebhookTypeNewGame           HookWebhookType = "new_game"
	HooksWebhookTypeUpdateGame       HookWebhookType = "update_game"
	HookWebhookTypeDeleteGame        HookWebhookType = "delete_game"
	HookWebhookTypeAddParticipant    HookWebhookType = "add_participant"
	HookWebhookTypeRemoveParticipant HookWebhookType = "remove_participant"
)

type HookWebhookEnvelope struct {
	Type HookWebhookType `json:"type"`
	Data any             `json:"data"`
}

// --- Event payloads ---
type HookNewEventPayload struct {
	ID        string     `json:"id"`
	ChatID    int64      `json:"chat_id"`
	UserID    int64      `json:"user_id"`
	UserName  string     `json:"user_name"`
	Name      string     `json:"name"`
	MessageID *int64     `json:"message_id"`
	Location  *string    `json:"location"`
	StartsAt  *time.Time `json:"starts_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type HookDeleteEventPayload struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UserName  string `json:"user_name"`
	DeletedAt string `json:"deleted_at"`
}

// --- Game payloads ---
type HookBGGInfo struct {
	IsSet    bool    `json:"is_set"`
	ID       *int64  `json:"id"`
	Name     *string `json:"name"`
	URL      *string `json:"url"`
	ImageURL *string `json:"image_url"`
}

type HookNewGamePayload struct {
	ID         string      `json:"id"`
	EventID    string      `json:"event_id"`
	UserID     int64       `json:"user_id"`
	UserName   string      `json:"user_name"`
	Name       string      `json:"name"`
	MaxPlayers int         `json:"max_players"`
	MessageID  *int64      `json:"message_id"`
	BGG        HookBGGInfo `json:"bgg"`
	CreatedAt  time.Time   `json:"created_at"`
}

type HookDeleteGamePayload struct {
	EventID   string `json:"event_id"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	DeletedAt string `json:"deleted_at"`
}

type HookUpdateGamePayload struct {
	ID         string      `json:"id"`
	EventID    string      `json:"event_id"`
	UserID     int64       `json:"user_id"`
	UserName   string      `json:"user_name"`
	Name       string      `json:"name"`
	MaxPlayers int         `json:"max_players"`
	MessageID  *int64      `json:"message_id"`
	BGG        HookBGGInfo `json:"bgg"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// --- Participant payloads ---
type HookAddParticipantPayload struct {
	ID       string    `json:"id"`
	EventID  string    `json:"event_id"`
	UserID   int64     `json:"user_id"`
	GameID   string    `json:"game_id"`
	UserName string    `json:"user_name"`
	AddedAt  time.Time `json:"added_at"`
}

type HookRemoveParticipantPayload struct {
	ID        string    `json:"id"`
	EventID   string    `json:"event_id"`
	GameID    string    `json:"game_id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	RemovedAt time.Time `json:"removed_at"`
}
