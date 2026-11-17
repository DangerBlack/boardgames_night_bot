package auth

import (
	"boardgame-night-bot/src/models"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	SessionCookie  = "bgn_session"
	InitDataHeader = "X-Telegram-Init-Data"
	InitDataField  = "tg_init_data"

	ctxPrincipal = "auth.principal"
	ctxAccount   = "auth.account"
	ctxTelegram  = "auth.telegram"
)

// Principal is the identity a web request acts as. The server derives it from
// verified Telegram init data or from a session cookie, never from IDs sent by
// the client.
type Principal struct {
	// UserID is the ID stored in events/participants: the Telegram user ID, or
	// the negative account ID for separated (web-only) accounts.
	UserID             int64
	DisplayName        string
	IsTelegramUsername bool
	// External is true for identities not backed by Telegram (separated accounts).
	External bool
}

// Middleware resolves the request identity and stores it in the gin context.
// It never rejects a request; handlers decide what needs authentication.
func (s *Service) Middleware(secureCookies bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var account *models.User
		if raw, err := ctx.Cookie(SessionCookie); err == nil && raw != "" {
			if u, err := s.SessionUser(raw); err == nil {
				account = u
				ctx.Set(ctxAccount, u)
			} else {
				// stale or revoked: drop it
				ClearSessionCookie(ctx, secureCookies)
			}
		}

		if initData := readInitData(ctx); initData != "" {
			tg, err := ValidateInitData(initData, s.Config.BotToken, s.Config.InitDataMaxAge, time.Now())
			if err != nil {
				log.Default().Printf("rejected telegram init data: %v", err)
			} else {
				ctx.Set(ctxTelegram, tg)
				name, isUsername := tg.DisplayName()
				ctx.Set(ctxPrincipal, &Principal{UserID: tg.ID, DisplayName: name, IsTelegramUsername: isUsername})
				ctx.Next()
				return
			}
		}

		if account != nil {
			if p := PrincipalForAccount(account); p != nil {
				ctx.Set(ctxPrincipal, p)
			}
		}

		ctx.Next()
	}
}

// PrincipalForAccount maps an account to the identity it acts as; nil while
// the account is still pending (no username chosen and no Telegram linked).
func PrincipalForAccount(u *models.User) *Principal {
	switch u.State() {
	case models.UserStateLinked:
		name := ""
		if u.TelegramName != nil {
			name = *u.TelegramName
		}
		return &Principal{UserID: *u.TelegramUserID, DisplayName: name, IsTelegramUsername: u.TelegramIsUsername}
	case models.UserStateSeparated:
		return &Principal{UserID: models.ExternalUserID(u.ID), DisplayName: *u.Username, External: true}
	}
	return nil
}

func readInitData(ctx *gin.Context) string {
	if v := ctx.GetHeader(InitDataHeader); v != "" {
		return v
	}
	ct := ctx.ContentType()
	if ctx.Request.Method == http.MethodPost && (ct == "application/x-www-form-urlencoded" || strings.HasPrefix(ct, "multipart/form-data")) {
		return ctx.PostForm(InitDataField)
	}
	return ""
}

func GetPrincipal(ctx *gin.Context) *Principal {
	if v, ok := ctx.Get(ctxPrincipal); ok {
		return v.(*Principal)
	}
	return nil
}

func GetAccount(ctx *gin.Context) *models.User {
	if v, ok := ctx.Get(ctxAccount); ok {
		return v.(*models.User)
	}
	return nil
}

func GetTelegramUser(ctx *gin.Context) *TelegramUser {
	if v, ok := ctx.Get(ctxTelegram); ok {
		return v.(*TelegramUser)
	}
	return nil
}

func SetSessionCookie(ctx *gin.Context, raw string, ttl time.Duration, secure bool) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(SessionCookie, raw, int(ttl.Seconds()), "/", "", secure, true)
}

func ClearSessionCookie(ctx *gin.Context, secure bool) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(SessionCookie, "", -1, "/", "", secure, true)
}
