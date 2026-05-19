package category

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	List(userID uuid.UUID, txType string) ([]domain.Category, error)
	FindForUser(userID, id uuid.UUID) (*domain.Category, error)
	FindAccessible(userID, id uuid.UUID) (*domain.Category, error)
	Create(c *domain.Category) error
	Update(c *domain.Category) error
	Delete(userID, id uuid.UUID) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List(userID uuid.UUID, txType string) ([]domain.Category, error) {
	var rows []domain.Category
	q := r.db.Where("user_id = ? OR is_system = TRUE", userID)
	if txType != "" {
		q = q.Where("type = ?", txType)
	}
	err := q.Order("is_system ASC, name ASC").Find(&rows).Error
	return rows, err
}

func (r *repository) FindForUser(userID, id uuid.UUID) (*domain.Category, error) {
	var c domain.Category
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *repository) FindAccessible(userID, id uuid.UUID) (*domain.Category, error) {
	var c domain.Category
	if err := r.db.Where("id = ? AND (user_id = ? OR is_system = TRUE)", id, userID).First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *repository) Create(c *domain.Category) error { return r.db.Create(c).Error }

func (r *repository) Update(c *domain.Category) error { return r.db.Save(c).Error }

func (r *repository) Delete(userID, id uuid.UUID) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&domain.Category{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
