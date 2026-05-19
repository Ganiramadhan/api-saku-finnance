package budget

import (
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	List(userID uuid.UUID) ([]domain.Budget, error)
	FindByID(userID, id uuid.UUID) (*domain.Budget, error)
	Create(b *domain.Budget) error
	Update(b *domain.Budget) error
	Delete(userID, id uuid.UUID) error
	SumSpent(userID, walletID, categoryID uuid.UUID, start, end time.Time) (float64, error)
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List(userID uuid.UUID) ([]domain.Budget, error) {
	var rows []domain.Budget
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rows).Error
	return rows, err
}

func (r *repository) FindByID(userID, id uuid.UUID) (*domain.Budget, error) {
	var b domain.Budget
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&b).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *repository) Create(b *domain.Budget) error { return r.db.Create(b).Error }

func (r *repository) Update(b *domain.Budget) error { return r.db.Save(b).Error }

func (r *repository) Delete(userID, id uuid.UUID) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&domain.Budget{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repository) SumSpent(userID, walletID, categoryID uuid.UUID, start, end time.Time) (float64, error) {
	var total float64
	err := r.db.
		Table("transactions").
		Joins("JOIN wallets ON wallets.id = transactions.wallet_id").
		Where("wallets.user_id = ?", userID).
		Where("transactions.wallet_id = ?", walletID).
		Where("transactions.category_id = ?", categoryID).
		Where("transactions.type = ?", "expense").
		Where("transactions.transaction_date BETWEEN ? AND ?", start, end).
		Where("transactions.deleted_at IS NULL").
		Select("COALESCE(SUM(transactions.amount), 0)").
		Scan(&total).Error
	return total, err
}
