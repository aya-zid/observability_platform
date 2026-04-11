package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// RateLimitByIP applies a simple fixed-window rate limit per client IP using Redis.
// Fail-open on Redis errors to avoid taking down auth when Redis is degraded.
func RateLimitByIP(redisClient *redis.Client, action string, limit int, window time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if redisClient == nil || limit <= 0 || window <= 0 {
				return next(c)
			}

			ip := c.RealIP()
			if ip == "" {
				ip = "unknown"
			}

			bucket := time.Now().Unix() / int64(window.Seconds())
			key := fmt.Sprintf("rl:%s:%s:%d", action, ip, bucket)
			ctx := c.Request().Context()

			n, err := redisClient.Incr(ctx, key).Result()
			if err != nil {
				c.Logger().Warnf("[ratelimit] redis incr failed (allowing request): %v", err)
				return next(c)
			}
			if n == 1 {
				_ = redisClient.Expire(ctx, key, window+5*time.Second).Err()
			}

			if n > int64(limit) {
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": "Too many requests",
					"code":  "RATE_LIMITED",
				})
			}

			return next(c)
		}
	}
}

