package api

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/hooks"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/utils"
	"boardgame-night-bot/src/web/limiter"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DangerBlack/gobgg"
	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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
	Service        *Service
	Limiter        *limiter.Limiter
}

func NewController(router *gin.RouterGroup, db *database.Database, bgg *gobgg.BGG, bot *telebot.Bot, LanguageBundle *i18n.Bundle, hook *hooks.WebhookClient, botMiniAppURL string, baseUrl string) *Controller {
	return &Controller{
		Router:         router,
		DB:             db,
		BGG:            bgg,
		Bot:            bot,
		LanguageBundle: LanguageBundle,
		Url: models.WebUrl{
			BaseUrl:       baseUrl,
			BotMiniAppURL: botMiniAppURL,
		},
		Hook: hook,
		Service: NewService(db, bgg, bot, LanguageBundle, models.WebUrl{
			BotMiniAppURL: botMiniAppURL,
			BaseUrl:       baseUrl,
		}),
		Limiter: limiter.NewLimiter(5, 5),
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
	c.Router.POST("/events", c.CreateEvent)
	c.Router.GET("/events/:event_id", c.GetEvent)
	c.Router.GET("/events/:event_id/games/:game_id", c.GetGame)
	c.Router.POST("/events/:event_id/games/:game_id", c.UpdateGame)
	c.Router.DELETE("/events/:event_id/games/:game_id", c.DeleteGame)
	c.Router.POST("/events/:event_id/add-game", c.AddGame)
	c.Router.POST("/events/:event_id/join", c.AddPlayer)
	c.Router.GET("/bgg/search", c.BggSearch)
	c.Router.POST(
		"/webhooks/:webhook_id",
		c.Limiter.GinHandler(),
		c.VerifyWebhook(),
		c.CheckEventID(),
		c.ListenWebhook,
	)
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

// is uuid
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func (c *Controller) Index(ctx *gin.Context) {
	action := ctx.Query("tgWebAppStartParam")
	log.Default().Printf("Index called with query param: %s", action)

	if IsValidUUID(action) {
		ctx.Redirect(http.StatusFound, fmt.Sprintf("/events/%s", action))
		return
	}

	args := strings.Split(action, "-")
	operation := args[0]

	switch operation {
	case "create_event":
		log.Default().Printf("Rendering new_event with args: %v", args)
		chatIDStr := args[1]
		chatIDInt, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			log.Default().Printf("Invalid chatID: %v", err)
			ctx.HTML(http.StatusBadRequest, "error", gin.H{"Error": "Invalid chat ID"})
			return
		}
		localizer := c.Localizer(&chatIDInt)
		var threadID string
		if len(args) > 2 {
			threadID = args[2]
		}
		ctx.HTML(http.StatusOK, "new_event", gin.H{
			"ChatID":                chatIDStr,
			"ThreadID":              threadID,
			"CreateNewEvent":        localizer.MustLocalizeMessage(&i18n.Message{ID: "WebCreateNewEvent"}),
			"Welcome":               localizer.MustLocalizeMessage(&i18n.Message{ID: "WebWelcome"}),
			"EventDetails":          localizer.MustLocalizeMessage(&i18n.Message{ID: "WebEventDetails"}),
			"EventName":             localizer.MustLocalizeMessage(&i18n.Message{ID: "WebEventName"}),
			"EventDate":             localizer.MustLocalizeMessage(&i18n.Message{ID: "WebEventDate"}),
			"EventLocation":         localizer.MustLocalizeMessage(&i18n.Message{ID: "WebEventLocation"}),
			"OnlyAuthorCanAddGames": localizer.MustLocalizeMessage(&i18n.Message{ID: "WebOnlyAuthorCanAddGames"}),
			"AllowAnyoneToJoin":     localizer.MustLocalizeMessage(&i18n.Message{ID: "WebAllowAnyoneToJoin"}),
			"CreateEvent":           localizer.MustLocalizeMessage(&i18n.Message{ID: "WebCreateEvent"}),
		})
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

func (c *Controller) CreateEvent(ctx *gin.Context) {
	var err error

	var newEvent models.CreateEventRequest
	if err = ctx.ShouldBind(&newEvent); err != nil {
		log.Println("failed to bind form:", err)
		c.renderError(ctx, nil, nil, "Invalid submitted form data")
		return
	}

	if newEvent.ThreadID != nil && *newEvent.ThreadID == 0 {
		newEvent.ThreadID = nil
	}

	if newEvent.IsLocked {
		newEvent.Name = fmt.Sprintf("🔒 %s", newEvent.Name)
	}

	var event *models.Event
	if event, err = c.Service.CreateEvent(newEvent.ChatID, newEvent.ThreadID, nil, newEvent.UserID, newEvent.UserName, newEvent.Name, newEvent.Location, newEvent.StartsAt, bool(newEvent.AllowGeneralJoin)); err != nil {
		log.Println("failed to create event:", err)
		c.renderError(ctx, nil, nil, "Failed to create event")
		return
	}

	ctx.Redirect(http.StatusFound, fmt.Sprintf("/events/%s", event.ID))
}

func (c *Controller) GetEvent(ctx *gin.Context) {
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

func (c *Controller) GetGame(ctx *gin.Context) {
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

	game = utils.PickGame(event, gameID)

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

	if event, game, err = c.Service.UpdateGame(eventID, gameID, bg.UserID, bg); err != nil {
		log.Println("failed to update game:", err)
		var chatID *int64
		if event != nil {
			chatID = &event.ChatID
		}
		c.renderError(ctx, &eventID, chatID, "Failed to update game")
		return
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
		Type: models.HookWebhookTypeUpdateGame,
		Data: models.HookUpdateGamePayload{
			ID:         game.UUID,
			EventID:    event.ID,
			UserID:     bg.UserID,
			UserName:   bg.UserName,
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

	var gameUUID string
	if gameUUID, err = c.DB.SelectGameUUIDByGameID(gameID); err != nil {
		log.Println("failed to load game UUID:", err)
		c.renderError(ctx, nil, nil, "Invalid game ID")
		return
	}

	if event, game, err = c.Service.DeleteGame(eventID, gameUUID, userID, username); err != nil {
		log.Println("failed to delete game:", err)
		var chatID *int64
		if event != nil {
			chatID = &event.ChatID
		}
		c.renderError(ctx, &eventID, chatID, "Failed to delete game")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Game deleted."})

	c.Hook.SendAllWebhookAsync(ctx, event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeDeleteGame,
		Data: models.HookDeleteGamePayload{
			ID:        game.UUID,
			EventID:   event.ID,
			Name:      game.Name,
			UserID:    userID,
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

	var bg models.AddGameRequest
	if err = ctx.ShouldBind(&bg); err != nil {
		log.Println("failed to bind form:", err)
		c.renderError(ctx, nil, nil, "Invalid submitted form data")
		return
	}

	var event *models.Event
	var game *models.BoardGame

	if event, game, err = c.Service.CreateGame(eventID, nil, bg.UserID, bg.Name, bg.MaxPlayers, bg.BggUrl); err != nil {
		log.Println("failed to add game:", err)
		var chatID *int64
		if event != nil {
			chatID = &event.ChatID
		}
		c.renderError(ctx, &eventID, chatID, "Failed to add game")
		return
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
			ID:         game.UUID,
			EventID:    event.ID,
			UserID:     event.UserID,
			UserName:   event.UserName,
			Name:       bg.Name,
			MaxPlayers: *bg.MaxPlayers,
			MessageID:  nil,
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

	var participantID string
	if participantID, err = c.Service.AddPlayer(nil, eventID, addPlayer.GameID, addPlayer.UserID, addPlayer.UserName); err != nil {
		log.Println("failed to add player:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Player added."})

	var event *models.Event
	if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
		log.Println("failed to load game:", err)
		return
	}

	game := utils.PickGame(event, addPlayer.GameID)

	c.Hook.SendAllWebhookAsync(context.Background(), event.ChatID, models.HookWebhookEnvelope{
		Type: models.HookWebhookTypeAddParticipant,
		Data: models.HookAddParticipantPayload{
			ID:       participantID,
			EventID:  event.ID,
			GameID:   game.UUID,
			UserID:   addPlayer.UserID,
			UserName: addPlayer.UserName,
			AddedAt:  time.Now(),
		},
	})
}

func P(x string) *string {
	return &x
}

func (c *Controller) VerifyWebhook() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		webhookID := ctx.Param("webhook_id")

		webhook, err := c.DB.GetWebhookByWebhookID(webhookID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid chat webhook ID"})
			return
		}

		body, err := ctx.GetRawData()
		if err != nil {
			ctx.AbortWithStatusJSON(400, gin.H{"error": "invalid body"})
			return
		}

		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		date := ctx.GetHeader("x-ms-date")
		if date == "" {
			ctx.AbortWithStatusJSON(400, gin.H{"error": "missing date"})
			return
		}

		log.Default().Printf("Verifying webhook %s at %s, secret [%s] with body: %d length", webhookID, date, webhook.Secret[0:3], len(body))

		contentHash := sha256.Sum256(body)
		contentHashHex := hex.EncodeToString(contentHash[:])

		log.Default().Printf("Computed content hash: %s", contentHashHex)

		if ctx.GetHeader("x-ms-content-sha256") != contentHashHex {
			ctx.AbortWithStatusJSON(400, gin.H{"error": "hash mismatch"})
			return
		}

		stringToSign := fmt.Sprintf("%s;%s", date, contentHashHex)

		expectedSig := hooks.ComputeHMACBase64(stringToSign, []byte(webhook.Secret))

		clientSig := ctx.GetHeader("X-BGNB-Signature")
		if clientSig == "" {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "missing signature"})
			return

		}

		if !hmac.Equal([]byte(expectedSig), []byte(clientSig)) {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "invalid signature"})
			return
		}

		reqTime, parseErr := time.Parse(time.RFC1123, date)
		if parseErr != nil {
			ctx.AbortWithStatusJSON(400, gin.H{"error": "invalid date"})
			return
		}

		if time.Since(reqTime) > 2*time.Minute {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "request too old"})
			return
		}

		ctx.Set("chat_id", webhook.ChatID)
		ctx.Set("thread_id", webhook.ThreadID)

		ctx.Next()
	}
}

func (c *Controller) CheckEventID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var err error
		var webhookEnvelope map[string]any
		if err = ctx.ShouldBindBodyWith(&webhookEnvelope, binding.JSON); err != nil {
			log.Println("failed to bind webhook json:", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		if webhookEnvelope["data"] != nil {
			data := webhookEnvelope["data"].(map[string]any)
			var eventID string
			if iEventID, ok := data["event_id"]; ok {
				eventID = iEventID.(string)

				var event *models.Event
				if event, err = c.DB.SelectEventByEventID(eventID); err != nil {
					log.Println("failed to load event:", err)
					ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
					return
				}

				chatID := ctx.GetInt64("chat_id")
				if event.ChatID != chatID {
					log.Printf("Webhook event chat ID %d does not match expected chat ID %d", event.ChatID, chatID)
					ctx.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
					return
				}
			}
		}

		ctx.Next()
	}
}

func (c *Controller) ListenWebhook(ctx *gin.Context) {
	var err error
	chatID := ctx.GetInt64("chat_id")
	threadID := ctx.GetInt64("thread_id")
	webhookID := ctx.Param("webhook_id")

	var webhookEnvelope models.HookWebhookEnvelope
	if err = ctx.ShouldBindBodyWith(&webhookEnvelope, binding.JSON); err != nil {
		log.Println("failed to bind webhook json:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
		return
	}

	log.Printf("Received webhook for chat %d: %v", chatID, webhookEnvelope)

	switch webhookEnvelope.Type {
	case models.HookWebhookTypeNewEvent:
		var payload *models.HookNewEventPayload
		if payload, err = Cast[models.HookNewEventPayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		if payload.ChatID == 0 {
			log.Printf("Setting missing ChatID in webhook payload to %d", chatID)
			payload.ChatID = chatID
		}

		if payload.ChatID != chatID {
			log.Printf("Webhook chat ID %d does not match expected chat ID %d", payload.ChatID, chatID)
			ctx.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}

		log.Printf("Processing new event webhook: %+v", payload)
		if _, err = c.Service.CreateEvent(payload.ChatID, &threadID, &payload.ID, payload.UserID, payload.UserName, payload.Name, payload.Location, payload.StartsAt, false); err != nil {
			log.Println("failed to add event from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add event"})
			return
		}
	case models.HookWebhookTypeDeleteEvent:
		var payload *models.HookDeleteEventPayload
		if payload, err = Cast[models.HookDeleteEventPayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Processing delete event webhook: %+v", payload)

		if err = c.Service.DeleteEvent(payload.EventID, payload.UserID, payload.UserName); err != nil {
			log.Println("failed to delete event from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete event"})
			return
		}
	case models.HookWebhookTypeNewGame:
		var payload *models.HookNewGamePayload
		if payload, err = Cast[models.HookNewGamePayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Processing new game webhook: %+v", payload)
		if _, _, err = c.Service.CreateGame(payload.EventID, &payload.ID, payload.UserID, payload.Name, &payload.MaxPlayers, payload.BGG.URL); err != nil {
			log.Println("failed to add game from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add game"})
			return
		}
	case models.HookWebhookTypeDeleteGame:
		var payload *models.HookDeleteGamePayload
		if payload, err = Cast[models.HookDeleteGamePayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Processing delete game webhook: %+v", payload)

		if _, _, err = c.Service.DeleteGame(payload.EventID, payload.ID, payload.UserID, payload.UserName); err != nil {
			log.Println("failed to delete game from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete game"})
			return
		}
	case models.HookWebhookTypeUpdateGame:
		var payload *models.HookUpdateGamePayload
		if payload, err = Cast[models.HookUpdateGamePayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Processing update game webhook: %+v", payload)

		gameID, err := c.DB.SelectGameIDByGameUUID(payload.ID)
		if err != nil {
			log.Println("failed to get game ID from UUID in webhook:", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update game"})
			return
		}

		unlink := ""
		if payload.BGG.URL == nil {
			unlink = "on"
		}
		if _, _, err = c.Service.UpdateGame(payload.EventID, gameID, payload.UserID, models.UpdateGameRequest{
			MaxPlayers: &payload.MaxPlayers,
			BggUrl:     payload.BGG.URL,
			UserID:     payload.UserID,
			UserName:   payload.UserName,
			Unlink:     unlink,
		}); err != nil {
			log.Println("failed to update game from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update game"})
			return
		}
	case models.HookWebhookTypeAddParticipant:
		var payload *models.HookAddParticipantPayload
		if payload, err = Cast[models.HookAddParticipantPayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Processing add participant webhook: %+v", payload)

		gameID, err := c.DB.SelectGameIDByGameUUID(payload.GameID)
		if err != nil {
			log.Println("failed to get game ID from UUID in webhook:", err)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Failed to add participant"})
			return
		}

		if _, err = c.Service.AddPlayer(&payload.ID, payload.EventID, gameID, payload.UserID, payload.UserName); err != nil {
			log.Println("failed to add participant from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add participant"})
			return
		}
	case models.HookWebhookTypeRemoveParticipant:
		var payload *models.HookRemoveParticipantPayload
		if payload, err = Cast[models.HookRemoveParticipantPayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Processing remove participant webhook: %+v", payload)

		if err = c.Service.DeletePlayer(payload.EventID, payload.UserID); err != nil {
			log.Println("failed to remove participant from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove participant"})
			return
		}
	case models.HookWebhookTypeSendMessage:
		var payload *models.HookSendMessagePayload
		if payload, err = Cast[models.HookSendMessagePayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Processing send message webhook: %+v", payload)

		name := ""
		if payload.UserName != nil {
			name = fmt.Sprintf(" @%s", *payload.UserName)
		}
		msg := fmt.Sprintf("🤖[%s%s] %s", webhookID[0:3], name, payload.Message)
		_, err = c.Bot.Send(telebot.ChatID(chatID), msg, &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
			ThreadID:  int(threadID),
		})
		if err != nil {
			log.Println("failed to send message from webhook:", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
			return
		}
	case models.HookWebhookTypeTestWebhook:
		log.Printf("Received test webhook for chat %d", chatID)
		var payload *models.HookTestPayload
		if payload, err = Cast[models.HookTestPayload](webhookEnvelope.Data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook data"})
			return
		}

		log.Printf("Test webhook payload: %s", payload.Message)
	default:
		log.Printf("Unhandled webhook type: %s", webhookEnvelope.Type)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Unhandled webhook type"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Webhook received."})
}

func Cast[T any](data any) (*T, error) {
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal payload for type %T: %v", (*T)(nil), err)
		return nil, err
	}
	var payload T
	if err = json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Printf("failed to unmarshal payload for type %T: %v", (*T)(nil), err)
		return nil, err
	}
	return &payload, nil
}
