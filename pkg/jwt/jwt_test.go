package jwt

import (
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestManager_GenerateAndValidate(t *testing.T) {
	m := New("test-secret", time.Hour)
	uid := uuid.New()

	tok, err := m.Generate(uid, "john@example.com", "user")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := m.Validate(tok)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("UserID = %s, want %s", claims.UserID, uid)
	}
	if claims.Email != "john@example.com" {
		t.Errorf("Email = %s", claims.Email)
	}
	if claims.Role != "user" {
		t.Errorf("Role = %s", claims.Role)
	}
}

func TestManager_Validate_RejectsWrongSecret(t *testing.T) {
	signer := New("secret-A", time.Hour)
	verifier := New("secret-B", time.Hour)
	tok, _ := signer.Generate(uuid.New(), "x@y.z", "user")
	if _, err := verifier.Validate(tok); err == nil {
		t.Fatal("expected error validating with wrong secret")
	}
}

func TestManager_Validate_RejectsExpired(t *testing.T) {
	const secret = "test-secret"
	m := New(secret, time.Hour)

	claims := Claims{
		UserID: uuid.New(),
		Email:  "x@y.z",
		Role:   "user",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	signed, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := m.Validate(signed); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestManager_Validate_RejectsNoneAlg(t *testing.T) {
	claims := Claims{
		UserID: uuid.New(),
		Email:  "evil@example.com",
		Role:   "admin",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok := jwtlib.NewWithClaims(jwtlib.SigningMethodNone, claims)
	signed, err := tok.SignedString(jwtlib.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}

	m := New("test-secret", time.Hour)
	if _, err := m.Validate(signed); err == nil {
		t.Fatal("expected validation to reject 'none' algorithm tokens")
	}
}

func TestManager_Validate_RejectsMalformed(t *testing.T) {
	m := New("test-secret", time.Hour)
	if _, err := m.Validate("not-a-jwt"); err == nil {
		t.Fatal("expected error for malformed token")
	}
	if _, err := m.Validate(strings.Repeat("a", 50)); err == nil {
		t.Fatal("expected error for garbage token")
	}
}
