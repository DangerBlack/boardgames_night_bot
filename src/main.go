package main

import (
	"boardgame-night-bot/src/database"
	langpack "boardgame-night-bot/src/language"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/telegram"
	"boardgame-night-bot/src/web"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"time"

	"github.com/DangerBlack/gobgg"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/robfig/cron/v3"
	"golang.org/x/text/language"
	"gopkg.in/telebot.v3"
)

func callEndpoint(url string) func() {
	return func() {
		resp, err := http.Get(url)
		if err != nil {
			log.Println("error calling endpoint:", err)
			return
		}

		defer resp.Body.Close()
		log.Println("endpoint called successfully at", time.Now())
	}
}

func InitHealthCheck(url string) {
	if url == "" {
		log.Println("the HEALTH_CHECK_URL is not set in .env file")
		return
	}

	defer callEndpoint(url)()

	c := cron.New()
	_, err := c.AddFunc("@hourly", callEndpoint(url))
	if err != nil {
		log.Println("error scheduling cron job:", err)
		return
	}

	c.Start()
	log.Println("cron job started...")
}

func StringOrDefault(s, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

func main() {
	var err error

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	lp, err := langpack.BuildLanguagePack()
	if err != nil {
		log.Fatal(err)
	}

	for _, lang := range lp.Languages {
		log.Default().Printf("Loading language file: %s", lang)
		bundle.MustLoadMessageFile(fmt.Sprintf("localization/active.%s.toml", lang))
	}

	if err = godotenv.Load(); err != nil {
		log.Default().Printf("warn loading .env file: %v", err)
	}

	botToken := os.Getenv("TOKEN")
	if botToken == "" {
		log.Fatal("the TOKEN is not set in .env file")
	}

	botName := os.Getenv("BOT_NAME")
	if botToken == "" {
		log.Fatal("the BOT_NAME is not set in .env file")
	}

	healthCheckUrl := os.Getenv("HEALTH_CHECK_URL")
	InitHealthCheck(healthCheckUrl)

	bggToken := os.Getenv("BGG_TOKEN")
	if bggToken == "" {
		log.Fatal("the BGG_TOKEN is not set in .env file")
	}

	baseUrl := os.Getenv("BASE_URL")
	portString := StringOrDefault(os.Getenv("PORT"), "8080")
	port, err := strconv.Atoi(portString)
	if err != nil {
		log.Fatal("the PORT is not set in .env file or is not a valid number")
	}

	dbPath := StringOrDefault(os.Getenv("DB_PATH"), "./archive")

	db := database.NewDatabase(dbPath)

	defer db.Close()

	log.Println("database connection established.")

	db.CreateTables()
	db.MigrateToV1()

	bot, err := telebot.NewBot(telebot.Settings{
		Token:     botToken,
		ParseMode: telebot.ModeHTML,
		Poller: &telebot.LongPoller{
			Timeout:        10 * time.Second,
			AllowedUpdates: []string{"message", "callback_query"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	bgg := gobgg.NewBGGClient(
		gobgg.SetClient(client),
		gobgg.SetBearerToken(bggToken),
	)

	telegram := telegram.Telegram{
		Bot:            bot,
		DB:             db,
		BGG:            bgg,
		LanguageBundle: bundle,
		LanguagePack:   lp,
		BaseUrl:        baseUrl,
		BotName:        botName,
	}

	log.Println("bot started")

	bot.Handle("/start", telegram.Start)
	bot.Handle("/help", telegram.Start)
	bot.Handle("/create", telegram.CreateGame)
	bot.Handle("/add_game", telegram.AddGame)
	bot.Handle("/language", telegram.SetLanguage)
	bot.Handle("/location", telegram.SetDefaultLocation)

	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if c.Message().ReplyTo == nil {
			return nil
		}

		return telegram.UpdateGameDispatcher(c)
	})

	bot.Handle(telebot.OnCallback, func(c telebot.Context) error {
		data := c.Callback().Data
		parts := strings.Split(data, "|")

		action := "$" + strings.Split(parts[0], "$")[1]

		log.Printf("User clicked on button: *%s* %d", action, len(action))
		switch action {
		case string(models.AddPlayer):
			return telegram.CallbackAddPlayer(c)
		case string(models.Cancel):
			return telegram.CallbackRemovePlayer(c)
		}

		return c.Reply("invalid action")
	})

	go func() {
		log.Println("server started")
		web.StartServer(port, db, bgg, bot, bundle, baseUrl, botName)
		log.Println("server stopped")
	}()
	go func() {
		log.Println("bot started")
		bot.Start()
		log.Println("bot stopped")
	}()

	<-signalChan
	log.Println("shutdown signal received.")

	// Gracefully stop the server and bot
	gracefulShutdown(bot)

	log.Println("shutdown complete.")
}

func gracefulShutdown(bot *telebot.Bot) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop the bot
	go func() {
		bot.Stop() // Assuming bot has a Stop() method
		log.Println("bot shutdown completed.")
	}()

	// Wait for the shutdown timeout or for cleanup to finish
	<-shutdownCtx.Done()
}
