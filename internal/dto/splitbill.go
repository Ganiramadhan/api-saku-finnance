package dto

import (
	"time"

	"github.com/google/uuid"
)

type SplitBillParticipantInput struct {
	ID     *uuid.UUID `json:"id,omitempty"`
	Name   string     `json:"name" validate:"required,min=1,max=120"`
	Phone  string     `json:"phone,omitempty" validate:"omitempty,max=32"`
	Amount float64    `json:"amount" validate:"gte=0"`
}

type CreateSplitBillRequest struct {
	Title        string                      `json:"title" validate:"required,min=1,max=120"`
	TotalAmount  float64                     `json:"total_amount" validate:"gte=0"`
	Currency     string                      `json:"currency,omitempty" validate:"omitempty,max=8"`
	Notes        string                      `json:"notes,omitempty"`
	Participants []SplitBillParticipantInput `json:"participants" validate:"required,min=1,dive"`
}

type UpdateSplitBillRequest struct {
	Title        string                      `json:"title" validate:"required,min=1,max=120"`
	TotalAmount  float64                     `json:"total_amount" validate:"gte=0"`
	Currency     string                      `json:"currency,omitempty" validate:"omitempty,max=8"`
	Notes        string                      `json:"notes,omitempty"`
	Participants []SplitBillParticipantInput `json:"participants" validate:"required,min=1,dive"`
}

type SplitBillParticipantResponse struct {
	ID     uuid.UUID  `json:"id"`
	Name   string     `json:"name"`
	Phone  string     `json:"phone,omitempty"`
	Amount float64    `json:"amount"`
	PaidAt *time.Time `json:"paid_at,omitempty"`
}

type SplitBillResponse struct {
	ID           uuid.UUID                      `json:"id"`
	OwnerUserID  uuid.UUID                      `json:"owner_user_id"`
	Title        string                         `json:"title"`
	TotalAmount  float64                        `json:"total_amount"`
	Currency     string                         `json:"currency"`
	Notes        string                         `json:"notes,omitempty"`
	Participants []SplitBillParticipantResponse `json:"participants"`
	CreatedAt    time.Time                      `json:"created_at"`
	UpdatedAt    time.Time                      `json:"updated_at"`
}

type SplitBillShareResponse struct {
	Text     string `json:"text"`
	WhatsApp string `json:"whatsapp_url"`
}
