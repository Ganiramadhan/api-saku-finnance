package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateWalletRequest struct {
	Name           string     `json:"name" validate:"required,min=2,max=120" example:"BCA Tabungan"`
	Type           string     `json:"type" validate:"required,oneof=personal business shared" example:"personal"`
	Currency       string     `json:"currency" validate:"omitempty,len=3" example:"IDR"`
	Balance        float64    `json:"balance" validate:"gte=0" example:"1500000"`
	IsDefault      bool       `json:"is_default" example:"false"`
	TargetName     *string    `json:"target_name,omitempty" validate:"omitempty,max=120"`
	TargetAmount   *float64   `json:"target_amount,omitempty" validate:"omitempty,gt=0"`
	TargetDeadline *time.Time `json:"target_deadline,omitempty"`
}

type UpdateWalletRequest struct {
	Name           string     `json:"name" validate:"omitempty,min=2,max=120"`
	Type           string     `json:"type" validate:"omitempty,oneof=personal business shared"`
	Currency       string     `json:"currency" validate:"omitempty,len=3"`
	Balance        *float64   `json:"balance" validate:"omitempty,gte=0"`
	IsDefault      *bool      `json:"is_default"`
	TargetName     *string    `json:"target_name" validate:"omitempty,max=120"`
	TargetAmount   *float64   `json:"target_amount" validate:"omitempty,gte=0"`
	TargetDeadline *time.Time `json:"target_deadline"`
}

type WalletResponse struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	Name           string     `json:"name"`
	Type           string     `json:"type"`
	Currency       string     `json:"currency"`
	Balance        float64    `json:"balance"`
	IsDefault      bool       `json:"is_default"`
	TargetName     *string    `json:"target_name,omitempty"`
	TargetAmount   *float64   `json:"target_amount,omitempty"`
	TargetDeadline *time.Time `json:"target_deadline,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CreateCategoryRequest struct {
	Name  string `json:"name" validate:"required,min=2,max=120" example:"Food"`
	Type  string `json:"type" validate:"required,oneof=income expense" example:"expense"`
	Icon  string `json:"icon" validate:"omitempty,max=64" example:"utensils"`
	Color string `json:"color" validate:"omitempty,max=16" example:"#FF8800"`
}

type UpdateCategoryRequest struct {
	Name  string `json:"name" validate:"omitempty,min=2,max=120"`
	Type  string `json:"type" validate:"omitempty,oneof=income expense"`
	Icon  string `json:"icon" validate:"omitempty,max=64"`
	Color string `json:"color" validate:"omitempty,max=16"`
}

type CategoryResponse struct {
	ID       uuid.UUID  `json:"id"`
	UserID   *uuid.UUID `json:"user_id,omitempty"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Icon     string     `json:"icon,omitempty"`
	Color    string     `json:"color,omitempty"`
	IsSystem bool       `json:"is_system"`
}

// ─── Transaction ─────────────────────────────────────────────────────────────

type CreateTransactionRequest struct {
	WalletID        uuid.UUID `json:"wallet_id" validate:"required"`
	CategoryID      uuid.UUID `json:"category_id" validate:"required"`
	Amount          float64   `json:"amount" validate:"required,gt=0"`
	Type            string    `json:"type" validate:"required,oneof=income expense"`
	Description     string    `json:"description" validate:"omitempty,max=2000"`
	MerchantName    string    `json:"merchant_name" validate:"omitempty,max=255"`
	TransactionDate time.Time `json:"transaction_date" validate:"required"`
	Source          string    `json:"source" validate:"omitempty,oneof=manual ai_ocr import api"`
	ConfidenceScore *float64  `json:"confidence_score" validate:"omitempty,gte=0,lte=1"`
}

type UpdateTransactionRequest struct {
	WalletID        *uuid.UUID `json:"wallet_id"`
	CategoryID      *uuid.UUID `json:"category_id"`
	Amount          *float64   `json:"amount" validate:"omitempty,gt=0"`
	Type            string     `json:"type" validate:"omitempty,oneof=income expense"`
	Description     *string    `json:"description" validate:"omitempty,max=2000"`
	MerchantName    *string    `json:"merchant_name" validate:"omitempty,max=255"`
	TransactionDate *time.Time `json:"transaction_date"`
}

