package api

import (
	"boardgame-night-bot/src/auth"
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// webMessageIDs are the strings shared by the auth/account pages and the
// viewer panel of the event page. They are exposed to templates as .T.<ID>.
var webMessageIDs = []string{
	"WebLoginTitle", "WebLoginIntro", "WebEmail", "WebSendLink",
	"WebCheckEmailTitle", "WebCheckEmailBody", "WebTooManyRequests", "WebEmailDisabled", "WebInvalidEmail",
	"WebVerifyTitle", "WebVerifyButton", "WebLinkExpired",
	"WebAccountTitle", "WebAccountPendingIntro",
	"WebClaimTelegramTitle", "WebClaimTelegramBody", "WebClaimTelegramButton", "WebLinkThisTelegram",
	"WebSeparateTitle", "WebSeparateBody", "WebUsername", "WebSaveUsername",
	"WebUsernameTaken", "WebUsernameInvalid",
	"WebAccountLinked", "WebAccountSeparated", "WebLinkLaterBody", "WebTelegramAlreadyLinked",
	"WebSignInRequired", "WebSignedInAs", "WebSignOut", "WebSignInWithEmail", "WebOpenInTelegram",
	"WebFinishAccountSetup", "WebShareEvent", "WebShareHint", "WebCopyLink", "WebLinkCopied",
	"WebAllowExternal", "WebExternalNotAllowed", "WebLeaveEvent", "WebBack",
}

func (c *Controller) messages(localizer *i18n.Localizer) map[string]string {
	out := make(map[string]string, len(webMessageIDs))
	for _, id := range webMessageIDs {
		out[id] = localizer.MustLocalizeMessage(&i18n.Message{ID: id})
	}
	return out
}

// WebLocalizer picks the language from the browser for pages not tied to a chat.
func (c *Controller) WebLocalizer(ctx *gin.Context) *i18n.Localizer {
	return i18n.NewLocalizer(c.LanguageBundle, ctx.GetHeader("Accept-Language"), "en")
}

func (c *Controller) renderMessage(ctx *gin.Context, status int, title, body string, next string) {
	ctx.HTML(status, "message", gin.H{
		"Title": title,
		"Body":  body,
		"Next":  next,
		"T":     c.messages(c.WebLocalizer(ctx)),
	})
}

// ---- login ----

func (c *Controller) LoginPage(ctx *gin.Context) {
	localizer := c.WebLocalizer(ctx)
	next := auth.SafeNext(ctx.Query("next"))
	if acc := auth.GetAccount(ctx); acc != nil {
		ctx.Redirect(http.StatusFound, next)
		return
	}
	ctx.HTML(http.StatusOK, "login", gin.H{
		"Next":         next,
		"EmailEnabled": c.Auth.EmailEnabled(),
		"T":            c.messages(localizer),
	})
}

func (c *Controller) Login(ctx *gin.Context) {
	localizer := c.WebLocalizer(ctx)
	T := c.messages(localizer)
	next := auth.SafeNext(ctx.PostForm("next"))
	email := ctx.PostForm("email")

	linkFor := func(token string) string {
		return fmt.Sprintf("%s/auth/verify?token=%s&next=%s", c.BaseURL, url.QueryEscape(token), url.QueryEscape(next))
	}
	compose := func(link string) auth.Email {
		subject := localizer.MustLocalizeMessage(&i18n.Message{ID: "EmailLoginSubject"})
		body := localizer.MustLocalizeMessage(&i18n.Message{ID: "EmailLoginBody"})
		button := localizer.MustLocalizeMessage(&i18n.Message{ID: "EmailLoginButton"})
		return auth.Email{
			Subject: subject,
			Text:    fmt.Sprintf("%s\n\n%s\n", body, link),
			HTML: fmt.Sprintf(`<p>%s</p><p><a href="%s" style="display:inline-block;padding:10px 20px;background:#007bff;color:#fff;border-radius:5px;text-decoration:none">%s</a></p><p style="color:#666;font-size:12px">%s</p>`,
				html.EscapeString(body), html.EscapeString(link), html.EscapeString(button), html.EscapeString(link)),
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 15*time.Second)
	defer cancel()

	err := c.Auth.RequestLogin(reqCtx, email, linkFor, compose)
	switch {
	case err == nil:
		c.renderMessage(ctx, http.StatusOK, T["WebCheckEmailTitle"], T["WebCheckEmailBody"], "")
	case errors.Is(err, auth.ErrInvalidEmail):
		c.renderMessage(ctx, http.StatusBadRequest, T["WebLoginTitle"], T["WebInvalidEmail"], "/login?next="+url.QueryEscape(next))
	case errors.Is(err, auth.ErrTooManyRequests):
		// Same wording as success would leak less, but a clear message is
		// friendlier and only reveals that *someone* asked for this address.
		c.renderMessage(ctx, http.StatusTooManyRequests, T["WebLoginTitle"], T["WebTooManyRequests"], "")
	case errors.Is(err, auth.ErrMailDisabled):
		c.renderMessage(ctx, http.StatusServiceUnavailable, T["WebLoginTitle"], T["WebEmailDisabled"], "")
	default:
		log.Default().Println("login request failed:", err)
		c.renderMessage(ctx, http.StatusInternalServerError, T["WebLoginTitle"], T["WebEmailDisabled"], "")
	}
}

// VerifyPage only shows a confirmation button: mail scanners prefetch GET
// links, and consuming the token on GET would burn it before the user clicks.
func (c *Controller) VerifyPage(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "verify", gin.H{
		"Token": ctx.Query("token"),
		"Next":  auth.SafeNext(ctx.Query("next")),
		"T":     c.messages(c.WebLocalizer(ctx)),
	})
}

func (c *Controller) Verify(ctx *gin.Context) {
	T := c.messages(c.WebLocalizer(ctx))
	next := auth.SafeNext(ctx.PostForm("next"))

	user, session, err := c.Auth.VerifyLogin(ctx.PostForm("token"))
	if err != nil {
		if !errors.Is(err, database.ErrInvalidOrExpiredToken) {
			log.Default().Println("login verification failed:", err)
		}
		c.renderMessage(ctx, http.StatusBadRequest, T["WebVerifyTitle"], T["WebLinkExpired"], "/login?next="+url.QueryEscape(next))
		return
	}

	auth.SetSessionCookie(ctx, session, c.Auth.Config.SessionTTL, c.SecureCookies)

	if user.State() == models.UserStatePending {
		ctx.Redirect(http.StatusSeeOther, "/account?next="+url.QueryEscape(next))
		return
	}
	ctx.Redirect(http.StatusSeeOther, next)
}

func (c *Controller) Logout(ctx *gin.Context) {
	if raw, err := ctx.Cookie(auth.SessionCookie); err == nil {
		if err = c.Auth.Logout(raw); err != nil {
			log.Default().Println("logout failed:", err)
		}
	}
	auth.ClearSessionCookie(ctx, c.SecureCookies)
	ctx.Redirect(http.StatusSeeOther, auth.SafeNext(ctx.PostForm("next")))
}

// ---- account ----

func (c *Controller) requireAccount(ctx *gin.Context) *models.User {
	acc := auth.GetAccount(ctx)
	if acc == nil {
		ctx.Redirect(http.StatusSeeOther, "/login?next="+url.QueryEscape(ctx.Request.URL.RequestURI()))
		return nil
	}
	return acc
}

func (c *Controller) AccountPage(ctx *gin.Context) {
	acc := c.requireAccount(ctx)
	if acc == nil {
		return
	}
	c.renderAccount(ctx, http.StatusOK, acc, ctx.Query("error"))
}

func (c *Controller) renderAccount(ctx *gin.Context, status int, acc *models.User, errorID string) {
	T := c.messages(c.WebLocalizer(ctx))
	errMsg := ""
	if errorID != "" {
		errMsg = T[errorID]
	}
	ctx.HTML(status, "account", gin.H{
		"T":            T,
		"Email":        acc.Email,
		"State":        string(acc.State()),
		"Username":     acc.Username,
		"TelegramName": acc.TelegramName,
		"Next":         auth.SafeNext(ctx.Query("next")),
		"Error":        errMsg,
	})
}

func (c *Controller) SetUsername(ctx *gin.Context) {
	acc := c.requireAccount(ctx)
	if acc == nil {
		return
	}
	next := auth.SafeNext(ctx.PostForm("next"))
	err := c.Auth.ChooseSeparatedUsername(acc, ctx.PostForm("username"))
	switch {
	case err == nil:
		ctx.Redirect(http.StatusSeeOther, next)
	case errors.Is(err, database.ErrUsernameTaken):
		ctx.Redirect(http.StatusSeeOther, "/account?error=WebUsernameTaken&next="+url.QueryEscape(next))
	case errors.Is(err, auth.ErrInvalidUsername):
		ctx.Redirect(http.StatusSeeOther, "/account?error=WebUsernameInvalid&next="+url.QueryEscape(next))
	default:
		log.Default().Println("set username failed:", err)
		ctx.Redirect(http.StatusSeeOther, "/account")
	}
}

// LinkTelegramViaBot sends the user to the bot with a one-time /start payload;
// the bot then links the account to whoever presses Start.
func (c *Controller) LinkTelegramViaBot(ctx *gin.Context) {
	acc := c.requireAccount(ctx)
	if acc == nil {
		return
	}
	payload, err := c.Auth.CreateTelegramLinkToken(acc)
	if err != nil {
		log.Default().Println("create link token failed:", err)
		ctx.Redirect(http.StatusSeeOther, "/account")
		return
	}
	ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("https://t.me/%s?start=%s", c.BotUsername, payload))
}

