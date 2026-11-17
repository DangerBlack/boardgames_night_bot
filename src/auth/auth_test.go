package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-TOKEN"

// SignInitData builds init data exactly like Telegram does, for tests.
func SignInitData(botToken string, fields map[string]string) string {
	pairs := make([]string, 0, len(fields))
	for k, v := range fields {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(strings.Join(pairs, "\n")))

	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("hash", hex.EncodeToString(m.Sum(nil)))
	return q.Encode()
}

func fields(now time.Time) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"query_id":  "AAH",
		"user":      `{"id":4242,"first_name":"Ann","last_name":"Lee","username":"ann"}`,
	}
}

func TestValidateInitData(t *testing.T) {
	now := time.Now()
	data := SignInitData(testBotToken, fields(now))

	u, err := ValidateInitData(data, testBotToken, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 4242 || u.Username != "ann" {
		t.Fatalf("user = %+v", u)
	}

	if _, err = ValidateInitData(data, "other:token", time.Hour, now); !errors.Is(err, ErrInitDataBadHash) {
		t.Fatalf("wrong bot token accepted: %v", err)
	}

	// Forge another user id while keeping the original hash.
	forged := strings.Replace(data, "4242", "1", 1)
	if _, err = ValidateInitData(forged, testBotToken, time.Hour, now); !errors.Is(err, ErrInitDataBadHash) {
		t.Fatalf("forged user accepted: %v", err)
	}

	if _, err = ValidateInitData(data, testBotToken, time.Hour, now.Add(2*time.Hour)); !errors.Is(err, ErrInitDataExpired) {
		t.Fatalf("stale init data accepted: %v", err)
	}

	if _, err = ValidateInitData("user=x", testBotToken, time.Hour, now); !errors.Is(err, ErrInitDataMissingHash) {
		t.Fatalf("missing hash accepted: %v", err)
	}
}

func TestNormalizeEmail(t *testing.T) {
	for in, want := range map[string]string{
		" Bob@Example.COM ": "bob@example.com",
	} {
		got, err := NormalizeEmail(in)
		if err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "bob", "bob@localhost", "Bob <bob@example.com>", "a@b.c\nBcc: x@y.z"} {
		if _, err := NormalizeEmail(bad); err == nil {
			t.Errorf("NormalizeEmail(%q) accepted", bad)
		}
	}
}

func TestNormalizeUsername(t *testing.T) {
	if got, err := NormalizeUsername("  Mario   Rossi "); err != nil || got != "Mario Rossi" {
		t.Fatalf("got %q %v", got, err)
	}
	for _, bad := range []string{"ab", "@mario", "<script>", strings.Repeat("a", 33)} {
		if _, err := NormalizeUsername(bad); err == nil {
			t.Errorf("NormalizeUsername(%q) accepted", bad)
		}
	}
}

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"":                 "/account",
		"/events/x":        "/events/x",
		"//evil.com":       "/account",
		"https://evil.com": "/account",
		"/\\evil.com":      "/account",
	}
	for in, want := range cases {
		if got := SafeNext(in); got != want {
			t.Errorf("SafeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
