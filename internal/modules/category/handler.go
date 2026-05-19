package category

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
// @Summary  List my + system categories
// @Tags     Categories
// @Produce  json
// @Param    type query string false "Filter by type" Enums(income, expense)
// @Success  200 {object} dto.APIResponse{data=[]dto.CategoryResponse}
// @Security BearerAuth
// @Router   /api/v1/categories [get]
func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.List(c.Context(), uid, c.Query("type"))
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgGetCategories, out)
}

// Get godoc
// @Summary  Get category
// @Tags     Categories
// @Produce  json
// @Param    id path string true "Category UUID"
// @Success  200 {object} dto.APIResponse{data=dto.CategoryResponse}
// @Security BearerAuth
// @Router   /api/v1/categories/{id} [get]
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
	return httpx.OK(c, constants.MsgGetCategory, out)
}

// Create godoc
// @Summary  Create category
// @Tags     Categories
// @Accept   json
// @Produce  json
// @Param    request body dto.CreateCategoryRequest true "Category data"
// @Success  201 {object} dto.APIResponse{data=dto.CategoryResponse}
// @Security BearerAuth
// @Router   /api/v1/categories [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CreateCategoryRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, constants.MsgCreateCategory, out)
}

// Update godoc
// @Summary  Update category
// @Tags     Categories
// @Accept   json
// @Produce  json
// @Param    id      path string                    true "Category UUID"
// @Param    request body dto.UpdateCategoryRequest true "Category data"
// @Success  200 {object} dto.APIResponse{data=dto.CategoryResponse}
// @Security BearerAuth
// @Router   /api/v1/categories/{id} [put]
func (h *Handler) Update(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateCategoryRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Update(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateCategory, out)
}

// Delete godoc
// @Summary  Delete category
// @Tags     Categories
// @Produce  json
// @Param    id path string true "Category UUID"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/categories/{id} [delete]
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
	return httpx.OK(c, constants.MsgDeleteCategory, nil)
}
