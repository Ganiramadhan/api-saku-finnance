package telegram

import (
	"strings"

	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service   Service
	validator *validator.Validator
	secret    string
}

func NewHandler(service Service, validator *validator.Validator, secret string) *Handler {
	return &Handler{service: service, validator: validator, secret: secret}
}

func (h *Handler) Webhook(c *fiber.Ctx) error {
	if err := h.validateSecret(c); err != nil {
		return err
	}
	var update Update
	if err := c.BodyParser(&update); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid telegram update")
	}
	if err := h.service.HandleUpdate(c.Context(), update); err != nil {
		return err
	}
	return httpx.OK(c, "OK", map[string]bool{"ok": true})
}

func (h *Handler) validateSecret(c *fiber.Ctx) error {
	if strings.TrimSpace(h.secret) == "" {
		return fiber.NewError(fiber.StatusNotFound, "telegram webhook is not configured")
	}
	pathSecret := strings.TrimSpace(c.Params("secret"))
	headerSecret := strings.TrimSpace(c.Get("X-Telegram-Bot-Api-Secret-Token"))
	if pathSecret == h.secret || headerSecret == h.secret {
		return nil
	}
	return fiber.NewError(fiber.StatusUnauthorized, "invalid telegram webhook secret")
}
