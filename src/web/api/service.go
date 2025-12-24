package api

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/DangerBlack/gobgg"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

type Service struct {
	DB             *database.Database
	BGG            *gobgg.BGG
	Bot            *telebot.Bot
	LanguageBundle *i18n.Bundle
	Url            models.WebUrl
}

func NewService(db *database.Database, bgg *gobgg.BGG, bot *telebot.Bot, languageBundle *i18n.Bundle, url models.WebUrl) *Service {
	return &Service{
		DB:  db,
		BGG: bgg,
		Bot: bot,

		LanguageBundle: languageBundle,
		Url:            url,
	}
}

func (s *Service) CreateEvent(chatID int64, theadID *int64, id *string, userID int64, userName, name string, location *string, startsAt *time.Time) (*models.Event, error) {
	var err error
	fullText := name
	log.Println("Full text for parsing:", fullText)

	var eventID string
	log.Printf("Creating event: %s by user: %s (%d) in chat: %d", name, userName, userID, chatID)

	if eventID, err = s.DB.InsertEvent(id, chatID, userID, userName, name, nil, location, startsAt); err != nil {
		log.Println("failed to create event:", err)
		return nil, errors.New("failed to create event")
	}
	log.Printf("Event created with id: %s", eventID)

	var event *models.Event

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return nil, errors.New("invalid event ID")
	}

	body, markup := event.FormatMsg(s.Localizer(&chatID), s.Url)

	to := &telebot.Chat{
		ID: chatID,
	}

	opts := &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
	}
	if theadID != nil {
		opts.ReplyTo = &telebot.Message{
			ID: int(*theadID),
		}
	}

	responseMsg, err := s.Bot.Send(to, body, opts, markup, telebot.NoPreview)
	if err != nil {
		log.Println("failed to create event:", err)
		return nil, errors.New("failed to create event")
	}

	if err = s.DB.UpdateEventMessageID(eventID, int64(responseMsg.ID)); err != nil {
		log.Println("failed to create event:", err)
		return nil, errors.New("failed to create event")
	}

	event.MessageID = utils.IntToPointer(responseMsg.ID)

	return event, nil
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
		log.Println("failed to load game:", err)
		return nil, nil, errors.New("invalid event ID")
	}

	if event.Locked && event.UserID != userID {
		log.Println("event is locked")
		return nil, nil, errors.New("unable to add game to locked event")
	}

	bgCtx := context.Background()

	var bgID *int64
	var bgName, bgUrl, bgImageUrl *string
	var finalMaxPlayers *int = maxPlayers

	if bggUrl != nil && *bggUrl != "" {
		var valid bool
		var id int64
		if id, valid = models.ExtractBoardGameID(*bggUrl); !valid {
			return nil, nil, errors.New("invalid bgg url")
		}

		var bgMaxPlayers *int

		if bgMaxPlayers, bgName, bgUrl, bgImageUrl, err = models.ExtractGameInfo(bgCtx, s.BGG, id, name); err != nil {
			log.Printf("Failed to get game %d: %v", id, err)
		} else {
			bgID = &id
			if finalMaxPlayers == nil || *finalMaxPlayers == 0 {
				finalMaxPlayers = bgMaxPlayers
			}
		}
	} else {
		log.Printf("Searching for game %s", name)
		var results []gobgg.SearchResult

		if results, err = s.BGG.Search(bgCtx, name); err != nil {
			log.Printf("Failed to search game %s: %v", name, err)
		}

		if len(results) == 0 {
			log.Printf("Game %s not found", name)
		} else {
			sort.Slice(results, func(i, j int) bool {
				return results[i].ID < results[j].ID
			})

			url := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", results[0].ID)
			bgUrl = &url
			if results[0].Name != "" {
				bgName = &results[0].Name
			}

			bgID = &results[0].ID

			log.Printf("Game %s id %d found: %s", name, *bgID, *bgUrl)

			var things []gobgg.ThingResult

			if things, err = s.BGG.GetThings(bgCtx, gobgg.GetThingIDs(*bgID)); err != nil {
				log.Printf("Failed to get game %s: %v", name, err)
			}

			if len(things) > 0 {
				log.Printf("Game details of %s found", name)
				if things[0].MaxPlayers > 0 && finalMaxPlayers == nil {
					finalMaxPlayers = &things[0].MaxPlayers
				}

				if things[0].Name != "" {
					bgName = &things[0].Name
				} else {
					bgName = &name
				}
				if things[0].Image != "" {
					bgImageUrl = &things[0].Image
				}
			}
		}
	}

	if finalMaxPlayers == nil {
		defaultMax := 5
		finalMaxPlayers = &defaultMax
	}

	log.Printf("Inserting %s in the db", name)

	if _, _, err = s.DB.InsertBoardGame(event.ID, id, name, *finalMaxPlayers, bgID, bgName, bgUrl, bgImageUrl); err != nil {
		log.Println("failed to insert board game:", err)
		return nil, nil, errors.New("failed to insert board game")
	}

	if event, err = s.updateTelegram(eventID); err != nil {
		log.Println("failed to update telegram", err)
	}

	var game *models.BoardGame
	for _, g := range event.BoardGames {
		if g.Name == name {
			game = &g
			break
		}
	}

	return event, game, nil
}

