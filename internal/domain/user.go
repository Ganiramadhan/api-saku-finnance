package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name              string    `gorm:"type:varchar(255);not null"`
	Email             string    `gorm:"type:varchar(255);not null"`
	Photo             string    `gorm:"type:varchar(500)"`
	Password          string    `gorm:"type:varchar(255);not null"`
	Phone             string    `gorm:"type:varchar(32)"`
	Role              string    `gorm:"type:varchar(50);not null;default:'user'"`
	Status            string    `gorm:"type:varchar(20);not null;default:'active'"`
	ReferralCode      string    `gorm:"type:varchar(32);not null;default:''"`
	ReferralReward    int64     `gorm:"not null;default:0"`
	ResetOTP          string    `gorm:"type:varchar(255)"`
	ResetOTPExpiresAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
