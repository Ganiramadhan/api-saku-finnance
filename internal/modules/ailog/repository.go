package ailog

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	ListByUser(userID uuid.UUID, feature string, page, limit int) ([]domain.AIProcessingLog, int64, error)
	ListAll(page, limit int) ([]domain.AIProcessingLog, int64, error)
	Create(log *domain.AIProcessingLog) error
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

func (r *repository) Create(log *domain.AIProcessingLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	return r.db.Create(log).Error
}
