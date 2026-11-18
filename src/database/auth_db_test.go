package database

import (
	"boardgame-night-bot/src/models"
	"errors"
	"testing"
	"time"
)

// newMigratedDB builds a database the way production does at boot: base
// tables, then every migration in order.
func newMigratedDB(t *testing.T) *Database {
	t.Helper()
	d := NewDatabase(t.TempDir())
	t.Cleanup(d.Close)
	d.CreateTables()
	d.MigrateToV1()
	d.MigrateToV2()
	d.MigrateToV3()
	d.MigrateToV4()
	d.MigrateToV5()
	d.MigrateToV6()
	return d
}

func TestMigrateToV7KeepsExistingDataAndIsIdempotent(t *testing.T) {
	d := newMigratedDB(t)

	// Data written by the current production version, before V7.
	eventID, err := d.InsertEventWithOptionalGame(nil, 100, 42, "alice", "Old event", nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	var gameID int64
	if err = d.db.QueryRow(`SELECT id FROM boardgames WHERE event_id = ?`, eventID).Scan(&gameID); err != nil {
		t.Fatal(err)
	}
	if _, err = d.InsertParticipant(nil, eventID, gameID, 42, "alice", true); err != nil {
		t.Fatal(err)
	}

	d.MigrateToV7()
	d.MigrateToV7() // boot twice: must be a no-op
	d.MigrateToV9() // event selects read the V9 columns

	ev, err := d.SelectEventByEventID(eventID)
	if err != nil {
		t.Fatal(err)
	}
	if ev.AllowExternal {
		t.Fatal("existing events must stay closed to external users")
	}
	if len(ev.BoardGames) != 1 || len(ev.BoardGames[0].Participants) != 1 || ev.BoardGames[0].Participants[0].UserID != 42 {
		t.Fatalf("existing participants changed: %+v", ev.BoardGames)
	}

	if err = d.SetEventAllowExternal(eventID, true); err != nil {
		t.Fatal(err)
	}
	if ev, _ = d.SelectEventByEventID(eventID); !ev.AllowExternal {
		t.Fatal("allow_external not persisted")
	}
	if err = d.SetEventAllowExternal("missing", true); !errors.Is(err, ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

func newV7DB(t *testing.T) *Database {
	d := newMigratedDB(t)
	d.MigrateToV7()
	d.MigrateToV8()
	d.MigrateToV9()
	return d
}

func TestAuthTokenIsSingleUseAndExpires(t *testing.T) {
	d := newV7DB(t)
	email := "a@example.com"

	if err := d.CreateAuthToken("h1", "login", &email, nil, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _, err := d.ConsumeAuthToken("h1", "login")
	if err != nil || got == nil || *got != email {
		t.Fatalf("first consume: %v %v", got, err)
	}
	if _, _, err = d.ConsumeAuthToken("h1", "login"); !errors.Is(err, ErrInvalidOrExpiredToken) {
		t.Fatalf("second consume should fail, got %v", err)
	}

	if err = d.CreateAuthToken("h2", "login", &email, nil, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err = d.ConsumeAuthToken("h2", "login"); !errors.Is(err, ErrInvalidOrExpiredToken) {
		t.Fatalf("expired token accepted: %v", err)
	}

	if err = d.CreateAuthToken("h3", "login", &email, nil, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, err = d.ConsumeAuthToken("h3", "link_telegram"); !errors.Is(err, ErrInvalidOrExpiredToken) {
		t.Fatalf("token accepted for wrong purpose: %v", err)
	}

	n, err := d.CountRecentAuthTokens(email, "login", time.Now().Add(-time.Hour))
	if err != nil || n != 3 {
		t.Fatalf("count = %d, %v", n, err)
	}
}

func TestUsersAndSessions(t *testing.T) {
	d := newV7DB(t)

	u1, err := d.GetOrCreateUserByEmail("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if u1.State() != models.UserStatePending {
		t.Fatalf("new user state = %s", u1.State())
	}
	again, err := d.GetOrCreateUserByEmail("bob@example.com")
	if err != nil || again.ID != u1.ID {
		t.Fatalf("second login created a new user: %v %v", again, err)
	}

	if err = d.SetSeparatedUsername(u1.ID, "Bob"); err != nil {
		t.Fatal(err)
	}
	u2, _ := d.GetOrCreateUserByEmail("carol@example.com")
	if err = d.SetSeparatedUsername(u2.ID, "bob"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("case-insensitive username clash not detected: %v", err)
	}

	if err = d.CreateSession("s1", u1.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	su, err := d.GetSessionUser("s1")
	if err != nil || su.ID != u1.ID || su.State() != models.UserStateSeparated {
		t.Fatalf("session user: %+v %v", su, err)
	}
	if err = d.CreateSession("s2", u1.ID, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = d.GetSessionUser("s2"); !errors.Is(err, ErrNoRows) {
		t.Fatalf("expired session accepted: %v", err)
	}
	if err = d.DeleteSession("s1"); err != nil {
		t.Fatal(err)
	}
	if _, err = d.GetSessionUser("s1"); !errors.Is(err, ErrNoRows) {
		t.Fatalf("deleted session accepted: %v", err)
	}
}

func TestLinkTelegramMergesSeparatedAccount(t *testing.T) {
	d := newV7DB(t)
	const tgID = int64(777)

	u, _ := d.GetOrCreateUserByEmail("dave@example.com")
	if err := d.SetSeparatedUsername(u.ID, "dave web"); err != nil {
		t.Fatal(err)
	}
	ext := models.ExternalUserID(u.ID)

	// Event A: joined only by the web identity, and owned by it.
	evA, _ := d.InsertEventWithOptionalGame(nil, 1, ext, "dave web", "A", nil, nil, true)
	a, _ := d.SelectEventByEventID(evA)
	d.InsertParticipant(nil, evA, a.BoardGames[0].ID, ext, "dave web", false)

	// Event B: joined by BOTH identities -> the Telegram one must survive.
	evB, _ := d.InsertEventWithOptionalGame(nil, 1, 1, "owner", "B", nil, nil, true)
	b, _ := d.SelectEventByEventID(evB)
	d.InsertParticipant(nil, evB, b.BoardGames[0].ID, tgID, "dave_tg", true)
	d.InsertParticipant(nil, evB, b.BoardGames[0].ID, ext, "dave web", false)

	changed, err := d.LinkTelegram(u.ID, tgID, "dave_tg", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 2 {
		t.Fatalf("changed events = %v", changed)
	}

	a, _ = d.SelectEventByEventID(evA)
	if a.UserID != tgID || a.UserName != "dave_tg" {
		t.Fatalf("event owner not moved: %d %s", a.UserID, a.UserName)
	}
	if p := a.BoardGames[0].Participants; len(p) != 1 || p[0].UserID != tgID || !p[0].IsTelegramUsername {
		t.Fatalf("event A participants: %+v", p)
	}
	b, _ = d.SelectEventByEventID(evB)
	if p := b.BoardGames[0].Participants; len(p) != 1 || p[0].UserID != tgID {
		t.Fatalf("event B participants: %+v", p)
	}

	linked, _ := d.GetUserByID(u.ID)
	if linked.State() != models.UserStateLinked || *linked.TelegramUserID != tgID {
		t.Fatalf("user not linked: %+v", linked)
	}

	// Relinking to the same Telegram user is a no-op; to another is refused.
	if _, err = d.LinkTelegram(u.ID, tgID, "dave_tg", true); err != nil {
		t.Fatalf("idempotent relink failed: %v", err)
	}
	if _, err = d.LinkTelegram(u.ID, 888, "x", false); !errors.Is(err, ErrAccountAlreadyLinked) {
		t.Fatalf("expected ErrAccountAlreadyLinked, got %v", err)
	}

	// Another email account cannot claim the same Telegram user.
	other, _ := d.GetOrCreateUserByEmail("eve@example.com")
	if _, err = d.LinkTelegram(other.ID, tgID, "dave_tg", true); !errors.Is(err, ErrTelegramAlreadyLinked) {
		t.Fatalf("expected ErrTelegramAlreadyLinked, got %v", err)
	}

	// A linked account cannot become separated.
	if err = d.SetSeparatedUsername(u.ID, "new name"); !errors.Is(err, ErrAccountAlreadyLinked) {
		t.Fatalf("expected ErrAccountAlreadyLinked, got %v", err)
	}
}

func TestForeignKeysCascadeAndOrphanCleanup(t *testing.T) {
	d := newV7DB(t)

	var fk int
	if err := d.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d, %v", fk, err)
	}

	// Cascade: deleting an event removes its games and participants.
	id, _ := d.InsertEventWithOptionalGame(nil, 1, 1, "a", "x", nil, nil, true)
	ev, _ := d.SelectEventByEventID(id)
	d.InsertParticipant(nil, id, ev.BoardGames[0].ID, 5, "p", false)
	if err := d.DeleteEvent(id); err != nil {
		t.Fatal(err)
	}
	var games, parts int
	d.db.QueryRow(`SELECT COUNT(*) FROM boardgames`).Scan(&games)
	d.db.QueryRow(`SELECT COUNT(*) FROM participants`).Scan(&parts)
	if games != 0 || parts != 0 {
		t.Fatalf("cascade failed: %d games, %d participants left", games, parts)
	}

	// Orphans created while FKs were off (as in production) are purged.
	conn, err := d.db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, q := range []string{
		`PRAGMA foreign_keys = OFF`,
		`INSERT INTO boardgames (uuid, event_id, name, max_players) VALUES ('g-orphan', 'gone', 'x', 4)`,
		`INSERT INTO participants (uuid, event_id, boardgame_id, user_id, user_name) VALUES ('p-orphan', 'gone', 9999, 7, 'x')`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err = conn.ExecContext(t.Context(), q); err != nil {
			t.Fatal(q, err)
		}
	}
	conn.Close()

	d.MigrateToV8()
	d.MigrateToV8()
	d.db.QueryRow(`SELECT COUNT(*) FROM boardgames`).Scan(&games)
	d.db.QueryRow(`SELECT COUNT(*) FROM participants`).Scan(&parts)
	if games != 0 || parts != 0 {
		t.Fatalf("orphans not purged: %d games, %d participants", games, parts)
	}
}

// Guards the production upgrade: V8 must only remove orphans. The event_id
// columns are declared INTEGER but hold text UUIDs, so a type mismatch in the
// NOT IN comparison would wipe every game and participant.
func TestMigrateToV8KeepsValidRows(t *testing.T) {
	d := newMigratedDB(t)
	id, _ := d.InsertEventWithOptionalGame(nil, 1, 1, "a", "x", nil, nil, true)
	var gid int64
	if err := d.db.QueryRow(`SELECT id FROM boardgames WHERE event_id = ?`, id).Scan(&gid); err != nil {
		t.Fatal(err)
	}
	d.InsertParticipant(nil, id, gid, 5, "p", false)
	d.MigrateToV7()
	d.MigrateToV8()
	var g, p int
	d.db.QueryRow(`SELECT COUNT(*) FROM boardgames`).Scan(&g)
	d.db.QueryRow(`SELECT COUNT(*) FROM participants`).Scan(&p)
	if g != 1 || p != 1 {
		t.Fatalf("V8 removed valid rows: games=%d participants=%d", g, p)
	}
}

func TestGameProposer(t *testing.T) {
	d := newMigratedDB(t)
	// A game created before V9 has no proposer.
	evOld, _ := d.InsertEventWithOptionalGame(nil, 1, 1, "owner", "old", nil, nil, true)
	d.MigrateToV7()
	d.MigrateToV8()
	d.MigrateToV9()
	d.MigrateToV9() // idempotent

	ev, err := d.SelectEventByEventID(evOld)
	if err != nil {
		t.Fatal(err)
	}
	if bg := ev.BoardGames[0]; bg.ProposedByID != nil || bg.ProposedByName != nil {
		t.Fatalf("pre-V9 game must have no proposer: %+v", bg)
	}

	// A separated account proposes a game it does not play.
	u, _ := d.GetOrCreateUserByEmail("frank@example.com")
	d.SetSeparatedUsername(u.ID, "frank")
	ext := models.ExternalUserID(u.ID)
	name := "frank"
	evID, _ := d.InsertEventWithOptionalGame(nil, 1, 1, "owner", "new", nil, nil, true)
	if _, _, err = d.InsertBoardGame(evID, nil, "Catan", 4, nil, nil, nil, nil, &ext, &name); err != nil {
		t.Fatal(err)
	}
	proposed := func() models.BoardGame {
		ev, err := d.SelectEventByEventID(evID)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range ev.BoardGames {
			if g.Name == "Catan" {
				return g
			}
		}
		t.Fatal("game not found")
		return models.BoardGame{}
	}
	if g := proposed(); g.ProposedByID == nil || *g.ProposedByID != ext || *g.ProposedByName != "frank" || !g.ProposedByExternal() || len(g.Participants) != 0 {
		t.Fatalf("proposer not stored apart from players: %+v", g)
	}

	// Linking Telegram moves the proposer too.
	if _, err = d.LinkTelegram(u.ID, 555, "frank_tg", true); err != nil {
		t.Fatal(err)
	}
	if g := proposed(); *g.ProposedByID != 555 || *g.ProposedByName != "frank_tg" || g.ProposedByExternal() {
		t.Fatalf("proposer not moved on link: %+v", g)
	}

	// Deleting a separated account forgets its name on proposed games.
	v, _ := d.GetOrCreateUserByEmail("gina@example.com")
	d.SetSeparatedUsername(v.ID, "gina")
	gext, gname := models.ExternalUserID(v.ID), "gina"
	d.InsertBoardGame(evID, nil, "Azul", 4, nil, nil, nil, nil, &gext, &gname)
	if _, err = d.DeleteUser(v.ID); err != nil {
		t.Fatal(err)
	}
	ev, _ = d.SelectEventByEventID(evID)
	for _, g := range ev.BoardGames {
		if g.Name == "Azul" && (g.ProposedByID != nil || g.ProposedByName != nil) {
			t.Fatalf("deleted account still shown as proposer: %+v", g)
		}
	}
}
