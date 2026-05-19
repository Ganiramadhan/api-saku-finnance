package savingsgoal

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

// List godoc
// @Summary  List my savings goals
// @Tags     SavingsGoals
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=[]dto.SavingsGoalResponse}
// @Security BearerAuth
// @Router   /api/v1/savings-goals [get]
func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.List(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, "savings goals retrieved", out)
}

// Get godoc
// @Summary  Get savings goal
// @Tags     SavingsGoals
// @Produce  json
// @Param    id path string true "Savings Goal UUID"
// @Success  200 {object} dto.APIResponse{data=dto.SavingsGoalResponse}
// @Security BearerAuth
// @Router   /api/v1/savings-goals/{id} [get]
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
	return httpx.OK(c, "savings goal retrieved", out)
}

// Create godoc
// @Summary  Create savings goal
// @Tags     SavingsGoals
// @Accept   json
// @Produce  json
// @Param    request body dto.CreateSavingsGoalRequest true "Goal data"
// @Success  201 {object} dto.APIResponse{data=dto.SavingsGoalResponse}
// @Security BearerAuth
// @Router   /api/v1/savings-goals [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CreateSavingsGoalRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, "savings goal created", out)
}

// Update godoc
// @Summary  Update savings goal
// @Tags     SavingsGoals
// @Accept   json
// @Produce  json
// @Param    id      path string                          true "Goal UUID"
// @Param    request body dto.UpdateSavingsGoalRequest    true "Goal data"
// @Success  200 {object} dto.APIResponse{data=dto.SavingsGoalResponse}
// @Security BearerAuth
// @Router   /api/v1/savings-goals/{id} [put]
func (h *Handler) Update(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateSavingsGoalRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Update(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "savings goal updated", out)
}

// Delete godoc
// @Summary  Delete savings goal
// @Tags     SavingsGoals
// @Produce  json
// @Param    id path string true "Goal UUID"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/savings-goals/{id} [delete]
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
	return httpx.OK(c, "savings goal deleted", nil)
}

// Contribute godoc
// @Summary  Add a contribution to a savings goal
// @Tags     SavingsGoals
// @Accept   json
// @Produce  json
// @Param    id      path string                              true "Goal UUID"
// @Param    request body dto.ContributeSavingsGoalRequest    true "Contribution data"
// @Success  200 {object} dto.APIResponse{data=dto.SavingsGoalResponse}
// @Security BearerAuth
// @Router   /api/v1/savings-goals/{id}/contribute [post]
func (h *Handler) Contribute(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.ContributeSavingsGoalRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Contribute(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "contribution recorded", out)
}

// ListContributions godoc
// @Summary  List contributions of a savings goal
// @Tags     SavingsGoals
// @Produce  json
// @Param    id path string true "Goal UUID"
// @Success  200 {object} dto.APIResponse{data=[]dto.SavingsGoalContributionResponse}
// @Security BearerAuth
// @Router   /api/v1/savings-goals/{id}/contributions [get]
func (h *Handler) ListContributions(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.ListContributions(c.Context(), uid, id)
	if err != nil {
		return err
	}
	return httpx.OK(c, "contributions retrieved", out)
}
