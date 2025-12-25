package limiter

import (
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Limiter uses x/time/rate for per-webhook_id rate limiting
// Allows up to 'limit' requests per second per webhook_id

type Limiter struct {
	limit    rate.Limit
	burst    int
	limiters map[string]*rate.Limiter
	mu       sync.Mutex
}

func NewLimiter(rps int, burst int) *Limiter {
	return &Limiter{
		limit:    rate.Limit(rps),
		burst:    burst,
		limiters: make(map[string]*rate.Limiter),
	}
}

func (l *Limiter) getLimiter(webhookID string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	limiter, exists := l.limiters[webhookID]
	if !exists {
		limiter = rate.NewLimiter(l.limit, l.burst)
		l.limiters[webhookID] = limiter
	}
	return limiter
}

// GinHandler returns a Gin middleware that limits requests per webhook_id path param
func (l *Limiter) GinHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		webhookID := c.Param("webhook_id")
		if webhookID == "" {
			c.AbortWithStatusJSON(400, gin.H{"error": "webhook_id path param required"})
			return
		}
		limiter := l.getLimiter(webhookID)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded for webhook_id"})
			return
		}
		c.Next()
	}
}
