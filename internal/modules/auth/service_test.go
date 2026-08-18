package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepo struct {
	users   map[uuid.UUID]*domain.User
	history map[uuid.UUID][]domain.UserPasswordHistory
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[uuid.UUID]*domain.User{}, history: map[uuid.UUID][]domain.UserPasswordHistory{}}
}

func (r *fakeUserRepo) FindAll(int, int, string) ([]domain.User, int64, error) { return nil, 0, nil }
func (r *fakeUserRepo) FindByID(id uuid.UUID) (*domain.User, error) {
	if u, ok := r.users[id]; ok {
		copy := *u
		return &copy, nil
	}
	return nil, domain.ErrNotFound
}
func (r *fakeUserRepo) FindByEmail(email string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			copy := *u
			return &copy, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *fakeUserRepo) FindByTelegramChatID(string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeUserRepo) FindByReferralCode(string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeUserRepo) Create(u *domain.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copy := *u
	r.users[u.ID] = &copy
	return nil
}
func (r *fakeUserRepo) Update(u *domain.User) error {
	copy := *u
	r.users[u.ID] = &copy
	return nil
}
func (r *fakeUserRepo) UpsertResetOTP(uuid.UUID, string, time.Time) error    { return nil }
func (r *fakeUserRepo) UpsertOTP(uuid.UUID, string, string, time.Time) error { return nil }
func (r *fakeUserRepo) FindOTP(uuid.UUID, string) (*domain.UserOTP, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeUserRepo) DeleteOTP(uuid.UUID) (bool, error) { return false, nil }
func (r *fakeUserRepo) ClearResetOTP(uuid.UUID) error     { return nil }
func (r *fakeUserRepo) ClearOTP(uuid.UUID, string) error  { return nil }
func (r *fakeUserRepo) ListPasswordHistory(userID uuid.UUID, limit int) ([]domain.UserPasswordHistory, error) {
	rows := append([]domain.UserPasswordHistory(nil), r.history[userID]...)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}
func (r *fakeUserRepo) AddPasswordHistory(userID uuid.UUID, hash string) error {
	r.history[userID] = append([]domain.UserPasswordHistory{{ID: uuid.New(), UserID: userID, PasswordHash: hash, CreatedAt: time.Now()}}, r.history[userID]...)
	return nil
}
func (r *fakeUserRepo) EnsureReferralCode(uuid.UUID, string) (*domain.UserReferral, error) {
	return nil, nil
}
func (r *fakeUserRepo) AddReferralReward(uuid.UUID, int64) error            { return nil }
func (r *fakeUserRepo) BindTelegramChatID(uuid.UUID, string) error          { return nil }
func (r *fakeUserRepo) UpdateTelegramUsernameByChatID(string, string) error { return nil }
func (r *fakeUserRepo) DisconnectTelegram(uuid.UUID) error                  { return nil }
func (r *fakeUserRepo) Delete(uuid.UUID) error                              { return nil }

type fakeMailer struct{}

func (fakeMailer) Send(string, string, string) error { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestChangePassword_WrongCurrentPassword_ReturnsCurrentPasswordMismatch(t *testing.T) {
	repo := newFakeUserRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("CorrectHorse1"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Email: "user@example.com", Password: string(hash), AuthProvider: "password"}

	svc := NewService(repo, jwt.New("test-secret", time.Hour), "", fakeMailer{})

	err = svc.ChangePassword(context.Background(), id, dto.ChangePasswordRequest{
		CurrentPassword: "TotallyWrongPassword1",
		NewPassword:     "BrandNewPass1",
	})

	if !errors.Is(err, domain.ErrCurrentPasswordMismatch) {
		t.Fatalf("err = %v, want ErrCurrentPasswordMismatch", err)
	}
	if errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err must not also satisfy ErrInvalidCredentials (would map to 401 and force-logout the user)")
	}

	stored := repo.users[id]
	if bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("CorrectHorse1")) != nil {
		t.Fatal("password should not have changed after a failed attempt")
	}
}

func TestChangePassword_CorrectCurrentPassword_Succeeds(t *testing.T) {
	repo := newFakeUserRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("CorrectHorse1"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Email: "user@example.com", Password: string(hash), AuthProvider: "password"}

	svc := NewService(repo, jwt.New("test-secret", time.Hour), "", fakeMailer{})

	if err := svc.ChangePassword(context.Background(), id, dto.ChangePasswordRequest{
		CurrentPassword: "CorrectHorse1",
		NewPassword:     "BrandNewPass1",
	}); err != nil {
		t.Fatalf("change password: %v", err)
	}

	stored := repo.users[id]
	if bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("BrandNewPass1")) != nil {
		t.Fatal("password should have changed to the new one")
	}
}

func TestLogin_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewService(repo, jwt.New("test-secret", time.Hour), "", fakeMailer{})

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Email:    "nobody@example.com",
		Password: "WhateverPass1",
	})

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLogin_UnknownEmailVsWrongPassword_TimingIsComparable(t *testing.T) {
	// Regression guard for the email-enumeration timing side-channel: the
	// "unknown email" branch must pay a bcrypt-comparison cost similar to
	// the "known email, wrong password" branch, otherwise response timing
	// leaks whether an email is registered even though both return an
	// identical error.
	if testing.Short() {
		t.Skip("timing-sensitive test skipped in -short mode")
	}

	repo := newFakeUserRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("CorrectHorse1"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Email: "user@example.com", Password: string(hash), AuthProvider: "password", Status: "active"}

	svc := NewService(repo, jwt.New("test-secret", time.Hour), "", fakeMailer{})

	measure := func(email string) time.Duration {
		start := time.Now()
		_, _ = svc.Login(context.Background(), dto.LoginRequest{Email: email, Password: "WrongPassword1"})
		return time.Since(start)
	}

	// Warm up (first bcrypt call can include extra scheduling overhead).
	measure("user@example.com")
	measure("nobody@example.com")

	knownEmail := measure("user@example.com")
	unknownEmail := measure("nobody@example.com")

	// Allow generous slack for CI/scheduler jitter — we're only guarding
	// against the old bug where the unknown-email path returned near-
	// instantly (orders of magnitude faster, not just a bit faster).
	ratio := float64(unknownEmail) / float64(knownEmail)
	if ratio < 0.3 {
		t.Fatalf("unknown-email login (%v) is suspiciously faster than known-email login (%v), ratio=%.2f — timing side-channel may have regressed", unknownEmail, knownEmail, ratio)
	}
}
