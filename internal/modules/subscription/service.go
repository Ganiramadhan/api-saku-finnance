package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
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
	qrisPaymentExpiry           = 15 * time.Minute
	virtualAccountExpiry        = 24 * time.Hour
	snapPageExpiry              = 1 * time.Hour
	proLaunchDiscountRate       = 0.30
)

type Service interface {
	// Public
	ListPlans(ctx context.Context) ([]dto.PlanResponse, error)
	// Authed
	Checkout(ctx context.Context, userID uuid.UUID, req dto.CheckoutRequest) (*dto.CheckoutResponse, error)
	ValidateVoucher(ctx context.Context, req dto.ValidateVoucherRequest) (*dto.ValidateVoucherResponse, error)
	RenewInvoice(ctx context.Context, userID, subscriptionID uuid.UUID) (*dto.CheckoutResponse, error)
	MySubscriptions(ctx context.Context, userID uuid.UUID) ([]dto.SubscriptionResponse, error)
	ActiveSubscription(ctx context.Context, userID uuid.UUID) (*dto.SubscriptionResponse, error)
	ConfirmCheckout(ctx context.Context, userID uuid.UUID, req dto.ConfirmSubscriptionRequest) (*dto.SubscriptionResponse, error)
	Cancel(ctx context.Context, userID, id uuid.UUID) error
	HasActiveProSubscription(ctx context.Context, userID uuid.UUID) (bool, error)
	ActivePlanCode(ctx context.Context, userID uuid.UUID) (string, bool, error)
	// Admin
	ListAllAdmin(ctx context.Context, limit, offset int) ([]dto.AdminSubscriptionResponse, error)
	ListVouchersAdmin(ctx context.Context, limit, offset int) ([]dto.VoucherResponse, error)
	CreateVoucherAdmin(ctx context.Context, req dto.VoucherRequest) (*dto.VoucherResponse, error)
	UpdateVoucherAdmin(ctx context.Context, id uuid.UUID, req dto.VoucherRequest) (*dto.VoucherResponse, error)
	DeleteVoucherAdmin(ctx context.Context, id uuid.UUID) error
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
		ID:               s.ID,
		PlanID:           s.PlanID,
		Status:           s.Status,
		Amount:           s.Amount,
		Currency:         s.Currency,
		OrderID:          s.MidtransOrderID,
		PaymentStatus:    s.PaymentStatus,
		PaymentType:      s.MidtransPaymentType,
		ExpiresAt:        s.PaymentExpiresAt,
		PaymentCreatedAt: s.PaymentCreatedAt,
		PaymentPaidAt:    s.PaymentPaidAt,
		PaymentExpiredAt: s.PaymentExpiredAt,
		OriginalAmount:   s.OriginalAmount,
		DiscountAmount:   s.DiscountAmount,
		VoucherCode:      s.VoucherCode,
		StartsAt:         s.StartsAt,
		EndsAt:           s.EndsAt,
		PaidAt:           s.PaidAt,
		NextBillingAt:    s.NextBillingAt,
		ReferralCode:     s.ReferralCode,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
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

func (s *service) ValidateVoucher(_ context.Context, req dto.ValidateVoucherRequest) (*dto.ValidateVoucherResponse, error) {
	req.Sanitize()
	if strings.TrimSpace(req.VoucherCode) == "" {
		return nil, fmt.Errorf("%w: voucher code is not valid", domain.ErrInvalidVoucher)
	}
	plan, err := s.repo.FindPlanByCode(req.PlanCode)
	if err != nil {
		return nil, err
	}
	originalAmount, discountAmount, payAmount, voucher, err := s.calculateCheckoutAmounts(plan, req.VoucherCode, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return &dto.ValidateVoucherResponse{
		Code:           voucher.Code,
		DiscountType:   voucher.DiscountType,
		DiscountValue:  voucher.DiscountValue,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		PayAmount:      payAmount,
		Currency:       plan.Currency,
	}, nil
}

func (s *service) Checkout(ctx context.Context, userID uuid.UUID, req dto.CheckoutRequest) (*dto.CheckoutResponse, error) {
	req.Sanitize()
	plan, err := s.repo.FindPlanByCode(req.PlanCode)
	if err != nil {
		return nil, err
	}
	if plan.Price <= 0 {
		return nil, fmt.Errorf("plan %q is free and does not require checkout", plan.Code)
	}
	if pending, err := s.repo.FindPendingByUserID(userID); err == nil && pending != nil {
		s.refreshPendingPaymentStatus(ctx, pending)
		if s.expirePendingIfNeeded(ctx, pending, time.Now().UTC()) {
			if err := s.repo.UpdateSubscription(pending); err != nil {
				return nil, err
			}
		}
		if pending.PaymentStatus == domain.PaymentStatusExpired {
			pending.PlanID = plan.ID
			pending.Plan = plan
			return s.createInvoice(ctx, pending, plan, req.VoucherCode)
		}
		if pending.PaymentStatus == domain.PaymentStatusPending {
			if pending.PlanID != plan.ID {
				return nil, fmt.Errorf("you already have a pending payment. Please cancel it before choosing another plan")
			}
			effectiveVoucherCode := req.VoucherCode
			if strings.TrimSpace(effectiveVoucherCode) == "" {
				effectiveVoucherCode = pending.VoucherCode
			}
			_, expectedDiscount, expectedAmount, _, err := s.calculateCheckoutAmounts(plan, effectiveVoucherCode, time.Now().UTC())
			if err != nil {
				return nil, err
			}
			if math.Round(pending.Amount) != math.Round(expectedAmount) || math.Round(pending.DiscountAmount) != math.Round(expectedDiscount) {
				if s.midtrans != nil && strings.TrimSpace(pending.MidtransOrderID) != "" {
					if err := s.midtrans.CancelTransaction(ctx, pending.MidtransOrderID); err != nil {
						log.Printf("subscription: cancel superseded invoice failed order_id=%s: %v", pending.MidtransOrderID, err)
					}
				}
				return s.createInvoice(ctx, pending, plan, effectiveVoucherCode)
			}
			if pending.SnapToken == "" || pending.SnapRedirectURL == "" || pending.PaymentStatus == domain.PaymentStatusExpired {
				return s.createInvoice(ctx, pending, plan, req.VoucherCode)
			}
			return &dto.CheckoutResponse{
				SubscriptionID: pending.ID,
				OrderID:        pending.MidtransOrderID,
				SnapToken:      pending.SnapToken,
				RedirectURL:    pending.SnapRedirectURL,
				ExpiresAt:      pending.PaymentExpiresAt,
				PaymentStatus:  pending.PaymentStatus,
				OriginalAmount: pending.OriginalAmount,
				DiscountAmount: pending.DiscountAmount,
				Amount:         pending.Amount,
				VoucherCode:    pending.VoucherCode,
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

	now := time.Now().UTC()
	sub := &domain.Subscription{
		UserID:           userID,
		PlanID:           plan.ID,
		Status:           domain.SubscriptionStatusPending,
		PaymentStatus:    domain.PaymentStatusPending,
		Amount:           plan.Price,
		OriginalAmount:   plan.Price,
		Currency:         plan.Currency,
		MidtransOrderID:  fmt.Sprintf("SAKU-%s-%s", strings.ToUpper(plan.Code), strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:13], "-", ""))),
		PaymentCreatedAt: &now,
		ReferralCode:     referralCode,
	}
	if referrer != nil {
		sub.ReferrerID = &referrer.ID
	}
	if err := s.repo.CreateSubscription(sub); err != nil {
		return nil, err
	}
	sub.Plan = plan
	return s.createInvoice(ctx, sub, plan, req.VoucherCode)
}

func (s *service) RenewInvoice(ctx context.Context, userID, subscriptionID uuid.UUID) (*dto.CheckoutResponse, error) {
	sub, err := s.repo.FindByUserID(userID, subscriptionID)
	if err != nil {
		return nil, err
	}
	if sub.Status != domain.SubscriptionStatusPending {
		return nil, fmt.Errorf("invoice can only be renewed for pending subscriptions")
	}
	if s.expirePendingIfNeeded(ctx, sub, time.Now().UTC()) {
		if err := s.repo.UpdateSubscription(sub); err != nil {
			return nil, err
		}
	}
	if sub.Plan == nil {
		if p, err := s.repo.FindPlanByID(sub.PlanID); err == nil {
			sub.Plan = p
		}
	}
	return s.createInvoice(ctx, sub, sub.Plan, sub.VoucherCode)
}

func (s *service) createInvoice(ctx context.Context, sub *domain.Subscription, plan *domain.Plan, voucherCode string) (*dto.CheckoutResponse, error) {
	if sub == nil {
		return nil, domain.ErrNotFound
	}
	if plan == nil {
		p, err := s.repo.FindPlanByID(sub.PlanID)
		if err != nil {
			return nil, err
		}
		plan = p
	}
	u, err := s.users.FindByID(sub.UserID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	originalAmount, discountAmount, chargeAmountFloat, voucher, err := s.calculateCheckoutAmounts(plan, voucherCode, now)
	if err != nil {
		return nil, err
	}
	chargeAmount := int64(math.Round(chargeAmountFloat))
	orderID := fmt.Sprintf("SAKU-%s-%s", strings.ToUpper(plan.Code), strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:13], "-", "")))
	durationLabel := planDurationLabel(plan.Period)
	featureSummary := strings.Join(subscriptionFeatureSummary(plan.Code), ", ")
	firstName, lastName := splitCustomerName(u.Name)
	itemName := fmt.Sprintf("SAKU %s - %s AI Financial Assistant", plan.Name, durationLabel)
	itemDescription := fmt.Sprintf(
		"Langganan SAKU %s untuk membuka fitur premium dan membantu mengelola keuangan dengan dukungan AI. Durasi: %s. Fitur: %s.",
		plan.Name,
		durationLabel,
		featureSummary,
	)
	paymentExpiresAt := now.Add(snapPageExpiry)

	customerDetails := map[string]any{
		"first_name": firstName,
		"last_name":  lastName,
		"email":      u.Email,
	}
	if strings.TrimSpace(u.Phone) != "" {
		customerDetails["phone"] = u.Phone
	}

	payload := map[string]any{
		"transaction_details": map[string]any{
			"order_id":     orderID,
			"gross_amount": chargeAmount, // IDR, integer
		},
		"customer_details": customerDetails,
		"item_details": []map[string]any{
			{
				"id":            plan.Code,
				"price":         chargeAmount,
				"quantity":      1,
				"name":          itemName,
				"brand":         "SAKU",
				"category":      "Subscription",
				"merchant_name": "SAKU Finance",
				"description":   itemDescription,
			},
		},
		"custom_field1": fmt.Sprintf("Plan: SAKU %s", plan.Name),
		"custom_field2": fmt.Sprintf("Duration: %s", durationLabel),
		"custom_field3": fmt.Sprintf("Features: %s", featureSummary),
		"credit_card": map[string]any{
			"secure": true,
		},
		"custom_expiry": map[string]any{
			"order_time":      now.Format("2006-01-02 15:04:05 -0700"),
			"expiry_duration": int(snapPageExpiry / time.Minute),
			"unit":            "minute",
		},
	}

	snap, err := s.midtrans.CreateSnapTransaction(ctx, payload)
	if err != nil {
		sub.PaymentStatus = domain.PaymentStatusFailed
		_ = s.repo.UpdateSubscription(sub)
		return nil, fmt.Errorf("create snap transaction: %w", err)
	}

	sub.MidtransOrderID = orderID
	sub.MidtransTxnID = ""
	sub.MidtransPaymentType = ""
	sub.SnapToken = snap.Token
	sub.SnapRedirectURL = snap.RedirectURL
	sub.PaymentStatus = domain.PaymentStatusPending
	sub.PaymentCreatedAt = &now
	sub.PaymentExpiresAt = &paymentExpiresAt
	sub.PaymentPaidAt = nil
	sub.PaymentExpiredAt = nil
	sub.OriginalAmount = originalAmount
	sub.DiscountAmount = discountAmount
	sub.Amount = float64(chargeAmount)
	sub.Currency = plan.Currency
	sub.VoucherCode = ""
	if voucher != nil {
		sub.VoucherCode = voucher.Code
	}
	if err := s.repo.UpdateSubscription(sub); err != nil {
		return nil, err
	}
	payment := &domain.SubscriptionPayment{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		OrderID:        orderID,
		Status:         domain.PaymentStatusPending,
		Amount:         sub.Amount,
		Currency:       sub.Currency,
		SnapToken:      snap.Token,
		RedirectURL:    snap.RedirectURL,
		ExpiresAt:      &paymentExpiresAt,
	}
	if err := s.repo.CreatePayment(payment); err != nil {
		return nil, err
	}
	s.recordPaymentEvent(payment, "", domain.PaymentStatusPending, "invoice_created")

	return &dto.CheckoutResponse{
		SubscriptionID: sub.ID,
		OrderID:        orderID,
		SnapToken:      snap.Token,
		RedirectURL:    snap.RedirectURL,
		ExpiresAt:      sub.PaymentExpiresAt,
		PaymentStatus:  sub.PaymentStatus,
		OriginalAmount: sub.OriginalAmount,
		DiscountAmount: sub.DiscountAmount,
		Amount:         sub.Amount,
		VoucherCode:    sub.VoucherCode,
		ClientKey:      s.clientKey,
		IsProduction:   s.isProd,
	}, nil
}

