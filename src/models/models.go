package models

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/DangerBlack/gobgg"

	"github.com/google/uuid"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

const PLAYER_COUNTER = "_PLAYER_COUNTER_"

type Event struct {
	ID         string
	ChatID     int64
	UserID     int64
	UserName   string
	MessageID  *int64
	Name       string
	BoardGames []BoardGame
	Locked     bool
	Location   *string
	StartsAt   *time.Time
}

type AddPlayerRequest struct {
	GameID   int64  `json:"game_id" binding:"required"`
	UserID   int64  `json:"user_id" binding:"required"`
	UserName string `json:"user_name" binding:"required"`
}

type BoardGame struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	MaxPlayers   int64         `json:"max_players"`
	Participants []Participant `json:"participants"`
	BggID        *int64        `json:"bgg_id"`
	BggName      *string       `json:"bgg_name"`
	BggUrl       *string       `json:"bgg_url"`
	BggImageUrl  *string       `json:"bgg_image_url"`
}

type AddGameRequest struct {
	Name       string  `json:"name" form:"name" binding:"required"`
	MaxPlayers *int    `json:"max_players" form:"max_players"`
	BggUrl     *string `json:"bgg_url" form:"bgg_url"`
	UserID     int64   `json:"user_id" form:"user_id"`
}

type UpdateGameRequest struct {
	MaxPlayers *int    `json:"max_players" form:"max_players"`
	BggUrl     *string `json:"bgg_url" form:"bgg_url"`
	UserID     int64   `json:"user_id" form:"user_id"`
	Unlink     string  `json:"unlink" form:"unlink"`
}

type Participant struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	UserName string `json:"user_name"`
}

// create enum with value add_player
type EventAction string

const (
	AddPlayer EventAction = "$add_player"
	Cancel    EventAction = "$cancel"
)

func (e Event) FormatBG(localizer *i18n.Localizer, baseUrl string, botName string, bg BoardGame) (string, telebot.InlineButton, error) {
	msg := ""

	complete := ""
	isComplete := len(bg.Participants) == int(bg.MaxPlayers)
	if isComplete {
		complete = "🚫"
	}

	link := ""
	if bg.BggUrl != nil && bg.BggName != nil && *bg.BggUrl != "" && *bg.BggName != "" {
		link = fmt.Sprintf(" - <a href='%s'>%s</a>\n", *bg.BggUrl, *bg.BggName)
	}

	name := bg.Name
	if bg.Name == PLAYER_COUNTER {
		name = localizer.MustLocalizeMessage(&i18n.Message{ID: "JoinEvent"})
	}

	maxPlayer := bg.MaxPlayers
	players := fmt.Sprintf("(%d/%d %s)", len(bg.Participants), bg.MaxPlayers, localizer.MustLocalizeMessage(&i18n.Message{ID: "Players"}))
	if maxPlayer == -1 {
		players = fmt.Sprintf("(%d %s)", len(bg.Participants), localizer.MustLocalizeMessage(&i18n.Message{ID: "Players"}))
	}

	msg += fmt.Sprintf("🎲 <b>%s [%s]</b> %s %s\n", link, name, players, complete)
	for _, p := range bg.Participants {
		msg += " - " + p.UserName + "\n"
	}
	msg += "\n"

	joinT := localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "Join",
		},
		TemplateData: map[string]string{
			"Name": bg.Name,
		},
	})

	if bg.Name == PLAYER_COUNTER {
		joinT = localizer.MustLocalizeMessage(&i18n.Message{ID: "JoinEvent"})
	}

	btn := telebot.InlineButton{
		Text:   joinT,
		Unique: string(AddPlayer),
		Data:   fmt.Sprintf("%s|%d", e.ID, bg.ID),
	}

	return msg, btn, nil
}

func (e Event) FormatMsg(localizer *i18n.Localizer, baseUrl string, botName string) (string, *telebot.ReplyMarkup) {
	btns := []telebot.InlineButton{}

	msg := "📆 <b>" + e.Name + "</b>\n\n"
	for _, bg := range e.BoardGames {
		bgMsg, btn, err := e.FormatBG(localizer, baseUrl, botName, bg)
		if err != nil {
			log.Printf("Failed to format board game: %v", err)
			continue
		}

		msg += bgMsg

		btns = append(btns, btn)

	}

	msg += localizer.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "UpdatedAt",
		},
		TemplateData: map[string]string{
			"Time": time.Now().Format("2006-01-02 15:04:05"),
		},
	})

	btn := telebot.InlineButton{
		Text:   localizer.MustLocalizeMessage(&i18n.Message{ID: "NotComing"}),
		Unique: string(Cancel),
		Data:   e.ID,
	}

	btns = append(btns, btn)

	if e.ChatID > 0 {
		btn2 := telebot.InlineButton{
			Text: localizer.MustLocalizeMessage(&i18n.Message{ID: "AddGame"}),
			WebApp: &telebot.WebApp{
				URL: fmt.Sprintf("%s/events/%s/", baseUrl, e.ID),
			},
		}
		btns = append(btns, btn2)
	} else {
		btn2 := telebot.InlineButton{
			Text: localizer.MustLocalizeMessage(&i18n.Message{ID: "AddGame"}),
			URL:  fmt.Sprintf("https://t.me/%s/home?startapp=%s", botName, e.ID),
		}
		btns = append(btns, btn2)

	}

	markup := &telebot.ReplyMarkup{}
	markup.InlineKeyboard = [][]telebot.InlineButton{}
	for _, btn := range btns {
		markup.InlineKeyboard = append(markup.InlineKeyboard, []telebot.InlineButton{btn})
	}

	return msg, markup
}

func ExtractBoardGameID(inputURL string) (int64, bool) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return 0, false
	}

	// Ensure the scheme is HTTPS and the host is correct
	if parsedURL.Scheme != "https" || parsedURL.Host != "boardgamegeek.com" {
		return 0, false
	}

	// Define regex to extract the ID
	pattern := `^/boardgame/(\d+)/?[a-zA-Z0-9-]*$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(parsedURL.Path)

	if len(matches) > 1 {
		id, err := strconv.ParseInt(matches[1], 10, 64)
		return id, err == nil
	}
	return 0, false
}

func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func ExtractGameInfo(ctx context.Context, BGG *gobgg.BGG, id int64, gameName string) (*int, *string, *string, *string, error) {
	var err error
	var bgUrl, bgName, bgImageUrl *string
	var maxPlayers *int
	url := fmt.Sprintf("https://boardgamegeek.com/boardgame/%d", id)
	bgUrl = &url

	var things []gobgg.ThingResult

	if things, err = BGG.GetThings(ctx, gobgg.GetThingIDs(id)); err != nil {
		log.Printf("Failed to get game %d: %v", id, err)
		return nil, nil, nil, nil, err
	}

	if len(things) > 0 {
		maxPlayers = &things[0].MaxPlayers
		if things[0].Name != "" {
			bgName = &things[0].Name
		} else {
			bgName = &gameName
		}
		if things[0].Image != "" {
			bgImageUrl = &things[0].Image
		}
	}

	return maxPlayers, bgName, bgUrl, bgImageUrl, nil
}

const MessageUnchangedErrorMessage = "specified new message content and reply markup are exactly the same as a current content and reply markup of the message"