type TransactionResponse struct {
	ID              uuid.UUID `json:"id"`
	WalletID        uuid.UUID `json:"wallet_id"`
	CategoryID      uuid.UUID `json:"category_id"`
	Amount          float64   `json:"amount"`
	Type            string    `json:"type"`
	Description     string    `json:"description,omitempty"`
	MerchantName    string    `json:"merchant_name,omitempty"`
	TransactionDate time.Time `json:"transaction_date"`
	Source          string    `json:"source"`
	ConfidenceScore *float64  `json:"confidence_score,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TransactionListQuery struct {
	WalletID   string `query:"wallet_id"`
	CategoryID string `query:"category_id"`
	Type       string `query:"type"`
	Source     string `query:"source"`
	From       string `query:"from"` // ISO date
	To         string `query:"to"`
	Search     string `query:"q"`
	Page       int    `query:"page"`
	Limit      int    `query:"limit"`
}

type AIProcessingLogResponse struct {
	ID                uuid.UUID      `json:"id"`
	UserID            uuid.UUID      `json:"user_id"`
	UserName          string         `json:"user_name,omitempty"`
	UserEmail         string         `json:"user_email,omitempty"`
	Feature           string         `json:"feature"`
	Status            string         `json:"status"`
	ExtractedAmount   *float64       `json:"extracted_amount"`
	ExtractedMerchant string         `json:"extracted_merchant,omitempty"`
	ExtractedCategory string         `json:"extracted_category,omitempty"`
	ConfidenceScore   *float64       `json:"confidence_score,omitempty"`
	ModelVersion      string         `json:"model_version,omitempty"`
	LatencyMs         int            `json:"latency_ms,omitempty"`
	ErrorMessage      string         `json:"error_message,omitempty"`
	RawResponse       map[string]any `json:"raw_response,omitempty" swaggertype:"object"`
	ImageURL          string         `json:"image_url,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type DeleteAIProcessingLogsRequest struct {
	IDs []uuid.UUID `json:"ids" validate:"required,min=1,dive,required"`
}

type CreateBudgetRequest struct {
	WalletID    uuid.UUID `json:"wallet_id" validate:"required"`
	CategoryID  uuid.UUID `json:"category_id" validate:"required"`
	LimitAmount float64   `json:"limit_amount" validate:"required,gt=0"`
	Period      string    `json:"period" validate:"required,oneof=daily weekly monthly"`
}

type UpdateBudgetRequest struct {
	LimitAmount *float64 `json:"limit_amount" validate:"omitempty,gt=0"`
	Period      string   `json:"period" validate:"omitempty,oneof=daily weekly monthly"`
}

