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
}

func (r *CheckoutRequest) Sanitize() {
	r.PlanCode = strings.ToLower(strings.TrimSpace(r.PlanCode))
	r.ReferralCode = sanitizeReferralInput(r.ReferralCode)
}

type CheckoutResponse struct {
	SubscriptionID uuid.UUID `json:"subscription_id"`
	OrderID        string    `json:"order_id"`
	SnapToken      string    `json:"snap_token"`
	RedirectURL    string    `json:"redirect_url"`
	ClientKey      string    `json:"client_key"`
	IsProduction   bool      `json:"is_production"`
}

type ConfirmSubscriptionRequest struct {
	OrderID string `json:"order_id" validate:"required,min=3,max=80"`
}

type SubscriptionResponse struct {
	ID            uuid.UUID  `json:"id"`
	PlanID        uuid.UUID  `json:"plan_id"`
	PlanCode      string     `json:"plan_code"`
	PlanName      string     `json:"plan_name"`
	Status        string     `json:"status"`
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	OrderID       string     `json:"order_id"`
	PaymentType   string     `json:"payment_type,omitempty"`
	StartsAt      *time.Time `json:"starts_at,omitempty"`
	EndsAt        *time.Time `json:"ends_at,omitempty"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	NextBillingAt *time.Time `json:"next_billing_at,omitempty"`
	ReferralCode  string     `json:"referral_code,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type AdminSubscriptionResponse struct {
	SubscriptionResponse
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	UserEmail string    `json:"user_email"`
	UserPhoto string    `json:"user_photo_url,omitempty"`
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
}
