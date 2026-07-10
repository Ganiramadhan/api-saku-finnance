package wallet

import (
	"context"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/subscription"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const freeWalletLimit = 2
const proWalletLimit = 1000000
const premiumWalletLimit = 1000000

type Service interface {
	List(ctx context.Context, userID uuid.UUID) ([]dto.WalletResponse, error)
	ListTransfers(ctx context.Context, userID uuid.UUID, limit int) ([]dto.WalletTransferResponse, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*dto.WalletResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error)
	Transfer(ctx context.Context, userID uuid.UUID, req dto.TransferWalletBalanceRequest) error
	DeleteTransfers(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type service struct {
	repo          Repository
	subscriptions subscription.Service
}

func NewService(r Repository, subs ...subscription.Service) Service {
	var subSvc subscription.Service
	if len(subs) > 0 {
		subSvc = subs[0]
	}
	return &service{repo: r, subscriptions: subSvc}
}

func toResp(w domain.Wallet) dto.WalletResponse {
	var targetName *string
	var targetAmount *float64
	var targetDeadline *time.Time
	if w.Target != nil {
		targetName = &w.Target.Name
		targetAmount = &w.Target.Amount
		targetDeadline = w.Target.Deadline
	}
	return dto.WalletResponse{
		ID:             w.ID,
		UserID:         w.UserID,
		Name:           w.Name,
		Type:           w.Type,
		Currency:       w.Currency,
		Balance:        w.BalanceCached,
		IsDefault:      w.IsDefault,
		TargetName:     targetName,
		TargetAmount:   targetAmount,
		TargetDeadline: targetDeadline,
		CreatedAt:      w.CreatedAt,
		UpdatedAt:      w.UpdatedAt,
	}
}

func transferToResp(t domain.WalletTransfer) dto.WalletTransferResponse {
	fromName := ""
	if t.FromWallet != nil {
		fromName = t.FromWallet.Name
	}
	toName := ""
	if t.ToWallet != nil {
		toName = t.ToWallet.Name
	}
	return dto.WalletTransferResponse{
		ID:             t.ID,
		UserID:         t.UserID,
		FromWalletID:   t.FromWalletID,
		FromWalletName: fromName,
		ToWalletID:     t.ToWalletID,
		ToWalletName:   toName,
		Amount:         t.Amount,
		Currency:       t.Currency,
		Note:           t.Note,
		CreatedAt:      t.CreatedAt,
	}
}

func (s *service) List(_ context.Context, userID uuid.UUID) ([]dto.WalletResponse, error) {
	rows, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.WalletResponse, 0, len(rows))
	for _, w := range rows {
		out = append(out, toResp(w))
	}
	return out, nil
}

func (s *service) ListTransfers(_ context.Context, userID uuid.UUID, limit int) ([]dto.WalletTransferResponse, error) {
	rows, err := s.repo.ListTransfers(userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.WalletTransferResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, transferToResp(row))
	}
	return out, nil
}

func (s *service) Get(_ context.Context, userID, id uuid.UUID) (*dto.WalletResponse, error) {
	w, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	r := toResp(*w)
	return &r, nil
}

func (s *service) Create(ctx context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
	if err := s.enforceFreeWalletLimit(ctx, userID); err != nil {
		return nil, err
	}
	currency := req.Currency
	if currency == "" {
		currency = "IDR"
	}
	walletType := req.Type
	if walletType == "" {
		walletType = domain.WalletTypeCash
	}
	w := domain.Wallet{
		ID:            uuid.New(),
		UserID:        userID,
		Name:          req.Name,
		Type:          walletType,
		Currency:      currency,
		BalanceCached: req.Balance,
		IsDefault:     req.IsDefault,
	}
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		txr := s.repo.WithTx(tx)
		if w.IsDefault {
			if err := txr.ClearDefault(tx, userID, uuid.Nil); err != nil {
				return err
			}
		}
		if err := txr.Create(&w); err != nil {
			return err
		}
		return upsertWalletTarget(tx, w.ID, req.TargetName, req.TargetAmount, req.TargetDeadline)
	})
	if err != nil {
		return nil, err
	}
	created, err := s.repo.FindByID(userID, w.ID)
	if err != nil {
		return nil, err
	}
	r := toResp(*created)
	return &r, nil
}

