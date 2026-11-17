package api

import (
	"boardgame-night-bot/src/auth"
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/hooks"
	"boardgame-night-bot/src/mocks"
	"boardgame-night-bot/src/models"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	langpack "boardgame-night-bot/src/language"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const httpTestBotToken = "123456:TEST-TOKEN"
const ownerTelegramID = int64(42)

type httpEnv struct {
	router  *gin.Engine
	db      *database.Database
	auth    *auth.Service
	eventID string
	gameID  int64
}

func newHTTPEnv(t *testing.T) *httpEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := database.NewDatabase(t.TempDir())
	t.Cleanup(db.Close)
	db.CreateTables()
	db.MigrateToV1()
	db.MigrateToV2()
	db.MigrateToV3()
	db.MigrateToV4()
	db.MigrateToV5()
	db.MigrateToV6()
	db.MigrateToV7()

	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	lp, err := langpack.BuildLanguagePack("../../../")
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range lp.Languages {
		bundle.MustLoadMessageFile(fmt.Sprintf("../../../localization/active.%s.toml", lang))
	}

	svc := NewService(db, mocks.NewMockBGGService(), mocks.NewMockTelegramService(), bundle, models.WebUrl{
		BaseUrl:       "https://example.com",
		BotMiniAppURL: "https://t.me/test_bot/app",
	})
	authSvc := auth.NewService(db, nil, auth.DefaultConfig(httpTestBotToken))

	router := gin.New()
	// Stubs for the helpers web.TemplateFuncs provides (importing web here would be a cycle).
	router.SetFuncMap(template.FuncMap{
		"int":        func(v any) int { return 0 },
		"add":        func(a, b int) int { return a + b },
		"sub":        func(a, b int) int { return a - b },
		"queuedText": func(n int, lang string) string { return "" },
	})
	router.LoadHTMLGlob("../../../templates/*")
	c := NewController(router.Group("/"), db, nil, nil, bundle, hooks.NewWebhookClient(db, time.Second, 1, time.Minute, 1), svc, authSvc, "https://example.com")
	c.InjectRoute()

	eventID, err := db.InsertEventWithOptionalGame(nil, 100, ownerTelegramID, "owner", "Game night", nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.UpdateEventMessageID(eventID, 1); err != nil {
		t.Fatal(err)
	}
	ev, err := db.SelectEventByEventID(eventID)
	if err != nil {
		t.Fatal(err)
	}

	return &httpEnv{router: router, db: db, auth: authSvc, eventID: eventID, gameID: ev.BoardGames[0].ID}
}

func signInitData(userID int64, username string) string {
	f := map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      fmt.Sprintf(`{"id":%d,"first_name":"T","username":%q}`, userID, username),
	}
	pairs := []string{}
	for k, v := range f {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	sec := hmac.New(sha256.New, []byte("WebAppData"))
	sec.Write([]byte(httpTestBotToken))
	m := hmac.New(sha256.New, sec.Sum(nil))
	m.Write([]byte(strings.Join(pairs, "\n")))
	q := url.Values{}
	for k, v := range f {
		q.Set(k, v)
	}
	q.Set("hash", hex.EncodeToString(m.Sum(nil)))
	return q.Encode()
}

type reqOpt func(*http.Request)

func withInitData(d string) reqOpt {
	return func(r *http.Request) { r.Header.Set(auth.InitDataHeader, d) }
}

func withSession(raw string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: raw}) }
}

func (e *httpEnv) do(method, path string, body any, opts ...reqOpt) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for _, o := range opts {
		o(req)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *httpEnv) participants(t *testing.T) []models.Participant {
	ev, err := e.db.SelectEventByEventID(e.eventID)
	if err != nil {
		t.Fatal(err)
	}
	return ev.BoardGames[0].Participants
}

