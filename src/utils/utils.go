package utils

import (
	"boardgame-night-bot/src/models"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"
)

func GenerateSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func IsValidURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != "" && (strings.HasPrefix(u.Scheme, "http"))
}

func IntToPointer(i int) *int64 {
	v := int64(i)
	return &v
}

func PickGame(event *models.Event, gameID int64) *models.BoardGame {
	for _, game := range event.BoardGames {
		if game.ID == gameID {
			return &game
		}
	}
	return nil
}

func PickGameUUID(event *models.Event, gameID string) *models.BoardGame {
	for _, game := range event.BoardGames {
		if game.UUID == gameID {
			return &game
		}
	}
	return nil
}
