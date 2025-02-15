package telegram

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/fzerorubigd/gobgg"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

type Telegram struct {
	Bot            *telebot.Bot
	DB             *database.Database
	BGG            *gobgg.BGG
	LanguageBundle *i18n.Bundle
	BaseUrl        string
}

func DefineUsername(user *telebot.User) string {
	if user.Username != "" {
		return user.Username
	}

	username := fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	if username != " " {
		return username
	}

	return fmt.Sprintf("user_%d", user.ID)
}

func (t Telegram) Localizer(c telebot.Context) *i18n.Localizer {
	return i18n.NewLocalizer(t.LanguageBundle, t.DB.GetPreferredLanguage(c.Chat().ID), "en")
}

func (t Telegram) Start(c telebot.Context) error {
	welcomeT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "Welcome",
		},
		TemplateData: map[string]string{},
	})

	return c.Send(welcomeT)
}

func (t Telegram) CreateGame(c telebot.Context) error {
	var err error
	args := c.Args()
	if len(args) < 1 {
		eventNameT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "EventName"}})
		usageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Usage",
			},
			TemplateData: map[string]string{
				"Command": "/create",
				"Example": eventNameT,
			},
		})
		return c.Reply(usageT)
	}
	eventName := strings.Join(args[0:], " ")
	userID := c.Sender().ID
	userName := DefineUsername(c.Sender())
	chatID := c.Chat().ID

	var eventID string
	log.Printf("Creating event: %s by user: %s (%d) in chat: %d", eventName, userName, userID, chatID)

	if eventID, err = t.DB.InsertEvent(chatID, userID, userName, eventName, nil); err != nil {
		log.Println("Failed to create event:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	var event *models.Event

	if event, err = t.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("Failed to load game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl)

	responseMsg, err := t.Bot.Reply(c.Message(), body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to create event:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	if err = t.DB.UpdateEventMessageID(eventID, int64(responseMsg.ID)); err != nil {
		log.Println("Failed to create event:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	return err
}

func (t Telegram) AddGame(c telebot.Context) error {
	var err error
	log.Println("User requested to add a game.")

	args := c.Args()
	if len(args) < 1 {
		gameNameT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameName"}})
		usageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Usage",
			},
			TemplateData: map[string]string{
				"Command": "/add_game",
				"Example": gameNameT,
			},
		})
		return c.Reply(usageT)
	}

	chatID := c.Chat().ID
	userID := c.Sender().ID
	userName := DefineUsername(c.Sender())
	gameName := strings.Join(args[0:], " ")
	maxPlayers := 5
	log.Printf("Adding game: %s with max players: %d", gameName, maxPlayers)

	var event *models.Event
	var boardGameID int64

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	ctx := context.Background()
	var results []gobgg.SearchResult
	var bgUrl, bgName *string
	var bgID *int64

	if results, err = t.BGG.Search(ctx, gameName); err != nil {
		log.Printf("Failed to search game %s: %v", gameName, err)
	}

	if len(results) == 0 {
		log.Printf("Game %s not found", gameName)
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

		log.Printf("Game %s id %d found: %s", gameName, *bgID, *bgUrl)

		var things []gobgg.ThingResult

		if things, err = t.BGG.GetThings(ctx, gobgg.GetThingIDs(*bgID)); err != nil {
			log.Printf("Failed to get game %s: %v", gameName, err)
		}

		if len(things) > 0 {
			maxPlayers = things[0].MaxPlayers
			if things[0].Name != "" {
				bgName = &things[0].Name
			} else {
				bgName = &gameName
			}
		}
	}

	if boardGameID, err = t.DB.InsertBoardGame(event.ID, gameName, maxPlayers, bgID, bgName, bgUrl); err != nil {
		log.Println("Failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if _, err = t.DB.InsertParticipant(event.ID, boardGameID, userID, userName); err != nil {
		log.Println("Failed to add user to participants table:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if event.MessageID == nil {
		log.Println("Event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl)

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return c.Respond()
		}

		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateMessageEvent"}})
		return c.Reply(failedT)
	}

	link := ""
	if bgUrl != nil && bgName != nil {
		link = fmt.Sprintf(", <a href='%s'>%s</a>", *bgUrl, *bgName)
	}

	message := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "GameAdded",
		},
		TemplateData: map[string]string{
			"Name":       gameName,
			"Link":       link,
			"MaxPlayers": strconv.Itoa(maxPlayers),
		},
	})

	responseMsg, err := t.Bot.Reply(
		c.Message(),
		message,
		telebot.NoPreview,
	)
	if err != nil {
		log.Println("Failed to dispatch add game message:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if err = t.DB.UpdateBoardGameMessageID(boardGameID, int64(responseMsg.ID)); err != nil {
		log.Println("Failed to update boardgame id:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	return c.Respond()
}

func (t Telegram) UpdateGameDispatcher(c telebot.Context) error {
	if c.Message().ReplyTo == nil {
		return nil
	}

	if strings.HasPrefix(c.Text(), "https://boardgamegeek.com/boardgame/") {
		return t.UpdateGameBGGInfo(c)
	}

	return t.UpdateGameNumberOfPlayer(c)
}

func (t Telegram) UpdateGameNumberOfPlayer(c telebot.Context) error {
	var err error
	chatID := c.Chat().ID
	messageID := c.Message().ReplyTo.ID
	maxPlayerS := c.Text()

	maxPlayers, err2 := strconv.ParseInt(maxPlayerS, 10, 64)
	if err2 != nil {
		invalidT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidNumberOfPlayers"}})

		return c.Reply(invalidT)
	}

	log.Printf("Updating game message id: %d with number of players: %d", messageID, maxPlayers)

	if err = t.DB.UpdateBoardGamePlayerNumber(int64(messageID), int(maxPlayers)); err != nil {
		if errors.Is(err, database.ErrNoRows) {
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
		}

		log.Println("Failed to update game:", err)

		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	var event *models.Event

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}})

		return c.Reply(failedT)
	}

	if event.MessageID == nil {
		log.Println("Event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl)

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return c.Respond()
		}

		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateMessageEvent"}})

		return c.Reply(failedT)
	}

	return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameUpdated"}}))
}

func (t Telegram) UpdateGameBGGInfo(c telebot.Context) error {
	var err error
	chatID := c.Chat().ID
	messageID := c.Message().ReplyTo.ID
	bggURL := strings.Trim(c.Text(), " ")

	var valid bool
	var id int64
	if id, valid = models.ExtractBoardGameID(bggURL); !valid {
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidBggURL"}}))
	}

	ctx := context.Background()
	var maxPlayers *int
	var bgName, bgUrl *string

	if maxPlayers, bgName, bgUrl, err = models.ExtractGameInfo(ctx, t.BGG, id, "old name"); err != nil {
		log.Printf("Failed to get game %d: %v", id, err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToGetGameInfo"}}))
	}

	log.Printf("Updating game message id: %d with number of players: %d", messageID, maxPlayers)

	if err = t.DB.UpdateBoardGameBGGInfo(int64(messageID), *maxPlayers, &id, bgName, bgUrl); err != nil {
		if errors.Is(err, database.ErrNoRows) {
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
		}

		log.Println("Failed to update game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	var event *models.Event

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	if event.MessageID == nil {
		log.Println("Event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl)

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return c.Respond()
		}

		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateMessageEvent"}}))
	}

	return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameUpdated"}}))
}

func (t Telegram) SetLanguage(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 {
		usageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Usage",
			},
			TemplateData: map[string]string{
				"Command": "/language",
				"Example": "en",
			},
		})
		return c.Reply(usageT)
	}

	chatID := c.Chat().ID
	language := args[0]
	log.Printf("Setting language to %s in chat %d", language, chatID)

	if err := t.DB.InsertChat(chatID, language); err != nil {
		log.Println("Failed to set language:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToSetLanguage"}}))
	}

	messageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "LanguageSet",
		},
		TemplateData: map[string]string{
			"Language": language,
		},
	})

	return c.Reply(messageT)
}

