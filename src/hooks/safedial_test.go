package hooks

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"boardgame-night-bot/src/utils"
)

func TestSafeClientRefusesNonPublicTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_, err := NewSafeHTTPClient(2*time.Second, false).Get(srv.URL) // 127.0.0.1
	if !errors.Is(err, ErrNonPublicAddress) {
		t.Fatalf("loopback webhook allowed: %v", err)
	}

	resp, err := NewSafeHTTPClient(2*time.Second, true).Get(srv.URL)
	if err != nil {
		t.Fatalf("ALLOW_LOCAL_WEBHOOKS should allow loopback: %v", err)
	}
	resp.Body.Close()
}

func TestIsNonPublicIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.20.0.1", "192.168.1.1", "169.254.169.254",
		"0.0.0.0", "100.64.1.1", "::1", "fe80::1", "fd00::1", "::ffff:127.0.0.1", "::ffff:169.254.169.254", "224.0.0.1"} {
		if !utils.IsNonPublicIP(net.ParseIP(s)) {
			t.Errorf("%s should be non-public", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if utils.IsNonPublicIP(net.ParseIP(s)) {
			t.Errorf("%s should be public", s)
		}
	}
	if !utils.IsLocalURL("http://169.254.169.254/latest/meta-data") || !utils.IsLocalURL("http://api.localhost/x") {
		t.Error("IsLocalURL missed an internal target")
	}
}
