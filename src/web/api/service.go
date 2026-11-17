package api

import (
	"boardgame-night-bot/src/bgg"
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/telegram_interface"
	"boardgame-night-bot/src/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DangerBlack/gobgg"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

type Service struct {
	DB             database.DatabaseService
	BGG            bgg.BGGService
	Bot            telegram_interface.TelegramService
	LanguageBundle *i18n.Bundle
	Url            models.WebUrl
	gameUpdateMu   sync.Map // map[int64]*sync.Mutex — serialises concurrent updates per game ID
}

func NewService(db database.DatabaseService, bgg bgg.BGGService, bot telegram_interface.TelegramService, languageBundle *i18n.Bundle, url models.WebUrl) *Service {
	return &Service{
		DB:  db,
		BGG: bgg,
		Bot: bot,

		LanguageBundle: languageBundle,
		Url:            url,
	}
}

func (s *Service) CreateEvent(chatID int64, threadID *int64, id *string, userID int64, userName, name string, location *string, startsAt *time.Time, allowGeneralJoin bool) (*models.Event, error) {
	var err error
	fullText := name
	log.Default().Println("Full text for parsing:", fullText)

	var eventID string
	log.Default().Printf("Creating event: %s by user: %s (%d) in chat: %d", name, userName, userID, chatID)

	if eventID, err = s.DB.InsertEventWithOptionalGame(id, chatID, userID, userName, name, location, startsAt, allowGeneralJoin); err != nil {
		log.Default().Println("failed to create event:", err)
		return nil, fmt.Errorf("failed to create event: %w", err)
	}
	log.Default().Printf("Event created with id: %s", eventID)
	var event *models.Event

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Default().Println("failed to load game:", err)
		return nil, fmt.Errorf("invalid event ID: %w", err)
	}

	body, markup := event.FormatMsg(s.Localizer(&chatID), s.Url)

	to := &telebot.Chat{
		ID: chatID,
	}

	opts := &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
	}
	if threadID != nil {
		opts.ReplyTo = &telebot.Message{
			ID: int(*threadID),
		}
	}

	responseMsg, err := s.Bot.Send(to, body, opts, markup, telebot.NoPreview)
	if err != nil {
		log.Default().Println("failed to create event:", err)
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	if err = s.DB.UpdateEventMessageID(eventID, int64(responseMsg.ID)); err != nil {
		log.Default().Println("failed to create event:", err)
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	event.MessageID = utils.IntToPointer(responseMsg.ID)

	return event, nil
}

// ErrExternalNotAllowed is returned when a web-only account tries to act on an
// event whose owner has not opened it to people outside Telegram.
var ErrExternalNotAllowed = errors.New("this event is not open to people outside Telegram")

// ErrGameNotInEvent is returned when a game ID does not belong to the event.
var ErrGameNotInEvent = errors.New("game does not belong to this event")

// ErrNotEventOwner is returned when someone other than the owner changes event settings.
var ErrNotEventOwner = errors.New("only the event owner can change this setting")

func checkExternalAccess(event *models.Event, userID int64) error {
	if models.IsExternalUserID(userID) && !event.AllowExternal {
		return ErrExternalNotAllowed
	}
	return nil
}

// SetAllowExternal opens or closes the event to web-only (email) accounts.
// Only the event owner may change it.
func (s *Service) SetAllowExternal(eventID string, userID int64, allow bool) (*models.Event, error) {
	event, err := s.DB.SelectEventByEventID(eventID)
	if err != nil {
		return nil, fmt.Errorf("invalid event ID: %w", err)
	}
	if event.UserID != userID {
		return event, ErrNotEventOwner
	}
	if err = s.DB.SetEventAllowExternal(eventID, allow); err != nil {
		return event, fmt.Errorf("failed to update event: %w", err)
	}
	if updated, err := s.updateTelegram(eventID); err == nil && updated != nil {
		return updated, nil
	}
	event.AllowExternal = allow
	return event, nil
}

// IsChatMember reports whether a Telegram user belongs to a chat. The web
// "create event" form receives the chat ID from the mini-app start parameter,
// which anyone can forge; without this check any user could make the bot post
// into any group it is in.
func (s *Service) IsChatMember(chatID, userID int64) bool {
	member, err := s.Bot.ChatMemberOf(&telebot.Chat{ID: chatID}, &telebot.User{ID: userID})
	if err != nil || member == nil {
		log.Default().Printf("chat membership check failed for user %d in chat %d: %v", userID, chatID, err)
		return false
	}
	switch member.Role {
	case telebot.Creator, telebot.Administrator, telebot.Member:
		return true
	case telebot.Restricted:
		return member.Member
	}
	return false
}

// RefreshEvents re-renders the Telegram messages of the given events, e.g.
// after an account merge renamed participants.
func (s *Service) RefreshEvents(eventIDs []string) {
	for _, id := range eventIDs {
		if _, err := s.updateTelegram(id); err != nil {
			log.Default().Printf("failed to refresh event %s: %v", id, err)
		}
	}
}

func (s *Service) DeleteEvent(eventID string, userID *int64, userName string) error {
	var err error
	var event *models.Event

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Default().Println("failed to load event:", err)
		return fmt.Errorf("invalid event ID: %w", err)
	}

	if event.Locked && (userID == nil || event.UserID != *userID) {
		log.Default().Println("event is locked")
		return errors.New("unable to delete locked event")
	}

	if err = s.DB.DeleteEvent(eventID); err != nil {
		log.Default().Println("failed to delete event:", err)
		return fmt.Errorf("failed to delete event: %w", err)
	}

	to := &telebot.Chat{
		ID: event.ChatID,
	}

	message := s.Localizer(&event.ChatID).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "EventHasBeenDeleted",
		},
		TemplateData: map[string]string{
			"Username": models.EscapeHTML(userName),
			"Event":    models.EscapeHTML(event.Name),
		},
	})

	if event.MessageID == nil {
		log.Default().Println("event message id is nil, skipping telegram notification")
		return errors.New("event message ID is nil")
	}

	options := &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
		ReplyTo: &telebot.Message{
			ID: int(*event.MessageID),
		},
	}

	log.Default().Printf("Sending delete message to chat %d: %s", to.ID, message)
	if _, err = s.Bot.Send(to, message, options); err != nil {
		log.Default().Println("failed to send message:", err)
	}

	if err = s.Bot.Delete(&telebot.Message{
		ID: int(*event.MessageID),
		Chat: &telebot.Chat{
			ID: event.ChatID,
		},
	}); err != nil {
		log.Default().Println("failed to delete message:", err)
	}

	return nil
}

