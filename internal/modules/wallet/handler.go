package wallet

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
// @Summary  List my wallets
// @Tags     Wallets
// @Produce  json
// @Success  200 {object} dto.APIResponse{data=[]dto.WalletResponse}
// @Security BearerAuth
// @Router   /api/v1/wallets [get]
func (h *Handler) List(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	out, err := h.service.List(c.Context(), uid)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgGetWallets, out)
}

// ListTransfers godoc
// @Summary  List wallet transfer history
// @Tags     Wallets
// @Produce  json
// @Param    limit query int false "Max rows"
// @Success  200 {object} dto.APIResponse{data=[]dto.WalletTransferResponse}
// @Security BearerAuth
// @Router   /api/v1/wallets/transfers [get]
func (h *Handler) ListTransfers(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	limit := c.QueryInt("limit", 20)
	out, err := h.service.ListTransfers(c.Context(), uid, limit)
	if err != nil {
		return err
	}
	return httpx.OK(c, "Wallet transfer history retrieved", out)
}

// DeleteTransfers godoc
// @Summary  Delete wallet transfer history rows
// @Tags     Wallets
// @Accept   json
// @Produce  json
// @Param    request body dto.DeleteWalletTransfersRequest true "Transfer history IDs"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/wallets/transfers [delete]
func (h *Handler) DeleteTransfers(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.DeleteWalletTransfersRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if err := h.service.DeleteTransfers(c.Context(), uid, req.IDs); err != nil {
		return err
	}
	return httpx.OK(c, "Wallet transfer history deleted", nil)
}

// Get godoc
// @Summary  Get wallet by ID
// @Tags     Wallets
// @Produce  json
// @Param    id path string true "Wallet UUID"
// @Success  200 {object} dto.APIResponse{data=dto.WalletResponse}
// @Security BearerAuth
// @Router   /api/v1/wallets/{id} [get]
func (h *Handler) Get(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	w, err := h.service.Get(c.Context(), uid, id)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgGetWallet, w)
}

// Create godoc
// @Summary  Create wallet
// @Tags     Wallets
// @Accept   json
// @Produce  json
// @Param    request body dto.CreateWalletRequest true "Wallet data"
// @Success  201 {object} dto.APIResponse{data=dto.WalletResponse}
// @Security BearerAuth
// @Router   /api/v1/wallets [post]
func (h *Handler) Create(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.CreateWalletRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	w, err := h.service.Create(c.Context(), uid, req)
	if err != nil {
		return err
	}
	return httpx.Created(c, constants.MsgCreateWallet, w)
}

// Update godoc
// @Summary  Update wallet
// @Tags     Wallets
// @Accept   json
// @Produce  json
// @Param    id      path string                  true "Wallet UUID"
// @Param    request body dto.UpdateWalletRequest true "Wallet data"
// @Success  200 {object} dto.APIResponse{data=dto.WalletResponse}
// @Security BearerAuth
// @Router   /api/v1/wallets/{id} [put]
func (h *Handler) Update(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	id, err := httpx.ParseUUID(c, "id")
	if err != nil {
		return err
	}
	var req dto.UpdateWalletRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	w, err := h.service.Update(c.Context(), uid, id, req)
	if err != nil {
		return err
	}
	return httpx.OK(c, constants.MsgUpdateWallet, w)
}

// Transfer godoc
// @Summary  Transfer balance between wallets
// @Tags     Wallets
// @Accept   json
// @Produce  json
// @Param    request body dto.TransferWalletBalanceRequest true "Transfer data"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/wallets/transfer [post]
func (h *Handler) Transfer(c *fiber.Ctx) error {
	uid, err := httpx.UserID(c)
	if err != nil {
		return err
	}
	var req dto.TransferWalletBalanceRequest
	if err := httpx.Bind(c, h.validator, &req); err != nil {
		return err
	}
	if err := h.service.Transfer(c.Context(), uid, req); err != nil {
		return err
	}
	return httpx.OK(c, "Wallet balance transferred", nil)
}

// Delete godoc
// @Summary  Delete wallet
// @Tags     Wallets
// @Produce  json
// @Param    id path string true "Wallet UUID"
// @Success  200 {object} dto.APIResponse
// @Security BearerAuth
// @Router   /api/v1/wallets/{id} [delete]
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
	return httpx.OK(c, constants.MsgDeleteWallet, nil)
}
