package api

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/hooks"
	"boardgame-night-bot/src/models"
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DangerBlack/gobgg"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

type Controller struct {
	Router         *gin.RouterGroup
	DB             *database.Database
	BGG            *gobgg.BGG
	Bot            *telebot.Bot
	LanguageBundle *i18n.Bundle
	Url            models.WebUrl
	Hook           *hooks.WebhookClient
}

func NewController(router *gin.RouterGroup, db *database.Database, bgg *gobgg.BGG, bot *telebot.Bot, LanguageBundle *i18n.Bundle, hook *hooks.WebhookClient, botMiniAppURL string) *Controller {
	return &Controller{
		Router:         router,
		DB:             db,
		BGG:            bgg,
		Bot:            bot,
		LanguageBundle: LanguageBundle,
		Url: models.WebUrl{
			BotMiniAppURL: botMiniAppURL,
		},
		Hook: hook,
	}
}

func (t Controller) Localizer(chatID *int64) *i18n.Localizer {
	if chatID == nil {
		return i18n.NewLocalizer(t.LanguageBundle, "en")
	}

	return i18n.NewLocalizer(t.LanguageBundle, t.DB.GetPreferredLanguage(*chatID), "en")
}

func (c *Controller) InjectRoute() {
	c.Router.GET("/", c.Index)
	c.Router.GET("/events/:event_id", c.Event)
	c.Router.GET("/events/:event_id/games/:game_id", c.Game)
	c.Router.POST("/events/:event_id/games/:game_id", c.UpdateGame)
	c.Router.DELETE("/events/:event_id/games/:game_id", c.DeleteGame)
	c.Router.POST("/events/:event_id/add-game", c.AddGame)
	c.Router.POST("/events/:event_id/join", c.AddPlayer)
	c.Router.GET("/bgg/search", c.BggSearch)
}

// BggSearch handles GET /bgg/search?name= for autocomplete
func (c *Controller) BggSearch(ctx *gin.Context) {
	name := ctx.Query("name")
	if name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing name parameter"})
		return
	}
	bgCtx := context.Background()
	results, err := c.BGG.Search(bgCtx, name)
	if err != nil {
		log.Printf("BGG search error: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search BGG"})
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	// Return top 10 results with name and bgg_url
	var out []map[string]string
	for i, r := range results {
		if i >= 10 {
			break
		}
		url := ""
		if r.ID > 0 {
			url = fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", r.ID)
		}
		out = append(out, map[string]string{
			"name":    r.Name,
			"bgg_url": url,
		})
	}
	ctx.JSON(http.StatusOK, out)
}

func (c *Controller) Index(ctx *gin.Context) {
	event_id := ctx.Query("tgWebAppStartParam")
	if event_id != "" {
		ctx.Redirect(http.StatusFound, fmt.Sprintf("/events/%s", event_id))
		return
	}

	ctx.HTML(http.StatusOK, "index", nil)
}

func (c *Controller) renderError(ctx *gin.Context, id *string, chatID *int64, err string) {
	localizer := c.Localizer(chatID)

	ctx.HTML(http.StatusOK, "error", gin.H{
		"Id":                 id,
		"SomethingWentWrong": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebSomethingWentWrong"}),
		"Error":              err,
	})
}

func (c *Controller) NoRoute(ctx *gin.Context) {
	localizer := c.Localizer(nil)
	ctx.HTML(http.StatusOK, "error", gin.H{
		"Id":                 nil,
		"SomethingWentWrong": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebSomethingWentWrong"}),
		"Error":              "Page not found",
	})
}

