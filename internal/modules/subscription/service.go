package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/internal/platform/mailer"
	"github.com/google/uuid"
)

const (
	referralPaymentReward int64 = 2000
	snapPaymentExpiry           = 1 * time.Hour
)

type Service interface {
	// Public
	ListPlans(ctx context.Context) ([]dto.PlanResponse, error)
	// Authed
	Checkout(ctx context.Context, userID uuid.UUID, req dto.CheckoutRequest) (*dto.CheckoutResponse, error)
	MySubscriptions(ctx context.Context, userID uuid.UUID) ([]dto.SubscriptionResponse, error)
	ActiveSubscription(ctx context.Context, userID uuid.UUID) (*dto.SubscriptionResponse, error)
	ConfirmCheckout(ctx context.Context, userID uuid.UUID, req dto.ConfirmSubscriptionRequest) (*dto.SubscriptionResponse, error)
	Cancel(ctx context.Context, userID, id uuid.UUID) error
	HasActiveProSubscription(ctx context.Context, userID uuid.UUID) (bool, error)
	ActivePlanCode(ctx context.Context, userID uuid.UUID) (string, bool, error)
	// Admin
	ListAllAdmin(ctx context.Context, limit, offset int) ([]dto.AdminSubscriptionResponse, error)
	// Webhook
	HandleWebhook(ctx context.Context, payload dto.MidtransWebhook) error
}

type service struct {
	repo      Repository
	users     user.Repository
	midtrans  *MidtransClient
	mailer    mailer.Mailer
	clientKey string
	isProd    bool
}

func NewService(repo Repository, users user.Repository, m *MidtransClient, mailer mailer.Mailer, clientKey string, isProd bool) Service {
	return &service{repo: repo, users: users, midtrans: m, mailer: mailer, clientKey: clientKey, isProd: isProd}
}

