package subscription

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
)

// raceRepo is a minimal Repository fake backing a single subscription/payment
// pair behind a real mutex, so FindPaymentByOrderID/UpdateSubscription behave
// like a real datastore under concurrent access instead of silently racing
// on a plain map (which would just panic or corrupt without asserting
// anything about the code under test).
type raceRepo struct {
	fakeSubRepo
	mu      sync.Mutex
	sub     *domain.Subscription
	payment *domain.SubscriptionPayment
}

func (r *raceRepo) FindPaymentByOrderID(orderID string) (*domain.SubscriptionPayment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.payment == nil || r.payment.OrderID != orderID {
		return nil, domain.ErrNotFound
	}
	// Return copies so callers mutating their local pointer (as the service
	// does before UpdatePayment/UpdateSubscription) can't cross-contaminate
	// concurrent readers — matches gorm.First's copy-out semantics.
	paymentCopy := *r.payment
	subCopy := *r.sub
	paymentCopy.Subscription = &subCopy
	return &paymentCopy, nil
}

func (r *raceRepo) UpdatePayment(p *domain.SubscriptionPayment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	saved := *p
	r.payment = &saved
	return nil
}

func (r *raceRepo) UpdateSubscription(s *domain.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	saved := *s
	r.sub = &saved
	return nil
}

func (r *raceRepo) ClaimPendingEmail(uuid.UUID) (bool, error) { return true, nil }

// countingUsersRepo counts AddReferralReward calls so the test can assert on
// how many times a referral payout actually landed — the core observable
// this test exists to protect.
type countingUsersRepo struct {
	fakeUsersRepo
	mu    sync.Mutex
	calls int
}

func (u *countingUsersRepo) AddReferralReward(uuid.UUID, int64) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls++
	return nil
}

func signWebhook(serverKey, orderID, statusCode, grossAmount string) string {
	h := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(h[:])
}

// TestHandleWebhook_ConcurrentDuplicateDeliveries_AppliesPaidExactlyOnce
// simulates Midtrans retrying the same "settlement" notification for an
// order twice, arriving at HandleWebhook at (as close to) the same instant.
// Before the read-before-lock fix, both deliveries could observe the
// subscription as not-yet-active and both grant the referral reward /
// activate the subscription. After the fix, only one delivery should ever
// see wasActive == false and grant the reward.
func TestHandleWebhook_ConcurrentDuplicateDeliveries_AppliesPaidExactlyOnce(t *testing.T) {
	const serverKey = "test-server-key"
	planID := uuid.New()
	userID := uuid.New()
	referrerID := uuid.New()
	subID := uuid.New()
	orderID := "SAKU-PRO-TESTORDER123"

	plan := &domain.Plan{ID: planID, Code: "pro", Price: 29000, Currency: "IDR", Period: domain.PlanPeriodMonthly}

	sub := &domain.Subscription{
		ID:             subID,
		UserID:         userID,
		PlanID:         planID,
		Plan:           plan,
		Status:         domain.SubscriptionStatusPending,
		PaymentStatus:  domain.PaymentStatusPending,
		Amount:         29000,
		OriginalAmount: 29000,
		Currency:       "IDR",
		MidtransOrderID: orderID,
		ReferrerID:      &referrerID,
	}
	payment := &domain.SubscriptionPayment{
		ID:             uuid.New(),
		SubscriptionID: subID,
		UserID:         userID,
		OrderID:        orderID,
		Status:         domain.PaymentStatusPending,
		Amount:         29000,
		Currency:       "IDR",
		Subscription:   sub,
	}

	repo := &raceRepo{
		fakeSubRepo: fakeSubRepo{plan: plan},
		sub:         sub,
		payment:     payment,
	}
	users := &countingUsersRepo{}
	midtrans := NewMidtransClient(serverKey, false)

	svc := NewService(repo, users, midtrans, nil, "", false)

	grossAmount := "29000.00"
	statusCode := "200"
	sig := signWebhook(serverKey, orderID, statusCode, grossAmount)

	webhook := dto.MidtransWebhook{
		OrderID:           orderID,
		StatusCode:        statusCode,
		GrossAmount:       grossAmount,
		SignatureKey:      sig,
		TransactionStatus: "settlement",
		PaymentType:       "qris",
		TransactionID:     uuid.NewString(),
	}

	const deliveries = 10
	var wg sync.WaitGroup
	wg.Add(deliveries)
	for i := 0; i < deliveries; i++ {
		go func() {
			defer wg.Done()
			_ = svc.HandleWebhook(context.Background(), webhook)
		}()
	}
	wg.Wait()

	if users.calls != 1 {
		t.Fatalf("AddReferralReward called %d times, want exactly 1 (duplicate/concurrent webhook deliveries must not double-grant referral rewards)", users.calls)
	}

	repo.mu.Lock()
	finalStatus := repo.sub.Status
	finalPaymentStatus := repo.sub.PaymentStatus
	repo.mu.Unlock()

	if finalStatus != domain.SubscriptionStatusActive {
		t.Fatalf("final subscription status = %q, want %q", finalStatus, domain.SubscriptionStatusActive)
	}
	if finalPaymentStatus != domain.PaymentStatusPaid {
		t.Fatalf("final payment status = %q, want %q", finalPaymentStatus, domain.PaymentStatusPaid)
	}

	_ = time.Now() // keep time import if unused paths change
}
