package database

import (
	"boardgame-night-bot/src/models"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUsernameTaken         = errors.New("username already taken")
	ErrTelegramAlreadyLinked = errors.New("telegram account already linked to another user")
	ErrAccountAlreadyLinked  = errors.New("account already linked to a telegram user")
	ErrInvalidOrExpiredToken = errors.New("invalid or expired token")
)

// MigrateToV7 adds web accounts (email registration), one-time auth tokens,
// sessions, and the per-event "allow external" flag. It is purely additive:
// no existing table is rebuilt and no existing row is rewritten, so it is safe
// to run against the production database while the bot keeps working.
func (d *Database) MigrateToV7() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			email TEXT UNIQUE COLLATE NOCASE,
			username TEXT UNIQUE COLLATE NOCASE,
			telegram_user_id INTEGER UNIQUE,
			telegram_name TEXT,
			telegram_is_username BOOLEAN NOT NULL DEFAULT 0,
			is_separated BOOLEAN NOT NULL DEFAULT 0,
			email_verified_at INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS auth_tokens (
			token_hash TEXT PRIMARY KEY,
			purpose TEXT NOT NULL,
			email TEXT COLLATE NOCASE,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			expires_at INTEGER NOT NULL,
			used_at INTEGER,
			created_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS auth_tokens_email_idx ON auth_tokens(email, purpose, created_at);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id);`,
	}

	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			log.Fatal("MigrateToV7: ", err)
		}
	}

	// Existing events stay closed to external users: default 0.
	if _, err := d.addColumnIfNotExists("events", "allow_external", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		log.Fatal("MigrateToV7: ", err)
	}

	log.Default().Println("database migration to v7 completed")
}

// MigrateToV9 records who proposed each game, kept apart from its players.
// Additive only: existing games get NULL (proposer unknown).
func (d *Database) MigrateToV9() {
	for _, col := range []struct{ name, typ string }{
		{"proposed_by_id", "INTEGER"},
		{"proposed_by_name", "TEXT"},
	} {
		if _, err := d.addColumnIfNotExists("boardgames", col.name, col.typ); err != nil {
			log.Fatal("MigrateToV9: ", err)
		}
	}
	log.Default().Println("database migration to v9 completed")
}

// MigrateToV8 removes rows orphaned while foreign keys were not enforced
// (games and participants of deleted events, participants of deleted games).
// It runs before anything relies on cascades and is idempotent.
func (d *Database) MigrateToV8() {
	queries := []string{
		`DELETE FROM participants WHERE event_id NOT IN (SELECT id FROM events)
			OR boardgame_id NOT IN (SELECT id FROM boardgames);`,
		`DELETE FROM boardgames WHERE event_id NOT IN (SELECT id FROM events);`,
		`DELETE FROM sessions WHERE user_id NOT IN (SELECT id FROM users);`,
		`DELETE FROM auth_tokens WHERE user_id IS NOT NULL AND user_id NOT IN (SELECT id FROM users);`,
	}
	var removed int64
	for _, q := range queries {
		res, err := d.db.Exec(q)
		if err != nil {
			log.Fatal("MigrateToV8: ", err)
		}
		n, _ := res.RowsAffected()
		removed += n
	}

	var violations int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil {
		log.Fatal("MigrateToV8: foreign key check: ", err)
	}
	if violations > 0 {
		log.Default().Printf("warning: %d foreign key violations remain after cleanup", violations)
	}

	log.Default().Printf("database migration to v8 completed (%d orphan rows removed)", removed)
}

func (d *Database) SetEventAllowExternal(eventID string, allow bool) error {
	res, err := d.db.Exec(`UPDATE events SET allow_external = @allow WHERE id = @id;`,
		NamedArgs(map[string]any{"allow": allow, "id": eventID})...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRows
	}
	return nil
}

// ---- auth tokens ----

func (d *Database) CreateAuthToken(tokenHash, purpose string, email *string, userID *int64, expiresAt time.Time) error {
	_, err := d.db.Exec(`INSERT INTO auth_tokens (token_hash, purpose, email, user_id, expires_at, created_at)
		VALUES (@hash, @purpose, @email, @user_id, @expires_at, @now);`,
		NamedArgs(map[string]any{
			"hash":       tokenHash,
			"purpose":    purpose,
			"email":      email,
			"user_id":    userID,
			"expires_at": expiresAt.Unix(),
			"now":        time.Now().Unix(),
		})...)
	return err
}

// ConsumeAuthToken atomically marks an unused, unexpired token as used and
// returns what it was issued for. A token can only be consumed once.
func (d *Database) ConsumeAuthToken(tokenHash, purpose string) (email *string, userID *int64, err error) {
	var e sql.NullString
	var u sql.NullInt64
	err = d.db.QueryRow(`UPDATE auth_tokens SET used_at = @now
		WHERE token_hash = @hash AND purpose = @purpose AND used_at IS NULL AND expires_at > @now
		RETURNING email, user_id;`,
		NamedArgs(map[string]any{"hash": tokenHash, "purpose": purpose, "now": time.Now().Unix()})...,
	).Scan(&e, &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrInvalidOrExpiredToken
	}
	if err != nil {
		return nil, nil, err
	}
	if e.Valid {
		email = &e.String
	}
	if u.Valid {
		userID = &u.Int64
	}
	return email, userID, nil
}

// PeekAuthToken returns what a token was issued for without consuming it,
// or ErrInvalidOrExpiredToken if it is unknown, used, or expired.
func (d *Database) PeekAuthToken(tokenHash, purpose string) (email *string, userID *int64, err error) {
	var e sql.NullString
	var u sql.NullInt64
	err = d.db.QueryRow(`SELECT email, user_id FROM auth_tokens
		WHERE token_hash = @hash AND purpose = @purpose AND used_at IS NULL AND expires_at > @now;`,
		NamedArgs(map[string]any{"hash": tokenHash, "purpose": purpose, "now": time.Now().Unix()})...,
	).Scan(&e, &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrInvalidOrExpiredToken
	}
	if err != nil {
		return nil, nil, err
	}
	if e.Valid {
		email = &e.String
	}
	if u.Valid {
		userID = &u.Int64
	}
	return email, userID, nil
}

// CountRecentAuthTokens counts tokens issued for an email since a given time,
// used to throttle outgoing emails per address.
func (d *Database) CountRecentAuthTokens(email, purpose string, since time.Time) (int, error) {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM auth_tokens WHERE email = @email AND purpose = @purpose AND created_at >= @since;`,
		NamedArgs(map[string]any{"email": email, "purpose": purpose, "since": since.Unix()})...).Scan(&n)
	return n, err
}

