package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersAndBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders(), LimitBody(16))
	r.POST("/x", func(c *gin.Context) {
		if _, err := c.GetRawData(); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/x", strings.NewReader("small")))
	if w.Code != http.StatusOK {
		t.Fatalf("small body: %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'self' https://web.telegram.org") {
		t.Fatalf("CSP = %q", w.Header().Get("Content-Security-Policy"))
	}
	if w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing security headers")
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/x", strings.NewReader(strings.Repeat("a", 100))))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d", w.Code)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	if got := ParseTrustedProxies(""); len(got) != len(DefaultTrustedProxies) {
		t.Fatalf("default = %v", got)
	}
	if got := ParseTrustedProxies(" 1.2.3.4 , 10.0.0.0/8,"); len(got) != 2 || got[0] != "1.2.3.4" {
		t.Fatalf("parsed = %v", got)
	}
}
