package main

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/hooks"
	langpack "boardgame-night-bot/src/language"
	"boardgame-night-bot/src/models"
	"boardgame-night-bot/src/telegram"
	"boardgame-night-bot/src/web"
	"boardgame-night-bot/src/web/api"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"time"

	"boardgame-night-bot/src/bgg"

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
			log.Default().Println("error calling endpoint:", err)
			return
		}

		defer resp.Body.Close()
		log.Default().Println("endpoint called successfully at", time.Now())
	}
}

func InitHealthCheck(url string) {
	if url == "" {
		log.Default().Println("the HEALTH_CHECK_URL is not set in .env file")
		return
	}

	defer callEndpoint(url)()

	c := cron.New()
	_, err := c.AddFunc("@hourly", callEndpoint(url))
	if err != nil {
		log.Default().Println("error scheduling cron job:", err)
		return
	}

	c.Start()
	log.Default().Println("cron job started...")
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

	lp, err := langpack.BuildLanguagePack(".")
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

	botMiniAppURL := os.Getenv("BOT_MINI_APP_URL")
	if botMiniAppURL == "" {
		log.Fatal("the BOT_MINI_APP_URL is not set in .env file")
	}

	baseUrl := os.Getenv("BASE_URL")
	if baseUrl == "" {
		log.Fatal("the BASE_URL is not set in .env file")
	}

	healthCheckUrl := os.Getenv("HEALTH_CHECK_URL")
	InitHealthCheck(healthCheckUrl)

	bggToken := os.Getenv("BGG_TOKEN")
	if bggToken == "" {
		log.Fatal("the BGG_TOKEN is not set in .env file")
	}

	portString := StringOrDefault(os.Getenv("PORT"), "8080")
	port, err := strconv.Atoi(portString)
	if err != nil {
		log.Fatal("the PORT is not set in .env file or is not a valid number")
	}

	httpTimeoutString := StringOrDefault(os.Getenv("HTTP_TIMEOUT"), "10s")
	httpTimeoutDuration, err := time.ParseDuration(httpTimeoutString)
	if err != nil {
		log.Fatal("the HTTP_TIMEOUT is not set in .env file or is not a valid duration")
	}

	httpMaxAttemptString := StringOrDefault(os.Getenv("HTTP_MAX_ATTEMPT"), "3")
	httpMaxAttempt, err := strconv.Atoi(httpMaxAttemptString)
	if err != nil {
		log.Fatal("the HTTP_MAX_ATTEMPT is not set in .env file or is not a valid number")
	}

	failureExpirationString := StringOrDefault(os.Getenv("FAILURE_EXPIRATION"), "10m")
	failureExpirationDuration, err := time.ParseDuration(failureExpirationString)
	if err != nil {
		log.Fatal("the FAILURE_EXPIRATION is not set in .env file or is not a valid duration")
	}

	maxFailureAttemptsString := StringOrDefault(os.Getenv("MAX_FAILURE_ATTEMPTS"), "5")
	maxFailureAttempts, err := strconv.Atoi(maxFailureAttemptsString)
	if err != nil {
		log.Fatal("the MAX_FAILURE_ATTEMPTS is not set in .env file or is not a valid number")
	}

	dbPath := StringOrDefault(os.Getenv("DB_PATH"), "./archive")

	db := database.NewDatabase(dbPath)

	defer db.Close()

	log.Default().Println("database connection established.")

	db.CreateTables()
	db.MigrateToV1()
	db.MigrateToV2()
	db.MigrateToV3()

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
	bggClient := gobgg.NewBGGClient(
		gobgg.SetClient(client),
		gobgg.SetBearerToken(bggToken),
	)

	bggService := bgg.NewBGGService(bggClient)

	wh := hooks.NewWebhookClient(db, httpTimeoutDuration, httpMaxAttempt, failureExpirationDuration, maxFailureAttempts)

	service := api.NewService(db, bggService, bot, bundle, models.WebUrl{
		BotMiniAppURL: botMiniAppURL,
		BaseUrl:       baseUrl,
	})

	telegram := telegram.Telegram{
		Bot:            bot,
		DB:             db,
		LanguageBundle: bundle,
		LanguagePack:   lp,
		Url: models.WebUrl{
			BaseUrl:       baseUrl,
			BotMiniAppURL: botMiniAppURL,
		},
		Hook:    wh,
		Service: service,
	}

	log.Default().Println("bot started")

	telegram.SetupHandlers()

	go func() {
		log.Default().Println("server started")
		web.StartServer(port, db, bggService, bot, bundle, wh, service)
		log.Default().Println("server stopped")
	}()
	go func() {
		log.Default().Println("bot started")
		bot.Start()
		log.Default().Println("bot stopped")
	}()

	<-signalChan
	log.Default().Println("shutdown signal received.")

	// Gracefully stop the server and bot
	gracefulShutdown(bot)

	log.Default().Println("shutdown complete.")
}

func gracefulShutdown(bot *telebot.Bot) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop the bot
	go func() {
		bot.Stop() // Assuming bot has a Stop() method
		log.Default().Println("bot shutdown completed.")
	}()

	// Wait for the shutdown timeout or for cleanup to finish
	<-shutdownCtx.Done()
}
