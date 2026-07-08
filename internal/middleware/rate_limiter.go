package middleware

import (
	"fmt"
	"net/http"
	"strings"
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
	loginRatePruned  time.Time
)

func SensitiveRateLimiter(maxAttempts int, window, blockFor time.Duration, message string) fiber.Handler {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if window <= 0 {
		window = time.Minute
	}
	if blockFor <= 0 {
		blockFor = window
	}
	if strings.TrimSpace(message) == "" {
		message = "Too many requests. Please try again later."
	}

	return func(c *fiber.Ctx) error {
		key := c.IP() + ":" + c.Method() + ":" + c.Path()
		now := time.Now()

		loginRateMu.Lock()
		pruneRateBuckets(now)
		bucket := loginRateBuckets[key]
		if bucket.resetAt.IsZero() || now.After(bucket.resetAt) {
			bucket = loginRateBucket{resetAt: now.Add(window)}
		}
		if bucket.blocked && now.Before(bucket.blockTil) {
			c.Set("Retry-After", fmt.Sprintf("%.0f", time.Until(bucket.blockTil).Seconds()))
			loginRateMu.Unlock()
			return fiber.NewError(http.StatusTooManyRequests, message)
		}
		bucket.count++
		if bucket.count > maxAttempts {
			bucket.blocked = true
			bucket.blockTil = now.Add(blockFor)
			bucket.resetAt = bucket.blockTil
			loginRateBuckets[key] = bucket
			c.Set("Retry-After", fmt.Sprintf("%.0f", blockFor.Seconds()))
			loginRateMu.Unlock()
			return fiber.NewError(http.StatusTooManyRequests, message)
		}
		loginRateBuckets[key] = bucket
		loginRateMu.Unlock()

		return c.Next()
	}
}

func pruneRateBuckets(now time.Time) {
	if !loginRatePruned.IsZero() && now.Sub(loginRatePruned) < time.Minute {
		return
	}
	for key, bucket := range loginRateBuckets {
		if bucket.resetAt.IsZero() || now.After(bucket.resetAt.Add(time.Minute)) {
			delete(loginRateBuckets, key)
		}
	}
	loginRatePruned = now
}

func LoginRateLimiter() fiber.Handler {
	return SensitiveRateLimiter(
		5,
		time.Minute,
		5*time.Minute,
		"Too many login attempts. Please try again later.",
	)
}
