package domain

import "errors"

var (
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("Email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailNotRegistered = errors.New("akun dengan email tersebut belum terdaftar")
	ErrInvalidOTP         = errors.New("kode OTP tidak valid atau sudah kedaluwarsa")
)