type BudgetResponse struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	WalletID    uuid.UUID `json:"wallet_id"`
	CategoryID  uuid.UUID `json:"category_id"`
	LimitAmount float64   `json:"limit_amount"`
	Period      string    `json:"period"`
	Spent       float64   `json:"spent"`
	Remaining   float64   `json:"remaining"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CategorizeRequest struct {
	Text           string   `json:"text" validate:"required,min=3" example:"Starbucks Coffee Rp 87.500"`
	UserCategories []string `json:"user_categories" validate:"omitempty" example:"Food,Shopping,Transport"`
	SessionID      string   `json:"session_id,omitempty" validate:"omitempty,max=120"`
}

type CategorizeResponse struct {
	Amount       float64          `json:"amount" example:"87500"`
	MerchantName string           `json:"merchant_name" example:"Starbucks"`
	Category     string           `json:"category" example:"Food & Beverage"`
	Type         string           `json:"type" example:"expense"`
	Confidence   float64          `json:"confidence" example:"0.95"`
	Date         string           `json:"date,omitempty" example:"2026-05-20"`
	NeedsReview  bool             `json:"needs_review" example:"false"`
	RawResponse  map[string]any   `json:"raw_response,omitempty" swaggertype:"object"`
	Transactions []CategorizeItem `json:"transactions,omitempty"`
}

type CategorizeItem struct {
	Amount       float64 `json:"amount" example:"14000"`
	MerchantName string  `json:"merchant_name" example:"Warung Bubur Ayam"`
	Category     string  `json:"category" example:"Food & Beverage"`
	Type         string  `json:"type" example:"expense"`
	Confidence   float64 `json:"confidence" example:"0.9"`
	Description  string  `json:"description,omitempty" example:"bubur ayam"`
	Date         string  `json:"date,omitempty" example:"2026-05-20"`
}

// ScanReceiptRequest accepts a base64-encoded receipt image (JPEG/PNG/WebP).
type ScanReceiptRequest struct {
	ImageBase64    string   `json:"image_base64" validate:"required" example:"iVBORw0KGgoAAAANSUhEUgAA..."`
	MediaType      string   `json:"media_type" validate:"omitempty,oneof=image/jpeg image/png image/webp" example:"image/jpeg"`
	UserCategories []string `json:"user_categories" validate:"omitempty"`
}

type ScanReceiptResponse struct {
	Amount       float64        `json:"amount" example:"87500"`
	MerchantName string         `json:"merchant_name" example:"Starbucks Sudirman"`
	Category     string         `json:"category" example:"Food & Beverage"`
	Type         string         `json:"type" example:"expense"`
	Currency     string         `json:"currency" example:"IDR"`
	Date         string         `json:"date" example:"2026-05-12"`
	Confidence   float64        `json:"confidence" example:"0.95"`
	NeedsReview  bool           `json:"needs_review" example:"false"`
	OCRText      string         `json:"ocr_text,omitempty"`
	LineItems    []string       `json:"line_items,omitempty"`
	RawResponse  map[string]any `json:"raw_response,omitempty" swaggertype:"object"`
}

type InsightsRequest struct {
	From  string `json:"from" validate:"omitempty" example:"2026-04-01"`
	To    string `json:"to" validate:"omitempty" example:"2026-04-30"`
	Limit int    `json:"limit" validate:"omitempty,gte=1,lte=500" example:"100"`
}

type InsightsResponse struct {
	Summary         string         `json:"summary" example:"You spent 4.2M this month, 18% above last month."`
	TopCategories   []string       `json:"top_categories" example:"Food,Transport,Shopping"`
	Recommendations []string       `json:"recommendations"`
	Anomalies       []string       `json:"anomalies"`
	HealthScore     int            `json:"health_score" example:"72"`
	Period          string         `json:"period" example:"2026-04-01 to 2026-04-30"`
	TotalIncome     float64        `json:"total_income" example:"8000000"`
	TotalExpense    float64        `json:"total_expense" example:"4250000"`
	RawResponse     map[string]any `json:"raw_response,omitempty" swaggertype:"object"`
}

type SuggestBudgetRequest struct {
	WalletID string `json:"wallet_id" validate:"omitempty,uuid"`
	Months   int    `json:"months" validate:"omitempty,gte=1,lte=12" example:"3"`
}

type BudgetSuggestion struct {
	Category    string  `json:"category" example:"Food & Beverage"`
	LimitAmount float64 `json:"limit_amount" example:"1500000"`
	Period      string  `json:"period" example:"monthly"`
	Reason      string  `json:"reason" example:"Average spend over last 3 months: 1.4M"`
}

type SuggestBudgetResponse struct {
	Suggestions []BudgetSuggestion `json:"suggestions"`
	Notes       string             `json:"notes,omitempty"`
	RawResponse map[string]any     `json:"raw_response,omitempty" swaggertype:"object"`
}

type ChatTurn struct {
	Role    string `json:"role" validate:"oneof=user assistant" example:"user"`
	Content string `json:"content" validate:"required,min=1,max=4000" example:"Berapa total pengeluaran saya?"`
}

type ChatRequest struct {
	Message        string     `json:"message" validate:"required,min=1,max=2000" example:"Berapa pengeluaran terbesar saya bulan ini?"`
	IncludeContext bool       `json:"include_context" example:"true"`
	History        []ChatTurn `json:"history,omitempty"`
	SessionID      string     `json:"session_id,omitempty" validate:"omitempty,max=120"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
}

