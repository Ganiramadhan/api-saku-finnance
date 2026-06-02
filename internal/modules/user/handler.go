package user

import (
	"net/http"
	"strconv"
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

// Me godoc
// @Summary   Get my profile
// @Tags      Users
// @Produce   json
// @Success   200  {object}  dto.APIResponse{data=dto.UserResponse}
// @Security  BearerAuth
// @Router    /api/v1/users/me [get]
func (h *Handler) Me(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	u, err := h.service.Get(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgGetUser, u)
}

// UpdateMe godoc
// @Summary   Update my profile
// @Description Self-service profile update. The role field is silently dropped to prevent privilege escalation.
// @Tags      Users
// @Accept    json
// @Produce   json
// @Param     request  body  dto.UpdateUserRequest  true  "User data"
// @Success   200  {object}  dto.APIResponse{data=dto.UserResponse}
// @Security  BearerAuth
// @Router    /api/v1/users/me [put]
func (h *Handler) UpdateMe(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.UpdateUserRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	req.Role = "" // prevent self-elevation
	u, err := h.service.Update(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateUser, u)
}

func (h *Handler) BindTelegram(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.BindTelegramRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	u, err := h.service.BindTelegram(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateUser, u)
}

func (h *Handler) DisconnectTelegram(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	u, err := h.service.DisconnectTelegram(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateUser, u)
}

// UploadPhoto godoc
// @Summary   Upload user photo
// @Description Uploads an optimized WebP image to Temp/Users/. Returns the object key — pass it as `photo` to PUT /users/me (or admin Create/Update) to attach it to a user.
// @Tags      Users
// @Accept    multipart/form-data
// @Produce   json
// @Param     image  formData  file  true  "Image file (jpeg/png/webp, max 5MB)"
// @Success   200  {object}  dto.APIResponse{data=dto.UploadResponse}
// @Failure   400  {object}  dto.APIResponse
// @Failure   500  {object}  dto.APIResponse
// @Security  BearerAuth
// @Router    /api/v1/users/upload-photo [post]
func (h *Handler) UploadPhoto(c *fiber.Ctx) error {
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

	ctx := c.Context()
	key, err := h.storage.Upload(ctx, file, "Temp/Users")
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, constants.ErrUploadFailed+": "+err.Error())
	}

	preview, err := h.storage.PresignedURL(ctx, key, 7*24*time.Hour)
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, constants.ErrPreviewFailed+": "+err.Error())
	}

	return httpx.OK(c, constants.MsgUploadImage, dto.UploadResponse{
		Image:            key,
		PreviewURL:       preview,
		PreviewExpiresIn: 7 * 24 * 60 * 60,
	})
}

// DeleteMyPhoto godoc
// @Summary   Delete my photo
// @Tags      Users
// @Produce   json
// @Success   200  {object}  dto.APIResponse{data=dto.UserResponse}
// @Security  BearerAuth
// @Router    /api/v1/users/me/photo [delete]
func (h *Handler) DeleteMyPhoto(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	u, err := h.service.DeletePhoto(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgDeleteUserPhoto, u)
}

// DeleteMe godoc
// @Summary   Delete my account
// @Tags      Users
// @Produce   json
// @Success   200  {object}  dto.APIResponse
// @Security  BearerAuth
// @Router    /api/v1/users/me [delete]
func (h *Handler) DeleteMe(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Context(), uid); err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     "saku_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   c.Protocol() == "https",
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	return httpx.OK(c, constants.MsgDeleteUser, nil)
}

// List godoc
// @Summary   List users (admin)
// @Tags      Users
// @Produce   json
// @Param     page   query  int     false  "Page number"     default(1)
// @Param     limit  query  int     false  "Items per page"  default(10)
// @Param     search query  string  false  "Search by name or email"
// @Success   200  {object}  dto.APIResponse{data=[]dto.UserResponse}
// @Security  BearerAuth
// @Router    /api/v1/admin/users [get]
func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search")
	if search == "" {
		search = c.Query("q")
	}
	users, meta, err := h.service.List(c.Context(), page, limit, search)
	if err != nil {
		return err
	}
	return httpx.List(c, constants.MsgGetUsers, users, meta)
}

// Get godoc
// @Summary   Get user by ID (admin)
// @Tags      Users
// @Produce   json
// @Param     id   path  string  true  "User UUID"
// @Success   200  {object}  dto.APIResponse{data=dto.UserResponse}
// @Failure   404  {object}  dto.APIResponse
// @Security  BearerAuth
// @Router    /api/v1/admin/users/{id} [get]
func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	u, err := h.service.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgGetUser, u)
}

// Create godoc
// @Summary   Create user (admin)
// @Tags      Users
// @Accept    json
// @Produce   json
// @Param     request  body  dto.CreateUserRequest  true  "User data"
// @Success   201  {object}  dto.APIResponse{data=dto.UserResponse}
// @Failure   409  {object}  dto.APIResponse
// @Security  BearerAuth
// @Router    /api/v1/admin/users [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	var req dto.CreateUserRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	u, err := h.service.Create(c.Context(), req)
	if err != nil {
		return err
	}
	return httpx.Created(c, constants.MsgCreateUser, u)
}

// Update godoc
// @Summary   Update user (admin)
// @Tags      Users
// @Accept    json
// @Produce   json
// @Param     id       path  string                 true  "User UUID"
// @Param     request  body  dto.UpdateUserRequest  true  "User data"
// @Success   200  {object}  dto.APIResponse{data=dto.UserResponse}
// @Failure   404  {object}  dto.APIResponse
// @Failure   409  {object}  dto.APIResponse
// @Security  BearerAuth
// @Router    /api/v1/admin/users/{id} [put]
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateUserRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	u, err := h.service.Update(c.Context(), id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateUser, u)
}

// Delete godoc
// @Summary   Delete user (admin)
// @Tags      Users
// @Produce   json
// @Param     id   path  string  true  "User UUID"
// @Success   200  {object}  dto.APIResponse
// @Failure   404  {object}  dto.APIResponse
// @Security  BearerAuth
// @Router    /api/v1/admin/users/{id} [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Context(), id); err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgDeleteUser, nil)
}
