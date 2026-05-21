package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	TxnTypeIncome  = "income"
	TxnTypeExpense = "expense"

	TxnSourceManual = "manual"
	TxnSourceAIOCR  = "ai_ocr"
	TxnSourceImport = "import"
	TxnSourceAPI    = "api"

	AIStatusPending = "pending"
	AIStatusSuccess = "success"
	AIStatusFailed  = "failed"

	WalletTypePersonal = "personal"
	WalletTypeBusiness = "business"
	WalletTypeShared   = "shared"

	BudgetPeriodDaily   = "daily"
	BudgetPeriodWeekly  = "weekly"
	BudgetPeriodMonthly = "monthly"

	BillingCycleWeekly  = "weekly"
	BillingCycleMonthly = "monthly"
	BillingCycleYearly  = "yearly"

	BillingStatusActive = "active"
	BillingStatusPaused = "paused"
)

type Wallet struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index"`
	Name           string    `gorm:"type:varchar(120);not null"`
	Type           string    `gorm:"type:wallet_type_enum;not null;default:'personal'"`
	Currency       string    `gorm:"type:varchar(8);not null;default:'IDR'"`
	BalanceCached  float64   `gorm:"type:decimal(18,2);not null;default:0"`
	IsDefault      bool      `gorm:"not null;default:false"`
	TargetName     *string   `gorm:"type:varchar(120)"`
	TargetAmount   *float64  `gorm:"type:decimal(18,2)"`
	TargetDeadline *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (w *Wallet) BeforeCreate(_ *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

type Category struct {
	ID       uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID   *uuid.UUID `gorm:"type:uuid;index"`
	Name     string     `gorm:"type:varchar(120);not null"`
	Type     string     `gorm:"type:transaction_type_enum;not null;index"`
	Icon     string     `gorm:"type:varchar(64)"`
	Color    string     `gorm:"type:varchar(16)"`
	IsSystem bool       `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (c *Category) BeforeCreate(_ *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

type Transaction struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	WalletID        uuid.UUID `gorm:"type:uuid;not null;index"`
	CategoryID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Amount          float64   `gorm:"type:decimal(18,2);not null"`
	Type            string    `gorm:"type:transaction_type_enum;not null"`
	Description     string    `gorm:"type:text"`
	MerchantName    string    `gorm:"type:varchar(255)"`
	TransactionDate time.Time `gorm:"not null;index"`

	Source          string   `gorm:"type:transaction_source_enum;not null;default:'manual'"`
	ConfidenceScore *float64 `gorm:"type:numeric(4,3)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Wallet   *Wallet   `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (t *Transaction) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

type AIProcessingLog struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID  uuid.UUID `gorm:"type:uuid;not null;index"`
	Feature string    `gorm:"type:varchar(32);not null;index;default:'categorize'"`

	Status            string   `gorm:"type:ai_status_enum;not null;index;default:'pending'"`
	ExtractedAmount   *float64 `gorm:"type:decimal(18,2)"`
	ExtractedMerchant string   `gorm:"type:varchar(255)"`
	ExtractedCategory string   `gorm:"type:varchar(120)"`

	ConfidenceScore *float64 `gorm:"type:numeric(4,3)"`
	ModelVersion    string   `gorm:"type:varchar(64)"`
	LatencyMs       int      `gorm:"type:int"`
	ErrorMessage    string   `gorm:"type:text"`

	RawResponse string `gorm:"type:jsonb"`

	CreatedAt time.Time
	UpdatedAt time.Time

	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (l *AIProcessingLog) BeforeCreate(_ *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

type Budget struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	WalletID   uuid.UUID `gorm:"type:uuid;not null;index"`
	CategoryID uuid.UUID `gorm:"type:uuid;not null;index"`

	LimitAmount float64 `gorm:"type:decimal(18,2);not null"`
	Period      string  `gorm:"type:budget_period_enum;not null;default:'monthly'"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User     *User     `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Wallet   *Wallet   `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (b *Budget) BeforeCreate(_ *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

type SavingsGoal struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID  `gorm:"type:uuid;not null;index"`
	WalletID      *uuid.UUID `gorm:"type:uuid;index"`
	Name          string     `gorm:"type:varchar(120);not null"`
	Description   string     `gorm:"type:text"`
	TargetAmount  float64    `gorm:"type:decimal(18,2);not null"`
	CurrentAmount float64    `gorm:"type:decimal(18,2);not null;default:0"`
	Deadline      *time.Time
	Icon          string `gorm:"type:varchar(32)"`
	Color         string `gorm:"type:varchar(16)"`

	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`

	User   *User   `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Wallet *Wallet `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

func (g *SavingsGoal) BeforeCreate(_ *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

type SavingsGoalContribution struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	GoalID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Amount    float64   `gorm:"type:decimal(18,2);not null"`
	Source    string    `gorm:"type:varchar(32);not null;default:'manual'"` // manual | auto_income | recurring
	Note      string    `gorm:"type:varchar(255)"`
	CreatedAt time.Time

	Goal *SavingsGoal `gorm:"foreignKey:GoalID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (c *SavingsGoalContribution) BeforeCreate(_ *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

type UpcomingBilling struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID   uuid.UUID `gorm:"type:uuid;not null;index"`
	Name     string    `gorm:"type:varchar(120);not null"`
	Provider string    `gorm:"type:varchar(120)"`
	Amount   float64   `gorm:"type:decimal(18,2);not null"`
	Currency string    `gorm:"type:varchar(8);not null;default:'IDR'"`
	Cycle    string    `gorm:"type:varchar(20);not null;default:'monthly'"`
	DueDate  time.Time `gorm:"not null;index"`
	Status   string    `gorm:"type:varchar(20);not null;default:'active'"`
	Notes    string    `gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Type      string    `gorm:"type:varchar(40);not null;index"`
	Title     string    `gorm:"type:varchar(160);not null"`
	Message   string    `gorm:"type:text;not null"`
	RefType   string    `gorm:"type:varchar(40);index"`
	RefID     string    `gorm:"type:varchar(80);index"`
	ReadAt    *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time

	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (n *Notification) BeforeCreate(_ *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (b *UpcomingBilling) BeforeCreate(_ *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
