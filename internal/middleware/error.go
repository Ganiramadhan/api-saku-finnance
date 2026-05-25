package middleware

import (
	"errors"
	"strings"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal server error"
	var data any

	var fe *fiber.Error
	switch {
	case errors.As(err, &fe):
		code = fe.Code
		message = fe.Message
	case errors.Is(err, domain.ErrNotFound):
		code = fiber.StatusNotFound
		message = err.Error()
	case errors.Is(err, domain.ErrAlreadyExists):
		code = fiber.StatusConflict
		message = err.Error()
	case errors.Is(err, domain.ErrInvalidCredentials):
		code = fiber.StatusUnauthorized
		message = err.Error()
	case errors.Is(err, domain.ErrAccountNotVerified):
		code = fiber.StatusForbidden
		message = err.Error()
	case errors.Is(err, domain.ErrUnauthorized):
		code = fiber.StatusUnauthorized
		message = err.Error()
	case errors.Is(err, domain.ErrInvalidInput):
		code = fiber.StatusBadRequest
		message = err.Error()
	case errors.Is(err, domain.ErrGmailRequired):
		code = fiber.StatusBadRequest
		message = err.Error()
	case errors.Is(err, domain.ErrInvalidReferral):
		code = fiber.StatusBadRequest
		message = err.Error()
	case errors.Is(err, domain.ErrInvalidOTP):
		code = fiber.StatusBadRequest
		message = err.Error()
	case errors.Is(err, domain.ErrEmailNotRegistered):
		code = fiber.StatusNotFound
		message = err.Error()
	default:
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				code = fiber.StatusConflict
				message = friendlyUniqueMessage(pgErr)
			case "23503":
				code = fiber.StatusBadRequest
				message = "Related record not found or still in use"
			case "23502":
				code = fiber.StatusBadRequest
				message = "Required field is missing"
			}
		}
	}

	if ve := httpx.ValidationFromCtx(c); ve != nil {
		data = fiber.Map{"errors": ve.Errors}
	}

	return c.Status(code).JSON(dto.APIResponse{
		Status:  "error",
		Code:    code,
		Message: message,
		Data:    data,
	})
}

func friendlyUniqueMessage(pgErr *pgconn.PgError) string {
	col := guessUniqueColumn(pgErr.ConstraintName)
	label := strings.ReplaceAll(col, "_", " ")
	if label == "" {
		label = "value"
	}
	return "This " + label + " is already in use"
}

func guessUniqueColumn(constraint string) string {
	constraint = strings.TrimPrefix(constraint, "idx_")
	constraint = strings.TrimPrefix(constraint, "uniq_")
	constraint = strings.TrimSuffix(constraint, "_key")
	parts := strings.Split(constraint, "_")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return constraint
}
