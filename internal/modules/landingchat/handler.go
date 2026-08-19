package landingchat

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

// Ask godoc
// @Summary  Public landing-page support chat
// @Tags     Public
// @Accept   json
// @Produce  json
// @Param    request body dto.LandingChatRequest true "Visitor message"
// @Success  200 {object} dto.APIResponse{data=dto.LandingChatResponse}
// @Router   /api/v1/landing-chat [post]
func (h *Handler) Ask(c *fiber.Ctx) error {
	var req dto.LandingChatRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Ask(c.Context(), req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "ok", out)
}