func (t Telegram) CallbackAddPlayer(c telebot.Context) error {
	var event *models.Event
	var err error

	data := c.Callback().Data
	parts := strings.Split(data, "|")
	if len(parts) != 3 {
		log.Println("Invalid data:", data)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidData"}}))
	}

	eventID := parts[1]
	boardGameID, err2 := strconv.ParseInt(parts[2], 10, 64)
	if !models.IsValidUUID(eventID) || err2 != nil {
		log.Println("Invalid parsed id:", data)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidData"}}))
	}

	chatID := c.Chat().ID
	userID := c.Sender().ID
	userName := DefineUsername(c.Sender())
	log.Printf("User %s (%d) clicked to join a game.", userName, userID)

	if _, err = t.DB.InsertParticipant(eventID, boardGameID, userID, userName); err != nil {
		log.Println("Failed to add user to participants table:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddPlayer"}}))
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "EventNotFound"}}))
	}

	if event.MessageID == nil {
		log.Println("Event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl)
	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return c.Respond()
		}

		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateMessageEvent"}}))
	}

	return c.Respond()
}

func (t Telegram) CallbackRemovePlayer(c telebot.Context) error {
	var event *models.Event
	var err error

	data := c.Callback().Data
	parts := strings.Split(data, "|")
	if len(parts) != 2 {
		log.Println("Invalid data:", data)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidData"}}))
	}

	eventID := parts[1]
	if !models.IsValidUUID(eventID) {
		log.Println("Invalid parsed id:", data)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidData"}}))
	}

	chatID := c.Chat().ID
	userID := c.Sender().ID
	userName := DefineUsername(c.Sender())
	log.Printf("User %s (%d) clicked to exit a game.", userName, userID)

	if err = t.DB.RemoveParticipant(eventID, userID); err != nil {
		log.Println("Failed to remove user to participants table:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRemovePlayer"}}))
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("Failed to add game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "EventNotFound"}}))
	}

	if event.MessageID == nil {
		log.Println("Event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl)
	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("Failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return c.Respond()
		}

		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateMessageEvent"}}))
	}

	return c.Respond()
}