// ---- users ----

const userColumns = `id, uuid, email, username, telegram_user_id, telegram_name, telegram_is_username, is_separated, created_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(r rowScanner) (*models.User, error) {
	var u models.User
	var email, username, tgName sql.NullString
	var tgID sql.NullInt64
	var createdAt sql.NullTime
	if err := r.Scan(&u.ID, &u.UUID, &email, &username, &tgID, &tgName, &u.TelegramIsUsername, &u.IsSeparated, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, err
	}
	if email.Valid {
		u.Email = &email.String
	}
	if username.Valid {
		u.Username = &username.String
	}
	if tgID.Valid {
		u.TelegramUserID = &tgID.Int64
	}
	if tgName.Valid {
		u.TelegramName = &tgName.String
	}
	if createdAt.Valid {
		u.CreatedAt = createdAt.Time
	}
	return &u, nil
}

func (d *Database) GetUserByID(id int64) (*models.User, error) {
	return scanUser(d.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?;`, id))
}

func (d *Database) GetUserByTelegramID(telegramUserID int64) (*models.User, error) {
	return scanUser(d.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE telegram_user_id = ?;`, telegramUserID))
}

// GetOrCreateUserByEmail returns the account for a verified email, creating it
// (in the pending state) on first login.
func (d *Database) GetOrCreateUserByEmail(email string) (*models.User, error) {
	_, err := d.db.Exec(`INSERT INTO users (uuid, email, email_verified_at) VALUES (?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET email_verified_at = COALESCE(users.email_verified_at, excluded.email_verified_at);`,
		uuid.New().String(), email, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	return scanUser(d.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE email = ?;`, email))
}

// SetSeparatedUsername gives a non-linked account its own username and marks
// it as separated from Telegram. It also renames the account's past
// participations so lists show the new name.
func (d *Database) SetSeparatedUsername(userID int64, username string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE users SET username = ?, is_separated = 1 WHERE id = ? AND telegram_user_id IS NULL;`, username, userID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUsernameTaken
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAccountAlreadyLinked
	}

	externalID := models.ExternalUserID(userID)
	if _, err = tx.Exec(`UPDATE participants SET user_name = ? WHERE user_id = ?;`, username, externalID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE events SET user_name = ? WHERE user_id = ?;`, username, externalID); err != nil {
		return err
	}

	return tx.Commit()
}