// Method signatures
func (s *Service) CreateGame(
	eventID string,
	id *string,
	userID int64,
	name string,
	maxPlayers *int,
	bggUrl *string,
) (*models.Event, *models.BoardGame, error) {
	var err error
	var event *models.Event

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Default().Println("failed to load game:", err)
		return nil, nil, fmt.Errorf("invalid event ID: %w", err)
	}

	if err = checkExternalAccess(event, userID); err != nil {
		return event, nil, err
	}

	if event.Locked && event.UserID != userID {
		log.Default().Println("event is locked")
		return nil, nil, errors.New("unable to add game to locked event")
	}

	bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer bgCancel()

	bgID, bgInfo, err := s.fetchBGGInfo(bgCtx, bggUrl, name)
	if err != nil {
		return nil, nil, err
	}

	finalMaxPlayers := resolveMaxPlayers(maxPlayers, bgInfo.MaxPlayers)

	log.Default().Printf("Inserting %s in the db", name)

	if _, _, err = s.DB.InsertBoardGame(event.ID, id, name, finalMaxPlayers, bgID, bgInfo.Name, bgInfo.Url, bgInfo.ImageUrl); err != nil {
		log.Default().Println("failed to insert board game:", err)
		return nil, nil, fmt.Errorf("failed to insert board game: %w", err)
	}

	if event, err = s.updateTelegram(eventID); err != nil {
		log.Default().Println("failed to update telegram", err)
	}

	var game *models.BoardGame
	for _, g := range event.BoardGames {
		if g.Name == name {
			game = &g
			break
		}
	}

	log.Default().Printf("Game %s created in event %s", name, event.Name)

	return event, game, nil
}

