package telegram

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/language"
	"boardgame-night-bot/src/models"
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DangerBlack/gobgg"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

var dateTimeRegex = `\d{2}-\d{2}-\d{4} \d{2}:\d{2}`
var locationRegex = `📍(.+)\n`

type Telegram struct {
	Bot            *telebot.Bot
	DB             *database.Database
	BGG            *gobgg.BGG
	LanguageBundle *i18n.Bundle
	LanguagePack   *language.LanguagePack
	BaseUrl        string
	BotName        string
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
	var err error
	args := c.Args()

	if len(args) < 1 {
		welcomeT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Welcome",
			},
			TemplateData: map[string]string{},
		})

		return c.Send(welcomeT)
	}

	eventID := args[0]
	var event *models.Event
	if event, err = t.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return c.Send(t.Localizer(c).MustLocalizeMessage(&i18n.Message{ID: "EventNotFound"}))
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		return c.Send(t.Localizer(c).MustLocalizeMessage(&i18n.Message{ID: "EventNotFound"}))
	}

	open := telebot.InlineButton{
		Text: "Web",
		WebApp: &telebot.WebApp{
			URL: fmt.Sprintf("%s/events/%s/", t.BaseUrl, eventID),
		},
	}
	markup := &telebot.ReplyMarkup{}
	markup.InlineKeyboard = [][]telebot.InlineButton{{open}}

	openT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "Open",
		},
		TemplateData: map[string]string{
			"Name": event.Name,
		},
	})
	return c.Send(openT, markup)
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
	var startsAt *time.Time
	var location *string

	fullText := c.Message().Text
	log.Println("Full text for parsing:", fullText)

	// check regex for datetime at the end of the event name DD-MM-YYYY HH:MM
	matched, err := regexp.MatchString(dateTimeRegex, fullText)
	if err != nil {
		log.Println("failed to parse date time:", err)
	}
	if matched {
		re := regexp.MustCompile(dateTimeRegex)
		dateTimeStr := re.FindString(fullText)
		layout := "02-01-2006 15:04"
		t, err := time.Parse(layout, dateTimeStr)
		if err != nil {
			log.Println("failed to parse date time:", err)
		} else {
			log.Printf("Parsed date time: %s\n", t.String())
			startsAt = &t
		}
	}

	// check regex for location
	matchedLoc, err := regexp.MatchString(locationRegex, fullText)
	if err != nil {
		log.Println("failed to parse location:", err)
	}
	if matchedLoc {
		re := regexp.MustCompile(locationRegex)
		locationStr := re.FindStringSubmatch(fullText)
		if len(locationStr) > 1 {
			loc := strings.TrimSpace(locationStr[1])
			log.Printf("Parsed location: %s\n", loc)
			location = &loc
		}
	}

	var eventID string
	log.Printf("Creating event: %s by user: %s (%d) in chat: %d", eventName, userName, userID, chatID)

	if eventID, err = t.DB.InsertEvent(chatID, userID, userName, eventName, nil, location, startsAt); err != nil {
		log.Println("failed to create event:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}
	log.Printf("Event created with id: %s", eventID)

	if strings.Contains(fullText, "👥") {
		if _, err = t.DB.InsertBoardGame(eventID, models.PLAYER_COUNTER, -1, nil, nil, nil, nil); err != nil {
			log.Println("failed to add game:", err)
			failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
			return c.Reply(failedT)
		}
	}

	var event *models.Event

	if event, err = t.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl, t.BotName)

	responseMsg, err := t.Bot.Reply(c.Message(), body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("failed to create event:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	if err = t.DB.UpdateEventMessageID(eventID, int64(responseMsg.ID)); err != nil {
		log.Println("failed to create event:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	return err
}

func (t Telegram) AddGame(c telebot.Context) error {
	var err error
	log.Println("user requested to add a game")

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
	log.Printf("Adding game: %s in chat id %d with max players: %d", gameName, chatID, maxPlayers)

	var event *models.Event
	var boardGameID int64

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if event.Locked && event.UserID != userID {
		log.Println("event is locked")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "EventLocked"}}))
	}

	ctx := context.Background()
	var results []gobgg.SearchResult
	var bgUrl, bgName, bggImageUrl *string
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
			if things[0].Image != "" {
				bggImageUrl = &things[0].Image
				log.Println("image URL:", *bggImageUrl)
			}
		}
	}

	if boardGameID, err = t.DB.InsertBoardGame(event.ID, gameName, maxPlayers, bgID, bgName, bgUrl, bggImageUrl); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if _, err = t.DB.InsertParticipant(event.ID, boardGameID, userID, userName); err != nil {
		log.Println("failed to add user to participants table:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	log.Printf("event message id: %d", *event.MessageID)

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl, t.BotName)

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return nil
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
		log.Println("failed to dispatch add game message:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if err = t.DB.UpdateBoardGameMessageID(boardGameID, int64(responseMsg.ID)); err != nil {
		log.Println("failed to update boardgame id:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	return nil
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
	if exists := t.DB.HasBoardGameWithMessageID(int64(messageID)); !exists {
		if err2 == nil {
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
		} else {
			return nil
		}
	}

	if err2 != nil {
		invalidT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidNumberOfPlayers"}})

		return c.Reply(invalidT)
	}

	log.Printf("Updating game message id: %d with number of players: %d", messageID, maxPlayers)

	if err = t.DB.UpdateBoardGamePlayerNumber(int64(messageID), int(maxPlayers)); err != nil {
		if errors.Is(err, database.ErrNoRows) {
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
		}

		log.Println("failed to update game:", err)

		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	var event *models.Event

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}})

		return c.Reply(failedT)
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl, t.BotName)

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return nil
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
	var bgName, bgUrl, bgImageUrl *string

	if maxPlayers, bgName, bgUrl, bgImageUrl, err = models.ExtractGameInfo(ctx, t.BGG, id, "old name"); err != nil {
		log.Printf("Failed to get game %d: %v", id, err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToGetGameInfo"}}))
	}

	log.Printf("Updating game message id: %d with number of players: %d", messageID, maxPlayers)

	if err = t.DB.UpdateBoardGameBGGInfo(int64(messageID), *maxPlayers, &id, bgName, bgUrl, bgImageUrl); err != nil {
		if errors.Is(err, database.ErrNoRows) {
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
		}

		log.Println("failed to update game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	var event *models.Event

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl, t.BotName)

	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return nil
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

	if !t.LanguagePack.HasLanguage(language) {
		log.Printf("Language %s not available\n", language)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "FailedLanguageNotAvailable",
			},
			TemplateData: map[string]string{
				"AvailableLanguages": strings.Join(t.LanguagePack.Languages, ", "),
			},
		},
		))
	}

	if err := t.DB.InsertChat(chatID, &language, nil); err != nil {
		log.Println("failed to set language:", err)
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

func (t Telegram) SetDefaultLocation(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 {
		usageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Usage",
			},
			TemplateData: map[string]string{
				"Command": "/location",
				"Example": "Circolo degli Artisti, Rome",
			},
		})
		return c.Reply(usageT)
	}

	chatID := c.Chat().ID
	location := strings.Join(args[0:], " ")
	log.Printf("Setting location to %s in chat %d", location, chatID)

	if err := t.DB.InsertChat(chatID, nil, &location); err != nil {
		log.Println("failed to set language:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToSetLocation"}}))
	}

	messageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "LocationSet",
		},
		TemplateData: map[string]string{
			"Location": location,
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
		log.Println("failed to add user to participants table:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddPlayer"}}))
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "EventNotFound"}}))
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl, t.BotName)
	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return nil
		}

		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateMessageEvent"}}))
	}

	return nil
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
		log.Println("failed to remove user to participants table:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRemovePlayer"}}))
	}

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "EventNotFound"}}))
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	body, markup := event.FormatMsg(t.Localizer(c), t.BaseUrl, t.BotName)
	_, err = t.Bot.Edit(&telebot.Message{
		ID:   int(*event.MessageID),
		Chat: c.Chat(),
	}, body, markup, telebot.NoPreview)
	if err != nil {
		log.Println("failed to edit message", err)
		if strings.Contains(err.Error(), models.MessageUnchangedErrorMessage) {
			return nil
		}

		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateMessageEvent"}}))
	}

	return nil
}
