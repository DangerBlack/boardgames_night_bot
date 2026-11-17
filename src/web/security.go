package web

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MaxBodyBytes caps request bodies; every form and JSON payload is tiny.
const MaxBodyBytes = 1 << 20

// SecurityHeaders hardens every response.
//   - frame-ancestors: the mini-app is legitimately framed by Telegram Web, so
//     X-Frame-Options: DENY is not an option; everyone else is refused
//     (protects the owner settings toggle and account actions from clickjacking).
//   - Referrer-Policy: no-referrer keeps one-time login tokens in URLs from
//     leaking to third parties (telegram.org script, BGG images and links).
func SecurityHeaders() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		h := ctx.Writer.Header()
		h.Set("Content-Security-Policy", "frame-ancestors 'self' https://web.telegram.org https://*.telegram.org")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		ctx.Next()
	}
}

// LimitBody rejects oversized request bodies.
func LimitBody(n int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.ContentLength > n {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request too large"})
			return
		}
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, n)
		ctx.Next()
	}
}

// DefaultTrustedProxies covers a reverse proxy on the same host or Docker
// network. Only these may set X-Forwarded-For; anyone else gets their socket
// address as client IP, so per-IP rate limits cannot be dodged by spoofing.
var DefaultTrustedProxies = []string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}

// ParseTrustedProxies reads a comma-separated TRUSTED_PROXIES value.
func ParseTrustedProxies(env string) []string {
	if strings.TrimSpace(env) == "" {
		return DefaultTrustedProxies
	}
	var out []string
	for _, p := range strings.Split(env, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	log.Default().Printf("trusted proxies: %v", out)
	return out
}
