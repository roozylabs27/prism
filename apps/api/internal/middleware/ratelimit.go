package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	prismredis "github.com/roozylabs/prism/internal/redis"
)

// RateLimiter decides whether one more request fits a key's window.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int) (prismredis.Decision, error)
}

// GatewayRateLimitMiddleware limits requests per gateway key per minute. The
// key's own rate_limit applies, or defaultLimit when the key has none. When
// the limiter cannot be reached, fails open for dev/test backward compatibility.
func GatewayRateLimitMiddleware(limiter RateLimiter, defaultLimit int) gin.HandlerFunc {
	return GatewayRateLimitMiddlewareWithPolicy(limiter, defaultLimit, false)
}

// GatewayRateLimitMiddlewareWithPolicy limits requests per gateway key per
// minute. When failClosed is true, a Redis/limiter error rejects requests with
// 503 Service Unavailable instead of letting traffic bypass enforcement.
func GatewayRateLimitMiddlewareWithPolicy(limiter RateLimiter, defaultLimit int, failClosed bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		gatewayKey := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
		tc := GetTenantContext(c)

		limit := gatewayKey.RateLimit
		if limit <= 0 {
			limit = defaultLimit
		}
		if limit <= 0 {
			c.Next()
			return
		}
		if limiter == nil {
			if failClosed {
				httputil.RespondError(c, http.StatusServiceUnavailable, "Rate limit enforcement temporarily unavailable", nil, "RATE_LIMITER_UNAVAILABLE")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		key := fmt.Sprintf("tenant:%s:gateway:%s:rate_limit", tc.OrgID, gatewayKey.KeyHash)
		d, err := limiter.Allow(c.Request.Context(), key, limit)
		if err != nil {
			logRateLimitFailure(err, failClosed)
			if failClosed {
				httputil.RespondError(c, http.StatusServiceUnavailable, "Rate limit enforcement temporarily unavailable", err, "RATE_LIMITER_UNAVAILABLE")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		remaining := int64(limit) - d.Count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if !d.Allowed {
			c.Header("Retry-After", strconv.FormatInt(int64(d.RetryAfter/time.Second), 10))
			httputil.RespondError(c, http.StatusTooManyRequests, "Rate limit exceeded", nil, "RATE_LIMIT_EXCEEDED")
			return
		}

		c.Next()
	}
}

var (
	rateLimitLogMu   sync.Mutex
	rateLimitLogLast time.Time
)

// logRateLimitFailure reports a limiter failure at most once every 10
// seconds: during a Redis outage every gateway request would hit it.
func logRateLimitFailure(err error, failClosed bool) {
	rateLimitLogMu.Lock()
	defer rateLimitLogMu.Unlock()
	if time.Since(rateLimitLogLast) < 10*time.Second {
		return
	}
	rateLimitLogLast = time.Now()
	if failClosed {
		log.Printf("gateway rate limit: limiter unavailable, failing closed (503): %v", err)
		return
	}
	log.Printf("gateway rate limit: limiter unavailable, letting requests through: %v", err)
}