func (e *httpEnv) separatedSession(t *testing.T, email, username string) (string, *models.User) {
	u, err := e.db.GetOrCreateUserByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	if username != "" {
		if err = e.db.SetSeparatedUsername(u.ID, username); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := e.auth.CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return raw, u
}

func TestJoinRequiresServerVerifiedIdentity(t *testing.T) {
	e := newHTTPEnv(t)
	join := "/events/" + e.eventID + "/join"

	// The old client-trusted payload is no longer enough.
	forged := map[string]any{"game_id": e.gameID, "user_id": 1, "user_name": "victim"}
	if w := e.do("POST", join, forged); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated join: %d %s", w.Code, w.Body)
	}

	bad := strings.Replace(signInitData(4242, "ann"), "4242", "1", 1)
	if w := e.do("POST", join, forged, withInitData(bad)); w.Code != http.StatusUnauthorized {
		t.Fatalf("tampered init data: %d %s", w.Code, w.Body)
	}

	// Valid init data wins over whatever the body claims.
	if w := e.do("POST", join, forged, withInitData(signInitData(4242, "ann"))); w.Code != http.StatusCreated {
		t.Fatalf("telegram join: %d %s", w.Code, w.Body)
	}
	p := e.participants(t)
	if len(p) != 1 || p[0].UserID != 4242 || p[0].UserName != "ann" || !p[0].IsTelegramUsername {
		t.Fatalf("participants = %+v", p)
	}
}

func TestExternalUsersNeedOwnerOptIn(t *testing.T) {
	e := newHTTPEnv(t)
	join := "/events/" + e.eventID + "/join"
	settings := "/events/" + e.eventID + "/settings"
	session, user := e.separatedSession(t, "web@example.com", "Web Person")

	if w := e.do("POST", join, map[string]any{"game_id": e.gameID}, withSession(session)); w.Code != http.StatusForbidden {
		t.Fatalf("external join on closed event: %d %s", w.Code, w.Body)
	}
	if len(e.participants(t)) != 0 {
		t.Fatal("external user was added to a closed event")
	}

	// Only the owner may open the event.
	if w := e.do("POST", settings, map[string]any{"allow_external": true}, withInitData(signInitData(4242, "ann"))); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner settings: %d %s", w.Code, w.Body)
	}
	if w := e.do("POST", settings, map[string]any{"allow_external": true}, withSession(session)); w.Code != http.StatusForbidden {
		t.Fatalf("external settings: %d %s", w.Code, w.Body)
	}
	if w := e.do("POST", settings, map[string]any{"allow_external": true}, withInitData(signInitData(ownerTelegramID, "owner"))); w.Code != http.StatusOK {
		t.Fatalf("owner settings: %d %s", w.Code, w.Body)
	}

	w := e.do("GET", "/events/"+e.eventID+"/viewer", nil, withSession(session))
	var viewer map[string]any
	json.Unmarshal(w.Body.Bytes(), &viewer)
	if viewer["can_act"] != true || viewer["external"] != true || viewer["is_owner"] != false {
		t.Fatalf("viewer = %v", viewer)
	}

	if w := e.do("POST", join, map[string]any{"game_id": e.gameID}, withSession(session)); w.Code != http.StatusCreated {
		t.Fatalf("external join on open event: %d %s", w.Code, w.Body)
	}
	p := e.participants(t)
	if len(p) != 1 || p[0].UserID != models.ExternalUserID(user.ID) || p[0].UserName != "Web Person" || p[0].IsTelegramUsername {
		t.Fatalf("participants = %+v", p)
	}

	if w := e.do("POST", "/events/"+e.eventID+"/leave", nil, withSession(session)); w.Code != http.StatusOK {
		t.Fatalf("leave: %d %s", w.Code, w.Body)
	}
	if len(e.participants(t)) != 0 {
		t.Fatal("participant not removed")
	}
}

func TestPendingAccountCannotAct(t *testing.T) {
	e := newHTTPEnv(t)
	session, _ := e.separatedSession(t, "pending@example.com", "")

	if w := e.do("POST", "/events/"+e.eventID+"/join", map[string]any{"game_id": e.gameID}, withSession(session)); w.Code != http.StatusUnauthorized {
		t.Fatalf("pending join: %d %s", w.Code, w.Body)
	}
	w := e.do("GET", "/events/"+e.eventID+"/viewer", nil, withSession(session))
	var viewer map[string]any
	json.Unmarshal(w.Body.Bytes(), &viewer)
	if viewer["authenticated"] != false || viewer["account_state"] != "pending" {
		t.Fatalf("viewer = %v", viewer)
	}
}

