package notification

import (
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	List(userID uuid.UUID, limit int) ([]domain.Notification, error)
	Create(n *domain.Notification) error
	MarkRead(userID, id uuid.UUID) error
	MarkAllRead(userID uuid.UUID) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List(userID uuid.UUID, limit int) ([]domain.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var rows []domain.Notification
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *repository) Create(n *domain.Notification) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(n).Error
}

func (r *repository) MarkRead(userID, id uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.Model(&domain.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("read_at", &now).Error
}

func (r *repository) MarkAllRead(userID uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.Model(&domain.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", &now).Error
}
