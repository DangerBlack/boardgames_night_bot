package telegram

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/hooks"
	"boardgame-night-bot/src/language"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/utils"
	"boardgame-night-bot/src/web/api"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

var dateTimeRegex = `\d{2}-\d{2}-\d{4} \d{2}:\d{2}`
var locationRegex = `📍(.+)\n`

type Telegram struct {
	Bot            *telebot.Bot
	DB             *database.Database
	LanguageBundle *i18n.Bundle
	LanguagePack   *language.LanguagePack
	Url            models.WebUrl
	Hook           *hooks.WebhookClient
	Service        *api.Service
}

func (t Telegram) SetupHandlers() {
	t.Bot.Handle("/start", t.Start)
	t.Bot.Handle("/help", t.Start)
	t.Bot.Handle("/create", t.CreateGame)
	t.Bot.Handle("/add_game", t.AddGame)
	t.Bot.Handle("/language", t.SetLanguage)
	t.Bot.Handle("/location", t.SetDefaultLocation)
	t.Bot.Handle("/register", t.RegisterWebhook)
	t.Bot.Handle("/test", t.TestWebhook)

	t.Bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if c.Message().ReplyTo == nil {
			return nil
		}

		return t.UpdateGameDispatcher(c)
	})

	t.Bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		data := c.Callback().Data
		parts := strings.Split(data, "|")

		action := "$" + strings.Split(parts[0], "$")[1]

		log.Printf("User clicked on button: *%s* %d", action, len(parts))
		switch action {
		case string(models.AddPlayer):
			return t.CallbackAddPlayer(c)
		case string(models.Cancel):
			return t.CallbackRemovePlayer(c)
		case string(models.Unregister):
			return t.CallbackUnregisterWebhook(c)
		}

		return c.Reply("invalid action")
	})
}

func DefineUsername(user *telebot.User) (string, bool) {
	if user.Username != "" {
		return user.Username, true
	}

	username := fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	if username != " " {
		return username, false
	}

	return fmt.Sprintf("user_%d", user.ID), false
}

func (t Telegram) Localizer(c telebot.Context) *i18n.Localizer {
	return i18n.NewLocalizer(t.LanguageBundle, t.DB.GetPreferredLanguage(c.Chat().ID), "en")
}