func parseFeatures(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

func toPlanResp(p domain.Plan) dto.PlanResponse {
	return dto.PlanResponse{
		ID:       p.ID,
		Code:     p.Code,
		Name:     p.Name,
		Price:    p.Price,
		Currency: p.Currency,
		Period:   p.Period,
		Features: parseFeatures(p.Features),
		IsActive: p.IsActive,
	}
}

func toSubResp(s domain.Subscription) dto.SubscriptionResponse {
	resp := dto.SubscriptionResponse{
		ID:            s.ID,
		PlanID:        s.PlanID,
		Status:        s.Status,
		Amount:        s.Amount,
		Currency:      s.Currency,
		OrderID:       s.MidtransOrderID,
		PaymentType:   s.MidtransPaymentType,
		ExpiresAt:     s.PaymentExpiresAt,
		StartsAt:      s.StartsAt,
		EndsAt:        s.EndsAt,
		PaidAt:        s.PaidAt,
		NextBillingAt: s.NextBillingAt,
		ReferralCode:  s.ReferralCode,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
	if s.Plan != nil {
		resp.PlanCode = s.Plan.Code
		resp.PlanName = s.Plan.Name
	}
	return resp
}

func (s *service) ListPlans(_ context.Context) ([]dto.PlanResponse, error) {
	rows, err := s.repo.ListActivePlans()
	if err != nil {
		return nil, err
	}
	out := make([]dto.PlanResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, toPlanResp(p))
	}
	return out, nil
}

func (s *service) Checkout(ctx context.Context, userID uuid.UUID, req dto.CheckoutRequest) (*dto.CheckoutResponse, error) {
	plan, err := s.repo.FindPlanByCode(req.PlanCode)
	if err != nil {
		return nil, err
	}
	if plan.Price <= 0 {
		return nil, fmt.Errorf("plan %q is free and does not require checkout", plan.Code)
	}
	if pending, err := s.repo.FindPendingByUserID(userID); err == nil && pending != nil {
		if s.expirePendingIfNeeded(ctx, pending, time.Now().UTC()) {
			if err := s.repo.UpdateSubscription(pending); err != nil {
				return nil, err
			}
		} else {
			if pending.PlanID != plan.ID {
				return nil, fmt.Errorf("you already have a pending payment. Please cancel it before choosing another plan")
			}
			if pending.SnapToken == "" || pending.SnapRedirectURL == "" {
				return nil, fmt.Errorf("pending payment cannot be resumed. Please cancel it and choose the plan again")
			}
			return &dto.CheckoutResponse{
				SubscriptionID: pending.ID,
				OrderID:        pending.MidtransOrderID,
				SnapToken:      pending.SnapToken,
				RedirectURL:    pending.SnapRedirectURL,
				ExpiresAt:      pending.PaymentExpiresAt,
				ClientKey:      s.clientKey,
				IsProduction:   s.isProd,
			}, nil
		}
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if active, err := s.repo.FindActiveByUserID(userID); err == nil && active != nil && active.Plan != nil && active.Plan.Code == plan.Code {
		return nil, fmt.Errorf("you are already subscribed to this plan")
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	u, err := s.users.FindByID(userID)
	if err != nil {
		return nil, err
	}
	referralCode := sanitizeReferralCode(req.ReferralCode)
	var referrer *domain.User
	if referralCode != "" {
		found, err := s.users.FindByReferralCode(referralCode)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, domain.ErrInvalidReferral
			}
			return nil, err
		}
		if found.ID == userID {
			return nil, domain.ErrInvalidReferral
		}
		referrer = found
	}

	orderID := fmt.Sprintf("SAKU-%s-%s", strings.ToUpper(plan.Code), strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:13], "-", "")))

	chargeAmount := int64(plan.Price)
	itemName := "SAKU " + plan.Name + " (" + plan.Period + ")"

	now := time.Now().UTC()
	paymentExpiresAt := now.Add(snapPaymentExpiry)
	sub := &domain.Subscription{
		UserID:           userID,
		PlanID:           plan.ID,
		Status:           domain.SubscriptionStatusPending,
		Amount:           plan.Price, // record the *full* recurring amount
		Currency:         plan.Currency,
		MidtransOrderID:  orderID,
		PaymentExpiresAt: &paymentExpiresAt,
		ReferralCode:     referralCode,
	}
	if referrer != nil {
		sub.ReferrerID = &referrer.ID
	}
	if err := s.repo.CreateSubscription(sub); err != nil {
		return nil, err
	}

	// Build Snap payload
	payload := map[string]any{
		"transaction_details": map[string]any{
			"order_id":     orderID,
			"gross_amount": chargeAmount, // IDR, integer
		},
		"customer_details": map[string]any{
			"first_name": u.Name,
			"email":      u.Email,
		},
		"item_details": []map[string]any{
			{
				"id":       plan.Code,
				"price":    chargeAmount,
				"quantity": 1,
				"name":     itemName,
			},
		},
		"credit_card": map[string]any{
			"secure": true,
		},
		"custom_expiry": map[string]any{
			"order_time":      now.Format("2006-01-02 15:04:05 -0700"),
			"expiry_duration": int(snapPaymentExpiry / time.Minute),
			"unit":            "minute",
		},
	}

	snap, err := s.midtrans.CreateSnapTransaction(ctx, payload)
	if err != nil {
		sub.Status = domain.SubscriptionStatusFailed
		_ = s.repo.UpdateSubscription(sub)
		return nil, fmt.Errorf("create snap transaction: %w", err)
	}

	sub.SnapToken = snap.Token
	sub.SnapRedirectURL = snap.RedirectURL
	if err := s.repo.UpdateSubscription(sub); err != nil {
		return nil, err
	}

	return &dto.CheckoutResponse{
		SubscriptionID: sub.ID,
		OrderID:        orderID,
		SnapToken:      snap.Token,
		RedirectURL:    snap.RedirectURL,
		ExpiresAt:      sub.PaymentExpiresAt,
		ClientKey:      s.clientKey,
		IsProduction:   s.isProd,
	}, nil
}

func (s *service) MySubscriptions(ctx context.Context, userID uuid.UUID) ([]dto.SubscriptionResponse, error) {
	rows, err := s.repo.ListByUserID(userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SubscriptionResponse, 0, len(rows))
	now := time.Now().UTC()
	for i := range rows {
		r := rows[i]
		if s.expirePendingIfNeeded(ctx, &r, now) {
			if err := s.repo.UpdateSubscription(&r); err != nil {
				return nil, err
			}
		}
		out = append(out, toSubResp(r))
	}
	return out, nil
}

