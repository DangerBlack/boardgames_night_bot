// Package auth handles web identity: email registration (magic links sent
// through Resend), browser sessions, verified Telegram Mini App identities,
// and linking ("claiming") a Telegram account from an email account.
package auth

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	PurposeLogin        = "login"
	PurposeLinkTelegram = "link_telegram"

	// LinkStartPrefix is the /start payload prefix used by the bot deep link.
	// Telegram limits start payloads to 64 chars of [A-Za-z0-9_-].
	LinkStartPrefix = "link_"
)

var (
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrInvalidUsername = errors.New("invalid username")
	ErrTooManyRequests = errors.New("too many requests")
	ErrMailDisabled    = errors.New("email registration is not configured")
)

// Store is the persistence the auth service needs; *database.Database implements it.
type Store interface {
	CreateAuthToken(tokenHash, purpose string, email *string, userID *int64, expiresAt time.Time) error
	ConsumeAuthToken(tokenHash, purpose string) (email *string, userID *int64, err error)
	CountRecentAuthTokens(email, purpose string, since time.Time) (int, error)
	GetOrCreateUserByEmail(email string) (*models.User, error)
	GetUserByID(id int64) (*models.User, error)
	SetSeparatedUsername(userID int64, username string) error
	LinkTelegram(userID, telegramUserID int64, telegramName string, isTelegramUsername bool) ([]string, error)
	CreateSession(tokenHash string, userID int64, expiresAt time.Time) error
	GetSessionUser(tokenHash string) (*models.User, error)
	DeleteSession(tokenHash string) error
}

var _ Store = (*database.Database)(nil)

type Config struct {
	BotToken       string
	InitDataMaxAge time.Duration
	LoginTokenTTL  time.Duration
	LinkTokenTTL   time.Duration
	SessionTTL     time.Duration
	// MaxEmailsPerWindow caps login emails per address within EmailWindow.
	MaxEmailsPerWindow int
	EmailWindow        time.Duration
}

func DefaultConfig(botToken string) Config {
	return Config{
		BotToken:           botToken,
		InitDataMaxAge:     24 * time.Hour,
		LoginTokenTTL:      15 * time.Minute,
		LinkTokenTTL:       10 * time.Minute,
		SessionTTL:         30 * 24 * time.Hour,
		MaxEmailsPerWindow: 3,
		EmailWindow:        15 * time.Minute,
	}
}

type Service struct {
	Store  Store
	Mailer Mailer // nil disables email registration
	Config Config
	// OnEventsChanged is called after a link moved participations, so the
	// Telegram messages of those events can be re-rendered.
	OnEventsChanged func(eventIDs []string)
}

func NewService(store Store, mailer Mailer, cfg Config) *Service {
	return &Service{Store: store, Mailer: mailer, Config: cfg}
}

func (s *Service) EmailEnabled() bool { return s.Mailer != nil }

// ---- tokens ----

// newToken returns a random URL-safe token and its SHA-256 hash. Only the hash
// is stored, so a database leak does not expose usable links or sessions.
func newToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ---- validation ----

func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

var usernameRe = regexp.MustCompile(`^[\p{L}\p{N}_.\- ]+$`)

// NormalizeUsername accepts 3-32 chars of letters, digits, space, "_", "." and "-".
// A leading "@" is rejected so separated users cannot look like Telegram handles.
func NormalizeUsername(username string) (string, error) {
	username = strings.Join(strings.Fields(username), " ")
	n := utf8.RuneCountInString(username)
	if n < 3 || n > 32 || !usernameRe.MatchString(username) {
		return "", ErrInvalidUsername
	}
	return username, nil
}

// SafeNext only allows local absolute paths as post-login redirects.
func SafeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/account"
	}
	return next
}

// ---- email login ----

