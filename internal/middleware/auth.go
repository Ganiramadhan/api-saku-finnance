package middleware

import (
	"net/http"
	"strings"

	"github.com/ganiramadhan/starter-go/internal/constants"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/gofiber/fiber/v2"
)

func AuthRequired(j *jwt.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.TrimSpace(c.Get("Authorization"))
		cookieToken := strings.TrimSpace(c.Cookies("saku_session"))
		if header == "" {
			if cookieToken == "" {
				return fiber.NewError(http.StatusUnauthorized, constants.ErrUnauthorized)
			}
		} else if !strings.HasPrefix(header, "Bearer ") {
			return fiber.NewError(http.StatusUnauthorized, constants.ErrInvalidToken)
		}
		token := cookieToken
		if header != "" {
			token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		}
		if token == "" {
			return fiber.NewError(http.StatusUnauthorized, constants.ErrInvalidToken)
		}
		claims, err := j.Validate(token)
		if err != nil {
			return fiber.NewError(http.StatusUnauthorized, constants.ErrInvalidToken)
		}
		c.Locals(httpx.LocalUserID, claims.UserID)
		c.Locals(httpx.LocalEmail, claims.Email)
		c.Locals(httpx.LocalRole, claims.Role)
		return c.Next()
	}
}

func RequireAdmin(c *fiber.Ctx) error {
	role := httpx.Role(c)
	if role != "admin" && role != "super_admin" {
		return fiber.NewError(http.StatusForbidden, constants.ErrForbidden)
	}
	return c.Next()
}

func RequireSuperAdmin(c *fiber.Ctx) error {
	if httpx.Role(c) != "super_admin" {
		return fiber.NewError(http.StatusForbidden, constants.ErrForbidden)
	}
	return c.Next()
}
