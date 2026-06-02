package support

import (
	"net/http"
	"time"

	"github.com/ganiramadhan/starter-go/internal/constants"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/platform/storage"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service   Service
	storage   storage.Storage
	validator *validator.Validator
}

func NewHandler(s Service, st storage.Storage, v *validator.Validator) *Handler {
	return &Handler{service: s, storage: st, validator: v}
}

func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.List(c.Context(), uid, httpx.Role(c), c.Query("status", "all"))
	if err != nil {
		return err
	}
	return httpx.OK(c, "Support tickets retrieved", out)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CreateSupportTicketRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, "Support ticket created", out)
}

func (h *Handler) Reply(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.ReplySupportTicketRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Reply(c.Context(), uid, httpx.Role(c), id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Support reply sent", out)
}

func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateSupportTicketStatusRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.UpdateStatus(c.Context(), id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Support ticket updated", out)
}

func (h *Handler) UploadAttachment(c *fiber.Ctx) error {
	if h.storage == nil {
		return fiber.NewError(http.StatusInternalServerError, "Storage is not configured")
	}
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	file, err := c.FormFile("image")
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, constants.ErrImageRequired)
	}
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
	if !allowed[file.Header.Get("Content-Type")] {
		return fiber.NewError(http.StatusBadRequest, constants.ErrInvalidFileType)
	}
	if file.Size > constants.MaxUploadSize {
		return fiber.NewError(http.StatusBadRequest, constants.ErrFileTooLarge)
	}
	key, err := h.storage.Upload(c.Context(), file, supportAttachmentFolder(uid))
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, constants.ErrUploadFailed+": "+err.Error())
	}
	preview, err := h.storage.PresignedURL(c.Context(), key, 24*time.Hour)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, constants.ErrPreviewFailed+": "+err.Error())
	}
	return httpx.OK(c, "Support attachment uploaded", dto.UploadResponse{
		Image:            key,
		PreviewURL:       preview,
		PreviewExpiresIn: 24 * 60 * 60,
	})
}