func (s *service) refreshPendingPaymentStatus(ctx context.Context, sub *domain.Subscription) {
	if sub == nil || sub.PaymentStatus != domain.PaymentStatusPending || strings.TrimSpace(sub.MidtransOrderID) == "" {
		return
	}
	payment, err := s.repo.FindPaymentByOrderID(sub.MidtransOrderID)
	if err != nil {
		return
	}
	if err := s.syncPaymentFromMidtrans(ctx, payment, sub); err != nil {
		log.Printf("subscription: refresh pending midtrans status failed order_id=%s: %v", sub.MidtransOrderID, err)
	}
}

func splitCustomerName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "SAKU", "User"
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0], "SAKU User"
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func planDurationLabel(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "yearly", "annual", "annually":
		return "12 bulan"
	case "monthly", "":
		return "1 bulan"
	default:
		return period
	}
}

func (s *service) calculateCheckoutAmounts(plan *domain.Plan, voucherCode string, now time.Time) (float64, float64, float64, *domain.Voucher, error) {
	if plan == nil {
		return 0, 0, 0, nil, domain.ErrNotFound
	}
	originalAmount := plan.Price
	launchDiscount := launchPromoDiscount(plan)
	voucherBaseAmount := originalAmount - launchDiscount
	if voucherBaseAmount < 0 {
		voucherBaseAmount = 0
	}
	voucherDiscount, voucher, err := s.resolveVoucherDiscount(voucherCode, voucherBaseAmount, now)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	discountAmount := launchDiscount + voucherDiscount
	payAmount := originalAmount - discountAmount
	if payAmount < 1000 {
		payAmount = 1000
	}
	return originalAmount, discountAmount, payAmount, voucher, nil
}

