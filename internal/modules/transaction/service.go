package transaction

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/wallet"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service interface {
	List(ctx context.Context, userID uuid.UUID, q dto.TransactionListQuery) ([]dto.TransactionResponse, *dto.PaginationMeta, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type service struct {
	repo    Repository
	wallets wallet.Repository
	cats    category.Repository
}

func NewService(r Repository, w wallet.Repository, cats category.Repository) Service {
	return &service{repo: r, wallets: w, cats: cats}
}

func toResp(t domain.Transaction) dto.TransactionResponse {
	return dto.TransactionResponse{
		ID:              t.ID,
		WalletID:        t.WalletID,
		CategoryID:      t.CategoryID,
		Amount:          t.Amount,
		Type:            t.Type,
		Description:     t.Description,
		MerchantName:    t.MerchantName,
		TransactionDate: t.TransactionDate,
		Source:          t.Source,
		ConfidenceScore: t.ConfidenceScore,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func parseUUID(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}

func (s *service) List(_ context.Context, userID uuid.UUID, q dto.TransactionListQuery) ([]dto.TransactionResponse, *dto.PaginationMeta, error) {
	f := ListFilter{
		UserID:     userID,
		WalletID:   parseUUID(q.WalletID),
		CategoryID: parseUUID(q.CategoryID),
		Type:       q.Type,
		Source:     q.Source,
		From:       parseTime(q.From),
		To:         parseTime(q.To),
		Search:     q.Search,
		Page:       q.Page,
		Limit:      q.Limit,
	}
	rows, total, err := s.repo.List(f)
	if err != nil {
		return nil, nil, err
	}
	out := make([]dto.TransactionResponse, 0, len(rows))
	for _, t := range rows {
		out = append(out, toResp(t))
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 {
		limit = 20
	}
	return out, dto.NewMeta(page, limit, total), nil
}

func (s *service) Get(_ context.Context, userID, id uuid.UUID) (*dto.TransactionResponse, error) {
	t, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	r := toResp(*t)
	return &r, nil
}

func (s *service) validateWalletAndCategory(userID, walletID, categoryID uuid.UUID) error {
	if _, err := s.wallets.FindByID(userID, walletID); err != nil {
		return err
	}
	if _, err := s.cats.FindAccessible(userID, categoryID); err != nil {
		return err
	}
	return nil
}

func (s *service) Create(_ context.Context, userID uuid.UUID, req dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	if err := s.validateWalletAndCategory(userID, req.WalletID, req.CategoryID); err != nil {
		return nil, err
	}
	source := req.Source
	if source == "" {
		source = domain.TxnSourceManual
	}
	description := strings.TrimSpace(req.Description)
	if source == domain.TxnSourceManual && description == "" {
		return nil, fmt.Errorf("%w: deskripsi wajib diisi untuk transaksi manual", domain.ErrInvalidInput)
	}
	t := domain.Transaction{
		ID:              uuid.New(),
		WalletID:        req.WalletID,
		CategoryID:      req.CategoryID,
		Amount:          req.Amount,
		Type:            req.Type,
		Description:     description,
		MerchantName:    req.MerchantName,
		TransactionDate: req.TransactionDate,
		Source:          source,
		ConfidenceScore: req.ConfidenceScore,
	}

	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		return adjustWalletBalance(tx, t.WalletID, signedAmount(t.Type, t.Amount))
	})
	if err != nil {
		return nil, err
	}
	r := toResp(t)
	return &r, nil
}

func (s *service) Update(_ context.Context, userID, id uuid.UUID, req dto.UpdateTransactionRequest) (*dto.TransactionResponse, error) {
	t, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	prevWalletID := t.WalletID
	prevDelta := signedAmount(t.Type, t.Amount)

	if req.WalletID != nil && *req.WalletID != t.WalletID {
		if _, err := s.wallets.FindByID(userID, *req.WalletID); err != nil {
			return nil, err
		}
		t.WalletID = *req.WalletID
	}
	if req.CategoryID != nil && *req.CategoryID != t.CategoryID {
		if _, err := s.cats.FindAccessible(userID, *req.CategoryID); err != nil {
			return nil, err
		}
		t.CategoryID = *req.CategoryID
	}
	if req.Amount != nil {
		t.Amount = *req.Amount
	}
	if req.Type != "" {
		t.Type = req.Type
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.MerchantName != nil {
		t.MerchantName = *req.MerchantName
	}
	if req.TransactionDate != nil {
		t.TransactionDate = *req.TransactionDate
	}

	newDelta := signedAmount(t.Type, t.Amount)

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(t).Error; err != nil {
			return err
		}
		if prevWalletID == t.WalletID {
			return adjustWalletBalance(tx, t.WalletID, newDelta-prevDelta)
		}
		if err := adjustWalletBalance(tx, prevWalletID, -prevDelta); err != nil {
			return err
		}
		return adjustWalletBalance(tx, t.WalletID, newDelta)
	})
	if err != nil {
		return nil, err
	}
	r := toResp(*t)
	return &r, nil
}

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	t, err := s.repo.FindByID(userID, id)
	if err != nil {
		return err
	}
	delta := signedAmount(t.Type, t.Amount)
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(t).Error; err != nil {
			return err
		}
		return adjustWalletBalance(tx, t.WalletID, -delta)
	})
}

func signedAmount(txType string, amount float64) float64 {
	if txType == domain.TxnTypeExpense {
		return -amount
	}
	return amount
}

func adjustWalletBalance(tx *gorm.DB, walletID uuid.UUID, delta float64) error {
	if delta == 0 {
		return nil
	}
	return tx.Model(&domain.Wallet{}).
		Where("id = ?", walletID).
		UpdateColumn("balance_cached", gorm.Expr("balance_cached + ?", delta)).Error
}
