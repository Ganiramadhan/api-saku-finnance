package transaction

import (
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ListFilter struct {
	UserID     uuid.UUID
	WalletID   *uuid.UUID
	CategoryID *uuid.UUID
	Type       string
	Source     string
	From       *time.Time
	To         *time.Time
	Search     string
	Page       int
	Limit      int
}

type Repository interface {
	List(f ListFilter) ([]domain.Transaction, int64, error)
	FindByID(userID, id uuid.UUID) (*domain.Transaction, error)
	Create(t *domain.Transaction) error
	Update(t *domain.Transaction) error
	Delete(userID, id uuid.UUID) error
	DB() *gorm.DB
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) scoped(userID uuid.UUID) *gorm.DB {
	return r.db.Model(&domain.Transaction{}).
		Joins("JOIN wallets ON wallets.id = transactions.wallet_id").
		Where("wallets.user_id = ?", userID)
}

func (r *repository) List(f ListFilter) ([]domain.Transaction, int64, error) {
	q := r.scoped(f.UserID)

	if f.WalletID != nil {
		q = q.Where("transactions.wallet_id = ?", *f.WalletID)
	}
	if f.CategoryID != nil {
		q = q.Where("transactions.category_id = ?", *f.CategoryID)
	}
	if f.Type != "" {
		q = q.Where("transactions.type = ?", f.Type)
	}
	if f.Source != "" {
		q = q.Where("transactions.source = ?", f.Source)
	}
	if f.From != nil {
		q = q.Where("transactions.transaction_date >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("transactions.transaction_date <= ?", *f.To)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("transactions.description ILIKE ? OR transactions.merchant_name ILIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	var rows []domain.Transaction
	err := q.
		Order("transactions.transaction_date DESC").
		Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

func (r *repository) FindByID(userID, id uuid.UUID) (*domain.Transaction, error) {
	var t domain.Transaction
	err := r.scoped(userID).
		Where("transactions.id = ?", id).
		First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *repository) Create(t *domain.Transaction) error { return r.db.Create(t).Error }

func (r *repository) Update(t *domain.Transaction) error { return r.db.Save(t).Error }

func (r *repository) Delete(userID, id uuid.UUID) error {
	t, err := r.FindByID(userID, id)
	if err != nil {
		return err
	}
	return r.db.Delete(t).Error
}
