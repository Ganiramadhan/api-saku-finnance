package wallet

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	List(userID uuid.UUID) ([]domain.Wallet, error)
	FindByID(userID, id uuid.UUID) (*domain.Wallet, error)
	Create(w *domain.Wallet) error
	Update(w *domain.Wallet) error
	Delete(userID, id uuid.UUID) error
	ClearDefault(tx *gorm.DB, userID, exceptID uuid.UUID) error
	WithTx(tx *gorm.DB) Repository
	DB() *gorm.DB
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) WithTx(tx *gorm.DB) Repository { return &repository{db: tx} }

func (r *repository) List(userID uuid.UUID) ([]domain.Wallet, error) {
	var out []domain.Wallet
	err := r.db.Preload("Target").
		Where("user_id = ?", userID).
		Order("is_default DESC, created_at DESC").
		Find(&out).Error
	return out, err
}

func (r *repository) FindByID(userID, id uuid.UUID) (*domain.Wallet, error) {
	var w domain.Wallet
	if err := r.db.Preload("Target").Where("id = ? AND user_id = ?", id, userID).First(&w).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

func (r *repository) Create(w *domain.Wallet) error { return r.db.Create(w).Error }

func (r *repository) Update(w *domain.Wallet) error { return r.db.Save(w).Error }

func (r *repository) Delete(userID, id uuid.UUID) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&domain.Wallet{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repository) ClearDefault(tx *gorm.DB, userID, exceptID uuid.UUID) error {
	db := tx
	if db == nil {
		db = r.db
	}
	return db.Model(&domain.Wallet{}).
		Where("user_id = ? AND id <> ? AND is_default = TRUE", userID, exceptID).
		Update("is_default", false).Error
}
