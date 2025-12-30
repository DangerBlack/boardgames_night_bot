package web

import (
	"boardgame-night-bot/src/bgg"
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/hooks"
	"boardgame-night-bot/src/web/api"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

func StartServer(port int, db *database.Database, bgg bgg.BGGService, bot *telebot.Bot, bundle *i18n.Bundle, hook *hooks.WebhookClient, service *api.Service) {
	var err error
	router := gin.Default()

	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*")

	controller := api.NewController(router.Group("/"), db, bgg, bot, bundle, hook, service)

	controller.InjectRoute()

	router.NoRoute(func(ctx *gin.Context) {
		controller.NoRoute(ctx)
	})

	if err = router.Run(fmt.Sprintf(":%d", port)); err != nil {
		panic(err)
	}
}
