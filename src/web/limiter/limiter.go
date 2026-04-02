package limiter

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Limiter uses x/time/rate for per-webhook_id rate limiting
// Allows up to 'limit' requests per second per webhook_id

const (
	// defaultTTL is how long an idle limiter entry is kept before eviction.
	defaultTTL = 10 * time.Minute
	// sweepInterval is how often the background sweeper runs.
	sweepInterval = 5 * time.Minute
)

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type Limiter struct {
	limit    rate.Limit
	burst    int
	limiters map[string]*entry
	mu       sync.Mutex
	ttl      time.Duration
}

func NewLimiter(rps int, burst int) *Limiter {
	l := &Limiter{
		limit:    rate.Limit(rps),
		burst:    burst,
		limiters: make(map[string]*entry),
		ttl:      defaultTTL,
	}
	go l.sweep()
	return l
}

// sweep periodically removes entries that have not been accessed within the TTL,
// preventing the map from growing unboundedly.
func (l *Limiter) sweep() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for id, e := range l.limiters {
			if time.Since(e.lastSeen) > l.ttl {
				delete(l.limiters, id)
			}
		}
		l.mu.Unlock()
	}
}

func (l *Limiter) getLimiter(webhookID string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, exists := l.limiters[webhookID]
	if !exists {
		e = &entry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.limiters[webhookID] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
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
