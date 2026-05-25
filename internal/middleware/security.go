package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Set("Cross-Origin-Resource-Policy", "same-site")
		if c.Method() != fiber.MethodGet || isSensitivePath(c.Path()) {
			c.Set("Cache-Control", "no-store")
			c.Set("Pragma", "no-cache")
		}
		return c.Next()
	}
}

func isSensitivePath(path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/v1/auth"):
		return true
	case strings.HasPrefix(path, "/api/v1/subscriptions"):
		return true
	default:
		return false
	}
}
