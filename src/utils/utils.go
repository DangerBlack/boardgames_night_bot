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

// IsLocalURL reports whether a URL points at a non-public address: a literal
// internal IP, "localhost", or a host name that resolves to one. It is a fast
// check at registration time; the webhook client enforces the same rule on
// every connection (see hooks.SafeDialer), which also covers DNS changes and
// redirects.
func IsLocalURL(rawUrl string) bool {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return false
	}

	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}

	if ip := net.ParseIP(host); ip != nil {
		return IsNonPublicIP(ip)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		// unresolvable now; the dialer re-checks when the webhook is called
		return false
	}
	for _, ip := range ips {
		if IsNonPublicIP(ip) {
			return true
		}
	}
	return false
}

var extraNonPublic = func() []*net.IPNet {
	var out []*net.IPNet
	for _, cidr := range []string{
		"0.0.0.0/8",     // "this network"
		"100.64.0.0/10", // carrier-grade NAT
		"192.0.0.0/24",  // IETF protocol assignments
		"198.18.0.0/15", // benchmarking
		"240.0.0.0/4",   // reserved
		"64:ff9b::/96",  // NAT64 (may embed internal IPv4)
	} {
		_, n, _ := net.ParseCIDR(cidr)
		out = append(out, n)
	}
	return out
}()

// IsNonPublicIP reports whether ip is loopback, private, link-local (which
// includes the 169.254.169.254 cloud metadata endpoint), unspecified,
// multicast or otherwise reserved.
func IsNonPublicIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, n := range extraNonPublic {
		if n.Contains(ip) {
			return true
		}
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
