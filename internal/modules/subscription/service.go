package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/google/uuid"
)

type Service interface {
	// Public
	ListPlans(ctx context.Context) ([]dto.PlanResponse, error)
	// Authed
	Checkout(ctx context.Context, userID uuid.UUID, req dto.CheckoutRequest) (*dto.CheckoutResponse, error)
	MySubscriptions(ctx context.Context, userID uuid.UUID) ([]dto.SubscriptionResponse, error)
	ActiveSubscription(ctx context.Context, userID uuid.UUID) (*dto.SubscriptionResponse, error)
	// Admin
	ListAllAdmin(ctx context.Context, limit, offset int) ([]dto.AdminSubscriptionResponse, error)
	// Webhook
	HandleWebhook(ctx context.Context, payload dto.MidtransWebhook) error
}

type service struct {
	repo      Repository
	users     user.Repository
	midtrans  *MidtransClient
	clientKey string
	isProd    bool
}

func NewService(repo Repository, users user.Repository, m *MidtransClient, clientKey string, isProd bool) Service {
	return &service{repo: repo, users: users, midtrans: m, clientKey: clientKey, isProd: isProd}
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
		StartsAt:      s.StartsAt,
		EndsAt:        s.EndsAt,
		PaidAt:        s.PaidAt,
		NextBillingAt: s.NextBillingAt,
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
	u, err := s.users.FindByID(userID)
	if err != nil {
		return nil, err
	}

	orderID := fmt.Sprintf("SAKU-%s-%d", strings.ToUpper(plan.Code), time.Now().UnixMilli())

	chargeAmount := int64(plan.Price)
	itemName := "SAKU " + plan.Name + " (" + plan.Period + ")"

	sub := &domain.Subscription{
		UserID:          userID,
		PlanID:          plan.ID,
		Status:          domain.SubscriptionStatusPending,
		Amount:          plan.Price, // record the *full* recurring amount
		Currency:        plan.Currency,
		MidtransOrderID: orderID,
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
		ClientKey:      s.clientKey,
		IsProduction:   s.isProd,
	}, nil
}

func (s *service) MySubscriptions(_ context.Context, userID uuid.UUID) ([]dto.SubscriptionResponse, error) {
	rows, err := s.repo.ListByUserID(userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SubscriptionResponse, 0, len(rows))
	for _, r := range rows {
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
	return s.repo.UpdateSubscription(sub)
}

func (s *service) activate(sub *domain.Subscription) {
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
}
