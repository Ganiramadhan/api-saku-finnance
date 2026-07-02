package subscription

import (
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	// Plans
	ListActivePlans() ([]domain.Plan, error)
	FindPlanByCode(code string) (*domain.Plan, error)
	FindPlanByID(id uuid.UUID) (*domain.Plan, error)

	// Vouchers
	ListVouchers(limit, offset int) ([]domain.Voucher, error)
	FindVoucherByID(id uuid.UUID) (*domain.Voucher, error)
	FindVoucherByCode(code string) (*domain.Voucher, error)
	CreateVoucher(v *domain.Voucher) error
	UpdateVoucher(v *domain.Voucher) error
	DeleteVoucher(id uuid.UUID) error

	// Subscriptions
	CreateSubscription(s *domain.Subscription) error
	UpdateSubscription(s *domain.Subscription) error
	ClaimPendingEmail(id uuid.UUID) (bool, error)
	FindByOrderID(orderID string) (*domain.Subscription, error)
	FindActiveByUserID(userID uuid.UUID) (*domain.Subscription, error)
	FindPendingByUserID(userID uuid.UUID) (*domain.Subscription, error)
	HasPaidSubscriptionHistory(userID uuid.UUID) (bool, error)
	FindByUserID(userID, id uuid.UUID) (*domain.Subscription, error)
	ListByUserID(userID uuid.UUID) ([]domain.Subscription, error)
	ListAll(limit, offset int) ([]domain.Subscription, error)
	ListActiveForReminder() ([]domain.Subscription, error)
	ExpirePendingBefore(now time.Time) error

	// Subscription payments
	CreatePayment(p *domain.SubscriptionPayment) error
	UpdatePayment(p *domain.SubscriptionPayment) error
	FindPaymentByOrderID(orderID string) (*domain.SubscriptionPayment, error)
	CreatePaymentEvent(e *domain.SubscriptionPaymentEvent) error
	IncrementVoucherUsage(code string) error
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

func (r *repository) ListVouchers(limit, offset int) ([]domain.Voucher, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []domain.Voucher
	err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, err
}

func (r *repository) FindVoucherByID(id uuid.UUID) (*domain.Voucher, error) {
	var v domain.Voucher
	if err := r.db.Where("id = ?", id).First(&v).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *repository) FindVoucherByCode(code string) (*domain.Voucher, error) {
	var v domain.Voucher
	if err := r.db.Where("code = ?", code).First(&v).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *repository) CreateVoucher(v *domain.Voucher) error {
	return r.db.Create(v).Error
}

func (r *repository) UpdateVoucher(v *domain.Voucher) error {
	return r.db.Save(v).Error
}

func (r *repository) DeleteVoucher(id uuid.UUID) error {
	return r.db.Delete(&domain.Voucher{}, "id = ?", id).Error
}

func (r *repository) CreateSubscription(s *domain.Subscription) error {
	return r.db.Create(s).Error
}

func (r *repository) UpdateSubscription(s *domain.Subscription) error {
	return r.db.Save(s).Error
}

func (r *repository) ClaimPendingEmail(id uuid.UUID) (bool, error) {
	result := r.db.Model(&domain.Subscription{}).
		Where("id = ? AND pending_email_sent = ?", id, false).
		Update("pending_email_sent", true)
	return result.RowsAffected == 1, result.Error
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

func (r *repository) HasPaidSubscriptionHistory(userID uuid.UUID) (bool, error) {
	var total int64
	err := r.db.Model(&domain.Subscription{}).
		Joins("JOIN plans ON plans.id = subscriptions.plan_id").
		Where("subscriptions.user_id = ?", userID).
		Where("subscriptions.payment_status = ?", domain.PaymentStatusPaid).
		Where("plans.price > 0").
		Count(&total).Error
	return total > 0, err
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

func (r *repository) ExpirePendingBefore(now time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.SubscriptionPayment{}).
			Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", domain.PaymentStatusPending, now).
			Updates(map[string]any{
				"status":     domain.PaymentStatusExpired,
				"expired_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Subscription{}).
			Where("status = ? AND payment_status = ?", domain.SubscriptionStatusPending, domain.PaymentStatusPending).
			Where("payment_expires_at IS NOT NULL AND payment_expires_at <= ?", now).
			Updates(map[string]any{
				"payment_status":     domain.PaymentStatusExpired,
				"payment_expired_at": now,
				"next_billing_at":    nil,
			}).Error
	})
}

func (r *repository) CreatePayment(p *domain.SubscriptionPayment) error {
	return r.db.Create(p).Error
}

func (r *repository) UpdatePayment(p *domain.SubscriptionPayment) error {
	return r.db.Save(p).Error
}

func (r *repository) FindPaymentByOrderID(orderID string) (*domain.SubscriptionPayment, error) {
	var p domain.SubscriptionPayment
	if err := r.db.Preload("Subscription").Where("order_id = ?", orderID).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *repository) CreatePaymentEvent(e *domain.SubscriptionPaymentEvent) error {
	return r.db.Create(e).Error
}

func (r *repository) IncrementVoucherUsage(code string) error {
	return r.db.Model(&domain.Voucher{}).
		Where("code = ?", code).
		UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error
}