// LinkTelegramViaWebApp links the signed-in account to the Telegram user whose
// verified init data accompanies the request (account opened inside Telegram).
func (c *Controller) LinkTelegramViaWebApp(ctx *gin.Context) {
	acc := auth.GetAccount(ctx)
	if acc == nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "sign in required"})
		return
	}
	tg := auth.GetTelegramUser(ctx)
	if tg == nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "telegram identity required"})
		return
	}
	if _, err := c.Auth.LinkWithTelegramUser(acc, *tg); err != nil {
		if errors.Is(err, database.ErrTelegramAlreadyLinked) {
			ctx.JSON(http.StatusConflict, gin.H{"error": c.messages(c.WebLocalizer(ctx))["WebTelegramAlreadyLinked"]})
			return
		}
		log.Default().Println("link via webapp failed:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "unable to link account"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "linked"})
}

// ---- event viewer / settings ----

// EventViewer tells the event page what the current visitor may do. The page
// calls it with the Telegram init data (if any) so the server, not the
// client, decides who the visitor is.
func (c *Controller) EventViewer(ctx *gin.Context) {
	eventID := ctx.Param("event_id")
	if !models.IsValidUUID(eventID) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}
	event, err := c.DB.SelectEventByEventID(eventID)
	if err != nil || event.ID == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Invalid event ID"})
		return
	}

	out := gin.H{
		"authenticated":  false,
		"allow_external": event.AllowExternal,
		"email_enabled":  c.Auth.EmailEnabled(),
	}
	if acc := auth.GetAccount(ctx); acc != nil {
		out["account_state"] = string(acc.State())
	}
	if p := auth.GetPrincipal(ctx); p != nil {
		out["authenticated"] = true
		out["display_name"] = p.DisplayName
		out["external"] = p.External
		out["is_owner"] = p.UserID == event.UserID
		out["can_act"] = !p.External || event.AllowExternal
		out["via_telegram"] = auth.GetTelegramUser(ctx) != nil
	}
	ctx.JSON(http.StatusOK, out)
}