func TestLinkedAccountActsAsTelegramUser(t *testing.T) {
	e := newHTTPEnv(t)
	session, user := e.separatedSession(t, "linked@example.com", "")
	if _, err := e.auth.LinkWithTelegramUser(user, auth.TelegramUser{ID: 5555, Username: "linked_tg"}); err != nil {
		t.Fatal(err)
	}

	// Event is closed to externals, but a linked account is a Telegram identity.
	if w := e.do("POST", "/events/"+e.eventID+"/join", map[string]any{"game_id": e.gameID}, withSession(session)); w.Code != http.StatusCreated {
		t.Fatalf("linked join: %d %s", w.Code, w.Body)
	}
	p := e.participants(t)
	if len(p) != 1 || p[0].UserID != 5555 || p[0].UserName != "linked_tg" || !p[0].IsTelegramUsername {
		t.Fatalf("participants = %+v", p)
	}
}

func TestLinkTelegramViaBotPayload(t *testing.T) {
	e := newHTTPEnv(t)
	_, user := e.separatedSession(t, "claim@example.com", "Claimer")

	// Separated user joins an open event first.
	e.db.SetEventAllowExternal(e.eventID, true)
	if _, err := e.db.InsertParticipant(nil, e.eventID, e.gameID, models.ExternalUserID(user.ID), "Claimer", false); err != nil {
		t.Fatal(err)
	}

	var refreshed []string
	e.auth.OnEventsChanged = func(ids []string) { refreshed = ids }

	payload, err := e.auth.CreateTelegramLinkToken(user)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 64 {
		t.Fatalf("start payload too long for Telegram: %d", len(payload))
	}
	// /start only inspects: it must not link anything by itself.
	pending, err := e.auth.InspectStartPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Email != "claim@example.com" {
		t.Fatalf("prompt must show the requesting email, got %q", pending.Email)
	}
	if again, _ := e.db.GetUserByID(user.ID); again.TelegramUserID != nil {
		t.Fatal("account linked without confirmation")
	}
	if _, err = e.auth.InspectStartPayload(payload); err != nil {
		t.Fatal("inspection consumed the token")
	}

	if _, err = e.auth.ConfirmLink(pending.Token, auth.TelegramUser{ID: 9999, FirstName: "Clara"}); err != nil {
		t.Fatal(err)
	}
	if _, err = e.auth.ConfirmLink(pending.Token, auth.TelegramUser{ID: 1234}); err == nil {
		t.Fatal("link token reused")
	}
	if _, err = e.auth.InspectStartPayload(payload); err == nil {
		t.Fatal("used token still inspectable")
	}

	p := e.participants(t)
	if len(p) != 1 || p[0].UserID != 9999 || p[0].UserName != "Clara" {
		t.Fatalf("participants = %+v", p)
	}
	if len(refreshed) != 1 || refreshed[0] != e.eventID {
		t.Fatalf("refreshed = %v", refreshed)
	}
}

func TestJoinRejectsGameFromAnotherEvent(t *testing.T) {
	e := newHTTPEnv(t)
	session, _ := e.separatedSession(t, "sneaky@example.com", "Sneaky")

	// A second event, closed to outsiders.
	closedID, err := e.db.InsertEventWithOptionalGame(nil, 200, ownerTelegramID, "owner", "Closed", nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	closed, _ := e.db.SelectEventByEventID(closedID)
	closedGame := closed.BoardGames[0].ID

	// Outsider is allowed in the first event only.
	e.db.SetEventAllowExternal(e.eventID, true)

	w := e.do("POST", "/events/"+e.eventID+"/join", map[string]any{"game_id": closedGame}, withSession(session))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cross-event join: %d %s", w.Code, w.Body)
	}
	closed, _ = e.db.SelectEventByEventID(closedID)
	if n := len(closed.BoardGames[0].Participants); n != 0 {
		t.Fatalf("outsider leaked into closed event: %+v", closed.BoardGames[0].Participants)
	}
}

