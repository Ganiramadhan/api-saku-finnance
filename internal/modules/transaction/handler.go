package transaction

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/constants"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/httpx"
	"github.com/ganiramadhan/starter-go/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type Handler struct {
	service   Service
	export    ExportService
	validator *validator.Validator
}

func NewHandler(s Service, v *validator.Validator) *Handler {
	return &Handler{service: s, validator: v}
}

func (h *Handler) SetExportService(e ExportService) { h.export = e }

// List godoc
// @Summary  List my transactions
// @Tags     Transactions
// @Produce  json
// @Param    wallet_id   query string false "Wallet UUID"
// @Param    category_id query string false "Category UUID"
// @Param    type        query string false "income | expense"
// @Param    source      query string false "manual | ai_ocr | import | api"
// @Param    from        query string false "ISO date / RFC3339 start"
// @Param    to          query string false "ISO date / RFC3339 end"
// @Param    q           query string false "Search description / merchant"
// @Param    page        query int    false "Page" default(1)
// @Param    limit       query int    false "Limit" default(20)
// @Success  200 {object} dto.APIResponse{data=[]dto.TransactionResponse}
// @Security BearerAuth
// @Router   /api/v1/transactions [get]
func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var q dto.TransactionListQuery
	if err := c.QueryParser(&q); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, constants.ErrInvalidRequest)
	}
	rows, meta, err := h.service.List(c.Context(), uid, q)
	if err != nil {
		return err
	}
	return httpx.List(c, constants.MsgGetTransactions, rows, meta)
}

// Get godoc
// @Summary  Get transaction
// @Tags     Transactions
// @Produce  json
// @Param    id path string true "Transaction UUID"
// @Success  200 {object} dto.APIResponse{data=dto.TransactionResponse}
// @Security BearerAuth
// @Router   /api/v1/transactions/{id} [get]
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
	return httpx.OK(c, constants.MsgGetTransaction, out)
}

// Create godoc
// @Summary  Create transaction
// @Tags     Transactions
// @Accept   json
// @Produce  json
// @Param    request body dto.CreateTransactionRequest true "Transaction data"
// @Success  201 {object} dto.APIResponse{data=dto.TransactionResponse}
// @Security BearerAuth
// @Router   /api/v1/transactions [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CreateTransactionRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	out, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, constants.MsgCreateTransaction, out)
}

// Update godoc
// @Summary  Update transaction
// @Tags     Transactions
// @Accept   json
// @Produce  json
// @Param    id      path string                       true "Transaction UUID"
// @Param    request body dto.UpdateTransactionRequest true "Transaction data"
// @Success  200 {object} dto.APIResponse{data=dto.TransactionResponse}
// @Security BearerAuth
// @Router   /api/v1/transactions/{id} [put]
func (h *Handler) Update(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateTransactionRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	applyRawUpdateTransactionIDs(c.Body(), &req)
	out, err := h.service.Update(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateTransaction, out)
}

// Delete godoc
// @Summary  Delete transaction
// @Tags     Transactions
// @Produce  json
// @Param    id path string true "Transaction UUID"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/transactions/{id} [delete]
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
	return httpx.OK(c, constants.MsgDeleteTransaction, nil)
}

func applyRawUpdateTransactionIDs(body []byte, req *dto.UpdateTransactionRequest) {
	if len(body) == 0 || req == nil {
		return
	}
	var raw struct {
		WalletID   *string `json:"wallet_id"`
		CategoryID *string `json:"category_id"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return
	}
	if raw.WalletID != nil {
		req.WalletID = strings.TrimSpace(*raw.WalletID)
	}
	if raw.CategoryID != nil {
		req.CategoryID = strings.TrimSpace(*raw.CategoryID)
	}
}

// Export godoc
// @Summary  Export transactions to XLSX
// @Tags     Transactions
// @Produce  application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param    wallet_id   query string false "Wallet UUID"
// @Param    category_id query string false "Category UUID"
// @Param    type        query string false "income | expense"
// @Param    month       query string false "YYYY-MM"
// @Param    from        query string false "ISO date / RFC3339 start"
// @Param    to          query string false "ISO date / RFC3339 end"
// @Success  200 {file} file
// @Security BearerAuth
// @Router   /api/v1/transactions/export [get]
func (h *Handler) Export(c *fiber.Ctx) error {
	if h.export == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "export service not configured")
	}
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}

	f := ExportFilter{UserID: uid, Type: c.Query("type")}
	if v := c.Query("wallet_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.WalletID = &id
		}
	}
	if v := c.Query("category_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.CategoryID = &id
		}
	}
	if v := c.Query("month"); v != "" {
		if t, err := time.Parse("2006-01", v); err == nil {
			start := t
			end := t.AddDate(0, 1, 0).Add(-time.Second)
			f.From = &start
			f.To = &end
		}
	}
	if v := c.Query("from"); v != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				f.From = &t
				break
			}
		}
	}
	if v := c.Query("to"); v != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, err := time.Parse(layout, v); err == nil {
				f.To = &t
				break
			}
		}
	}

	data, filename, err := h.export.ExportXLSX(c.Context(), f)
	if err != nil {
		return err
	}
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	return c.Send(data)
}