func launchPromoDiscount(plan *domain.Plan) float64 {
	if plan == nil {
		return 0
	}
	code := strings.ToLower(strings.TrimSpace(plan.Code))
	period := strings.ToLower(strings.TrimSpace(plan.Period))
	if code != "pro" || period != domain.PlanPeriodMonthly {
		return 0
	}
	return math.Round(plan.Price * proLaunchDiscountRate)
}

func subscriptionFeatureSummary(code string) []string {
	base := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(code)), "_yearly")
	switch base {
	case "premium":
		return []string{
			"AI Chat 1.200/bulan",
			"OCR 300/bulan",
			"Advanced Reports",
			"PDF Export",
			"Priority Support",
		}
	case "pro":
		return []string{
			"Unlimited Wallets",
			"AI Chat 300/bulan",
			"OCR 100/bulan",
			"Split Bill",
			"Recurring Transactions",
			"Export CSV/Excel",
		}
	default:
		return []string{
			"Unlimited Transactions",
			"2 Wallets",
			"Budget Targets",
			"AI Chat 20/bulan",
			"OCR 10/bulan",
		}
	}
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
	payment, err := s.repo.FindPaymentByOrderID(req.OrderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			sub, subErr := s.repo.FindByOrderID(req.OrderID)
			if subErr != nil {
				return nil, err
			}
			payment = &domain.SubscriptionPayment{
				SubscriptionID: sub.ID,
				UserID:         sub.UserID,
				OrderID:        sub.MidtransOrderID,
				Status:         fallbackText(sub.PaymentStatus, domain.PaymentStatusPending),
				Amount:         sub.Amount,
				Currency:       sub.Currency,
				SnapToken:      sub.SnapToken,
				RedirectURL:    sub.SnapRedirectURL,
				ExpiresAt:      sub.PaymentExpiresAt,
				Subscription:   sub,
			}
		} else {
			return nil, err
		}
	}
	sub := payment.Subscription
	if sub == nil {
		sub, err = s.repo.FindByOrderID(payment.OrderID)
		if err != nil {
			return nil, err
		}
	}
	if sub.UserID != userID {
		return nil, domain.ErrUnauthorized
	}
	if sub.Status == domain.SubscriptionStatusPending && sub.PaymentStatus != domain.PaymentStatusPaid {
		if err := s.syncPaymentFromMidtrans(ctx, payment, sub); err != nil {
			log.Printf("subscription: sync midtrans status failed order_id=%s: %v", payment.OrderID, err)
		}
	}
	if sub.PaymentStatus != domain.PaymentStatusPaid && s.expirePendingIfNeeded(ctx, sub, time.Now().UTC()) {
		if err := s.repo.UpdateSubscription(sub); err != nil {
			return nil, err
		}
	} else if sub.Status == domain.SubscriptionStatusPending && !s.isProd {
		wasActive := sub.Status == domain.SubscriptionStatusActive
		s.activate(sub)
		from := payment.Status
		now := time.Now().UTC()
		payment.Status = domain.PaymentStatusPaid
		payment.PaidAt = &now
		if err := s.repo.UpdatePayment(payment); err != nil {
			return nil, err
		}
		if from != payment.Status {
			s.recordPaymentEvent(payment, from, payment.Status, "manual_confirm")
		}
		if err := s.repo.UpdateSubscription(sub); err != nil {
			return nil, err
		}
		if !wasActive && sub.Status == domain.SubscriptionStatusActive {
			if strings.TrimSpace(sub.VoucherCode) != "" && sub.DiscountAmount > 0 {
				if err := s.repo.IncrementVoucherUsage(sub.VoucherCode); err != nil {
					log.Printf("subscription: increment voucher usage failed code=%s: %v", sub.VoucherCode, err)
				}
			}
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
	if payment, err := s.repo.FindPaymentByOrderID(sub.MidtransOrderID); err == nil {
		from := payment.Status
		payment.Status = domain.PaymentStatusCancelled
		if err := s.repo.UpdatePayment(payment); err != nil {
			return err
		}
		if from != payment.Status {
			s.recordPaymentEvent(payment, from, payment.Status, "user_cancel")
		}
	}
	sub.Status = domain.SubscriptionStatusCancelled
	sub.PaymentStatus = domain.PaymentStatusCancelled
	sub.NextBillingAt = nil
	return s.repo.UpdateSubscription(sub)
}

func (s *service) HandleWebhook(_ context.Context, p dto.MidtransWebhook) error {
	if !s.midtrans.VerifySignature(p.OrderID, p.StatusCode, p.GrossAmount, p.SignatureKey) {
		return fmt.Errorf("invalid signature for order %s", p.OrderID)
	}
	payment, err := s.repo.FindPaymentByOrderID(p.OrderID)
	if err != nil {
		return err
	}
	sub := payment.Subscription
	if sub == nil {
		return domain.ErrNotFound
	}

	sub.MidtransTxnID = p.TransactionID
	sub.MidtransPaymentType = p.PaymentType
	payment.TransactionID = p.TransactionID
	payment.PaymentType = p.PaymentType
	wasActive := sub.Status == domain.SubscriptionStatusActive
	fromPaymentStatus := payment.Status
	now := time.Now().UTC()

	switch p.TransactionStatus {
	case "capture":
		if p.FraudStatus == "challenge" {
			payment.Status = domain.PaymentStatusPending
			sub.PaymentStatus = domain.PaymentStatusPending
		} else {
			payment.Status = domain.PaymentStatusPaid
			payment.PaidAt = &now
			sub.PaymentStatus = domain.PaymentStatusPaid
			sub.PaymentPaidAt = &now
			s.activate(sub)
		}
	case "settlement":
		payment.Status = domain.PaymentStatusPaid
		payment.PaidAt = &now
		sub.PaymentStatus = domain.PaymentStatusPaid
		sub.PaymentPaidAt = &now
		s.activate(sub)
	case "pending":
		payment.Status = domain.PaymentStatusPending
		sub.PaymentStatus = domain.PaymentStatusPending
		sub.Status = domain.SubscriptionStatusPending
		expiresAt := payment.CreatedAt.UTC().Add(expiryForPaymentType(p.PaymentType))
		payment.ExpiresAt = &expiresAt
		sub.PaymentExpiresAt = &expiresAt
	case "deny", "cancel", "expire":
		if p.TransactionStatus == "expire" {
			payment.Status = domain.PaymentStatusExpired
			payment.ExpiredAt = &now
			sub.PaymentStatus = domain.PaymentStatusExpired
			sub.PaymentExpiredAt = &now
			sub.Status = domain.SubscriptionStatusPending
		} else if p.TransactionStatus == "cancel" {
			payment.Status = domain.PaymentStatusCancelled
			sub.PaymentStatus = domain.PaymentStatusCancelled
			sub.Status = domain.SubscriptionStatusCancelled
		} else {
			payment.Status = domain.PaymentStatusFailed
			sub.PaymentStatus = domain.PaymentStatusFailed
		}
	case "failure":
		payment.Status = domain.PaymentStatusFailed
		sub.PaymentStatus = domain.PaymentStatusFailed
	}
	if err := s.repo.UpdatePayment(payment); err != nil {
		return err
	}
	if err := s.repo.UpdateSubscription(sub); err != nil {
		return err
	}
	if fromPaymentStatus != payment.Status {
		s.recordPaymentEvent(payment, fromPaymentStatus, payment.Status, "midtrans_"+p.TransactionStatus)
	}
	if !wasActive && sub.Status == domain.SubscriptionStatusActive {
		if strings.TrimSpace(sub.VoucherCode) != "" && sub.DiscountAmount > 0 {
			if err := s.repo.IncrementVoucherUsage(sub.VoucherCode); err != nil {
				log.Printf("subscription: increment voucher usage failed code=%s: %v", sub.VoucherCode, err)
			}
		}
		s.sendPaymentSuccessEmail(sub)
	}
	return nil
}

func (s *service) syncPaymentFromMidtrans(ctx context.Context, payment *domain.SubscriptionPayment, sub *domain.Subscription) error {
	if s.midtrans == nil || !s.midtrans.Enabled() || strings.TrimSpace(payment.OrderID) == "" {
		return nil
	}
	status, err := s.midtrans.GetTransactionStatus(ctx, payment.OrderID)
	if err != nil {
		return err
	}
	p := dto.MidtransWebhook{
		OrderID:           status.OrderID,
		StatusCode:        status.StatusCode,
		GrossAmount:       status.GrossAmount,
		TransactionStatus: status.TransactionStatus,
		FraudStatus:       status.FraudStatus,
		PaymentType:       status.PaymentType,
		TransactionID:     status.TransactionID,
	}
	return s.applyMidtransStatus(payment, sub, p, "midtrans_status_"+p.TransactionStatus)
}

func (s *service) applyMidtransStatus(payment *domain.SubscriptionPayment, sub *domain.Subscription, p dto.MidtransWebhook, eventSource string) error {
	if payment == nil || sub == nil {
		return domain.ErrNotFound
	}
	sub.MidtransTxnID = p.TransactionID
	sub.MidtransPaymentType = p.PaymentType
	payment.TransactionID = p.TransactionID
	payment.PaymentType = p.PaymentType
	wasActive := sub.Status == domain.SubscriptionStatusActive
	fromPaymentStatus := payment.Status
	now := time.Now().UTC()

	switch p.TransactionStatus {
	case "capture":
		if p.FraudStatus == "challenge" {
			payment.Status = domain.PaymentStatusPending
			sub.PaymentStatus = domain.PaymentStatusPending
		} else {
			payment.Status = domain.PaymentStatusPaid
			payment.PaidAt = &now
			sub.PaymentStatus = domain.PaymentStatusPaid
			sub.PaymentPaidAt = &now
			s.activate(sub)
		}
	case "settlement":
		payment.Status = domain.PaymentStatusPaid
		payment.PaidAt = &now
		sub.PaymentStatus = domain.PaymentStatusPaid
		sub.PaymentPaidAt = &now
		s.activate(sub)
	case "pending":
		payment.Status = domain.PaymentStatusPending
		sub.PaymentStatus = domain.PaymentStatusPending
		sub.Status = domain.SubscriptionStatusPending
		expiresAt := payment.CreatedAt.UTC().Add(expiryForPaymentType(p.PaymentType))
		payment.ExpiresAt = &expiresAt
		sub.PaymentExpiresAt = &expiresAt
	case "deny", "cancel", "expire":
		if p.TransactionStatus == "expire" {
			payment.Status = domain.PaymentStatusExpired
			payment.ExpiredAt = &now
			sub.PaymentStatus = domain.PaymentStatusExpired
			sub.PaymentExpiredAt = &now
			sub.Status = domain.SubscriptionStatusPending
		} else if p.TransactionStatus == "cancel" {
			payment.Status = domain.PaymentStatusCancelled
			sub.PaymentStatus = domain.PaymentStatusCancelled
			sub.Status = domain.SubscriptionStatusCancelled
		} else {
			payment.Status = domain.PaymentStatusFailed
			sub.PaymentStatus = domain.PaymentStatusFailed
		}
	case "failure":
		payment.Status = domain.PaymentStatusFailed
		sub.PaymentStatus = domain.PaymentStatusFailed
	default:
		return nil
	}
	if err := s.repo.UpdatePayment(payment); err != nil {
		return err
	}
	if err := s.repo.UpdateSubscription(sub); err != nil {
		return err
	}
	if fromPaymentStatus != payment.Status {
		s.recordPaymentEvent(payment, fromPaymentStatus, payment.Status, eventSource)
	}
	if !wasActive && sub.Status == domain.SubscriptionStatusActive {
		if strings.TrimSpace(sub.VoucherCode) != "" && sub.DiscountAmount > 0 {
			if err := s.repo.IncrementVoucherUsage(sub.VoucherCode); err != nil {
				log.Printf("subscription: increment voucher usage failed code=%s: %v", sub.VoucherCode, err)
			}
		}
		s.sendPaymentSuccessEmail(sub)
	}
	return nil
}

func (s *service) activate(sub *domain.Subscription) {
	wasActive := sub.Status == domain.SubscriptionStatusActive
	now := time.Now().UTC()
	sub.Status = domain.SubscriptionStatusActive
	sub.PaymentStatus = domain.PaymentStatusPaid
	sub.PaidAt = &now
	sub.PaymentPaidAt = &now
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
	if sub == nil || sub.Status != domain.SubscriptionStatusPending || sub.PaymentStatus != domain.PaymentStatusPending {
		return false
	}
	if sub.PaymentExpiresAt == nil {
		deadline := sub.CreatedAt.UTC().Add(snapPageExpiry)
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
	sub.PaymentStatus = domain.PaymentStatusExpired
	sub.PaymentExpiredAt = &now
	sub.NextBillingAt = nil
	if payment, err := s.repo.FindPaymentByOrderID(sub.MidtransOrderID); err == nil {
		from := payment.Status
		payment.Status = domain.PaymentStatusExpired
		payment.ExpiredAt = &now
		if payment.ExpiresAt == nil {
			payment.ExpiresAt = sub.PaymentExpiresAt
		}
		if err := s.repo.UpdatePayment(payment); err != nil {
			log.Printf("subscription: update expired payment failed order_id=%s: %v", payment.OrderID, err)
		} else if from != payment.Status {
			s.recordPaymentEvent(payment, from, payment.Status, "local_expiry")
		}
	}
	return true
}

func expiryForPaymentType(paymentType string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(paymentType)) {
	case "qris", "gopay", "gopay_later", "gopaylater":
		return qrisPaymentExpiry
	case "bank_transfer", "echannel", "permata", "bca_va", "bni_va", "bri_va", "mandiri_va", "cimb_va":
		return virtualAccountExpiry
	default:
		return snapPageExpiry
	}
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
		validUntil = endsAt.Format("02 Jan 2006")
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

func (s *service) resolveVoucherDiscount(code string, amount float64, now time.Time) (float64, *domain.Voucher, error) {
	code = sanitizeReferralCode(code)
	if code == "" {
		return 0, nil, nil
	}
	v, err := s.repo.FindVoucherByCode(code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, nil, fmt.Errorf("%w: voucher code is not valid", domain.ErrInvalidVoucher)
		}
		return 0, nil, err
	}
	if !v.IsActive {
		return 0, nil, fmt.Errorf("%w: voucher code is not active", domain.ErrInvalidVoucher)
	}
	if v.StartsAt != nil && now.Before(v.StartsAt.UTC()) {
		return 0, nil, fmt.Errorf("%w: voucher code is not active yet", domain.ErrInvalidVoucher)
	}
	if v.EndsAt != nil && now.After(v.EndsAt.UTC()) {
		return 0, nil, fmt.Errorf("%w: voucher code has expired", domain.ErrInvalidVoucher)
	}
	if v.MinAmount > 0 && amount < v.MinAmount {
		return 0, nil, fmt.Errorf("%w: minimum payment amount for this voucher is %.0f", domain.ErrInvalidVoucher, v.MinAmount)
	}
	if v.MaxRedemptions > 0 && v.UsedCount >= v.MaxRedemptions {
		return 0, nil, fmt.Errorf("%w: voucher code has reached its usage limit", domain.ErrInvalidVoucher)
	}
	discount := v.DiscountValue
	if v.DiscountType == domain.VoucherDiscountPercent {
		discount = amount * (v.DiscountValue / 100)
	}
	if v.MaxDiscount > 0 && discount > v.MaxDiscount {
		discount = v.MaxDiscount
	}
	if discount > amount {
		discount = amount
	}
	return math.Round(discount), v, nil
}

func (s *service) recordPaymentEvent(payment *domain.SubscriptionPayment, from, to, reason string) {
	if payment == nil || payment.ID == uuid.Nil {
		return
	}
	if err := s.repo.CreatePaymentEvent(&domain.SubscriptionPaymentEvent{
		SubscriptionID: payment.SubscriptionID,
		PaymentID:      payment.ID,
		OrderID:        payment.OrderID,
		FromStatus:     from,
		ToStatus:       to,
		Reason:         reason,
	}); err != nil {
		log.Printf("subscription: record payment event failed order_id=%s: %v", payment.OrderID, err)
	}
}

func fallbackText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func toVoucherResp(v domain.Voucher) dto.VoucherResponse {
	return dto.VoucherResponse{
		ID:             v.ID,
		Code:           v.Code,
		Name:           v.Name,
		DiscountType:   v.DiscountType,
		DiscountValue:  v.DiscountValue,
		MaxDiscount:    v.MaxDiscount,
		MinAmount:      v.MinAmount,
		MaxRedemptions: v.MaxRedemptions,
		UsedCount:      v.UsedCount,
		StartsAt:       v.StartsAt,
		EndsAt:         v.EndsAt,
		IsActive:       v.IsActive,
		CreatedAt:      v.CreatedAt,
		UpdatedAt:      v.UpdatedAt,
	}
}

func (s *service) ListVouchersAdmin(_ context.Context, limit, offset int) ([]dto.VoucherResponse, error) {
	rows, err := s.repo.ListVouchers(limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]dto.VoucherResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toVoucherResp(row))
	}
	return out, nil
}