// LinkTelegram ties an account to a Telegram user (the user "claims" it).
// If the account was separated, everything it did under its external ID is
// moved to the Telegram ID. When both identities joined the same event, the
// Telegram participation wins and the external duplicate is dropped (this must
// happen before the UPDATE: participants has UNIQUE(event_id, user_id) ON
// CONFLICT REPLACE, which would otherwise silently delete rows).
// Returns the IDs of the events whose rendering changed.
func (d *Database) LinkTelegram(userID, telegramUserID int64, telegramName string, isTelegramUsername bool) ([]string, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var current sql.NullInt64
	if err = tx.QueryRow(`SELECT telegram_user_id FROM users WHERE id = ?;`, userID).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, err
	}
	if current.Valid {
		if current.Int64 == telegramUserID {
			return nil, tx.Commit() // already linked to this very Telegram user: no-op
		}
		return nil, ErrAccountAlreadyLinked
	}

	var other int64
	err = tx.QueryRow(`SELECT id FROM users WHERE telegram_user_id = ?;`, telegramUserID).Scan(&other)
	if err == nil {
		return nil, ErrTelegramAlreadyLinked
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	externalID := models.ExternalUserID(userID)

	rows, err := tx.Query(`SELECT DISTINCT event_id FROM participants WHERE user_id = ?
		UNION SELECT id FROM events WHERE user_id = ?;`, externalID, externalID)
	if err != nil {
		return nil, err
	}
	var eventIDs []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		eventIDs = append(eventIDs, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}

	steps := []struct {
		q    string
		args []any
	}{
		{`DELETE FROM participants WHERE user_id = ? AND event_id IN (SELECT event_id FROM participants WHERE user_id = ?);`,
			[]any{externalID, telegramUserID}},
		{`UPDATE participants SET user_id = ?, user_name = ?, is_telegram_username = ? WHERE user_id = ?;`,
			[]any{telegramUserID, telegramName, isTelegramUsername, externalID}},
		{`UPDATE events SET user_id = ?, user_name = ? WHERE user_id = ?;`,
			[]any{telegramUserID, telegramName, externalID}},
		{`UPDATE boardgames SET proposed_by_id = ?, proposed_by_name = ? WHERE proposed_by_id = ?;`,
			[]any{telegramUserID, telegramName, externalID}},
		{`UPDATE users SET telegram_user_id = ?, telegram_name = ?, telegram_is_username = ?, is_separated = 0 WHERE id = ?;`,
			[]any{telegramUserID, telegramName, isTelegramUsername, userID}},
	}
	for _, st := range steps {
		if _, err = tx.Exec(st.q, st.args...); err != nil {
			if isUniqueViolation(err) {
				return nil, ErrTelegramAlreadyLinked
			}
			return nil, fmt.Errorf("link telegram: %w", err)
		}
	}

	return eventIDs, tx.Commit()
}

// DeleteUser erases an account. A separated account's participations exist
// only through it, so they are deleted too; a linked account's participations
// belong to the Telegram user and are kept. Returns the events to re-render.
func (d *Database) DeleteUser(userID int64) ([]string, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var email sql.NullString
	if err = tx.QueryRow(`SELECT email FROM users WHERE id = ?;`, userID).Scan(&email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, err
	}

	externalID := models.ExternalUserID(userID)
	rows, err := tx.Query(`SELECT DISTINCT event_id FROM participants WHERE user_id = ?;`, externalID)
	if err != nil {
		return nil, err
	}
	var eventIDs []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		eventIDs = append(eventIDs, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Foreign keys are not enforced on this connection, so clean up explicitly.
	steps := []struct {
		q    string
		args []any
	}{
		{`DELETE FROM participants WHERE user_id = ?;`, []any{externalID}},
		{`UPDATE boardgames SET proposed_by_id = NULL, proposed_by_name = NULL WHERE proposed_by_id = ?;`, []any{externalID}},
		{`DELETE FROM sessions WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM auth_tokens WHERE user_id = ? OR (email IS NOT NULL AND email = ?);`, []any{userID, email}},
		{`DELETE FROM users WHERE id = ?;`, []any{userID}},
	}
	for _, st := range steps {
		if _, err = tx.Exec(st.q, st.args...); err != nil {
			return nil, fmt.Errorf("delete user: %w", err)
		}
	}
	return eventIDs, tx.Commit()
}

// ---- sessions ----

func (d *Database) CreateSession(tokenHash string, userID int64, expiresAt time.Time) error {
	_, err := d.db.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?);`,
		tokenHash, userID, expiresAt.Unix(), time.Now().Unix())
	return err
}

func (d *Database) GetSessionUser(tokenHash string) (*models.User, error) {
	return scanUser(d.db.QueryRow(`SELECT `+prefixed("u.", userColumns)+` FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?;`, tokenHash, time.Now().Unix()))
}

func (d *Database) DeleteSession(tokenHash string) error {
	_, err := d.db.Exec(`DELETE FROM sessions WHERE token_hash = ?;`, tokenHash)
	return err
}

// PurgeExpiredAuthData removes expired sessions and stale tokens.
func (d *Database) PurgeExpiredAuthData() error {
	now := time.Now().Unix()
	if _, err := d.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?;`, now); err != nil {
		return err
	}
	// Keep tokens for a day after expiry so per-email throttling still sees them.
	_, err := d.db.Exec(`DELETE FROM auth_tokens WHERE expires_at <= ?;`, now-86400)
	return err
}

func prefixed(prefix, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = prefix + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
