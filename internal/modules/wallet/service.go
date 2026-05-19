package wallet

import (
	"context"

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
	return dto.WalletResponse{
		ID:             w.ID,
		UserID:         w.UserID,
		Name:           w.Name,
		Type:           w.Type,
		Currency:       w.Currency,
		Balance:        w.BalanceCached,
		IsDefault:      w.IsDefault,
		TargetName:     w.TargetName,
		TargetAmount:   w.TargetAmount,
		TargetDeadline: w.TargetDeadline,
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
	w := domain.Wallet{
		ID:             uuid.New(),
		UserID:         userID,
		Name:           req.Name,
		Type:           req.Type,
		Currency:       currency,
		BalanceCached:  req.Balance,
		IsDefault:      req.IsDefault,
		TargetName:     req.TargetName,
		TargetAmount:   req.TargetAmount,
		TargetDeadline: req.TargetDeadline,
	}
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		txr := s.repo.WithTx(tx)
		if err := txr.Create(&w); err != nil {
			return err
		}
		if w.IsDefault {
			return txr.ClearDefault(tx, userID, w.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	r := toResp(w)
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
	if req.TargetName != nil {
		if *req.TargetName == "" {
			w.TargetName = nil
		} else {
			w.TargetName = req.TargetName
		}
	}
	if req.TargetAmount != nil {
		if *req.TargetAmount == 0 {
			w.TargetAmount = nil
		} else {
			w.TargetAmount = req.TargetAmount
		}
	}
	if req.TargetDeadline != nil {
		if req.TargetDeadline.IsZero() {
			w.TargetDeadline = nil
		} else {
			w.TargetDeadline = req.TargetDeadline
		}
	}

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		txr := s.repo.WithTx(tx)
		if err := txr.Update(w); err != nil {
			return err
		}
		if w.IsDefault {
			return txr.ClearDefault(tx, userID, w.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	r := toResp(*w)
	return &r, nil
}

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(userID, id)
}
