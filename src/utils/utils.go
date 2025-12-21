package utils

import (
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
