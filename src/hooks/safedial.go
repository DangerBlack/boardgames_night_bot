package hooks

import (
	"boardgame-night-bot/src/utils"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
)

var ErrNonPublicAddress = errors.New("webhook target resolves to a non-public address")

// SafeDialer refuses connections to non-public IPs. The check runs on the
// resolved address of every connection, so it also covers host names that
// resolve to internal IPs, DNS rebinding and redirects — which a check on the
// URL text at registration time cannot.
func SafeDialer(allowLocal bool) *net.Dialer {
	return &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			if allowLocal {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || utils.IsNonPublicIP(ip) {
				return fmt.Errorf("%w: %s", ErrNonPublicAddress, host)
			}
			return nil
		},
	}
}

// NewSafeHTTPClient is the HTTP client used to call webhooks.
func NewSafeHTTPClient(timeout time.Duration, allowLocal bool) *http.Client {
	dialer := SafeDialer(allowLocal)
	transport := &http.Transport{
		// no proxy: a proxy would make the dialer see the proxy's address
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Webhook receivers have no reason to redirect; never follow.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func allowLocalWebhooks() bool {
	return os.Getenv("ALLOW_LOCAL_WEBHOOKS") == "true"
}
