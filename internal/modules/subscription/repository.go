package subscription

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	// Plans
	ListActivePlans() ([]domain.Plan, error)
	FindPlanByCode(code string) (*domain.Plan, error)
	FindPlanByID(id uuid.UUID) (*domain.Plan, error)

	// Subscriptions
	CreateSubscription(s *domain.Subscription) error
	UpdateSubscription(s *domain.Subscription) error
	FindByOrderID(orderID string) (*domain.Subscription, error)
	FindActiveByUserID(userID uuid.UUID) (*domain.Subscription, error)
	FindPendingByUserID(userID uuid.UUID) (*domain.Subscription, error)
	FindByUserID(userID, id uuid.UUID) (*domain.Subscription, error)
	ListByUserID(userID uuid.UUID) ([]domain.Subscription, error)
	ListAll(limit, offset int) ([]domain.Subscription, error)
	ListActiveForReminder() ([]domain.Subscription, error)
}

func (r *repository) FindByUserID(userID, id uuid.UUID) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := r.db.Preload("Plan").Where("id = ? AND user_id = ?", id, userID).First(&s).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) ListActivePlans() ([]domain.Plan, error) {
	var rows []domain.Plan
	err := r.db.Where("is_active = ?", true).Order("sort_key ASC").Find(&rows).Error
	return rows, err
}

func (r *repository) FindPlanByCode(code string) (*domain.Plan, error) {
	var p domain.Plan
	if err := r.db.Where("code = ? AND is_active = ?", code, true).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *repository) FindPlanByID(id uuid.UUID) (*domain.Plan, error) {
	var p domain.Plan
	if err := r.db.Where("id = ?", id).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *repository) CreateSubscription(s *domain.Subscription) error {
	return r.db.Create(s).Error
}

func (r *repository) UpdateSubscription(s *domain.Subscription) error {
	return r.db.Save(s).Error
}

func (r *repository) FindByOrderID(orderID string) (*domain.Subscription, error) {
	var s domain.Subscription
	if err := r.db.Where("midtrans_order_id = ?", orderID).First(&s).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *repository) FindActiveByUserID(userID uuid.UUID) (*domain.Subscription, error) {

	var rows []domain.Subscription
	err := r.db.
		Preload("Plan").
		Where("user_id = ? AND status = ?", userID, domain.SubscriptionStatusActive).
		Order("created_at DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, domain.ErrNotFound
	}
	return &rows[0], nil
}

func (r *repository) FindPendingByUserID(userID uuid.UUID) (*domain.Subscription, error) {
	var rows []domain.Subscription
	err := r.db.
		Preload("Plan").
		Where("user_id = ? AND status = ?", userID, domain.SubscriptionStatusPending).
		Order("created_at DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, domain.ErrNotFound
	}
	return &rows[0], nil
}

func (r *repository) ListByUserID(userID uuid.UUID) ([]domain.Subscription, error) {
	var rows []domain.Subscription
	err := r.db.
		Preload("Plan").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *repository) ListAll(limit, offset int) ([]domain.Subscription, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []domain.Subscription
	err := r.db.
		Joins("JOIN users ON users.id = subscriptions.user_id AND users.deleted_at IS NULL").
		Preload("Plan").
		Preload("User").
		Order("subscriptions.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	return rows, err
}

func (r *repository) ListActiveForReminder() ([]domain.Subscription, error) {
	var rows []domain.Subscription
	err := r.db.
		Preload("Plan").
		Preload("User").
		Where("status = ?", domain.SubscriptionStatusActive).
		Find(&rows).Error
	return rows, err
}
