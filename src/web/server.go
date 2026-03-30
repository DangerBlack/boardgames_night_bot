package web

import (
	"boardgame-night-bot/src/bgg"
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/hooks"
	"boardgame-night-bot/src/web/api"
	"fmt"
	"html/template"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"gopkg.in/telebot.v3"
)

func StartServer(port int, db *database.Database, bgg bgg.BGGService, bot *telebot.Bot, bundle *i18n.Bundle, hook *hooks.WebhookClient, service *api.Service) {
	var err error
	router := gin.Default()

	router.Use(gin.Logger())

	// Add custom template functions
	router.SetFuncMap(template.FuncMap{
		"int": func(v any) int {
			switch val := v.(type) {
			case int:
				return val
			case int64:
				return int(val)
			case int32:
				return int(val)
			case float64:
				return int(val)
			case float32:
				return int(val)
			case string:
				i, err := strconv.Atoi(val)
				if err != nil {
					log.Printf("Warning: failed to convert string to int: %v", err)
					return 0
				}
				return i
			default:
				log.Printf("Warning: unsupported type for int conversion: %T", v)
				return 0
			}
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"queuedText": func(num int, format string) string {
			// Replace {{.Number}} with the actual number
			return strings.ReplaceAll(format, "{{.Number}}", fmt.Sprintf("%d", num))
		},
	})

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
