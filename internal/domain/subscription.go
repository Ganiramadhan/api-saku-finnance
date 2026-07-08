package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	SubscriptionStatusPending   = "pending"
	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusCancelled = "cancelled"
	SubscriptionStatusFailed    = "failed"

	PaymentStatusPending   = "pending"
	PaymentStatusPaid      = "paid"
	PaymentStatusExpired   = "expired"
	PaymentStatusCancelled = "cancelled"
	PaymentStatusFailed    = "failed"

	VoucherDiscountFixed   = "fixed"
	VoucherDiscountPercent = "percent"

	PlanPeriodMonthly = "monthly"
	PlanPeriodYearly  = "yearly"
)

type Plan struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code     string    `gorm:"type:varchar(32);not null;uniqueIndex"`
	Name     string    `gorm:"type:varchar(64);not null"`
	Price    float64   `gorm:"type:decimal(18,2);not null;default:0"`
	Currency string    `gorm:"type:varchar(8);not null;default:'IDR'"`
	Period   string    `gorm:"type:varchar(16);not null;default:'monthly'"`
	Features string    `gorm:"type:jsonb"` // JSON array of feature strings
	IsActive bool      `gorm:"not null;default:true"`
	SortKey  int       `gorm:"not null;default:0"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (p *Plan) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

type Subscription struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID uuid.UUID `gorm:"type:uuid;not null;index"`
	PlanID uuid.UUID `gorm:"type:uuid;not null;index"`

	Status   string  `gorm:"type:varchar(16);not null;default:'pending';index"`
	Amount   float64 `gorm:"type:decimal(18,2);not null"`
	Currency string  `gorm:"type:varchar(8);not null;default:'IDR'"`

	MidtransOrderID     string     `gorm:"type:varchar(64);not null;uniqueIndex"`
	MidtransTxnID       string     `gorm:"type:varchar(64);index"`
	MidtransPaymentType string     `gorm:"type:varchar(32)"`
	SnapToken           string     `gorm:"type:varchar(64)"`
	SnapRedirectURL     string     `gorm:"type:varchar(255)"`
	PaymentStatus       string     `gorm:"type:varchar(16);not null;default:'pending';index"`
	PaymentCreatedAt    *time.Time `gorm:"index"`
	PaymentExpiresAt    *time.Time `gorm:"index"`
	PaymentPaidAt       *time.Time
	PaymentExpiredAt    *time.Time
	PendingEmailSent    bool       `gorm:"not null;default:false"`
	ReferralCode        string     `gorm:"type:varchar(32);index"`
	ReferralRewardPaid  bool       `gorm:"not null;default:false"`
	ReferrerID          *uuid.UUID `gorm:"type:uuid;index"`
	VoucherCode         string     `gorm:"type:varchar(32);index"`
	OriginalAmount      float64    `gorm:"type:decimal(18,2);not null;default:0"`
	DiscountAmount      float64    `gorm:"type:decimal(18,2);not null;default:0"`

	StartsAt      *time.Time
	EndsAt        *time.Time
	PaidAt        *time.Time
	TrialEndsAt   *time.Time `gorm:"index"`
	IsTrial       bool       `gorm:"not null;default:false;index"`
	SavedTokenID  string     `gorm:"type:varchar(128)"`
	MidtransSubID string     `gorm:"type:varchar(64);index"`
	NextBillingAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	User     *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Plan     *Plan `gorm:"foreignKey:PlanID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Referrer *User `gorm:"foreignKey:ReferrerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

func (s *Subscription) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type SubscriptionPayment struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	SubscriptionID uuid.UUID `gorm:"type:uuid;not null;index"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index"`
	OrderID        string    `gorm:"type:varchar(64);not null;uniqueIndex"`
	TransactionID  string    `gorm:"type:varchar(64);index"`
	PaymentType    string    `gorm:"type:varchar(32);index"`
	Status         string    `gorm:"type:varchar(16);not null;default:'pending';index"`
	Amount         float64   `gorm:"type:decimal(18,2);not null"`
	Currency       string    `gorm:"type:varchar(8);not null;default:'IDR'"`
	SnapToken      string    `gorm:"type:varchar(64)"`
	RedirectURL    string    `gorm:"type:varchar(255)"`
	CreatedAt      time.Time
	ExpiresAt      *time.Time `gorm:"index"`
	PaidAt         *time.Time
	ExpiredAt      *time.Time
	UpdatedAt      time.Time

	Subscription *Subscription `gorm:"foreignKey:SubscriptionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	User         *User         `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (p *SubscriptionPayment) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

type SubscriptionPaymentEvent struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	SubscriptionID uuid.UUID `gorm:"type:uuid;not null;index"`
	PaymentID      uuid.UUID `gorm:"type:uuid;not null;index"`
	OrderID        string    `gorm:"type:varchar(64);not null;index"`
	FromStatus     string    `gorm:"type:varchar(16)"`
	ToStatus       string    `gorm:"type:varchar(16);not null;index"`
	Reason         string    `gorm:"type:varchar(64)"`
	CreatedAt      time.Time
}

func (e *SubscriptionPaymentEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

type Voucher struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Code           string     `gorm:"type:varchar(32);not null;uniqueIndex"`
	Name           string     `gorm:"type:varchar(96);not null"`
	DiscountType   string     `gorm:"type:varchar(16);not null;default:'fixed'"`
	DiscountValue  float64    `gorm:"type:decimal(18,2);not null"`
	MaxDiscount    float64    `gorm:"type:decimal(18,2);not null;default:0"`
	MinAmount      float64    `gorm:"type:decimal(18,2);not null;default:0"`
	MaxRedemptions int        `gorm:"not null;default:0"`
	UsedCount      int        `gorm:"not null;default:0"`
	StartsAt       *time.Time `gorm:"index"`
	EndsAt         *time.Time `gorm:"index"`
	IsActive       bool       `gorm:"not null;default:true;index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (v *Voucher) BeforeCreate(_ *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}
