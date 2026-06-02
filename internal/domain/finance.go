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

	WalletTypeEWallet     = "e_wallet"
	WalletTypeBankAccount = "bank_account"
	WalletTypeCash        = "cash"
	WalletTypeCreditCard  = "credit_card"
	WalletTypeInvestment  = "investment"
	WalletTypeSavings     = "savings"

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
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID `gorm:"type:uuid;not null;index"`
	Name          string    `gorm:"type:varchar(120);not null"`
	Type          string    `gorm:"type:varchar(32);not null;default:'cash'"`
	Currency      string    `gorm:"type:varchar(8);not null;default:'IDR'"`
	BalanceCached float64   `gorm:"type:decimal(18,2);not null;default:0"`
	IsDefault     bool      `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User   *User         `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Target *WalletTarget `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (w *Wallet) BeforeCreate(_ *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

type WalletTarget struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	WalletID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	Name      string    `gorm:"type:varchar(120);not null"`
	Amount    float64   `gorm:"type:decimal(18,2);not null"`
	Deadline  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time

	Wallet *Wallet `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (t *WalletTarget) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

type WalletTransfer struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	FromWalletID uuid.UUID `gorm:"type:uuid;not null;index"`
	ToWalletID   uuid.UUID `gorm:"type:uuid;not null;index"`
	Amount       float64   `gorm:"type:decimal(18,2);not null"`
	Currency     string    `gorm:"type:varchar(8);not null;default:'IDR'"`
	Note         string    `gorm:"type:varchar(255)"`
	CreatedAt    time.Time

	User       *User   `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	FromWallet *Wallet `gorm:"foreignKey:FromWalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ToWallet   *Wallet `gorm:"foreignKey:ToWalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (t *WalletTransfer) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

type Category struct {
	ID       uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID   *uuid.UUID `gorm:"type:uuid;index"`
	Name     string     `gorm:"type:varchar(120);not null"`
	Type     string     `gorm:"type:varchar(16);not null;index"`
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
	Type            string    `gorm:"type:varchar(16);not null"`
	Description     string    `gorm:"type:text"`
	MerchantName    string    `gorm:"type:varchar(255)"`
	TransactionDate time.Time `gorm:"not null;index"`

	Source          string   `gorm:"type:varchar(20);not null;default:'manual'"`
	ConfidenceScore *float64 `gorm:"type:numeric(4,3)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Wallet   *Wallet   `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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

	Status            string   `gorm:"type:varchar(20);not null;index;default:'pending'"`
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

type SupportTicket struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Subject   string    `gorm:"type:varchar(180);not null"`
	Category  string    `gorm:"type:varchar(64);not null"`
	Priority  string    `gorm:"type:varchar(20);not null;default:'normal'"`
	Status    string    `gorm:"type:varchar(24);not null;default:'open';index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User     *User            `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Messages []SupportMessage `gorm:"foreignKey:TicketID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (t *SupportTicket) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

type SupportMessage struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	TicketID       uuid.UUID `gorm:"type:uuid;not null;index"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index"`
	Role           string    `gorm:"type:varchar(16);not null"`
	Body           string    `gorm:"type:text;not null"`
	AttachmentKey  string    `gorm:"type:varchar(512)"`
	AttachmentName string    `gorm:"type:varchar(255)"`
	AttachmentType string    `gorm:"type:varchar(80)"`
	CreatedAt      time.Time

	Ticket *SupportTicket `gorm:"foreignKey:TicketID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	User   *User          `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (m *SupportMessage) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (AIProcessingLog) TableName() string {
	return "ai_processing_logs"
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
	Period      string  `gorm:"type:varchar(20);not null;default:'monthly'"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User     *User     `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Wallet   *Wallet   `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Category *Category `gorm:"foreignKey:CategoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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
	Wallet *Wallet `gorm:"foreignKey:WalletID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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