// fetchBGGInfo retrieves BGG metadata for a game. If bggUrl is provided it
// fetches by URL; otherwise it searches BGG by name. BGG network errors are
// non-fatal — the caller receives whatever partial data was obtained.
// Returns an error only for invalid BGG URLs.
func (s *Service) fetchBGGInfo(ctx context.Context, bggUrl *string, name string) (bgID *int64, info models.BggInfo, err error) {
	if bggUrl != nil && *bggUrl != "" {
		var id int64
		var valid bool
		if id, valid = models.ExtractBoardGameID(*bggUrl); !valid {
			return nil, models.BggInfo{}, errors.New("invalid bgg url")
		}

		var bgInfo *models.BggInfo
		if bgInfo, err = s.BGG.ExtractCachedGameInfo(ctx, id, name); err != nil {
			log.Default().Printf("Failed to get BGG info for id %d: %v", id, err)
			return nil, models.BggInfo{}, nil
		}

		return &id, *bgInfo, nil
	}

	// No URL — search by name
	log.Default().Printf("Searching BGG for game: %s", name)
	results, searchErr := s.BGG.Search(ctx, name)
	if searchErr != nil || len(results) == 0 {
		log.Default().Printf("BGG search for %q returned no results: %v", name, searchErr)
		return nil, models.BggInfo{}, nil
	}

	sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })

	bgURL := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", results[0].ID)
	info.Url = &bgURL
	if results[0].Name != "" {
		info.Name = &results[0].Name
	}
	bgID = &results[0].ID
	log.Default().Printf("BGG search matched %q → id %d", name, *bgID)

	things, thingErr := s.BGG.GetThings(ctx, gobgg.GetThingIDs(*bgID))
	if thingErr != nil || len(things) == 0 {
		log.Default().Printf("Failed to get BGG thing details for %d: %v", *bgID, thingErr)
		return bgID, info, nil
	}

	t := things[0]
	if t.MaxPlayers > 0 {
		info.MaxPlayers = &t.MaxPlayers
	}
	if t.Name != "" {
		info.Name = &t.Name
	} else {
		info.Name = &name
	}
	if t.Image != "" {
		info.ImageUrl = &t.Image
	}

	return bgID, info, nil
}

// resolveMaxPlayers picks the final max-player value: prefer the user-supplied
// value, fall back to BGG data, and default to 5 if neither is available.
func resolveMaxPlayers(requested *int, fromBGG *int) int {
	if requested != nil && *requested != 0 {
		return *requested
	}
	if fromBGG != nil {
		return *fromBGG
	}
	return 5
}

