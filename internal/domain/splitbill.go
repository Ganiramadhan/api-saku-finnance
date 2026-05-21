package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SplitBill struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OwnerUserID uuid.UUID `gorm:"type:uuid;not null;index"`

	Title       string  `gorm:"type:varchar(120);not null"`
	TotalAmount float64 `gorm:"type:decimal(18,2);not null;default:0"`
	Currency    string  `gorm:"type:varchar(8);not null;default:'IDR'"`
	Notes       string  `gorm:"type:text"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Owner        *User                  `gorm:"foreignKey:OwnerUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Participants []SplitBillParticipant `gorm:"foreignKey:SplitBillID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (s *SplitBill) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type SplitBillParticipant struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	SplitBillID uuid.UUID `gorm:"type:uuid;not null;index"`

	Name   string  `gorm:"type:varchar(120);not null"`
	Phone  string  `gorm:"type:varchar(32)"`
	Amount float64 `gorm:"type:decimal(18,2);not null;default:0"`
	PaidAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	SplitBill *SplitBill `gorm:"foreignKey:SplitBillID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (p *SplitBillParticipant) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
