package dto

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required" example:"password123"`
}

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=120" example:"John Doe"`
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required,min=6,max=72" example:"password123"`
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