func (s *service) Update(_ context.Context, userID, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error) {
	w, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		w.Name = req.Name
	}
	if req.Type != "" {
		w.Type = req.Type
	}
	if req.Currency != "" {
		w.Currency = req.Currency
	}
	if req.Balance != nil {
		w.BalanceCached = *req.Balance
	}
	if req.IsDefault != nil {
		w.IsDefault = *req.IsDefault
	}

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		txr := s.repo.WithTx(tx)
		if req.IsDefault != nil && *req.IsDefault {
			if err := txr.ClearDefault(tx, userID, w.ID); err != nil {
				return err
			}
		}
		if err := txr.Update(w); err != nil {
			return err
		}
		if req.TargetName != nil || req.TargetAmount != nil || req.TargetDeadline != nil {
			return upsertWalletTarget(tx, w.ID, req.TargetName, req.TargetAmount, req.TargetDeadline)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	w, err = s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	r := toResp(*w)
	return &r, nil
}

func (s *service) Transfer(_ context.Context, userID uuid.UUID, req dto.TransferWalletBalanceRequest) error {
	if req.FromWalletID == req.ToWalletID {
		return fiber.NewError(fiber.StatusBadRequest, "Source and destination wallets must be different")
	}

	from, err := s.repo.FindByID(userID, req.FromWalletID)
	if err != nil {
		return err
	}
	to, err := s.repo.FindByID(userID, req.ToWalletID)
	if err != nil {
		return err
	}
	if from.Currency != to.Currency {
		return fiber.NewError(fiber.StatusBadRequest, "Wallet currencies must match")
	}
	if from.BalanceCached < req.Amount {
		return fiber.NewError(fiber.StatusBadRequest, "Source wallet balance is not enough")
	}
	if from.Target != nil && !req.ClearSourceTarget {
		return fiber.NewError(fiber.StatusConflict, "Source wallet has an active target. Confirm target removal before transferring balance")
	}

	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		txr := s.repo.WithTx(tx)
		from.BalanceCached -= req.Amount
		to.BalanceCached += req.Amount
		if err := txr.Update(from); err != nil {
			return err
		}
		if err := txr.Update(to); err != nil {
			return err
		}
		if err := txr.CreateTransfer(&domain.WalletTransfer{
			ID:           uuid.New(),
			UserID:       userID,
			FromWalletID: from.ID,
			ToWalletID:   to.ID,
			Amount:       req.Amount,
			Currency:     from.Currency,
			Note:         strings.TrimSpace(req.Note),
		}); err != nil {
			return err
		}
		if from.Target != nil {
			return tx.Where("wallet_id = ?", from.ID).Delete(&domain.WalletTarget{}).Error
		}
		return nil
	})
}

func (s *service) DeleteTransfers(_ context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	return s.repo.DeleteTransfers(userID, ids)
}

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(userID, id)
}

func (s *service) enforceFreeWalletLimit(ctx context.Context, userID uuid.UUID) error {
	limit := freeWalletLimit
	message := "Free plan can create up to 2 wallets. Upgrade to Pro for more wallets"
	if s.subscriptions != nil {
		planCode, active, err := s.subscriptions.ActivePlanCode(ctx, userID)
		if err != nil {
			return err
		}
		if active {
			limit = proWalletLimit
			message = "Pro plan includes unlimited wallets"
			if strings.Contains(planCode, "premium") {
				limit = premiumWalletLimit
				message = "Premium plan includes unlimited wallets"
			}
		} else {
			hasPaidHistory, err := s.subscriptions.HasPaidSubscriptionHistory(ctx, userID)
			if err != nil {
				return err
			}
			if hasPaidHistory {
				return fiber.NewError(fiber.StatusForbidden, "Your subscription has expired. Renew your plan to create more wallets")
			}
		}
	}
	rows, err := s.repo.List(userID)
	if err != nil {
		return err
	}
	if len(rows) >= limit {
		return fiber.NewError(fiber.StatusForbidden, message)
	}
	return nil
}

func upsertWalletTarget(tx *gorm.DB, walletID uuid.UUID, name *string, amount *float64, deadline *time.Time) error {
	if name == nil && amount == nil && deadline == nil {
		return nil
	}
	if (name == nil || *name == "") && (amount == nil || *amount <= 0) && (deadline == nil || deadline.IsZero()) {
		return tx.Where("wallet_id = ?", walletID).Delete(&domain.WalletTarget{}).Error
	}

	var target domain.WalletTarget
	err := tx.Where("wallet_id = ?", walletID).First(&target).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		target = domain.WalletTarget{WalletID: walletID}
	}
	if name != nil {
		target.Name = *name
	}
	if amount != nil {
		target.Amount = *amount
	}
	if deadline != nil {
		if deadline.IsZero() {
			target.Deadline = nil
		} else {
			target.Deadline = deadline
		}
	}
	if target.Name == "" {
		target.Name = "Kantong Tujuan"
	}
	if target.Amount <= 0 {
		return tx.Where("wallet_id = ?", walletID).Delete(&domain.WalletTarget{}).Error
	}
	if target.ID == uuid.Nil {
		return tx.Create(&target).Error
	}
	return tx.Save(&target).Error
}
