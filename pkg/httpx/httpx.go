package httpx

import (
	"net/http"

	"github.com/ganiramadhan/starter-go/internal/constants"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	LocalUserID = "userID"
	LocalEmail  = "email"
	LocalRole   = "role"
)

func UserID(c *fiber.Ctx) (uuid.UUID, error) {
	uid, ok := c.Locals(LocalUserID).(uuid.UUID)
	if !ok {
		return uuid.Nil, fiber.NewError(http.StatusUnauthorized, constants.ErrUnauthorized)
	}
	return uid, nil
}

func Role(c *fiber.Ctx) string {
	r, _ := c.Locals(LocalRole).(string)
	return r
}

func Bind(c *fiber.Ctx, v *validator.Validator, dst any) error {
	if err := c.BodyParser(dst); err != nil {
		return fiber.NewError(http.StatusBadRequest, constants.ErrInvalidRequest)
	}
	if v == nil {
		return nil
	}
	if err := v.Struct(dst); err != nil {
		if ve, ok := validator.AsValidation(err); ok {
			fe := fiber.NewError(http.StatusUnprocessableEntity, ve.Error())
			c.Locals(localValidationKey, ve)
			return fe
		}
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func ParseUUID(c *fiber.Ctx, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(param))
	if err != nil {
		return uuid.Nil, fiber.NewError(http.StatusBadRequest, constants.ErrInvalidUUID)
	}
	return id, nil
}

func OK(c *fiber.Ctx, msg string, data any) error {
	return write(c, http.StatusOK, "success", msg, data, nil)
}

func Created(c *fiber.Ctx, msg string, data any) error {
	return write(c, http.StatusCreated, "success", msg, data, nil)
}

func List(c *fiber.Ctx, msg string, data any, meta *dto.PaginationMeta) error {
	return write(c, http.StatusOK, "success", msg, data, meta)
}

func write(c *fiber.Ctx, code int, status, msg string, data any, meta *dto.PaginationMeta) error {
	return c.Status(code).JSON(dto.APIResponse{
		Status:  status,
		Code:    code,
		Message: msg,
		Data:    data,
		Meta:    meta,
	})
}

const localValidationKey = "_httpx_validation"

func ValidationFromCtx(c *fiber.Ctx) *validator.ValidationError {
	if v, ok := c.Locals(localValidationKey).(*validator.ValidationError); ok {
		return v
	}
	return nil
}
