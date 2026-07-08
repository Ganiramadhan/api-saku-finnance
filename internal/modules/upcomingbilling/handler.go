package upcomingbilling

import (
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
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
	out, err := h.service.List(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Successfully retrieved upcoming billings", out)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.UpcomingBillingRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, "Upcoming billing created", out)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateUpcomingBillingRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Update(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Upcoming billing updated", out)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Context(), uid, id); err != nil {
		return err
	}
	return httpx.OK(c, "Upcoming billing deleted", nil)
}