func (t Telegram) Start(c telebot.Context) error {
	var err error
	args := c.Args()

	opts := &telebot.SendOptions{
		ThreadID: c.Message().ThreadID,
	}

	if len(args) < 1 {
		welcomeT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Welcome",
			},
			TemplateData: map[string]string{},
		})

		return c.Send(welcomeT, opts)
	}

	eventID := args[0]
	var event *models.Event
	if event, err = t.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return c.Send(t.Localizer(c).MustLocalizeMessage(&i18n.Message{ID: "EventNotFound"}), opts)
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		return c.Send(t.Localizer(c).MustLocalizeMessage(&i18n.Message{ID: "EventNotFound"}), opts)
	}

	open := telebot.InlineButton{
		Text: "Web",
		URL:  fmt.Sprintf("%s?startapp=%s", t.Url.BotMiniAppURL, eventID),
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
	return c.Send(openT, markup, opts)
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

		chatID := c.Chat().ID
		btn := telebot.InlineButton{
			Text: t.Localizer(c).MustLocalizeMessage(&i18n.Message{ID: "CreateEventWeb"}),
			URL:  fmt.Sprintf("%s?startapp=create_event-%d-%d", t.Url.BotMiniAppURL, chatID, c.Message().ThreadID),
		}

		markup := &telebot.ReplyMarkup{}
		markup.InlineKeyboard = [][]telebot.InlineButton{}
		markup.InlineKeyboard = append(markup.InlineKeyboard, []telebot.InlineButton{btn})
		return c.Reply(usageT, markup)
	}
	eventName := strings.Join(args[0:], " ")
	userID := c.Sender().ID
	userName, _ := DefineUsername(c.Sender())
	chatID := c.Chat().ID
	var threadID *int64
	if c.Message().ThreadID != 0 {
		threadID = utils.IntToPointer(c.Message().ThreadID)
	}
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

	log.Printf("Creating event: %s by user: %s (%d) in chat: %d", eventName, userName, userID, chatID)

	allowGeneralJoin := false
	if strings.Contains(fullText, "👥") {
		allowGeneralJoin = true
	}

	var event *models.Event
	if event, err = t.Service.CreateEvent(chatID, threadID, nil, userID, userName, eventName, location, startsAt, allowGeneralJoin); err != nil {
		log.Println("failed to create event:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToCreateEvent"}})
		return c.Reply(failedT)
	}

	log.Printf("Event created with id: %s", event.ID)

	t.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeNewEvent,
		Data: models.HookNewEventPayload{
			ID:        event.ID,
			ChatID:    event.ChatID,
			UserID:    event.UserID,
			UserName:  event.UserName,
			Name:      event.Name,
			MessageID: event.MessageID,
			Location:  event.Location,
			StartsAt:  event.StartsAt,
			CreatedAt: time.Now(),
		},
	})

	if allowGeneralJoin {
		counterGameID := ""
		for _, e := range event.BoardGames {
			if e.Name == models.PLAYER_COUNTER {
				counterGameID = e.UUID
				break
			}
		}
		t.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
			Type: models.HookWebhookTypeNewGame,
			Data: models.HookNewGamePayload{
				ID:         counterGameID,
				EventID:    event.ID,
				UserID:     userID,
				UserName:   userName,
				Name:       models.PLAYER_COUNTER,
				MaxPlayers: -1,
				MessageID:  event.MessageID,
				BGG: models.HookBGGInfo{
					IsSet: false,
				},
				CreatedAt: time.Now(),
			},
		})
	}

	return nil
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
	userName, _ := DefineUsername(c.Sender())
	gameName := strings.Join(args[0:], " ")
	log.Printf("Adding game: %s in chat id %d", gameName, chatID)

	var event *models.Event

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	if event.Locked && event.UserID != userID {
		log.Println("event is locked")
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "EventLocked"}}))
	}

	var game *models.BoardGame
	if event, game, err = t.Service.CreateGame(event.ID, nil, userID, gameName, nil, nil); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	link := ""
	if game.BggUrl != nil && game.BggName != nil {
		link = fmt.Sprintf(", <a href='%s'>%s</a>", *game.BggUrl, *game.BggName)
	}

	message := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "GameAdded",
		},
		TemplateData: map[string]string{
			"Name":       gameName,
			"Link":       link,
			"MaxPlayers": strconv.Itoa(int(game.MaxPlayers)),
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

	if err = t.DB.UpdateBoardGameMessageID(game.ID, int64(responseMsg.ID)); err != nil {
		log.Println("failed to update boardgame id:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddGame"}})
		return c.Reply(failedT)
	}

	t.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeNewGame,
		Data: models.HookNewGamePayload{
			ID:         game.UUID,
			EventID:    event.ID,
			UserID:     userID,
			UserName:   userName,
			Name:       gameName,
			MaxPlayers: int(game.MaxPlayers),
			MessageID:  utils.IntToPointer(responseMsg.ID),
			BGG: models.HookBGGInfo{
				IsSet:    game.BggID != nil,
				ID:       game.BggID,
				Name:     game.BggName,
				URL:      game.BggUrl,
				ImageURL: game.BggImageUrl,
			},
			CreatedAt: time.Now(),
		},
	})

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
	userID := c.Sender().ID
	userName, _ := DefineUsername(c.Sender())
	messageID := c.Message().ReplyTo.ID
	maxPlayerS := c.Text()

	maxPlayers, err2 := strconv.ParseInt(maxPlayerS, 10, 64)
	if err2 != nil {
		invalidT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidNumberOfPlayers"}})

		return c.Reply(invalidT)
	}

	if exists := t.DB.HasBoardGameWithMessageID(int64(messageID)); !exists {
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	log.Printf("Updating game message id: %d with number of players: %d", messageID, maxPlayers)

	var event *models.Event
	var game *models.BoardGame
	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}})
		return c.Reply(failedT)
	}

	for _, g := range event.BoardGames {
		if g.MessageID != nil && *g.MessageID == int64(messageID) {
			game = &g
			break
		}
	}

	if game == nil {
		log.Printf("game with message id %d not found in event %s", messageID, event.ID)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	mPlayer := int(maxPlayers)
	if event, game, err = t.Service.UpdateGame(event.ID, game.ID, userID, models.UpdateGameRequest{
		MaxPlayers: &mPlayer,
		UserID:     userID,
		UserName:   userName,
		Unlink:     "false",
	}); err != nil {
		if errors.Is(err, errors.New("invalid bgg url")) {
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidBggURL"}}))
		}
		log.Println("failed to update game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	t.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeUpdateGame,
		Data: models.HookUpdateGamePayload{
			ID:         game.UUID,
			EventID:    event.ID,
			UserID:     userID,
			UserName:   userName,
			Name:       game.Name,
			MaxPlayers: int(maxPlayers),
			MessageID:  utils.IntToPointer(messageID),
			BGG: models.HookBGGInfo{
				IsSet:    game.BggID != nil,
				ID:       game.BggID,
				Name:     game.BggName,
				URL:      game.BggUrl,
				ImageURL: game.BggImageUrl,
			},
			UpdatedAt: time.Now(),
		},
	})

	return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameUpdated"}}))
}

