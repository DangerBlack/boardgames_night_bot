package utils

import (
	"boardgame-night-bot/src/models"
	"crypto/rand"
	"encoding/hex"
	"net"
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

func IsLocalURL(rawUrl string) bool {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return false
	}

	host := u.Hostname()

	// Check localhost and loopback
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Host is a domain, not IP
		return false
	}

	// Check private IPv4 ranges
	privateIPv4 := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateIPv4 {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	// Loopback IPv6
	if ip.IsLoopback() {
		return true
	}

	return false
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
