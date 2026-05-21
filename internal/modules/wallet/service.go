package wallet

import (
	"context"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service interface {
	List(ctx context.Context, userID uuid.UUID) ([]dto.WalletResponse, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*dto.WalletResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateWalletRequest) (*dto.WalletResponse, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type service struct{ repo Repository }

func NewService(r Repository) Service { return &service{repo: r} }

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

func (s *service) Get(_ context.Context, userID, id uuid.UUID) (*dto.WalletResponse, error) {
	w, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	r := toResp(*w)
	return &r, nil
}

func (s *service) Create(_ context.Context, userID uuid.UUID, req dto.CreateWalletRequest) (*dto.WalletResponse, error) {
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

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(userID, id)
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
