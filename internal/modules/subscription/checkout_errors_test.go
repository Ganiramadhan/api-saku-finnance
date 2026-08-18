package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
)

type fakeSubRepo struct {
	plan           *domain.Plan
	pending        *domain.Subscription
	byUserID       *domain.Subscription
	findPaymentErr error
}

func (r *fakeSubRepo) ListActivePlans() ([]domain.Plan, error) { return nil, nil }
func (r *fakeSubRepo) FindPlanByCode(code string) (*domain.Plan, error) {
	if r.plan != nil && r.plan.Code == code {
		return r.plan, nil
	}
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) FindPlanByID(id uuid.UUID) (*domain.Plan, error) {
	if r.plan != nil && r.plan.ID == id {
		return r.plan, nil
	}
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) ListVouchers(int, int) ([]domain.Voucher, error) { return nil, nil }
func (r *fakeSubRepo) FindVoucherByID(uuid.UUID) (*domain.Voucher, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) FindVoucherByCode(string) (*domain.Voucher, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) CreateVoucher(*domain.Voucher) error           { return nil }
func (r *fakeSubRepo) UpdateVoucher(*domain.Voucher) error           { return nil }
func (r *fakeSubRepo) DeleteVoucher(uuid.UUID) error                 { return nil }
func (r *fakeSubRepo) CreateSubscription(*domain.Subscription) error { return nil }
func (r *fakeSubRepo) UpdateSubscription(*domain.Subscription) error { return nil }
func (r *fakeSubRepo) ClaimPendingEmail(uuid.UUID) (bool, error)     { return false, nil }
func (r *fakeSubRepo) FindByOrderID(string) (*domain.Subscription, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) FindActiveByUserID(uuid.UUID) (*domain.Subscription, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) FindPendingByUserID(uuid.UUID) (*domain.Subscription, error) {
	if r.pending != nil {
		return r.pending, nil
	}
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) HasPaidSubscriptionHistory(uuid.UUID) (bool, error) { return false, nil }
func (r *fakeSubRepo) HasCurrentActivePaidSubscription(uuid.UUID, time.Time) (bool, error) {
	return false, nil
}
func (r *fakeSubRepo) FindByUserID(userID, id uuid.UUID) (*domain.Subscription, error) {
	if r.byUserID != nil {
		return r.byUserID, nil
	}
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) ListByUserID(uuid.UUID) ([]domain.Subscription, error) { return nil, nil }
func (r *fakeSubRepo) ListAll(int, int) ([]domain.Subscription, error)       { return nil, nil }
func (r *fakeSubRepo) ListActiveForReminder() ([]domain.Subscription, error) { return nil, nil }
func (r *fakeSubRepo) ExpirePendingBefore(time.Time) error                   { return nil }
func (r *fakeSubRepo) CreatePayment(*domain.SubscriptionPayment) error       { return nil }
func (r *fakeSubRepo) UpdatePayment(*domain.SubscriptionPayment) error       { return nil }
func (r *fakeSubRepo) FindPaymentByOrderID(string) (*domain.SubscriptionPayment, error) {
	if r.findPaymentErr != nil {
		return nil, r.findPaymentErr
	}
	return nil, domain.ErrNotFound
}
func (r *fakeSubRepo) CreatePaymentEvent(*domain.SubscriptionPaymentEvent) error { return nil }
func (r *fakeSubRepo) IncrementVoucherUsage(string) error                        { return nil }

// fakeUsersRepo is a minimal stand-in for user.Repository — none of these
// tests exercise it beyond satisfying the interface.
type fakeUsersRepo struct{}

func (fakeUsersRepo) FindAll(int, int, string) ([]domain.User, int64, error) { return nil, 0, nil }
func (fakeUsersRepo) FindByID(uuid.UUID) (*domain.User, error)               { return nil, domain.ErrNotFound }
func (fakeUsersRepo) FindByEmail(string) (*domain.User, error)               { return nil, domain.ErrNotFound }
func (fakeUsersRepo) FindByTelegramChatID(string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (fakeUsersRepo) FindByReferralCode(string) (*domain.User, error)      { return nil, domain.ErrNotFound }
func (fakeUsersRepo) Create(*domain.User) error                            { return nil }
func (fakeUsersRepo) Update(*domain.User) error                            { return nil }
func (fakeUsersRepo) UpsertResetOTP(uuid.UUID, string, time.Time) error    { return nil }
func (fakeUsersRepo) UpsertOTP(uuid.UUID, string, string, time.Time) error { return nil }
func (fakeUsersRepo) FindOTP(uuid.UUID, string) (*domain.UserOTP, error) {
	return nil, domain.ErrNotFound
}
func (fakeUsersRepo) DeleteOTP(uuid.UUID) (bool, error) { return false, nil }
func (fakeUsersRepo) ClearResetOTP(uuid.UUID) error     { return nil }
func (fakeUsersRepo) ClearOTP(uuid.UUID, string) error  { return nil }
func (fakeUsersRepo) ListPasswordHistory(uuid.UUID, int) ([]domain.UserPasswordHistory, error) {
	return nil, nil
}
func (fakeUsersRepo) AddPasswordHistory(uuid.UUID, string) error { return nil }
func (fakeUsersRepo) EnsureReferralCode(uuid.UUID, string) (*domain.UserReferral, error) {
	return nil, nil
}
func (fakeUsersRepo) AddReferralReward(uuid.UUID, int64) error            { return nil }
func (fakeUsersRepo) BindTelegramChatID(uuid.UUID, string) error          { return nil }
func (fakeUsersRepo) UpdateTelegramUsernameByChatID(string, string) error { return nil }
func (fakeUsersRepo) DisconnectTelegram(uuid.UUID) error                  { return nil }
func (fakeUsersRepo) Delete(uuid.UUID) error                              { return nil }

func TestCheckout_PendingPaymentForDifferentPlan_ReturnsConflict(t *testing.T) {
	planA := &domain.Plan{ID: uuid.New(), Code: "pro", Price: 49000, Currency: "IDR"}
	planB := uuid.New()
	repo := &fakeSubRepo{
		plan: planA,
		pending: &domain.Subscription{
			ID:            uuid.New(),
			PlanID:        planB, // different plan than the one being checked out
			Status:        domain.SubscriptionStatusPending,
			PaymentStatus: domain.PaymentStatusPending,
		},
	}
	svc := NewService(repo, fakeUsersRepo{}, nil, nil, "", false)

	_, err := svc.Checkout(context.Background(), uuid.New(), dto.CheckoutRequest{PlanCode: "pro"})

	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestRenewInvoice_NonPendingSubscription_ReturnsConflict(t *testing.T) {
	repo := &fakeSubRepo{
		byUserID: &domain.Subscription{ID: uuid.New(), Status: domain.SubscriptionStatusActive},
	}
	svc := NewService(repo, fakeUsersRepo{}, nil, nil, "", false)

	_, err := svc.RenewInvoice(context.Background(), uuid.New(), uuid.New())

	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestCancel_NonCancellableStatus_ReturnsConflict(t *testing.T) {
	repo := &fakeSubRepo{
		byUserID: &domain.Subscription{ID: uuid.New(), Status: domain.SubscriptionStatusExpired},
	}
	svc := NewService(repo, fakeUsersRepo{}, nil, nil, "", false)

	err := svc.Cancel(context.Background(), uuid.New(), uuid.New())

	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}