type eventSettingsRequest struct {
	AllowExternal bool `json:"allow_external"`
}

func (c *Controller) UpdateEventSettings(ctx *gin.Context) {
	p := auth.GetPrincipal(ctx)
	if p == nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "sign in required"})
		return
	}
	eventID := ctx.Param("event_id")
	if !models.IsValidUUID(eventID) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}
	var req eventSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}
	if _, err := c.Service.SetAllowExternal(eventID, p.UserID, req.AllowExternal); err != nil {
		if errors.Is(err, ErrNotEventOwner) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		log.Default().Println("failed to update event settings:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"allow_external": req.AllowExternal})
}

// principalOrAbort returns the request identity or answers 401.
func (c *Controller) principalOrAbort(ctx *gin.Context, asJSON bool) *auth.Principal {
	p := auth.GetPrincipal(ctx)
	if p != nil {
		return p
	}
	if asJSON {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "sign in required"})
	} else {
		T := c.messages(c.WebLocalizer(ctx))
		next := ctx.Request.URL.Path
		if i := strings.Index(next, "/games/"); i > 0 {
			next = next[:i]
		}
		next = strings.TrimSuffix(next, "/add-game")
		c.renderMessage(ctx, http.StatusUnauthorized, T["WebSignInRequired"], "", "/login?next="+url.QueryEscape(next))
	}
	return nil
}

func (c *Controller) externalErrorStatus(err error) int {
	if errors.Is(err, ErrExternalNotAllowed) {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}