func (c *Controller) Event(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")

	if !models.IsValidUUID(eventID) {
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	var event *models.Event

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	localizer := c.Localizer(&event.ChatID)
	timeT := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "WebUpdatedAt",
		},
		TemplateData: map[string]string{
			"Time": time.Now().Format("2006-01-02 15:04:05"),
		},
	})

	for i, bg := range event.BoardGames {
		if bg.Name == models.PLAYER_COUNTER {
			event.BoardGames[i].Name = localizer.MustLocalizeMessage(&i18n.Message{ID: "JoinEvent"})
		}
	}

	// serve an html file
	ctx.HTML(http.StatusOK, "event", gin.H{
		"Id":             event.ID,
		"Title":          event.Name,
		"StartsAt":       event.FormatStartAt(),
		"Location":       event.Location,
		"Games":          event.BoardGames,
		"UpdatedAt":      timeT,
		"NoParticipants": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebNoParticipants"}),
		"Players":        localizer.MustLocalizeMessage(&i18n.Message{ID: "WebPlayers"}),
		"Join":           localizer.MustLocalizeMessage(&i18n.Message{ID: "WebJoin"}),
		"AddGame":        localizer.MustLocalizeMessage(&i18n.Message{ID: "WebAddGame"}),
		"Welcome":        localizer.MustLocalizeMessage(&i18n.Message{ID: "WebWelcome"}),
		"AddNewGame":     localizer.MustLocalizeMessage(&i18n.Message{ID: "WebAddNewGame"}),
		"GameName":       localizer.MustLocalizeMessage(&i18n.Message{ID: "WebGameName"}),
		"MaxPlayers":     localizer.MustLocalizeMessage(&i18n.Message{ID: "WebMaxPlayers"}),
	})
}

