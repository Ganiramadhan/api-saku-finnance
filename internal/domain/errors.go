package domain

import "errors"

var (
	ErrNotFound                = errors.New("resource not found")
	ErrAlreadyExists           = errors.New("Email already registered")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrAccountNotVerified      = errors.New("account is not verified")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrForbidden               = errors.New("forbidden")
	ErrInvalidInput            = errors.New("invalid input")
	ErrGmailRequired           = errors.New("registration requires a Gmail address")
	ErrInvalidReferral         = errors.New("referral code not found")
	ErrEmailNotRegistered      = errors.New("email is not registered")
	ErrInvalidOTP              = errors.New("invalid or expired OTP code")
	ErrProSubscriptionRequired = errors.New("pro subscription required")
)