// lockGame returns a function that unlocks the per-game mutex for gameID.
// Callers must defer the returned unlock to ensure the lock is always released.
func (s *Service) lockGame(gameID int64) func() {
	v, _ := s.gameUpdateMu.LoadOrStore(gameID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *Service) UpdateGame(eventID string, gameID int64, userID int64, bg models.UpdateGameRequest) (*models.Event, *models.BoardGame, error) {
	defer s.lockGame(gameID)()
	var err error
	var event *models.Event
	var game *models.BoardGame

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Default().Println("failed to load game:", err)
		return nil, nil, fmt.Errorf("invalid event ID: %w", err)
	}

	if err = checkExternalAccess(event, userID); err != nil {
		return event, nil, err
	}

	if event.Locked && event.UserID != userID {
		log.Default().Println("event is locked")
		return nil, nil, errors.New("unable to add game to locked event")
	}

	game = utils.PickGame(event, gameID)
	if game == nil {
		log.Default().Printf("invalid game ID: %d", gameID)
		return nil, nil, errors.New("invalid game ID")
	}

	maxPlayers := int(game.MaxPlayers)
	if bg.MaxPlayers != nil && *bg.MaxPlayers >= 0 {
		maxPlayers = *bg.MaxPlayers
	}

	bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer bgCancel()

	bgID := game.BggID
	bgName := game.BggName
	bgUrl := game.BggUrl
	bgImageUrl := game.BggImageUrl
	if bg.Unlink == "on" {
		bgID = nil
		bgName = nil
		bgUrl = nil
		bgImageUrl = nil
	}

	if bg.BggUrl != nil && *bg.BggUrl != "" {
		var valid bool
		var id int64
		if id, valid = models.ExtractBoardGameID(*bg.BggUrl); !valid {
			return nil, nil, errors.New("invalid bgg url")
		}

		var bgInfo *models.BggInfo

		if bgInfo, err = s.BGG.ExtractCachedGameInfo(bgCtx, id, game.Name); err != nil {
			log.Default().Printf("Failed to get game %d: %v", id, err)
		} else {
			bgID = &id
			bgName = bgInfo.Name
			bgUrl = bgInfo.Url
			bgImageUrl = bgInfo.ImageUrl

			if bgInfo.MaxPlayers != nil && (bg.MaxPlayers == nil || *bg.MaxPlayers == 0) {
				maxPlayers = int(*bgInfo.MaxPlayers)
			}
		}
	}

	if err = s.DB.UpdateBoardGameBGGInfoByID(gameID, maxPlayers, bgID, bgName, bgUrl, bgImageUrl); err != nil {
		log.Default().Println("failed to update board game:", err)
		return nil, nil, fmt.Errorf("failed to update board game: %w", err)
	}

	if event, err = s.updateTelegram(eventID); err != nil {
		log.Default().Println("failed to update telegram", err)
		return nil, nil, err
	}

	game = utils.PickGame(event, gameID)
	if game == nil {
		log.Default().Printf("invalid game ID: %d", gameID)
		return nil, nil, errors.New("invalid game ID")
	}

	return event, game, nil
}

func (s *Service) DeleteGame(eventID string, gameUUID string, userID int64, username string) (*models.Event, *models.BoardGame, error) {
	var err error
	var event *models.Event
	var game *models.BoardGame

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Default().Println("failed to load game:", err)
		return nil, nil, fmt.Errorf("invalid event ID: %w", err)
	}

	if err = checkExternalAccess(event, userID); err != nil {
		return event, nil, err
	}

	if event.Locked && event.UserID != userID {
		log.Default().Println("event is locked")
		return nil, nil, errors.New("unable to delete game from locked event")
	}

	game = utils.PickGameUUID(event, gameUUID)

	if game == nil {
		log.Default().Printf("invalid game ID: %s", gameUUID)
		return nil, nil, errors.New("invalid game ID")
	}

	if err = s.DB.DeleteBoardGameByID(gameUUID); err != nil {
		log.Default().Println("failed to delete board game:", err)
		return nil, nil, fmt.Errorf("failed to delete board game: %w", err)
	}

	to := &telebot.Chat{
		ID: event.ChatID,
	}

	message := s.Localizer(&event.ChatID).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "GameHasBeenDeleted",
		},
		TemplateData: map[string]string{
			"Username": models.EscapeHTML(username),
			"Game":     models.EscapeHTML(game.Name),
			"Event":    models.EscapeHTML(event.Name),
		},
	})

	options := &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
		ReplyTo: &telebot.Message{
			ID: int(*event.MessageID),
		},
	}

	log.Default().Printf("Sending delete message to chat %d: %s", to.ID, message)
	if _, err = s.Bot.Send(to, message, options); err != nil {
		log.Default().Println("failed to send message:", err)
	}

	log.Default().Printf("Game %s deleted from event %s", game.Name, event.Name)
	if _, err = s.updateTelegram(eventID); err != nil {
		log.Default().Println("failed to update telegram", err)
	}

	return event, game, nil
}

