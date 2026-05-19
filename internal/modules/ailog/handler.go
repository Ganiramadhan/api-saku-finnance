package ailog

import (
	"github.com/ganiramadhan/starter-go/internal/constants"
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

// List godoc
// @Summary  List my AI processing logs
// @Tags     AI Logs
// @Produce  json
// @Param    feature query string false "Filter by feature (scan_receipt|chat|categorize|insights|suggest_budget)"
// @Param    page    query int    false "Page (default 1)"
// @Param    limit   query int    false "Page size (default 20)"
// @Success  200 {object} dto.APIResponse{data=[]dto.AIProcessingLogResponse}
// @Security BearerAuth
// @Router   /api/v1/ai-logs [get]
func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	feature := c.Query("feature", "")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	out, meta, err := h.service.List(c.Context(), uid, feature, page, limit)
	if err != nil {
		return err
	}
	return httpx.List(c, constants.MsgGetAILogs, out, meta)
}

// ListAll godoc
// @Summary  List all AI processing logs (super admin only)
// @Tags     AI Logs
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=[]dto.AIProcessingLogResponse}
// @Security BearerAuth
// @Router   /api/v1/admin/ai-logs [get]
func (h *Handler) ListAll(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	out, meta, err := h.service.ListAll(c.Context(), page, limit)
	if err != nil {
		return err
	}
	return httpx.List(c, constants.MsgGetAILogs, out, meta)
}
