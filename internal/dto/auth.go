package dto

import (
	"strings"
	"unicode"
)

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required" example:"password123"`
}

func (r *LoginRequest) Sanitize() {
	r.Email = sanitizeEmailInput(r.Email)
}

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=120" example:"John Doe"`
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required,min=8,max=72" example:"password123"`
}

func (r *RegisterRequest) Sanitize() {
	r.Name = sanitizeNameInput(r.Name)
	r.Email = sanitizeEmailInput(r.Email)
}

type AuthResponse struct {
	Token string       `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  UserResponse `json:"user"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required" example:"oldSecret123"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=72,nefield=CurrentPassword" example:"newSecret123"`
}

type GoogleLoginRequest struct {
	IDToken string `json:"id_token" validate:"required" example:"eyJhbGciOi..."`
	Mode    string `json:"mode,omitempty" validate:"omitempty,oneof=login register" example:"login"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email" example:"john@example.com"`
}

type ResetPasswordRequest struct {
	Email       string `json:"email" validate:"required,email" example:"john@example.com"`
	OTP         string `json:"otp" validate:"required,len=6" example:"123456"`
	NewPassword string `json:"new_password,omitempty" validate:"omitempty,min=8,max=72" example:"newSecret123"`
}

func sanitizeEmailInput(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sanitizeNameInput(value string) string {
	value = strings.TrimSpace(value)
	var out strings.Builder
	lastSpace := false
	for _, r := range value {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				out.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		out.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(out.String())
}

func sanitizeReferralInput(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}