func (s *Service) AddPlayer(id *string, eventID string, gameID int64, userID int64, username string, isTelegramUsername bool) (string, *models.Event, *models.BoardGame, error) {
	var err error
	var participantID string

	var current *models.Event
	if current, err = s.DB.SelectEventByEventID(eventID); err != nil {
		return "", nil, nil, fmt.Errorf("invalid event ID: %w", err)
	}

	// The game must belong to this event. Participants are listed per game, so
	// joining game X "through" another event would show the user in X's event,
	// bypassing that event's access rules (e.g. allow_external).
	if utils.PickGame(current, gameID) == nil {
		return "", current, nil, ErrGameNotInEvent
	}

	if err = checkExternalAccess(current, userID); err != nil {
		return "", current, nil, err
	}
	if models.IsExternalUserID(userID) {
		// web-only names are never Telegram handles
		isTelegramUsername = false
	}

	if participantID, err = s.DB.InsertParticipant(id, eventID, gameID, userID, username, isTelegramUsername); err != nil {
		log.Default().Println("failed to add user to participants table:", err)
		return "", nil, nil, fmt.Errorf("invalid form data: %w", err)
	}

	if _, err = s.updateTelegram(eventID); err != nil {
		log.Default().Println("failed to update telegram", err)
		return "", nil, nil, err
	}

	var event *models.Event
	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Default().Println("failed to load game:", err)
		return "", nil, nil, fmt.Errorf("invalid event ID: %w", err)
	}

	game := utils.PickGame(event, gameID)

	return participantID, event, game, nil
}

func (s *Service) DeletePlayer(eventID string, userID int64) (string, *models.Event, *models.BoardGame, error) {
	var err error
	var participantID string
	var gameID int64
	var event *models.Event
	var game *models.BoardGame
	if participantID, gameID, err = s.DB.RemoveParticipant(eventID, userID); err != nil {
		log.Default().Println("failed to remove participant from webhook:", err)
		if errors.Is(err, database.ErrNoRows) {
			return "", nil, nil, database.ErrNoRows
		}

		return "", nil, nil, fmt.Errorf("failed to remove participant: %w", err)
	}

	if event, err = s.updateTelegram(eventID); err != nil {
		log.Default().Println("failed to update telegram", err)
		return "", nil, nil, err
	}

	game = utils.PickGame(event, gameID)

	return participantID, event, game, nil
}

func (s *Service) updateTelegram(eventID string) (*models.Event, error) {
	var err error
	var event *models.Event

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Default().Println("failed to load game:", err)
		return nil, err
	}

	if event.MessageID == nil {
		log.Default().Println("event message id is nil")
		return nil, err
	}

	body, markup := event.FormatMsg(s.Localizer(&event.ChatID), s.Url)

	_, err = s.Bot.Edit(&telebot.Message{
		ID: int(*event.MessageID),
		Chat: &telebot.Chat{
			ID: event.ChatID,
		},
	}, body, markup, telebot.NoPreview)
	if err != nil {
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			// Content is already up-to-date; not a real error.
			err = nil
		} else {
			log.Default().Println("failed to edit message:", err)
		}
	}

	return event, nil
}

func (t Service) Localizer(chatID *int64) *i18n.Localizer {
	if chatID == nil {
		return i18n.NewLocalizer(t.LanguageBundle, "en")
	}

	return i18n.NewLocalizer(t.LanguageBundle, t.DB.GetPreferredLanguage(*chatID), "en")
}