type UpcomingBillingRequest struct {
	Name     string    `json:"name" validate:"required,min=2,max=120"`
	Provider string    `json:"provider,omitempty" validate:"omitempty,max=120"`
	Amount   float64   `json:"amount" validate:"required,gt=0"`
	Currency string    `json:"currency,omitempty" validate:"omitempty,max=8"`
	Cycle    string    `json:"cycle" validate:"required,oneof=weekly monthly yearly"`
	DueDate  time.Time `json:"due_date" validate:"required"`
	Status   string    `json:"status,omitempty" validate:"omitempty,oneof=active paused"`
	Notes    string    `json:"notes,omitempty" validate:"omitempty,max=500"`
}

type UpdateUpcomingBillingRequest struct {
	Name     *string    `json:"name,omitempty" validate:"omitempty,min=2,max=120"`
	Provider *string    `json:"provider,omitempty" validate:"omitempty,max=120"`
	Amount   *float64   `json:"amount,omitempty" validate:"omitempty,gt=0"`
	Currency *string    `json:"currency,omitempty" validate:"omitempty,max=8"`
	Cycle    *string    `json:"cycle,omitempty" validate:"omitempty,oneof=weekly monthly yearly"`
	DueDate  *time.Time `json:"due_date,omitempty"`
	Status   *string    `json:"status,omitempty" validate:"omitempty,oneof=active paused"`
	Notes    *string    `json:"notes,omitempty" validate:"omitempty,max=500"`
}

type UpcomingBillingResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Cycle     string    `json:"cycle"`
	DueDate   time.Time `json:"due_date"`
	Status    string    `json:"status"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateSavingsGoalRequest struct {
	Name         string     `json:"name" validate:"required,max=120"`
	Description  string     `json:"description" validate:"omitempty,max=2000"`
	TargetAmount float64    `json:"target_amount" validate:"required,gt=0"`
	Deadline     *time.Time `json:"deadline"`
	WalletID     *uuid.UUID `json:"wallet_id"`
	Icon         string     `json:"icon" validate:"omitempty,max=32"`
	Color        string     `json:"color" validate:"omitempty,max=16"`
}

type UpdateSavingsGoalRequest struct {
	Name         *string    `json:"name" validate:"omitempty,max=120"`
	Description  *string    `json:"description" validate:"omitempty,max=2000"`
	TargetAmount *float64   `json:"target_amount" validate:"omitempty,gt=0"`
	Deadline     *time.Time `json:"deadline"`
	WalletID     *uuid.UUID `json:"wallet_id"`
	Icon         *string    `json:"icon" validate:"omitempty,max=32"`
	Color        *string    `json:"color" validate:"omitempty,max=16"`
}

type ContributeSavingsGoalRequest struct {
	Amount float64 `json:"amount" validate:"required,gt=0"`
	Note   string  `json:"note" validate:"omitempty,max=255"`
}

type SavingsGoalResponse struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	WalletID      *uuid.UUID `json:"wallet_id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	TargetAmount  float64    `json:"target_amount"`
	CurrentAmount float64    `json:"current_amount"`
	Remaining     float64    `json:"remaining"`
	ProgressPct   float64    `json:"progress_pct"`
	Deadline      *time.Time `json:"deadline"`
	DaysLeft      *int       `json:"days_left"`
	Icon          string     `json:"icon"`
	Color         string     `json:"color"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type SavingsGoalContributionResponse struct {
	ID        uuid.UUID `json:"id"`
	GoalID    uuid.UUID `json:"goal_id"`
	Amount    float64   `json:"amount"`
	Source    string    `json:"source"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}
