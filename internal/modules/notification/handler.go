package notification

import (
	"strconv"

	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	service   Service
	validator *validator.Validator
}

func NewHandler(s Service, v *validator.Validator) *Handler {
	return &Handler{service: s, validator: v}
}

func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	limit, _ := strconv.Atoi(c.Query("limit", "30"))
	out, err := h.service.List(c.Context(), uid, limit)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Notifications retrieved", out)
}

func (h *Handler) MarkRead(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid notification id")
	}
	if err := h.service.MarkRead(c.Context(), uid, id); err != nil {
		return err
	}
	return httpx.OK(c, "Notification marked as read", nil)
}

func (h *Handler) MarkAllRead(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	if err := h.service.MarkAllRead(c.Context(), uid); err != nil {
		return err
	}
	return httpx.OK(c, "Notifications marked as read", nil)
}
