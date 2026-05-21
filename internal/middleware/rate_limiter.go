package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type loginRateBucket struct {
	count    int
	resetAt  time.Time
	blocked  bool
	blockTil time.Time
}

var (
	loginRateMu      sync.Mutex
	loginRateBuckets = map[string]loginRateBucket{}
)

func LoginRateLimiter() fiber.Handler {
	const maxAttempts = 5
	window := time.Minute

	return func(c *fiber.Ctx) error {
		key := c.IP()
		now := time.Now()

		loginRateMu.Lock()
		bucket := loginRateBuckets[key]
		if bucket.resetAt.IsZero() || now.After(bucket.resetAt) {
			bucket = loginRateBucket{resetAt: now.Add(window)}
		}
		if bucket.blocked && now.Before(bucket.blockTil) {
			c.Set("Retry-After", fmt.Sprintf("%.0f", time.Until(bucket.blockTil).Seconds()))
			loginRateMu.Unlock()
			return fiber.NewError(http.StatusTooManyRequests, "Terlalu banyak percobaan login. Silakan coba lagi nanti.")
		}
		bucket.count++
		if bucket.count > maxAttempts {
			bucket.blocked = true
			bucket.blockTil = now.Add(window)
			bucket.resetAt = bucket.blockTil
			loginRateBuckets[key] = bucket
			c.Set("Retry-After", "60")
			loginRateMu.Unlock()
			return fiber.NewError(http.StatusTooManyRequests, "Terlalu banyak percobaan login. Silakan coba lagi nanti.")
		}
		loginRateBuckets[key] = bucket
		loginRateMu.Unlock()

		return c.Next()
	}
}
