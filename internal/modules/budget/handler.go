package budget

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

// List godoc
// @Summary  List my budgets
// @Tags     Budgets
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=[]dto.BudgetResponse}
// @Security BearerAuth
// @Router   /api/v1/budgets [get]
func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.List(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgGetBudgets, out)
}

// Get godoc
// @Summary  Get budget
// @Tags     Budgets
// @Produce  json
// @Param    id path string true "Budget UUID"
// @Success  200 {object} dto.APIResponse{data=dto.BudgetResponse}
// @Security BearerAuth
// @Router   /api/v1/budgets/{id} [get]
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
	return httpx.OK(c, constants.MsgGetBudget, out)
}

// Create godoc
// @Summary  Create budget
// @Tags     Budgets
// @Accept   json
// @Produce  json
// @Param    request body dto.CreateBudgetRequest true "Budget data"
// @Success  201 {object} dto.APIResponse{data=dto.BudgetResponse}
// @Security BearerAuth
// @Router   /api/v1/budgets [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CreateBudgetRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, constants.MsgCreateBudget, out)
}

// Update godoc
// @Summary  Update budget
// @Tags     Budgets
// @Accept   json
// @Produce  json
// @Param    id      path string                  true "Budget UUID"
// @Param    request body dto.UpdateBudgetRequest true "Budget data"
// @Success  200 {object} dto.APIResponse{data=dto.BudgetResponse}
// @Security BearerAuth
// @Router   /api/v1/budgets/{id} [put]
func (h *Handler) Update(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateBudgetRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Update(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateBudget, out)
}

// Delete godoc
// @Summary  Delete budget
// @Tags     Budgets
// @Produce  json
// @Param    id path string true "Budget UUID"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/budgets/{id} [delete]
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
	return httpx.OK(c, constants.MsgDeleteBudget, nil)
}
