package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type PlanResponse struct {
	ID       uuid.UUID `json:"id"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Price    float64   `json:"price"`
	Currency string    `json:"currency"`
	Period   string    `json:"period"`
	Features []string  `json:"features"`
	IsActive bool      `json:"is_active"`
}

type CheckoutRequest struct {
	PlanCode     string `json:"plan_code" validate:"required,min=2,max=32" example:"pro"`
	ReferralCode string `json:"referral_code,omitempty" validate:"omitempty,max=32" example:"SAKU1A2B3C4D"`
	VoucherCode  string `json:"voucher_code,omitempty" validate:"omitempty,max=32" example:"HEMAT20"`
}

func (r *CheckoutRequest) Sanitize() {
	r.PlanCode = strings.ToLower(strings.TrimSpace(r.PlanCode))
	r.ReferralCode = sanitizeReferralInput(r.ReferralCode)
	r.VoucherCode = sanitizeReferralInput(r.VoucherCode)
}

type CheckoutResponse struct {
	SubscriptionID uuid.UUID  `json:"subscription_id"`
	OrderID        string     `json:"order_id"`
	SnapToken      string     `json:"snap_token"`
	RedirectURL    string     `json:"redirect_url"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	PaymentStatus  string     `json:"payment_status"`
	OriginalAmount float64    `json:"original_amount"`
	DiscountAmount float64    `json:"discount_amount"`
	Amount         float64    `json:"amount"`
	VoucherCode    string     `json:"voucher_code,omitempty"`
	ClientKey      string     `json:"client_key"`
	IsProduction   bool       `json:"is_production"`
}

type ConfirmSubscriptionRequest struct {
	OrderID string `json:"order_id" validate:"required,min=3,max=80"`
}

type ValidateVoucherRequest struct {
	PlanCode    string `json:"plan_code" validate:"required,min=2,max=32"`
	VoucherCode string `json:"voucher_code" validate:"required,min=3,max=32"`
}

func (r *ValidateVoucherRequest) Sanitize() {
	r.PlanCode = strings.ToLower(strings.TrimSpace(r.PlanCode))
	r.VoucherCode = sanitizeReferralInput(r.VoucherCode)
}

type ValidateVoucherResponse struct {
	Code           string  `json:"code"`
	DiscountType   string  `json:"discount_type"`
	DiscountValue  float64 `json:"discount_value"`
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	PayAmount      float64 `json:"pay_amount"`
	Currency       string  `json:"currency"`
}

type SubscriptionResponse struct {
	ID               uuid.UUID  `json:"id"`
	PlanID           uuid.UUID  `json:"plan_id"`
	PlanCode         string     `json:"plan_code"`
	PlanName         string     `json:"plan_name"`
	Status           string     `json:"status"`
	Amount           float64    `json:"amount"`
	Currency         string     `json:"currency"`
	OrderID          string     `json:"order_id"`
	PaymentStatus    string     `json:"payment_status"`
	PaymentType      string     `json:"payment_type,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	PaymentCreatedAt *time.Time `json:"payment_created_at,omitempty"`
	PaymentPaidAt    *time.Time `json:"payment_paid_at,omitempty"`
	PaymentExpiredAt *time.Time `json:"payment_expired_at,omitempty"`
	OriginalAmount   float64    `json:"original_amount"`
	DiscountAmount   float64    `json:"discount_amount"`
	VoucherCode      string     `json:"voucher_code,omitempty"`
	StartsAt         *time.Time `json:"starts_at,omitempty"`
	EndsAt           *time.Time `json:"ends_at,omitempty"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	NextBillingAt    *time.Time `json:"next_billing_at,omitempty"`
	ReferralCode     string     `json:"referral_code,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type AdminSubscriptionResponse struct {
	SubscriptionResponse
	UserID          uuid.UUID  `json:"user_id"`
	UserName        string     `json:"user_name"`
	UserEmail       string     `json:"user_email"`
	UserPhoto       string     `json:"user_photo_url,omitempty"`
	UserLastLoginAt *time.Time `json:"user_last_login_at,omitempty"`
}

type MidtransWebhook struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	PaymentType       string `json:"payment_type"`
	TransactionID     string `json:"transaction_id"`
	TransactionTime   string `json:"transaction_time"`
	ExpiryTime        string `json:"expiry_time"`
}

type VoucherRequest struct {
	Code           string     `json:"code" validate:"required,min=3,max=32"`
	Name           string     `json:"name" validate:"required,min=2,max=96"`
	DiscountType   string     `json:"discount_type" validate:"required,oneof=fixed percent"`
	DiscountValue  float64    `json:"discount_value" validate:"required,gt=0"`
	MaxDiscount    float64    `json:"max_discount,omitempty" validate:"omitempty,gte=0"`
	MinAmount      float64    `json:"min_amount,omitempty" validate:"omitempty,gte=0"`
	MaxRedemptions int        `json:"max_redemptions,omitempty" validate:"omitempty,gte=0"`
	StartsAt       *time.Time `json:"starts_at,omitempty"`
	EndsAt         *time.Time `json:"ends_at,omitempty"`
	IsActive       bool       `json:"is_active"`
}

func (r *VoucherRequest) Sanitize() {
	r.Code = sanitizeReferralInput(r.Code)
	r.Name = strings.TrimSpace(r.Name)
	r.DiscountType = strings.ToLower(strings.TrimSpace(r.DiscountType))
}

type VoucherResponse struct {
	ID             uuid.UUID  `json:"id"`
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	DiscountType   string     `json:"discount_type"`
	DiscountValue  float64    `json:"discount_value"`
	MaxDiscount    float64    `json:"max_discount"`
	MinAmount      float64    `json:"min_amount"`
	MaxRedemptions int        `json:"max_redemptions"`
	UsedCount      int        `json:"used_count"`
	StartsAt       *time.Time `json:"starts_at,omitempty"`
	EndsAt         *time.Time `json:"ends_at,omitempty"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
