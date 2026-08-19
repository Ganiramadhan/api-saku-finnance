package dto

type LandingChatRequest struct {
	Message  string     `json:"message" validate:"required,min=1,max=1000" example:"Kenapa kode OTP registrasi tidak masuk?"`
	History  []ChatTurn `json:"history,omitempty" validate:"omitempty,max=20,dive"`
	Language string     `json:"language,omitempty" validate:"omitempty,oneof=id en"`
}

type LandingChatResponse struct {
	Reply string `json:"reply"`
}