func (t Telegram) UpdateGameBGGInfo(c telebot.Context) error {
	var err error
	chatID := c.Chat().ID
	messageID := c.Message().ReplyTo.ID
	bggURL := strings.Trim(c.Text(), " ")

	userID := c.Sender().ID
	userName, _ := DefineUsername(c.Sender())
	var event *models.Event
	var game *models.BoardGame

	if event, err = t.DB.SelectEvent(chatID); err != nil {
		log.Println("failed to add game:", err)
		failedT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}})
		return c.Reply(failedT)
	}

	for _, g := range event.BoardGames {
		if g.MessageID != nil && *g.MessageID == int64(messageID) {
			game = &g
			break
		}
	}

	if game == nil {
		log.Printf("game with message id %d not found in event %s", messageID, event.ID)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "GameNotFound"}}))
	}

	if event, game, err = t.Service.UpdateGame(event.ID, game.ID, userID, models.UpdateGameRequest{
		BggUrl:   &bggURL,
		UserID:   userID,
		UserName: userName,
		Unlink:   "false",
	}); err != nil {
		if errors.Is(err, errors.New("invalid bgg url")) {
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidBggURL"}}))
		}
		log.Println("failed to update game:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUpdateGame"}}))
	}

	t.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeUpdateGame,
		Data: models.HookUpdateGamePayload{
			ID:         game.UUID,
			EventID:    event.ID,
			UserID:     userID,
			UserName:   userName,
			Name:       game.Name,
			MaxPlayers: int(game.MaxPlayers),
			MessageID:  nil,
			BGG: models.HookBGGInfo{
				IsSet:    game.BggID != nil,
				ID:       game.BggID,
				Name:     game.BggName,
				URL:      game.BggUrl,
				ImageURL: game.BggImageUrl,
			},
			UpdatedAt: time.Now(),
		},
	})

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
		log.Println("failed to set location:", err)
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

func (t Telegram) RegisterWebhook(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 {
		usageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID: "Usage",
			},
			TemplateData: map[string]string{
				"Command": "/register",
				"Example": "https://example.com/webhook",
			},
		})
		return c.Reply(usageT)
	}

	var err error
	var secret string
	chatID := c.Chat().ID
	if secret, err = utils.GenerateSecret(32); err != nil {
		log.Println("failed to register webhook:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRegisterWebhook"}}))
	}
	webhookUrl := args[0]
	chatName := ""
	chat := c.Chat()
	if chat.Title != "" {
		// Group or channel name
		chatName = chat.Title
	}

	if chatID < 0 {
		var admins []telebot.ChatMember
		if admins, err = t.Bot.AdminsOf(c.Chat()); err != nil {
			log.Println("failed to get chat admins:", err)
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRegisterWebhook"}}))
		}

		isAdmin := false
		for _, admin := range admins {
			if admin.User.ID == c.Sender().ID {
				isAdmin = true
				break
			}
		}

		if !isAdmin {
			log.Printf("user %d is not admin in chat %d", c.Sender().ID, chatID)
			return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "OnlyAdminsCanRegisterWebhook"}}))
		}
	}

	if !utils.IsValidURL(webhookUrl) {
		log.Println("invalid webhook URL:", webhookUrl)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidWebhookURL"}}))
	}

	if utils.IsLocalURL(webhookUrl) && os.Getenv("ALLOW_LOCAL_WEBHOOKS") != "true" {
		log.Println("local webhook URL not allowed:", webhookUrl)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidWebhookURL"}}))
	}

	testMessage := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "WebhookTestSendPrivateMessage",
		},
	})

	var tmpMsg *telebot.Message
	if tmpMsg, err = t.Bot.Send(c.Sender(), testMessage); err != nil {
		log.Println("failed to send test message to webhook:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRegisterWebhook"}}))
	}

	// delete test message
	if err = t.Bot.Delete(tmpMsg); err != nil {
		log.Println("failed to delete test message:", err)
	}

	log.Printf("Registering webhook %s with secret %s in chat %d", webhookUrl, secret, chatID)

	threadIDx := c.Message().ThreadID

	var threadID *int64
	if threadIDx != 0 {
		threadID = utils.IntToPointer(threadIDx)
	}

	var webhookID *int64
	var webhookUUID *string
	if webhookID, webhookUUID, err = t.DB.InsertWebhook(chatID, threadID, webhookUrl, secret); err != nil {
		log.Println("failed to register webhook:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRegisterWebhook"}}))
	}

	messageT := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "WebhookRegistered",
		},
		TemplateData: map[string]string{
			"WebhookUrl": webhookUrl,
		},
	})

	btn := telebot.InlineButton{
		Text:   "Unregister",
		Unique: string(models.Unregister),
		Data:   fmt.Sprintf("%d", *webhookID),
	}

	markup := &telebot.ReplyMarkup{}
	markup.InlineKeyboard = [][]telebot.InlineButton{}
	markup.InlineKeyboard = append(markup.InlineKeyboard, []telebot.InlineButton{btn})

	privateMessage := t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "WebhookSecret",
		},
		TemplateData: map[string]string{
			"Secret":      secret,
			"WebhookUrl":  webhookUrl,
			"ChatName":    chatName,
			"CallbackUrl": fmt.Sprintf("%s/webhooks/%s", t.Url.BaseUrl, *webhookUUID),
		},
	})

	if _, err = t.Bot.Send(c.Sender(), privateMessage, markup); err != nil {
		log.Println("failed to send private message with webhook secret:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRegisterWebhook"}}))
	}

	return c.Reply(messageT)
}

