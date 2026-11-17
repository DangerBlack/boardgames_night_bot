package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"boardgame-night-bot/src/models"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// TestTemplatesRender executes every page touched by email registration with
// representative data, so template errors surface in CI instead of in prod.
func TestTemplatesRender(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	tmpl := template.Must(template.New("").Funcs(TemplateFuncs(bundle)).ParseGlob("../../templates/*"))

	T := map[string]string{"WebLoginTitle": "Sign in", "WebAllowExternal": "allow"}
	user := "Bob"
	tg := "bob_tg"
	email := "bob@example.com"
	games := []models.BoardGame{{ID: 1, Name: "Catan", MaxPlayers: 5, Participants: []models.Participant{
		{UserID: 5, UserName: "tg_user"},
		{UserID: -3, UserName: "Web Person"},
	}}}

	cases := []struct {
		name string
		data map[string]any
		want []string
	}{
		{"event", map[string]any{"Id": "e1", "Title": `Night "1"`, "Games": games, "ShareURL": "https://x/events/e1", "MiniAppURL": "https://t.me/b/app?startapp=e1", "ExternalMarker": "✉️", "T": T, "QueuedLang": "en"},
			[]string{"https://x/events/e1", "Web Person ✉️", "tg-init-data"}},
		{"game_info", map[string]any{"Id": "e1", "Game": games[0]}, []string{"Web Person ✉️", "tg-init-data"}},
		{"new_event", map[string]any{"ChatID": "1", "AllowExternal": "allow"}, []string{`name="allow_external"`, "tgInitData"}},
		{"login", map[string]any{"Next": "/events/e1", "EmailEnabled": true, "T": T}, []string{`value="/events/e1"`}},
		{"verify", map[string]any{"Token": "abc", "Next": "/account", "T": T}, []string{`value="abc"`}},
		{"message", map[string]any{"Title": "t", "Body": "b", "Next": "/login", "T": T}, []string{"/login"}},
		{"account", map[string]any{"State": "pending", "Email": &email, "Next": "/account", "T": T}, []string{"/account/username", "/account/link-telegram"}},
		{"account", map[string]any{"State": "separated", "Username": &user, "Email": &email, "Next": "/events/e1", "T": T}, []string{"Bob", "/account/link-telegram"}},
		{"account", map[string]any{"State": "linked", "TelegramName": &tg, "Email": &email, "Next": "/account", "T": T}, []string{"bob_tg"}},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, c.name, c.data); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, w := range c.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: output missing %q", c.name, w)
			}
		}
	}
}