func (c *Controller) Game(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")
	gameID, err2 := strconv.ParseInt(ctx.Param("game_id"), 10, 64)
	if err2 != nil {
		c.renderError(ctx, nil, nil, "Invalid game ID")
		return
	}

	if !models.IsValidUUID(eventID) {
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	var event *models.Event
	var game *models.BoardGame

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	localizer := c.Localizer(&event.ChatID)

	for _, g := range event.BoardGames {
		if g.ID == gameID {
			game = &g
			break
		}
	}

	if game.Name == models.PLAYER_COUNTER {
		game.Name = localizer.MustLocalizeMessage(&i18n.Message{ID: "JoinEvent"})
	}

	ctx.HTML(http.StatusOK, "game_info", gin.H{
		"Id":                      event.ID,
		"Title":                   event.Name,
		"StartsAt":                event.FormatStartAt(),
		"Location":                event.Location,
		"Game":                    game,
		"NoParticipants":          localizer.MustLocalizeMessage(&i18n.Message{ID: "WebNoParticipants"}),
		"Players":                 localizer.MustLocalizeMessage(&i18n.Message{ID: "WebPlayers"}),
		"MaxPlayers":              localizer.MustLocalizeMessage(&i18n.Message{ID: "WebMaxPlayers"}),
		"UpdateGame":              localizer.MustLocalizeMessage(&i18n.Message{ID: "WebUpdateGame"}),
		"Update":                  localizer.MustLocalizeMessage(&i18n.Message{ID: "Update"}),
		"UnlinkFormBoardGameGeek": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebUnlinkFormBoardGameGeek"}),
		"GameDeletedSuccessfully": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebGameDeletedSuccessfully"}),
		"DeleteGameConfirmation":  localizer.MustLocalizeMessage(&i18n.Message{ID: "WebDeleteGameConfirmation"}),
		"FailedToDeleteGame":      localizer.MustLocalizeMessage(&i18n.Message{ID: "WebFailedToDeleteGame"}),
		"Delete":                  localizer.MustLocalizeMessage(&i18n.Message{ID: "WebDelete"}),
	})
}

func (c *Controller) UpdateGame(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")
	gameID, err2 := strconv.ParseInt(ctx.Param("game_id"), 10, 64)
	if err2 != nil {
		c.renderError(ctx, nil, nil, "Invalid game ID")
		return
	}

	if !models.IsValidUUID(eventID) {
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	var bg models.UpdateGameRequest
	if err = ctx.ShouldBind(&bg); err != nil {
		log.Println("failed to bind form:", err)
		c.renderError(ctx, nil, nil, "Invalid submitted form")
		return
	}

	var event *models.Event
	var game *models.BoardGame

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	if event.Locked && event.UserID != bg.UserID {
		log.Println("event is locked")
		c.renderError(ctx, &event.ID, &event.ChatID, "Unable to add game to locked event")
		return
	}

	for _, g := range event.BoardGames {
		if g.ID == gameID {
			game = &g
			break
		}
	}

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
			c.renderError(ctx, &eventID, &event.ChatID, "Invalid bgg url")
			return
		}

		var bgMaxPlayers *int

		if bgMaxPlayers, bgName, bgUrl, bgImageUrl, err = models.ExtractGameInfo(bgCtx, c.BGG, id, game.Name); err != nil {
			log.Printf("Failed to get game %d: %v", id, err)
		}
		if bgMaxPlayers != nil {
			maxPlayers = int(*bgMaxPlayers)
		}
	}

	if err = c.DB.UpdateBoardGameBGGInfoByID(gameID, maxPlayers, bgID, bgName, bgUrl, bgImageUrl); err != nil {
		log.Println("failed to update board game:", err)
		c.renderError(ctx, &eventID, &event.ChatID, "Failed to update board game")
		return
	}

	if event, err = c.updateTelegram(ctx, eventID); err != nil {
		log.Println("failed to update telegram", err)
	}

	for _, g := range event.BoardGames {
		if g.ID == gameID {
			game = &g
			break
		}
	}

	localizer := c.Localizer(&event.ChatID)
	if game.Name == models.PLAYER_COUNTER {
		game.Name = localizer.MustLocalizeMessage(&i18n.Message{ID: "JoinEvent"})
	}

	ctx.HTML(http.StatusOK, "game_info", gin.H{
		"Id":                      event.ID,
		"Title":                   event.Name,
		"StartsAt":                event.FormatStartAt(),
		"Location":                event.Location,
		"Game":                    game,
		"NoParticipants":          localizer.MustLocalizeMessage(&i18n.Message{ID: "WebNoParticipants"}),
		"Players":                 localizer.MustLocalizeMessage(&i18n.Message{ID: "WebPlayers"}),
		"MaxPlayers":              localizer.MustLocalizeMessage(&i18n.Message{ID: "WebMaxPlayers"}),
		"UpdateGame":              localizer.MustLocalizeMessage(&i18n.Message{ID: "WebUpdateGame"}),
		"Update":                  localizer.MustLocalizeMessage(&i18n.Message{ID: "Update"}),
		"UnlinkFormBoardGameGeek": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebUnlinkFormBoardGameGeek"}),
		"GameDeletedSuccessfully": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebGameDeletedSuccessfully"}),
		"DeleteGameConfirmation":  localizer.MustLocalizeMessage(&i18n.Message{ID: "WebDeleteGameConfirmation"}),
		"FailedToDeleteGame":      localizer.MustLocalizeMessage(&i18n.Message{ID: "WebFailedToDeleteGame"}),
		"Delete":                  localizer.MustLocalizeMessage(&i18n.Message{ID: "WebDelete"}),
	})

	c.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HooksWebhookTypeUpdateGame,
		Data: models.HookUpdateGamePayload{
			ID:         gameID,
			EventID:    event.ID,
			UserID:     bg.UserID,
			UserName:   bg.UserName,
			Name:       game.Name,
			MaxPlayers: *bg.MaxPlayers,
			MessageID:  nil,
			BGG: models.HookBGGInfo{
				IsSet:    bgID != nil,
				ID:       bgID,
				Name:     bgName,
				URL:      bgUrl,
				ImageURL: game.BggImageUrl,
			},
			UpdatedAt: time.Now(),
		},
	})
}

