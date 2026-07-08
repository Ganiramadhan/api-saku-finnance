package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type quotaSubscriptionStub struct {
	code           string
	active         bool
	paidHistory    bool
	activePlanErr  error
	paidHistoryErr error
}

func (s quotaSubscriptionStub) ActivePlanCode(context.Context, uuid.UUID) (string, bool, error) {
	if s.activePlanErr != nil {
		return "free", false, s.activePlanErr
	}
	if s.code == "" {
		s.code = "free"
	}
	return s.code, s.active, nil
}

func (s quotaSubscriptionStub) HasPaidSubscriptionHistory(context.Context, uuid.UUID) (bool, error) {
	return s.paidHistory, s.paidHistoryErr
}

func TestEnforceDailyQuotaBlocksExpiredPaidSubscriberBeforeFreeQuota(t *testing.T) {
	svc := &service{subs: quotaSubscriptionStub{code: "free", active: false, paidHistory: true}}

	err := svc.enforceDailyQuota(context.Background(), uuid.New(), []string{featureChat}, freeChatMonthlyLimit, "free quota reached")
	if err == nil {
		t.Fatal("expected expired paid subscriber to be blocked")
	}
	fiberErr, ok := err.(*fiber.Error)
	if !ok {
		t.Fatalf("expected fiber error, got %T", err)
	}
	if fiberErr.Code != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", fiberErr.Code)
	}
	if !strings.Contains(strings.ToLower(fiberErr.Message), "subscription has expired") {
		t.Fatalf("expected expired subscription message, got %q", fiberErr.Message)
	}
}