func (s *service) ListAllAdmin(_ context.Context, limit, offset int) ([]dto.AdminSubscriptionResponse, error) {
	rows, err := s.repo.ListAll(limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]dto.AdminSubscriptionResponse, 0, len(rows))
	for _, r := range rows {
		base := toSubResp(r)
		entry := dto.AdminSubscriptionResponse{
			SubscriptionResponse: base,
			UserID:               r.UserID,
		}
		if r.User != nil {
			entry.UserName = r.User.Name
			entry.UserEmail = r.User.Email
			entry.UserLastLoginAt = r.User.LastLoginAt
			if strings.HasPrefix(r.User.Photo, "http://") || strings.HasPrefix(r.User.Photo, "https://") {
				entry.UserPhoto = r.User.Photo
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *service) ActiveSubscription(_ context.Context, userID uuid.UUID) (*dto.SubscriptionResponse, error) {
	row, err := s.repo.FindActiveByUserID(userID)
	if err != nil {
		return nil, err
	}
	if row.Plan == nil {
		if p, err := s.repo.FindPlanByID(row.PlanID); err == nil {
			row.Plan = p
		}
	}
	r := toSubResp(*row)
	return &r, nil
}

func (s *service) ConfirmCheckout(ctx context.Context, userID uuid.UUID, req dto.ConfirmSubscriptionRequest) (*dto.SubscriptionResponse, error) {
	sub, err := s.repo.FindByOrderID(req.OrderID)
	if err != nil {
		return nil, err
	}
	if sub.UserID != userID {
		return nil, domain.ErrUnauthorized
	}
	if s.expirePendingIfNeeded(ctx, sub, time.Now().UTC()) {
		if err := s.repo.UpdateSubscription(sub); err != nil {
			return nil, err
		}
	} else if sub.Status == domain.SubscriptionStatusPending && !s.isProd {
		wasActive := sub.Status == domain.SubscriptionStatusActive
		s.activate(sub)
		if err := s.repo.UpdateSubscription(sub); err != nil {
			return nil, err
		}
		if !wasActive && sub.Status == domain.SubscriptionStatusActive {
			s.sendPaymentSuccessEmail(sub)
		}
	}
	if sub.Plan == nil {
		if p, err := s.repo.FindPlanByID(sub.PlanID); err == nil {
			sub.Plan = p
		}
	}
	resp := toSubResp(*sub)
	return &resp, nil
}

func (s *service) Cancel(ctx context.Context, userID, id uuid.UUID) error {
	sub, err := s.repo.FindByUserID(userID, id)
	if err != nil {
		return err
	}
	if sub.Status != domain.SubscriptionStatusActive && sub.Status != domain.SubscriptionStatusPending {
		return fmt.Errorf("subscription cannot be cancelled from status %s", sub.Status)
	}
	if sub.Status == domain.SubscriptionStatusPending && s.midtrans != nil && strings.TrimSpace(sub.MidtransOrderID) != "" {
		if err := s.midtrans.CancelTransaction(ctx, sub.MidtransOrderID); err != nil {
			log.Printf("subscription: midtrans cancel failed order_id=%s: %v", sub.MidtransOrderID, err)
		}
	}
	sub.Status = domain.SubscriptionStatusCancelled
	sub.NextBillingAt = nil
	return s.repo.UpdateSubscription(sub)
}

func (s *service) HandleWebhook(_ context.Context, p dto.MidtransWebhook) error {
	if !s.midtrans.VerifySignature(p.OrderID, p.StatusCode, p.GrossAmount, p.SignatureKey) {
		return fmt.Errorf("invalid signature for order %s", p.OrderID)
	}
	sub, err := s.repo.FindByOrderID(p.OrderID)
	if err != nil {
		return err
	}

	sub.MidtransTxnID = p.TransactionID
	sub.MidtransPaymentType = p.PaymentType
	wasActive := sub.Status == domain.SubscriptionStatusActive

	switch p.TransactionStatus {
	case "capture":
		if p.FraudStatus == "challenge" {
			// keep as pending
		} else {
			s.activate(sub)
		}
	case "settlement":
		s.activate(sub)
	case "pending":
		sub.Status = domain.SubscriptionStatusPending
	case "deny", "cancel", "expire":
		if p.TransactionStatus == "expire" {
			sub.Status = domain.SubscriptionStatusExpired
		} else if p.TransactionStatus == "cancel" {
			sub.Status = domain.SubscriptionStatusCancelled
		} else {
			sub.Status = domain.SubscriptionStatusFailed
		}
	case "failure":
		sub.Status = domain.SubscriptionStatusFailed
	}
	if err := s.repo.UpdateSubscription(sub); err != nil {
		return err
	}
	if !wasActive && sub.Status == domain.SubscriptionStatusActive {
		s.sendPaymentSuccessEmail(sub)
	}
	return nil
}

func (s *service) activate(sub *domain.Subscription) {
	wasActive := sub.Status == domain.SubscriptionStatusActive
	now := time.Now().UTC()
	sub.Status = domain.SubscriptionStatusActive
	sub.PaidAt = &now
	if sub.StartsAt == nil {
		start := now
		sub.StartsAt = &start
	}

	period := domain.PlanPeriodMonthly
	if sub.Plan != nil {
		period = sub.Plan.Period
	} else if p, err := s.repo.FindPlanByID(sub.PlanID); err == nil {
		period = p.Period
	}

	anchor := *sub.StartsAt
	end := anchor
	switch period {
	case domain.PlanPeriodYearly:
		end = end.AddDate(1, 0, 0)
	default:
		end = end.AddDate(0, 1, 0)
	}
	sub.EndsAt = &end
	sub.NextBillingAt = &end
	if !wasActive && !sub.ReferralRewardPaid && sub.ReferrerID != nil {
		if err := s.users.AddReferralReward(*sub.ReferrerID, referralPaymentReward); err != nil {
			log.Printf("subscription: add referral reward failed: %v", err)
		} else {
			sub.ReferralRewardPaid = true
		}
	}
}

func (s *service) expirePendingIfNeeded(ctx context.Context, sub *domain.Subscription, now time.Time) bool {
	if sub == nil || sub.Status != domain.SubscriptionStatusPending {
		return false
	}
	if sub.PaymentExpiresAt == nil {
		deadline := sub.CreatedAt.UTC().Add(snapPaymentExpiry)
		sub.PaymentExpiresAt = &deadline
	}
	if now.Before(sub.PaymentExpiresAt.UTC()) {
		return false
	}
	if s.midtrans != nil && strings.TrimSpace(sub.MidtransOrderID) != "" {
		if err := s.midtrans.CancelTransaction(ctx, sub.MidtransOrderID); err != nil {
			log.Printf("subscription: midtrans expire cancel failed order_id=%s: %v", sub.MidtransOrderID, err)
		}
	}
	sub.Status = domain.SubscriptionStatusExpired
	sub.NextBillingAt = nil
	return true
}

func (s *service) sendPaymentSuccessEmail(sub *domain.Subscription) {
	if s.mailer == nil {
		return
	}
	u, err := s.users.FindByID(sub.UserID)
	if err != nil {
		log.Printf("subscription: payment email user lookup failed: %v", err)
		return
	}
	planName := "SAKU"
	planPeriod := ""
	if sub.Plan == nil {
		if p, err := s.repo.FindPlanByID(sub.PlanID); err == nil {
			sub.Plan = p
		}
	}
	if sub.Plan != nil {
		planName = sub.Plan.Name
		planPeriod = sub.Plan.Period
	}
	subject := "Your SAKU payment is confirmed"
	body := paymentSuccessEmailHTML(u.Name, planName, planPeriod, sub.Amount, sub.Currency, sub.MidtransOrderID, sub.EndsAt)
	if err := s.mailer.Send(u.Email, subject, body); err != nil {
		log.Printf("subscription: queue payment success email failed: %v", err)
	}
}

func paymentSuccessEmailHTML(name, planName, period string, amount float64, currency, orderID string, endsAt *time.Time) string {
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = "SAKU user"
	}
	validUntil := "-"
	if endsAt != nil {
		validUntil = endsAt.Format("02 Jan 2006 15:04 MST")
	}
	detail := fmt.Sprintf("Plan: %s %s\nAmount: %s %.0f\nOrder ID: %s\nActive until: %s", planName, period, currency, amount, orderID, validUntil)
	return mailer.BlueTemplate(mailer.BlueTemplateData{
		Title:       "Payment Confirmed",
		Preheader:   "Your SAKU subscription payment has been confirmed.",
		Badge:       "Subscription",
		Greeting:    fmt.Sprintf("Hi %s,", displayName),
		Intro:       "Your SAKU subscription payment has been confirmed. Your Pro access is ready to use.",
		DetailLabel: "Payment Detail",
		DetailValue: detail,
		Warning:     "You can now use Pro features such as AI receipt scanning, Chat with AI, and advanced finance insights.",
		Footer:      "Thank you for using SAKU. This is an automated email, please do not reply.",
	})
}

func sanitizeReferralCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func (s *service) HasActiveProSubscription(_ context.Context, userID uuid.UUID) (bool, error) {
	sub, err := s.repo.FindActiveByUserID(userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if sub == nil {
		return false, nil
	}
	if sub.Status != domain.SubscriptionStatusActive {
		return false, nil
	}
	planCode := ""
	if sub.Plan != nil {
		planCode = sub.Plan.Code
	} else if plan, err := s.repo.FindPlanByID(sub.PlanID); err == nil {
		planCode = plan.Code
	}
	return strings.HasPrefix(planCode, "pro") || strings.HasPrefix(planCode, "premium"), nil
}

func (s *service) ActivePlanCode(_ context.Context, userID uuid.UUID) (string, bool, error) {
	sub, err := s.repo.FindActiveByUserID(userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "free", false, nil
		}
		return "free", false, err
	}
	if sub == nil || sub.Status != domain.SubscriptionStatusActive {
		return "free", false, nil
	}
	if sub.Plan != nil && sub.Plan.Code != "" {
		return sub.Plan.Code, true, nil
	}
	return "pro", true, nil
}