func (c *Controller) DeleteGame(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")
	log.Println("Deleting game for event:", eventID)
	gameID, err2 := strconv.ParseInt(ctx.Param("game_id"), 10, 64)
	if err2 != nil {
		c.renderError(ctx, nil, nil, "Invalid game ID")
		return
	}
	userID, err2 := strconv.ParseInt(ctx.Query("user_id"), 10, 64)
	if err2 != nil {
		c.renderError(ctx, nil, nil, "Invalid user ID")
		return
	}

	username := ctx.Query("username")

	if !models.IsValidUUID(eventID) {
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	var event *models.Event
	var game *models.BoardGame

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	if event.Locked && event.UserID != userID {
		log.Println("event is locked")
		c.renderError(ctx, &event.ID, &event.ChatID, "Unable to delete game to locked event")
		return
	}

	for _, g := range event.BoardGames {
		if g.ID == gameID {
			game = &g
			break
		}
	}

	if game == nil {
		c.renderError(ctx, &event.ID, &event.ChatID, "Invalid game ID")
		return
	}

	if err = c.DB.DeleteBoardGameByID(gameID); err != nil {
		log.Println("failed to delete board game:", err)
		c.renderError(ctx, &event.ID, &event.ChatID, "Failed to delete board game")
		return
	}

	to := &telebot.Chat{
		ID: event.ChatID,
	}

	message := c.Localizer(&event.ChatID).MustLocalize(&i18n.LocalizeConfig{
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
	if _, err = c.Bot.Send(to, message, options); err != nil {
		log.Println("failed to send message:", err)
	}

	log.Printf("Game %s deleted from event %s", game.Name, event.Name)
	if _, err = c.updateTelegram(ctx, eventID); err != nil {
		log.Println("failed to update telegram", err)
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Game deleted."})

	c.Hook.SendAllWebhookAsync(ctx, event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeDeleteGame,
		Data: models.HookDeleteGamePayload{
			ID:        game.ID,
			EventID:   event.ID,
			Name:      game.Name,
			UserName:  username,
			DeletedAt: time.Now().Format("2006-01-02 15:04:05"),
		},
	})
}

func (c *Controller) AddGame(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")

	if !models.IsValidUUID(eventID) {
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	var event *models.Event

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		c.renderError(ctx, nil, nil, "Invalid event ID")
		return
	}

	var bg models.AddGameRequest
	if err = ctx.ShouldBind(&bg); err != nil {
		log.Println("failed to bind form:", err)
		c.renderError(ctx, &event.ID, &event.ChatID, "Invalid submitted form data")
		return
	}

	if event.Locked && event.UserID != bg.UserID {
		log.Println("event is locked")
		c.renderError(ctx, &event.ID, &event.ChatID, "Unable to add game to locked event")
		return
	}

	bgCtx := context.Background()

	var bgID *int64
	var bgName, bgUrl, bgImageUrl *string
	if bg.BggUrl != nil && *bg.BggUrl != "" {
		var valid bool
		var id int64
		if id, valid = models.ExtractBoardGameID(*bg.BggUrl); !valid {
			c.renderError(ctx, &event.ID, &event.ChatID, "Invalid bgg url")

			return
		}

		var bgMaxPlayers *int

		if bgMaxPlayers, bgName, bgUrl, bgImageUrl, err = models.ExtractGameInfo(bgCtx, c.BGG, id, bg.Name); err != nil {
			log.Printf("Failed to get game %d: %v", id, err)
		} else {
			bgID = &id
			if bg.MaxPlayers == nil || *bg.MaxPlayers == 0 {
				bg.MaxPlayers = bgMaxPlayers
			}
		}
	} else {
		log.Printf("Searching for game %s", bg.Name)
		var results []gobgg.SearchResult

		if results, err = c.BGG.Search(bgCtx, bg.Name); err != nil {
			log.Printf("Failed to search game %s: %v", bg.Name, err)
		}

		if len(results) == 0 {
			log.Printf("Game %s not found", bg.Name)
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

			log.Printf("Game %s id %d found: %s", bg.Name, *bgID, *bgUrl)

			var things []gobgg.ThingResult

			if things, err = c.BGG.GetThings(bgCtx, gobgg.GetThingIDs(*bgID)); err != nil {
				log.Printf("Failed to get game %s: %v", bg.Name, err)
			}

			if len(things) > 0 {
				log.Printf("Game details of %s found", bg.Name)
				if things[0].MaxPlayers > 0 && bg.MaxPlayers == nil {
					bg.MaxPlayers = &things[0].MaxPlayers
				}

				if things[0].Name != "" {
					bgName = &things[0].Name
				} else {
					bgName = &bg.Name
				}
				if things[0].Image != "" {
					bgImageUrl = &things[0].Image
				}
			}
		}
	}

	if bg.MaxPlayers == nil {
		defaultMax := 5
		bg.MaxPlayers = &defaultMax
	}

	log.Printf("Inserting %s in the db", bg.Name)

	var boardGameID int64
	if boardGameID, err = c.DB.InsertBoardGame(event.ID, bg.Name, *bg.MaxPlayers, bgID, bgName, bgUrl, bgImageUrl); err != nil {
		log.Println("failed to insert board game:", err)
		c.renderError(ctx, &event.ID, &event.ChatID, "Failed to insert board game")
		return
	}

	if event, err = c.updateTelegram(ctx, eventID); err != nil {
		log.Println("failed to update telegram", err)
	}

	var game *models.BoardGame
	for _, g := range event.BoardGames {
		if g.Name == bg.Name {
			game = &g
			break
		}
	}

	localizer := c.Localizer(&event.ChatID)

	ctx.HTML(http.StatusOK, "game_info", gin.H{
		"Id":                      event.ID,
		"Title":                   event.Name,
		"StartsAt":                event.FormatStartAt(),
		"Location":                event.Location,
		"Game":                    game,
		"NoParticipants":          localizer.MustLocalizeMessage(&i18n.Message{ID: "WebNoParticipants"}),
		"Players":                 localizer.MustLocalizeMessage(&i18n.Message{ID: "WebPlayers"}),
		"MaxPlayers":              localizer.MustLocalizeMessage(&i18n.Message{ID: "WebMaxPlayers"}),
		"UpdateGame":              localizer.MustLocalizeMessage(&i18n.Message{ID: "WebUpdateGame"}),
		"Update":                  localizer.MustLocalizeMessage(&i18n.Message{ID: "Update"}),
		"UnlinkFormBoardGameGeek": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebUnlinkFormBoardGameGeek"}),
		"GameDeletedSuccessfully": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebGameDeletedSuccessfully"}),
		"DeleteGameConfirmation":  localizer.MustLocalizeMessage(&i18n.Message{ID: "WebDeleteGameConfirmation"}),
		"FailedToDeleteGame":      localizer.MustLocalizeMessage(&i18n.Message{ID: "WebFailedToDeleteGame"}),
		"Delete":                  localizer.MustLocalizeMessage(&i18n.Message{ID: "WebDelete"}),
	})

	c.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeNewGame,
		Data: models.HookNewGamePayload{
			ID:         boardGameID,
			EventID:    event.ID,
			UserID:     event.UserID,
			UserName:   event.UserName,
			Name:       bg.Name,
			MaxPlayers: *bg.MaxPlayers,
			MessageID:  nil,
			BGG: models.HookBGGInfo{
				IsSet:    bgID != nil,
				ID:       bgID,
				Name:     bgName,
				URL:      bgUrl,
				ImageURL: game.BggImageUrl,
			},
			CreatedAt: time.Now(),
		},
	})
}

func (c *Controller) AddPlayer(ctx *gin.Context) {
	var err error
	eventID := ctx.Param("event_id")

	if !models.IsValidUUID(eventID) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	var addPlayer models.AddPlayerRequest
	if err = ctx.ShouldBindJSON(&addPlayer); err != nil {
		log.Println("failed to bind form:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	var participantID int64
	if participantID, err = c.DB.InsertParticipant(eventID, addPlayer.GameID, addPlayer.UserID, addPlayer.UserName); err != nil {
		log.Println("failed to add user to participants table:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	if _, err = c.updateTelegram(ctx, eventID); err != nil {
		log.Println("failed to update telegram", err)
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Player added."})

	var event *models.Event
	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return
	}

	c.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeAddParticipant,
		Data: models.HookAddParticipantPayload{
			ID:       participantID,
			EventID:  event.ID,
			GameID:   addPlayer.GameID,
			UserID:   addPlayer.UserID,
			UserName: addPlayer.UserName,
			AddedAt:  time.Now(),
		},
	})
}

func P(x string) *string {
	return &x
}

func (c *Controller) updateTelegram(ctx *gin.Context, eventID string) (*models.Event, error) {
	var err error
	var event *models.Event

	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		c.renderError(ctx, &eventID, &event.ChatID, "Invalid event ID")

		return nil, err
	}

	if event.MessageID == nil {
		log.Println("event message id is nil")
		c.renderError(ctx, &eventID, &event.ChatID, "Invalid message ID")
		return nil, err
	}

	body, markup := event.FormatMsg(c.Localizer(&event.ChatID), c.Url)

	_, err = c.Bot.Edit(&telebot.Message{
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
