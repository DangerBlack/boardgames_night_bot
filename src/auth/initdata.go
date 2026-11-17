package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TelegramUser is the subset of the Telegram WebApp user object we rely on.
type TelegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// DisplayName mirrors telegram.DefineUsername so names look the same whether
// a user acts through the bot or through the mini-app.
func (u TelegramUser) DisplayName() (string, bool) {
	if u.Username != "" {
		return u.Username, true
	}
	name := strings.TrimSpace(fmt.Sprintf("%s %s", u.FirstName, u.LastName))
	if name != "" {
		return name, false
	}
	return fmt.Sprintf("user_%d", u.ID), false
}

var (
	ErrInitDataMissingHash = errors.New("init data: missing hash")
	ErrInitDataBadHash     = errors.New("init data: invalid signature")
	ErrInitDataExpired     = errors.New("init data: expired")
	ErrInitDataNoUser      = errors.New("init data: missing user")
)

// ValidateInitData verifies Telegram Mini App init data as documented in
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
//
//	secret_key = HMAC_SHA256(key="WebAppData", msg=bot_token)
//	hash       = hex(HMAC_SHA256(key=secret_key, msg=data_check_string))
//
// where data_check_string is every field except "hash", as "key=value",
// sorted by key and joined by "\n". maxAge <= 0 disables the freshness check.
func ValidateInitData(initData, botToken string, maxAge time.Duration, now time.Time) (*TelegramUser, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("init data: %w", err)
	}

	hash := values.Get("hash")
	if hash == "" {
		return nil, ErrInitDataMissingHash
	}

	pairs := make([]string, 0, len(values))
	for k, v := range values {
		if k == "hash" {
			continue
		}
		pairs = append(pairs, k+"="+strings.Join(v, ""))
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	expected := hex.EncodeToString(hmacSHA256(secret, []byte(dataCheckString)))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(hash))) {
		return nil, ErrInitDataBadHash
	}

	if maxAge > 0 {
		authDate, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
		if err != nil || now.Sub(time.Unix(authDate, 0)) > maxAge {
			return nil, ErrInitDataExpired
		}
	}

	rawUser := values.Get("user")
	if rawUser == "" {
		return nil, ErrInitDataNoUser
	}
	var user TelegramUser
	if err := json.Unmarshal([]byte(rawUser), &user); err != nil || user.ID == 0 {
		return nil, ErrInitDataNoUser
	}

	return &user, nil
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