func (t Telegram) TestWebhook(c telebot.Context) error {
	chatID := c.Chat().ID
	log.Printf("Testing webhooks in chat %d", chatID)

	sentAt := time.Now()
	t.Hook.SendAllWebhookAsync(context.Background(), chatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeTestWebhook,
		Data: models.HookTestPayload{
			Message:   "This is a test webhook message.",
			Timestamp: &sentAt,
		},
	})

	return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "WebhookTestDispatched"}}))
}

func (t Telegram) CallbackAddPlayer(c telebot.Context) error {
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
	userName, isTelegramUsername := DefineUsername(c.Sender())
	log.Printf("User %s (%d) clicked to join a game.", userName, userID)

	var participantID string
	var game *models.BoardGame
	if participantID, _, game, err = t.Service.AddPlayer(nil, eventID, boardGameID, userID, userName, isTelegramUsername); err != nil {
		log.Println("failed to add user to participants table:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToAddPlayer"}}))
	}

	t.Hook.SendAllWebhookAsync(context.Background(), chatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeAddParticipant,
		Data: models.HookAddParticipantPayload{
			ID:       participantID,
			EventID:  eventID,
			GameID:   game.UUID,
			UserID:   userID,
			UserName: userName,
			AddedAt:  time.Now(),
		},
	})

	return nil
}

func (t Telegram) CallbackRemovePlayer(c telebot.Context) error {
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
	userName, _ := DefineUsername(c.Sender())
	log.Printf("User %s (%d) clicked to exit a game.", userName, userID)

	var participantID string
	var game *models.BoardGame
	if participantID, _, game, err = t.Service.DeletePlayer(eventID, userID); err != nil {
		if errors.Is(err, database.ErrNoRows) {
			return nil
		}

		log.Println("failed to delete player:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToRemovePlayer"}}))
	}

	t.Hook.SendAllWebhookAsync(context.Background(), chatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeRemoveParticipant,
		Data: models.HookRemoveParticipantPayload{
			ID:        participantID,
			EventID:   eventID,
			UserID:    userID,
			GameID:    game.UUID,
			UserName:  userName,
			RemovedAt: time.Now(),
		},
	})

	return nil
}

func (t Telegram) CallbackUnregisterWebhook(c telebot.Context) error {
	var err error

	data := c.Callback().Data
	parts := strings.Split(data, "|")
	if len(parts) != 2 {
		log.Println("Invalid data:", data)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "InvalidData"}}))
	}

	// parse to int64
	webhookID, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err2 != nil {
		log.Println("Invalid webhook id:", parts[1])
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUnregisterWebhook"}}))
	}

	userID := c.Sender().ID
	userName, _ := DefineUsername(c.Sender())
	log.Printf("User %s (%d) clicked to unregister a webhook.", userName, userID)

	if err = t.DB.RemoveWebhook(webhookID); err != nil {
		log.Println("failed to remove webhook:", err)
		return c.Reply(t.Localizer(c).MustLocalize(&i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: "FailedToUnregisterWebhook"}}))
	}

	messageID := c.Callback().Message.ID

	if err = t.Bot.Delete(&telebot.Message{
		ID:   messageID,
		Chat: c.Chat(),
	}); err != nil {
		log.Println("failed to delete webhook message:", err)
	}

	return nil
}
