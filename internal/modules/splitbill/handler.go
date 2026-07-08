package splitbill

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/subscription"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service      Service
	validator    *validator.Validator
	subscription subscription.Service
}

func NewHandler(s Service, v *validator.Validator, sub subscription.Service) *Handler {
	return &Handler{service: s, validator: v, subscription: sub}
}

// List godoc
// @Summary  List my split bills
// @Tags     SplitBills
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=[]dto.SplitBillResponse}
// @Security BearerAuth
// @Router   /api/v1/split-bills [get]
func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.List(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Successfully retrieved split bills", out)
}

// Get godoc
// @Summary  Get split bill detail
// @Tags     SplitBills
// @Produce  json
// @Param    id path string true "SplitBill UUID"
// @Success  200 {object} dto.APIResponse{data=dto.SplitBillResponse}
// @Security BearerAuth
// @Router   /api/v1/split-bills/{id} [get]
func (h *Handler) Get(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.Get(c.Context(), uid, id)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Split bill detail", out)
}

// Create godoc
// @Summary  Create split bill
// @Tags     SplitBills
// @Accept   json
// @Produce  json
// @Param    request body dto.CreateSplitBillRequest true "Split bill"
// @Success  201 {object} dto.APIResponse{data=dto.SplitBillResponse}
// @Security BearerAuth
// @Router   /api/v1/split-bills [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}

	// Check pro subscription
	hasPro, err := h.subscription.HasActiveProSubscription(c.Context(), uid)
	if err != nil {
		return err
	}
	if !hasPro {
		return domain.ErrProSubscriptionRequired
	}

	var req dto.CreateSplitBillRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, "Split bill created", out)
}

// Update godoc
// @Summary  Update split bill
// @Tags     SplitBills
// @Accept   json
// @Produce  json
// @Param    id      path string                     true "SplitBill UUID"
// @Param    request body dto.UpdateSplitBillRequest true "Split bill"
// @Success  200 {object} dto.APIResponse{data=dto.SplitBillResponse}
// @Security BearerAuth
// @Router   /api/v1/split-bills/{id} [put]
func (h *Handler) Update(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}

	// Check pro subscription
	hasPro, err := h.subscription.HasActiveProSubscription(c.Context(), uid)
	if err != nil {
		return err
	}
	if !hasPro {
		return domain.ErrProSubscriptionRequired
	}

	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateSplitBillRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Update(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Split bill updated", out)
}

// Delete godoc
// @Summary  Delete split bill
// @Tags     SplitBills
// @Produce  json
// @Param    id path string true "SplitBill UUID"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/split-bills/{id} [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}

	// Check pro subscription
	hasPro, err := h.subscription.HasActiveProSubscription(c.Context(), uid)
	if err != nil {
		return err
	}
	if !hasPro {
		return domain.ErrProSubscriptionRequired
	}

	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Context(), uid, id); err != nil {
		return err
	}
	return httpx.OK(c, "Split bill deleted", nil)
}

// MarkParticipantPaid godoc
// @Summary  Toggle a participant's paid status
// @Tags     SplitBills
// @Accept   json
// @Produce  json
// @Param    id   path string true "SplitBill UUID"
// @Param    pid  path string true "Participant UUID"
// @Param    body body object{paid=bool} true "paid flag"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/split-bills/{id}/participants/{pid}/paid [patch]
func (h *Handler) MarkParticipantPaid(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}

	// Check pro subscription
	hasPro, err := h.subscription.HasActiveProSubscription(c.Context(), uid)
	if err != nil {
		return err
	}
	if !hasPro {
		return domain.ErrProSubscriptionRequired
	}

	billID, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	partID, err := httpx.ParseUUID(c, "pid")
	if err != nil {
		return err
	}
	var body struct {
		Paid bool `json:"paid"`
	}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid payload")
	}
	if err := h.service.MarkParticipantPaid(c.Context(), uid, billID, partID, body.Paid); err != nil {
		return err
	}
	return httpx.OK(c, "Participant updated", nil)
}

// Share godoc
// @Summary  Generate WhatsApp share text + URL
// @Tags     SplitBills
// @Produce  json
// @Param    id    path  string true  "SplitBill UUID"
// @Param    phone query string false "Optional WhatsApp number (Indonesian local or international)"
// @Success  200 {object} dto.APIResponse{data=dto.SplitBillShareResponse}
// @Security BearerAuth
// @Router   /api/v1/split-bills/{id}/share [get]
func (h *Handler) Share(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}

	// Check pro subscription
	hasPro, err := h.subscription.HasActiveProSubscription(c.Context(), uid)
	if err != nil {
		return err
	}
	if !hasPro {
		return domain.ErrProSubscriptionRequired
	}

	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.BuildShare(c.Context(), uid, id, c.Query("phone"))
	if err != nil {
		return err
	}
	return httpx.OK(c, "Share payload", out)
}
