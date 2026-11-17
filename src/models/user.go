package models

import "time"

// UserState describes where an email account is in the onboarding flow.
type UserState string

const (
	// UserStatePending: email verified, but the user has not yet chosen between
	// linking a Telegram account and picking a separate username.
	UserStatePending UserState = "pending"
	// UserStateSeparated: web-only account with its own username, not tied to Telegram.
	UserStateSeparated UserState = "separated"
	// UserStateLinked: the account is tied to a Telegram user (claimed).
	UserStateLinked UserState = "linked"
)

// User is a web account (registered by email). Telegram-only users do not have
// a row: they are identified by their Telegram user ID as before.
type User struct {
	ID                 int64
	UUID               string
	Email              *string
	Username           *string
	TelegramUserID     *int64
	TelegramName       *string
	TelegramIsUsername bool
	IsSeparated        bool
	CreatedAt          time.Time
}

func (u User) State() UserState {
	switch {
	case u.TelegramUserID != nil:
		return UserStateLinked
	case u.IsSeparated && u.Username != nil:
		return UserStateSeparated
	default:
		return UserStatePending
	}
}

// ExternalUserID is the ID a separated account uses in events/participants.
// It is the negated row ID, so it can never collide with a (positive) Telegram user ID.
func ExternalUserID(accountID int64) int64 {
	return -accountID
}