// RequestLogin issues a single-use login link and emails it. compose builds the
// (localized) email around the link. To avoid account enumeration callers
// should show the same response whether or not the address is known.
func (s *Service) RequestLogin(ctx context.Context, rawEmail string, linkFor func(token string) string, compose func(link string) Email) error {
	if s.Mailer == nil {
		return ErrMailDisabled
	}
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return err
	}

	n, err := s.Store.CountRecentAuthTokens(email, PurposeLogin, time.Now().Add(-s.Config.EmailWindow))
	if err != nil {
		return err
	}
	if n >= s.Config.MaxEmailsPerWindow {
		return ErrTooManyRequests
	}

	raw, hash, err := newToken()
	if err != nil {
		return err
	}
	if err = s.Store.CreateAuthToken(hash, PurposeLogin, &email, nil, time.Now().Add(s.Config.LoginTokenTTL)); err != nil {
		return err
	}

	msg := compose(linkFor(raw))
	msg.To = email
	if err = s.Mailer.Send(ctx, msg); err != nil {
		log.Default().Printf("failed to send login email: %v", err)
		return fmt.Errorf("send login email: %w", err)
	}
	return nil
}

// VerifyLogin consumes a login token, creates the account on first use, and
// opens a session. It returns the raw session token for the cookie.
func (s *Service) VerifyLogin(rawToken string) (*models.User, string, error) {
	email, _, err := s.Store.ConsumeAuthToken(HashToken(rawToken), PurposeLogin)
	if err != nil {
		return nil, "", err
	}
	if email == nil {
		return nil, "", database.ErrInvalidOrExpiredToken
	}

	user, err := s.Store.GetOrCreateUserByEmail(*email)
	if err != nil {
		return nil, "", err
	}

	session, err := s.CreateSession(user.ID)
	if err != nil {
		return nil, "", err
	}
	return user, session, nil
}

func (s *Service) CreateSession(userID int64) (string, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	if err = s.Store.CreateSession(hash, userID, time.Now().Add(s.Config.SessionTTL)); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) SessionUser(rawSession string) (*models.User, error) {
	if rawSession == "" {
		return nil, database.ErrNoRows
	}
	return s.Store.GetSessionUser(HashToken(rawSession))
}

func (s *Service) Logout(rawSession string) error {
	if rawSession == "" {
		return nil
	}
	return s.Store.DeleteSession(HashToken(rawSession))
}

// ---- separated accounts ----

func (s *Service) ChooseSeparatedUsername(user *models.User, rawUsername string) error {
	if user.TelegramUserID != nil {
		return database.ErrAccountAlreadyLinked
	}
	username, err := NormalizeUsername(rawUsername)
	if err != nil {
		return err
	}
	return s.Store.SetSeparatedUsername(user.ID, username)
}

// ---- Telegram linking ----

// CreateTelegramLinkToken returns the /start payload for the bot deep link
// that lets the account owner prove which Telegram user they are.
func (s *Service) CreateTelegramLinkToken(user *models.User) (string, error) {
	if user.TelegramUserID != nil {
		return "", database.ErrAccountAlreadyLinked
	}
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	if err = s.Store.CreateAuthToken(hash, PurposeLinkTelegram, nil, &user.ID, time.Now().Add(s.Config.LinkTokenTTL)); err != nil {
		return "", err
	}
	return LinkStartPrefix + raw, nil
}

// LinkWithStartPayload is called by the bot when it receives /start link_<token>.
// The Telegram identity comes from the update sender, which Telegram vouches for.
func (s *Service) LinkWithStartPayload(payload string, tg TelegramUser) (*models.User, error) {
	raw, ok := strings.CutPrefix(payload, LinkStartPrefix)
	if !ok || raw == "" {
		return nil, database.ErrInvalidOrExpiredToken
	}
	_, userID, err := s.Store.ConsumeAuthToken(HashToken(raw), PurposeLinkTelegram)
	if err != nil {
		return nil, err
	}
	if userID == nil {
		return nil, database.ErrInvalidOrExpiredToken
	}
	return s.link(*userID, tg)
}

// LinkWithTelegramUser links using an identity already verified from Mini App
// init data (reconciliation when an email user opens the app inside Telegram).
func (s *Service) LinkWithTelegramUser(user *models.User, tg TelegramUser) (*models.User, error) {
	return s.link(user.ID, tg)
}

func (s *Service) link(userID int64, tg TelegramUser) (*models.User, error) {
	name, isUsername := tg.DisplayName()
	eventIDs, err := s.Store.LinkTelegram(userID, tg.ID, name, isUsername)
	if err != nil {
		return nil, err
	}
	if len(eventIDs) > 0 && s.OnEventsChanged != nil {
		s.OnEventsChanged(eventIDs)
	}
	return s.Store.GetUserByID(userID)
}