func (s *Service) UpdateGame(eventID string, gameID int64, userID int64, bg models.UpdateGameRequest) (*models.Event, *models.BoardGame, error) {
	var err error
	var event *models.Event
	var game *models.BoardGame

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return nil, nil, errors.New("invalid event ID")
	}

	if event.Locked && event.UserID != userID {
		log.Println("event is locked")
		return nil, nil, errors.New("unable to add game to locked event")
	}

	game = utils.PickGame(event, gameID)

	maxPlayers := int(game.MaxPlayers)
	if bg.MaxPlayers != nil && *bg.MaxPlayers >= 0 {
		maxPlayers = *bg.MaxPlayers
	}

	bgCtx := context.Background()

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

		var bgMaxPlayers *int

		if bgMaxPlayers, bgName, bgUrl, bgImageUrl, err = models.ExtractGameInfo(bgCtx, s.BGG, id, game.Name); err != nil {
			log.Printf("Failed to get game %d: %v", id, err)
		}
		if bgMaxPlayers != nil {
			maxPlayers = int(*bgMaxPlayers)
		}
	}

	if err = s.DB.UpdateBoardGameBGGInfoByID(gameID, maxPlayers, bgID, bgName, bgUrl, bgImageUrl); err != nil {
		log.Println("failed to update board game:", err)
		return nil, nil, errors.New("failed to update board game")
	}

	if event, err = s.updateTelegram(eventID); err != nil {
		log.Println("failed to update telegram", err)
		return nil, nil, err
	}

	game = utils.PickGame(event, gameID)

	return event, game, nil
}

func (s *Service) DeleteGame(eventID string, gameUUID string, userID int64, username string) (*models.Event, *models.BoardGame, error) {
	var err error
	var event *models.Event
	var game *models.BoardGame

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return nil, nil, errors.New("invalid event ID")
	}

	if event.Locked && event.UserID != userID {
		log.Println("event is locked")
		return nil, nil, errors.New("unable to delete game from locked event")
	}

	game = utils.PickGameUUID(event, gameUUID)

	if game == nil {
		log.Printf("invalid game ID: %s", gameUUID)
		return nil, nil, errors.New("invalid game ID")
	}

	if err = s.DB.DeleteBoardGameByID(gameUUID); err != nil {
		log.Println("failed to delete board game:", err)
		return nil, nil, errors.New("failed to delete board game")
	}

	to := &telebot.Chat{
		ID: event.ChatID,
	}

	message := s.Localizer(&event.ChatID).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "GameHasBeenDeleted",
		},
		TemplateData: map[string]string{
			"Username": username,
			"Game":     game.Name,
			"Event":    event.Name,
		},
	})

	options := &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
		ReplyTo: &telebot.Message{
			ID: int(*event.MessageID),
		},
	}

	log.Printf("Sending delete message to chat %d: %s", to.ID, message)
	if _, err = s.Bot.Send(to, message, options); err != nil {
		log.Println("failed to send message:", err)
	}

	log.Printf("Game %s deleted from event %s", game.Name, event.Name)
	if _, err = s.updateTelegram(eventID); err != nil {
		log.Println("failed to update telegram", err)
	}

	return event, game, nil
}

func (s *Service) AddPlayer(id *string, eventID string, gameID int64, userID int64, username string) (string, error) {
	var err error
	var participantID string
	if participantID, err = s.DB.InsertParticipant(id, eventID, gameID, userID, username); err != nil {
		log.Println("failed to add user to participants table:", err)
		return "", errors.New("invalid form data")
	}

	if _, err = s.updateTelegram(eventID); err != nil {
		log.Println("failed to update telegram", err)
		return "", err
	}

	return participantID, nil
}

func (s *Service) DeletePlayer(eventID string, userID int64) error {
	var err error
	if _, _, err = s.DB.RemoveParticipant(eventID, userID); err != nil {
		log.Println("failed to remove participant from webhook:", err)
		return errors.New("failed to remove participant")
	}

	if _, err = s.updateTelegram(eventID); err != nil {
		log.Println("failed to update telegram", err)
		return err
	}

	return nil
}

func (s *Service) updateTelegram(eventID string) (*models.Event, error) {
	var err error
	var event *models.Event

	if event, err = s.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return nil, err
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
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
		log.Println("failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			log.Println("failed because unchanged", err)
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