func (s *service) CreateVoucherAdmin(_ context.Context, req dto.VoucherRequest) (*dto.VoucherResponse, error) {
	req.Sanitize()
	v := &domain.Voucher{
		Code:           req.Code,
		Name:           req.Name,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValue,
		MaxDiscount:    req.MaxDiscount,
		MinAmount:      req.MinAmount,
		MaxRedemptions: req.MaxRedemptions,
		StartsAt:       req.StartsAt,
		EndsAt:         req.EndsAt,
		IsActive:       req.IsActive,
	}
	if err := s.repo.CreateVoucher(v); err != nil {
		return nil, err
	}
	resp := toVoucherResp(*v)
	return &resp, nil
}

func (s *service) UpdateVoucherAdmin(_ context.Context, id uuid.UUID, req dto.VoucherRequest) (*dto.VoucherResponse, error) {
	req.Sanitize()
	existing, err := s.repo.FindVoucherByID(id)
	if err != nil {
		return nil, err
	}
	if other, err := s.repo.FindVoucherByCode(req.Code); err == nil && other.ID != id {
		return nil, fmt.Errorf("voucher code already exists")
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	existing.Code = req.Code
	existing.Name = req.Name
	existing.DiscountType = req.DiscountType
	existing.DiscountValue = req.DiscountValue
	existing.MaxDiscount = req.MaxDiscount
	existing.MinAmount = req.MinAmount
	existing.MaxRedemptions = req.MaxRedemptions
	existing.StartsAt = req.StartsAt
	existing.EndsAt = req.EndsAt
	existing.IsActive = req.IsActive
	if err := s.repo.UpdateVoucher(existing); err != nil {
		return nil, err
	}
	resp := toVoucherResp(*existing)
	return &resp, nil
}

func (s *service) DeleteVoucherAdmin(_ context.Context, id uuid.UUID) error {
	return s.repo.DeleteVoucher(id)
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
