package ailog

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	ListByUser(userID uuid.UUID, feature string, page, limit int) ([]domain.AIProcessingLog, int64, error)
	ListAll(page, limit int) ([]domain.AIProcessingLog, int64, error)
	FindByID(userID, id uuid.UUID) (*domain.AIProcessingLog, error)
	Create(log *domain.AIProcessingLog) error
	Delete(userID, id uuid.UUID) error
	DeleteMany(userID uuid.UUID, ids []uuid.UUID) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) ListByUser(userID uuid.UUID, feature string, page, limit int) ([]domain.AIProcessingLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var (
		rows  []domain.AIProcessingLog
		total int64
	)
	q := r.db.Model(&domain.AIProcessingLog{}).Where("user_id = ?", userID)
	if feature != "" {
		q = q.Where("feature = ?", feature)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&rows).Error
	return rows, total, err
}

func (r *repository) ListAll(page, limit int) ([]domain.AIProcessingLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var (
		rows  []domain.AIProcessingLog
		total int64
	)
	q := r.db.Model(&domain.AIProcessingLog{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&rows).Error
	return rows, total, err
}

func (r *repository) FindByID(userID, id uuid.UUID) (*domain.AIProcessingLog, error) {
	var l domain.AIProcessingLog
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&l).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &l, nil
}

func (r *repository) Create(log *domain.AIProcessingLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	return r.db.Create(log).Error
}

func (r *repository) Delete(userID, id uuid.UUID) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&domain.AIProcessingLog{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repository) DeleteMany(userID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	res := r.db.Where("user_id = ? AND id IN ?", userID, ids).Delete(&domain.AIProcessingLog{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
