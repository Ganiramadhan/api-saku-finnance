package subscription

import (
	"strconv"

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

// ListPlans godoc
// @Summary  List subscription plans (public)
// @Tags     Subscriptions
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=[]dto.PlanResponse}
// @Router   /api/v1/subscriptions/plans [get]
func (h *Handler) ListPlans(c *fiber.Ctx) error {
	out, err := h.service.ListPlans(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, "Successfully retrieved plans", out)
}

// Checkout godoc
// @Summary  Create a Snap checkout for a plan
// @Tags     Subscriptions
// @Accept   json
// @Produce  json
// @Param    request body dto.CheckoutRequest true "Plan code"
// @Success  201 {object} dto.APIResponse{data=dto.CheckoutResponse}
// @Security BearerAuth
// @Router   /api/v1/subscriptions/checkout [post]
func (h *Handler) Checkout(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CheckoutRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Checkout(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, "Checkout created", out)
}

// MySubscriptions godoc
// @Summary  List my subscriptions
// @Tags     Subscriptions
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=[]dto.SubscriptionResponse}
// @Security BearerAuth
// @Router   /api/v1/subscriptions/me [get]
func (h *Handler) MySubscriptions(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.MySubscriptions(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Successfully retrieved subscriptions", out)
}

// ActiveSubscription godoc
// @Summary  Get my currently active subscription (if any)
// @Tags     Subscriptions
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=dto.SubscriptionResponse}
// @Security BearerAuth
// @Router   /api/v1/subscriptions/me/active [get]
func (h *Handler) ActiveSubscription(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.ActiveSubscription(c.Context(), uid)
	if err != nil {
		// 404 → return null payload instead of error so the UI can show "no active sub".
		return httpx.OK(c, "No active subscription", nil)
	}
	return httpx.OK(c, "Active subscription", out)
}

// Webhook godoc
// @Summary  Midtrans transaction notification
// @Tags     Subscriptions
// @Accept   json
// @Produce  json
// @Param    request body dto.MidtransWebhook true "Midtrans payload"
// @Success  200 {object} dto.APIResponse
// @Router   /api/v1/subscriptions/webhook [post]
func (h *Handler) Webhook(c *fiber.Ctx) error {
	var p dto.MidtransWebhook
	if err := c.BodyParser(&p); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid webhook payload")
	}
	if err := h.service.HandleWebhook(c.Context(), p); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return httpx.OK(c, "ok", nil)
}

// ListAllAdmin godoc
// @Summary  Admin: list all subscriptions across users
// @Tags     Admin
// @Produce  json
// @Param    page  query int false "Page number (1-based)" default(1)
// @Param    limit query int false "Page size" default(50)
// @Success  200 {object} dto.APIResponse{data=[]dto.AdminSubscriptionResponse}
// @Security BearerAuth
// @Router   /api/v1/admin/subscriptions [get]
func (h *Handler) ListAllAdmin(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit
	out, err := h.service.ListAllAdmin(c.Context(), limit, offset)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Successfully retrieved subscriptions", out)
}
