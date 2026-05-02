package dto

import "github.com/google/uuid"

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=120" example:"John Doe"`
	Email    string `json:"email" validate:"required,email" example:"john@example.com"`
	Password string `json:"password" validate:"required,min=6,max=72" example:"password123"`
	Role     string `json:"role" validate:"omitempty,oneof=user admin" example:"user"`
	Photo    string `json:"photo,omitempty" example:"temp/users/avatar-1a2b3c4d.png"`
}

type UpdateUserRequest struct {
	Name     string `json:"name" validate:"omitempty,min=2,max=120" example:"Jane Doe"`
	Email    string `json:"email" validate:"omitempty,email" example:"jane@example.com"`
	Password string `json:"password,omitempty" validate:"omitempty,min=6,max=72" example:"newpassword123"`
	Role     string `json:"role" validate:"omitempty,oneof=user admin" example:"admin"`
	Photo    string `json:"photo,omitempty" example:"temp/users/avatar-1a2b3c4d.png"`
}

type UserResponse struct {
	ID       uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name     string    `json:"name" example:"John Doe"`
	Email    string    `json:"email" example:"john@example.com"`
	Role     string    `json:"role" example:"user"`
	Photo    string    `json:"photo,omitempty" example:"users/<uuid>/avatar.png"`
	PhotoURL string    `json:"photo_url,omitempty" example:"https://minio.local/starter/users/<uuid>/avatar.png?X-Amz-..."`
}

type UploadResponse struct {
	Image            string `json:"image" example:"temp/users/avatar-1a2b3c4d.png"`
	PreviewURL       string `json:"preview_url" example:"https://minio.local/starter/temp/users/avatar-1a2b3c4d.png?X-Amz-..."`
	PreviewExpiresIn int    `json:"preview_expires_in" example:"604800"`
}
