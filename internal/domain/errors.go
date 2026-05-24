package domain

import "errors"

var (
	ErrNotFound                = errors.New("resource not found")
	ErrAlreadyExists           = errors.New("Email already registered")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrInvalidInput            = errors.New("invalid input")
	ErrInvalidReferral         = errors.New("referral code not found")
	ErrEmailNotRegistered      = errors.New("email is not registered")
	ErrInvalidOTP              = errors.New("invalid or expired OTP code")
	ErrProSubscriptionRequired = errors.New("pro subscription required")
)
