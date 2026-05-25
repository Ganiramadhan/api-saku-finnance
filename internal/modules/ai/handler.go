package ai

import (
	"github.com/ganiramadhan/starter-go/internal/constants"
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

// Categorize godoc
// @Summary  AI: categorize transaction from raw OCR text
// @Description Uses Claude Sonnet 4 to extract amount, merchant, category & type from free-form text.
// @Tags     AI
// @Accept   json
// @Produce  json
// @Param    request body dto.CategorizeRequest true "OCR text + optional user categories"
// @Success  200 {object} dto.APIResponse{data=dto.CategorizeResponse}
// @Failure  400 {object} dto.APIResponse
// @Failure  401 {object} dto.APIResponse
// @Failure  500 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/ai/categorize [post]
func (h *Handler) Categorize(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CategorizeRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Categorize(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgAICategorize, out)
}

// ScanReceipt godoc
// @Summary  AI: scan receipt image (vision)
// @Description Sends a base64-encoded receipt image to Claude vision and extracts structured fields.
// @Tags     AI
// @Accept   json
// @Produce  json
// @Param    request body dto.ScanReceiptRequest true "Base64 image (jpeg/png/webp)"
// @Success  200 {object} dto.APIResponse{data=dto.ScanReceiptResponse}
// @Failure  400 {object} dto.APIResponse
// @Failure  401 {object} dto.APIResponse
// @Failure  500 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/ai/scan-receipt [post]
func (h *Handler) ScanReceipt(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.ScanReceiptRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.ScanReceipt(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgAIScanReceipt, out)
}

func (h *Handler) PromoteScanImage(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.PromoteScanImageRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.PromoteScanImage(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Scan image promoted", out)
}

// Insights godoc
// @Summary  AI: financial insights for the current user
// @Description Aggregates the user's transactions over a date range and asks Claude for a narrative summary, recommendations and anomalies.
// @Tags     AI
// @Accept   json
// @Produce  json
// @Param    request body dto.InsightsRequest true "Date range (defaults to last 30 days)"
// @Success  200 {object} dto.APIResponse{data=dto.InsightsResponse}
// @Failure  401 {object} dto.APIResponse
// @Failure  500 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/ai/insights [post]
func (h *Handler) Insights(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.InsightsRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Insights(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgAIInsights, out)
}

// SuggestBudget godoc
// @Summary  AI: recommend a monthly budget per category
// @Description Uses recent expense history to suggest sensible monthly budget caps for each category.
// @Tags     AI
// @Accept   json
// @Produce  json
// @Param    request body dto.SuggestBudgetRequest true "Window in months (default 3) + optional wallet filter"
// @Success  200 {object} dto.APIResponse{data=dto.SuggestBudgetResponse}
// @Failure  401 {object} dto.APIResponse
// @Failure  500 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/ai/suggest-budget [post]
func (h *Handler) SuggestBudget(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.SuggestBudgetRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.SuggestBudget(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgAISuggestBudget, out)
}

// Chat godoc
// @Summary  AI: conversational finance assistant
// @Description Free-form Q&A with optional 30-day transaction context attached.
// @Tags     AI
// @Accept   json
// @Produce  json
// @Param    request body dto.ChatRequest true "User question"
// @Success  200 {object} dto.APIResponse{data=dto.ChatResponse}
// @Failure  400 {object} dto.APIResponse
// @Failure  401 {object} dto.APIResponse
// @Failure  500 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/ai/chat [post]
func (h *Handler) Chat(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.ChatRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Chat(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgAIChat, out)
}
