package upcomingbilling

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	List(userID uuid.UUID) ([]domain.UpcomingBilling, error)
	ListActiveWithUsers() ([]domain.UpcomingBilling, error)
	FindByID(userID, id uuid.UUID) (*domain.UpcomingBilling, error)
	Create(b *domain.UpcomingBilling) error
	Update(b *domain.UpcomingBilling) error
	Delete(userID, id uuid.UUID) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List(userID uuid.UUID) ([]domain.UpcomingBilling, error) {
	var rows []domain.UpcomingBilling
	err := r.db.Where("user_id = ?", userID).Order("due_date ASC, created_at DESC").Find(&rows).Error
	return rows, err
}

func (r *repository) ListActiveWithUsers() ([]domain.UpcomingBilling, error) {
	var rows []domain.UpcomingBilling
	err := r.db.Preload("User").Where("status = ?", domain.BillingStatusActive).Find(&rows).Error
	return rows, err
}

func (r *repository) FindByID(userID, id uuid.UUID) (*domain.UpcomingBilling, error) {
	var row domain.UpcomingBilling
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) Create(b *domain.UpcomingBilling) error { return r.db.Create(b).Error }

func (r *repository) Update(b *domain.UpcomingBilling) error { return r.db.Save(b).Error }

func (r *repository) Delete(userID, id uuid.UUID) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&domain.UpcomingBilling{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