func TestDeleteSeparatedAccountRemovesParticipations(t *testing.T) {
	e := newHTTPEnv(t)
	session, user := e.separatedSession(t, "bye@example.com", "Leaving")
	e.db.SetEventAllowExternal(e.eventID, true)
	if w := e.do("POST", "/events/"+e.eventID+"/join", map[string]any{"game_id": e.gameID}, withSession(session)); w.Code != http.StatusCreated {
		t.Fatalf("join: %d", w.Code)
	}

	req := httptest.NewRequest("POST", "/account/delete", strings.NewReader("confirm=yes"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: session})
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
	if len(e.participants(t)) != 0 {
		t.Fatal("participations of a deleted separated account must be removed")
	}
	if _, err := e.db.GetUserByID(user.ID); err == nil {
		t.Fatal("user still exists")
	}
	if _, err := e.auth.SessionUser(session); err == nil {
		t.Fatal("session survived account deletion")
	}
}

func signedWebhookRequest(t *testing.T, webhookUUID, secret string, date time.Time) *http.Request {
	t.Helper()
	body := []byte(`{"type":"test","data":{"message":"hi"}}`)
	sum := sha256.Sum256(body)
	contentHash := hex.EncodeToString(sum[:])
	d := date.UTC().Format(time.RFC1123)
	req := httptest.NewRequest("POST", "/webhooks/"+webhookUUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ms-date", d)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("X-BGNB-Signature", hooks.ComputeHMACBase64(d+";"+contentHash, []byte(secret)))
	return req
}

func TestWebhookRejectsReplayAndFutureDates(t *testing.T) {
	e := newHTTPEnv(t)
	const secret = "s3cr3t-s3cr3t-s3cr3t"
	_, webhookUUID, err := e.db.InsertWebhook(100, nil, "https://example.com/hook", secret)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	send := func(r *http.Request) int {
		w := httptest.NewRecorder()
		e.router.ServeHTTP(w, r)
		return w.Code
	}

	if code := send(signedWebhookRequest(t, *webhookUUID, secret, now)); code != http.StatusOK {
		t.Fatalf("first delivery: %d", code)
	}
	if code := send(signedWebhookRequest(t, *webhookUUID, secret, now)); code != http.StatusConflict {
		t.Fatalf("replay accepted: %d", code)
	}
	if code := send(signedWebhookRequest(t, *webhookUUID, secret, now.Add(10*time.Minute))); code != http.StatusUnauthorized {
		t.Fatalf("future-dated request accepted: %d", code)
	}
	if code := send(signedWebhookRequest(t, *webhookUUID, secret, now.Add(-5*time.Minute))); code != http.StatusUnauthorized {
		t.Fatalf("stale request accepted: %d", code)
	}
}

// Every message ID the bot and the web pages localize must exist in every
// language: MustLocalize panics at runtime on a missing key.
func TestAllAuthMessagesTranslated(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	ids := append([]string{}, webMessageIDs...)
	ids = append(ids, "EmailLoginSubject", "EmailLoginBody", "EmailLoginButton",
		"LinkSuccess", "LinkFailed", "LinkTelegramTaken", "LinkPrivateOnly", "LinkOpenAccount",
		"LinkConfirmQuestion", "LinkConfirmButton", "LinkCancelButton", "LinkCancelled", "ShareEvent")
	for _, lang := range []string{"en", "it", "de"} {
		mf, err := bundle.LoadMessageFile(fmt.Sprintf("../../../localization/active.%s.toml", lang))
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for _, m := range mf.Messages {
			have[m.ID] = true
		}
		for _, id := range ids {
			if !have[id] {
				t.Errorf("%s: missing message %s", lang, id)
			}
		}
	}
}

func TestVerifyPageShowsTargetEmail(t *testing.T) {
	e := newHTTPEnv(t)
	email := "attacker@example.com"
	if err := e.db.CreateAuthToken(auth.HashToken("tok"), auth.PurposeLogin, &email, nil, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	w := e.do("GET", "/auth/verify?token=tok", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), email) {
		t.Fatalf("verify page must name the account: %d", w.Code)
	}
	// Showing the page must not consume the token.
	if _, err := e.auth.LoginTokenEmail("tok"); err != nil {
		t.Fatal("GET consumed the login token")
	}
	if w := e.do("GET", "/auth/verify?token=nope", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown token: %d", w.Code)
	}
}
